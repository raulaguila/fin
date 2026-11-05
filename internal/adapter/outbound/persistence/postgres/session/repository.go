package session

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

var _ output.SessionRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type row struct {
	ID                       uuid.UUID  `gorm:"column:id"`
	UserID                   uuid.UUID  `gorm:"column:user_id"`
	ExpiresAt                time.Time  `gorm:"column:expires_at"`
	RevokedAt                *time.Time `gorm:"column:revoked_at"`
	RevocationReason         *string    `gorm:"column:revocation_reason"`
	RefreshTokenHash         []byte     `gorm:"column:refresh_token_hash"`
	PreviousRefreshTokenHash []byte     `gorm:"column:previous_refresh_token_hash"`
	IPAddress                *string    `gorm:"column:ip_address"`
	UserAgent                *string    `gorm:"column:user_agent"`
	CreatedAt                time.Time  `gorm:"column:created_at"`
	LastSeenAt               time.Time  `gorm:"column:last_seen_at"`
}

func (m row) toDomain() *domain.Session {
	return &domain.Session{
		ID:                       m.ID.String(),
		UserID:                   m.UserID.String(),
		ExpiresAt:                m.ExpiresAt,
		RevokedAt:                m.RevokedAt,
		RevocationReason:         m.RevocationReason,
		RefreshTokenHash:         m.RefreshTokenHash,
		PreviousRefreshTokenHash: m.PreviousRefreshTokenHash,
		IPAddress:                m.IPAddress,
		UserAgent:                m.UserAgent,
		CreatedAt:                m.CreatedAt,
		LastSeenAt:               m.LastSeenAt,
	}
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	var m row
	err := r.db.WithContext(ctx).Table("usr.usr_session").Where("id = ?", id).Take(&m).Error
	if err != nil {
		return nil, shared.WrapError(err, "session")
	}
	return m.toDomain(), nil
}

func (r *Repository) Create(ctx context.Context, s *domain.Session) error {
	id := uuid.New()
	if s.ID != "" {
		parsed, err := uuid.Parse(s.ID)
		if err != nil {
			return shared.WrapError(err, "session")
		}
		id = parsed
	}
	now := time.Now()
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_session (
			id, user_id, expires_at, revoked_at, revocation_reason,
			refresh_token_hash, previous_refresh_token_hash,
			ip_address, user_agent, created_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?::inet, ?, ?, ?)
	`, id, s.UserID, s.ExpiresAt, s.RevokedAt, s.RevocationReason,
		s.RefreshTokenHash, nilIfEmpty(s.PreviousRefreshTokenHash),
		s.IPAddress, s.UserAgent, now, now).Error
	if err != nil {
		return shared.WrapError(err, "session")
	}
	s.ID = id.String()
	s.CreatedAt = now
	s.LastSeenAt = now
	return nil
}

func (r *Repository) Rotate(ctx context.Context, id string, currentHash, newHash []byte, expiresAt, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_session
		SET previous_refresh_token_hash = refresh_token_hash,
		    refresh_token_hash = ?, expires_at = ?, last_seen_at = ?
		WHERE id = ? AND refresh_token_hash = ? AND revoked_at IS NULL
	`, newHash, expiresAt, now, id, currentHash)
	if res.Error != nil {
		return false, shared.WrapError(res.Error, "session")
	}
	return res.RowsAffected == 1, nil
}

func (r *Repository) Revoke(ctx context.Context, id string, now time.Time, reason string) error {
	err := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_session
		SET revoked_at = ?, revocation_reason = ?, last_seen_at = ?
		WHERE id = ? AND revoked_at IS NULL
	`, now, reason, now, id).Error
	return shared.WrapError(err, "session")
}

func (r *Repository) RevokeAllForUser(ctx context.Context, userID string, now time.Time, reason string) error {
	err := r.db.WithContext(ctx).Exec(`
		UPDATE usr.usr_session
		SET revoked_at = ?, revocation_reason = ?, last_seen_at = ?
		WHERE user_id = ? AND revoked_at IS NULL
	`, now, reason, now, userID).Error
	if err != nil {
		return shared.WrapError(err, "session")
	}
	return nil
}

func nilIfEmpty(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}
