package finance

import (
	"context"
	"slices"
	"strings"

	"github.com/raulaguila/fin/internal/core/domain"
)

// Category names the automatic categorization relies on.
const (
	catOwnAccounts = "Entre minhas contas"
	catInvest      = "Investimentos"
	catSalary      = "Salário"
	catOtherIncome = "Outras receitas"
	catTransfers   = "Pix e transferências"
)

// DefaultCategories is what a new user starts with; every name is editable later.
var DefaultCategories = []domain.Category{
	{Name: "Mercado", Kind: domain.KindExpense},
	{Name: "Delivery", Kind: domain.KindExpense},
	{Name: "Restaurantes", Kind: domain.KindExpense},
	{Name: "Transporte", Kind: domain.KindExpense},
	{Name: "Moradia", Kind: domain.KindExpense},
	{Name: "Contas da casa", Kind: domain.KindExpense},
	{Name: "Saúde", Kind: domain.KindExpense},
	{Name: "Assinaturas", Kind: domain.KindExpense},
	{Name: "Compras", Kind: domain.KindExpense},
	{Name: "Lazer", Kind: domain.KindExpense},
	{Name: "Educação", Kind: domain.KindExpense},
	{Name: "Viagem", Kind: domain.KindExpense},
	{Name: "Impostos e taxas", Kind: domain.KindExpense},
	{Name: catTransfers, Kind: domain.KindExpense},
	{Name: "Outros", Kind: domain.KindExpense},
	{Name: catSalary, Kind: domain.KindIncome},
	{Name: catOtherIncome, Kind: domain.KindIncome},
	{Name: catOwnAccounts, Kind: domain.KindTransfer},
	{Name: catInvest, Kind: domain.KindTransfer},
}

// providerMap maps words of the Pluggy category (English) to a default
// category name. Checked in order: the first match wins.
var providerMap = []struct{ word, category string }{
	{"credit card payment", catOwnAccounts},
	{"same person", catOwnAccounts},
	{"investment", catInvest},
	{"salary", catSalary},
	{"food delivery", "Delivery"},
	{"groceries", "Mercado"},
	{"supermarket", "Mercado"},
	{"eating out", "Restaurantes"},
	{"restaurant", "Restaurantes"},
	{"taxi", "Transporte"},
	{"ride", "Transporte"},
	{"gas station", "Transporte"},
	{"parking", "Transporte"},
	{"public transport", "Transporte"},
	{"transport", "Transporte"},
	{"toll", "Transporte"},
	{"rent", "Moradia"},
	{"housing", "Moradia"},
	{"electricity", "Contas da casa"},
	{"water", "Contas da casa"},
	{"internet", "Contas da casa"},
	{"telecommunication", "Contas da casa"},
	{"mobile", "Contas da casa"},
	{"utilities", "Contas da casa"},
	{"pharmacy", "Saúde"},
	{"health", "Saúde"},
	{"hospital", "Saúde"},
	{"dentist", "Saúde"},
	{"gym", "Saúde"},
	{"streaming", "Assinaturas"},
	{"video", "Assinaturas"},
	{"music", "Assinaturas"},
	{"software", "Assinaturas"},
	{"subscription", "Assinaturas"},
	{"shopping", "Compras"},
	{"clothing", "Compras"},
	{"electronics", "Compras"},
	{"leisure", "Lazer"},
	{"entertainment", "Lazer"},
	{"education", "Educação"},
	{"school", "Educação"},
	{"travel", "Viagem"},
	{"airport", "Viagem"},
	{"accommodation", "Viagem"},
	{"tax", "Impostos e taxas"},
	{"bank fees", "Impostos e taxas"},
	{"interest", "Impostos e taxas"},
	{"transfer", catTransfers},
	{"pix", catTransfers},
}

// ownAccountWords spot a card bill payment by description, on either side.
var ownAccountWords = []string{"pagamento fatura", "pag fatura", "pgto fatura", "pagamento de fatura", "pagamento recebido"}

type categorizer struct {
	rules  []domain.Rule
	byName map[string]domain.Category
	byID   map[string]domain.Category
}

func (s *Service) categorizer(ctx context.Context, userID string) (*categorizer, error) {
	cats, err := s.repo().Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo().Rules(ctx, userID)
	if err != nil {
		return nil, err
	}
	return newCategorizer(cats, rules), nil
}

func newCategorizer(cats []domain.Category, rules []domain.Rule) *categorizer {
	c := &categorizer{byName: map[string]domain.Category{}, byID: map[string]domain.Category{}}
	for _, cat := range cats {
		c.byName[strings.ToLower(cat.Name)] = cat
		c.byID[cat.ID] = cat
	}
	// Longest keyword first: "uber eats" wins over "uber".
	c.rules = slices.Clone(rules)
	slices.SortStableFunc(c.rules, func(a, b domain.Rule) int { return len(b.Keyword) - len(a.Keyword) })
	return c
}

// apply sets the category of t: user rules, then the description of a card bill
// payment, then the provider category. Nothing found leaves it uncategorized.
func (c *categorizer) apply(t domain.Transaction) domain.Transaction {
	desc := strings.ToLower(t.Description)
	for _, r := range c.rules {
		if strings.Contains(desc, strings.ToLower(r.Keyword)) {
			if _, ok := c.byID[r.CategoryID]; ok {
				t.CategoryID, t.CategorySource = r.CategoryID, domain.SourceRule
				return t
			}
		}
	}
	t.CategorySource = domain.SourceAuto
	name := ""
	for _, w := range ownAccountWords {
		if strings.Contains(desc, w) {
			name = catOwnAccounts
			break
		}
	}
	if name == "" {
		provider := strings.ToLower(t.ProviderCategory)
		for _, m := range providerMap {
			if provider != "" && strings.Contains(provider, m.word) {
				name = m.category
				break
			}
		}
	}
	// A transfer received from someone else is income, not a negative expense.
	if name == catTransfers && t.Amount > 0 {
		name = catOtherIncome
	}
	if cat, ok := c.byName[strings.ToLower(name)]; ok && name != "" {
		t.CategoryID = cat.ID
	}
	return t
}
