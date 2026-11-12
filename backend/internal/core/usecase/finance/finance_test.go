package finance

import (
	"testing"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

func day(s string) time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return t
}

func TestNormalizeFlipsCardAmounts(t *testing.T) {
	charge := normalize(domain.OFTransaction{ID: "1", Amount: 5990, Description: "  NETFLIX   COM "}, domain.AccountCredit)
	if charge.Amount != -5990 || charge.Description != "NETFLIX COM" {
		t.Fatalf("card charge: %+v", charge)
	}
	debit := normalize(domain.OFTransaction{ID: "2", Amount: -2350}, domain.AccountBank)
	if debit.Amount != -2350 {
		t.Fatalf("bank debit keeps its sign: %+v", debit)
	}
}

func TestCategorizer(t *testing.T) {
	cats := []domain.Category{
		{ID: "c-ifood", Name: "Delivery", Kind: domain.KindExpense},
		{ID: "c-uber", Name: "Transporte", Kind: domain.KindExpense},
		{ID: "c-own", Name: catOwnAccounts, Kind: domain.KindTransfer},
		{ID: "c-pix", Name: catTransfers, Kind: domain.KindExpense},
		{ID: "c-in", Name: catOtherIncome, Kind: domain.KindIncome},
		{ID: "c-mine", Name: "Comida", Kind: domain.KindExpense},
		{ID: "c-salary", Name: catSalary, Kind: domain.KindIncome},
		{ID: "c-yield", Name: catYield, Kind: domain.KindIncome},
		{ID: "c-loan", Name: catLoans, Kind: domain.KindTransfer},
		{ID: "c-interest", Name: catInterest, Kind: domain.KindExpense},
		{ID: "c-subs", Name: "Assinaturas", Kind: domain.KindExpense},
		{ID: "c-ins", Name: "Seguros", Kind: domain.KindExpense},
	}
	rules := []domain.Rule{{Keyword: "uber", CategoryID: "c-uber"}, {Keyword: "uber eats", CategoryID: "c-ifood"}}
	c := newCategorizer(cats, rules)
	cases := []struct {
		tx     domain.Transaction
		want   string
		source string
	}{
		{domain.Transaction{Description: "UBER EATS *PEDIDO", Amount: -3000}, "c-ifood", domain.SourceRule}, // longest rule wins
		{domain.Transaction{Description: "UBER *TRIP", Amount: -2000}, "c-uber", domain.SourceRule},
		{domain.Transaction{Description: "PAGAMENTO FATURA CARTAO", Amount: -150000}, "c-own", domain.SourceAuto},
		{domain.Transaction{Description: "Pagamento recebido", Amount: 150000}, "c-own", domain.SourceAuto},
		{domain.Transaction{Description: "PIX JOAO", ProviderCategory: "Transfer - PIX", Amount: -5000}, "c-pix", domain.SourceAuto},
		{domain.Transaction{Description: "PIX MARIA", ProviderCategory: "Transfer - PIX", Amount: 5000}, "c-in", domain.SourceAuto},
		{domain.Transaction{Description: "PADARIA", ProviderCategory: "Bakery", Amount: -900}, "", domain.SourceAuto},
		// Real cases a bank labeled wrong.
		{domain.Transaction{Description: "SALÁRIO EMPRESA X", ProviderCategory: "Services", Amount: 300000}, "c-salary", domain.SourceAuto},
		{domain.Transaction{Description: "RENDIMENTOS REND PAGO APLIC", ProviderCategory: "Proceeds interests and dividends", Amount: 5}, "c-yield", domain.SourceAuto},
		{domain.Transaction{Description: "Saída juros limite da conta", ProviderCategory: "Interests charged", Amount: -1000}, "c-interest", domain.SourceAuto},
		{domain.Transaction{Description: "Débito automático Fatura Itau Mc", ProviderCategory: "Transfer - Internal", Amount: -36200}, "c-own", domain.SourceAuto},
		{domain.Transaction{Description: "Pagamento debito automatico", ProviderCategory: "Transfer - Internal", Amount: 36200}, "c-own", domain.SourceAuto},
		{domain.Transaction{Description: "Entrada CREDITO CONSIGNADO", Amount: 350000}, "c-loan", domain.SourceAuto},
		{domain.Transaction{Description: "APPLE.COM/BILL", ProviderCategory: "Digital services", Amount: -1990}, "c-subs", domain.SourceAuto},
		{domain.Transaction{Description: "PAGAMENTO DE SEGURO", ProviderCategory: "Insurance", Amount: -1370}, "c-ins", domain.SourceAuto},
	}
	for _, tc := range cases {
		got := c.apply(tc.tx)
		if got.CategoryID != tc.want || got.CategorySource != tc.source {
			t.Errorf("%q: got %q/%q, want %q/%q", tc.tx.Description, got.CategoryID, got.CategorySource, tc.want, tc.source)
		}
	}
}

func TestCombineLeavesTransfersOut(t *testing.T) {
	cats := []domain.Category{
		{ID: "food", Name: "Mercado", Kind: domain.KindExpense, Limit: 100000},
		{ID: "salary", Name: catSalary, Kind: domain.KindIncome},
		{ID: "own", Name: catOwnAccounts, Kind: domain.KindTransfer},
	}
	totals := []domain.CategoryTotal{
		{CategoryID: "food", Total: -80000},
		{CategoryID: "salary", Total: 500000},
		{CategoryID: "own", Total: -300000},
		{Total: -1000}, {Total: 2000}, // uncategorized out and in
	}
	income, expense, list := combine(cats, totals)
	if income != 502000 || expense != -81000 || len(list) != 5 || list[0].Limit != 100000 {
		t.Fatalf("income %d expense %d list %+v", income, expense, list)
	}
}

func TestRecurring(t *testing.T) {
	spending := []domain.Transaction{
		{Description: "NETFLIX.COM 0811", Amount: -5990, Date: day("2026-11-08")},
		{Description: "MERCADO BOM", Amount: -20000, Date: day("2026-11-07")},
		{Description: "MERCADO BOM", Amount: -15000, Date: day("2026-11-03")},
		{Description: "NETFLIX.COM 0810", Amount: -4490, Date: day("2026-10-08")},
		{Description: "MERCADO BOM", Amount: -18000, Date: day("2026-10-05")},
		{Description: "MERCADO BOM", Amount: -18000, Date: day("2026-10-01")},
		{Description: "NETFLIX.COM 0809", Amount: -4490, Date: day("2026-09-08")},
		{Description: "MERCADO BOM", Amount: -18000, Date: day("2026-09-02")},
		{Description: "LOJA X 3/10", Amount: -10000, Date: day("2026-11-02"), Installment: 3, Installments: 10},
	}
	got := recurring(spending)
	if len(got) != 1 || got[0].Name != "NETFLIX.COM 0811" || got[0].Amount != -4490 || !got[0].Changed || got[0].Previous != -4490 {
		t.Fatalf("recurring: %+v", got)
	}
	inst := installments(spending, day("2026-11-01"), day("2026-12-01"))
	if len(inst) != 1 || inst[0].Remaining != 7 {
		t.Fatalf("installments: %+v", inst)
	}
}

func TestBRL(t *testing.T) {
	for v, want := range map[domain.Money]string{0: "R$ 0,00", 5: "R$ 0,05", 123456789: "R$ 1.234.567,89", -150000: "-R$ 1.500,00"} {
		if got := brl(v); got != want {
			t.Errorf("brl(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestConnStatus(t *testing.T) {
	for in, want := range map[string]string{"UPDATED": domain.ConnOK, "UPDATING": domain.ConnSyncing, "LOGIN_ERROR": domain.ConnLoginRequired, "OUTDATED": domain.ConnError} {
		if got := connStatus(in); got != want {
			t.Errorf("%s: got %s", in, got)
		}
	}
}

func TestSuggestKeyword(t *testing.T) {
	cases := map[string]string{
		"Pagamento de Pix QR Code Paymentwall, Inc.": "paymentwall",
		"IFOOD *PEDIDO 123":                          "ifood",
		"APPLE.COM/BILLSAO PAULOBRA":                 "apple.com",
		"Pagamento de boleto LLZ SOLUCAO COBRANCA":   "llz solucao",
		"Débito automático DA TIM CELU 3634997":      "da tim",
		"123 LOJA":    "",
		"Pix enviado": "",
	}
	for desc, want := range cases {
		if got := suggestKeyword(desc); got != want {
			t.Errorf("%q: got %q, want %q", desc, got, want)
		}
	}
}

func TestPersonOf(t *testing.T) {
	name, keyword, ok := personOf("PIX RECEBIDO MARIA SILVA 12/11")
	if !ok || name != "Maria Silva" || keyword != "pix recebido maria silva" {
		t.Fatalf("received: %q %q %v", name, keyword, ok)
	}
	name, keyword, ok = personOf("Transferência Recebida|JOAO DE SOUZA")
	if !ok || name != "Joao De Souza" || keyword != "transferência recebida|joao de souza" {
		t.Fatalf("pipe: %q %q %v", name, keyword, ok)
	}
	if _, keyword, ok = personOf("PIX ENVIADO ANA"); !ok || keyword != "" {
		t.Fatalf("a sent transfer gives no refund keyword: %q %v", keyword, ok)
	}
	if _, _, ok = personOf("PIX RECEBIDO PIX RECEBIDO"); ok {
		t.Fatal("a description with no name names nobody")
	}
	if _, _, ok = personOf("Pagamento de Pix QR Code LOJA"); ok {
		t.Fatal("a QR code payment is a merchant, not a person")
	}
}

func TestSpendingAlerts(t *testing.T) {
	today := day("2026-11-12")
	spending := []domain.Transaction{ // newest first
		{ID: "d1", AccountID: "a", Date: today, Description: "Paymentwall", Amount: -4849},
		{ID: "d2", AccountID: "a", Date: today, Description: "Paymentwall", Amount: -4849},
		{ID: "j1", AccountID: "a", Date: today, Description: "Juros limite", Amount: -1000, CategoryName: catInterest},
		{ID: "s1", AccountID: "a", Date: day("2026-11-10"), Description: "STREAMING", Amount: -5990},
		{ID: "s2", AccountID: "a", Date: day("2026-10-10"), Description: "STREAMING", Amount: -3990},
		{ID: "s3", AccountID: "a", Date: day("2026-09-10"), Description: "STREAMING", Amount: -3990},
		{ID: "old", AccountID: "a", Date: day("2026-09-01"), Description: "Juros limite", Amount: -900, CategoryName: catInterest},
	}
	kinds := map[string]int{}
	for _, a := range spendingAlerts(spending, today) {
		kinds[a.kind]++
	}
	if kinds[NotifyDuplicate] != 1 || kinds[NotifyOverdraft] != 1 || kinds[NotifyPrice] != 1 {
		t.Fatalf("alerts: %v", kinds)
	}
}

func TestForecastAndNextBill(t *testing.T) {
	now := day("2026-11-12")
	sum := &domain.MonthSummary{Income: 100000, Expense: -40000, Recurring: []domain.Recurring{
		{Amount: -5000, LastDate: day("2026-10-20")}, // not charged yet this month
		{Amount: -3000, LastDate: day("2026-11-02")},
	}}
	f := forecast(now, sum, 100000, 300000)
	if f.DaysLeft != 18 || f.PendingBills != -5000 || f.ExpectedIncome != 200000 || f.Projected != 100000-40000+200000-5000 {
		t.Fatalf("forecast: %+v", f)
	}
	due := day("2026-11-06")
	if got := (domain.Account{BillDueDate: &due}).NextBillDue(now); got.Format(time.DateOnly) != "2026-12-06" {
		t.Fatalf("a past due date rolls to the next month: %v", got)
	}
}
