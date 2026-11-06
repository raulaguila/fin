package output

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// ProductRepository persists app.product.
type ProductRepository interface {
	// List returns one page and the count of all matches (-1 in cursor mode).
	List(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int64, error)
	// ListOptions returns enabled products with only id and name.
	ListOptions(ctx context.Context, limit int) ([]domain.Product, error)
	FindByID(ctx context.Context, id string) (*domain.Product, error)
	Create(ctx context.Context, p *domain.Product) error
	// Update writes only the fields set in patch, so two edits of different fields both land.
	Update(ctx context.Context, id string, patch domain.ProductPatch) (*domain.Product, error)
	Delete(ctx context.Context, id string) error
}
