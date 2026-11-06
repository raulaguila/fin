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
	// SetPermission replaces the actions of a role on a resource; zero removes them.
	SetPermission(ctx context.Context, roleID, resource string, actions domain.Action) (*domain.Role, error)
	Members(ctx context.Context, roleID string, page domain.Page) ([]domain.User, int64, error)
	AddMember(ctx context.Context, roleID, userID string) error
	RemoveMember(ctx context.Context, roleID, userID string) error
}
