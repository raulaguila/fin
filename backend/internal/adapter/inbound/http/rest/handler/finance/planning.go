package finance

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/fin/internal/core/domain"
)

// planningRoutes are the year report, calendar, splits, imports, subscriptions and goals.
func (h *Handler) planningRoutes(r fiber.Router) {
	r.Post("/transactions/:id/split", h.Split)
	r.Delete("/transactions/:id/split", h.Unsplit)
	r.Get("/year", h.Year)
	r.Get("/calendar", h.Calendar)
	r.Post("/accounts", h.CreateManualAccount)
	r.Post("/accounts/:id/import", h.Import)
	r.Get("/subscriptions", h.Subscriptions)
	r.Post("/subscriptions", h.CreateSubscription)
	r.Post("/subscriptions/ignore", h.IgnoreRecurring)
	r.Put("/subscriptions/:id", h.UpdateSubscription)
	r.Delete("/subscriptions/:id", h.DeleteSubscription)
	r.Get("/goals", h.Goals)
	r.Post("/goals", h.CreateGoal)
	r.Put("/goals/:id", h.UpdateGoal)
	r.Delete("/goals/:id", h.DeleteGoal)
}

// ---- splits ----

type splitDTO struct {
	Amount     int64  `json:"amount"` // cents, same sign as the transaction and smaller
	CategoryID string `json:"category_id"`
}

// @Summary Move part of a transaction to another category
// @Tags Transactions
// @Accept json
// @Param id path string true "Transaction ID"
// @Param body body splitDTO true "Part and its category"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/transactions/{id}/split [post]
func (h *Handler) Split(c fiber.Ctx) error {
	var body splitDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	if err := h.svc.SplitTransaction(c.Context(), middleware.UserID(c), c.Params("id"), body.Amount, body.CategoryID); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Join the parts of a split transaction again
// @Tags Transactions
// @Param id path string true "ID of the original transaction"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/transactions/{id}/split [delete]
func (h *Handler) Unsplit(c fiber.Ctx) error {
	if err := h.svc.UnsplitTransaction(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ---- year report ----

type yearDTO struct {
	Year       int                `json:"year"`
	Income     int64              `json:"income"`
	Expense    int64              `json:"expense"`
	Categories []categoryTotalDTO `json:"categories"`
	Months     []monthTotalDTO    `json:"months"`
}

// @Summary Totals of a calendar year by category and month, for the income tax return
// @Tags Transactions
// @Produce json
// @Param year query int false "Year; the current one when absent"
// @Security BearerAuth
// @Success 200 {object} yearDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/year [get]
func (h *Handler) Year(c fiber.Ctx) error {
	r, err := h.svc.YearReport(c.Context(), middleware.UserID(c), fiber.Query[int](c, "year"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := yearDTO{Year: r.Year, Income: r.Income, Expense: r.Expense, Categories: []categoryTotalDTO{}, Months: []monthTotalDTO{}}
	for _, t := range r.Categories {
		out.Categories = append(out.Categories, categoryTotalDTO{CategoryID: t.CategoryID, Name: t.Name, Kind: t.Kind, Limit: t.Limit, Total: t.Total, Count: t.Count})
	}
	for _, m := range r.Months {
		out.Months = append(out.Months, monthTotalDTO{Month: m.Month.Format("2006-01"), Income: m.Income, Expense: m.Expense})
	}
	return c.JSON(out)
}

// ---- calendar ----

type calendarEventDTO struct {
	Date    string `json:"date" example:"2026-11-20"`
	Kind    string `json:"kind" example:"bill"` // bill, recurring or income
	Name    string `json:"name"`
	Amount  int64  `json:"amount"`
	Balance int64  `json:"balance"` // bank balance after the event
}

type calendarDTO struct {
	Start  int64              `json:"start"` // bank balance today
	Lowest int64              `json:"lowest"`
	Events []calendarEventDTO `json:"events"`
}

// @Summary Expected bills, charges and income of the next 30 days with the projected balance
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} calendarDTO
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/calendar [get]
func (h *Handler) Calendar(c fiber.Ctx) error {
	cal, err := h.svc.Calendar(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := calendarDTO{Start: cal.Start, Lowest: cal.Lowest, Events: []calendarEventDTO{}}
	for _, e := range cal.Events {
		out.Events = append(out.Events, calendarEventDTO{Date: e.Date.Format(time.DateOnly), Kind: e.Kind, Name: e.Name, Amount: e.Amount, Balance: e.Balance})
	}
	return c.JSON(out)
}

// ---- imported accounts ----

type manualAccountDTO struct {
	Institution string `json:"institution" example:"Nubank"`
	Name        string `json:"name" example:"Cartão"`
	Type        string `json:"type" example:"CREDIT"` // BANK or CREDIT
}

// @Summary Create an account for imported statements (OFX or CSV)
// @Tags Connections
// @Accept json
// @Produce json
// @Param body body manualAccountDTO true "Institution, name and type"
// @Security BearerAuth
// @Success 201 {object} accountDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/accounts [post]
func (h *Handler) CreateManualAccount(c fiber.Ctx) error {
	var body manualAccountDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	acc, err := h.svc.CreateManualAccount(c.Context(), middleware.UserID(c), body.Institution, body.Name, body.Type)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toAccount(*acc))
}

type importLineDTO struct {
	ID          string `json:"id"` // the file's own id (OFX FITID), optional
	Date        string `json:"date" example:"2026-11-10"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"` // cents; negative leaves the user
}

type importDTO struct {
	Transactions []importLineDTO `json:"transactions"`
	Balance      *int64          `json:"balance"` // cents; becomes the account balance when given
}

// @Summary Import statement lines into an imported account; the browser parses the file
// @Tags Connections
// @Accept json
// @Produce json
// @Param id path string true "Account ID"
// @Param body body importDTO true "Up to 2000 lines"
// @Security BearerAuth
// @Success 200 {object} object{inserted=int}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Router /v1/finance/accounts/{id}/import [post]
func (h *Handler) Import(c fiber.Ctx) error {
	var body importDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	txs := make([]domain.Transaction, 0, len(body.Transactions))
	for i, l := range body.Transactions {
		d, err := parseDate("transactions["+strconv.Itoa(i)+"].date", l.Date)
		if err != nil {
			return mapper.WriteError(c, err)
		}
		txs = append(txs, domain.Transaction{ExternalID: l.ID, Date: d, Description: l.Description, Amount: l.Amount})
	}
	n, err := h.svc.Import(c.Context(), middleware.UserID(c), c.Params("id"), txs, body.Balance)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"inserted": n})
}

// ---- goals ----

type goalDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Target    int64  `json:"target"`
	Saved     int64  `json:"saved"`      // typed by the user; ignored when an account is linked
	AccountID string `json:"account_id"` // empty: progress is saved
	DueDate   string `json:"due_date"`   // YYYY-MM-DD or empty
	Current   int64  `json:"current"`
}

func toGoal(g domain.Goal) goalDTO {
	out := goalDTO{ID: g.ID, Name: g.Name, Target: g.Target, Saved: g.Saved, AccountID: g.AccountID, Current: g.Current}
	if g.DueDate != nil {
		out.DueDate = g.DueDate.Format(time.DateOnly)
	}
	return out
}

func fromGoal(c fiber.Ctx) (domain.Goal, error) {
	var body goalDTO
	if err := c.Bind().Body(&body); err != nil {
		return domain.Goal{}, invalidBody()
	}
	g := domain.Goal{Name: body.Name, Target: body.Target, Saved: body.Saved, AccountID: body.AccountID}
	if body.DueDate != "" {
		d, err := parseDate("due_date", body.DueDate)
		if err != nil {
			return g, err
		}
		g.DueDate = &d
	}
	return g, nil
}

// @Summary List savings goals with their progress
// @Tags Goals
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]goalDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/goals [get]
func (h *Handler) Goals(c fiber.Ctx) error {
	list, err := h.svc.Goals(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]goalDTO, 0, len(list))
	for _, g := range list {
		out = append(out, toGoal(g))
	}
	return c.JSON(fiber.Map{"items": out})
}

// @Summary Create a savings goal
// @Tags Goals
// @Accept json
// @Produce json
// @Param body body goalDTO true "Name, target, saved amount or linked account, due date"
// @Security BearerAuth
// @Success 201 {object} goalDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/goals [post]
func (h *Handler) CreateGoal(c fiber.Ctx) error {
	g, err := fromGoal(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out, err := h.svc.CreateGoal(c.Context(), middleware.UserID(c), g)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toGoal(*out))
}

// @Summary Replace a savings goal
// @Tags Goals
// @Accept json
// @Produce json
// @Param id path string true "Goal ID"
// @Param body body goalDTO true "Every field of the goal"
// @Security BearerAuth
// @Success 200 {object} goalDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/goals/{id} [put]
func (h *Handler) UpdateGoal(c fiber.Ctx) error {
	g, err := fromGoal(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out, err := h.svc.UpdateGoal(c.Context(), middleware.UserID(c), c.Params("id"), g)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(toGoal(*out))
}

// @Summary Delete a savings goal
// @Tags Goals
// @Param id path string true "Goal ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/goals/{id} [delete]
func (h *Handler) DeleteGoal(c fiber.Ctx) error {
	if err := h.svc.DeleteGoal(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
