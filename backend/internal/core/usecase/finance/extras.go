package finance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/apperror"
	"github.com/raulaguila/fin/pkg/validate"
)

const (
	maxTags      = 10
	maxNote      = 500
	exportCap    = 10_000 // rows in one CSV
	cardMonths   = 6      // card bills shown back
	cardRowsCap  = 5_000
	maxNetMonths = 24
)

// ---- transactions typed by the user ----

// normTags lowers, trims and dashes each tag ("Viagem SP" gives "viagem-sp"),
// drops repeats and refuses what a filter or the CSV could not hold.
func normTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		tag := strings.ToLower(strings.Join(strings.Fields(raw), "-"))
		if tag == "" {
			continue
		}
		if n := utf8.RuneCountInString(tag); n > 30 || !utf8.ValidString(tag) || strings.IndexFunc(tag, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
		}) >= 0 {
			return nil, apperror.New(apperror.CodeInvalidInput, "a tag has up to 30 letters, digits, - or _").WithField("tags")
		}
		if !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	if len(out) > maxTags {
		return nil, apperror.New(apperror.CodeInvalidInput, "at most 10 tags").WithField("tags")
	}
	return out, nil
}

func checkNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNote || !utf8.ValidString(note) {
		return "", apperror.New(apperror.CodeInvalidInput, "note must have up to 500 characters").WithField("note")
	}
	return note, nil
}

func (s *Service) AddTransaction(ctx context.Context, userID string, t domain.Transaction) (*domain.Transaction, error) {
	acc, err := s.manualAccount(ctx, userID, t.AccountID)
	if err != nil {
		return nil, err
	}
	if t.Description, err = shortText("description", t.Description, 200); err != nil {
		return nil, err
	}
	if t.Amount == 0 || abs(t.Amount) > 1_000_000_000_00 {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid amount").WithField("amount")
	}
	if now := s.Clock.Now(); t.Date.IsZero() || t.Date.Year() < 2000 || t.Date.After(now.AddDate(1, 0, 0)) {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid date").WithField("date")
	}
	if t.Note, err = checkNote(t.Note); err != nil {
		return nil, err
	}
	if t.Tags, err = normTags(t.Tags); err != nil {
		return nil, err
	}
	if t.CategoryID != "" {
		if _, err := s.ownCategory(ctx, userID, t.CategoryID); err != nil {
			return nil, err
		}
		t.CategorySource = domain.SourceUser
	} else {
		cz, err := s.categorizer(ctx, userID)
		if err != nil {
			return nil, err
		}
		t.AccountType = acc.Type
		t = cz.apply(t)
	}
	t.ExternalID, t.Status, t.AccountName, t.AccountType = domain.ManualItemPrefix+uuid.NewString(), "POSTED", acc.Name, acc.Type
	if err := s.repo().CreateTransaction(ctx, userID, acc.ID, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) DeleteTransaction(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().DeleteManualTransaction(ctx, userID, id)
}

func (s *Service) SetTransactionDetails(ctx context.Context, userID, id, note string, tags []string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	note, err := checkNote(note)
	if err != nil {
		return err
	}
	if tags, err = normTags(tags); err != nil {
		return err
	}
	return s.repo().SetTransactionDetails(ctx, userID, id, note, tags)
}

const maxBatch = 200

func (s *Service) CategorizeMany(ctx context.Context, userID string, ids []string, categoryID string) (int64, error) {
	if len(ids) == 0 || len(ids) > maxBatch {
		return 0, apperror.New(apperror.CodeInvalidInput, "send 1 to 200 ids").WithField("ids")
	}
	for _, id := range ids {
		if err := validate.ID(id); err != nil {
			return 0, err
		}
	}
	if categoryID != "" {
		if _, err := s.ownCategory(ctx, userID, categoryID); err != nil {
			return 0, err
		}
	}
	return s.repo().SetTransactionsCategory(ctx, userID, ids, categoryID)
}

func (s *Service) ExportTransactions(ctx context.Context, userID string, f domain.TransactionFilter) ([]domain.Transaction, error) {
	if err := s.checkFilter(&f); err != nil {
		return nil, err
	}
	n, err := s.repo().CountTransactions(ctx, userID, f)
	if err != nil {
		return nil, err
	}
	if n > exportCap {
		return nil, apperror.New(apperror.CodeInvalidInput, "more than 10000 rows: narrow the period").WithField("from")
	}
	return s.repo().ExportTransactions(ctx, userID, f, exportCap)
}

// ---- budgets ----

// carryOver moves last month's unspent limit (or overspend) into this month's
// limit of the expense categories with rollover. Limit becomes the limit in
// force and Carry says how much of it came from last month.
// shortcut: one month back only, against today's limit; a chain of months needs stored limits per month.
func carryOver(list []domain.CategoryTotal, prev []domain.CategoryTotal) {
	spent := map[string]domain.Money{}
	for _, t := range prev {
		if t.CategoryID != "" {
			spent[t.CategoryID] += t.Total
		}
	}
	for i, c := range list {
		if !c.Rollover || c.Limit <= 0 || c.Kind != domain.KindExpense {
			continue
		}
		carry := c.Limit + spent[c.CategoryID] // spending is negative
		list[i].Carry = carry
		list[i].Limit = max(0, c.Limit+carry)
	}
}

func anyRollover(cats []domain.Category) bool {
	return slices.ContainsFunc(cats, func(c domain.Category) bool { return c.Rollover && c.Limit > 0 })
}

// ---- net worth ----

func (s *Service) netWorthToday(ctx context.Context, userID string, now time.Time) (domain.NetWorthPoint, error) {
	p := domain.NetWorthPoint{Day: monthDay(now)}
	accounts, err := s.repo().Accounts(ctx, userID)
	if err != nil {
		return p, err
	}
	for _, a := range accounts {
		if a.Type == domain.AccountCredit {
			p.Cards += a.Balance
		} else {
			p.Banks += a.Balance
		}
	}
	investments, err := s.repo().Investments(ctx, userID)
	if err != nil {
		return p, err
	}
	for _, i := range investments {
		p.Investments += i.Balance
	}
	return p, nil
}

// snapshotNetWorth writes today's net worth; the hourly alerts keep the history going.
func (s *Service) snapshotNetWorth(ctx context.Context, userID string, now time.Time) error {
	p, err := s.netWorthToday(ctx, userID, now)
	if err != nil {
		return err
	}
	return s.repo().SaveNetWorth(ctx, userID, p)
}

func (s *Service) NetWorth(ctx context.Context, userID string, months int) ([]domain.NetWorthPoint, error) {
	if months < 1 || months > maxNetMonths {
		months = 12
	}
	now := s.Clock.Now()
	if err := s.snapshotNetWorth(ctx, userID, now); err != nil {
		return nil, err
	}
	return s.repo().NetWorth(ctx, userID, monthStart(now).AddDate(0, -months, 0))
}

// ---- cards ----

func (s *Service) Cards(ctx context.Context, userID string) (*domain.Cards, error) {
	accounts, err := s.repo().Accounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	from := monthStart(now).AddDate(0, -cardMonths, 0)
	rows, err := s.repo().CardTransactions(ctx, userID, from, cardRowsCap)
	if err != nil {
		return nil, err
	}
	return cardBills(accounts, rows, monthStart(now)), nil
}

// closeOf is the closing day of the bill that holds a charge on day: the card's
// closing day on or after it, or the month's last day when the bank gave none.
func closeOf(day time.Time, closeDay int) time.Time {
	last := func(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }
	y, m := day.Year(), day.Month()
	if closeDay <= 0 {
		return time.Date(y, m, last(y, m), 0, 0, 0, 0, time.UTC)
	}
	if day.Day() > min(closeDay, last(y, m)) {
		m++
	}
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	return time.Date(first.Year(), first.Month(), min(closeDay, last(first.Year(), first.Month())), 0, 0, 0, 0, time.UTC)
}

// cardBills groups card rows (newest first) into bills by closing day, and
// spreads the installments still to come over the months from thisMonth on.
func cardBills(accounts []domain.Account, rows []domain.Transaction, thisMonth time.Time) *domain.Cards {
	type card struct {
		closeDay, dueDay int
	}
	cards := map[string]card{}
	for _, a := range accounts {
		if a.Type != domain.AccountCredit {
			continue
		}
		var c card
		if a.BillCloseDate != nil {
			c.closeDay = a.BillCloseDate.Day()
		}
		if a.BillDueDate != nil {
			c.dueDay = a.BillDueDate.Day()
		}
		cards[a.ID] = c
	}
	out := &domain.Cards{Bills: []domain.CardBill{}, Future: []domain.MonthAmount{}}
	index := map[string]int{}
	future := map[time.Time]domain.Money{}
	seen := map[string]bool{}
	for _, t := range rows {
		c := cards[t.AccountID]
		close := closeOf(t.Date, c.closeDay)
		key := t.AccountID + close.Format(time.DateOnly)
		i, ok := index[key]
		if !ok {
			b := domain.CardBill{AccountID: t.AccountID, AccountName: t.AccountName, Close: close}
			if c.dueDay > 0 {
				due := time.Date(close.Year(), close.Month(), c.dueDay, 0, 0, 0, 0, time.UTC)
				if c.dueDay <= close.Day() {
					due = due.AddDate(0, 1, 0)
				}
				b.Due = &due
			}
			out.Bills = append(out.Bills, b)
			i = len(out.Bills) - 1
			index[key] = i
		}
		b := &out.Bills[i]
		b.Count++
		if t.Amount < 0 {
			b.Charges += t.Amount
		} else {
			b.Credits += t.Amount
		}

		// The newest row of a purchase in installments says how many are left.
		if t.Amount >= 0 || t.Installments < 2 || t.Installment < 1 || t.Installment >= t.Installments {
			continue
		}
		buy := fmt.Sprintf("%s|%s|%d|%d", t.AccountID, merchantKey(t.Description), t.Amount, t.Installments)
		if seen[buy] {
			continue
		}
		seen[buy] = true
		for k := 1; k <= t.Installments-t.Installment; k++ {
			if m := monthStart(t.Date).AddDate(0, k, 0); !m.Before(thisMonth) {
				future[m] += t.Amount
			}
		}
	}
	slices.SortStableFunc(out.Bills, func(a, b domain.CardBill) int {
		if c := b.Close.Compare(a.Close); c != 0 {
			return c
		}
		return strings.Compare(a.AccountName, b.AccountName)
	})
	for m, v := range future {
		out.Future = append(out.Future, domain.MonthAmount{Month: m, Amount: v})
	}
	slices.SortFunc(out.Future, func(a, b domain.MonthAmount) int { return a.Month.Compare(b.Month) })
	return out
}

// ---- debts ----

func checkDebt(d *domain.Debt) error {
	var err error
	if d.Person, err = shortText("person", d.Person, 60); err != nil {
		return err
	}
	d.Description = strings.Join(strings.Fields(d.Description), " ")
	if utf8.RuneCountInString(d.Description) > 120 {
		return apperror.New(apperror.CodeInvalidInput, "description must have up to 120 characters").WithField("description")
	}
	if d.Amount == 0 || abs(d.Amount) > 1_000_000_000_00 {
		return apperror.New(apperror.CodeInvalidInput, "invalid amount").WithField("amount")
	}
	return nil
}

func (s *Service) Debts(ctx context.Context, userID string) ([]domain.Debt, error) {
	return s.repo().Debts(ctx, userID)
}

func (s *Service) CreateDebt(ctx context.Context, userID string, d domain.Debt) (*domain.Debt, error) {
	if err := checkDebt(&d); err != nil {
		return nil, err
	}
	if err := s.repo().CreateDebt(ctx, userID, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Service) UpdateDebt(ctx context.Context, userID, id string, d domain.Debt) (*domain.Debt, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	if err := checkDebt(&d); err != nil {
		return nil, err
	}
	d.ID = id
	if err := s.repo().UpdateDebt(ctx, userID, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Service) DeleteDebt(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().DeleteDebt(ctx, userID, id)
}
