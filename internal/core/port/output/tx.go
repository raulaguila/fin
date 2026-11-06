package output

import (
	"context"
)

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
	Preferences() PreferenceRepository
	Notifications() NotificationRepository
}
