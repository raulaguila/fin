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
	UOW   output.RoleStore
	Tx    output.TxManager
	Perms output.PermissionReader
}

var _ input.RoleService = (*Service)(nil)

func New(uow output.RoleStore, tx output.TxManager, perms output.PermissionReader) *Service {
	return &Service{UOW: uow, Tx: tx, Perms: perms}
}

func errInvalidName() error {
	return apperror.New(apperror.CodeInvalidInput, "name must have 2 to 40 of a-z, 0-9, _ and -").WithField("name")
}

// List leaves out protected roles: the API cannot change them, so a role form
// has nothing to do with them.
func (s *Service) List(ctx context.Context) ([]domain.Role, error) {
	roles, err := s.UOW.Roles().List(ctx, listCap)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(roles, domain.Role.Protected), nil
}

func (s *Service) Create(ctx context.Context, name string) (*domain.Role, error) {
	if !nameFormat.MatchString(name) {
		return nil, errInvalidName()
	}
	return s.UOW.Roles().Create(ctx, name)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	role, err := s.editable(ctx, s.UOW, id)
	if err != nil {
		return err
	}
	if role.IsDefault {
		return apperror.New(apperror.CodeConflict, "the default role cannot be deleted") // activation needs it
	}
	return s.UOW.Roles().Delete(ctx, id)
}

// Update renames or switches a role on and off. The default role stays
// enabled: activation looks only for an enabled one.
func (s *Service) Update(ctx context.Context, id string, patch domain.RolePatch) (*domain.Role, error) {
	role, err := s.editable(ctx, s.UOW, id)
	if err != nil {
		return nil, err
	}
	if patch.Name != nil && !nameFormat.MatchString(*patch.Name) {
		return nil, errInvalidName()
	}
	if role.IsDefault && patch.Enabled != nil && !*patch.Enabled {
		return nil, apperror.New(apperror.CodeConflict, "the default role cannot be disabled")
	}
	if err := s.UOW.Roles().Update(ctx, id, patch); err != nil {
		return nil, err
	}
	return s.UOW.Roles().FindByID(ctx, id)
}

// SetPermissions replaces every permission of a role in one transaction, so a
// failure leaves the previous ones. Only catalog resources pass, which also
// refuses "*": that sentinel would turn the role into full access. A resource
// with no actions gets no row.
func (s *Service) SetPermissions(ctx context.Context, roleID string, permissions map[string]domain.Action) (*domain.Role, error) {
	resources, err := s.Perms.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	for resource := range permissions {
		known := slices.ContainsFunc(resources, func(r domain.Resource) bool { return r.Code == resource })
		if !known {
			return nil, apperror.New(apperror.CodeInvalidInput, "unknown resource").WithField("permissions")
		}
	}
	var updated *domain.Role
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if err := validate.ID(roleID); err != nil {
			return err
		}
		if err := uow.Lock(ctx, "role:"+roleID); err != nil { // two saves at once would collide on the rows
			return err
		}
		if _, err := s.editable(ctx, uow, roleID); err != nil {
			return err
		}
		if err := uow.Roles().ClearPermissions(ctx, roleID); err != nil {
			return err
		}
		for resource, actions := range permissions {
			if actions == 0 {
				continue
			}
			if err := uow.Roles().Grant(ctx, roleID, resource, actions); err != nil {
				return err
			}
		}
		updated, err = uow.Roles().FindByID(ctx, roleID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
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
	if _, err := s.editable(ctx, s.UOW, roleID); err != nil {
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
	if _, err := s.editable(ctx, s.UOW, roleID); err != nil {
		return err
	}
	if err := validate.ID(userID); err != nil {
		return err
	}
	return s.UOW.Roles().Unassign(ctx, userID, roleID)
}

// editable loads a role the API may change. No transaction: nothing here can
// make a role protected, so the answer cannot go stale before the write.
func (s *Service) editable(ctx context.Context, uow output.RoleStore, id string) (*domain.Role, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	role, err := uow.Roles().FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.Protected() {
		return nil, apperror.New(apperror.CodeForbidden, "a superuser role cannot be changed")
	}
	return role, nil
}
