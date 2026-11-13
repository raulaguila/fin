package finance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/apperror"
)

const (
	archiveRows     = 20_000 // transactions per manual account and overrides in one export
	archivePart     = 500    // transactions or overrides per import call: the body limit is small
	archiveSettings = 1_000  // categories, rules, goals, subscriptions or debts per import call
)

func dateText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

func parseDate(field, s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, apperror.New(apperror.CodeInvalidInput, "dates are YYYY-MM-DD").WithField(field)
	}
	return &t, nil
}

// Export gathers everything the user set up into one archive.
func (s *Service) Export(ctx context.Context, userID string) (*domain.Archive, error) {
	a := &domain.Archive{Version: domain.ArchiveVersion, ExportedAt: s.Clock.Now().UTC().Format(time.RFC3339)}
	cats, err := s.repo().Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, c := range cats {
		names[c.ID] = c.Name
		a.Categories = append(a.Categories, domain.ArchiveCategory{Name: c.Name, Kind: c.Kind, Limit: c.Limit, Rollover: c.Rollover})
	}
	rules, err := s.repo().Rules(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		a.Rules = append(a.Rules, domain.ArchiveRule{Keyword: r.Keyword, Category: names[r.CategoryID]})
	}
	goals, err := s.repo().Goals(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, g := range goals {
		a.Goals = append(a.Goals, domain.ArchiveGoal{Name: g.Name, Target: g.Target, Saved: g.Saved, DueDate: dateText(g.DueDate)})
	}
	subs, err := s.repo().Subscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, x := range subs {
		a.Subscriptions = append(a.Subscriptions, domain.ArchiveSubscription{Name: x.Name, Amount: x.Amount, Day: x.Day, Month: x.Month,
			Keyword: x.Keyword, Category: names[x.CategoryID]})
	}
	debts, err := s.repo().Debts(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, d := range debts {
		a.Debts = append(a.Debts, domain.ArchiveDebt{Person: d.Person, Amount: d.Amount, Description: d.Description,
			DueDate: dateText(d.DueDate), SettledOn: dateText(d.SettledOn)})
	}
	conns, err := s.repo().Connections(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range conns {
		if !c.Manual() {
			continue
		}
		for _, acc := range c.Accounts {
			rows, err := s.repo().ExportTransactions(ctx, userID, domain.TransactionFilter{AccountID: acc.ID}, archiveRows)
			if err != nil {
				return nil, err
			}
			out := domain.ArchiveAccount{Institution: c.Institution, Name: acc.Name, Type: acc.Type}
			for i, t := range rows {
				out.Transactions = append(out.Transactions, domain.ArchiveTransaction{Seq: i, Date: t.Date.Format(time.DateOnly),
					Description: t.Description, Amount: t.Amount, Category: t.CategoryName, Note: t.Note, Tags: t.Tags})
			}
			a.Accounts = append(a.Accounts, out)
		}
	}
	if a.Overrides, err = s.repo().ArchiveOverrides(ctx, userID, archiveRows); err != nil {
		return nil, err
	}
	return a, nil
}

// ImportArchive adds what the archive has and the user does not: nothing is
// deleted, and a part sent twice adds nothing twice. The browser sends the
// settings first, then the transactions and overrides in parts.
func (s *Service) ImportArchive(ctx context.Context, userID string, a domain.Archive) (*domain.ArchiveResult, error) {
	if a.Version != domain.ArchiveVersion {
		return nil, apperror.New(apperror.CodeInvalidInput, fmt.Sprintf("this Fin reads archive version %d", domain.ArchiveVersion)).WithField("version")
	}
	if len(a.Categories) > archiveSettings || len(a.Rules) > archiveSettings || len(a.Goals) > archiveSettings ||
		len(a.Subscriptions) > archiveSettings || len(a.Debts) > archiveSettings || len(a.Overrides) > archivePart {
		return nil, apperror.New(apperror.CodeInvalidInput, "archive part too big").WithField("archive")
	}
	if err := s.repo().EnsureCategories(ctx, userID, DefaultCategories); err != nil {
		return nil, err
	}
	res := &domain.ArchiveResult{}
	cats, err := s.importCategories(ctx, userID, a.Categories, res)
	if err != nil {
		return nil, err
	}
	if err := s.importSettings(ctx, userID, a, cats, res); err != nil {
		return nil, err
	}
	for i, acc := range a.Accounts {
		if err := s.importAccount(ctx, userID, i, acc, res); err != nil {
			return nil, err
		}
	}
	if len(a.Overrides) > 0 {
		if res.Overrides, err = s.repo().ApplyOverrides(ctx, userID, a.Overrides); err != nil {
			return nil, err
		}
	}
	if res.Transactions > 0 || res.Rules > 0 {
		if _, err := s.Recategorize(ctx, userID); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// importCategories creates the missing ones and takes limit and rollover from the archive;
// it returns every category by lower-case name.
func (s *Service) importCategories(ctx context.Context, userID string, list []domain.ArchiveCategory, res *domain.ArchiveResult) (map[string]domain.Category, error) {
	have, err := s.repo().Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	byName := map[string]domain.Category{}
	for _, c := range have {
		byName[strings.ToLower(c.Name)] = c
	}
	for _, in := range list {
		c, ok := byName[strings.ToLower(in.Name)]
		if !ok {
			made, err := s.CreateCategory(ctx, userID, domain.Category{Name: in.Name, Kind: in.Kind})
			if err != nil {
				return nil, err
			}
			c = *made
			res.Categories++
		}
		if c.Limit != in.Limit || c.Rollover != in.Rollover {
			limit, rollover := in.Limit, in.Rollover
			updated, err := s.UpdateCategory(ctx, userID, c.ID, nil, &limit, &rollover)
			if err != nil {
				return nil, err
			}
			c = *updated
		}
		byName[strings.ToLower(c.Name)] = c
	}
	return byName, nil
}

func (s *Service) importSettings(ctx context.Context, userID string, a domain.Archive, cats map[string]domain.Category, res *domain.ArchiveResult) error {
	catID := func(name string) string { return cats[strings.ToLower(name)].ID }
	rules, err := s.repo().Rules(ctx, userID)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, r := range rules {
		seen[strings.ToLower(r.Keyword)] = true
	}
	for _, r := range a.Rules {
		if seen[strings.ToLower(r.Keyword)] || catID(r.Category) == "" {
			continue
		}
		if _, _, err := s.CreateRule(ctx, userID, r.Keyword, catID(r.Category)); err != nil {
			return err
		}
		seen[strings.ToLower(r.Keyword)] = true
		res.Rules++
	}

	goals, err := s.repo().Goals(ctx, userID)
	if err != nil {
		return err
	}
	seen = map[string]bool{}
	for _, g := range goals {
		seen[strings.ToLower(g.Name)] = true
	}
	for i, g := range a.Goals {
		if seen[strings.ToLower(g.Name)] {
			continue
		}
		due, err := parseDate(fmt.Sprintf("goals[%d].due_date", i), g.DueDate)
		if err != nil {
			return err
		}
		if _, err := s.CreateGoal(ctx, userID, domain.Goal{Name: g.Name, Target: g.Target, Saved: g.Saved, DueDate: due}); err != nil {
			return err
		}
		seen[strings.ToLower(g.Name)] = true
		res.Goals++
	}

	subs, err := s.repo().Subscriptions(ctx, userID)
	if err != nil {
		return err
	}
	seen = map[string]bool{}
	for _, x := range subs {
		seen[strings.ToLower(x.Name)] = true
	}
	for _, x := range a.Subscriptions {
		if seen[strings.ToLower(x.Name)] {
			continue
		}
		sub := domain.Subscription{Name: x.Name, Amount: x.Amount, Day: x.Day, Month: x.Month, Keyword: x.Keyword, CategoryID: catID(x.Category)}
		if _, err := s.CreateSubscription(ctx, userID, sub); err != nil {
			return err
		}
		seen[strings.ToLower(x.Name)] = true
		res.Subscriptions++
	}

	debts, err := s.repo().Debts(ctx, userID)
	if err != nil {
		return err
	}
	debtKey := func(person string, amount domain.Money, desc string) string {
		return fmt.Sprintf("%s|%d|%s", strings.ToLower(person), amount, strings.ToLower(desc))
	}
	seen = map[string]bool{}
	for _, d := range debts {
		seen[debtKey(d.Person, d.Amount, d.Description)] = true
	}
	for i, d := range a.Debts {
		key := debtKey(d.Person, d.Amount, d.Description)
		if seen[key] {
			continue
		}
		due, err := parseDate(fmt.Sprintf("debts[%d].due_date", i), d.DueDate)
		if err != nil {
			return err
		}
		settled, err := parseDate(fmt.Sprintf("debts[%d].settled_on", i), d.SettledOn)
		if err != nil {
			return err
		}
		if _, err := s.CreateDebt(ctx, userID, domain.Debt{Person: d.Person, Amount: d.Amount, Description: d.Description, DueDate: due, SettledOn: settled}); err != nil {
			return err
		}
		seen[key] = true
		res.Debts++
	}
	return nil
}

// importAccount finds the manual account by institution and name, or makes it,
// and adds the transactions it does not have yet.
func (s *Service) importAccount(ctx context.Context, userID string, i int, in domain.ArchiveAccount, res *domain.ArchiveResult) error {
	if len(in.Transactions) > archivePart {
		return apperror.New(apperror.CodeInvalidInput, "archive part too big").WithField(fmt.Sprintf("accounts[%d].transactions", i))
	}
	conns, err := s.repo().Connections(ctx, userID)
	if err != nil {
		return err
	}
	accountID := ""
	for _, c := range conns {
		for _, acc := range c.Accounts {
			if c.Manual() && strings.EqualFold(c.Institution, in.Institution) && strings.EqualFold(acc.Name, in.Name) {
				accountID = acc.ID
			}
		}
	}
	if accountID == "" {
		acc, err := s.CreateManualAccount(ctx, userID, in.Institution, in.Name, in.Type)
		if err != nil {
			return err
		}
		accountID = acc.ID
		res.Accounts++
	}
	txs := make([]domain.Transaction, 0, len(in.Transactions))
	for j, t := range in.Transactions {
		field := fmt.Sprintf("accounts[%d].transactions[%d]", i, j)
		day, err := parseDate(field, t.Date)
		if err != nil || day == nil || t.Amount == 0 || abs(t.Amount) > maxMoneyCents {
			return apperror.New(apperror.CodeInvalidInput, "each transaction needs a date and a non-zero amount").WithField(field)
		}
		desc, err := shortText(field, t.Description, 200)
		if err != nil {
			return err
		}
		note, err := checkNote(t.Note)
		if err != nil {
			return err
		}
		tags, err := normTags(t.Tags)
		if err != nil {
			return err
		}
		// The position in the exported account makes the id: the same part sent again adds nothing.
		txs = append(txs, domain.Transaction{ExternalID: fmt.Sprintf("archive:%s:%d", accountID, t.Seq), Date: *day, Description: desc,
			Amount: t.Amount, CategoryName: t.Category, Note: note, Tags: tags})
	}
	if len(txs) == 0 {
		return nil
	}
	n, err := s.repo().RestoreTransactions(ctx, userID, accountID, txs)
	res.Transactions += n
	return err
}
