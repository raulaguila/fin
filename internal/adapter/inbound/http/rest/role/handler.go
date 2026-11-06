package role

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
	svc input.RoleService
}

func NewHandler(svc input.RoleService) *Handler {
	return &Handler{svc: svc}
}

// Routes expects a router already behind Bearer.
func (h *Handler) Routes(r fiber.Router, authz input.Authorizer) {
	read := middleware.Require(authz, domain.ResourceRoles, domain.ActionRead)
	update := middleware.Require(authz, domain.ResourceRoles, domain.ActionUpdate)
	r.Get("/", read, h.List)
	r.Post("/", middleware.Require(authz, domain.ResourceRoles, domain.ActionCreate), h.Create)
	r.Delete("/:id", middleware.Require(authz, domain.ResourceRoles, domain.ActionDelete), h.Delete)
	r.Put("/:id/permissions/:resource", update, h.SetPermission)
	r.Get("/:id/members", read, h.Members)
	r.Put("/:id/members/:user_id", update, h.AddMember)
	r.Delete("/:id/members/:user_id", update, h.RemoveMember)
}

type roleDTO struct {
	ID          string              `json:"id"`
	Name        string              `json:"name" example:"support"`
	Enabled     bool                `json:"enabled"`
	IsDefault   bool                `json:"is_default"`
	IsSuperuser bool                `json:"is_superuser"`
	Protected   bool                `json:"protected"`
	Permissions map[string][]string `json:"permissions"`
	CreatedAt   time.Time           `json:"created_at"`
}

type createDTO struct {
	Name string `json:"name" example:"support"`
}

type permissionDTO struct {
	Actions []string `json:"actions" example:"read,update"`
}

type memberDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status" example:"active"`
}

func toDTO(r domain.Role) roleDTO {
	permissions := make(map[string][]string, len(r.Permissions))
	for resource, actions := range r.Permissions {
		permissions[resource] = domain.ActionNames(actions)
	}
	return roleDTO{
		ID: r.ID, Name: r.Name, Enabled: r.Enabled, IsDefault: r.IsDefault, IsSuperuser: r.IsSuperuser,
		Protected: r.Protected(), Permissions: permissions, CreatedAt: r.CreatedAt,
	}
}

// @Summary List roles with their permissions
// @Tags Roles
// @Produce json
// @Security BearerAuth
// @Success 200 {object} object{items=[]roleDTO}
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles [get]
func (h *Handler) List(c fiber.Ctx) error {
	roles, err := h.svc.List(c.Context())
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]roleDTO, 0, len(roles))
	for _, r := range roles {
		out = append(out, toDTO(r))
	}
	return c.JSON(fiber.Map{"items": out})
}

// @Summary Create a role
// @Tags Roles
// @Produce json
// @Accept json
// @Param body body createDTO true "Request"
// @Security BearerAuth
// @Success 201 {object} roleDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles [post]
func (h *Handler) Create(c fiber.Ctx) error {
	var body createDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	role, err := h.svc.Create(c.Context(), body.Name)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toDTO(*role))
}

// @Summary Delete a role without members
// @Tags Roles
// @Produce json
// @Param id path string true "Role ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 409 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles/{id} [delete]
func (h *Handler) Delete(c fiber.Ctx) error {
	if err := h.svc.Delete(c.Context(), c.Params("id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Replace the actions of a role on a resource; an empty list removes them
// @Tags Roles
// @Produce json
// @Accept json
// @Param id path string true "Role ID"
// @Param resource path string true "Catalog resource"
// @Param body body permissionDTO true "Request"
// @Security BearerAuth
// @Success 200 {object} roleDTO
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles/{id}/permissions/{resource} [put]
func (h *Handler) SetPermission(c fiber.Ctx) error {
	var body permissionDTO
	if err := c.Bind().Body(&body); err != nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "invalid body"))
	}
	if body.Actions == nil {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "actions required").WithField("actions"))
	}
	actions, ok := domain.ParseActions(body.Actions)
	if !ok {
		return mapper.WriteError(c, apperror.New(apperror.CodeInvalidInput, "actions must be create, read, update or delete").WithField("actions"))
	}
	role, err := h.svc.SetPermission(c.Context(), c.Params("id"), c.Params("resource"), actions)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	return c.JSON(toDTO(*role))
}

// @Summary List the members of a role
// @Tags Roles
// @Produce json
// @Param id path string true "Role ID"
// @Param page query int false "Page" default(1)
// @Param limit query int false "Items per page, up to 100" default(20)
// @Security BearerAuth
// @Success 200 {object} object{items=[]memberDTO,pagination=mapper.PaginationDTO}
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles/{id}/members [get]
func (h *Handler) Members(c fiber.Ctx) error {
	page := domain.NewPage(fiber.Query[int](c, "page"), fiber.Query[int](c, "limit"))
	users, total, err := h.svc.Members(c.Context(), c.Params("id"), page)
	if err != nil {
		return mapper.WriteError(c, err)
	}
	out := make([]memberDTO, 0, len(users))
	for _, u := range users {
		out = append(out, memberDTO{ID: u.ID, Name: u.Name, Email: u.Email, Status: u.Status})
	}
	return c.JSON(fiber.Map{"items": out, "pagination": mapper.Pagination(page, total)})
}

// @Summary Add a user to a role
// @Tags Roles
// @Produce json
// @Param id path string true "Role ID"
// @Param user_id path string true "User ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles/{id}/members/{user_id} [put]
func (h *Handler) AddMember(c fiber.Ctx) error {
	if err := h.svc.AddMember(c.Context(), c.Params("id"), c.Params("user_id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Remove a user from a role
// @Tags Roles
// @Produce json
// @Param id path string true "Role ID"
// @Param user_id path string true "User ID"
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} mapper.errorBody
// @Failure 401 {object} mapper.errorBody
// @Failure 403 {object} mapper.errorBody
// @Failure 404 {object} mapper.errorBody
// @Failure 429 {object} mapper.errorBody
// @Router /v1/roles/{id}/members/{user_id} [delete]
func (h *Handler) RemoveMember(c fiber.Ctx) error {
	if err := h.svc.RemoveMember(c.Context(), c.Params("id"), c.Params("user_id")); err != nil {
		return mapper.WriteError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
