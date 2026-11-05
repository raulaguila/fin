package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// Authorizer checks whether a user may perform actions on a resource.
type Authorizer interface {
	Authorize(ctx context.Context, userID, resource string, need domain.Action) error
	LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error)
	Invalidate(ctx context.Context, userID string) error
}
