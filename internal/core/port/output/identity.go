package output

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// UserRepository persists usr.usr_user.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	UpdateStatus(ctx context.Context, id, status string) error
}

// CredentialRepository persists usr.usr_credential.
type CredentialRepository interface {
	FindByUserID(ctx context.Context, userID string) (*domain.Credential, error)
	// Create inserts the empty credential of a new user.
	Create(ctx context.Context, userID string) error
	// SetPassword stores a new hash and clears the lockout.
	SetPassword(ctx context.Context, userID, hash, algorithm string) error
	// RecordFailure atomically counts a bad password, restarting the count after
	// a lapsed lock, and locks until lockUntil once maxAttempts is reached.
	RecordFailure(ctx context.Context, userID string, maxAttempts int, lockUntil, now time.Time) (lockedUntil *time.Time, err error)
	// RecordSuccess clears the counters only if the verified hash is still current
	// and no lock started meanwhile. False means the login must be refused.
	RecordSuccess(ctx context.Context, userID, verifiedHash string, now time.Time) (bool, error)
}

// SessionRepository persists usr.usr_session.
type SessionRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Session, error)
	Create(ctx context.Context, session *domain.Session) error
	// Rotate swaps the refresh hash only if currentHash is still the live one.
	// False means another request already rotated or revoked the session.
	Rotate(ctx context.Context, id string, currentHash, newHash []byte, expiresAt, now time.Time) (bool, error)
	// Revoke is idempotent: an unknown or already revoked session is not an error.
	Revoke(ctx context.Context, id string, now time.Time, reason string) error
	RevokeAllForUser(ctx context.Context, userID string, now time.Time, reason string) error
}

// OutboxRepository persists sys.outbox.
type OutboxRepository interface {
	Enqueue(ctx context.Context, purpose string, payload []byte) error
	// ClaimPending leases due rows until now+lease; a crashed worker's rows come back after that.
	ClaimPending(ctx context.Context, limit int, now time.Time, lease time.Duration) ([]domain.OutboxMessage, error)
	MarkSent(ctx context.Context, id string, processedAt time.Time) error
	MarkFailed(ctx context.Context, id string, attempts int, availableAt time.Time, processedAt *time.Time, lastError string, terminal bool) error
}

// ProductRepository persists app.product.
type ProductRepository interface {
	List(ctx context.Context, limit int) ([]domain.Product, error)
	FindByID(ctx context.Context, id string) (*domain.Product, error)
	Create(ctx context.Context, p *domain.Product) error
	Update(ctx context.Context, p *domain.Product) error
	Delete(ctx context.Context, id string) error
}
