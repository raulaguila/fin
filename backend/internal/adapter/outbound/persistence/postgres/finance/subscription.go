package finance

import (
	"context"

	"github.com/google/uuid"

	"github.com/raulaguila/fin/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/fin/internal/core/domain"
)

type subscriptionRow struct {
	ID         uuid.UUID  `gorm:"column:id"`
	Name       string     `gorm:"column:name"`
	Amount     int64      `gorm:"column:amount_cents"`
	Day        int        `gorm:"column:day"`
	Month      *int       `gorm:"column:month"`
	Keyword    string     `gorm:"column:keyword"`
	CategoryID *uuid.UUID `gorm:"column:category_id"`
	AccountID  *uuid.UUID `gorm:"column:account_id"`
}

func (r *Repository) Subscriptions(ctx context.Context, userID string) ([]domain.Subscription, error) {
	var rows []subscriptionRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, name, amount_cents, day, month, keyword, category_id, account_id
		FROM app.fin_subscription WHERE user_id = ? ORDER BY name, id LIMIT 200`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "subscription")
	}
	out := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		s := domain.Subscription{ID: row.ID.String(), Name: row.Name, Amount: row.Amount, Day: row.Day, Keyword: row.Keyword}
		if row.Month != nil {
			s.Month = *row.Month
		}
		if row.CategoryID != nil {
			s.CategoryID = row.CategoryID.String()
		}
		if row.AccountID != nil {
			s.AccountID = row.AccountID.String()
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *Repository) CreateSubscription(ctx context.Context, userID string, s *domain.Subscription) error {
	var id string
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO app.fin_subscription (user_id, name, amount_cents, day, month, keyword, category_id, account_id)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, NULLIF(?, '')::uuid, NULLIF(?, '')::uuid) RETURNING id`,
		userID, s.Name, s.Amount, s.Day, s.Month, s.Keyword, s.CategoryID, s.AccountID).Take(&id).Error
	if err != nil {
		return shared.WrapError(err, "subscription")
	}
	s.ID = id
	return nil
}

func (r *Repository) UpdateSubscription(ctx context.Context, userID string, s *domain.Subscription) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_subscription SET name = ?, amount_cents = ?, day = ?, month = NULLIF(?, 0), keyword = ?,
		       category_id = NULLIF(?, '')::uuid, account_id = NULLIF(?, '')::uuid
		WHERE id = ? AND user_id = ?`,
		s.Name, s.Amount, s.Day, s.Month, s.Keyword, s.CategoryID, s.AccountID, s.ID, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "subscription")
	}
	if res.RowsAffected == 0 {
		return notFound("subscription")
	}
	return nil
}

func (r *Repository) DeleteSubscription(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_subscription WHERE id = ? AND user_id = ?`, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "subscription")
	}
	if res.RowsAffected == 0 {
		return notFound("subscription")
	}
	return nil
}

func (r *Repository) IgnoredRecurring(ctx context.Context, userID string) ([]string, error) {
	var keys []string
	err := r.db.WithContext(ctx).Raw(`
		SELECT key FROM app.fin_recurring_ignored WHERE user_id = ? ORDER BY key LIMIT 500`, userID).Find(&keys).Error
	return keys, shared.WrapError(err, "subscription")
}

func (r *Repository) IgnoreRecurring(ctx context.Context, userID, key string) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO app.fin_recurring_ignored (user_id, key) VALUES (?, ?) ON CONFLICT DO NOTHING`, userID, key).Error
	return shared.WrapError(err, "subscription")
}
