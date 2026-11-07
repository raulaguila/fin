package user

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

type Handler struct {
	svc input.UserService
}

func New(svc input.UserService) *Handler {
	return &Handler{svc: svc}
}

// Routes expects a router already behind Bearer.
func (h *Handler) Routes(r fiber.Router, authz input.Authorizer) {
	read := middleware.Require(authz, domain.ResourceUsers, domain.ActionRead)
	update := middleware.Require(authz, domain.ResourceUsers, domain.ActionUpdate)
	r.Get("/", read, h.List)
	r.Get("/:id", read, h.Get)
	r.Post("/batch-disable", update, h.DisableMany)
	r.Post("/batch-enable", update, h.EnableMany)
	r.Post("/:id/disable", update, h.Disable)
	r.Post("/:id/enable", update, h.Enable)
}

type userDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status" example:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type roleDTO struct {
	ID   string `json:"id"`
	Name string `json:"name" example:"support"`
}

type detailsDTO struct {
	userDTO
	Roles []roleDTO `json:"roles"`
}

func toDTO(u domain.User) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, Email: u.Email, Status: u.Status, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
}

// @Summary List users, without those holding a protected role
// @Tags Users
// @Produce json
// @Param page query int false "Page" default(1)
// @Param limit query int false "Items per page, up to 100" default(20)
// @Param search query string false "Part of the name or email"
// @Param status query string false "Status" Enums(pending, active, locked, disabled)
// @Security BearerAuth
// @Success 200 {object} object{items=[]userDTO,pagination=mapper.PaginationDTO}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/users [get]
func (h *Handler) List(c fiber.Ctx) error {
	filter := domain.UserFilter{
		Page:   domain.NewPage(fiber.Query[int](c, "page"), fiber.Query[int](c, "limit")),
		Search: c.Query("search"),
		Status: c.Query("status"),
	}
	users, total, err := h.svc.List(c.Context(), filter)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toDTO(u))
	}
	return c.JSON(fiber.Map{"items": out, "pagination": mapper.Pagination(filter.Page, total)})
}

// @Summary Get a user with their roles
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Security BearerAuth
// @Success 200 {object} detailsDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/users/{id} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	user, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := detailsDTO{userDTO: toDTO(user.User), Roles: make([]roleDTO, 0, len(user.Roles))}
	for _, r := range user.Roles {
		out.Roles = append(out.Roles, roleDTO{ID: r.ID, Name: r.Name})
	}
	return c.JSON(out)
}

// @Summary Disable a user and end their sessions
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/users/{id}/disable [post]
func (h *Handler) Disable(c fiber.Ctx) error {
	if err := h.svc.Disable(c.Context(), middleware.UserID(c), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type idsDTO struct {
	IDs []string `json:"ids"`
}

func (h *Handler) bindIDs(c fiber.Ctx) ([]string, error) {
	var body idsDTO
	if err := c.Bind().Body(&body); err != nil {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid body")
	}
	return body.IDs, nil
}

// @Summary Disable several users at once and end their sessions; all or none
// @Tags Users
// @Accept json
// @Produce json
// @Param body body idsDTO true "User IDs, 1 to 100; not your own"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/users/batch-disable [post]
func (h *Handler) DisableMany(c fiber.Ctx) error {
	ids, err := h.bindIDs(c)
	if err == nil {
		err = h.svc.DisableMany(c.Context(), middleware.UserID(c), ids)
	}
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Enable several users at once; all or none
// @Tags Users
// @Accept json
// @Produce json
// @Param body body idsDTO true "User IDs, 1 to 100"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/users/batch-enable [post]
func (h *Handler) EnableMany(c fiber.Ctx) error {
	ids, err := h.bindIDs(c)
	if err == nil {
		err = h.svc.EnableMany(c.Context(), ids)
	}
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Enable a disabled user; one who never set a password goes back to pending
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/users/{id}/enable [post]
func (h *Handler) Enable(c fiber.Ctx) error {
	if err := h.svc.Enable(c.Context(), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
