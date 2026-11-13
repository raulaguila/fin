package domain

import (
	"strings"
	"time"
)

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
	SourcePair = "pair" // matched with the opposite movement in another of the user's accounts
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

// ManualItemPrefix marks a connection made for imported files: no provider behind it.
const ManualItemPrefix = "manual:"

// Manual reports whether the connection holds imported data instead of a bank link.
func (c Connection) Manual() bool { return strings.HasPrefix(c.ItemID, ManualItemPrefix) }

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
	ID       string
	Name     string
	Kind     string
	Limit    Money // monthly; 0 means none
	Rollover bool  // last month's unspent limit (or overspend) moves into this month
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
	SplitOf          string // the original transaction when this row is a split part
	Note             string // the user's; only list and export read it
	Tags             []string
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
	Tag        string
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
	Rollover   bool
	Carry      Money // with rollover: last month's limit minus its spending, added to Limit
}

// Recurring is a charge seen in most recent months with a similar amount.
type Recurring struct {
	Key      string // the merchant key; the user may mark it as not recurring
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

// Change is how many transactions a category change touched, and the undo that puts them back.
type Change struct {
	Count  int64
	UndoID string
}

// AccountHealth is what the data health screen shows of one account.
type AccountHealth struct {
	ID          string
	Name        string
	Institution string
	Type        string
	Manual      bool
	LastDate    *time.Time // newest transaction
	CheckSince  *time.Time
	// Bank accounts: how much the balance moved beyond the transactions since CheckSince.
	// Not zero means transactions are missing (or extra) in Fin.
	Gap *Money
}

// BackupRun is the last outcome of one backup step: dump, copy or restore_check.
type BackupRun struct {
	Kind   string
	OK     bool
	Detail string
	At     time.Time
}

// DataHealth tells whether the numbers can be trusted.
type DataHealth struct {
	Accounts      []AccountHealth
	Uncategorized int64 // in the last 90 days
	Paired        int64 // transfers matched between own accounts, last 90 days
	Backups       []BackupRun
}

// KeyTotal is a sum with its count under a key: a month (YYYY-MM) or a description.
type KeyTotal struct {
	Key   string
	Total Money
	Count int64
}

// CategoryDetail is where one category's money went in a period.
type CategoryDetail struct {
	Months       []KeyTotal // oldest first
	Descriptions []KeyTotal // biggest first, at most CategoryDetailTop
}

// CategoryDetailTop caps the descriptions a category detail lists.
const CategoryDetailTop = 50

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
	Investments  []Investment
}

// Investment is a position the provider reports (fund, CDB, stocks), at its current value.
type Investment struct {
	ID           string
	ConnectionID string
	Name         string
	Type         string
	Balance      Money
}

// Goal is a savings target; Current is the linked account balance, or Saved without one.
type Goal struct {
	ID        string
	Name      string
	Target    Money
	Saved     Money
	AccountID string
	DueDate   *time.Time
	Current   Money
}

// Subscription is a subscription or fixed bill the user registered.
type Subscription struct {
	ID         string
	Name       string
	Amount     Money // what it costs, positive
	Day        int   // day of the month it is due
	Month      int   // 0 every month; 1-12 once a year in that month
	Keyword    string
	CategoryID string
	AccountID  string
	// Read from the transactions, not stored.
	LastPaid    *time.Time
	LastAmount  Money  // last charge, positive
	LastAccount string // account of the last charge
	Paid        bool   // charged in the current period
	Late        bool   // due a few days ago and not charged
	NextDue     time.Time
}

// Subscriptions lists the registered ones and the recurring charges found
// in the history that are neither registered nor dismissed.
type Subscriptions struct {
	Items    []Subscription
	Detected []Recurring
	Monthly  Money // what the registered ones cost a month, yearly ones spread over 12
}

// CalendarEvent is money expected on a day of the next weeks.
type CalendarEvent struct {
	Date    time.Time
	Kind    string // bill, recurring or income
	Name    string
	Amount  Money
	Balance Money // bank balance after this event
}

// Calendar projects the bank balance through the expected events.
type Calendar struct {
	Start  Money // bank balance today
	Events []CalendarEvent
	Lowest Money // lowest balance reached
}

// YearReport is one calendar year by category and by month, for the income tax return.
type YearReport struct {
	Year       int
	Income     Money
	Expense    Money
	Categories []CategoryTotal
	Months     []MonthTotal
}

// ChatMessage is one turn of the assistant conversation; the client keeps the history.
type ChatMessage struct {
	Role    string // user or assistant
	Content string
}

// Kinds of ChatAction.
const (
	ActionRule         = "rule"
	ActionSubscription = "subscription"
)

// ChatAction is a change the assistant proposes; it happens only when the user applies it.
type ChatAction struct {
	Kind         string
	Keyword      string // both kinds
	CategoryID   string // rule
	CategoryName string // rule
	Name         string // subscription
	Amount       Money  // subscription, positive
	Day          int    // subscription, monthly
}

// ChatReply is the assistant's answer and the changes it proposes.
type ChatReply struct {
	Answer  string
	Actions []ChatAction
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
	BankCode      string // COMPE code of a bank account ("341" is Itaú), empty for cards
}

type OFInvestment struct {
	ID      string
	Name    string
	Type    string
	Balance Money
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

// Debt is money lent or borrowed. Amount > 0: the person owes the user; < 0: the user owes.
type Debt struct {
	ID          string
	Person      string
	Amount      Money
	Description string
	DueDate     *time.Time
	SettledOn   *time.Time
	CreatedAt   time.Time
}

// NetWorthPoint is one day of the net worth: bank balances and investments, less card bills.
type NetWorthPoint struct {
	Day         time.Time
	Banks       Money
	Investments Money
	Cards       Money // what the cards owe, positive
}

// CardBill is one bill of a credit card: what was charged between two closing days.
type CardBill struct {
	AccountID   string
	AccountName string
	Close       time.Time  // the closing day; the bill holds the charges up to it
	Due         *time.Time // when the bank gave a due day
	Charges     Money      // purchases, negative
	Credits     Money      // refunds and payments, positive
	Count       int
}

// MonthAmount is an amount that falls in a month.
type MonthAmount struct {
	Month  time.Time
	Amount Money
}

// Cards is the bills of every card and what installments already commit ahead.
type Cards struct {
	Bills  []CardBill
	Future []MonthAmount // installments still to come, negative
}
