package finance

import (
	"strings"
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
	for _, company := range []string{"PIX RECEBIDO NAIP INSTITUICAO DE PAGAMENTO SA", "PIX ENVIADO NUUVEM SERVICOS DE GAMES LTDA"} {
		if _, _, ok := personOf(company); ok {
			t.Fatalf("%q is a company, not a person", company)
		}
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
	f := forecast(now, sum, 100000, 300000, nil)
	if f.DaysLeft != 18 || f.PendingBills != -5000 || f.ExpectedIncome != 200000 || f.Projected != 100000-40000+200000-5000 {
		t.Fatalf("forecast: %+v", f)
	}
	due := day("2026-11-06")
	if got := (domain.Account{BillDueDate: &due}).NextBillDue(now); got.Format(time.DateOnly) != "2026-12-06" {
		t.Fatalf("a past due date rolls to the next month: %v", got)
	}
}

func TestCalendar(t *testing.T) {
	today := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 0, 0, 0, 0, time.UTC) }
	due := d(11, 20)
	accounts := []domain.Account{
		{Type: domain.AccountBank, Balance: 100000},
		{Type: domain.AccountCredit, Name: "Gold", Balance: 80000, BillDueDate: &due},
	}
	// Rent paid from the bank three months running; a card subscription stays in the bill.
	var spending []domain.Transaction
	for _, m := range []time.Month{11, 10, 9} {
		spending = append(spending,
			domain.Transaction{Date: d(m, 5), Description: "ALUGUEL", Amount: -150000, AccountType: domain.AccountBank},
			domain.Transaction{Date: d(m, 1), Description: "NETFLIX", Amount: -5590, AccountType: domain.AccountCredit})
	}
	incomes := []domain.Transaction{{Date: d(11, 5), Description: "SALARIO", Amount: 500000}}
	cal := calendar(today, accounts, spending, incomes, nil, nil)
	// Dec 5: salary first, then rent; Nov 20: card bill.
	want := []struct {
		kind    string
		amount  domain.Money
		balance domain.Money
	}{{"bill", -80000, 20000}, {"income", 500000, 520000}, {"recurring", -150000, 370000}}
	if cal.Start != 100000 || len(cal.Events) != len(want) || cal.Lowest != 20000 {
		t.Fatalf("calendar: %+v", cal)
	}
	for i, w := range want {
		if e := cal.Events[i]; e.Kind != w.kind || e.Amount != w.amount || e.Balance != w.balance {
			t.Fatalf("event %d: %+v, want %+v", i, e, w)
		}
	}
}

func TestImportIDRepeatsAreDistinctButStable(t *testing.T) {
	line := domain.Transaction{Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Description: "Café", Amount: -500}
	seen := map[string]int{}
	a, b := importID("acc", line, seen), importID("acc", line, seen)
	if a == b {
		t.Fatal("two equal lines of one file must both be kept")
	}
	if again := importID("acc", line, map[string]int{}); again != a {
		t.Fatal("re-importing the file must give the same ids")
	}
	line.ExternalID = "FIT1"
	if importID("acc", line, seen) != "import:acc:FIT1" {
		t.Fatal("the file's own id wins")
	}
}

func TestBankNameFromAccountCode(t *testing.T) {
	accs := []domain.OFAccount{{Type: domain.AccountCredit}, {BankCode: "0260"}}
	if got := bankName(accs); got != "Nubank" {
		t.Fatalf("bankName = %q, want Nubank", got)
	}
	if got := bankName([]domain.OFAccount{{BankCode: "999"}}); got != "" {
		t.Fatalf("unknown code must give no name, got %q", got)
	}
}

func TestSubscriptionStatus(t *testing.T) {
	today := day("2026-11-12")
	spending := []domain.Transaction{ // newest first
		{Date: day("2026-10-30"), Description: "NETFLIX.COM 1030", Amount: -5990},
		{Date: day("2026-10-08"), Description: "SMARTFIT MENSAL", Amount: -9990},
		{Date: day("2026-01-15"), Description: "Amazon Prime anual", Amount: -16680},
	}
	subs := []domain.Subscription{
		{Name: "Netflix", Keyword: "Netflix.com", Amount: 5590, Day: 1}, // charged Oct 30 for Nov 1
		{Name: "Academia", Keyword: "smartfit", Amount: 9990, Day: 8},   // Nov 8 not charged: late
		{Name: "Prime", Keyword: "amazon prime", Amount: 16680, Day: 15, Month: 1},
		{Name: "Luz", Keyword: "enel", Amount: 20000, Day: 20}, // due later this month
	}
	subscriptionStatus(subs, spending, today)
	check := func(i int, paid, isLate bool, next string) {
		t.Helper()
		if s := subs[i]; s.Paid != paid || s.Late != isLate || s.NextDue.Format(time.DateOnly) != next {
			t.Fatalf("%s: paid %v late %v next %s", s.Name, s.Paid, s.Late, s.NextDue.Format(time.DateOnly))
		}
	}
	check(0, true, false, "2026-12-01")
	check(1, false, true, "2026-11-08")
	check(2, false, false, "2027-01-15") // January's has long passed: waits for next year
	check(3, false, false, "2026-11-20")
	if subs[0].LastAmount != 5990 {
		t.Fatalf("last charge: %d", subs[0].LastAmount)
	}
	if a := subscriptionAlerts(subs, spending, today); len(a) != 1 || a[0].kind != NotifySubscription {
		t.Fatalf("only the gym is late: %+v", a)
	}
	f := forecast(today, &domain.MonthSummary{}, 0, 0, subs)
	if f.PendingBills != -(9990 + 20000) {
		t.Fatalf("pending this month: gym (late) and power: %d", f.PendingBills)
	}

	found := []domain.Recurring{{Key: "netflix com", Name: "NETFLIX.COM"}, {Key: "spotify", Name: "SPOTIFY"}, {Key: "padaria", Name: "PADARIA"}}
	if got := detected(found, subs, []string{"padaria"}); len(got) != 1 || got[0].Key != "spotify" {
		t.Fatalf("registered and dismissed charges leave the detected list: %+v", got)
	}

	// The power bill is due in the window; the gym, late, is expected today; a card one stays in the bill.
	accounts := []domain.Account{{ID: "bank", Type: domain.AccountBank, Balance: 100000}, {ID: "card", Type: domain.AccountCredit}}
	subs[0].LastAccount = "card" // charged on the card last time, no account registered
	cal := calendar(today, accounts, nil, nil, subs, nil)
	var names []string
	for _, e := range cal.Events {
		names = append(names, e.Date.Format("01-02")+" "+e.Name)
	}
	if got := strings.Join(names, ", "); got != "11-12 Academia, 11-20 Luz, 12-08 Academia" {
		t.Fatalf("calendar events: %s", got)
	}
}
