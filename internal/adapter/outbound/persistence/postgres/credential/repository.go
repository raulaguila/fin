package credential

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

var _ output.CredentialRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type row struct {
	UserID              string     `gorm:"column:user_id"`
	PasswordHash        *string    `gorm:"column:password_hash"`
	PasswordAlgorithm   *string    `gorm:"column:password_algorithm"`
	FailedLoginAttempts int        `gorm:"column:failed_login_attempts"`
	LockedUntil         *time.Time `gorm:"column:locked_until"`
	LastLoginAt         *time.Time `gorm:"column:last_login_at"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at"`
}

func (m row) toDomain() *domain.Credential {
	return &domain.Credential{
		UserID:              m.UserID,
		PasswordHash:        m.PasswordHash,
		PasswordAlgorithm:   m.PasswordAlgorithm,
		FailedLoginAttempts: m.FailedLoginAttempts,
		LockedUntil:         m.LockedUntil,
		LastLoginAt:         m.LastLoginAt,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}

func (r *Repository) FindByUserID(ctx context.Context, userID string) (*domain.Credential, error) {
	var m row
	err := r.db.WithContext(ctx).Table("usr.usr_credential").Where("user_id = ?", userID).Take(&m).Error
	if err != nil {
		return nil, shared.WrapError(err, "credential")
	}
	return m.toDomain(), nil
}

func (r *Repository) Create(ctx context.Context, userID string) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_credential (user_id) VALUES (?) ON CONFLICT (user_id) DO NOTHING
	`, userID).Error
	return shared.WrapError(err, "credential")
}

func (r *Repository) SetPassword(ctx context.Context, userID, hash, algorithm string) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_credential
		SET password_hash = ?, password_algorithm = ?, failed_login_attempts = 0, locked_until = NULL
		WHERE user_id = ?
	`, hash, algorithm, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "credential")
	}
	if res.RowsAffected == 0 {
		return shared.WrapError(gorm.ErrRecordNotFound, "credential")
	}
	return nil
}

func (r *Repository) RecordFailure(ctx context.Context, userID string, maxAttempts int, lockUntil, now time.Time) (*time.Time, error) {
	var out struct {
		LockedUntil *time.Time `gorm:"column:locked_until"`
	}
	err := r.db.WithContext(ctx).Raw(`
		UPDATE usr.usr_credential SET
			failed_login_attempts = CASE WHEN locked_until <= ? THEN 1 ELSE failed_login_attempts + 1 END,
			locked_until = CASE
				WHEN (CASE WHEN locked_until <= ? THEN 1 ELSE failed_login_attempts + 1 END) >= ? THEN ?::timestamptz
				WHEN locked_until <= ? THEN NULL
				ELSE locked_until
			END
		WHERE user_id = ?
		RETURNING locked_until
	`, now, now, maxAttempts, lockUntil, now, userID).Scan(&out).Error
	if err != nil {
		return nil, shared.WrapError(err, "credential")
	}
	return out.LockedUntil, nil
}

func (r *Repository) RecordSuccess(ctx context.Context, userID, verifiedHash string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_credential
		SET failed_login_attempts = 0, locked_until = NULL, last_login_at = ?
		WHERE user_id = ? AND password_hash = ? AND (locked_until IS NULL OR locked_until <= ?)
	`, now, userID, verifiedHash, now)
	if res.Error != nil {
		return false, shared.WrapError(res.Error, "credential")
	}
	return res.RowsAffected == 1, nil
}
