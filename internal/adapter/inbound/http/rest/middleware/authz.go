package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

const (
	LocalUserID      = "auth.user_id"
	LocalSessionID   = "auth.session_id"
	LocalAccessToken = "auth.access_token"
)

// Bearer authenticates an access JWT and optionally rejects revoked sessions.
func Bearer(tokens output.TokenIssuer, sessions output.SessionRepository) fiber.Handler {
	return func(c fiber.Ctx) error {
		raw := c.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(raw, prefix) {
			return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "missing bearer token"))
		}
		token := strings.TrimSpace(strings.TrimPrefix(raw, prefix))
		if token == "" {
			return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "missing bearer token"))
		}

		claims, err := tokens.ParseAccess(token)
		if err != nil {
			return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "invalid access token").WithCause(err))
		}
		if claims.UserID == "" || claims.SessionID == "" {
			return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "invalid access token"))
		}
		if !claims.ExpiresAt.IsZero() && !claims.ExpiresAt.After(time.Now()) {
			return mapper.WriteError(c, apperror.New(apperror.CodeTokenExpired, "access token expired"))
		}

		if sessions != nil {
			sess, err := sessions.FindByID(c.Context(), claims.SessionID)
			if err != nil {
				if apperror.IsNotFound(err) {
					return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "session not found"))
				}
				return mapper.WriteError(c, err)
			}
			if sess.UserID != claims.UserID {
				return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "session mismatch"))
			}
			if sess.RevokedAt != nil {
				return mapper.WriteError(c, apperror.New(apperror.CodeUnauthorized, "session revoked"))
			}
			if !sess.ExpiresAt.After(time.Now()) {
				return mapper.WriteError(c, apperror.New(apperror.CodeTokenExpired, "session expired"))
			}
		}

		c.Locals(LocalUserID, claims.UserID)
		c.Locals(LocalSessionID, claims.SessionID)
		c.Locals(LocalAccessToken, token)
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

// AccessToken returns the raw bearer token from locals.
func AccessToken(c fiber.Ctx) string {
	tok, _ := c.Locals(LocalAccessToken).(string)
	return tok
}
