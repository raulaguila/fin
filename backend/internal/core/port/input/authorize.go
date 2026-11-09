package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// Authorizer checks that the principal's session is live and that the user may
// perform actions on a resource, in one round trip.
type Authorizer interface {
	Authorize(ctx context.Context, p domain.Principal, resource string, need domain.Action) error
	LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error)
}

// ResourceCatalog lists the resources that grants may name.
type ResourceCatalog interface {
	ListResources(ctx context.Context) ([]domain.Resource, error)
}
