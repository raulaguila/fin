package finance

import (
	"context"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/fin/internal/core/domain"
)

type changeRow struct {
	UndoID string `gorm:"column:undo_id"`
	N      int64  `gorm:"column:n"`
}

// changeCategories sets the category of the user's rows matching where (on alias t)
// and keeps what they had in a new undo, all in one statement.
func (r *Repository) changeCategories(ctx context.Context, userID, ruleID, categoryID, source, where string, args ...any) (domain.Change, error) {
	var row changeRow
	params := []any{userID, ruleID, categoryID, source, userID}
	params = append(params, args...)
	params = append(params, categoryID, source, categoryID, source)
	err := r.db.WithContext(ctx).Raw(`
		WITH u AS (
			INSERT INTO app.fin_undo (user_id, rule_id, set_category_id, set_source)
			VALUES (?, NULLIF(?, '')::uuid, NULLIF(?, '')::uuid, ?) RETURNING id
		), old AS (
			SELECT t.id, t.category_id, t.category_source FROM app.fin_transaction t
			WHERE t.user_id = ? AND `+where+`
			  AND (t.category_id IS DISTINCT FROM NULLIF(?, '')::uuid OR t.category_source <> ?)
			FOR UPDATE
		), saved AS (
			INSERT INTO app.fin_undo_row (undo_id, transaction_id, category_id, category_source)
			SELECT u.id, old.id, old.category_id, old.category_source FROM u, old
		), upd AS (
			UPDATE app.fin_transaction t SET category_id = NULLIF(?, '')::uuid, category_source = ?
			FROM old WHERE t.id = old.id RETURNING t.id
		)
		SELECT (SELECT id::text FROM u) AS undo_id, (SELECT count(*) FROM upd) AS n`, params...).Take(&row).Error
	if err != nil {
		return domain.Change{}, shared.WrapError(err, "transaction")
	}
	return domain.Change{Count: row.N, UndoID: row.UndoID}, nil
}

func (r *Repository) SetTransactionsCategory(ctx context.Context, userID string, ids []string, categoryID string) (domain.Change, error) {
	return r.changeCategories(ctx, userID, "", categoryID, domain.SourceUser,
		"t.id = ANY(string_to_array(?, ',')::uuid[])", strings.Join(ids, ","))
}

func (r *Repository) ApplyRule(ctx context.Context, userID, ruleID, keyword, categoryID string) (domain.Change, error) {
	return r.changeCategories(ctx, userID, ruleID, categoryID, domain.SourceRule,
		"t.description ILIKE ? AND t.category_source <> 'user'", shared.Contains(keyword))
}

type undoRow struct {
	Found int64 `gorm:"column:found"`
	N     int64 `gorm:"column:n"`
}

func (r *Repository) Undo(ctx context.Context, userID, undoID string) (int64, error) {
	var row undoRow
	// Deleting the undo cascades to its rows at the end of the statement; upd still reads them.
	err := r.db.WithContext(ctx).Raw(`
		WITH u AS (
			DELETE FROM app.fin_undo WHERE id = ? AND user_id = ? RETURNING id, rule_id, set_category_id, set_source
		), upd AS (
			UPDATE app.fin_transaction t SET category_id = r.category_id, category_source = r.category_source
			FROM app.fin_undo_row r, u
			WHERE r.undo_id = u.id AND t.id = r.transaction_id AND t.user_id = ?
			  AND t.category_id IS NOT DISTINCT FROM u.set_category_id AND t.category_source = u.set_source
			RETURNING t.id
		), rule AS (
			DELETE FROM app.fin_rule WHERE id = (SELECT rule_id FROM u) AND user_id = ?
		)
		SELECT (SELECT count(*) FROM u) AS found, (SELECT count(*) FROM upd) AS n`, undoID, userID, userID, userID).Take(&row).Error
	if err != nil {
		return 0, shared.WrapError(err, "undo")
	}
	if row.Found == 0 {
		return 0, notFound("undo")
	}
	return row.N, nil
}

func (r *Repository) PruneUndo(ctx context.Context, before time.Time) error {
	return shared.WrapError(r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_undo WHERE created_at < ?`, before).Error, "undo")
}

func (r *Repository) PairTransfers(ctx context.Context, userID, categoryID string, from time.Time, days int, minimum domain.Money) (int64, error) {
	// A pair is kept only when each side has exactly one match: two R$ 50 Pix on the
	// same day stay for the user to file.
	res := r.db.WithContext(ctx).Exec(`
		WITH c AS (
			SELECT t.id, t.account_id, t.date, t.amount_cents FROM app.fin_transaction t
			LEFT JOIN app.fin_category cat ON cat.id = t.category_id
			WHERE t.user_id = ? AND t.date >= ?::date AND t.category_source = 'auto' AND t.status <> 'PENDING'
			  AND t.split_of IS NULL AND abs(t.amount_cents) >= ? AND cat.kind IS DISTINCT FROM 'transfer'
			  AND NOT EXISTS (SELECT 1 FROM app.fin_transaction s WHERE s.split_of = t.id)
		), m AS (
			SELECT o.id AS out_id, i.id AS in_id,
			       count(*) OVER (PARTITION BY o.id) AS outs, count(*) OVER (PARTITION BY i.id) AS ins
			FROM c o JOIN c i ON i.amount_cents = -o.amount_cents AND i.account_id <> o.account_id
			 AND abs(i.date - o.date) <= ?
			WHERE o.amount_cents < 0
		)
		UPDATE app.fin_transaction t SET category_id = ?::uuid, category_source = 'pair'
		FROM m WHERE m.outs = 1 AND m.ins = 1 AND t.user_id = ? AND t.id IN (m.out_id, m.in_id)`,
		userID, from.Format(time.DateOnly), abs(minimum), days, categoryID, userID)
	return res.RowsAffected, shared.WrapError(res.Error, "transaction")
}

func (r *Repository) StartBalanceCheck(ctx context.Context, userID, accountID string, reset bool) error {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_account a
		SET check_balance_cents = a.balance_cents, check_since = now(),
		    check_sum_cents = (SELECT coalesce(sum(t.amount_cents), 0) FROM app.fin_transaction t WHERE t.account_id = a.id)
		WHERE a.id = ? AND a.user_id = ? AND a.type = 'BANK' AND (? OR a.check_since IS NULL)`, accountID, userID, reset)
	if res.Error != nil {
		return shared.WrapError(res.Error, "account")
	}
	if reset && res.RowsAffected == 0 {
		return notFound("account")
	}
	return nil
}

type accountHealthRow struct {
	ID          string     `gorm:"column:id"`
	Name        string     `gorm:"column:name"`
	Institution string     `gorm:"column:institution"`
	Type        string     `gorm:"column:type"`
	ItemID      string     `gorm:"column:item_id"`
	LastDate    *time.Time `gorm:"column:last_date"`
	CheckSince  *time.Time `gorm:"column:check_since"`
	Gap         *int64     `gorm:"column:gap"`
}

type healthCountRow struct {
	Uncategorized int64 `gorm:"column:uncategorized"`
	Paired        int64 `gorm:"column:paired"`
}

type backupRow struct {
	Kind      string    `gorm:"column:kind"`
	OK        bool      `gorm:"column:ok"`
	Detail    string    `gorm:"column:detail"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (r *Repository) DataHealth(ctx context.Context, userID string, from time.Time) (*domain.DataHealth, error) {
	db := r.db.WithContext(ctx)
	var accounts []accountHealthRow
	err := db.Raw(`
		SELECT a.id, a.name, c.institution, a.type, c.item_id, a.check_since,
		       (SELECT max(t.date) FROM app.fin_transaction t WHERE t.account_id = a.id AND t.date <= current_date) AS last_date,
		       CASE WHEN a.check_since IS NOT NULL THEN
		         (a.balance_cents - a.check_balance_cents)
		         - ((SELECT coalesce(sum(t.amount_cents), 0) FROM app.fin_transaction t WHERE t.account_id = a.id) - a.check_sum_cents)
		       END AS gap
		FROM app.fin_account a JOIN app.fin_connection c ON c.id = a.connection_id
		WHERE a.user_id = ? ORDER BY c.institution, a.name LIMIT 200`, userID).Find(&accounts).Error
	if err != nil {
		return nil, shared.WrapError(err, "account")
	}
	var counts healthCountRow
	err = db.Raw(`
		SELECT count(*) FILTER (WHERE category_id IS NULL) AS uncategorized,
		       count(*) FILTER (WHERE category_source = 'pair') AS paired
		FROM app.fin_transaction WHERE user_id = ? AND date >= ?::date`, userID, from.Format(time.DateOnly)).Take(&counts).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	var backups []backupRow
	err = db.Raw(`
		SELECT DISTINCT ON (kind) kind, ok, detail, created_at FROM sys.backup_run
		ORDER BY kind, created_at DESC LIMIT 3`).Find(&backups).Error
	if err != nil {
		return nil, shared.WrapError(err, "backup")
	}
	out := &domain.DataHealth{Uncategorized: counts.Uncategorized, Paired: counts.Paired}
	for _, a := range accounts {
		h := domain.AccountHealth{ID: a.ID, Name: a.Name, Institution: a.Institution, Type: a.Type,
			Manual: strings.HasPrefix(a.ItemID, domain.ManualItemPrefix), LastDate: a.LastDate, CheckSince: a.CheckSince}
		if a.Gap != nil {
			g := domain.Money(*a.Gap)
			h.Gap = &g
		}
		out.Accounts = append(out.Accounts, h)
	}
	for _, b := range backups {
		out.Backups = append(out.Backups, domain.BackupRun{Kind: b.Kind, OK: b.OK, Detail: b.Detail, At: b.CreatedAt})
	}
	return out, nil
}
