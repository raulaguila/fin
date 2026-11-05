package attempt

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// notCredentialFailures are refusals that tested no password. Counting them
// would let any repeated request keep a victim blocked forever.
var notCredentialFailures = []string{
	domain.FailureRateLimitedEmail, domain.FailureRateLimitedIP, domain.FailureAccountLocked,
}

type Repository struct {
	db *gorm.DB
}

var _ output.LoginAttemptRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, a *domain.LoginAttempt) error {
	id := uuid.New()
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO usr.usr_login_attempt (id, created_at, email, ip_address, user_agent, success, failure_kind)
		VALUES (?, now(), ?, ?::inet, ?, ?, ?)
	`, id, a.Email, a.IPAddress, a.UserAgent, a.Success, a.FailureKind).Error
	if err != nil {
		return shared.WrapError(err, "login_attempt")
	}
	a.ID = id.String()
	return nil
}

func (r *Repository) CountRecentFailuresByEmail(ctx context.Context, email string, since time.Time) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("usr.usr_login_attempt").
		Where("email = ? AND success = false AND failure_kind NOT IN ? AND created_at >= ?", email, notCredentialFailures, since).
		Count(&n).Error
	if err != nil {
		return 0, shared.WrapError(err, "login_attempt")
	}
	return int(n), nil
}

func (r *Repository) CountRecentFailuresByIP(ctx context.Context, ip string, since time.Time) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("usr.usr_login_attempt").
		Where("ip_address = ?::inet AND success = false AND failure_kind NOT IN ? AND created_at >= ?", ip, notCredentialFailures, since).
		Count(&n).Error
	if err != nil {
		return 0, shared.WrapError(err, "login_attempt")
	}
	return int(n), nil
}
