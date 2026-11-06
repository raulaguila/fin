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

const maxSearchBytes = 120

var statuses = []string{domain.UserStatusPending, domain.UserStatusActive, domain.UserStatusLocked, domain.UserStatusDisabled}

type Service struct {
	UOW   output.UserStore
	Tx    output.TxManager
	Clock output.Clock
}

var _ input.UserService = (*Service)(nil)

func New(uow output.UserStore, tx output.TxManager, clock output.Clock) *Service {
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
	return s.UOW.Users().List(ctx, filter)
}

func (s *Service) Get(ctx context.Context, id string) (*domain.UserDetails, error) {
	return reachable(ctx, s.UOW, id)
}

// Disable refuses the caller's own account, compared on the stored id: the path
// may spell the same UUID in another case.
func (s *Service) Disable(ctx context.Context, actorID, id string) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		target, err := reachable(ctx, uow, id)
		if err != nil {
			return err
		}
		if target.ID == actorID {
			return apperror.New(apperror.CodeForbidden, "your own account cannot be disabled")
		}
		return policy.Disable(ctx, uow, target.ID, s.Clock.Now())
	})
}

func (s *Service) Enable(ctx context.Context, id string) error {
	target, err := reachable(ctx, s.UOW, id)
	if err != nil {
		return err
	}
	return s.UOW.Users().Enable(ctx, target.ID)
}

// reachable loads a user the API may see. Nothing here can give a user a protected
// role, so the answer cannot go stale before a write.
func reachable(ctx context.Context, uow output.UserStore, id string) (*domain.UserDetails, error) {
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
