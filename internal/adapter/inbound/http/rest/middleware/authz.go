package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

const (
	LocalUserID    = "auth.user_id"
	LocalSessionID = "auth.session_id"
)

// Bearer authenticates the request's access token.
func Bearer(authn input.Authenticator) fiber.Handler {
	return func(c fiber.Ctx) error {
		const prefix = "Bearer "
		header := c.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if !strings.HasPrefix(header, prefix) || token == "" {
			return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "missing bearer token"))
		}
		principal, err := authn.Authenticate(c.Context(), token)
		if err != nil {
			return mapper.WriteError(c, err)
		}
		c.Locals(LocalUserID, principal.UserID)
		c.Locals(LocalSessionID, principal.SessionID)
		return c.Next()
	}
}

// Require enforces a CRUD action on a resource type (ADR-004). Must run after Bearer.
func Require(authz input.Authorizer, resource string, need domain.Action) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, _ := c.Locals(LocalUserID).(string)
		if userID == "" {
			return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "unauthenticated"))
		}
		if err := authz.Authorize(c.Context(), userID, resource, need); err != nil {
			return mapper.WriteError(c, err)
		}
		return c.Next()
	}
}

// UserID returns the authenticated user id from locals.
func UserID(c fiber.Ctx) string {
	userID, _ := c.Locals(LocalUserID).(string)
	return userID
}

// SessionID returns the authenticated session id from locals.
func SessionID(c fiber.Ctx) string {
	sessionID, _ := c.Locals(LocalSessionID).(string)
	return sessionID
}
