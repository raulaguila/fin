package middleware_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

type stubAuthz struct {
	deny bool
}

func (s stubAuthz) Authorize(ctx context.Context, userID, resource string, need domain.Action) error {
	if s.deny {
		return apperror.New(apperror.CodeForbidden, "forbidden")
	}
	return nil
}

func (s stubAuthz) LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error) {
	return &domain.EffectivePermissions{UserID: userID, ByResource: map[string]domain.Action{}}, nil
}

func (s stubAuthz) Invalidate(ctx context.Context, userID string) error { return nil }

func withUser(c fiber.Ctx) error {
	c.Locals(middleware.LocalUserID, "u1")
	return c.Next()
}

func TestRequireAllowsAndDenies(t *testing.T) {
	app := fiber.New()
	app.Get("/ok", withUser, middleware.Require(stubAuthz{deny: false}, "products", domain.ActionRead), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/deny", withUser, middleware.Require(stubAuthz{deny: true}, "products", domain.ActionRead), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/ok", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("allow status %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/deny", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("deny status %d body %s", resp.StatusCode, body)
	}
}

type stubTokens struct {
	output.TokenIssuer
	claims *output.AccessClaims
}

func (s stubTokens) ParseAccess(token string) (*output.AccessClaims, error) {
	if token != "good" {
		return nil, errors.New("bad token")
	}
	return s.claims, nil
}

type stubSessions struct {
	output.SessionRepository
	sess *domain.Session
	err  error
}

func (s stubSessions) FindByID(context.Context, string) (*domain.Session, error) {
	return s.sess, s.err
}

func TestBearer(t *testing.T) {
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	claims := &output.AccessClaims{UserID: "u1", SessionID: "s1"}
	live := &domain.Session{UserID: "u1", ExpiresAt: future}

	cases := []struct {
		name   string
		header string
		claims *output.AccessClaims
		sess   *domain.Session
		err    error
		want   int
	}{
		{"valid", "Bearer good", claims, live, nil, fiber.StatusOK},
		{"no header", "", claims, live, nil, fiber.StatusUnauthorized},
		{"wrong scheme", "Basic good", claims, live, nil, fiber.StatusUnauthorized},
		{"empty token", "Bearer  ", claims, live, nil, fiber.StatusUnauthorized},
		{"bad token", "Bearer bad", claims, live, nil, fiber.StatusUnauthorized},
		{"no subject", "Bearer good", &output.AccessClaims{SessionID: "s1"}, live, nil, fiber.StatusUnauthorized},
		{"expired token", "Bearer good", &output.AccessClaims{UserID: "u1", SessionID: "s1", ExpiresAt: past}, live, nil, fiber.StatusUnauthorized},
		{"unknown session", "Bearer good", claims, nil, apperror.New(apperror.CodeNotFound, "x"), fiber.StatusUnauthorized},
		{"database down", "Bearer good", claims, nil, apperror.New(apperror.CodeDatabaseError, "x"), fiber.StatusInternalServerError},
		{"other user's session", "Bearer good", claims, &domain.Session{UserID: "u2", ExpiresAt: future}, nil, fiber.StatusUnauthorized},
		{"revoked session", "Bearer good", claims, &domain.Session{UserID: "u1", ExpiresAt: future, RevokedAt: &past}, nil, fiber.StatusUnauthorized},
		{"expired session", "Bearer good", claims, &domain.Session{UserID: "u1", ExpiresAt: past}, nil, fiber.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", middleware.Bearer(stubTokens{claims: tc.claims}, stubSessions{sess: tc.sess, err: tc.err}), func(c fiber.Ctx) error {
				if middleware.UserID(c) != "u1" {
					return c.SendStatus(fiber.StatusTeapot)
				}
				return c.SendStatus(fiber.StatusOK)
			})
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", tc.header)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.want {
				t.Fatalf("status %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}
