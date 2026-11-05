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
}

// LoginAttemptRepository appends login audits.
type LoginAttemptRepository interface {
	Create(ctx context.Context, attempt *domain.LoginAttempt) error
	CountRecentFailuresByEmail(ctx context.Context, email string, since time.Time) (int, error)
	CountRecentFailuresByIP(ctx context.Context, ip string, since time.Time) (int, error)
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
