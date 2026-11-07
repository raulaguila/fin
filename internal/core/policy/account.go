package policy

import (
	"context"
	"slices"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/validate"
)

// Reachable loads a user the API may see or change. Nothing in the API can give
// a user a protected role, so the answer cannot go stale before a write.
func Reachable(ctx context.Context, uow output.IdentityStore, id string) (*domain.UserDetails, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	user, err := uow.Users().FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	roles, err := uow.Roles().ForUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(roles, domain.Role.Protected) {
		return nil, apperror.New(apperror.CodeForbidden, "a user with a superuser role cannot be managed here")
	}
	return &domain.UserDetails{User: *user, Roles: roles}, nil
}

// Disable blocks a user and revokes their sessions, so access tokens stop at the
// next request: Bearer checks the session every time.
func Disable(ctx context.Context, uow output.UnitOfWork, id string, now time.Time) error {
	if err := uow.Users().UpdateStatus(ctx, id, domain.UserStatusDisabled); err != nil {
		return err
	}
	return uow.Sessions().RevokeAllForUser(ctx, id, now, domain.RevokeAccountDisabled)
}
