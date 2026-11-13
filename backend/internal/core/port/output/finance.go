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
	// LastBill returns the last closed bill of a card account; nil when there is none.
	LastBill(ctx context.Context, accountID string) (due *time.Time, amount *domain.Money, err error)
	// Transactions returns every transaction of the account dated from `from` on.
	Transactions(ctx context.Context, accountID string, from time.Time) ([]domain.OFTransaction, error)
	// Investments returns the positions of the item at their current value.
	Investments(ctx context.Context, itemID string) ([]domain.OFInvestment, error)
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
	// SetTransactionCategory returns the transaction description.
	SetTransactionCategory(ctx context.Context, userID, id, categoryID string) (string, error)
	// Categorizable returns, newest first, the transactions not categorized by
	// hand (id, description, amount, provider category, category, source).
	Categorizable(ctx context.Context, userID string, limit int) ([]domain.Transaction, error)
	// SetCategories writes category id and source of each transaction, skipping
	// any the user set by hand meanwhile. It returns how many changed.
	SetCategories(ctx context.Context, userID string, txs []domain.Transaction) (int64, error)
	// Uncategorized returns date, description and amount of the transactions
	// without a category in [from, now), newest first.
	Uncategorized(ctx context.Context, userID string, from time.Time, limit int) ([]domain.Transaction, error)
	// PersonTransfers returns Pix and TED with a person named in the description
	// (date, description, amount, provider category, category name).
	PersonTransfers(ctx context.Context, userID string, from time.Time, limit int) ([]domain.Transaction, error)
	// CategoryDetail sums one category by month and by description in [from, to).
	// An empty categoryID is the uncategorized money of that sign (1 or -1).
	CategoryDetail(ctx context.Context, userID, categoryID string, sign int, from, to time.Time) (*domain.CategoryDetail, error)
	// MonthlyTotals sums income and expense by month in [from, to).
	MonthlyTotals(ctx context.Context, userID string, from, to time.Time) ([]domain.MonthTotal, error)
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
	// ApplyRule sets categoryID on matching transactions not set by the user, keeping an undo tied to the rule.
	ApplyRule(ctx context.Context, userID, ruleID, keyword, categoryID string) (domain.Change, error)

	// DeleteMissingPending removes the pending transactions of the account dated
	// from `from` on whose external id is not in keep: the bank replaced them.
	DeleteMissingPending(ctx context.Context, accountID string, from time.Time, keep []string) (int64, error)
	// SplitTransaction moves amount of the transaction (same sign, smaller) to a
	// new row in categoryID; the original keeps the rest.
	SplitTransaction(ctx context.Context, userID, id string, amount domain.Money, categoryID string) error
	// UnsplitTransaction folds the split parts of the original transaction id back into it.
	UnsplitTransaction(ctx context.Context, userID, id string) error

	// UpsertInvestments saves the connection's positions and drops the ones the provider no longer lists.
	UpsertInvestments(ctx context.Context, userID, connectionID string, list []domain.OFInvestment) error
	Investments(ctx context.Context, userID string) ([]domain.Investment, error)

	Subscriptions(ctx context.Context, userID string) ([]domain.Subscription, error)
	CreateSubscription(ctx context.Context, userID string, s *domain.Subscription) error
	UpdateSubscription(ctx context.Context, userID string, s *domain.Subscription) error
	DeleteSubscription(ctx context.Context, userID, id string) error
	// IgnoredRecurring lists the merchant keys the user said are not recurring.
	IgnoredRecurring(ctx context.Context, userID string) ([]string, error)
	IgnoreRecurring(ctx context.Context, userID, key string) error
	Goals(ctx context.Context, userID string) ([]domain.Goal, error)
	CreateGoal(ctx context.Context, userID string, g *domain.Goal) error
	UpdateGoal(ctx context.Context, userID string, g *domain.Goal) error
	DeleteGoal(ctx context.Context, userID, id string) error

	// CreateTransaction inserts one transaction the user typed, with its note and tags.
	CreateTransaction(ctx context.Context, userID, accountID string, t *domain.Transaction) error
	// The account's balance moves with the amount in the same statement.
	// DeleteManualTransaction removes a transaction of an imported account (never
	// one a bank keeps, never a split part) and takes it back off the balance.
	DeleteManualTransaction(ctx context.Context, userID, id string) error
	SetTransactionDetails(ctx context.Context, userID, id, note string, tags []string) error
	// SetTransactionsCategory sets the category of the user's transactions in ids, as the user's choice, keeping an undo.
	SetTransactionsCategory(ctx context.Context, userID string, ids []string, categoryID string) (domain.Change, error)
	// Undo puts back what one change overwrote, on rows not changed again since, and drops
	// the rule that made it; it returns how many rows went back.
	Undo(ctx context.Context, userID, undoID string) (int64, error)
	// PruneUndo forgets the undos made before t.
	PruneUndo(ctx context.Context, before time.Time) error
	// PairTransfers files as categoryID (source pair) the unambiguous pairs of opposite
	// amounts of at least minimum between two of the user's accounts, at most days apart,
	// dated from on, that only automatic categorization touched.
	PairTransfers(ctx context.Context, userID, categoryID string, from time.Time, days int, minimum domain.Money) (int64, error)
	// StartBalanceCheck records the balance and the transactions sum of a bank account,
	// once; reset starts it over from now.
	StartBalanceCheck(ctx context.Context, userID, accountID string, reset bool) error
	// ArchiveOverrides lists, newest first, the bank transactions the user filed, noted or tagged.
	ArchiveOverrides(ctx context.Context, userID string, limit int) ([]domain.ArchiveOverride, error)
	// ApplyOverrides writes them back on the user's transactions with the same bank id.
	ApplyOverrides(ctx context.Context, userID string, list []domain.ArchiveOverride) (int64, error)
	// RestoreTransactions inserts into a manual account the rows not in yet (by external id),
	// with category by name, note and tags, and adds them to its balance.
	RestoreTransactions(ctx context.Context, userID, accountID string, txs []domain.Transaction) (int64, error)
	// DataHealth reads the accounts with their balance check, the counts since from and the last backups.
	DataHealth(ctx context.Context, userID string, from time.Time) (*domain.DataHealth, error)
	// ExportTransactions returns up to limit rows of the filter, with notes and tags, newest first.
	ExportTransactions(ctx context.Context, userID string, f domain.TransactionFilter, limit int) ([]domain.Transaction, error)
	// CountTransactions counts the rows of the filter.
	CountTransactions(ctx context.Context, userID string, f domain.TransactionFilter) (int64, error)
	// CardTransactions returns the credit card rows from `from` on (account, date,
	// description, amount, installments), newest first.
	CardTransactions(ctx context.Context, userID string, from time.Time, limit int) ([]domain.Transaction, error)

	// SaveNetWorth writes the day's picture, replacing one of the same day.
	SaveNetWorth(ctx context.Context, userID string, p domain.NetWorthPoint) error
	NetWorth(ctx context.Context, userID string, from time.Time) ([]domain.NetWorthPoint, error)

	Debts(ctx context.Context, userID string) ([]domain.Debt, error)
	CreateDebt(ctx context.Context, userID string, d *domain.Debt) error
	UpdateDebt(ctx context.Context, userID string, d *domain.Debt) error
	DeleteDebt(ctx context.Context, userID, id string) error

	// MarkAlert records key for the user and reports whether it is new.
	MarkAlert(ctx context.Context, userID, key string) (bool, error)
}
