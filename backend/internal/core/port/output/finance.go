package output

import (
	"context"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

// OpenFinance is the aggregator that reaches the banks (Pluggy).
type OpenFinance interface {
	Enabled() bool
	// ConnectToken lets the web widget link a bank for clientUserID; with itemID
	// it reopens that connection to renew its consent or credentials.
	ConnectToken(ctx context.Context, clientUserID, itemID string) (string, error)
	Item(ctx context.Context, id string) (*domain.OFItem, error)
	// RefreshItem asks the provider to fetch fresh data from the bank.
	RefreshItem(ctx context.Context, id string) error
	DeleteItem(ctx context.Context, id string) error
	Accounts(ctx context.Context, itemID string) ([]domain.OFAccount, error)
	// Transactions returns every transaction of the account dated from `from` on.
	Transactions(ctx context.Context, accountID string, from time.Time) ([]domain.OFTransaction, error)
}

// FinanceRepository persists the app.fin_* tables. Methods that take a userID
// only see that user's rows; a row of another user answers not found.
type FinanceRepository interface {
	CreateConnection(ctx context.Context, c *domain.Connection) error
	Connections(ctx context.Context, userID string) ([]domain.Connection, error)
	// FindConnection reads one connection of any user, for jobs and webhooks.
	FindConnection(ctx context.Context, id string) (*domain.Connection, error)
	ConnectionByItem(ctx context.Context, itemID string) (*domain.Connection, error)
	ConnectionIDs(ctx context.Context) ([]string, error)
	// SaveConnectionState writes institution, image, status, error, consent and,
	// when synced is true, last_sync_at = now.
	SaveConnectionState(ctx context.Context, c *domain.Connection, synced bool) error
	DeleteConnection(ctx context.Context, userID, id string) error

	UpsertAccount(ctx context.Context, userID, connectionID string, a domain.OFAccount) (*domain.Account, error)
	// UpsertTransactions inserts new transactions and updates known ones,
	// keeping a category the user chose. It returns how many were new.
	UpsertTransactions(ctx context.Context, userID, accountID string, txs []domain.Transaction) (int, error)
	Transactions(ctx context.Context, userID string, filter domain.TransactionFilter) ([]domain.Transaction, int64, error)
	// Spending returns the transactions in [from, to) not in a transfer
	// category, up to limit, newest first: input for recurring and insights.
	Spending(ctx context.Context, userID string, from, to time.Time, limit int) ([]domain.Transaction, error)
	SetTransactionCategory(ctx context.Context, userID, id, categoryID string) error
	// Totals sums [from, to) by category; uncategorized money comes as two rows
	// with an empty category id, one for inflows and one for outflows.
	Totals(ctx context.Context, userID string, from, to time.Time) ([]domain.CategoryTotal, error)
	Accounts(ctx context.Context, userID string) ([]domain.Account, error)

	// EnsureCategories creates the defaults for a user who has none.
	EnsureCategories(ctx context.Context, userID string, defaults []domain.Category) error
	Categories(ctx context.Context, userID string) ([]domain.Category, error)
	CreateCategory(ctx context.Context, userID string, c *domain.Category) error
	UpdateCategory(ctx context.Context, userID string, c *domain.Category) error
	DeleteCategory(ctx context.Context, userID, id string) error

	Rules(ctx context.Context, userID string) ([]domain.Rule, error)
	CreateRule(ctx context.Context, userID string, r *domain.Rule) error
	DeleteRule(ctx context.Context, userID, id string) error
	// CountMatches counts transactions whose description contains keyword.
	CountMatches(ctx context.Context, userID, keyword string) (int64, error)
	// ApplyRule sets categoryID on matching transactions not set by the user.
	ApplyRule(ctx context.Context, userID, keyword, categoryID string) (int64, error)

	// MarkAlert records key for the user and reports whether it is new.
	MarkAlert(ctx context.Context, userID, key string) (bool, error)
}
