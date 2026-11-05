package postgres

import (
	"context"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/attempt"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/credential"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/outbox"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/role"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/session"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/token"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/user"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

// Store is the root UnitOfWork / TxManager backed by GORM.
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Users() output.UserRepository             { return user.NewRepository(s.db) }
func (s *Store) Credentials() output.CredentialRepository { return credential.NewRepository(s.db) }
func (s *Store) Sessions() output.SessionRepository       { return session.NewRepository(s.db) }
func (s *Store) Tokens() output.TokenRepository           { return token.NewRepository(s.db) }
func (s *Store) Attempts() output.LoginAttemptRepository  { return attempt.NewRepository(s.db) }
func (s *Store) Roles() output.RoleRepository             { return role.NewRepository(s.db) }
func (s *Store) Outbox() output.OutboxRepository          { return outbox.NewRepository(s.db) }

func (s *Store) WithinTx(ctx context.Context, fn func(ctx context.Context, uow output.UnitOfWork) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ctx, &Store{db: tx})
	})
}

var (
	_ output.TxManager  = (*Store)(nil)
	_ output.UnitOfWork = (*Store)(nil)
)
