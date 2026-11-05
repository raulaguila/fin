package me

import (
	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
)

type Handler struct {
	authz input.Authorizer
}

func NewHandler(authz input.Authorizer) *Handler {
	return &Handler{authz: authz}
}

// Get returns the authenticated principal.
func (h *Handler) Get(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"user_id":    middleware.UserID(c),
		"session_id": c.Locals(middleware.LocalSessionID),
	})
}

// Permissions returns effective permissions (requires permissions:read).
func (h *Handler) Permissions(c fiber.Ctx) error {
	eff, err := h.authz.LoadEffective(c.Context(), middleware.UserID(c))
	if err != nil {
		return mapper.WriteError(c, err)
	}
	byResource := map[string][]string{}
	if eff != nil {
		for res, actions := range eff.ByResource {
			byResource[res] = actionNames(actions)
		}
	}
	return c.JSON(fiber.Map{
		"user_id":      middleware.UserID(c),
		"is_superuser": eff != nil && eff.IsSuperuser,
		"permissions":  byResource,
	})
}

func actionNames(a domain.Action) []string {
	var out []string
	if domain.Allows(a, domain.ActionCreate) {
		out = append(out, "create")
	}
	if domain.Allows(a, domain.ActionRead) {
		out = append(out, "read")
	}
	if domain.Allows(a, domain.ActionUpdate) {
		out = append(out, "update")
	}
	if domain.Allows(a, domain.ActionDelete) {
		out = append(out, "delete")
	}
	return out
}
