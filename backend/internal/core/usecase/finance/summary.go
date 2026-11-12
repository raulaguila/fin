package finance

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

const (
	recentCount     = 8
	recurringMonths = 4    // window the recurring detection looks at
	recurringMin    = 3    // months a charge must appear in
	spendingCap     = 5000 // rows read for recurring detection
)

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Summary is the dashboard of month: totals, budgets, accounts, recurring charges.
func (s *Service) Summary(ctx context.Context, userID string, month time.Time) (*domain.MonthSummary, error) {
	if month.IsZero() {
		month = s.Clock.Now()
	}
	from := monthStart(month)
	to := from.AddDate(0, 1, 0)

	cats, err := s.Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	totals, err := s.repo().Totals(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	prevTotals, err := s.repo().Totals(ctx, userID, from.AddDate(0, -1, 0), from)
	if err != nil {
		return nil, err
	}
	conns, err := s.repo().Connections(ctx, userID)
	if err != nil {
		return nil, err
	}
	recent, _, err := s.repo().Transactions(ctx, userID, domain.TransactionFilter{From: from, To: to, Page: domain.Page{Number: 1, Limit: recentCount}})
	if err != nil {
		return nil, err
	}
	spending, err := s.repo().Spending(ctx, userID, from.AddDate(0, 1-recurringMonths, 0), to, spendingCap)
	if err != nil {
		return nil, err
	}
	investments, err := s.repo().Investments(ctx, userID)
	if err != nil {
		return nil, err
	}

	sum := &domain.MonthSummary{
		Month: from, Previous: map[string]domain.Money{}, Institutions: map[string]string{},
		Recent: recent, Recurring: recurring(spending), Installments: installments(spending, from, to), Investments: investments,
	}
	sum.Income, sum.Expense, sum.Categories = combine(cats, totals)
	for _, t := range prevTotals {
		if t.CategoryID != "" || t.Total < 0 { // uncategorized inflows stay out of the comparison
			sum.Previous[t.CategoryID] += t.Total
		}
	}
	for _, c := range conns {
		sum.Institutions[c.ID] = c.Institution
		sum.Accounts = append(sum.Accounts, c.Accounts...)
	}
	if now := s.Clock.Now(); monthStart(now).Equal(from) {
		sum.Forecast = forecast(now, sum, categoryTotal(totals, cats, catSalary), categoryTotal(prevTotals, cats, catSalary))
	}
	return sum, nil
}

// combine lists every category with its total, uncategorized money last, and
// sums income and expense. Transfers between own accounts count in neither.
func combine(cats []domain.Category, totals []domain.CategoryTotal) (income, expense domain.Money, out []domain.CategoryTotal) {
	byID := map[string]domain.CategoryTotal{}
	var loose []domain.CategoryTotal
	for _, t := range totals {
		if t.CategoryID == "" {
			loose = append(loose, t)
			continue
		}
		byID[t.CategoryID] = t
	}
	for _, c := range cats {
		t := byID[c.ID]
		t.CategoryID, t.Name, t.Kind, t.Limit = c.ID, c.Name, c.Kind, c.Limit
		out = append(out, t)
		switch c.Kind {
		case domain.KindIncome:
			income += t.Total
		case domain.KindExpense:
			expense += t.Total
		}
	}
	for _, t := range loose {
		if t.Total > 0 {
			t.Kind = domain.KindIncome
			income += t.Total
		} else {
			t.Kind = domain.KindExpense
			expense += t.Total
		}
		out = append(out, t)
	}
	return income, expense, out
}

var (
	noiseDigits = regexp.MustCompile(`[0-9]+`)
	noiseMarks  = regexp.MustCompile(`[^\p{L} ]+`)
)

// merchantKey groups descriptions of the same merchant: "NETFLIX.COM 0412" and
// "Netflix.com 0511" share one key.
func merchantKey(desc string) string {
	s := strings.ToLower(desc)
	s = noiseDigits.ReplaceAllString(s, " ")
	s = noiseMarks.ReplaceAllString(s, " ")
	words := strings.Fields(s)
	if len(words) > 3 {
		words = words[:3]
	}
	return strings.Join(words, " ")
}

// recurring finds charges seen in at least recurringMin of the last months,
// with a typical amount. spending is newest first.
func recurring(spending []domain.Transaction) []domain.Recurring {
	type group struct {
		name   string
		months map[string]bool
		charge []domain.Transaction
	}
	groups := map[string]*group{}
	var order []string
	for _, t := range spending {
		if t.Installments > 1 || t.Amount >= 0 {
			continue // installments have their own list
		}
		key := merchantKey(t.Description)
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &group{name: t.Description, months: map[string]bool{}}
			groups[key] = g
			order = append(order, key)
		}
		g.months[t.Date.Format("2006-01")] = true
		g.charge = append(g.charge, t)
	}
	var out []domain.Recurring
	for _, key := range order {
		g := groups[key]
		// A merchant hit many times a month (a supermarket) is a habit, not a bill.
		if len(g.months) < recurringMin || len(g.charge) > len(g.months)+1 {
			continue
		}
		amounts := make([]domain.Money, 0, len(g.charge))
		for _, t := range g.charge {
			amounts = append(amounts, t.Amount)
		}
		r := domain.Recurring{Name: g.name, Amount: median(amounts), Months: len(g.months), LastDate: g.charge[0].Date, Last: g.charge[0].Amount}
		if len(g.charge) > 1 {
			last, prev := g.charge[0].Amount, g.charge[1].Amount
			if diff := last - prev; prev != 0 && abs(diff)*10 > abs(prev) {
				r.Changed, r.Previous = true, prev
			}
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b domain.Recurring) int { return int(a.Amount - b.Amount) })
	return out
}

// installments lists card installments charged in [from, to) with more to come.
func installments(spending []domain.Transaction, from, to time.Time) []domain.Installment {
	seen := map[string]bool{}
	var out []domain.Installment
	for _, t := range spending {
		if t.Installments < 2 || t.Installment >= t.Installments || t.Date.Before(from) || !t.Date.Before(to) {
			continue
		}
		key := merchantKey(t.Description)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, domain.Installment{Description: t.Description, Amount: t.Amount, Remaining: t.Installments - t.Installment})
	}
	return out
}

func median(v []domain.Money) domain.Money {
	s := slices.Clone(v)
	slices.Sort(s)
	return s[len(s)/2]
}

func abs(v domain.Money) domain.Money {
	if v < 0 {
		return -v
	}
	return v
}
