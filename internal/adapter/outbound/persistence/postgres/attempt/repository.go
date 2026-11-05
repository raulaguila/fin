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

// countedFailure excludes refusals that tested no password: counting them would
// let any repeated request keep a victim blocked forever. Literal on purpose: it
// must match the partial indexes of migration 000007 for the planner to use them.
const countedFailure = `success = false AND failure_kind NOT IN ('rate_limited_email', 'rate_limited_ip', 'account_locked')`

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
		Where("email = ? AND created_at >= ? AND "+countedFailure, email, since).
		Count(&n).Error
	if err != nil {
		return 0, shared.WrapError(err, "login_attempt")
	}
	return int(n), nil
}

func (r *Repository) CountRecentFailuresByIP(ctx context.Context, ip string, since time.Time) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("usr.usr_login_attempt").
		Where("ip_address = ?::inet AND created_at >= ? AND "+countedFailure, ip, since).
		Count(&n).Error
	if err != nil {
		return 0, shared.WrapError(err, "login_attempt")
	}
	return int(n), nil
}
