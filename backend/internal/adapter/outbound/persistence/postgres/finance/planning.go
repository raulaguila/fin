package finance

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/raulaguila/fin/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/apperror"
)

// ---- pending transactions ----

func (r *Repository) DeleteMissingPending(ctx context.Context, accountID string, from time.Time, keep []string) (int64, error) {
	q := r.db.WithContext(ctx)
	sql := `DELETE FROM app.fin_transaction
		WHERE account_id = ? AND status = 'PENDING' AND split_of IS NULL AND date >= ?::date`
	args := []any{accountID, from.Format(time.DateOnly)}
	if len(keep) > 0 {
		sql, args = sql+` AND external_id NOT IN ?`, append(args, keep)
	}
	res := q.Exec(sql, args...)
	return res.RowsAffected, shared.WrapError(res.Error, "transaction")
}

// ---- splits ----

type splitSource struct {
	ID         uuid.UUID `gorm:"column:id"`
	ExternalID string    `gorm:"column:external_id"`
	Amount     int64     `gorm:"column:amount_cents"`
}

// SplitTransaction runs inside the caller's transaction: the row lock holds
// off a sync or a second split until the part is written.
func (r *Repository) SplitTransaction(ctx context.Context, userID, id string, amount domain.Money, categoryID string) error {
	db := r.db.WithContext(ctx)
	var src splitSource
	err := db.Raw(`SELECT id, external_id, amount_cents FROM app.fin_transaction
		WHERE id = ? AND user_id = ? AND split_of IS NULL FOR UPDATE`, id, userID).Take(&src).Error
	if err != nil {
		return shared.WrapError(err, "transaction")
	}
	if amount == 0 || (amount < 0) != (src.Amount < 0) || abs(amount) >= abs(src.Amount) {
		return apperror.New(apperror.CodeInvalidInput, "the part must have the sign of the transaction and be smaller").WithField("amount")
	}
	err = db.Exec(`
		INSERT INTO app.fin_transaction (user_id, account_id, external_id, date, description, amount_cents, status,
		                                 provider_category, category_id, category_source, split_of)
		SELECT user_id, account_id, external_id || ':split:' || gen_random_uuid(), date, description, ?, status,
		       provider_category, ?, 'user', id
		FROM app.fin_transaction WHERE id = ?`, amount, categoryID, src.ID).Error
	if err != nil {
		return shared.WrapError(err, "transaction")
	}
	err = db.Exec(`UPDATE app.fin_transaction SET amount_cents = amount_cents - ? WHERE id = ?`, amount, src.ID).Error
	return shared.WrapError(err, "transaction")
}

func (r *Repository) UnsplitTransaction(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`
		WITH parts AS (DELETE FROM app.fin_transaction WHERE split_of = ? AND user_id = ? RETURNING amount_cents)
		UPDATE app.fin_transaction SET amount_cents = amount_cents + (SELECT coalesce(sum(amount_cents), 0) FROM parts)
		WHERE id = ? AND user_id = ? AND split_of IS NULL`, id, userID, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "transaction")
	}
	if res.RowsAffected == 0 {
		return notFound("transaction")
	}
	return nil
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// ---- investments ----

func (r *Repository) UpsertInvestments(ctx context.Context, userID, connectionID string, list []domain.OFInvestment) error {
	db := r.db.WithContext(ctx)
	keep := make([]string, 0, len(list))
	for _, i := range list {
		err := db.Exec(`
			INSERT INTO app.fin_investment AS t (user_id, connection_id, external_id, name, type, balance_cents)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (external_id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type,
			    balance_cents = EXCLUDED.balance_cents, updated_at = now()
			WHERE t.user_id = EXCLUDED.user_id`,
			userID, connectionID, i.ID, i.Name, i.Type, i.Balance).Error
		if err != nil {
			return shared.WrapError(err, "investment")
		}
		keep = append(keep, i.ID)
	}
	sql, args := `DELETE FROM app.fin_investment WHERE connection_id = ?`, []any{connectionID}
	if len(keep) > 0 {
		sql, args = sql+` AND external_id NOT IN ?`, append(args, keep)
	}
	return shared.WrapError(db.Exec(sql, args...).Error, "investment")
}

type investmentRow struct {
	ID           uuid.UUID `gorm:"column:id"`
	ConnectionID uuid.UUID `gorm:"column:connection_id"`
	Name         string    `gorm:"column:name"`
	Type         string    `gorm:"column:type"`
	Balance      int64     `gorm:"column:balance_cents"`
}

func (r *Repository) Investments(ctx context.Context, userID string) ([]domain.Investment, error) {
	var rows []investmentRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, connection_id, name, type, balance_cents FROM app.fin_investment
		WHERE user_id = ? ORDER BY balance_cents DESC, id LIMIT 200`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "investment")
	}
	out := make([]domain.Investment, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Investment{ID: row.ID.String(), ConnectionID: row.ConnectionID.String(), Name: row.Name, Type: row.Type, Balance: row.Balance})
	}
	return out, nil
}

// ---- goals ----

type goalRow struct {
	ID        uuid.UUID  `gorm:"column:id"`
	Name      string     `gorm:"column:name"`
	Target    int64      `gorm:"column:target_cents"`
	Saved     int64      `gorm:"column:saved_cents"`
	AccountID *uuid.UUID `gorm:"column:account_id"`
	DueDate   *time.Time `gorm:"column:due_date"`
	Current   int64      `gorm:"column:current"`
}

func (r *Repository) Goals(ctx context.Context, userID string) ([]domain.Goal, error) {
	var rows []goalRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT g.id, g.name, g.target_cents, g.saved_cents, g.account_id, g.due_date,
		       coalesce(a.balance_cents, g.saved_cents) AS current
		FROM app.fin_goal g LEFT JOIN app.fin_account a ON a.id = g.account_id
		WHERE g.user_id = ? ORDER BY g.created_at, g.id LIMIT 100`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "goal")
	}
	out := make([]domain.Goal, 0, len(rows))
	for _, row := range rows {
		g := domain.Goal{ID: row.ID.String(), Name: row.Name, Target: row.Target, Saved: row.Saved, DueDate: row.DueDate, Current: row.Current}
		if row.AccountID != nil {
			g.AccountID = row.AccountID.String()
		}
		out = append(out, g)
	}
	return out, nil
}

func (r *Repository) CreateGoal(ctx context.Context, userID string, g *domain.Goal) error {
	var id string
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO app.fin_goal (user_id, name, target_cents, saved_cents, account_id, due_date)
		VALUES (?, ?, ?, ?, NULLIF(?, '')::uuid, ?) RETURNING id`,
		userID, g.Name, g.Target, g.Saved, g.AccountID, g.DueDate).Take(&id).Error
	if err != nil {
		return shared.WrapError(err, "goal")
	}
	g.ID = id
	return nil
}

func (r *Repository) UpdateGoal(ctx context.Context, userID string, g *domain.Goal) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_goal SET name = ?, target_cents = ?, saved_cents = ?, account_id = NULLIF(?, '')::uuid, due_date = ?
		WHERE id = ? AND user_id = ?`, g.Name, g.Target, g.Saved, g.AccountID, g.DueDate, g.ID, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "goal")
	}
	if res.RowsAffected == 0 {
		return notFound("goal")
	}
	return nil
}

func (r *Repository) DeleteGoal(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_goal WHERE id = ? AND user_id = ?`, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "goal")
	}
	if res.RowsAffected == 0 {
		return notFound("goal")
	}
	return nil
}
