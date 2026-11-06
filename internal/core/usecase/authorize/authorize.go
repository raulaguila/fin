package authorize

import (
	"context"
	"fmt"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

// UseCase loads effective permissions and checks CRUD bits.
type UseCase struct {
	Reader output.PermissionReader
}

var _ input.Authorizer = (*UseCase)(nil)

func New(reader output.PermissionReader) *UseCase {
	return &UseCase{Reader: reader}
}

func (u *UseCase) LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error) {
	if userID == "" {
		return nil, apperror.New(apperror.CodeInvalidInput, "user id required")
	}

	if u.Reader == nil {
		return nil, apperror.New(apperror.CodeInternal, "permission reader not configured")
	}

	return u.Reader.LoadEffective(ctx, userID)
}

func (u *UseCase) Authorize(ctx context.Context, userID, resource string, need domain.Action) error {
	if resource == "" {
		return apperror.New(apperror.CodeInvalidInput, "resource required")
	}
	if need == 0 {
		return apperror.New(apperror.CodeInvalidInput, "action required")
	}

	eff, err := u.LoadEffective(ctx, userID)
	if err != nil {
		return err
	}
	if eff == nil {
		return apperror.New(apperror.CodeForbidden, "forbidden")
	}
	if eff.IsSuperuser {
		return nil
	}
	if _, ok := eff.ByResource[domain.ResourceAll]; ok {
		return nil
	}
	if have, ok := eff.ByResource[resource]; ok && domain.Allows(have, need) {
		return nil
	}
	return apperror.New(apperror.CodeForbidden, fmt.Sprintf("missing %s permission on %s", needLabel(need), resource))
}

func needLabel(need domain.Action) string {
	switch {
	case domain.Allows(need, domain.ActionCreate):
		return "create"
	case domain.Allows(need, domain.ActionRead):
		return "read"
	case domain.Allows(need, domain.ActionUpdate):
		return "update"
	case domain.Allows(need, domain.ActionDelete):
		return "delete"
	default:
		return "action"
	}
}
