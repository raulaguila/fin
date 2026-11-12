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
	// SetCategory returns the keyword a rule for this merchant would use; empty when none fits.
	SetCategory(ctx context.Context, userID, transactionID, categoryID string) (string, error)
	// Recategorize runs the rules and the automatic categorization again over
	// the history, leaving choices made by hand alone. It returns how many changed.
	Recategorize(ctx context.Context, userID string) (int64, error)
	// Trend is income and expense of the last months, oldest first.
	Trend(ctx context.Context, userID string, months int) ([]domain.MonthTotal, error)
	// People groups Pix and TED by person over the last 12 months.
	People(ctx context.Context, userID string) ([]domain.Person, error)

	Categories(ctx context.Context, userID string) ([]domain.Category, error)
	CreateCategory(ctx context.Context, userID string, c domain.Category) (*domain.Category, error)
	UpdateCategory(ctx context.Context, userID, id string, name *string, limit *domain.Money) (*domain.Category, error)
	DeleteCategory(ctx context.Context, userID, id string) error

	Rules(ctx context.Context, userID string) ([]domain.Rule, error)
	PreviewRule(ctx context.Context, userID, keyword string) (int64, error)
	// CreateRule saves the rule and applies it to past transactions, returning how many changed.
	CreateRule(ctx context.Context, userID, keyword, categoryID string) (*domain.Rule, int64, error)
	DeleteRule(ctx context.Context, userID, id string) error
	// RuleSuggestions lists merchants that repeat among uncategorized transactions.
	RuleSuggestions(ctx context.Context, userID string) ([]domain.RuleSuggestion, error)

	// SplitTransaction moves part of a transaction to another category.
	SplitTransaction(ctx context.Context, userID, id string, amount domain.Money, categoryID string) error
	// UnsplitTransaction joins the parts of a split transaction again.
	UnsplitTransaction(ctx context.Context, userID, id string) error
	// YearReport totals a calendar year by category and month.
	YearReport(ctx context.Context, userID string, year int) (*domain.YearReport, error)
	// Calendar projects the bank balance over the next 30 days.
	Calendar(ctx context.Context, userID string) (*domain.Calendar, error)

	// CreateManualAccount makes an account that takes imported statements.
	CreateManualAccount(ctx context.Context, userID, institution, name, accountType string) (*domain.Account, error)
	// Import saves statement lines into a manual account; balance, when given,
	// becomes the account balance. It returns how many were new.
	Import(ctx context.Context, userID, accountID string, txs []domain.Transaction, balance *domain.Money) (int, error)

	Subscriptions(ctx context.Context, userID string) (*domain.Subscriptions, error)
	CreateSubscription(ctx context.Context, userID string, s domain.Subscription) (*domain.Subscription, error)
	UpdateSubscription(ctx context.Context, userID, id string, s domain.Subscription) (*domain.Subscription, error)
	DeleteSubscription(ctx context.Context, userID, id string) error
	// IgnoreRecurring keeps a detected charge out of the lists and the forecasts.
	IgnoreRecurring(ctx context.Context, userID, key string) error
	Goals(ctx context.Context, userID string) ([]domain.Goal, error)
	CreateGoal(ctx context.Context, userID string, g domain.Goal) (*domain.Goal, error)
	UpdateGoal(ctx context.Context, userID, id string, g domain.Goal) (*domain.Goal, error)
	DeleteGoal(ctx context.Context, userID, id string) error

	Chat(ctx context.Context, userID string, history []domain.ChatMessage) (string, error)
}
