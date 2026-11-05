// Package admin holds operator actions that no HTTP route exposes yet.
package admin

import (
	"context"
	"strings"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

const adminRole = "admin"

// Service implements the commands of cmd/admin.
type Service struct {
	Tx    output.TxManager
	Clock output.Clock
	Authz input.Authorizer
}

func New(tx output.TxManager, clock output.Clock, authz input.Authorizer) *Service {
	return &Service{Tx: tx, Clock: clock, Authz: authz}
}

// Promote gives an active user the admin role.
func (s *Service) Promote(ctx context.Context, email string) error {
	var userID string
	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		user, err := uow.Users().FindByEmail(ctx, normalize(email))
		if err != nil {
			return err
		}
		if user.Status != domain.UserStatusActive {
			return apperror.New(apperror.CodeConflict, "user is "+user.Status+", not active")
		}
		roleID, err := uow.Roles().FindIDByName(ctx, adminRole)
		if err != nil {
			return err
		}
		userID = user.ID
		return uow.Roles().Assign(ctx, user.ID, roleID)
	})
	if err != nil {
		return err
	}
	return s.Authz.Invalidate(ctx, userID)
}

// Disable blocks a user and revokes their sessions, so access tokens stop at
// the next request: Bearer checks the session every time.
func (s *Service) Disable(ctx context.Context, email string) error {
	var userID string
	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		user, err := uow.Users().FindByEmail(ctx, normalize(email))
		if err != nil {
			return err
		}
		if err := uow.Users().UpdateStatus(ctx, user.ID, domain.UserStatusDisabled); err != nil {
			return err
		}
		userID = user.ID
		return uow.Sessions().RevokeAllForUser(ctx, user.ID, s.Clock.Now(), domain.RevokeAccountDisabled)
	})
	if err != nil {
		return err
	}
	return s.Authz.Invalidate(ctx, userID)
}

func normalize(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
