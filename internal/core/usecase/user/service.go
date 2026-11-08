// Package user manages other people's accounts at runtime. Users with a protected
// role stay out of reach here: they change only through cmd/admin.
package user

import (
	"context"
	"slices"
	"strings"

	"github.com/raulaguila/authenticity/internal/core/policy"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/validate"
)

const (
	maxSearchBytes = 120
	maxBatch       = 100 // ids in one batch request; a page holds at most 100 rows
)

var sortable = []string{"name", "email", "status", "created_at"}

var statuses = []string{domain.UserStatusPending, domain.UserStatusActive, domain.UserStatusLocked, domain.UserStatusDisabled}

type Service struct {
	UOW   output.IdentityStore
	Tx    output.TxManager
	Clock output.Clock
}

var _ input.UserService = (*Service)(nil)

func New(uow output.IdentityStore, tx output.TxManager, clock output.Clock) *Service {
	return &Service{UOW: uow, Tx: tx, Clock: clock}
}

func (s *Service) List(ctx context.Context, filter domain.UserFilter) ([]domain.User, int64, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if err := validate.Search(filter.Search, maxSearchBytes); err != nil {
		return nil, 0, err
	}
	if filter.Status != "" && !slices.Contains(statuses, filter.Status) {
		return nil, 0, apperror.New(apperror.CodeInvalidInput, "status must be one of "+strings.Join(statuses, ", ")).WithField("status")
	}
	if filter.RoleID != "" {
		if err := validate.ID(filter.RoleID); err != nil {
			return nil, 0, err
		}
	}
	if err := validate.Sort(&filter.Sort, &filter.Order, sortable, "name", "asc"); err != nil {
		return nil, 0, err
	}
	return s.UOW.Users().List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.UserDetails, error) {
	return policy.Reachable(ctx, s.UOW, id)
}

// DisableMany refuses the caller's own account, compared on the stored id: the
// path may spell the same UUID in another case.
func (s *Service) DisableMany(ctx context.Context, actorID string, ids []string) error {
	return s.each(ctx, ids, func(ctx context.Context, uow output.UnitOfWork, target *domain.UserDetails) error {
		if target.ID == actorID {
			return apperror.New(apperror.CodeForbidden, "your own account cannot be disabled")
		}
		if err := policy.Disable(ctx, uow, target.ID, s.Clock.Now()); err != nil {
			return err
		}
		return policy.Audit(ctx, uow, "user.disable", target.ID, nil)
	})
}

// RevokeSessions ends every session of another user; they must sign in again.
func (s *Service) RevokeSessions(ctx context.Context, actorID, id string) error {
	return s.each(ctx, []string{id}, func(ctx context.Context, uow output.UnitOfWork, target *domain.UserDetails) error {
		if target.ID == actorID {
			return apperror.New(apperror.CodeForbidden, "end your own sessions from your profile")
		}
		if err := uow.Sessions().RevokeAllForUser(ctx, target.ID, s.Clock.Now(), domain.RevokeAdmin); err != nil {
			return err
		}
		return policy.Audit(ctx, uow, "user.sessions_revoke", target.ID, nil)
	})
}

func (s *Service) EnableMany(ctx context.Context, ids []string) error {
	return s.each(ctx, ids, func(ctx context.Context, uow output.UnitOfWork, target *domain.UserDetails) error {
		if err := uow.Users().Enable(ctx, target.ID); err != nil {
			return err
		}
		return policy.Audit(ctx, uow, "user.enable", target.ID, nil)
	})
}

// each runs change on every reachable user in one transaction, so one refusal undoes all.
func (s *Service) each(ctx context.Context, ids []string, change func(context.Context, output.UnitOfWork, *domain.UserDetails) error) error {
	if len(ids) == 0 || len(ids) > maxBatch {
		return apperror.New(apperror.CodeInvalidInput, "ids must hold 1 to 100 ids").WithField("ids")
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		for _, id := range slices.Compact(slices.Sorted(slices.Values(ids))) {
			target, err := policy.Reachable(ctx, uow, id)
			if err != nil {
				return err
			}
			if err := change(ctx, uow, target); err != nil {
				return err
			}
		}
		return nil
	})
}
