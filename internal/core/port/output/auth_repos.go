package output

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// TokenRepository persists usr.usr_token.
type TokenRepository interface {
	Create(ctx context.Context, token *domain.EmailToken) error
	FindActiveByHash(ctx context.Context, purpose string, hash []byte) (*domain.EmailToken, error)
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error
	IncrementAttempts(ctx context.Context, id string) (attempts int, err error)
	InvalidateActive(ctx context.Context, userID, purpose string) error
	// IssuedSince reports whether a live token of this purpose was created at or after since.
	IssuedSince(ctx context.Context, userID, purpose string, since time.Time) (bool, error)
	// CountIssuedSince counts tokens of this purpose created at or after since, spent ones too.
	CountIssuedSince(ctx context.Context, userID, purpose string, since time.Time) (int, error)
}

// LoginAttemptRepository appends login audits.
type LoginAttemptRepository interface {
	Create(ctx context.Context, attempt *domain.LoginAttempt) error
	CountRecentFailuresByEmail(ctx context.Context, email string, since time.Time) (int, error)
	// CountRecentFailuresByIP counts the whole client: an IPv6 address counts its /64.
	CountRecentFailuresByIP(ctx context.Context, ip string, since time.Time) (int, error)
	// CountRecentFailuresByEmailAndIP counts one client's failures against one account.
	CountRecentFailuresByEmailAndIP(ctx context.Context, email, ip string, since time.Time) (int, error)
	// Lock makes concurrent logins sharing a key wait until the transaction ends.
	Lock(ctx context.Context, key string) error
	// Settle replaces the outcome of a reserved attempt.
	Settle(ctx context.Context, id string, success bool, failureKind *string) error
	// Release drops a reserved attempt that tested no password.
	Release(ctx context.Context, id string) error
}

// RateCounter counts hits per key in a fixed window shared by all processes.
type RateCounter interface {
	// Hit adds one and returns the window's total and the time until it resets.
	Hit(ctx context.Context, key string, window time.Duration) (count int64, reset time.Duration, err error)
}

// RoleRepository resolves roles and assigns them to users.
type RoleRepository interface {
	FindDefaultRoleID(ctx context.Context) (string, error)
	FindIDByName(ctx context.Context, name string) (string, error)
	Assign(ctx context.Context, userID, roleID string) error
}

// Tx runs fn inside a database transaction. Repositories obtained from the
// returned UnitOfWork participate in the same transaction.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, uow UnitOfWork) error) error
}

// UnitOfWork exposes transactional repositories.
type UnitOfWork interface {
	Users() UserRepository
	Credentials() CredentialRepository
	Sessions() SessionRepository
	Tokens() TokenRepository
	Attempts() LoginAttemptRepository
	Roles() RoleRepository
	Outbox() OutboxRepository
}
