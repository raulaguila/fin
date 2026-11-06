package permission_test

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/permission"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

type stubAuthz struct{ allow string }

func (s stubAuthz) Authorize(_ context.Context, _, resource string, need domain.Action) error {
	if resource == s.allow && need == domain.ActionRead {
		return nil
	}
	return apperror.New(apperror.CodeForbidden, "forbidden")
}

func (s stubAuthz) LoadEffective(context.Context, string) (*domain.EffectivePermissions, error) {
	return nil, nil
}

type stubCatalog struct{}

func (stubCatalog) ListResources(context.Context) ([]domain.Resource, error) {
	return []domain.Resource{{Code: "products", Description: "Example"}}, nil
}

func TestResourcesNeedPermissionsRead(t *testing.T) {
	for _, tc := range []struct {
		allow  string
		status int
	}{
		{domain.ResourcePermissions, fiber.StatusOK},
		{domain.ResourceProducts, fiber.StatusForbidden},
	} {
		app := fiber.New()
		group := app.Group("/permissions", func(c fiber.Ctx) error {
			c.Locals(middleware.LocalUserID, "u1")
			return c.Next()
		})
		permission.New(stubCatalog{}).Routes(group, stubAuthz{allow: tc.allow})

		resp, err := app.Test(httptest.NewRequest("GET", "/permissions/resources", nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != tc.status {
			t.Fatalf("allow %s: %d %s", tc.allow, resp.StatusCode, body)
		}
		if tc.status == fiber.StatusOK && !strings.Contains(string(body), `"code":"products"`) {
			t.Fatalf("body %s", body)
		}
	}
}
