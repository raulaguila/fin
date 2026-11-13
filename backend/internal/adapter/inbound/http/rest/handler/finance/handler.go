// Package finance serves the personal finance of the signed in user at /v1/finance.
package finance

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/port/input"
	"github.com/raulaguila/fin/pkg/apperror"
	"github.com/raulaguila/fin/pkg/reqctx"
)

type Handler struct {
	svc input.FinanceService
}

func New(svc input.FinanceService) *Handler {
	return &Handler{svc: svc}
}

// Routes expects a router already behind Bearer; chatLimit guards the paid assistant.
func (h *Handler) Routes(r fiber.Router, chatLimit fiber.Handler) {
	r.Get("/status", h.Status)
	r.Post("/connect-token", h.ConnectToken)
	r.Get("/connections", h.Connections)
	r.Post("/connections", h.AddConnection)
	r.Post("/connections/:id/sync", h.Sync)
	r.Delete("/connections/:id", h.DeleteConnection)
	r.Get("/summary", h.Summary)
	r.Get("/trend", h.Trend)
	r.Get("/people", h.People)
	r.Get("/transactions", h.Transactions)
	r.Patch("/transactions/:id", h.SetCategory)
	r.Post("/recategorize", h.Recategorize)
	r.Get("/categories", h.Categories)
	r.Post("/categories", h.CreateCategory)
	r.Patch("/categories/:id", h.UpdateCategory)
	r.Delete("/categories/:id", h.DeleteCategory)
	r.Get("/rules", h.Rules)
	r.Get("/rules/preview", h.PreviewRule)
	r.Get("/rules/suggestions", h.RuleSuggestions)
	r.Post("/rules", h.CreateRule)
	r.Delete("/rules/:id", h.DeleteRule)
	h.planningRoutes(r)
	h.extrasRoutes(r)
	h.trustRoutes(r)
	r.Post("/assistant/chat", chatLimit, h.Chat)
}

func invalidBody() error { return apperror.New(apperror.CodeInvalidInput, "invalid body") }

// ---- DTOs ----

type accountDTO struct {
	ID            string     `json:"id"`
	ConnectionID  string     `json:"connection_id"`
	Type          string     `json:"type" example:"BANK"`
	Subtype       string     `json:"subtype" example:"CHECKING_ACCOUNT"`
	Name          string     `json:"name"`
	Number        string     `json:"number"`
	Balance       int64      `json:"balance"` // cents; on a card, the open bill
	CreditLimit   *int64     `json:"credit_limit"`
	Available     *int64     `json:"available"`
	BillCloseDate *time.Time `json:"bill_close_date"`
	BillDueDate   *time.Time `json:"bill_due_date"`
	NextBillDue   *time.Time `json:"next_bill_due"` // bill_due_date, or the same day of a later month once it passed
	LastBillDue   *time.Time `json:"last_bill_due"`
	LastBill      *int64     `json:"last_bill"` // cents of the last closed bill
}

func toAccount(a domain.Account) accountDTO {
	return accountDTO{ID: a.ID, ConnectionID: a.ConnectionID, Type: a.Type, Subtype: a.Subtype, Name: a.Name, Number: a.Number, Balance: a.Balance,
		CreditLimit: a.CreditLimit, Available: a.Available, BillCloseDate: a.BillCloseDate, BillDueDate: a.BillDueDate,
		NextBillDue: a.NextBillDue(time.Now().UTC().Truncate(24 * time.Hour)), LastBillDue: a.LastBillDue, LastBill: a.LastBill}
}

type connectionDTO struct {
	ID               string       `json:"id"`
	ItemID           string       `json:"item_id"` // the provider item, for the widget to renew it
	Institution      string       `json:"institution"`
	ImageURL         string       `json:"image_url"`
	Status           string       `json:"status" example:"ok"`
	Error            string       `json:"error"`
	ConsentExpiresAt *time.Time   `json:"consent_expires_at"`
	LastSyncAt       *time.Time   `json:"last_sync_at"`
	CreatedAt        time.Time    `json:"created_at"`
	Accounts         []accountDTO `json:"accounts"`
	Manual           bool         `json:"manual"` // imported statements, no bank behind it
}

func toConnection(c domain.Connection) connectionDTO {
	out := connectionDTO{ID: c.ID, ItemID: c.ItemID, Institution: c.Institution, ImageURL: c.ImageURL, Status: c.Status, Error: c.Error,
		ConsentExpiresAt: c.ConsentExpiresAt, LastSyncAt: c.LastSyncAt, CreatedAt: c.CreatedAt, Accounts: []accountDTO{}, Manual: c.Manual()}
	for _, a := range c.Accounts {
		out.Accounts = append(out.Accounts, toAccount(a))
	}
	return out
}

type transactionDTO struct {
	ID             string `json:"id"`
	AccountID      string `json:"account_id"`
	AccountName    string `json:"account_name"`
	AccountType    string `json:"account_type"`
	Institution    string `json:"institution"`
	Date           string `json:"date" example:"2026-11-10"`
	Description    string `json:"description"`
	Amount         int64  `json:"amount"` // cents; negative leaves the user
	Status         string `json:"status"`
	CategoryID     string `json:"category_id"`
	CategoryName   string `json:"category_name"`
	CategoryKind   string `json:"category_kind"`
	CategorySource string `json:"category_source" example:"auto"`
	Installment    int    `json:"installment"`
	Installments   int    `json:"installments"`
	SplitOf        string   `json:"split_of"` // the original transaction when this row is a split part
	Note           string   `json:"note"`
	Tags           []string `json:"tags"`
}

func toTransaction(t domain.Transaction) transactionDTO {
	return transactionDTO{ID: t.ID, AccountID: t.AccountID, AccountName: t.AccountName, AccountType: t.AccountType, Institution: t.Institution,
		Date: t.Date.Format(time.DateOnly), Description: t.Description, Amount: t.Amount, Status: t.Status, CategoryID: t.CategoryID,
		CategoryName: t.CategoryName, CategoryKind: t.CategoryKind, CategorySource: t.CategorySource, Installment: t.Installment, Installments: t.Installments, SplitOf: t.SplitOf, Note: t.Note, Tags: t.Tags}
}

func toTransactions(list []domain.Transaction) []transactionDTO {
	out := make([]transactionDTO, 0, len(list))
	for _, t := range list {
		out = append(out, toTransaction(t))
	}
	return out
}

type categoryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind" example:"expense"`
	Limit    int64  `json:"limit"` // monthly, cents; 0 is none
	Rollover bool   `json:"rollover"`
}

type ruleDTO struct {
	ID           string `json:"id"`
	Keyword      string `json:"keyword"`
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	Hits         int64  `json:"hits"`
}

func toRule(r domain.Rule) ruleDTO {
	return ruleDTO{ID: r.ID, Keyword: r.Keyword, CategoryID: r.CategoryID, CategoryName: r.CategoryName, Hits: r.Hits}
}

// ---- status and connections ----

// @Summary Which finance features this server has configured
// @Tags Connections
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{open_finance=bool,assistant=bool}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/status [get]
func (h *Handler) Status(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"open_finance": h.svc.OpenFinanceEnabled(), "assistant": h.svc.AssistantEnabled()})
}

type connectTokenDTO struct {
	ConnectionID string `json:"connection_id"` // set to renew that connection
}

// @Summary Token for the Pluggy Connect widget, to link a bank or renew a connection
// @Tags Connections
// @Accept json
// @Produce json
// @Param body body connectTokenDTO false "Connection to renew"
// @Security BearerAuth
// @Success 200 {object} object{access_token=string}
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Router /v1/finance/connect-token [post]
func (h *Handler) ConnectToken(c fiber.Ctx) error {
	var body connectTokenDTO
	if len(c.Body()) > 0 {
		if err := c.Bind().Body(&body); err != nil {
			return mapper.WriteError(c, invalidBody())
		}
	}
	token, err := h.svc.ConnectToken(c.Context(), middleware.UserID(c), body.ConnectionID)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"access_token": token})
}

// @Summary List the bank connections of the current user with their accounts
// @Tags Connections
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]connectionDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/connections [get]
func (h *Handler) Connections(c fiber.Ctx) error {
	list, err := h.svc.Connections(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]connectionDTO, 0, len(list))
	for _, conn := range list {
		out = append(out, toConnection(conn))
	}
	return c.JSON(fiber.Map{"items": out})
}

type addConnectionDTO struct {
	ItemID string `json:"item_id"`
}

// @Summary Save the bank the widget linked; its data syncs in the background
// @Tags Connections
// @Accept json
// @Produce json
// @Param body body addConnectionDTO true "Item id the widget returned"
// @Security BearerAuth
// @Success 201 {object} connectionDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Router /v1/finance/connections [post]
func (h *Handler) AddConnection(c fiber.Ctx) error {
	var body addConnectionDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	conn, err := h.svc.AddConnection(c.Context(), middleware.UserID(c), body.ItemID)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toConnection(*conn))
}

// @Summary Ask the bank for fresh data and sync it in the background
// @Tags Connections
// @Param id path string true "Connection ID"
// @Security BearerAuth
// @Success 202
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/connections/{id}/sync [post]
func (h *Handler) Sync(c fiber.Ctx) error {
	if err := h.svc.SyncConnection(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusAccepted)
}

// @Summary Remove a connection, revoking it at the provider, with its accounts and transactions
// @Tags Connections
// @Param id path string true "Connection ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/connections/{id} [delete]
func (h *Handler) DeleteConnection(c fiber.Ctx) error {
	if err := h.svc.DeleteConnection(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- summary and transactions ----

type categoryTotalDTO struct {
	CategoryID string `json:"category_id"` // empty: uncategorized
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Limit      int64  `json:"limit"`
	Total      int64  `json:"total"`
	Previous   int64  `json:"previous"` // total of the month before
	Count      int64  `json:"count"`
	Rollover   bool   `json:"rollover"`
	Carry      int64  `json:"carry"` // with rollover: the part of limit that came from last month (negative: overspent)
}

type recurringDTO struct {
	Key      string `json:"key"` // send to /subscriptions/ignore to dismiss it
	Name     string `json:"name"`
	Amount   int64  `json:"amount"`
	Months   int    `json:"months"`
	LastDate string `json:"last_date"`
	Changed  bool   `json:"changed"`
	Previous int64  `json:"previous"`
	Last     int64  `json:"last"`
}

func toRecurring(r domain.Recurring) recurringDTO {
	return recurringDTO{Key: r.Key, Name: r.Name, Amount: r.Amount, Months: r.Months, LastDate: r.LastDate.Format(time.DateOnly), Changed: r.Changed, Previous: r.Previous, Last: r.Last}
}

type forecastDTO struct {
	DaysLeft       int   `json:"days_left"`
	PendingBills   int64 `json:"pending_bills"`   // recurring charges not seen yet this month, negative
	ExpectedIncome int64 `json:"expected_income"` // salary still to come
	Projected      int64 `json:"projected"`       // left at the end of the month
}

type installmentDTO struct {
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	Remaining   int    `json:"remaining"`
}

type summaryDTO struct {
	Month        string             `json:"month" example:"2026-11"`
	Income       int64              `json:"income"`
	Expense      int64              `json:"expense"`
	Categories   []categoryTotalDTO `json:"categories"`
	Accounts     []accountDTO       `json:"accounts"`
	Institutions map[string]string  `json:"institutions"` // connection id → institution
	Recent       []transactionDTO   `json:"recent"`
	Recurring    []recurringDTO     `json:"recurring"`
	Installments []installmentDTO   `json:"installments"`
	Forecast     *forecastDTO       `json:"forecast"` // only for the current month
	Investments  []investmentDTO    `json:"investments"`
}

type investmentDTO struct {
	ID           string `json:"id"`
	ConnectionID string `json:"connection_id"`
	Name         string `json:"name"`
	Type         string `json:"type" example:"FIXED_INCOME"`
	Balance      int64  `json:"balance"`
}

func parseMonth(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01", s)
	if err != nil || t.Year() < 2000 || t.Year() > 2100 {
		return time.Time{}, apperror.New(apperror.CodeInvalidInput, "month must be YYYY-MM").WithField("month")
	}
	return t, nil
}

// @Summary Month dashboard: totals by category, budgets, accounts, recurring charges and installments
// @Tags Transactions
// @Produce json
// @Param month query string false "Month as YYYY-MM; the current one when empty"
// @Security BearerAuth
// @Success 200 {object} summaryDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/summary [get]
func (h *Handler) Summary(c fiber.Ctx) error {
	month, err := parseMonth(c.Query("month"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	s, err := h.svc.Summary(c.Context(), middleware.UserID(c), month)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := summaryDTO{Month: s.Month.Format("2006-01"), Income: s.Income, Expense: s.Expense, Institutions: s.Institutions,
		Categories: []categoryTotalDTO{}, Accounts: []accountDTO{}, Recent: toTransactions(s.Recent), Recurring: []recurringDTO{}, Installments: []installmentDTO{}, Investments: []investmentDTO{}}
	for _, t := range s.Categories {
		dto := categoryTotalDTO{CategoryID: t.CategoryID, Name: t.Name, Kind: t.Kind, Limit: t.Limit, Total: t.Total, Count: t.Count, Rollover: t.Rollover, Carry: t.Carry}
		if t.CategoryID != "" || t.Total < 0 {
			dto.Previous = s.Previous[t.CategoryID]
		}
		out.Categories = append(out.Categories, dto)
	}
	for _, a := range s.Accounts {
		out.Accounts = append(out.Accounts, toAccount(a))
	}
	for _, r := range s.Recurring {
		out.Recurring = append(out.Recurring, toRecurring(r))
	}
	if f := s.Forecast; f != nil {
		out.Forecast = &forecastDTO{DaysLeft: f.DaysLeft, PendingBills: f.PendingBills, ExpectedIncome: f.ExpectedIncome, Projected: f.Projected}
	}
	for _, i := range s.Installments {
		out.Installments = append(out.Installments, installmentDTO{Description: i.Description, Amount: i.Amount, Remaining: i.Remaining})
	}
	for _, i := range s.Investments {
		out.Investments = append(out.Investments, investmentDTO{ID: i.ID, ConnectionID: i.ConnectionID, Name: i.Name, Type: i.Type, Balance: i.Balance})
	}
	return c.JSON(out)
}

type monthTotalDTO struct {
	Month   string `json:"month" example:"2026-11"`
	Income  int64  `json:"income"`
	Expense int64  `json:"expense"` // negative
}

// @Summary Income and expense by month, transfers left out, oldest first
// @Tags Transactions
// @Produce json
// @Param months query int false "How many months, up to 24" default(12)
// @Security BearerAuth
// @Success 200 {object} object{items=[]monthTotalDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/trend [get]
func (h *Handler) Trend(c fiber.Ctx) error {
	list, err := h.svc.Trend(c.Context(), middleware.UserID(c), fiber.Query[int](c, "months"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]monthTotalDTO, 0, len(list))
	for _, m := range list {
		out = append(out, monthTotalDTO{Month: m.Month.Format("2006-01"), Income: m.Income, Expense: m.Expense})
	}
	return c.JSON(fiber.Map{"items": out})
}

type personDTO struct {
	Name     string `json:"name"`
	Keyword  string `json:"keyword"` // matches transfers received from this person; empty when none came in
	Sent     int64  `json:"sent"`    // negative
	Received int64  `json:"received"`
	Refunded int64  `json:"refunded"` // received and filed as a refund
	Count    int    `json:"count"`
	LastDate string `json:"last_date"`
}

// @Summary Pix and TED of the last 12 months grouped by person
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]personDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/people [get]
func (h *Handler) People(c fiber.Ctx) error {
	list, err := h.svc.People(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]personDTO, 0, len(list))
	for _, p := range list {
		out = append(out, personDTO{Name: p.Name, Keyword: p.Keyword, Sent: p.Sent, Received: p.Received, Refunded: p.Refunded,
			Count: p.Count, LastDate: p.LastDate.Format(time.DateOnly)})
	}
	return c.JSON(fiber.Map{"items": out})
}

func parseDate(field, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, apperror.New(apperror.CodeInvalidInput, field+" must be YYYY-MM-DD").WithField(field)
	}
	return t, nil
}

func transactionFilter(c fiber.Ctx) (domain.TransactionFilter, error) {
	from, err := parseDate("from", c.Query("from"))
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	to, err := parseDate("to", c.Query("to"))
	if err != nil {
		return domain.TransactionFilter{}, err
	}
	return domain.TransactionFilter{From: from, To: to, Search: c.Query("search"), AccountID: c.Query("account_id"),
		CategoryID: c.Query("category_id"), Tag: c.Query("tag")}, nil
}

// @Summary List transactions of the current user, newest first
// @Tags Transactions
// @Produce json
// @Param from query string false "First day, YYYY-MM-DD"
// @Param to query string false "Day after the last, YYYY-MM-DD"
// @Param search query string false "Text in the description"
// @Param account_id query string false "Account ID"
// @Param category_id query string false "Category ID, or none for uncategorized"
// @Param page query int false "Page" default(1)
// @Param limit query int false "Items per page, up to 100" default(20)
// @Security BearerAuth
// @Success 200 {object} object{items=[]transactionDTO,pagination=mapper.PaginationDTO}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/transactions [get]
func (h *Handler) Transactions(c fiber.Ctx) error {
	f, err := transactionFilter(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	f.Page = domain.NewPage(fiber.Query[int](c, "page"), fiber.Query[int](c, "limit"))
	items, total, err := h.svc.Transactions(c.Context(), middleware.UserID(c), f)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"items": toTransactions(items), "pagination": mapper.Pagination(f.Page, total)})
}

type setCategoryDTO struct {
	CategoryID *string   `json:"category_id"` // empty clears it; absent keeps it
	Note       *string   `json:"note"`
	Tags       *[]string `json:"tags"` // replaces them all; needs note too when only tags change
}

// @Summary Set the category, or the note and tags, of a transaction; syncs and rules never change a category set here
// @Tags Transactions
// @Accept json
// @Param id path string true "Transaction ID"
// @Param body body setCategoryDTO true "Category"
// @Security BearerAuth
// @Produce json
// @Success 200 {object} object{keyword=string} "keyword: what a rule for this merchant would match; empty when none fits"
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/transactions/{id} [patch]
func (h *Handler) SetCategory(c fiber.Ctx) error {
	var body setCategoryDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	ctx, userID, id := c.Context(), middleware.UserID(c), c.Params("id")
	if body.Note != nil || body.Tags != nil {
		var note string
		var tags []string
		if body.Note != nil {
			note = *body.Note
		}
		if body.Tags != nil {
			tags = *body.Tags
		}
		if err := h.svc.SetTransactionDetails(ctx, userID, id, note, tags); err != nil {
			return mapper.WriteError(c, err)
		}
	}
	keyword := ""
	if body.CategoryID != nil {
		var err error
		if keyword, err = h.svc.SetCategory(ctx, userID, id, *body.CategoryID); err != nil {
			return mapper.WriteError(c, err)
		}
	}
	return c.JSON(fiber.Map{"keyword": keyword})
}

// @Summary Run the automatic categorization again over the history; hand and rule choices stay
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{changed=int}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/recategorize [post]
func (h *Handler) Recategorize(c fiber.Ctx) error {
	n, err := h.svc.Recategorize(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"changed": n})
}

// ---- categories and rules ----

// @Summary List the categories of the current user
// @Tags Categories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]categoryDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/categories [get]
func (h *Handler) Categories(c fiber.Ctx) error {
	list, err := h.svc.Categories(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]categoryDTO, 0, len(list))
	for _, cat := range list {
		out = append(out, categoryDTO{ID: cat.ID, Name: cat.Name, Kind: cat.Kind, Limit: cat.Limit, Rollover: cat.Rollover})
	}
	return c.JSON(fiber.Map{"items": out})
}

// @Summary Create a category
// @Tags Categories
// @Accept json
// @Produce json
// @Param body body categoryDTO true "Name, kind (expense, income or transfer) and monthly limit"
// @Security BearerAuth
// @Success 201 {object} categoryDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Router /v1/finance/categories [post]
func (h *Handler) CreateCategory(c fiber.Ctx) error {
	var body categoryDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	cat, err := h.svc.CreateCategory(c.Context(), middleware.UserID(c), domain.Category{Name: body.Name, Kind: body.Kind, Limit: body.Limit})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(categoryDTO{ID: cat.ID, Name: cat.Name, Kind: cat.Kind, Limit: cat.Limit, Rollover: cat.Rollover})
}

type updateCategoryDTO struct {
	Name  *string `json:"name"`
	Limit    *int64  `json:"limit"`
	Rollover *bool   `json:"rollover"`
}

// @Summary Rename a category or change its monthly limit
// @Tags Categories
// @Accept json
// @Produce json
// @Param id path string true "Category ID"
// @Param body body updateCategoryDTO true "Fields to change"
// @Security BearerAuth
// @Success 200 {object} categoryDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Router /v1/finance/categories/{id} [patch]
func (h *Handler) UpdateCategory(c fiber.Ctx) error {
	var body updateCategoryDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	cat, err := h.svc.UpdateCategory(c.Context(), middleware.UserID(c), c.Params("id"), body.Name, body.Limit, body.Rollover)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(categoryDTO{ID: cat.ID, Name: cat.Name, Kind: cat.Kind, Limit: cat.Limit, Rollover: cat.Rollover})
}

// @Summary Delete a category; its transactions become uncategorized
// @Tags Categories
// @Param id path string true "Category ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/categories/{id} [delete]
func (h *Handler) DeleteCategory(c fiber.Ctx) error {
	if err := h.svc.DeleteCategory(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary List the categorization rules of the current user
// @Tags Categories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]ruleDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/rules [get]
func (h *Handler) Rules(c fiber.Ctx) error {
	list, err := h.svc.Rules(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]ruleDTO, 0, len(list))
	for _, r := range list {
		out = append(out, toRule(r))
	}
	return c.JSON(fiber.Map{"items": out})
}

// @Summary Count the transactions a rule keyword would match
// @Tags Categories
// @Produce json
// @Param keyword query string true "Text the description contains"
// @Security BearerAuth
// @Success 200 {object} object{matches=int}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/rules/preview [get]
func (h *Handler) PreviewRule(c fiber.Ctx) error {
	n, err := h.svc.PreviewRule(c.Context(), middleware.UserID(c), c.Query("keyword"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"matches": n})
}

type ruleSuggestionDTO struct {
	Keyword string `json:"keyword"`
	Example string `json:"example"`
	Count   int    `json:"count"`
	Total   int64  `json:"total"`
}

// @Summary Merchants that repeat among uncategorized transactions, as rule candidates
// @Tags Categories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]ruleSuggestionDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/rules/suggestions [get]
func (h *Handler) RuleSuggestions(c fiber.Ctx) error {
	list, err := h.svc.RuleSuggestions(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]ruleSuggestionDTO, 0, len(list))
	for _, r := range list {
		out = append(out, ruleSuggestionDTO{Keyword: r.Keyword, Example: r.Example, Count: r.Count, Total: r.Total})
	}
	return c.JSON(fiber.Map{"items": out})
}

type createRuleDTO struct {
	Keyword    string `json:"keyword"`
	CategoryID string `json:"category_id"`
}

// @Summary Create a rule and apply it to past transactions not set by hand
// @Tags Categories
// @Accept json
// @Produce json
// @Param body body createRuleDTO true "Keyword and category"
// @Security BearerAuth
// @Success 201 {object} object{rule=ruleDTO,applied=int,undo_id=string}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Router /v1/finance/rules [post]
func (h *Handler) CreateRule(c fiber.Ctx) error {
	var body createRuleDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	rule, applied, err := h.svc.CreateRule(c.Context(), middleware.UserID(c), body.Keyword, body.CategoryID)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"rule": toRule(*rule), "applied": applied.Count, "undo_id": applied.UndoID})
}

// @Summary Delete a rule; transactions keep their category
// @Tags Categories
// @Param id path string true "Rule ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/rules/{id} [delete]
func (h *Handler) DeleteRule(c fiber.Ctx) error {
	if err := h.svc.DeleteRule(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- assistant ----

type chatMessageDTO struct {
	Role    string `json:"role" example:"user"`
	Content string `json:"content"`
}

type chatDTO struct {
	Messages []chatMessageDTO `json:"messages"`
}

type chatActionDTO struct {
	Kind         string `json:"kind" enums:"rule,subscription"`
	Keyword      string `json:"keyword"`
	CategoryID   string `json:"category_id,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	Name         string `json:"name,omitempty"`
	Amount       int64  `json:"amount,omitempty"`
	Day          int    `json:"day,omitempty"`
}

// chatEventDTO is one server-sent event of the chat stream.
type chatEventDTO struct {
	Type    string          `json:"type" enums:"delta,reset,status,done,error"`
	Text    string          `json:"text,omitempty"`
	Answer  string          `json:"answer"`
	Actions []chatActionDTO `json:"actions,omitempty"`
	Error   any             `json:"error,omitempty"`
}

// streamWindow is how long one write of the stream may take; each event
// extends it, so the answer can outlast API_WRITE_TIMEOUT.
const streamWindow = 30 * time.Second

// @Summary Ask the assistant about the current user's finances; the client keeps the history
// @Description Server-sent events, one JSON object per "data:" line: delta (answer text), reset (drop the text so far: it was a tool call), status (what the assistant is reading), then done (clean answer and proposed actions) or error. Actions change nothing until the client calls the rule or subscription endpoint.
// @Tags Assistant
// @Accept json
// @Produce text/event-stream
// @Param body body chatDTO true "Conversation, oldest first, ending with the user's message"
// @Security BearerAuth
// @Success 200 {object} chatEventDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/finance/assistant/chat [post]
func (h *Handler) Chat(c fiber.Ctx) error {
	var body chatDTO
	if err := c.Bind().Body(&body); err != nil || len(body.Messages) > 100 {
		return mapper.WriteError(c, invalidBody())
	}
	history := make([]domain.ChatMessage, 0, len(body.Messages))
	for _, m := range body.Messages {
		history = append(history, domain.ChatMessage{Role: m.Role, Content: m.Content})
	}
	userID := middleware.UserID(c)
	// The stream outlives the handler and its request context; AI_TIMEOUT bounds each model call.
	ctx, cancel := context.WithCancel(reqctx.WithRequestID(context.Background(), reqctx.RequestID(c.Context())))
	conn := c.RequestCtx().Conn()
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set("X-Accel-Buffering", "no")
	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer cancel()
		send := func(ev chatEventDTO) {
			data, _ := json.Marshal(ev)
			_ = conn.SetWriteDeadline(time.Now().Add(streamWindow))
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			if w.Flush() != nil {
				cancel() // the browser went away: stop paying for the answer
			}
		}
		reply, err := h.svc.Chat(ctx, userID, history, func(kind, text string) { send(chatEventDTO{Type: kind, Text: text}) })
		if err != nil {
			send(chatEventDTO{Type: "error", Error: mapper.ErrorJSON(err)})
			return
		}
		actions := make([]chatActionDTO, 0, len(reply.Actions))
		for _, a := range reply.Actions {
			actions = append(actions, chatActionDTO{Kind: a.Kind, Keyword: a.Keyword, CategoryID: a.CategoryID, CategoryName: a.CategoryName, Name: a.Name, Amount: int64(a.Amount), Day: a.Day})
		}
		send(chatEventDTO{Type: "done", Answer: reply.Answer, Actions: actions})
	})
}

// ---- webhook ----

type webhookDTO struct {
	Event  string `json:"event" example:"item/updated"`
	ItemID string `json:"itemId"`
}

// Webhook returns the provider webhook handler. The provider sends no custom
// header, so the secret travels in the URL query; an empty secret turns it off.
//
// @Summary Pluggy webhook: queues a sync of the item it names
// @Tags Webhooks
// @Accept json
// @Param token query string true "PLUGGY_WEBHOOK_SECRET"
// @Param body body webhookDTO true "Event"
// @Success 202
// @Failure 404 {object} mapper.errorBody
// @Router /v1/webhooks/pluggy [post]
func (h *Handler) Webhook(secret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if secret == "" || subtle.ConstantTimeCompare([]byte(c.Query("token")), []byte(secret)) != 1 {
			return mapper.WriteError(c, apperror.New(apperror.CodeNotFound, "not found"))
		}
		var body webhookDTO
		if err := c.Bind().Body(&body); err != nil {
			return mapper.WriteError(c, invalidBody())
		}
		if err := h.svc.Webhook(c.Context(), body.Event, body.ItemID); err != nil {
			return mapper.WriteError(c, err)
		}
		return c.SendStatus(fiber.StatusAccepted)
	}
}
