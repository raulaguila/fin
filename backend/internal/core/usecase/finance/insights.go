package finance

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/raulaguila/fin/internal/core/domain"
)

const (
	recategorizeCap = 20000 // automatic rows read per run
	historyMonths   = 12    // window of suggestions and people
	suggestCap      = 2000  // uncategorized rows read for suggestions
	suggestMax      = 15
	peopleCap       = 5000
	peopleMax       = 100
	trendDefault    = 12
	trendMax        = 24
)

// Recategorize runs the categorizer again over every row the user did not set by
// hand, so a deleted rule or category stops counting.
func (s *Service) Recategorize(ctx context.Context, userID string) (int64, error) {
	cat, err := s.categorizer(ctx, userID)
	if err != nil {
		return 0, err
	}
	// shortcut: reads up to 20 000 automatic rows per run; page it past that.
	txs, err := s.repo().Categorizable(ctx, userID, recategorizeCap)
	if err != nil {
		return 0, err
	}
	var changed []domain.Transaction
	for _, t := range txs {
		if n := cat.apply(t); n.CategoryID != t.CategoryID || n.CategorySource != t.CategorySource {
			changed = append(changed, n)
		}
	}
	if len(changed) == 0 {
		return 0, nil
	}
	return s.repo().SetCategories(ctx, userID, changed)
}

// Trend returns one row per month, months with no movement included as zero.
func (s *Service) Trend(ctx context.Context, userID string, months int) ([]domain.MonthTotal, error) {
	if months < 1 {
		months = trendDefault
	}
	months = min(months, trendMax)
	to := monthStart(s.Clock.Now()).AddDate(0, 1, 0)
	from := to.AddDate(0, -months, 0)
	got, err := s.repo().MonthlyTotals(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	byMonth := map[string]domain.MonthTotal{}
	for _, m := range got {
		byMonth[m.Month.Format("2006-01")] = m
	}
	out := make([]domain.MonthTotal, 0, months)
	for m := from; m.Before(to); m = m.AddDate(0, 1, 0) {
		t := byMonth[m.Format("2006-01")]
		t.Month = m
		out = append(out, t)
	}
	return out, nil
}

// merchantPrefixes are bank words before the merchant or person name.
var merchantPrefixes = []string{
	"pagamento de pix qr code", "pagamento de boleto", "pagamento de pix", "compra debito", "compra débito",
	"compra no debito", "compra no débito", "debito automatico", "débito automático", "pix enviado", "pix recebido",
}

// suggestKeyword picks the merchant out of a description: "Pagamento de Pix QR
// Code Paymentwall, Inc." gives "paymentwall". Empty when nothing usable is left.
func suggestKeyword(desc string) string {
	s := strings.ToLower(strings.Join(strings.Fields(desc), " "))
	for _, p := range merchantPrefixes {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(s[len(p):])
			break
		}
	}
	words := strings.Fields(s)
	if len(words) == 0 || strings.IndexFunc(words[0], unicode.IsDigit) >= 0 {
		return ""
	}
	// Cut at a separator so "ifood*pedido" and "apple.com/bill" match every variant.
	first := strings.FieldsFunc(words[0], func(r rune) bool { return r == '*' || r == '/' || r == ',' })
	if len(first) == 0 {
		return ""
	}
	k := strings.Trim(first[0], ".-:;")
	if utf8.RuneCountInString(k) < 4 && len(words) > 1 && first[0] == words[0] && strings.IndexFunc(words[1], unicode.IsDigit) < 0 {
		k = words[0] + " " + strings.TrimRight(words[1], ",.;:*")
	}
	if n := utf8.RuneCountInString(k); n < 3 || n > 60 {
		return ""
	}
	return k
}

// RuleSuggestions groups the uncategorized transactions of the last year by merchant.
func (s *Service) RuleSuggestions(ctx context.Context, userID string) ([]domain.RuleSuggestion, error) {
	from := monthStart(s.Clock.Now()).AddDate(0, 1-historyMonths, 0)
	txs, err := s.repo().Uncategorized(ctx, userID, from, suggestCap)
	if err != nil {
		return nil, err
	}
	groups := map[string]*domain.RuleSuggestion{}
	var order []string
	for _, t := range txs {
		k := suggestKeyword(t.Description)
		if k == "" {
			continue
		}
		g, ok := groups[k]
		if !ok {
			g = &domain.RuleSuggestion{Keyword: k, Example: t.Description}
			groups[k] = g
			order = append(order, k)
		}
		g.Count++
		g.Total += t.Amount
	}
	var out []domain.RuleSuggestion
	for _, k := range order {
		if g := groups[k]; g.Count >= 2 {
			out = append(out, *g)
		}
	}
	slices.SortStableFunc(out, func(a, b domain.RuleSuggestion) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return int(abs(b.Total) - abs(a.Total))
	})
	return out[:min(len(out), suggestMax)], nil
}

// personPrefixes start a transfer to or from a person, received ones marked.
var personPrefixes = []struct {
	word     string
	received bool
}{
	{"pix enviado", false}, {"pix recebido", true}, {"ted enviada", false}, {"ted recebida", true},
	{"transferência enviada", false}, {"transferencia enviada", false},
	{"transferência recebida", true}, {"transferencia recebida", true},
}

// personOf reads the person out of a transfer description. keyword is the
// description up to the end of the name, to match later transfers by rule.
func personOf(desc string) (name, keyword string, ok bool) {
	for _, p := range personPrefixes {
		if len(desc) <= len(p.word) || !strings.EqualFold(desc[:len(p.word)], p.word) {
			continue
		}
		rest := strings.TrimLeft(desc[len(p.word):], " |-:")
		if cut := strings.IndexFunc(rest, unicode.IsDigit); cut >= 0 {
			rest = rest[:cut]
		}
		rest = strings.TrimSpace(rest)
		if utf8.RuneCountInString(rest) < 3 {
			return "", "", false
		}
		for _, q := range personPrefixes { // "PIX RECEBIDO PIX RECEBIDO" names nobody
			if len(rest) >= len(q.word) && strings.EqualFold(rest[:len(q.word)], q.word) {
				return "", "", false
			}
		}
		end := strings.Index(desc, rest) + len(rest)
		keyword = strings.ToLower(desc[:end])
		if !p.received || utf8.RuneCountInString(keyword) > 60 {
			keyword = ""
		}
		return titleCase(rest), keyword, true
	}
	return "", "", false
}

func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r, size := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[size:]
	}
	return strings.Join(words, " ")
}

// People groups transfers by person; money between the user's own accounts stays out.
func (s *Service) People(ctx context.Context, userID string) ([]domain.Person, error) {
	from := monthStart(s.Clock.Now()).AddDate(0, 1-historyMonths, 0)
	txs, err := s.repo().PersonTransfers(ctx, userID, from, peopleCap)
	if err != nil {
		return nil, err
	}
	groups := map[string]*domain.Person{}
	var order []string
	for _, t := range txs {
		if t.CategoryKind == domain.KindTransfer || strings.Contains(strings.ToLower(t.ProviderCategory), "same person") {
			continue
		}
		name, keyword, ok := personOf(t.Description)
		if !ok {
			continue
		}
		key := strings.ToLower(name)
		p, found := groups[key]
		if !found {
			p = &domain.Person{Name: name, LastDate: t.Date} // newest first
			groups[key] = p
			order = append(order, key)
		}
		if p.Keyword == "" {
			p.Keyword = keyword
		}
		p.Count++
		switch {
		case t.Amount < 0:
			p.Sent += t.Amount
		case t.CategoryName == catRefunds:
			p.Refunded += t.Amount
		default:
			p.Received += t.Amount
		}
	}
	out := make([]domain.Person, 0, len(order))
	for _, k := range order {
		out = append(out, *groups[k])
	}
	slices.SortStableFunc(out, func(a, b domain.Person) int {
		return int((abs(b.Sent) + b.Received + b.Refunded) - (abs(a.Sent) + a.Received + a.Refunded))
	})
	return out[:min(len(out), peopleMax)], nil
}

// forecast projects the end of the current month: what came in and went out,
// plus recurring charges not seen yet and the salary still to come (as much as
// last month's, less what already arrived).
func forecast(now time.Time, sum *domain.MonthSummary, salary, salaryBefore domain.Money) *domain.Forecast {
	from := monthStart(now)
	last := from.AddDate(0, 1, -1)
	f := &domain.Forecast{DaysLeft: last.Day() - now.Day(), ExpectedIncome: max(0, salaryBefore-salary)}
	for _, r := range sum.Recurring {
		if r.LastDate.Before(from) {
			f.PendingBills += r.Amount
		}
	}
	f.Projected = sum.Income + sum.Expense + f.ExpectedIncome + f.PendingBills
	return f
}

// categoryTotal sums the totals of the category named name.
func categoryTotal(totals []domain.CategoryTotal, cats []domain.Category, name string) domain.Money {
	for _, c := range cats {
		if strings.EqualFold(c.Name, name) {
			var sum domain.Money
			for _, t := range totals {
				if t.CategoryID == c.ID {
					sum += t.Total
				}
			}
			return sum
		}
	}
	return 0
}

// recentWindow keeps the transactions of the last days (spending is newest first).
func recentWindow(spending []domain.Transaction, since time.Time) []domain.Transaction {
	for i, t := range spending {
		if t.Date.Before(since) {
			return spending[:i]
		}
	}
	return spending
}

// duplicates finds the same charge (account, day, amount, merchant) more than once.
func duplicates(recent []domain.Transaction) [][]domain.Transaction {
	groups := map[string][]domain.Transaction{}
	var order []string
	for _, t := range recent {
		key := t.AccountID + "|" + t.Date.Format(time.DateOnly) + "|" + brl(t.Amount) + "|" + merchantKey(t.Description)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], t)
	}
	var out [][]domain.Transaction
	for _, k := range order {
		if len(groups[k]) > 1 {
			out = append(out, groups[k])
		}
	}
	return out
}
