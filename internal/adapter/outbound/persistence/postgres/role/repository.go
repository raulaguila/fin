package role

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
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

func (r *Repository) HasMembers(ctx context.Context, roleID string) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).Raw(`
		SELECT EXISTS (SELECT 1 FROM usr.usr_user_role WHERE role_id = ?)
	`, roleID).Scan(&found).Error
	if err != nil {
		return false, shared.WrapError(err, "user_role")
	}
	return found, nil
}

type roleRow struct {
	ID          string         `gorm:"column:id"`
	Name        string         `gorm:"column:name"`
	Enabled     bool           `gorm:"column:enabled"`
	IsDefault   bool           `gorm:"column:is_default"`
	IsSuperuser bool           `gorm:"column:is_superuser"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	Resource    *string        `gorm:"column:resource"`
	Actions     *domain.Action `gorm:"column:actions"`
}

// load reads the roles of source with their permissions in one round trip.
func (r *Repository) load(ctx context.Context, source string, args ...any) ([]domain.Role, error) {
	var rows []roleRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT r.id::text AS id, r.name, r.enabled, r.is_default, r.is_superuser, r.created_at, p.resource, p.actions
		FROM `+source+` r
		LEFT JOIN usr.usr_role_permission p ON p.role_id = r.id
		ORDER BY r.name, p.resource
	`, args...).Scan(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "role")
	}
	roles := []domain.Role{}
	for _, row := range rows {
		if len(roles) == 0 || roles[len(roles)-1].ID != row.ID {
			roles = append(roles, domain.Role{
				ID: row.ID, Name: row.Name, Enabled: row.Enabled, IsDefault: row.IsDefault,
				IsSuperuser: row.IsSuperuser, CreatedAt: row.CreatedAt, Permissions: map[string]domain.Action{},
			})
		}
		if row.Resource != nil && row.Actions != nil {
			roles[len(roles)-1].Permissions[*row.Resource] = *row.Actions
		}
	}
	return roles, nil
}

func (r *Repository) List(ctx context.Context, limit int) ([]domain.Role, error) {
	return r.load(ctx, `(SELECT * FROM usr.usr_role ORDER BY name LIMIT ?)`, limit)
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Role, error) {
	roles, err := r.load(ctx, `(SELECT * FROM usr.usr_role WHERE id = ?)`, id)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, shared.WrapError(gorm.ErrRecordNotFound, "role")
	}
	return &roles[0], nil
}

func (r *Repository) Create(ctx context.Context, name string) (*domain.Role, error) {
	var row roleRow
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO usr.usr_role (name) VALUES (?)
		RETURNING id::text AS id, name, enabled, is_default, is_superuser, created_at
	`, name).Scan(&row).Error
	if err != nil {
		return nil, shared.WrapError(err, "role")
	}
	return &domain.Role{
		ID: row.ID, Name: row.Name, Enabled: row.Enabled, IsDefault: row.IsDefault,
		IsSuperuser: row.IsSuperuser, CreatedAt: row.CreatedAt, Permissions: map[string]domain.Action{},
	}, nil
}

// Delete fails with RESOURCE_IN_USE while the role has members.
func (r *Repository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM usr.usr_role WHERE id = ?`, id)
	if res.Error != nil {
		return shared.WrapError(res.Error, "role")
	}
	if res.RowsAffected == 0 {
		return shared.WrapError(gorm.ErrRecordNotFound, "role")
	}
	return nil
}

func (r *Repository) SetPermission(ctx context.Context, roleID, resource string, actions domain.Action) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_role_permission (role_id, resource, actions)
		VALUES (?, ?, ?)
		ON CONFLICT (role_id, resource) DO UPDATE SET actions = EXCLUDED.actions
	`, roleID, resource, int32(actions)).Error
	if err != nil {
		return shared.WrapError(err, "role_permission")
	}
	return nil
}

func (r *Repository) RemovePermission(ctx context.Context, roleID, resource string) error {
	err := r.db.WithContext(ctx).Exec(`
		DELETE FROM usr.usr_role_permission WHERE role_id = ? AND resource = ?
	`, roleID, resource).Error
	if err != nil {
		return shared.WrapError(err, "role_permission")
	}
	return nil
}

func (r *Repository) Members(ctx context.Context, roleID string, page domain.Page) ([]domain.User, int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Raw(`
		SELECT count(*) FROM usr.usr_user_role WHERE role_id = ?
	`, roleID).Scan(&total).Error
	if err != nil {
		return nil, 0, shared.WrapError(err, "user_role")
	}
	var users []domain.User
	err = r.db.WithContext(ctx).Raw(`
		SELECT u.id::text AS id, u.name, u.email::text AS email, u.status, u.created_at, u.updated_at
		FROM usr.usr_user u
		JOIN usr.usr_user_role ur ON ur.user_id = u.id
		WHERE ur.role_id = ?
		ORDER BY u.name, u.id
		LIMIT ? OFFSET ?
	`, roleID, page.Limit, page.Offset()).Scan(&users).Error
	if err != nil {
		return nil, 0, shared.WrapError(err, "user_role")
	}
	return users, total, nil
}

func (r *Repository) Unassign(ctx context.Context, userID, roleID string) error {
	err := r.db.WithContext(ctx).Exec(`
		DELETE FROM usr.usr_user_role WHERE user_id = ? AND role_id = ?
	`, userID, roleID).Error
	if err != nil {
		return shared.WrapError(err, "user_role")
	}
	return nil
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
