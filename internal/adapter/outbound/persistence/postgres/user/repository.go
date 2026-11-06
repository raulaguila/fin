package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

type Repository struct {
	db *gorm.DB
}

var _ output.UserRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type row struct {
	ID        uuid.UUID `gorm:"column:id"`
	Name      string    `gorm:"column:name"`
	Email     string    `gorm:"column:email"`
	Status    string    `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (m row) toDomain() *domain.User {
	return &domain.User{
		ID:        m.ID.String(),
		Name:      m.Name,
		Email:     m.Email,
		Status:    m.Status,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var m row
	err := r.db.WithContext(ctx).Table("usr.usr_user").Where("id = ?", id).Take(&m).Error
	if err != nil {
		return nil, shared.WrapError(err, "user")
	}
	return m.toDomain(), nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var m row
	err := r.db.WithContext(ctx).Table("usr.usr_user").Where("email = ?", email).Take(&m).Error
	if err != nil {
		return nil, shared.WrapError(err, "user")
	}
	return m.toDomain(), nil
}

func (r *Repository) Create(ctx context.Context, u *domain.User) error {
	id := uuid.New()
	if u.ID != "" {
		parsed, err := uuid.Parse(u.ID)
		if err != nil {
			return shared.WrapError(err, "user")
		}
		id = parsed
	}
	now := time.Now()
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_user (id, name, email, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, u.Name, u.Email, u.Status, now, now).Error
	if err != nil {
		return shared.WrapError(err, "user")
	}
	u.ID = id.String()
	u.CreatedAt = now
	u.UpdatedAt = now
	return nil
}

// unprotected must agree with domain.Role.Protected.
const unprotected = `NOT EXISTS (
	SELECT 1 FROM usr.usr_user_role ur
	JOIN usr.usr_role r ON r.id = ur.role_id
	WHERE ur.user_id = u.id AND (
		r.is_superuser OR EXISTS (SELECT 1 FROM usr.usr_role_permission p WHERE p.role_id = r.id AND p.resource = '*')
	)
)`

// shortcut: OFFSET and leading-wildcard ILIKE scan what they skip, fine for thousands of users; move to keyset and pg_trgm beyond that.
func (r *Repository) List(ctx context.Context, filter domain.UserFilter) ([]domain.User, int64, error) {
	matching := func() *gorm.DB {
		q := r.db.WithContext(ctx).Table("usr.usr_user u").Where(unprotected)
		if filter.Search != "" {
			pattern := shared.Contains(filter.Search)
			q = q.Where("(u.name ILIKE ? OR u.email::text ILIKE ?)", pattern, pattern)
		}
		if filter.Status != "" {
			q = q.Where("u.status = ?", filter.Status)
		}
		return q
	}
	var total int64
	if err := matching().Count(&total).Error; err != nil {
		return nil, 0, shared.WrapError(err, "user")
	}
	var rows []row
	err := matching().
		Select("u.id, u.name, u.email, u.status, u.created_at, u.updated_at").
		Order("u.name, u.id"). // id keeps pages stable on ties
		Limit(filter.Page.Limit).Offset(filter.Page.Offset()).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, shared.WrapError(err, "user")
	}
	out := make([]domain.User, 0, len(rows))
	for _, m := range rows {
		out = append(out, *m.toDomain())
	}
	return out, total, nil
}

func (r *Repository) Enable(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_user u
		SET status = CASE
		      WHEN EXISTS (SELECT 1 FROM usr.usr_credential c WHERE c.user_id = u.id AND c.password_hash IS NOT NULL)
		      THEN ? ELSE ? END,
		    updated_at = now()
		WHERE u.id = ? AND u.status = ?
	`, domain.UserStatusActive, domain.UserStatusPending, id, domain.UserStatusDisabled).Error
	return shared.WrapError(err, "user")
}

func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	err := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_user SET status = ?, updated_at = now() WHERE id = ? AND status <> ?
	`, status, id, domain.UserStatusDisabled).Error
	return shared.WrapError(err, "user")
}
