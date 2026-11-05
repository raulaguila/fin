package role

import (
	"context"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

type Repository struct {
	db *gorm.DB
}

var _ output.RoleRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindDefaultRoleID(ctx context.Context) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Raw(`
		SELECT id::text FROM usr.usr_role WHERE is_default = true AND enabled = true LIMIT 1
	`).Scan(&id).Error
	if err != nil {
		return "", shared.WrapError(err, "role")
	}
	if id == "" {
		return "", shared.WrapError(gorm.ErrRecordNotFound, "role")
	}
	return id, nil
}

func (r *Repository) FindIDByName(ctx context.Context, name string) (string, error) {
	var id string
	err := r.db.WithContext(ctx).Raw(`SELECT id::text FROM usr.usr_role WHERE name = ?`, name).Scan(&id).Error
	if err != nil {
		return "", shared.WrapError(err, "role")
	}
	if id == "" {
		return "", shared.WrapError(gorm.ErrRecordNotFound, "role")
	}
	return id, nil
}

func (r *Repository) Assign(ctx context.Context, userID, roleID string) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_user_role (user_id, role_id, created_at)
		VALUES (?, ?, now())
		ON CONFLICT DO NOTHING
	`, userID, roleID).Error
	if err != nil {
		return shared.WrapError(err, "user_role")
	}
	return nil
}
