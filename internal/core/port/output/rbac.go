package output

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// PermissionReader loads effective RBAC state for a user (Postgres view + superuser).
type PermissionReader interface {
	LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error)
}
