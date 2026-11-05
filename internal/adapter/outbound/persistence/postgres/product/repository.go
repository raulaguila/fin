package product

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
)

type Repository struct {
	db *gorm.DB
}

var _ output.ProductRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type row struct {
	ID        uuid.UUID `gorm:"column:id"`
	Name      string    `gorm:"column:name"`
	Price     string    `gorm:"column:price"`
	Enabled   bool      `gorm:"column:enabled"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (m row) toDomain() domain.Product {
	return domain.Product{
		ID:        m.ID.String(),
		Name:      m.Name,
		Price:     m.Price,
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func (r *Repository) List(ctx context.Context, limit int) ([]domain.Product, error) {
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, name, price::text, enabled, created_at, updated_at
		FROM app.product
		ORDER BY created_at DESC
		LIMIT ?
	`, limit).Scan(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "product")
	}
	out := make([]domain.Product, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Product, error) {
	var m row
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, name, price::text, enabled, created_at, updated_at
		FROM app.product WHERE id = ?
	`, id).Take(&m).Error
	if err != nil {
		return nil, shared.WrapError(err, "product")
	}
	p := m.toDomain()
	return &p, nil
}

func (r *Repository) Create(ctx context.Context, p *domain.Product) error {
	id := uuid.New()
	now := time.Now()
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO app.product (id, name, price, enabled, created_at, updated_at)
		VALUES (?, ?, ?::numeric, ?, ?, ?)
	`, id, p.Name, p.Price, p.Enabled, now, now).Error
	if err != nil {
		return shared.WrapError(err, "product")
	}
	p.ID = id.String()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (r *Repository) Update(ctx context.Context, p *domain.Product) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.product
		SET name = ?, price = ?::numeric, enabled = ?, updated_at = now()
		WHERE id = ?
	`, p.Name, p.Price, p.Enabled, p.ID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "product")
	}
	if res.RowsAffected == 0 {
		return shared.WrapError(gorm.ErrRecordNotFound, "product")
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.product WHERE id = ?`, id)
	if res.Error != nil {
		return shared.WrapError(res.Error, "product")
	}
	if res.RowsAffected == 0 {
		return shared.WrapError(gorm.ErrRecordNotFound, "product")
	}
	return nil
}
