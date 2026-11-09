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

// sortColumns is the allowlist behind ORDER BY, which takes no bind parameters.
var sortColumns = map[string]string{
	"name": "name", "price": "price", "created_at": "created_at", "updated_at": "updated_at",
}

// List keeps numbered pages compatible; cursor mode avoids OFFSET and COUNT.
func (r *Repository) List(ctx context.Context, filter domain.ProductFilter) ([]domain.Product, int64, error) {
	matching := func() *gorm.DB {
		q := r.db.WithContext(ctx).Table("app.product")
		if filter.Search != "" {
			q = q.Where("name ILIKE ?", shared.Contains(filter.Search))
		}
		if filter.Enabled != nil {
			q = q.Where("enabled = ?", *filter.Enabled)
		}
		return q
	}
	total := int64(-1) // cursor pages deliberately do not calculate a total
	if !filter.Cursor {
		if err := matching().Count(&total).Error; err != nil {
			return nil, 0, shared.WrapError(err, "product")
		}
	}
	column, ok := sortColumns[filter.Sort]
	if !ok {
		column = "created_at"
	}
	direction := "DESC"
	if filter.Order == "asc" {
		direction = "ASC"
	}
	q := matching()
	order := "app.product." + column + " " + direction + ", app.product.id"
	if filter.Cursor {
		order = "app.product.created_at DESC, app.product.id DESC"
		if filter.After != nil {
			q = q.Where("(app.product.created_at, app.product.id) < (?, ?::uuid)", filter.After.CreatedAt, filter.After.ID)
		}
	} else {
		q = q.Offset(filter.Page.Offset())
	}
	var rows []row
	err := q.Select("id, name, price::text AS price, enabled, created_at, updated_at").Order(order).Limit(filter.Page.Limit).Find(&rows).Error
	if err != nil {
		return nil, 0, shared.WrapError(err, "product")
	}
	out := make([]domain.Product, 0, len(rows))
	for _, m := range rows {
		out = append(out, m.toDomain())
	}
	return out, total, nil
}

func (r *Repository) ListOptions(ctx context.Context, limit int) ([]domain.Product, error) {
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, name FROM app.product WHERE enabled ORDER BY name, id LIMIT ?
	`, limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "product")
	}
	out := make([]domain.Product, 0, len(rows))
	for _, m := range rows {
		out = append(out, m.toDomain())
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

func (r *Repository) Update(ctx context.Context, id string, patch domain.ProductPatch) (*domain.Product, error) {
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
		UPDATE app.product
		SET name = COALESCE(?::text, name),
		    price = COALESCE(?::numeric, price),
		    enabled = COALESCE(?::boolean, enabled),
		    updated_at = now()
		WHERE id = ?
		RETURNING id, name, price::text AS price, enabled, created_at, updated_at
	`, patch.Name, patch.Price, patch.Enabled, id).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "product")
	}
	if len(rows) == 0 {
		return nil, shared.WrapError(gorm.ErrRecordNotFound, "product")
	}
	p := rows[0].toDomain()
	return &p, nil
}

// DeleteMany is one statement: the count check and the delete see the same snapshot.
func (r *Repository) DeleteMany(ctx context.Context, ids []string) error {
	res := r.db.WithContext(ctx).Exec(`
		DELETE FROM app.product WHERE id IN ?
		AND (SELECT count(*) FROM app.product WHERE id IN ?) = ?
	`, ids, ids, len(ids))
	if res.Error != nil {
		return shared.WrapError(res.Error, "product")
	}
	if res.RowsAffected == 0 {
		return shared.WrapError(gorm.ErrRecordNotFound, "product")
	}
	return nil
}
