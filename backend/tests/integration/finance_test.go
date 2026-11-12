package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/port/output"
	"github.com/raulaguila/fin/internal/core/usecase/finance"
	"github.com/raulaguila/fin/pkg/apperror"
)

// fakeBank is the Open Finance provider: one item with a checking account and a card.
type fakeBank struct {
	owner  string
	status string
	txs    map[string][]domain.OFTransaction
}

func (f *fakeBank) Enabled() bool { return true }
func (f *fakeBank) ConnectToken(context.Context, string, string) (string, error) {
	return "token", nil
}
func (f *fakeBank) Item(_ context.Context, id string) (*domain.OFItem, error) {
	exp := time.Now().AddDate(0, 0, 10) // inside the 15-day warning
	return &domain.OFItem{ID: id, ClientUserID: f.owner, Institution: "Banco Teste", Status: f.status, ConsentExpiresAt: &exp}, nil
}
func (f *fakeBank) RefreshItem(context.Context, string) error { return nil }
func (f *fakeBank) DeleteItem(context.Context, string) error  { return nil }
func (f *fakeBank) Accounts(context.Context, string) ([]domain.OFAccount, error) {
	limit, avail := domain.Money(500000), domain.Money(380000)
	due := time.Now().AddDate(0, 0, 2)
	return []domain.OFAccount{
		{ID: "acc-bank-" + f.owner, Type: domain.AccountBank, Name: "Conta corrente", Number: "0001", Balance: 845237},
		{ID: "acc-card-" + f.owner, Type: domain.AccountCredit, Name: "Cartão Gold", Balance: 120000, CreditLimit: &limit, Available: &avail, BillDueDate: &due},
	}, nil
}
func (f *fakeBank) LastBill(context.Context, string) (*time.Time, *domain.Money, error) {
	due, amount := time.Now().AddDate(0, 0, -20), domain.Money(98765)
	return &due, &amount, nil
}
func (f *fakeBank) Transactions(_ context.Context, accountID string, _ time.Time) ([]domain.OFTransaction, error) {
	return f.txs[accountID], nil
}

type fakeAI struct{ prompts []string }

func (a *fakeAI) Ask(_ context.Context, _, prompt string) (string, error) {
	a.prompts = append(a.prompts, prompt)
	return "Resposta do modelo.", nil
}

var _ output.OpenFinance = (*fakeBank)(nil)

func TestFinanceSyncSummaryRulesAndAlerts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	userID := e.userID(t, e.activeUser(t))
	now := time.Now().UTC()
	day := func(d int) time.Time {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d)
	}
	prevMonth := day(0).AddDate(0, -1, 0)

	bank := &fakeBank{owner: userID, status: "UPDATING", txs: map[string][]domain.OFTransaction{
		"acc-bank-" + userID: {
			{ID: "t-salary-" + userID, Date: day(0), Description: "SALARIO ACME", Amount: 1000000, Category: "Salary"},
			{ID: "t-uber-" + userID, Date: day(0), Description: "UBER *TRIP", Amount: -3550, Category: "Taxi and ride-hailing"},
			{ID: "t-bill-" + userID, Date: day(0), Description: "PAGAMENTO FATURA CARTAO", Amount: -150000, Category: "Credit card payment"},
			{ID: "t-padaria-" + userID, Date: day(0), Description: "PADARIA SAO JOAO", Amount: -1890, Category: "Bakery"},
			{ID: "t-old-" + userID, Date: prevMonth, Description: "MERCADO BOM", Amount: -20000, Category: "Groceries"},
			{ID: "t-pix1-" + userID, Date: day(0), Description: "PIX ENVIADO ANA LIMA", Amount: -5000, Category: "Transfer - PIX"},
			{ID: "t-pix2-" + userID, Date: day(0), Description: "PIX RECEBIDO ANA LIMA", Amount: 2500, Category: "Transfer - PIX"},
			{ID: "t-self-" + userID, Date: day(0), Description: "PIX ENVIADO EU MESMO", Amount: -9000, Category: "Same person transfer"},
			{ID: "t-pw1-" + userID, Date: prevMonth, Description: "Pagamento de Pix QR Code Paymentwall, Inc.", Amount: -4849, Category: "Entrepreneurial activities"},
			{ID: "t-pw2-" + userID, Date: prevMonth, Description: "Pagamento de Pix QR Code Paymentwall, Inc.", Amount: -4849, Category: "Entrepreneurial activities"},
		},
		"acc-card-" + userID: {
			{ID: "t-ifood-" + userID, Date: day(0), Description: "IFOOD *PEDIDO", Amount: 8900, Category: "Food delivery"},
			{ID: "t-tv-" + userID, Date: day(0), Description: "LOJA TV 2/10", Amount: 25000, Category: "Electronics", Installment: 2, Installments: 10},
		},
	}}
	ai := &fakeAI{}
	svc := finance.New(e.store, e.store, bank, ai, output.RealClock{}, nil)

	// Someone else's item is refused.
	other := e.userID(t, e.activeUser(t))
	if _, err := svc.AddConnection(ctx, other, "11111111-1111-4111-8111-"+userID[24:]); !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("foreign item: %v", err)
	}

	itemID := "22222222-2222-4222-8222-" + userID[24:]
	conn, err := svc.AddConnection(ctx, userID, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if conn.Status != domain.ConnSyncing {
		t.Fatalf("status %s", conn.Status)
	}

	// The queued sync retries while the bank is still updating.
	payload, _ := json.Marshal(map[string]string{"connection_id": conn.ID})
	if err := svc.Jobs()[finance.JobSync](ctx, payload); !errors.Is(err, finance.ErrStillUpdating) {
		t.Fatalf("want still updating, got %v", err)
	}
	bank.status = "UPDATED"
	if err := svc.Jobs()[finance.JobSync](ctx, payload); err != nil {
		t.Fatal(err)
	}
	// A second run changes nothing: everything is an upsert.
	if err := svc.Jobs()[finance.JobSync](ctx, payload); err != nil {
		t.Fatal(err)
	}

	conns, err := svc.Connections(ctx, userID)
	if err != nil || len(conns) != 1 || conns[0].Status != domain.ConnOK || len(conns[0].Accounts) != 2 || conns[0].LastSyncAt == nil {
		t.Fatalf("connections: %+v %v", conns, err)
	}
	if others, _ := svc.Connections(ctx, other); len(others) != 0 {
		t.Fatal("another user sees the connection")
	}

	sum, err := svc.Summary(ctx, userID, now)
	if err != nil {
		t.Fatal(err)
	}
	// Income: salary. Expense: uber, padaria (uncategorized), ifood, tv. The bill payment is a transfer.
	if sum.Income != 1000000+2500 || sum.Expense != -(3550+1890+8900+25000+5000) {
		t.Fatalf("income %d expense %d", sum.Income, sum.Expense)
	}
	if len(sum.Installments) != 1 || sum.Installments[0].Remaining != 8 {
		t.Fatalf("installments %+v", sum.Installments)
	}

	txs, total, err := svc.Transactions(ctx, userID, domain.TransactionFilter{Search: "ifood", Page: domain.NewPage(1, 10)})
	if err != nil || total != 1 || txs[0].Amount != -8900 || txs[0].CategoryName != "Delivery" || txs[0].AccountType != domain.AccountCredit {
		t.Fatalf("ifood: %+v %d %v", txs, total, err)
	}
	if _, total, _ := svc.Transactions(ctx, userID, domain.TransactionFilter{CategoryID: "none", Page: domain.NewPage(1, 10)}); total != 3 {
		t.Fatalf("uncategorized: %d", total)
	}

	// Card bill: the last closed one comes from the provider, and the next due date is ahead.
	for _, a := range conns[0].Accounts {
		if a.Type == domain.AccountCredit && (a.LastBill == nil || *a.LastBill != 98765 || a.NextBillDue(now) == nil) {
			t.Fatalf("card bill: %+v", a)
		}
	}

	// Insights: suggestions, people and the monthly trend.
	sugg, err := svc.RuleSuggestions(ctx, userID)
	if err != nil || len(sugg) != 1 || sugg[0].Keyword != "paymentwall" || sugg[0].Count != 2 {
		t.Fatalf("suggestions: %+v %v", sugg, err)
	}
	people, err := svc.People(ctx, userID)
	if err != nil || len(people) != 1 || people[0].Name != "Ana Lima" || people[0].Sent != -5000 || people[0].Received != 2500 ||
		people[0].Keyword != "pix recebido ana lima" {
		t.Fatalf("people: %+v %v", people, err)
	}
	trend, err := svc.Trend(ctx, userID, 2)
	if err != nil || len(trend) != 2 || trend[1].Income != sum.Income || trend[1].Expense != sum.Expense || trend[0].Expense != -(20000+4849*2) {
		t.Fatalf("trend: %+v %v", trend, err)
	}
	if sum.Forecast == nil {
		t.Fatal("the current month has a forecast")
	}
	if n, err := svc.Recategorize(ctx, userID); err != nil || n != 0 {
		t.Fatalf("recategorize right after a sync changes nothing: %d %v", n, err)
	}

	// A rule moves past transactions, and a later sync keeps it.
	cats, _ := svc.Categories(ctx, userID)
	var food string
	for _, c := range cats {
		if c.Name == "Restaurantes" {
			food = c.ID
		}
	}
	if n, err := svc.PreviewRule(ctx, userID, "padaria"); err != nil || n != 1 {
		t.Fatalf("preview %d %v", n, err)
	}
	rule, applied, err := svc.CreateRule(ctx, userID, "padaria", food)
	if err != nil || applied != 1 || rule.CategoryName != "Restaurantes" {
		t.Fatalf("rule %+v %d %v (%v)", rule, applied, err, errors.Unwrap(err))
	}
	if _, _, err := svc.CreateRule(ctx, other, "x1", food); !apperror.IsNotFound(err) {
		t.Fatalf("rule on someone else's category: %v", err)
	}

	// The user's own choice survives a sync.
	uber, _, _ := svc.Transactions(ctx, userID, domain.TransactionFilter{Search: "uber", Page: domain.NewPage(1, 10)})
	if err := svc.SetCategory(ctx, userID, uber[0].ID, food); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCategory(ctx, other, uber[0].ID, ""); !apperror.IsNotFound(err) {
		t.Fatalf("set category of another user's transaction: %v", err)
	}
	if err := svc.Jobs()[finance.JobSync](ctx, payload); err != nil {
		t.Fatal(err)
	}
	uber, _, _ = svc.Transactions(ctx, userID, domain.TransactionFilter{Search: "uber", Page: domain.NewPage(1, 10)})
	padaria, _, _ := svc.Transactions(ctx, userID, domain.TransactionFilter{Search: "padaria", Page: domain.NewPage(1, 10)})
	if uber[0].CategoryID != food || uber[0].CategorySource != domain.SourceUser || padaria[0].CategorySource != domain.SourceRule {
		t.Fatalf("after resync: uber %+v padaria %+v", uber[0], padaria[0])
	}

	// Budget alert once, at 100%, and the consent and bill alerts.
	limit := domain.Money(1000)
	if _, err := svc.UpdateCategory(ctx, userID, food, nil, &limit); err != nil {
		t.Fatal(err)
	}
	if err := svc.Alerts(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Alerts(ctx, userID); err != nil {
		t.Fatal(err)
	}
	notes, _, err := e.store.Notifications().List(ctx, userID, domain.NewPage(1, 50))
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, n := range notes {
		kinds[n.Kind]++
	}
	if kinds[finance.NotifyBudget] != 1 || kinds[finance.NotifyConsent] != 1 || kinds[finance.NotifyBill] != 1 {
		t.Fatalf("alerts: %v", kinds)
	}

	// The assistant gets the computed facts, never an account number.
	answer, err := svc.Chat(ctx, userID, []domain.ChatMessage{{Role: "user", Content: "Posso gastar R$ 500?"}})
	if err != nil || answer == "" {
		t.Fatalf("chat %q %v", answer, err)
	}
	prompt := ai.prompts[len(ai.prompts)-1]
	if !strings.Contains(prompt, "R$ 10.000,00") || !strings.Contains(prompt, "Cartão Gold") || strings.Contains(prompt, "0001") {
		t.Fatalf("prompt:\n%s", prompt)
	}

	// Deleting the connection removes its accounts and transactions.
	if err := svc.DeleteConnection(ctx, other, conn.ID); !apperror.IsNotFound(err) {
		t.Fatalf("delete by another user: %v", err)
	}
	if err := svc.DeleteConnection(ctx, userID, conn.ID); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := svc.Transactions(ctx, userID, domain.TransactionFilter{Page: domain.NewPage(1, 10)}); total != 0 {
		t.Fatalf("transactions left: %d", total)
	}
}
