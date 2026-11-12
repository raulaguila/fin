package input

import (
	"context"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

// FinanceService is the personal finance of the signed in user: every method
// acts on userID's own data only.
type FinanceService interface {
	OpenFinanceEnabled() bool
	AssistantEnabled() bool

	ConnectToken(ctx context.Context, userID, connectionID string) (string, error)
	AddConnection(ctx context.Context, userID, itemID string) (*domain.Connection, error)
	Connections(ctx context.Context, userID string) ([]domain.Connection, error)
	SyncConnection(ctx context.Context, userID, id string) error
	DeleteConnection(ctx context.Context, userID, id string) error
	// Webhook takes a provider event; the payload is only a hint, data is re-read.
	Webhook(ctx context.Context, event, itemID string) error

	Summary(ctx context.Context, userID string, month time.Time) (*domain.MonthSummary, error)
	Transactions(ctx context.Context, userID string, filter domain.TransactionFilter) ([]domain.Transaction, int64, error)
	SetCategory(ctx context.Context, userID, transactionID, categoryID string) error

	Categories(ctx context.Context, userID string) ([]domain.Category, error)
	CreateCategory(ctx context.Context, userID string, c domain.Category) (*domain.Category, error)
	UpdateCategory(ctx context.Context, userID, id string, name *string, limit *domain.Money) (*domain.Category, error)
	DeleteCategory(ctx context.Context, userID, id string) error

	Rules(ctx context.Context, userID string) ([]domain.Rule, error)
	PreviewRule(ctx context.Context, userID, keyword string) (int64, error)
	// CreateRule saves the rule and applies it to past transactions, returning how many changed.
	CreateRule(ctx context.Context, userID, keyword, categoryID string) (*domain.Rule, int64, error)
	DeleteRule(ctx context.Context, userID, id string) error

	Chat(ctx context.Context, userID string, history []domain.ChatMessage) (string, error)
}
