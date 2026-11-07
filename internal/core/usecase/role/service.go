// Package role manages roles at runtime. Protected roles stay out of reach
// here: giving or taking full access is an operator action (cmd/admin).
package role

import (
	"context"
	"regexp"
	"slices"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/policy"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/validate"
)

const listCap = 1000

// Same rule as chk_usr_role_name.
var nameFormat = regexp.MustCompile(`^[a-z0-9_-]{2,40}$`)

type Service struct {
	UOW   output.IdentityStore
	Tx    output.TxManager
	Perms output.PermissionReader
}

var _ input.RoleService = (*Service)(nil)

func New(uow output.IdentityStore, tx output.TxManager, perms output.PermissionReader) *Service {
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
	var created *domain.Role
	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		var err error
		if created, err = uow.Roles().Create(ctx, name); err != nil {
			return err
		}
		return policy.Audit(ctx, uow, "role.create", created.ID, map[string]any{"name": name})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		role, err := s.editable(ctx, uow, id)
		if err != nil {
			return err
		}
		if role.IsDefault {
			return apperror.New(apperror.CodeConflict, "the default role cannot be deleted") // activation needs it
		}
		if err := uow.Roles().Delete(ctx, role.ID); err != nil {
			return err
		}
		return policy.Audit(ctx, uow, "role.delete", role.ID, map[string]any{"name": role.Name})
	})
}

// Update renames or switches a role on and off. The default role stays
// enabled: activation looks only for an enabled one.
func (s *Service) Update(ctx context.Context, id string, patch domain.RolePatch) (*domain.Role, error) {
	if patch.Name != nil && !nameFormat.MatchString(*patch.Name) {
		return nil, errInvalidName()
	}
	var updated *domain.Role
	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		role, err := s.editable(ctx, uow, id)
		if err != nil {
			return err
		}
		if role.IsDefault && patch.Enabled != nil && !*patch.Enabled {
			return apperror.New(apperror.CodeConflict, "the default role cannot be disabled")
		}
		if err := uow.Roles().Update(ctx, role.ID, patch); err != nil {
			return err
		}
		changes := map[string]any{}
		if patch.Name != nil {
			changes["name"] = *patch.Name
		}
		if patch.Enabled != nil {
			changes["enabled"] = *patch.Enabled
		}
		if err := policy.Audit(ctx, uow, "role.update", role.ID, changes); err != nil {
			return err
		}
		updated, err = uow.Roles().FindByID(ctx, role.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
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
		if err := policy.Audit(ctx, uow, "role.permissions", roleID, map[string]any{"permissions": permissions}); err != nil {
			return err
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
	if _, err := s.editable(ctx, s.UOW, roleID); err != nil {
		return nil, 0, err
	}
	return s.UOW.Roles().Members(ctx, roleID, page)
}

func (s *Service) AddMember(ctx context.Context, roleID, userID string) error {
	return s.changeMember(ctx, roleID, userID, "role.member_add", output.RoleRepository.Assign)
}

func (s *Service) RemoveMember(ctx context.Context, roleID, userID string) error {
	return s.changeMember(ctx, roleID, userID, "role.member_remove", output.RoleRepository.Unassign)
}

func (s *Service) changeMember(ctx context.Context, roleID, userID, action string, change func(output.RoleRepository, context.Context, string, string) error) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		role, err := s.editable(ctx, uow, roleID)
		if err != nil {
			return err
		}
		target, err := policy.Reachable(ctx, uow, userID)
		if err != nil {
			return err
		}
		if err := change(uow.Roles(), ctx, target.ID, role.ID); err != nil {
			return err
		}
		return policy.Audit(ctx, uow, action, role.ID, map[string]any{"user_id": target.ID})
	})
}

// editable loads a role the API may change. No transaction: nothing here can
// make a role protected, so the answer cannot go stale before the write.
func (s *Service) editable(ctx context.Context, uow output.IdentityStore, id string) (*domain.Role, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	role, err := uow.Roles().FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.Protected() {
		return nil, apperror.New(apperror.CodeForbidden, "a superuser role cannot be managed here")
	}
	return role, nil
}
