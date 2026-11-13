package finance

import (
	"slices"
	"testing"

	"github.com/raulaguila/fin/internal/core/domain"
)

func TestNormTags(t *testing.T) {
	got, err := normTags([]string{" Viagem  SP ", "viagem-sp", "", "Reembolso"})
	if err != nil || !slices.Equal(got, []string{"viagem-sp", "reembolso"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []string{"a,b", "x;y", "trinta-e-um-caracteres-aqui-sim"} {
		if _, err := normTags([]string{bad}); err == nil {
			t.Errorf("%q passed", bad)
		}
	}
	if _, err := normTags(make([]string, 0)); err != nil {
		t.Error(err)
	}
	many := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	if _, err := normTags(many); err == nil {
		t.Error("11 tags passed")
	}
}

func TestCloseOf(t *testing.T) {
	cases := []struct {
		day   string
		close int
		want  string
	}{
		{"2026-11-03", 5, "2026-11-05"},
		{"2026-11-05", 5, "2026-11-05"},
		{"2026-11-06", 5, "2026-12-05"},
		{"2026-12-20", 5, "2027-01-05"},
		{"2026-02-28", 30, "2026-02-28"}, // February has no 30th
		{"2026-01-31", 30, "2026-02-28"},
		{"2026-11-17", 0, "2026-11-30"}, // no closing day: the calendar month
	}
	for _, c := range cases {
		if got := closeOf(day(c.day), c.close).Format("2006-01-02"); got != c.want {
			t.Errorf("closeOf(%s, %d) = %s, want %s", c.day, c.close, got, c.want)
		}
	}
}

func TestCardBills(t *testing.T) {
	closing, due := day("2026-10-05"), day("2026-10-12")
	accounts := []domain.Account{{ID: "c1", Type: domain.AccountCredit, BillCloseDate: &closing, BillDueDate: &due}, {ID: "b1", Type: domain.AccountBank}}
	rows := []domain.Transaction{ // newest first, as the repository gives them
		{AccountID: "c1", AccountName: "Nubank", Date: day("2026-11-10"), Description: "LOJA 3/4", Amount: -100, Installment: 3, Installments: 4},
		{AccountID: "c1", AccountName: "Nubank", Date: day("2026-11-04"), Description: "Mercado", Amount: -50},
		{AccountID: "c1", AccountName: "Nubank", Date: day("2026-11-01"), Description: "Estorno", Amount: 20},
		{AccountID: "c1", AccountName: "Nubank", Date: day("2026-10-10"), Description: "LOJA 2/4", Amount: -100, Installment: 2, Installments: 4},
	}
	got := cardBills(accounts, rows, day("2026-11-01"))
	if len(got.Bills) != 2 {
		t.Fatalf("bills = %+v", got.Bills)
	}
	dec, nov := got.Bills[0], got.Bills[1]
	if dec.Close.Format("2006-01-02") != "2026-12-05" || dec.Charges != -100 || dec.Count != 1 || dec.Due.Format("2006-01-02") != "2026-12-12" {
		t.Errorf("december bill = %+v", dec)
	}
	if nov.Close.Format("2006-01-02") != "2026-11-05" || nov.Charges != -150 || nov.Credits != 20 || nov.Count != 3 {
		t.Errorf("november bill = %+v", nov)
	}
	// The newest row (3/4) leaves one more, in December; 2/4 is the same purchase.
	if len(got.Future) != 1 || got.Future[0].Month.Format("2006-01") != "2026-12" || got.Future[0].Amount != -100 {
		t.Errorf("future = %+v", got.Future)
	}
}

func TestCarryOver(t *testing.T) {
	list := []domain.CategoryTotal{
		{CategoryID: "food", Kind: domain.KindExpense, Limit: 1000, Rollover: true},
		{CategoryID: "fun", Kind: domain.KindExpense, Limit: 500, Rollover: true},
		{CategoryID: "home", Kind: domain.KindExpense, Limit: 800},
	}
	prev := []domain.CategoryTotal{{CategoryID: "food", Total: -700}, {CategoryID: "fun", Total: -1200}, {CategoryID: "home", Total: -100}}
	carryOver(list, prev)
	if list[0].Carry != 300 || list[0].Limit != 1300 {
		t.Errorf("food = %+v", list[0])
	}
	if list[1].Carry != -700 || list[1].Limit != 0 { // overspent past this month's limit
		t.Errorf("fun = %+v", list[1])
	}
	if list[2].Carry != 0 || list[2].Limit != 800 {
		t.Errorf("home = %+v", list[2])
	}
}
