package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// UserService manages the accounts of other people. Users with a protected role
// stay out of reach here, as protected roles do.
type UserService interface {
	List(ctx context.Context, filter domain.UserFilter) ([]domain.User, int64, error)
	Get(ctx context.Context, id string) (*domain.UserDetails, error)
	// Disable blocks a user and ends their sessions. actorID is who asks.
	Disable(ctx context.Context, actorID, id string) error
	Enable(ctx context.Context, id string) error
	// DisableMany and EnableMany change every listed user or none.
	DisableMany(ctx context.Context, actorID string, ids []string) error
	EnableMany(ctx context.Context, ids []string) error
}
