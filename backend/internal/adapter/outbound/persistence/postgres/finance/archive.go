package finance

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/raulaguila/fin/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/fin/internal/core/domain"
)

type overrideRow struct {
	ExternalID   string `gorm:"column:external_id"`
	Category     string `gorm:"column:category"`
	UserCategory bool   `gorm:"column:user_category"`
	Note         string `gorm:"column:note"`
	Tags         string `gorm:"column:tags"`
}

func (r *Repository) ArchiveOverrides(ctx context.Context, userID string, limit int) ([]domain.ArchiveOverride, error) {
	var rows []overrideRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.external_id, coalesce(cat.name, '') AS category, t.category_source = 'user' AS user_category,
		       t.note, array_to_string(t.tags, ',') AS tags
		FROM app.fin_transaction t
		JOIN app.fin_account a ON a.id = t.account_id
		JOIN app.fin_connection c ON c.id = a.connection_id
		LEFT JOIN app.fin_category cat ON cat.id = t.category_id
		WHERE t.user_id = ? AND c.item_id NOT LIKE '`+domain.ManualItemPrefix+`%' AND t.split_of IS NULL
		  AND (t.category_source = 'user' OR t.note <> '' OR cardinality(t.tags) > 0)
		ORDER BY t.date DESC, t.id LIMIT ?`, userID, limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	out := make([]domain.ArchiveOverride, 0, len(rows))
	for _, row := range rows {
		o := domain.ArchiveOverride{ExternalID: row.ExternalID, UserCategory: row.UserCategory, Note: row.Note}
		if row.UserCategory {
			o.Category = row.Category
		}
		if row.Tags != "" {
			o.Tags = strings.Split(row.Tags, ",")
		}
		out = append(out, o)
	}
	return out, nil
}

func (r *Repository) ApplyOverrides(ctx context.Context, userID string, list []domain.ArchiveOverride) (int64, error) {
	data, err := json.Marshal(list)
	if err != nil {
		return 0, err
	}
	// A category name the user no longer has leaves the category as it is.
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_transaction t SET
		    category_id = CASE WHEN v.user_category AND c.id IS NOT NULL THEN c.id ELSE t.category_id END,
		    category_source = CASE WHEN v.user_category AND c.id IS NOT NULL THEN 'user' ELSE t.category_source END,
		    note = coalesce(v.note, ''), tags = coalesce(v.tags, '{}')
		FROM jsonb_to_recordset(?::jsonb) AS v(external_id text, category text, user_category boolean, note text, tags text[])
		LEFT JOIN app.fin_category c ON c.user_id = ? AND lower(c.name) = lower(v.category)
		WHERE t.user_id = ? AND t.external_id = v.external_id`, string(data), userID, userID)
	return res.RowsAffected, shared.WrapError(res.Error, "transaction")
}

type restoreRow struct {
	ExternalID  string   `json:"external_id"`
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Amount      int64    `json:"amount"`
	Category    string   `json:"category"`
	Note        string   `json:"note"`
	Tags        []string `json:"tags"`
}

func (r *Repository) RestoreTransactions(ctx context.Context, userID, accountID string, txs []domain.Transaction) (int64, error) {
	rows := make([]restoreRow, 0, len(txs))
	for _, t := range txs {
		rows = append(rows, restoreRow{ExternalID: t.ExternalID, Date: t.Date.Format(time.DateOnly), Description: t.Description,
			Amount: t.Amount, Category: t.CategoryName, Note: t.Note, Tags: t.Tags})
	}
	data, err := json.Marshal(rows)
	if err != nil {
		return 0, err
	}
	var n int64
	// Rows already in (a part sent twice) are skipped and leave the balance alone.
	err = r.db.WithContext(ctx).Raw(`
		WITH v AS (
		    SELECT * FROM jsonb_to_recordset(?::jsonb)
		        AS v(external_id text, date date, description text, amount bigint, category text, note text, tags text[])
		), ins AS (
		    INSERT INTO app.fin_transaction (user_id, account_id, external_id, date, description, amount_cents, status,
		                                     category_id, category_source, note, tags)
		    SELECT ?, ?, v.external_id, v.date, v.description, v.amount, 'POSTED',
		           c.id, CASE WHEN c.id IS NULL THEN 'auto' ELSE 'user' END, coalesce(v.note, ''), coalesce(v.tags, '{}')
		    FROM v LEFT JOIN app.fin_category c ON c.user_id = ? AND lower(c.name) = lower(v.category)
		    ON CONFLICT (external_id) DO NOTHING
		    RETURNING amount_cents
		), upd AS (
		    UPDATE app.fin_account a SET balance_cents = a.balance_cents + `+
		strings.ReplaceAll(balanceDelta, "%s", "(SELECT coalesce(sum(amount_cents), 0) FROM ins)")+`
		    WHERE a.id = ? AND a.user_id = ?
		)
		SELECT count(*) FROM ins`, string(data), userID, accountID, userID, accountID, userID).Row().Scan(&n)
	return n, shared.WrapError(err, "transaction")
}
