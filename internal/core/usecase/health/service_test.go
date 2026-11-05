package health

import (
	"context"
	"testing"
)

type countingPinger struct{ calls int }

func (p *countingPinger) Ping(context.Context) error {
	p.calls++
	return nil
}

func TestRepeatedChecksReuseTheReport(t *testing.T) {
	db := &countingPinger{}
	uc := New(db, nil)
	for range 1000 {
		if _, err := uc.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if db.calls != 1 {
		t.Fatalf("%d database pings", db.calls)
	}
}
