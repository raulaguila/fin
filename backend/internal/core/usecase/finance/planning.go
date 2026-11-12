package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/port/output"
	"github.com/raulaguila/fin/pkg/apperror"
	"github.com/raulaguila/fin/pkg/validate"
)

const (
	calendarDays  = 30
	incomeLookup  = 35   // days of salaries read to expect the next ones
	importMax     = 2000 // lines per import call
	maxMoneyCents = 1_000_000_000_00
)

// ---- splits ----

func (s *Service) SplitTransaction(ctx context.Context, userID, id string, amount domain.Money, categoryID string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	if _, err := s.ownCategory(ctx, userID, categoryID); err != nil {
		return err
	}
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		return uow.Finance().SplitTransaction(ctx, userID, id, amount, categoryID)
	})
}

func (s *Service) UnsplitTransaction(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().UnsplitTransaction(ctx, userID, id)
}

// ---- year report ----

func (s *Service) YearReport(ctx context.Context, userID string, year int) (*domain.YearReport, error) {
	now := s.Clock.Now()
	if year == 0 {
		year = now.Year()
	}
	if year < 2000 || year > now.Year() {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid year").WithField("year")
	}
	from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, 0)
	cats, err := s.Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	totals, err := s.repo().Totals(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	months, err := s.repo().MonthlyTotals(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	r := &domain.YearReport{Year: year, Months: months}
	r.Income, r.Expense, r.Categories = combine(cats, totals)
	return r, nil
}

// ---- calendar ----

func (s *Service) Calendar(ctx context.Context, userID string) (*domain.Calendar, error) {
	today := monthDay(s.Clock.Now())
	accounts, err := s.repo().Accounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	spending, err := s.repo().Spending(ctx, userID, monthStart(today).AddDate(0, 1-recurringMonths, 0), today.AddDate(0, 0, 1), spendingCap)
	if err != nil {
		return nil, err
	}
	cats, err := s.Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	var incomes []domain.Transaction
	for _, c := range cats {
		if strings.EqualFold(c.Name, catSalary) {
			incomes, _, err = s.repo().Transactions(ctx, userID, domain.TransactionFilter{
				From: today.AddDate(0, 0, -incomeLookup), CategoryID: c.ID, Page: domain.NewPage(1, 50)})
			if err != nil {
				return nil, err
			}
		}
	}
	return calendar(today, accounts, spending, incomes), nil
}

// calendar projects the bank balance through the card bills, the recurring
// charges of bank accounts (card charges are inside the bill) and the salaries
// expected a month after the last ones.
func calendar(today time.Time, accounts []domain.Account, spending, incomes []domain.Transaction) *domain.Calendar {
	end := today.AddDate(0, 0, calendarDays)
	inWindow := func(d time.Time) bool { return !d.Before(today) && !d.After(end) }
	cal := &domain.Calendar{}
	var events []domain.CalendarEvent
	for _, a := range accounts {
		switch a.Type {
		case domain.AccountBank:
			cal.Start += a.Balance
		case domain.AccountCredit:
			if due := a.NextBillDue(today); due != nil && inWindow(*due) && a.Balance > 0 {
				events = append(events, domain.CalendarEvent{Date: *due, Kind: "bill", Name: a.Name, Amount: -a.Balance})
			}
		}
	}
	var bank []domain.Transaction
	for _, t := range spending {
		if t.AccountType == domain.AccountBank {
			bank = append(bank, t)
		}
	}
	for _, r := range recurring(bank) {
		next := r.LastDate.AddDate(0, 1, 0)
		if next.Before(today) {
			next = today // late this month: still to come
		}
		for ; inWindow(next); next = next.AddDate(0, 1, 0) {
			events = append(events, domain.CalendarEvent{Date: next, Kind: "recurring", Name: r.Name, Amount: r.Amount})
		}
	}
	for _, t := range incomes {
		if next := t.Date.AddDate(0, 1, 0); t.Amount > 0 && inWindow(next) {
			events = append(events, domain.CalendarEvent{Date: next, Kind: "income", Name: t.Description, Amount: t.Amount})
		}
	}
	// Same day: money in first, as a salary usually lands before the bills.
	slices.SortStableFunc(events, func(a, b domain.CalendarEvent) int {
		if c := a.Date.Compare(b.Date); c != 0 {
			return c
		}
		return int(b.Amount - a.Amount)
	})
	balance := cal.Start
	cal.Lowest = balance
	for i := range events {
		balance += events[i].Amount
		events[i].Balance = balance
		cal.Lowest = min(cal.Lowest, balance)
	}
	cal.Events = events
	return cal
}

// ---- imported accounts ----

func shortText(field, v string, max int) (string, error) {
	v = strings.Join(strings.Fields(v), " ")
	if n := utf8.RuneCountInString(v); n < 1 || n > max || !utf8.ValidString(v) {
		return "", apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("%s must have 1 to %d characters", field, max)).WithField(field)
	}
	return v, nil
}

func (s *Service) CreateManualAccount(ctx context.Context, userID, institution, name, accountType string) (*domain.Account, error) {
	institution, err := shortText("institution", institution, 60)
	if err != nil {
		return nil, err
	}
	if name, err = shortText("name", name, 60); err != nil {
		return nil, err
	}
	subtype := map[string]string{domain.AccountBank: "CHECKING_ACCOUNT", domain.AccountCredit: "CREDIT_CARD"}[accountType]
	if subtype == "" {
		return nil, apperror.New(apperror.CodeInvalidInput, "type must be BANK or CREDIT").WithField("type")
	}
	var acc *domain.Account
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if err := uow.Finance().EnsureCategories(ctx, userID, DefaultCategories); err != nil {
			return err
		}
		c := &domain.Connection{UserID: userID, ItemID: domain.ManualItemPrefix + uuid.NewString(), Institution: institution, Status: domain.ConnOK}
		if err := uow.Finance().CreateConnection(ctx, c); err != nil {
			return err
		}
		var err error
		acc, err = uow.Finance().UpsertAccount(ctx, userID, c.ID, domain.OFAccount{
			ID: domain.ManualItemPrefix + uuid.NewString(), Type: accountType, Subtype: subtype, Name: name})
		return err
	})
	return acc, err
}

// manualAccount finds the user's account and refuses one a bank keeps: an
// import there would duplicate what the next sync brings.
func (s *Service) manualAccount(ctx context.Context, userID, accountID string) (*domain.Account, error) {
	if err := validate.ID(accountID); err != nil {
		return nil, err
	}
	conns, err := s.repo().Connections(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range conns {
		for _, a := range c.Accounts {
			if a.ID != accountID {
				continue
			}
			if !c.Manual() {
				return nil, apperror.New(apperror.CodeConflict, "only imported accounts take files").WithField("account_id")
			}
			return &a, nil
		}
	}
	return nil, apperror.New(apperror.CodeNotFound, "account not found").WithDetails("resource", "account")
}

// importID keeps a re-imported file from duplicating lines: the file's own id
// when it has one, else the line's content and its order among equal lines.
func importID(accountID string, t domain.Transaction, seen map[string]int) string {
	if t.ExternalID != "" {
		return "import:" + accountID + ":" + t.ExternalID
	}
	key := fmt.Sprintf("%s|%d|%s", t.Date.Format(time.DateOnly), t.Amount, strings.ToLower(t.Description))
	seen[key]++
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%d", key, seen[key]))
	return "import:" + accountID + ":" + hex.EncodeToString(sum[:12])
}

func (s *Service) Import(ctx context.Context, userID, accountID string, txs []domain.Transaction, balance *domain.Money) (int, error) {
	acc, err := s.manualAccount(ctx, userID, accountID)
	if err != nil {
		return 0, err
	}
	if len(txs) > importMax {
		return 0, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("at most %d lines per import", importMax)).WithField("transactions")
	}
	if balance != nil && (*balance > maxMoneyCents || *balance < -maxMoneyCents) {
		return 0, apperror.New(apperror.CodeInvalidInput, "invalid balance").WithField("balance")
	}
	cat, err := s.categorizer(ctx, userID)
	if err != nil {
		return 0, err
	}
	earliest, latest := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), s.Clock.Now().AddDate(0, 0, 1)
	seen := map[string]int{}
	clean := make([]domain.Transaction, 0, len(txs))
	for i, t := range txs {
		field := fmt.Sprintf("transactions[%d]", i)
		if t.Date.Before(earliest) || t.Date.After(latest) || t.Amount == 0 || t.Amount > maxMoneyCents || t.Amount < -maxMoneyCents {
			return 0, apperror.New(apperror.CodeInvalidInput, "each line needs a date up to today and a non-zero amount").WithField(field)
		}
		t.Description = validate.Text(strings.Join(strings.Fields(t.Description), " "), 300)
		if t.Description == "" {
			return 0, apperror.New(apperror.CodeInvalidInput, "each line needs a description").WithField(field)
		}
		t.ExternalID = importID(acc.ID, domain.Transaction{ExternalID: validate.Text(t.ExternalID, 100), Date: t.Date, Amount: t.Amount, Description: t.Description}, seen)
		clean = append(clean, cat.apply(domain.Transaction{ExternalID: t.ExternalID, Date: t.Date, Description: t.Description, Amount: t.Amount,
			Status: "POSTED", AccountType: acc.Type}))
	}
	n, err := s.repo().UpsertTransactions(ctx, userID, acc.ID, clean)
	if err != nil {
		return n, err
	}
	if balance != nil {
		_, err = s.repo().UpsertAccount(ctx, userID, acc.ConnectionID, domain.OFAccount{
			ID: acc.ExternalID, Type: acc.Type, Subtype: acc.Subtype, Name: acc.Name, Number: acc.Number, Balance: *balance})
	}
	return n, err
}

// ---- goals ----

func (s *Service) Goals(ctx context.Context, userID string) ([]domain.Goal, error) {
	return s.repo().Goals(ctx, userID)
}

func (s *Service) checkGoal(ctx context.Context, userID string, g *domain.Goal) error {
	var err error
	if g.Name, err = shortText("name", g.Name, 60); err != nil {
		return err
	}
	if g.Target <= 0 || g.Target > maxMoneyCents {
		return apperror.New(apperror.CodeInvalidInput, "invalid target").WithField("target")
	}
	if g.Saved < 0 || g.Saved > maxMoneyCents {
		return apperror.New(apperror.CodeInvalidInput, "invalid saved amount").WithField("saved")
	}
	if g.AccountID == "" {
		return nil
	}
	accounts, err := s.repo().Accounts(ctx, userID)
	if err != nil {
		return err
	}
	for _, a := range accounts {
		if a.ID == g.AccountID {
			return nil
		}
	}
	return apperror.New(apperror.CodeNotFound, "account not found").WithDetails("resource", "account")
}

func (s *Service) CreateGoal(ctx context.Context, userID string, g domain.Goal) (*domain.Goal, error) {
	if err := s.checkGoal(ctx, userID, &g); err != nil {
		return nil, err
	}
	if err := s.repo().CreateGoal(ctx, userID, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Service) UpdateGoal(ctx context.Context, userID, id string, g domain.Goal) (*domain.Goal, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	g.ID = id
	if err := s.checkGoal(ctx, userID, &g); err != nil {
		return nil, err
	}
	if err := s.repo().UpdateGoal(ctx, userID, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Service) DeleteGoal(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().DeleteGoal(ctx, userID, id)
}
