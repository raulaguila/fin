package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

type CreateProductCommand struct {
	Name    string
	Price   string
	Enabled *bool
}

type UpdateProductCommand struct {
	ID      string
	Name    *string
	Price   *string
	Enabled *bool
}

// ProductService is the inbound port for product CRUD.
type ProductService interface {
	List(ctx context.Context, limit int) ([]domain.Product, error)
	Get(ctx context.Context, id string) (*domain.Product, error)
	Create(ctx context.Context, cmd CreateProductCommand) (*domain.Product, error)
	Update(ctx context.Context, cmd UpdateProductCommand) (*domain.Product, error)
	Delete(ctx context.Context, id string) error
}
