package middleware_test

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
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
