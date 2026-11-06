// Package role manages roles at runtime. Protected roles stay out of reach
// here: giving or taking full access is an operator action (cmd/admin).
package role

import (
	"context"
	"regexp"
	"slices"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/validate"
)

const listCap = 1000

// Same rule as chk_usr_role_name.
var nameFormat = regexp.MustCompile(`^[a-z0-9_-]{2,40}$`)

type Service struct {
	UOW   output.UnitOfWork
	Perms output.PermissionReader
}

var _ input.RoleService = (*Service)(nil)

func New(uow output.UnitOfWork, perms output.PermissionReader) *Service {
	return &Service{UOW: uow, Perms: perms}
}

func (s *Service) List(ctx context.Context) ([]domain.Role, error) {
	return s.UOW.Roles().List(ctx, listCap)
}

func (s *Service) Create(ctx context.Context, name string) (*domain.Role, error) {
	if !nameFormat.MatchString(name) {
		return nil, apperror.New(apperror.CodeInvalidInput, "name must have 2 to 40 of a-z, 0-9, _ and -").WithField("name")
	}
	return s.UOW.Roles().Create(ctx, name)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	role, err := s.editable(ctx, id)
	if err != nil {
		return err
	}
	if role.IsDefault {
		return apperror.New(apperror.CodeConflict, "the default role cannot be deleted") // activation needs it
	}
	return s.UOW.Roles().Delete(ctx, id)
}

// SetPermission only accepts catalog resources, which also refuses "*": that
// sentinel would turn the role into full access.
func (s *Service) SetPermission(ctx context.Context, roleID, resource string, actions domain.Action) (*domain.Role, error) {
	if _, err := s.editable(ctx, roleID); err != nil {
		return nil, err
	}
	resources, err := s.Perms.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	known := slices.ContainsFunc(resources, func(r domain.Resource) bool { return r.Code == resource })
	if !known {
		return nil, apperror.New(apperror.CodeNotFound, "resource not found")
	}
	if actions == 0 {
		err = s.UOW.Roles().RemovePermission(ctx, roleID, resource)
	} else {
		err = s.UOW.Roles().SetPermission(ctx, roleID, resource, actions)
	}
	if err != nil {
		return nil, err
	}
	return s.UOW.Roles().FindByID(ctx, roleID)
}

func (s *Service) Members(ctx context.Context, roleID string, page domain.Page) ([]domain.User, int64, error) {
	if err := validate.ID(roleID); err != nil {
		return nil, 0, err
	}
	if _, err := s.UOW.Roles().FindByID(ctx, roleID); err != nil {
		return nil, 0, err
	}
	return s.UOW.Roles().Members(ctx, roleID, page)
}

func (s *Service) AddMember(ctx context.Context, roleID, userID string) error {
	if _, err := s.editable(ctx, roleID); err != nil {
		return err
	}
	if err := validate.ID(userID); err != nil {
		return err
	}
	if _, err := s.UOW.Users().FindByID(ctx, userID); err != nil {
		return err
	}
	return s.UOW.Roles().Assign(ctx, userID, roleID)
}

func (s *Service) RemoveMember(ctx context.Context, roleID, userID string) error {
	if _, err := s.editable(ctx, roleID); err != nil {
		return err
	}
	if err := validate.ID(userID); err != nil {
		return err
	}
	return s.UOW.Roles().Unassign(ctx, userID, roleID)
}

// editable loads a role the API may change. No transaction: nothing here can
// make a role protected, so the answer cannot go stale before the write.
func (s *Service) editable(ctx context.Context, id string) (*domain.Role, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	role, err := s.UOW.Roles().FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.Protected() {
		return nil, apperror.New(apperror.CodeForbidden, "the admin role cannot be changed")
	}
	return role, nil
}
