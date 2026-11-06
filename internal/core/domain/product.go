package domain

import "time"

// ProductFilter selects one page of products.
type ProductFilter struct {
	Page   Page
	Search string
	Sort   string
	Order  string
}

// Product is the example app domain entity (app.product).
type Product struct {
	ID        string
	Name      string
	Price     string // decimal as string to avoid float
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
