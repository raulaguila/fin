package finance

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/fin/internal/core/domain"
)

func (h *Handler) extrasRoutes(r fiber.Router) {
	r.Post("/transactions", h.AddTransaction)
	r.Post("/transactions/categorize", h.CategorizeMany)
	r.Get("/transactions/export", h.ExportTransactions)
	r.Delete("/transactions/:id", h.DeleteTransaction)
	r.Get("/networth", h.NetWorth)
	r.Get("/cards", h.Cards)
	r.Get("/debts", h.Debts)
	r.Post("/debts", h.CreateDebt)
	r.Put("/debts/:id", h.UpdateDebt)
	r.Delete("/debts/:id", h.DeleteDebt)
}

// ---- transactions typed by the user ----

type newTransactionDTO struct {
	AccountID   string   `json:"account_id"` // an imported (manual) account
	Date        string   `json:"date" example:"2026-11-10"`
	Description string   `json:"description"`
	Amount      int64    `json:"amount"`      // cents; negative leaves the user
	CategoryID  string   `json:"category_id"` // empty: the rules choose
	Note        string   `json:"note"`
	Tags        []string `json:"tags"`
}

// @Summary Record a transaction by hand on an imported account; its balance follows
// @Tags Transactions
// @Accept json
// @Produce json
// @Param body body newTransactionDTO true "Transaction"
// @Security BearerAuth
// @Success 201 {object} transactionDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody "the account is kept by a bank"
// @Router /v1/finance/transactions [post]
func (h *Handler) AddTransaction(c fiber.Ctx) error {
	var body newTransactionDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	date, err := parseDate("date", body.Date)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	t, err := h.svc.AddTransaction(c.Context(), middleware.UserID(c), domain.Transaction{AccountID: body.AccountID, Date: date,
		Description: body.Description, Amount: body.Amount, CategoryID: body.CategoryID, Note: body.Note, Tags: body.Tags})
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toTransaction(*t))
}

type categorizeDTO struct {
	IDs        []string `json:"ids"`         // 1 to 200
	CategoryID string   `json:"category_id"` // empty clears it
}

// @Summary Set one category on many transactions; syncs and rules never change it again
// @Tags Transactions
// @Accept json
// @Produce json
// @Param body body categorizeDTO true "Transactions and category"
// @Security BearerAuth
// @Success 200 {object} object{changed=int}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/transactions/categorize [post]
func (h *Handler) CategorizeMany(c fiber.Ctx) error {
	var body categorizeDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	n, err := h.svc.CategorizeMany(c.Context(), middleware.UserID(c), body.IDs, body.CategoryID)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"changed": n})
}

// @Summary Delete a transaction recorded by hand or imported; the account balance gives it back
// @Tags Transactions
// @Param id path string true "Transaction ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody "not found, kept by a bank, or a split part"
// @Router /v1/finance/transactions/{id} [delete]
func (h *Handler) DeleteTransaction(c fiber.Ctx) error {
	if err := h.svc.DeleteTransaction(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Download transactions as CSV, newest first, with the filters of the list
// @Tags Transactions
// @Produce text/csv
// @Param from query string false "First day, YYYY-MM-DD"
// @Param to query string false "Day after the last, YYYY-MM-DD"
// @Param search query string false "Text in the description"
// @Param account_id query string false "Account ID"
// @Param category_id query string false "Category ID, or none"
// @Param tag query string false "Tag"
// @Security BearerAuth
// @Success 200 {string} string "CSV with a header row"
// @Failure 400 {object} mapper.errorBody "bad filter, or more than 10000 rows"
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/transactions/export [get]
func (h *Handler) ExportTransactions(c fiber.Ctx) error {
	f, err := transactionFilter(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	list, err := h.svc.ExportTransactions(c.Context(), middleware.UserID(c), f)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="transacoes-`+time.Now().UTC().Format("20060102")+`.csv"`)
	return c.Send(transactionsCSV(list))
}

func transactionsCSV(list []domain.Transaction) []byte {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF") // BOM: spreadsheets then read the accents as UTF-8
	w := csv.NewWriter(&buf)
	w.Comma = ';' // Brazilian spreadsheets use the comma for decimals
	_ = w.Write([]string{"data", "descricao", "valor", "categoria", "conta", "instituicao", "nota", "tags"})
	for _, t := range list {
		_ = w.Write([]string{t.Date.Format(time.DateOnly), mapper.CSVCell(t.Description), money(t.Amount), mapper.CSVCell(t.CategoryName),
			mapper.CSVCell(t.AccountName), mapper.CSVCell(t.Institution), mapper.CSVCell(t.Note), strings.Join(t.Tags, " ")})
	}
	w.Flush()
	return buf.Bytes()
}

// money writes cents as "-1234,56".
func money(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return sign + strconv.FormatInt(cents/100, 10) + "," + strconv.FormatInt(100+cents%100, 10)[1:]
}

// ---- net worth and cards ----

type netWorthDTO struct {
	Day         string `json:"day" example:"2026-11-10"`
	Banks       int64  `json:"banks"`
	Investments int64  `json:"investments"`
	Cards       int64  `json:"cards"` // what the cards owe, positive
	Total       int64  `json:"total"`
}

// @Summary Net worth by day: today is recorded on every call, earlier days by the hourly job
// @Tags Transactions
// @Produce json
// @Param months query int false "Months back, 1 to 24; default 12"
// @Security BearerAuth
// @Success 200 {object} object{items=[]netWorthDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/networth [get]
func (h *Handler) NetWorth(c fiber.Ctx) error {
	list, err := h.svc.NetWorth(c.Context(), middleware.UserID(c), fiber.Query[int](c, "months"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]netWorthDTO, 0, len(list))
	for _, p := range list {
		out = append(out, netWorthDTO{Day: p.Day.Format(time.DateOnly), Banks: p.Banks, Investments: p.Investments, Cards: p.Cards,
			Total: p.Banks + p.Investments - p.Cards})
	}
	return c.JSON(fiber.Map{"items": out})
}

type cardBillDTO struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Close       string `json:"close" example:"2026-11-05"`
	Due         string `json:"due"` // empty when the bank gave no due day
	Charges     int64  `json:"charges"`
	Credits     int64  `json:"credits"`
	Count       int    `json:"count"`
}

type monthAmountDTO struct {
	Month  string `json:"month" example:"2026-12"`
	Amount int64  `json:"amount"`
}

// @Summary Card bills of the last six months and installments still to come by month
// @Tags Transactions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{bills=[]cardBillDTO,future=[]monthAmountDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/cards [get]
func (h *Handler) Cards(c fiber.Ctx) error {
	cards, err := h.svc.Cards(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	bills := make([]cardBillDTO, 0, len(cards.Bills))
	for _, b := range cards.Bills {
		dto := cardBillDTO{AccountID: b.AccountID, AccountName: b.AccountName, Close: b.Close.Format(time.DateOnly),
			Charges: b.Charges, Credits: b.Credits, Count: b.Count}
		if b.Due != nil {
			dto.Due = b.Due.Format(time.DateOnly)
		}
		bills = append(bills, dto)
	}
	future := make([]monthAmountDTO, 0, len(cards.Future))
	for _, m := range cards.Future {
		future = append(future, monthAmountDTO{Month: m.Month.Format("2006-01"), Amount: m.Amount})
	}
	return c.JSON(fiber.Map{"bills": bills, "future": future})
}

// ---- debts ----

type debtDTO struct {
	ID          string `json:"id"`
	Person      string `json:"person"`
	Amount      int64  `json:"amount"` // cents; positive: the person owes the user
	Description string `json:"description"`
	DueDate     string `json:"due_date"`   // YYYY-MM-DD or empty
	SettledOn   string `json:"settled_on"` // YYYY-MM-DD or empty: still open
}

func toDebt(d domain.Debt) debtDTO {
	out := debtDTO{ID: d.ID, Person: d.Person, Amount: d.Amount, Description: d.Description}
	if d.DueDate != nil {
		out.DueDate = d.DueDate.Format(time.DateOnly)
	}
	if d.SettledOn != nil {
		out.SettledOn = d.SettledOn.Format(time.DateOnly)
	}
	return out
}

func optionalDate(field, s string) (*time.Time, error) {
	d, err := parseDate(field, s)
	if err != nil || d.IsZero() {
		return nil, err
	}
	return &d, nil
}

func fromDebt(c fiber.Ctx) (domain.Debt, error) {
	var body debtDTO
	if err := c.Bind().Body(&body); err != nil {
		return domain.Debt{}, invalidBody()
	}
	d := domain.Debt{Person: body.Person, Amount: body.Amount, Description: body.Description}
	var err error
	if d.DueDate, err = optionalDate("due_date", body.DueDate); err != nil {
		return d, err
	}
	d.SettledOn, err = optionalDate("settled_on", body.SettledOn)
	return d, err
}

// @Summary List money lent and borrowed, open ones first
// @Tags Debts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]debtDTO}
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/debts [get]
func (h *Handler) Debts(c fiber.Ctx) error {
	list, err := h.svc.Debts(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]debtDTO, 0, len(list))
	for _, d := range list {
		out = append(out, toDebt(d))
	}
	return c.JSON(fiber.Map{"items": out})
}

// @Summary Record money lent (positive) or borrowed (negative)
// @Tags Debts
// @Accept json
// @Produce json
// @Param body body debtDTO true "Debt"
// @Security BearerAuth
// @Success 201 {object} debtDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/debts [post]
func (h *Handler) CreateDebt(c fiber.Ctx) error {
	d, err := fromDebt(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out, err := h.svc.CreateDebt(c.Context(), middleware.UserID(c), d)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toDebt(*out))
}

// @Summary Replace a debt; settled_on marks it paid
// @Tags Debts
// @Accept json
// @Produce json
// @Param id path string true "Debt ID"
// @Param body body debtDTO true "Every field of the debt"
// @Security BearerAuth
// @Success 200 {object} debtDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/debts/{id} [put]
func (h *Handler) UpdateDebt(c fiber.Ctx) error {
	d, err := fromDebt(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out, err := h.svc.UpdateDebt(c.Context(), middleware.UserID(c), c.Params("id"), d)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(toDebt(*out))
}

// @Summary Delete a debt
// @Tags Debts
// @Param id path string true "Debt ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/debts/{id} [delete]
func (h *Handler) DeleteDebt(c fiber.Ctx) error {
	if err := h.svc.DeleteDebt(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
