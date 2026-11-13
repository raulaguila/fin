package finance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/raulaguila/fin/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/pkg/apperror"
)

// balanceDelta is what an amount does to its account's balance: a card's balance
// is what it owes, so a purchase (negative) raises it.
const balanceDelta = `CASE a.type WHEN 'CREDIT' THEN -%s ELSE %s END`

func (r *Repository) CreateTransaction(ctx context.Context, userID, accountID string, t *domain.Transaction) error {
	var ids []string
	err := r.db.WithContext(ctx).Raw(`
		WITH ins AS (
		    INSERT INTO app.fin_transaction (user_id, account_id, external_id, date, description, amount_cents, status,
		                                     category_id, category_source, note, tags)
		    VALUES (?, ?, ?, ?::date, ?, ?, 'POSTED', NULLIF(?, '')::uuid, ?, ?, string_to_array(?, ','))
		    RETURNING id, account_id, amount_cents
		), upd AS (
		    UPDATE app.fin_account a SET balance_cents = a.balance_cents + `+
		strings.ReplaceAll(balanceDelta, "%s", "ins.amount_cents")+`
		    FROM ins WHERE a.id = ins.account_id AND a.user_id = ?
		)
		SELECT id::text FROM ins`,
		userID, accountID, t.ExternalID, t.Date.Format(time.DateOnly), t.Description, t.Amount, t.CategoryID,
		t.CategorySource, t.Note, strings.Join(t.Tags, ","), userID).Find(&ids).Error
	if err != nil {
		return shared.WrapError(err, "transaction")
	}
	if len(ids) == 0 {
		return notFound("account")
	}
	t.ID = ids[0]
	return nil
}

func (r *Repository) DeleteManualTransaction(ctx context.Context, userID, id string) error {
	// The CTEs see the rows as they were before the statement, so the split
	// parts (deleted by the cascade) still count toward what leaves the balance.
	var ids []string
	err := r.db.WithContext(ctx).Raw(`
		WITH gone AS (
		    DELETE FROM app.fin_transaction t
		    USING app.fin_account a, app.fin_connection c
		    WHERE t.id = ? AND t.user_id = ? AND t.split_of IS NULL
		      AND a.id = t.account_id AND c.id = a.connection_id AND c.item_id LIKE '`+domain.ManualItemPrefix+`%'
		    RETURNING t.id, t.account_id,
		              t.amount_cents + coalesce((SELECT sum(s.amount_cents) FROM app.fin_transaction s WHERE s.split_of = t.id), 0) AS amount
		), upd AS (
		    UPDATE app.fin_account a SET balance_cents = a.balance_cents - `+
		strings.ReplaceAll(balanceDelta, "%s", "gone.amount")+`
		    FROM gone WHERE a.id = gone.account_id
		)
		SELECT id::text FROM gone`, id, userID).Find(&ids).Error
	if err != nil {
		return shared.WrapError(err, "transaction")
	}
	if len(ids) == 0 {
		return apperror.New(apperror.CodeNotFound, "manual transaction not found").WithDetails("resource", "transaction")
	}
	return nil
}

func (r *Repository) SetTransactionDetails(ctx context.Context, userID, id, note string, tags []string) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE app.fin_transaction SET note = ?, tags = string_to_array(?, ',') WHERE id = ? AND user_id = ?`,
		note, strings.Join(tags, ","), id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "transaction")
	}
	if res.RowsAffected == 0 {
		return notFound("transaction")
	}
	return nil
}

func (r *Repository) SetTransactionsCategory(ctx context.Context, userID string, ids []string, categoryID string) (int64, error) {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_transaction SET category_id = NULLIF(?, '')::uuid, category_source = 'user'
		WHERE user_id = ? AND id = ANY(string_to_array(?, ',')::uuid[])`, categoryID, userID, strings.Join(ids, ","))
	return res.RowsAffected, shared.WrapError(res.Error, "transaction")
}

func (r *Repository) CardTransactions(ctx context.Context, userID string, from time.Time, limit int) ([]domain.Transaction, error) {
	var rows []transactionRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.account_id, a.name AS account_name, t.date, t.description, t.amount_cents, t.installment, t.installments
		FROM app.fin_transaction t JOIN app.fin_account a ON a.id = t.account_id
		WHERE t.user_id = ? AND t.date >= ?::date AND a.type = 'CREDIT'
		ORDER BY t.date DESC, t.id LIMIT ?`, userID, from.Format(time.DateOnly), limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), nil
}

// ---- net worth ----

type netWorthRow struct {
	Day         time.Time `gorm:"column:day"`
	Banks       int64     `gorm:"column:banks_cents"`
	Investments int64     `gorm:"column:investments_cents"`
	Cards       int64     `gorm:"column:cards_cents"`
}

func (r *Repository) SaveNetWorth(ctx context.Context, userID string, p domain.NetWorthPoint) error {
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO app.fin_net_worth (user_id, day, banks_cents, investments_cents, cards_cents) VALUES (?, ?::date, ?, ?, ?)
		ON CONFLICT (user_id, day) DO UPDATE SET banks_cents = EXCLUDED.banks_cents,
		    investments_cents = EXCLUDED.investments_cents, cards_cents = EXCLUDED.cards_cents`,
		userID, p.Day.Format(time.DateOnly), p.Banks, p.Investments, p.Cards).Error
	return shared.WrapError(err, "net worth")
}

func (r *Repository) NetWorth(ctx context.Context, userID string, from time.Time) ([]domain.NetWorthPoint, error) {
	var rows []netWorthRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT day, banks_cents, investments_cents, cards_cents FROM app.fin_net_worth
		WHERE user_id = ? AND day >= ?::date ORDER BY day LIMIT 800`, userID, from.Format(time.DateOnly)).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "net worth")
	}
	out := make([]domain.NetWorthPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.NetWorthPoint{Day: row.Day, Banks: row.Banks, Investments: row.Investments, Cards: row.Cards})
	}
	return out, nil
}

// ---- debts ----

type debtRow struct {
	ID          uuid.UUID  `gorm:"column:id"`
	Person      string     `gorm:"column:person"`
	Amount      int64      `gorm:"column:amount_cents"`
	Description string     `gorm:"column:description"`
	DueDate     *time.Time `gorm:"column:due_date"`
	SettledOn   *time.Time `gorm:"column:settled_on"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
}

func (r *Repository) Debts(ctx context.Context, userID string) ([]domain.Debt, error) {
	var rows []debtRow
	// Open ones first; settled ones keep only the latest.
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, person, amount_cents, description, due_date, settled_on, created_at FROM app.fin_debt
		WHERE user_id = ? ORDER BY settled_on DESC NULLS FIRST, due_date NULLS LAST, created_at, id LIMIT 300`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "debt")
	}
	out := make([]domain.Debt, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Debt{ID: row.ID.String(), Person: row.Person, Amount: row.Amount, Description: row.Description,
			DueDate: row.DueDate, SettledOn: row.SettledOn, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *Repository) CreateDebt(ctx context.Context, userID string, d *domain.Debt) error {
	var row debtRow
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO app.fin_debt (user_id, person, amount_cents, description, due_date, settled_on)
		VALUES (?, ?, ?, ?, ?, ?) RETURNING id, created_at`,
		userID, d.Person, d.Amount, d.Description, d.DueDate, d.SettledOn).Take(&row).Error
	if err != nil {
		return shared.WrapError(err, "debt")
	}
	d.ID, d.CreatedAt = row.ID.String(), row.CreatedAt
	return nil
}

func (r *Repository) UpdateDebt(ctx context.Context, userID string, d *domain.Debt) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_debt SET person = ?, amount_cents = ?, description = ?, due_date = ?, settled_on = ?
		WHERE id = ? AND user_id = ?`, d.Person, d.Amount, d.Description, d.DueDate, d.SettledOn, d.ID, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "debt")
	}
	if res.RowsAffected == 0 {
		return notFound("debt")
	}
	return nil
}

func (r *Repository) DeleteDebt(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_debt WHERE id = ? AND user_id = ?`, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "debt")
	}
	if res.RowsAffected == 0 {
		return notFound("debt")
	}
	return nil
}
