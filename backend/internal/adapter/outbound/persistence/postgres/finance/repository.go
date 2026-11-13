package finance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/raulaguila/fin/internal/adapter/outbound/persistence/postgres/shared"
	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/port/output"
	"github.com/raulaguila/fin/pkg/apperror"
)

// Repository owns the app.fin_* tables. Every user-facing method filters by user_id.
type Repository struct {
	db *gorm.DB
}

var _ output.FinanceRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func notFound(resource string) error {
	return apperror.New(apperror.CodeNotFound, resource+" not found").WithDetails("resource", resource)
}

// ---- connections ----

type connectionRow struct {
	ID               uuid.UUID  `gorm:"column:id"`
	UserID           uuid.UUID  `gorm:"column:user_id"`
	ItemID           string     `gorm:"column:item_id"`
	Institution      string     `gorm:"column:institution"`
	ImageURL         string     `gorm:"column:image_url"`
	Status           string     `gorm:"column:status"`
	Error            string     `gorm:"column:error"`
	ConsentExpiresAt *time.Time `gorm:"column:consent_expires_at"`
	LastSyncAt       *time.Time `gorm:"column:last_sync_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
}

func (r connectionRow) toDomain() domain.Connection {
	return domain.Connection{
		ID: r.ID.String(), UserID: r.UserID.String(), ItemID: r.ItemID, Institution: r.Institution, ImageURL: r.ImageURL,
		Status: r.Status, Error: r.Error, ConsentExpiresAt: r.ConsentExpiresAt, LastSyncAt: r.LastSyncAt, CreatedAt: r.CreatedAt,
	}
}

const connectionColumns = `id, user_id, item_id, institution, image_url, status, error, consent_expires_at, last_sync_at, created_at`

func (r *Repository) CreateConnection(ctx context.Context, c *domain.Connection) error {
	var row connectionRow
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO app.fin_connection (user_id, item_id, institution, image_url, status, error, consent_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING `+connectionColumns,
		c.UserID, c.ItemID, c.Institution, c.ImageURL, c.Status, c.Error, c.ConsentExpiresAt).Take(&row).Error
	if err != nil {
		return shared.WrapError(err, "connection")
	}
	*c = row.toDomain()
	return nil
}

func (r *Repository) Connections(ctx context.Context, userID string) ([]domain.Connection, error) {
	var rows []connectionRow
	err := r.db.WithContext(ctx).Raw(`SELECT `+connectionColumns+` FROM app.fin_connection WHERE user_id = ? ORDER BY created_at, id LIMIT 100`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "connection")
	}
	accounts, err := r.Accounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Connection, 0, len(rows))
	for _, row := range rows {
		c := row.toDomain()
		for _, a := range accounts {
			if a.ConnectionID == c.ID {
				c.Accounts = append(c.Accounts, a)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *Repository) findConnection(ctx context.Context, where string, arg any) (*domain.Connection, error) {
	var row connectionRow
	err := r.db.WithContext(ctx).Raw(`SELECT `+connectionColumns+` FROM app.fin_connection WHERE `+where, arg).Take(&row).Error
	if err != nil {
		return nil, shared.WrapError(err, "connection")
	}
	c := row.toDomain()
	return &c, nil
}

func (r *Repository) FindConnection(ctx context.Context, id string) (*domain.Connection, error) {
	return r.findConnection(ctx, "id = ?", id)
}

func (r *Repository) ConnectionByItem(ctx context.Context, itemID string) (*domain.Connection, error) {
	return r.findConnection(ctx, "item_id = ?", itemID)
}

func (r *Repository) ConnectionIDs(ctx context.Context) ([]string, error) {
	var ids []string
	// shortcut: one batch of 10 000 connections; page it if the app ever serves that many.
	err := r.db.WithContext(ctx).Raw(`SELECT id::text FROM app.fin_connection WHERE item_id NOT LIKE '` + domain.ManualItemPrefix + `%' ORDER BY created_at, id LIMIT 10000`).Find(&ids).Error
	return ids, shared.WrapError(err, "connection")
}

func (r *Repository) SaveConnectionState(ctx context.Context, c *domain.Connection, synced bool) error {
	err := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_connection
		SET institution = ?, image_url = ?, status = ?, error = ?, consent_expires_at = ?,
		    last_sync_at = CASE WHEN ? THEN now() ELSE last_sync_at END
		WHERE id = ?`,
		c.Institution, c.ImageURL, c.Status, c.Error, c.ConsentExpiresAt, synced, c.ID).Error
	return shared.WrapError(err, "connection")
}

func (r *Repository) DeleteConnection(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_connection WHERE id = ? AND user_id = ?`, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "connection")
	}
	if res.RowsAffected == 0 {
		return notFound("connection")
	}
	return nil
}

// ---- accounts ----

type accountRow struct {
	ID            uuid.UUID  `gorm:"column:id"`
	ConnectionID  uuid.UUID  `gorm:"column:connection_id"`
	ExternalID    string     `gorm:"column:external_id"`
	Type          string     `gorm:"column:type"`
	Subtype       string     `gorm:"column:subtype"`
	Name          string     `gorm:"column:name"`
	Number        string     `gorm:"column:number"`
	Balance       int64      `gorm:"column:balance_cents"`
	CreditLimit   *int64     `gorm:"column:credit_limit_cents"`
	Available     *int64     `gorm:"column:available_cents"`
	BillCloseDate *time.Time `gorm:"column:bill_close_date"`
	BillDueDate   *time.Time `gorm:"column:bill_due_date"`
	LastBillDue   *time.Time `gorm:"column:last_bill_due"`
	LastBill      *int64     `gorm:"column:last_bill_cents"`
}

func (r accountRow) toDomain() domain.Account {
	return domain.Account{
		ID: r.ID.String(), ConnectionID: r.ConnectionID.String(), ExternalID: r.ExternalID, Type: r.Type, Subtype: r.Subtype, Name: r.Name,
		Number: r.Number, Balance: r.Balance, CreditLimit: r.CreditLimit, Available: r.Available,
		BillCloseDate: r.BillCloseDate, BillDueDate: r.BillDueDate, LastBillDue: r.LastBillDue, LastBill: r.LastBill,
	}
}

const accountColumns = `id, connection_id, external_id, type, subtype, name, number, balance_cents, credit_limit_cents, available_cents,
	bill_close_date, bill_due_date, last_bill_due, last_bill_cents`

func (r *Repository) UpsertAccount(ctx context.Context, userID, connectionID string, a domain.OFAccount) (*domain.Account, error) {
	var rows []accountRow
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO app.fin_account AS t (user_id, connection_id, external_id, type, subtype, name, number, balance_cents,
		                                  credit_limit_cents, available_cents, bill_close_date, bill_due_date, last_bill_due, last_bill_cents)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (external_id) DO UPDATE SET
		    type = EXCLUDED.type, subtype = EXCLUDED.subtype, name = EXCLUDED.name, number = EXCLUDED.number,
		    balance_cents = EXCLUDED.balance_cents, credit_limit_cents = EXCLUDED.credit_limit_cents,
		    available_cents = EXCLUDED.available_cents, bill_close_date = EXCLUDED.bill_close_date,
		    bill_due_date = EXCLUDED.bill_due_date, last_bill_due = EXCLUDED.last_bill_due,
		    last_bill_cents = EXCLUDED.last_bill_cents
		WHERE t.user_id = EXCLUDED.user_id
		RETURNING `+accountColumns,
		userID, connectionID, a.ID, a.Type, a.Subtype, a.Name, a.Number, a.Balance, a.CreditLimit, a.Available, a.BillCloseDate,
		a.BillDueDate, a.LastBillDue, a.LastBill).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "account")
	}
	if len(rows) == 0 { // the external id belongs to another user
		return nil, apperror.New(apperror.CodeConflict, "account belongs to another user")
	}
	acc := rows[0].toDomain()
	return &acc, nil
}

func (r *Repository) Accounts(ctx context.Context, userID string) ([]domain.Account, error) {
	var rows []accountRow
	err := r.db.WithContext(ctx).Raw(`SELECT `+accountColumns+` FROM app.fin_account WHERE user_id = ? ORDER BY type, name, id LIMIT 500`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "account")
	}
	out := make([]domain.Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

// ---- transactions ----

const upsertBatch = 500 // 13 parameters each, far under the 65 535 limit

func (r *Repository) UpsertTransactions(ctx context.Context, userID, accountID string, txs []domain.Transaction) (int, error) {
	inserted := 0
	for start := 0; start < len(txs); start += upsertBatch {
		batch := txs[start:min(start+upsertBatch, len(txs))]
		values := make([]string, 0, len(batch))
		args := make([]any, 0, len(batch)*13)
		for _, t := range batch {
			values = append(values, "(?, ?, ?, ?::date, ?, ?, ?, ?, NULLIF(?, '')::uuid, ?, NULLIF(?, 0), NULLIF(?, 0), now())")
			args = append(args, userID, accountID, t.ExternalID, t.Date.Format(time.DateOnly), t.Description, t.Amount,
				t.Status, t.ProviderCategory, t.CategoryID, t.CategorySource, t.Installment, t.Installments)
		}
		var news []bool
		err := r.db.WithContext(ctx).Raw(`
			INSERT INTO app.fin_transaction AS t (user_id, account_id, external_id, date, description, amount_cents, status,
			                                      provider_category, category_id, category_source, installment, installments, created_at)
			VALUES `+strings.Join(values, ", ")+`
			ON CONFLICT (external_id) DO UPDATE SET
			    date = EXCLUDED.date, description = EXCLUDED.description,
			    -- split parts keep their share; the original keeps the rest
			    amount_cents = EXCLUDED.amount_cents - coalesce((SELECT sum(s.amount_cents) FROM app.fin_transaction s WHERE s.split_of = t.id), 0),
			    status = EXCLUDED.status, provider_category = EXCLUDED.provider_category,
			    installment = EXCLUDED.installment, installments = EXCLUDED.installments,
			    category_id = CASE WHEN t.category_source = 'user' THEN t.category_id ELSE EXCLUDED.category_id END,
			    category_source = CASE WHEN t.category_source = 'user' THEN 'user' ELSE EXCLUDED.category_source END
			WHERE t.user_id = EXCLUDED.user_id
			RETURNING (xmax = 0)`, args...).Find(&news).Error
		if err != nil {
			return inserted, shared.WrapError(err, "transaction")
		}
		for _, isNew := range news {
			if isNew {
				inserted++
			}
		}
	}
	return inserted, nil
}

type transactionRow struct {
	ID               uuid.UUID  `gorm:"column:id"`
	AccountID        uuid.UUID  `gorm:"column:account_id"`
	AccountName      string     `gorm:"column:account_name"`
	AccountType      string     `gorm:"column:account_type"`
	Institution      string     `gorm:"column:institution"`
	ExternalID       string     `gorm:"column:external_id"`
	Date             time.Time  `gorm:"column:date"`
	Description      string     `gorm:"column:description"`
	Amount           int64      `gorm:"column:amount_cents"`
	Status           string     `gorm:"column:status"`
	ProviderCategory string     `gorm:"column:provider_category"`
	CategoryID       *uuid.UUID `gorm:"column:category_id"`
	CategoryName     *string    `gorm:"column:category_name"`
	CategoryKind     *string    `gorm:"column:category_kind"`
	CategorySource   string     `gorm:"column:category_source"`
	Installment      *int       `gorm:"column:installment"`
	Installments     *int       `gorm:"column:installments"`
	SplitOf          *uuid.UUID `gorm:"column:split_of"`
	Note             string     `gorm:"column:note"` // only the list and export select it
	Tags             string     `gorm:"column:tags"` // comma-joined; tags never hold a comma
}

func (r transactionRow) toDomain() domain.Transaction {
	t := domain.Transaction{
		ID: r.ID.String(), AccountID: r.AccountID.String(), AccountName: r.AccountName, AccountType: r.AccountType,
		Institution: r.Institution, ExternalID: r.ExternalID, Date: r.Date, Description: r.Description, Amount: r.Amount,
		Status: r.Status, ProviderCategory: r.ProviderCategory, CategorySource: r.CategorySource, Note: r.Note,
	}
	if r.Tags != "" {
		t.Tags = strings.Split(r.Tags, ",")
	}
	if r.CategoryID != nil {
		t.CategoryID = r.CategoryID.String()
	}
	if r.CategoryName != nil {
		t.CategoryName = *r.CategoryName
	}
	if r.CategoryKind != nil {
		t.CategoryKind = *r.CategoryKind
	}
	if r.Installment != nil {
		t.Installment = *r.Installment
	}
	if r.Installments != nil {
		t.Installments = *r.Installments
	}
	if r.SplitOf != nil {
		t.SplitOf = r.SplitOf.String()
	}
	return t
}

const transactionSelect = `SELECT ` + transactionColumns + transactionFrom

// transactionDetailSelect adds what only screens that show one row in full need.
const transactionDetailSelect = `SELECT ` + transactionColumns + `, t.note, array_to_string(t.tags, ',') AS tags` + transactionFrom

const transactionColumns = `t.id, t.account_id, a.name AS account_name, a.type AS account_type, c.institution, t.external_id, t.date,
	       t.description, t.amount_cents, t.status, t.provider_category, t.category_id, cat.name AS category_name,
	       cat.kind AS category_kind, t.category_source, t.installment, t.installments, t.split_of`

const transactionFrom = `
	FROM app.fin_transaction t
	JOIN app.fin_account a ON a.id = t.account_id
	JOIN app.fin_connection c ON c.id = a.connection_id
	LEFT JOIN app.fin_category cat ON cat.id = t.category_id`

// transactionWhere turns the filter into a condition on t (the transaction table).
func transactionWhere(userID string, f domain.TransactionFilter) (string, []any) {
	where := []string{"t.user_id = ?"}
	args := []any{userID}
	if !f.From.IsZero() {
		where, args = append(where, "t.date >= ?::date"), append(args, f.From.Format(time.DateOnly))
	}
	if !f.To.IsZero() {
		where, args = append(where, "t.date < ?::date"), append(args, f.To.Format(time.DateOnly))
	}
	if f.Search != "" {
		// shortcut: leading-wildcard ILIKE served by the trigram index; fine to millions of rows per user.
		where, args = append(where, "t.description ILIKE ?"), append(args, shared.Contains(f.Search))
	}
	if f.AccountID != "" {
		where, args = append(where, "t.account_id = ?"), append(args, f.AccountID)
	}
	switch f.CategoryID {
	case "":
	case "none":
		where = append(where, "t.category_id IS NULL")
	default:
		where, args = append(where, "t.category_id = ?"), append(args, f.CategoryID)
	}
	if f.Tag != "" {
		where, args = append(where, "t.tags @> ARRAY[?]::text[]"), append(args, f.Tag) // @> uses the GIN index
	}
	return strings.Join(where, " AND "), args
}

func (r *Repository) CountTransactions(ctx context.Context, userID string, f domain.TransactionFilter) (int64, error) {
	cond, args := transactionWhere(userID, f)
	var total int64
	err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM app.fin_transaction t WHERE `+cond, args...).Take(&total).Error
	return total, shared.WrapError(err, "transaction")
}

func (r *Repository) Transactions(ctx context.Context, userID string, f domain.TransactionFilter) ([]domain.Transaction, int64, error) {
	total, err := r.CountTransactions(ctx, userID, f)
	if err != nil {
		return nil, 0, err
	}
	cond, args := transactionWhere(userID, f)
	var rows []transactionRow
	// shortcut: OFFSET pages; a person's history stays in the tens of thousands of rows.
	err = r.db.WithContext(ctx).Raw(transactionDetailSelect+` WHERE `+cond+` ORDER BY t.date DESC, t.created_at DESC, t.id LIMIT ? OFFSET ?`,
		append(args, f.Page.Limit, f.Page.Offset())...).Find(&rows).Error
	if err != nil {
		return nil, 0, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), total, nil
}

func (r *Repository) ExportTransactions(ctx context.Context, userID string, f domain.TransactionFilter, limit int) ([]domain.Transaction, error) {
	cond, args := transactionWhere(userID, f)
	var rows []transactionRow
	err := r.db.WithContext(ctx).Raw(transactionDetailSelect+` WHERE `+cond+` ORDER BY t.date DESC, t.created_at DESC, t.id LIMIT ?`,
		append(args, limit)...).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), nil
}

func (r *Repository) Spending(ctx context.Context, userID string, from, to time.Time, limit int) ([]domain.Transaction, error) {
	var rows []transactionRow
	err := r.db.WithContext(ctx).Raw(transactionSelect+`
		WHERE t.user_id = ? AND t.date >= ?::date AND t.date < ?::date AND t.amount_cents < 0
		  AND (cat.kind IS NULL OR cat.kind = 'expense')
		ORDER BY t.date DESC, t.id LIMIT ?`,
		userID, from.Format(time.DateOnly), to.Format(time.DateOnly), limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), nil
}

func (r *Repository) SetTransactionCategory(ctx context.Context, userID, id, categoryID string) (string, error) {
	var desc []string
	err := r.db.WithContext(ctx).Raw(`
		UPDATE app.fin_transaction SET category_id = NULLIF(?, '')::uuid, category_source = 'user'
		WHERE id = ? AND user_id = ? RETURNING description`, categoryID, id, userID).Find(&desc).Error
	if err != nil {
		return "", shared.WrapError(err, "transaction")
	}
	if len(desc) == 0 {
		return "", notFound("transaction")
	}
	return desc[0], nil
}

func scanTransactions(rows []transactionRow) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out
}

func (r *Repository) Categorizable(ctx context.Context, userID string, limit int) ([]domain.Transaction, error) {
	var rows []transactionRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, description, amount_cents, provider_category, category_id, category_source
		FROM app.fin_transaction WHERE user_id = ? AND category_source <> 'user'
		ORDER BY date DESC, id LIMIT ?`, userID, limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), nil
}

const setCategoriesBatch = 1000 // 3 parameters each

func (r *Repository) SetCategories(ctx context.Context, userID string, txs []domain.Transaction) (int64, error) {
	var changed int64
	for start := 0; start < len(txs); start += setCategoriesBatch {
		batch := txs[start:min(start+setCategoriesBatch, len(txs))]
		values := make([]string, 0, len(batch))
		args := make([]any, 0, len(batch)*3+1)
		for _, t := range batch {
			values = append(values, "(?::uuid, NULLIF(?, '')::uuid, ?)")
			args = append(args, t.ID, t.CategoryID, t.CategorySource)
		}
		res := r.db.WithContext(ctx).Exec(`
			UPDATE app.fin_transaction t SET category_id = v.category_id, category_source = v.source
			FROM (VALUES `+strings.Join(values, ", ")+`) AS v(id, category_id, source)
			WHERE t.id = v.id AND t.user_id = ? AND t.category_source <> 'user'`, append(args, userID)...)
		if res.Error != nil {
			return changed, shared.WrapError(res.Error, "transaction")
		}
		changed += res.RowsAffected
	}
	return changed, nil
}

func (r *Repository) Uncategorized(ctx context.Context, userID string, from time.Time, limit int) ([]domain.Transaction, error) {
	var rows []transactionRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT date, description, amount_cents FROM app.fin_transaction
		WHERE user_id = ? AND date >= ?::date AND category_id IS NULL
		ORDER BY date DESC, id LIMIT ?`, userID, from.Format(time.DateOnly), limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), nil
}

func (r *Repository) PersonTransfers(ctx context.Context, userID string, from time.Time, limit int) ([]domain.Transaction, error) {
	var rows []transactionRow
	// shortcut: prefix ILIKEs filtered after the (user_id, date) index; fine for a person's year of history.
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.date, t.description, t.amount_cents, t.provider_category, cat.name AS category_name, cat.kind AS category_kind
		FROM app.fin_transaction t LEFT JOIN app.fin_category cat ON cat.id = t.category_id
		WHERE t.user_id = ? AND t.date >= ?::date
		  AND (t.description ILIKE 'pix enviado%' OR t.description ILIKE 'pix recebido%' OR t.description ILIKE 'ted %'
		       OR t.description ILIKE 'transfer%')
		ORDER BY t.date DESC, t.id LIMIT ?`, userID, from.Format(time.DateOnly), limit).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	return scanTransactions(rows), nil
}

type monthRow struct {
	Month   time.Time `gorm:"column:month"`
	Income  int64     `gorm:"column:income"`
	Expense int64     `gorm:"column:expense"`
}

func (r *Repository) MonthlyTotals(ctx context.Context, userID string, from, to time.Time) ([]domain.MonthTotal, error) {
	var rows []monthRow
	// Same rule as the dashboard: uncategorized money counts by its sign, transfers nowhere.
	err := r.db.WithContext(ctx).Raw(`
		SELECT date_trunc('month', t.date)::date AS month,
		       coalesce(sum(t.amount_cents) FILTER (WHERE cat.kind = 'income' OR (cat.kind IS NULL AND t.amount_cents > 0)), 0)::bigint AS income,
		       coalesce(sum(t.amount_cents) FILTER (WHERE cat.kind = 'expense' OR (cat.kind IS NULL AND t.amount_cents < 0)), 0)::bigint AS expense
		FROM app.fin_transaction t LEFT JOIN app.fin_category cat ON cat.id = t.category_id
		WHERE t.user_id = ? AND t.date >= ?::date AND t.date < ?::date
		GROUP BY 1 ORDER BY 1 LIMIT 120`, userID, from.Format(time.DateOnly), to.Format(time.DateOnly)).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	out := make([]domain.MonthTotal, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.MonthTotal{Month: row.Month, Income: row.Income, Expense: row.Expense})
	}
	return out, nil
}

type totalRow struct {
	CategoryID *uuid.UUID `gorm:"column:category_id"`
	Name       *string    `gorm:"column:name"`
	Kind       *string    `gorm:"column:kind"`
	Limit      *int64     `gorm:"column:limit_cents"`
	Total      int64      `gorm:"column:total"`
	Count      int64      `gorm:"column:n"`
}

func (r *Repository) Totals(ctx context.Context, userID string, from, to time.Time) ([]domain.CategoryTotal, error) {
	var rows []totalRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.category_id, cat.name, cat.kind, cat.limit_cents, sum(t.amount_cents)::bigint AS total, count(*) AS n
		FROM app.fin_transaction t
		LEFT JOIN app.fin_category cat ON cat.id = t.category_id
		WHERE t.user_id = ? AND t.date >= ?::date AND t.date < ?::date
		GROUP BY t.category_id, cat.name, cat.kind, cat.limit_cents,
		         CASE WHEN t.category_id IS NULL THEN t.amount_cents > 0 END`,
		userID, from.Format(time.DateOnly), to.Format(time.DateOnly)).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "transaction")
	}
	out := make([]domain.CategoryTotal, 0, len(rows))
	for _, row := range rows {
		ct := domain.CategoryTotal{Total: row.Total, Count: row.Count}
		if row.CategoryID != nil {
			ct.CategoryID, ct.Name, ct.Kind = row.CategoryID.String(), *row.Name, *row.Kind
			if row.Limit != nil {
				ct.Limit = *row.Limit
			}
		}
		out = append(out, ct)
	}
	return out, nil
}

// ---- categories ----

type categoryRow struct {
	ID    uuid.UUID `gorm:"column:id"`
	Name  string    `gorm:"column:name"`
	Kind  string    `gorm:"column:kind"`
	Limit int64     `gorm:"column:limit_cents"`
	Rollover bool   `gorm:"column:rollover"`
}

func (r *Repository) EnsureCategories(ctx context.Context, userID string, defaults []domain.Category) error {
	if len(defaults) == 0 {
		return nil
	}
	values := make([]string, 0, len(defaults))
	args := []any{userID}
	for _, c := range defaults {
		values = append(values, "(?, ?, ?::bigint)")
		args = append(args, c.Name, c.Kind, c.Limit)
	}
	args = append(args, userID)
	err := r.db.WithContext(ctx).Exec(`
		INSERT INTO app.fin_category (user_id, name, kind, limit_cents)
		SELECT ?::uuid, v.name, v.kind, v.limit_cents FROM (VALUES `+strings.Join(values, ", ")+`) AS v(name, kind, limit_cents)
		WHERE NOT EXISTS (SELECT 1 FROM app.fin_category WHERE user_id = ?)
		ON CONFLICT DO NOTHING`, args...).Error
	return shared.WrapError(err, "category")
}

func (r *Repository) Categories(ctx context.Context, userID string) ([]domain.Category, error) {
	var rows []categoryRow
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, name, kind, limit_cents, rollover FROM app.fin_category WHERE user_id = ?
		ORDER BY CASE kind WHEN 'expense' THEN 0 WHEN 'income' THEN 1 ELSE 2 END, name LIMIT 500`, userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "category")
	}
	out := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Category{ID: row.ID.String(), Name: row.Name, Kind: row.Kind, Limit: row.Limit, Rollover: row.Rollover})
	}
	return out, nil
}

func (r *Repository) CreateCategory(ctx context.Context, userID string, c *domain.Category) error {
	var id string
	err := r.db.WithContext(ctx).Raw(`
		INSERT INTO app.fin_category (user_id, name, kind, limit_cents) VALUES (?, ?, ?, ?) RETURNING id`,
		userID, c.Name, c.Kind, c.Limit).Take(&id).Error
	if err != nil {
		return shared.WrapError(err, "category")
	}
	c.ID = id
	return nil
}

func (r *Repository) UpdateCategory(ctx context.Context, userID string, c *domain.Category) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE app.fin_category SET name = ?, limit_cents = ?, rollover = ? WHERE id = ? AND user_id = ?`,
		c.Name, c.Limit, c.Rollover, c.ID, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "category")
	}
	if res.RowsAffected == 0 {
		return notFound("category")
	}
	return nil
}

func (r *Repository) DeleteCategory(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_category WHERE id = ? AND user_id = ?`, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "category")
	}
	if res.RowsAffected == 0 {
		return notFound("category")
	}
	return nil
}

// ---- rules ----

type ruleRow struct {
	ID           uuid.UUID `gorm:"column:id"`
	Keyword      string    `gorm:"column:keyword"`
	CategoryID   uuid.UUID `gorm:"column:category_id"`
	CategoryName string    `gorm:"column:category_name"`
	Hits         int64     `gorm:"column:hits"`
}

// likeKeyword escapes LIKE wildcards inside SQL, matching shared.Contains.
const likeKeyword = `'%' || replace(replace(replace(r.keyword, '\', '\\'), '%', '\%'), '_', '\_') || '%'`

// shortcut: hits counts each rule with an ILIKE scan of the user's transactions; cache it past ~100k rows.
func (r *Repository) Rules(ctx context.Context, userID string) ([]domain.Rule, error) {
	var rows []ruleRow
	err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT r.id, r.keyword, r.category_id, c.name AS category_name,
		       (SELECT count(*) FROM app.fin_transaction t WHERE t.user_id = r.user_id AND t.description ILIKE %s) AS hits
		FROM app.fin_rule r JOIN app.fin_category c ON c.id = r.category_id
		WHERE r.user_id = ? ORDER BY r.created_at DESC, r.id LIMIT 500`, likeKeyword), userID).Find(&rows).Error
	if err != nil {
		return nil, shared.WrapError(err, "rule")
	}
	out := make([]domain.Rule, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Rule{ID: row.ID.String(), Keyword: row.Keyword, CategoryID: row.CategoryID.String(), CategoryName: row.CategoryName, Hits: row.Hits})
	}
	return out, nil
}

func (r *Repository) CreateRule(ctx context.Context, userID string, rule *domain.Rule) error {
	var id string
	err := r.db.WithContext(ctx).Raw(`INSERT INTO app.fin_rule (user_id, keyword, category_id) VALUES (?, ?, ?) RETURNING id`,
		userID, rule.Keyword, rule.CategoryID).Take(&id).Error
	if err != nil {
		return shared.WrapError(err, "rule")
	}
	rule.ID = id
	return nil
}

func (r *Repository) DeleteRule(ctx context.Context, userID, id string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM app.fin_rule WHERE id = ? AND user_id = ?`, id, userID)
	if res.Error != nil {
		return shared.WrapError(res.Error, "rule")
	}
	if res.RowsAffected == 0 {
		return notFound("rule")
	}
	return nil
}

func (r *Repository) CountMatches(ctx context.Context, userID, keyword string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Raw(`SELECT count(*) FROM app.fin_transaction WHERE user_id = ? AND description ILIKE ?`,
		userID, shared.Contains(keyword)).Take(&n).Error
	return n, shared.WrapError(err, "transaction")
}

func (r *Repository) ApplyRule(ctx context.Context, userID, keyword, categoryID string) (int64, error) {
	res := r.db.WithContext(ctx).Exec(`
		UPDATE app.fin_transaction SET category_id = ?, category_source = 'rule'
		WHERE user_id = ? AND description ILIKE ? AND category_source <> 'user'`,
		categoryID, userID, shared.Contains(keyword))
	return res.RowsAffected, shared.WrapError(res.Error, "transaction")
}

// ---- alerts ----

func (r *Repository) MarkAlert(ctx context.Context, userID, key string) (bool, error) {
	res := r.db.WithContext(ctx).Exec(`INSERT INTO app.fin_alert (user_id, key) VALUES (?, ?) ON CONFLICT DO NOTHING`, userID, key)
	return res.RowsAffected == 1, shared.WrapError(res.Error, "alert")
}
