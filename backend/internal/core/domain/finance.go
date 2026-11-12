package domain

import "time"

// Money is integer cents. A negative transaction amount is money leaving the user.
type Money = int64

// Connection statuses match app.fin_connection.
const (
	ConnSyncing       = "syncing"
	ConnOK            = "ok"
	ConnError         = "error"
	ConnLoginRequired = "login_required"
)

// Account types match app.fin_account and Pluggy.
const (
	AccountBank   = "BANK"
	AccountCredit = "CREDIT"
)

// Category kinds: only expense counts as spending; transfer is money moving
// between the user's own accounts (card bill payments, investments) and counts nowhere.
const (
	KindExpense  = "expense"
	KindIncome   = "income"
	KindTransfer = "transfer"
)

// Category sources: a user choice is never overwritten by a sync or a rule.
const (
	SourceAuto = "auto"
	SourceRule = "rule"
	SourceUser = "user"
)

// Connection is one bank linked through Open Finance (a Pluggy item).
type Connection struct {
	ID               string
	UserID           string
	ItemID           string
	Institution      string
	ImageURL         string
	Status           string
	Error            string
	ConsentExpiresAt *time.Time
	LastSyncAt       *time.Time
	CreatedAt        time.Time
	Accounts         []Account
}

type Account struct {
	ID            string
	ConnectionID  string
	ExternalID    string
	Type          string
	Subtype       string // CHECKING_ACCOUNT, SAVINGS_ACCOUNT, CREDIT_CARD...
	Name          string
	Number        string
	Balance       Money
	CreditLimit   *Money
	Available     *Money
	BillCloseDate *time.Time
	BillDueDate   *time.Time
	LastBillDue   *time.Time // the last closed card bill
	LastBill      *Money
}

// NextBillDue is the next due date of a card from today: the provider's date
// when still ahead, else the same day of the following months.
func (a Account) NextBillDue(today time.Time) *time.Time {
	if a.BillDueDate == nil {
		return nil
	}
	due := *a.BillDueDate
	for i := 0; due.Before(today) && i < 24; i++ {
		due = a.BillDueDate.AddDate(0, i+1, 0)
	}
	return &due
}

type Category struct {
	ID    string
	Name  string
	Kind  string
	Limit Money // monthly; 0 means none
}

type Transaction struct {
	ID               string
	AccountID        string
	AccountName      string
	AccountType      string
	Institution      string
	ExternalID       string
	Date             time.Time
	Description      string
	Amount           Money
	Status           string
	ProviderCategory string
	CategoryID       string
	CategoryName     string
	CategoryKind     string
	CategorySource   string
	Installment      int
	Installments     int
}

type Rule struct {
	ID           string
	Keyword      string
	CategoryID   string
	CategoryName string
	Hits         int64 // transactions it currently matches
}

// TransactionFilter selects transactions of one user.
type TransactionFilter struct {
	From       time.Time // inclusive
	To         time.Time // exclusive
	Search     string
	AccountID  string
	CategoryID string
	Page       Page
}

// CategoryTotal is what one category moved in a period.
type CategoryTotal struct {
	CategoryID string
	Name       string
	Kind       string
	Limit      Money
	Total      Money // sum of amounts, negative for spending
	Count      int64
}

// Recurring is a charge seen in most recent months with a similar amount.
type Recurring struct {
	Name     string
	Amount   Money // typical amount, negative
	Months   int
	LastDate time.Time
	Changed  bool // the last charge differs from the one before by more than 10%
	Previous Money
	Last     Money
}

// Installment is a future card installment already committed.
type Installment struct {
	Description string
	Amount      Money // per installment, negative
	Remaining   int
}

// Forecast projects the rest of the current month from recurring charges and
// the salary of the month before.
type Forecast struct {
	DaysLeft       int
	PendingBills   Money // recurring charges not seen yet this month, negative
	ExpectedIncome Money // salary still to come
	Projected      Money // what is left at the end of the month
}

// MonthTotal is what came in and went out in one month, transfers left out.
type MonthTotal struct {
	Month   time.Time
	Income  Money
	Expense Money // negative
}

// Person is someone the user sent money to or received money from (Pix, TED).
type Person struct {
	Name     string
	Keyword  string // matches the transfers received from this person, for a rule
	Sent     Money  // negative
	Received Money
	Refunded Money // received and filed as a refund
	Count    int
	LastDate time.Time
}

// RuleSuggestion is a merchant that repeats among uncategorized transactions.
type RuleSuggestion struct {
	Keyword string
	Example string
	Count   int
	Total   Money
}

// MonthSummary is the dashboard of one month.
type MonthSummary struct {
	Month        time.Time
	Income       Money
	Expense      Money // negative
	Categories   []CategoryTotal
	Previous     map[string]Money // category id → total of the previous month
	Accounts     []Account
	Institutions map[string]string // connection id → institution
	Recent       []Transaction
	Recurring    []Recurring
	Installments []Installment
	Forecast     *Forecast // only for the current month
}

// ChatMessage is one turn of the assistant conversation; the client keeps the history.
type ChatMessage struct {
	Role    string // user or assistant
	Content string
}

// Open Finance data as the provider returns it, before normalization.

type OFItem struct {
	ID               string
	ClientUserID     string
	Institution      string
	ImageURL         string
	Status           string // provider status, e.g. UPDATED, LOGIN_ERROR, OUTDATED
	ExecutionStatus  string
	Error            string
	ConsentExpiresAt *time.Time
}

type OFAccount struct {
	ID            string
	Type          string
	Subtype       string
	Name          string
	Number        string
	Balance       Money
	CreditLimit   *Money
	Available     *Money
	BillCloseDate *time.Time
	BillDueDate   *time.Time
	LastBillDue   *time.Time
	LastBill      *Money
}

type OFTransaction struct {
	ID           string
	Date         time.Time
	Description  string
	Amount       Money // provider sign; normalized by the account type at sync
	Status       string
	Category     string
	Installment  int
	Installments int
}
