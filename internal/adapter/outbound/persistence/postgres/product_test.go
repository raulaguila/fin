package postgres_test

import (
	"context"
	"testing"

	productrepo "github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/product"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/usecase/product"
	"github.com/raulaguila/authenticity/pkg/apperror"
)

func TestProductListPagesSearchesAndSorts(t *testing.T) {
	if err := testDB.Exec("DELETE FROM app.product").Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	svc := product.New(productrepo.NewRepository(testDB))
	for _, p := range []struct {
		name, price string
		enabled     bool
	}{{"100% cotton", "3", true}, {"Cotton", "1", true}, {"Wool", "2", true}, {"Hidden", "4", false}} {
		enabled := p.enabled
		if _, err := svc.Create(ctx, input.CreateProductCommand{Name: p.name, Price: p.price, Enabled: &enabled}); err != nil {
			t.Fatal(err)
		}
	}
	names := func(f domain.ProductFilter) ([]string, int64) {
		t.Helper()
		items, total, err := svc.List(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(items))
		for _, p := range items {
			out = append(out, p.Name)
		}
		return out, total
	}
	all := domain.NewPage(1, 100)

	if got, total := names(domain.ProductFilter{Page: domain.NewPage(2, 3), Sort: "price", Order: "asc"}); total != 4 || len(got) != 1 || got[0] != "Hidden" {
		t.Fatalf("page: %v of %d", got, total)
	}
	if got, total := names(domain.ProductFilter{Page: all, Search: "COTTON", Sort: "name", Order: "asc"}); total != 2 || got[0] != "100% cotton" {
		t.Fatalf("search: %v of %d", got, total)
	}
	if got, total := names(domain.ProductFilter{Page: all, Search: "%"}); total != 1 || got[0] != "100% cotton" {
		t.Fatalf("wildcard must be literal: %v of %d", got, total)
	}
	if got, total := names(domain.ProductFilter{Page: domain.NewPage(9, 100)}); total != 4 || len(got) != 0 {
		t.Fatalf("past the end: %v of %d", got, total)
	}

	_, _, err := svc.List(ctx, domain.ProductFilter{Page: all, Sort: "password_hash"})
	wantCode(t, err, apperror.CodeInvalidInput)
	_, _, err = svc.List(ctx, domain.ProductFilter{Page: all, Order: "desc; DROP TABLE app.product"})
	wantCode(t, err, apperror.CodeInvalidInput)
	_, _, err = svc.List(ctx, domain.ProductFilter{Page: all, Search: "a\x00b"})
	wantCode(t, err, apperror.CodeInvalidInput)

	options, err := svc.Options(ctx)
	if err != nil || len(options) != 3 || options[0].Name != "100% cotton" || options[0].ID == "" {
		t.Fatalf("options: %+v %v", options, err)
	}
}

func TestProductUpdateReturnsNewTimestamp(t *testing.T) {
	ctx := context.Background()
	svc := product.New(productrepo.NewRepository(testDB))
	p, err := svc.Create(ctx, input.CreateProductCommand{Name: "Stamp", Price: "1"})
	if err != nil {
		t.Fatal(err)
	}
	name := "Stamped"
	updated, err := svc.Update(ctx, input.UpdateProductCommand{ID: p.ID, Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.UpdatedAt.After(p.CreatedAt) {
		t.Fatalf("updated_at stale: %v vs %v", updated.UpdatedAt, p.CreatedAt)
	}
}

func TestProductUpdateKeepsFieldsItDoesNotCarry(t *testing.T) {
	ctx := context.Background()
	svc := product.New(productrepo.NewRepository(testDB))
	p, err := svc.Create(ctx, input.CreateProductCommand{Name: "Patch", Price: "5"})
	if err != nil {
		t.Fatal(err)
	}
	name, price := "Patched", "7.5"
	if _, err := svc.Update(ctx, input.UpdateProductCommand{ID: p.ID, Name: &name}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Update(ctx, input.UpdateProductCommand{ID: p.ID, Price: &price})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Patched" || got.Price != "7.50" || !got.Enabled {
		t.Fatalf("%+v", got)
	}
	missing := "11111111-1111-4111-8111-111111111111"
	_, err = svc.Update(ctx, input.UpdateProductCommand{ID: missing, Name: &name})
	wantCode(t, err, apperror.CodeNotFound)
}
