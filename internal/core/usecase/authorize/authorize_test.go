package authorize

import (
	"context"
	"testing"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

type stubReader struct {
	eff *domain.EffectivePermissions
}

func (s stubReader) LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error) {
	return s.eff, nil
}

func TestAuthorizeSuperuser(t *testing.T) {
	uc := New(stubReader{eff: &domain.EffectivePermissions{
		UserID:      "u1",
		IsSuperuser: true,
		ByResource:  map[string]domain.Action{},
	}}, nil, 0)

	if err := uc.Authorize(context.Background(), "u1", "products", domain.ActionDelete); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizeStar(t *testing.T) {
	uc := New(stubReader{eff: &domain.EffectivePermissions{
		UserID:     "u1",
		ByResource: map[string]domain.Action{domain.ResourceAll: domain.Action(0x7FFFFFFF)},
	}}, nil, 0)

	if err := uc.Authorize(context.Background(), "u1", "users", domain.ActionCreate); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizeGranular(t *testing.T) {
	uc := New(stubReader{eff: &domain.EffectivePermissions{
		UserID:     "u1",
		ByResource: map[string]domain.Action{"products": domain.ActionRead},
	}}, nil, 0)

	if err := uc.Authorize(context.Background(), "u1", "products", domain.ActionRead); err != nil {
		t.Fatal(err)
	}
	err := uc.Authorize(context.Background(), "u1", "products", domain.ActionCreate)
	if err == nil || !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
