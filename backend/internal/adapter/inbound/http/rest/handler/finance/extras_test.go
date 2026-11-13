package finance

import (
	"strings"
	"testing"
	"time"

	"github.com/raulaguila/fin/internal/core/domain"
)

func TestMoney(t *testing.T) {
	for cents, want := range map[int64]string{0: "0,00", 5: "0,05", -123456: "-1234,56", 100: "1,00"} {
		if got := money(cents); got != want {
			t.Errorf("money(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestTransactionsCSV(t *testing.T) {
	out := string(transactionsCSV([]domain.Transaction{{Date: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
		Description: "=HYPERLINK(1)", Amount: -1050, Note: "a; b", Tags: []string{"x", "y"}}}))
	lines := strings.Split(strings.TrimPrefix(out, "\xef\xbb\xbf"), "\n")
	if lines[1] != `2026-11-03;'=HYPERLINK(1);-10,50;;;;"a; b";x y` {
		t.Errorf("row = %q", lines[1])
	}
}
