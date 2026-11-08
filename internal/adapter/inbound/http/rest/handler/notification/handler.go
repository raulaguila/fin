package notification

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
)

type Handler struct {
	svc input.AccountService
}

func New(svc input.AccountService) *Handler {
	return &Handler{svc: svc}
}

// Routes expects a router already behind Bearer.
func (h *Handler) Routes(r fiber.Router) {
	r.Get("/", h.List)
	r.Get("/count", h.Count)
	r.Put("/:id/read", h.MarkRead)
}

type notificationDTO struct {
	ID        string     `json:"id"`
	Level     string     `json:"level" example:"warning"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// @Summary List the notifications of the current user, newest first
// @Tags Notifications
// @Produce json
// @Param page query int false "Page" default(1)
// @Param limit query int false "Items per page, up to 100" default(20)
// @Security BearerAuth
// @Success 200 {object} object{items=[]notificationDTO,pagination=mapper.PaginationDTO}
// @Failure 401 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/me/notifications [get]
func (h *Handler) List(c fiber.Ctx) error {
	page := domain.NewPage(fiber.Query[int](c, "page"), fiber.Query[int](c, "limit"))
	items, total, err := h.svc.Notifications(c.Context(), middleware.UserID(c), page)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]notificationDTO, 0, len(items))
	for _, n := range items {
		out = append(out, notificationDTO{
			ID: n.ID, Level: n.Level, Title: n.Title, Message: n.Message, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt,
		})
	}
	return c.JSON(fiber.Map{"items": out, "pagination": mapper.Pagination(page, total)})
}

// @Summary Count unread notifications of the current user
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{unread=int}
// @Failure 401 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/me/notifications/count [get]
func (h *Handler) Count(c fiber.Ctx) error {
	unread, err := h.svc.CountUnread(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(fiber.Map{"unread": unread})
}

// @Summary Mark a notification as read
// @Tags Notifications
// @Produce json
// @Param id path string true "Notification ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/me/notifications/{id}/read [put]
func (h *Handler) MarkRead(c fiber.Ctx) error {
	if err := h.svc.MarkRead(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
