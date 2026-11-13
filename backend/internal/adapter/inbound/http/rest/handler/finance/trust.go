package finance

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/fin/internal/core/domain"
)

// trustRoutes are what keeps the numbers right: undo, transfer matching, balance check and data health.
func (h *Handler) trustRoutes(r fiber.Router) {
	r.Post("/undo/:id", h.Undo)
	r.Post("/transactions/pair", h.PairTransfers)
	r.Post("/accounts/:id/balance-check", h.ResetBalanceCheck)
	r.Get("/health", h.DataHealth)
	r.Get("/archive", h.ExportArchive)
	r.Post("/archive", h.ImportArchive)
}

// @Summary Put back what a bulk category change or a new rule overwrote, and drop the rule
// @Description Rows changed again since are left alone. Undos last 7 days.
// @Tags Transactions
// @Produce json
// @Param id path string true "Undo ID from the change"
// @Security BearerAuth
// @Success 200 {object} object{restored=int}
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/undo/{id} [post]
func (h *Handler) Undo(c fiber.Ctx) error {
	n, err := h.svc.Undo(c.Context(), middleware.UserID(c), c.Params("id"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"restored": n})
}

// @Summary File transfers between the user's own accounts (runs after every sync too)
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{paired=int}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/transactions/pair [post]
func (h *Handler) PairTransfers(c fiber.Ctx) error {
	n, err := h.svc.PairTransfers(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"paired": n})
}

// @Summary Accept the balance gap of a bank account and check it from now on
// @Tags Connections
// @Param id path string true "Account ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/accounts/{id}/balance-check [post]
func (h *Handler) ResetBalanceCheck(c fiber.Ctx) error {
	if err := h.svc.ResetBalanceCheck(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type accountHealthDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Institution string  `json:"institution"`
	Type        string  `json:"type" example:"BANK"`
	Manual      bool    `json:"manual"`
	LastDate    *string `json:"last_date" example:"2026-11-10"`
	CheckSince  *string `json:"check_since"`
	Gap         *int64  `json:"gap"` // bank accounts: balance moved beyond the transactions; null before the first check
}

type backupRunDTO struct {
	Kind   string `json:"kind" example:"dump"` // dump, copy or restore_check
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	At     string `json:"at"`
}

type dataHealthDTO struct {
	Accounts      []accountHealthDTO `json:"accounts"`
	Uncategorized int64              `json:"uncategorized"` // last 90 days
	Paired        int64              `json:"paired"`        // last 90 days
	Backups       []backupRunDTO     `json:"backups"`
}

// @Summary Whether the numbers can be trusted: account freshness, balance check, counts and backups
// @Tags Connections
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dataHealthDTO
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/health [get]
func (h *Handler) DataHealth(c fiber.Ctx) error {
	d, err := h.svc.DataHealth(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := dataHealthDTO{Accounts: []accountHealthDTO{}, Uncategorized: d.Uncategorized, Paired: d.Paired, Backups: []backupRunDTO{}}
	for _, a := range d.Accounts {
		dto := accountHealthDTO{ID: a.ID, Name: a.Name, Institution: a.Institution, Type: a.Type, Manual: a.Manual}
		if a.LastDate != nil {
			s := a.LastDate.Format(time.DateOnly)
			dto.LastDate = &s
		}
		if a.CheckSince != nil {
			s := a.CheckSince.Format(time.RFC3339)
			dto.CheckSince = &s
		}
		if a.Gap != nil {
			g := int64(*a.Gap)
			dto.Gap = &g
		}
		out.Accounts = append(out.Accounts, dto)
	}
	for _, b := range d.Backups {
		out.Backups = append(out.Backups, backupRunDTO{Kind: b.Kind, OK: b.OK, Detail: b.Detail, At: b.At.Format(time.RFC3339)})
	}
	return c.JSON(out)
}

// @Summary Everything the user set up as one JSON file: categories, rules, goals, subscriptions, debts, manual accounts with their transactions, and what was changed on bank transactions
// @Tags Connections
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object "Archive, version 1: see internal/core/domain/archive.go"
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/archive [get]
func (h *Handler) ExportArchive(c fiber.Ctx) error {
	a, err := h.svc.Export(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="fin-%s.json"`, time.Now().Format(time.DateOnly)))
	return c.JSON(a)
}

// @Summary Import one part of an archive: what is missing is added, nothing is deleted, a part sent twice adds nothing twice
// @Description Send the settings first, then at most 500 transactions of one account or 500 overrides per call.
// @Tags Connections
// @Accept json
// @Produce json
// @Param body body object true "Archive part: see internal/core/domain/archive.go"
// @Security BearerAuth
// @Success 200 {object} object "Archive, version 1: see internal/core/domain/archive.go"Result
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/archive [post]
func (h *Handler) ImportArchive(c fiber.Ctx) error {
	var body domain.Archive
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	res, err := h.svc.ImportArchive(c.Context(), middleware.UserID(c), body)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(res)
}
