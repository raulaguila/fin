package output

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// ProductRepository persists app.product.
type ProductRepository interface {
	// List returns one page and the count of all matches.
	List(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int64, error)
	// ListOptions returns enabled products with only id and name.
	ListOptions(ctx context.Context, limit int) ([]domain.Product, error)
	FindByID(ctx context.Context, id string) (*domain.Product, error)
	Create(ctx context.Context, p *domain.Product) error
	Update(ctx context.Context, p *domain.Product) error
	Delete(ctx context.Context, id string) error
}
