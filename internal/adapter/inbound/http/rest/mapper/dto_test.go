package mapper

import (
	"testing"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

func TestPaginationCountsPartialPages(t *testing.T) {
	for total, want := range map[int64]int64{0: 0, 1: 1, 20: 1, 21: 2, 40: 2} {
		if got := Pagination(domain.NewPage(1, 20), total).TotalPages; got != want {
			t.Errorf("total %d: got %d pages, want %d", total, got, want)
		}
	}
}
