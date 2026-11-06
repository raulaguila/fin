package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// RoleService manages roles at runtime. Protected roles are read-only here.
type RoleService interface {
	List(ctx context.Context) ([]domain.Role, error)
	Create(ctx context.Context, name string) (*domain.Role, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, id string, patch domain.RolePatch) (*domain.Role, error)
	// SetPermissions replaces every permission of a role at once.
	SetPermissions(ctx context.Context, roleID string, permissions map[string]domain.Action) (*domain.Role, error)
	Members(ctx context.Context, roleID string, page domain.Page) ([]domain.User, int64, error)
	AddMember(ctx context.Context, roleID, userID string) error
	RemoveMember(ctx context.Context, roleID, userID string) error
}
