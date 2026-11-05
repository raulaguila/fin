package user

import (
	"context"
	"errors"
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

func (r row) toDomain() *domain.User {
	return &domain.User{
		ID:        r.ID.String(),
		Name:      r.Name,
		Email:     r.Email,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var m row
	err := r.db.WithContext(ctx).Table("usr.usr_user").Where("id = ?", id).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.WrapError(err, "user")
	}
	if err != nil {
		return nil, shared.WrapError(err, "user")
	}
	return m.toDomain(), nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var m row
	err := r.db.WithContext(ctx).Table("usr.usr_user").Where("email = ?", email).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.WrapError(err, "user")
	}
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

func (r *Repository) UpdateStatus(ctx context.Context, id, status string) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_user SET status = ?, updated_at = now() WHERE id = ?
	`, status, id)
	if res.Error != nil {
		return shared.WrapError(res.Error, "user")
	}
	if res.RowsAffected == 0 {
		return shared.WrapError(gorm.ErrRecordNotFound, "user")
	}
	return nil
}
