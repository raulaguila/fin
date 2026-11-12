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
