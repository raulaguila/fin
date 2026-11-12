package finance

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/fin/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/fin/internal/core/domain"
)

type subscriptionDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Amount     int64  `json:"amount"`      // what it costs, positive cents
	Day        int    `json:"day"`         // due day of the month, 1-31
	Month      int    `json:"month"`       // 0 every month; 1-12 once a year in that month
	Keyword    string `json:"keyword"`     // words of the statement line that identify the charge
	CategoryID string `json:"category_id"` // optional
	AccountID  string `json:"account_id"`  // optional; a card account keeps it out of the bank forecast
	LastPaid   string `json:"last_paid"`   // read only: date of the last charge found, or empty
	LastAmount int64  `json:"last_amount"` // read only: that charge, positive cents
	Paid       bool   `json:"paid"`        // read only: charged in the current period
	Late       bool   `json:"late"`        // read only: due a few days ago and not charged
	NextDue    string `json:"next_due"`    // read only: YYYY-MM-DD
}

type subscriptionsDTO struct {
	Items    []subscriptionDTO `json:"items"`
	Detected []recurringDTO    `json:"detected"` // recurring charges neither registered nor dismissed
	Monthly  int64             `json:"monthly"`  // monthly cost of the registered ones, yearly spread over 12
}

func toSubscription(s domain.Subscription) subscriptionDTO {
	out := subscriptionDTO{ID: s.ID, Name: s.Name, Amount: s.Amount, Day: s.Day, Month: s.Month, Keyword: s.Keyword,
		CategoryID: s.CategoryID, AccountID: s.AccountID, LastAmount: s.LastAmount, Paid: s.Paid, Late: s.Late}
	if s.LastPaid != nil {
		out.LastPaid = s.LastPaid.Format(time.DateOnly)
	}
	if !s.NextDue.IsZero() {
		out.NextDue = s.NextDue.Format(time.DateOnly)
	}
	return out
}

func fromSubscription(c fiber.Ctx) (domain.Subscription, error) {
	var body subscriptionDTO
	if err := c.Bind().Body(&body); err != nil {
		return domain.Subscription{}, invalidBody()
	}
	return domain.Subscription{Name: body.Name, Amount: body.Amount, Day: body.Day, Month: body.Month,
		Keyword: body.Keyword, CategoryID: body.CategoryID, AccountID: body.AccountID}, nil
}

// @Summary List registered subscriptions with their status, and recurring charges found in the history
// @Tags Subscriptions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} subscriptionsDTO
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/subscriptions [get]
func (h *Handler) Subscriptions(c fiber.Ctx) error {
	list, err := h.svc.Subscriptions(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := subscriptionsDTO{Items: make([]subscriptionDTO, 0, len(list.Items)), Detected: make([]recurringDTO, 0, len(list.Detected)), Monthly: list.Monthly}
	for _, s := range list.Items {
		out.Items = append(out.Items, toSubscription(s))
	}
	for _, r := range list.Detected {
		out.Detected = append(out.Detected, toRecurring(r))
	}
	return c.JSON(out)
}

// @Summary Register a subscription or fixed bill
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param body body subscriptionDTO true "Name, amount, due day, month for a yearly one, keyword, category and account"
// @Security BearerAuth
// @Success 201 {object} subscriptionDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/subscriptions [post]
func (h *Handler) CreateSubscription(c fiber.Ctx) error {
	s, err := fromSubscription(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out, err := h.svc.CreateSubscription(c.Context(), middleware.UserID(c), s)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toSubscription(*out))
}

// @Summary Replace a subscription or fixed bill
// @Tags Subscriptions
// @Accept json
// @Produce json
// @Param id path string true "Subscription ID"
// @Param body body subscriptionDTO true "Every field of the subscription"
// @Security BearerAuth
// @Success 200 {object} subscriptionDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/subscriptions/{id} [put]
func (h *Handler) UpdateSubscription(c fiber.Ctx) error {
	s, err := fromSubscription(c)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out, err := h.svc.UpdateSubscription(c.Context(), middleware.UserID(c), c.Params("id"), s)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(toSubscription(*out))
}

// @Summary Delete a subscription or fixed bill
// @Tags Subscriptions
// @Param id path string true "Subscription ID"
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Router /v1/finance/subscriptions/{id} [delete]
func (h *Handler) DeleteSubscription(c fiber.Ctx) error {
	if err := h.svc.DeleteSubscription(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Mark a detected charge as not recurring: it leaves the lists and the forecasts
// @Tags Subscriptions
// @Accept json
// @Param body body object{key=string} true "The key of the detected charge"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Router /v1/finance/subscriptions/ignore [post]
func (h *Handler) IgnoreRecurring(c fiber.Ctx) error {
	var body struct {
		Key string `json:"key"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, invalidBody())
	}
	if err := h.svc.IgnoreRecurring(c.Context(), middleware.UserID(c), body.Key); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
