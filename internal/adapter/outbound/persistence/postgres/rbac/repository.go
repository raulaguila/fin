package rbac

import (
	"context"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// Repository reads effective permissions and superuser flag from Postgres.
type Repository struct {
	db *gorm.DB
}

var _ output.PermissionReader = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type effectiveRow struct {
	Resource string `gorm:"column:resource"`
	Actions  int32  `gorm:"column:actions"`
}

func (r *Repository) LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error) {
	var superuser bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM usr.usr_user_role ur
			JOIN usr.usr_role role ON role.id = ur.role_id AND role.enabled = true
			WHERE ur.user_id = ? AND role.is_superuser = true
		)
	`, userID).Scan(&superuser).Error
	if err != nil {
		return nil, shared.WrapError(err, "permissions")
	}

	var rows []effectiveRow
	err = r.db.WithContext(ctx).Raw(`
		SELECT resource, actions
		FROM usr.vw_user_effective_permissions
		WHERE user_id = ?
	`, userID).Scan(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "permissions")
	}

	byResource := make(map[string]domain.Action, len(rows))
	for _, row := range rows {
		byResource[row.Resource] = domain.Action(row.Actions)
	}

	return &domain.EffectivePermissions{
		UserID:      userID,
		IsSuperuser: superuser,
		ByResource:  byResource,
	}, nil
}
