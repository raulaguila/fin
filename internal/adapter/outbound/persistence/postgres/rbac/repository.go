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

// ListResources returns the catalog. It has few rows, so it has no pagination.
func (r *Repository) ListResources(ctx context.Context) ([]domain.Resource, error) {
	var rows []domain.Resource
	err := r.db.WithContext(ctx).Raw(`
		SELECT code, description FROM usr.usr_resource ORDER BY code
	`).Scan(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "resources")
	}
	return rows, nil
}

type effectiveRow struct {
	Superuser bool    `gorm:"column:superuser"`
	Resource  *string `gorm:"column:resource"`
	Actions   *int32  `gorm:"column:actions"`
}

// LoadEffective runs on every protected request, so it is one round trip: the
// left join keeps one row for a user with no grant, to carry the superuser flag.
func (r *Repository) LoadEffective(ctx context.Context, userID string) (*domain.EffectivePermissions, error) {
	var rows []effectiveRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT flag.superuser, p.resource, p.actions
		FROM (
			SELECT EXISTS (
				SELECT 1
				FROM usr.usr_user_role ur
				JOIN usr.usr_role role ON role.id = ur.role_id AND role.enabled = true
				WHERE ur.user_id = ? AND role.is_superuser = true
			) AS superuser
		) flag
		LEFT JOIN usr.vw_user_effective_permissions p ON p.user_id = ?
	`, userID, userID).Scan(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "permissions")
	}

	eff := &domain.EffectivePermissions{UserID: userID, ByResource: make(map[string]domain.Action, len(rows))}
	for _, row := range rows {
		eff.IsSuperuser = row.Superuser
		if row.Resource != nil && row.Actions != nil {
			eff.ByResource[*row.Resource] = domain.Action(*row.Actions)
		}
	}
	return eff, nil
}
