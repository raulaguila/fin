// Package finance is the personal finance of each user: bank connections
// through Open Finance, transactions, budgets, alerts and the assistant.
package finance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/policy"
	"github.com/raulaguila/fin/internal/core/port/input"
	"github.com/raulaguila/fin/internal/core/port/output"
	"github.com/raulaguila/fin/pkg/apperror"
	"github.com/raulaguila/fin/pkg/validate"
)

const (
	maxSearchBytes = 120
	// firstSyncDays is how far back the first sync reads; later syncs reread a
	// week before the last one to pick up late postings and status changes.
	firstSyncDays = 365
	resyncDays    = 7
)

// ErrStillUpdating makes the sync job retry while the bank is still sending data.
var ErrStillUpdating = errors.New("open finance item is still updating")

// Service implements input.FinanceService. OF and AI may be disabled; AI is nil then.
type Service struct {
	Store output.UnitOfWork
	Tx    output.TxManager
	OF    output.OpenFinance
	AI    output.Assistant
	Clock output.Clock
	Log   output.Logger
	// Mail seals alert emails for the mailer; nil sends none (SMTP off).
	Mail output.EmailTokenCodec
}

var _ input.FinanceService = (*Service)(nil)

func New(store output.UnitOfWork, tx output.TxManager, of output.OpenFinance, ai output.Assistant, clock output.Clock, log output.Logger) *Service {
	return &Service{Store: store, Tx: tx, OF: of, AI: ai, Clock: clock, Log: output.Log(log)}
}

func (s *Service) repo() output.FinanceRepository { return s.Store.Finance() }

func (s *Service) OpenFinanceEnabled() bool { return s.OF != nil && s.OF.Enabled() }
func (s *Service) AssistantEnabled() bool   { return s.AI != nil }

func errManual() error {
	return apperror.New(apperror.CodeConflict, "an imported account has no bank to reach").WithDetails("feature", "manual")
}

func disabled(what string) error {
	return apperror.New(apperror.CodeConflict, what+" is not configured").WithDetails("feature", what)
}

// ---- connections ----

// ownConnection reads a connection and answers not found when it is another user's.
func (s *Service) ownConnection(ctx context.Context, userID, id string) (*domain.Connection, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	c, err := s.repo().FindConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.UserID != userID {
		return nil, apperror.New(apperror.CodeNotFound, "connection not found").WithDetails("resource", "connection")
	}
	return c, nil
}

func (s *Service) ConnectToken(ctx context.Context, userID, connectionID string) (string, error) {
	if !s.OpenFinanceEnabled() {
		return "", disabled("open finance")
	}
	itemID := ""
	if connectionID != "" {
		c, err := s.ownConnection(ctx, userID, connectionID)
		if err != nil {
			return "", err
		}
		if c.Manual() {
			return "", errManual()
		}
		itemID = c.ItemID
	}
	return s.OF.ConnectToken(ctx, userID, itemID)
}

func (s *Service) AddConnection(ctx context.Context, userID, itemID string) (*domain.Connection, error) {
	if !s.OpenFinanceEnabled() {
		return nil, disabled("open finance")
	}
	if err := validate.ID(itemID); err != nil {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid item id").WithField("itemId")
	}
	item, err := s.OF.Item(ctx, itemID)
	if err != nil {
		return nil, err
	}
	// The connect token carries the user id: an item made for anyone else is refused.
	if item.ClientUserID != userID {
		return nil, apperror.New(apperror.CodeForbidden, "this connection belongs to another user")
	}
	if existing, err := s.repo().ConnectionByItem(ctx, itemID); err == nil {
		if existing.UserID != userID {
			return nil, apperror.New(apperror.CodeForbidden, "this connection belongs to another user")
		}
		return existing, s.enqueueSync(ctx, s.Store, existing.ID)
	} else if !apperror.IsNotFound(err) {
		return nil, err
	}

	c := &domain.Connection{UserID: userID, ItemID: itemID}
	applyItem(c, item)
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if err := uow.Finance().EnsureCategories(ctx, userID, DefaultCategories); err != nil {
			return err
		}
		if err := uow.Finance().CreateConnection(ctx, c); err != nil {
			return err
		}
		return s.enqueueSync(ctx, uow, c.ID)
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) Connections(ctx context.Context, userID string) ([]domain.Connection, error) {
	return s.repo().Connections(ctx, userID)
}

func (s *Service) SyncConnection(ctx context.Context, userID, id string) error {
	if !s.OpenFinanceEnabled() {
		return disabled("open finance")
	}
	c, err := s.ownConnection(ctx, userID, id)
	if err != nil {
		return err
	}
	if c.Manual() {
		return errManual()
	}
	// Ask the bank for fresh data; the sync job reads it once the item is updated.
	if err := s.OF.RefreshItem(ctx, c.ItemID); err != nil {
		return err
	}
	c.Status, c.Error = domain.ConnSyncing, ""
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if err := uow.Finance().SaveConnectionState(ctx, c, false); err != nil {
			return err
		}
		return s.enqueueSync(ctx, uow, c.ID)
	})
}

func (s *Service) DeleteConnection(ctx context.Context, userID, id string) error {
	c, err := s.ownConnection(ctx, userID, id)
	if err != nil {
		return err
	}
	if s.OpenFinanceEnabled() && !c.Manual() {
		// Revoke at the provider first: a local delete alone leaves the consent alive.
		if err := s.OF.DeleteItem(ctx, c.ItemID); err != nil {
			return err
		}
	}
	return s.repo().DeleteConnection(ctx, userID, id)
}

func (s *Service) Webhook(ctx context.Context, event, itemID string) error {
	if itemID == "" {
		return nil
	}
	c, err := s.repo().ConnectionByItem(ctx, itemID)
	if apperror.IsNotFound(err) {
		return nil // an item this app never saved, or one already removed
	}
	if err != nil {
		return err
	}
	s.Log.Info("Open finance webhook: " + validate.Text(event, 60))
	return s.enqueueSync(ctx, s.Store, c.ID)
}

// ---- sync ----

func (s *Service) enqueueSync(ctx context.Context, uow output.UnitOfWork, connectionID string) error {
	return policy.EnqueueJob(ctx, uow, JobSync, syncJob{ConnectionID: connectionID})
}

// connStatus maps the provider item status to the connection status.
func connStatus(status string) string {
	switch status {
	case "UPDATED":
		return domain.ConnOK
	case "UPDATING", "CREATED":
		return domain.ConnSyncing
	case "LOGIN_ERROR", "WAITING_USER_INPUT", "WAITING_USER_ACTION":
		return domain.ConnLoginRequired
	default: // OUTDATED and anything new
		return domain.ConnError
	}
}

// meuPluggy is the connector that shares banks linked at meu.pluggy.ai: one item per
// bank, all with this name, so the bank comes from the accounts instead (bankName).
const meuPluggy = "MeuPluggy"

// banks names the COMPE codes of the banks people use most; others keep the connector name.
var banks = map[string]string{
	"001": "Banco do Brasil", "033": "Santander", "041": "Banrisul", "070": "BRB", "077": "Inter",
	"104": "Caixa", "208": "BTG Pactual", "212": "Banco Original", "237": "Bradesco", "260": "Nubank",
	"290": "PagBank", "323": "Mercado Pago", "336": "C6 Bank", "341": "Itaú", "380": "PicPay",
	"422": "Safra", "748": "Sicredi", "756": "Sicoob",
}

// bankName is the bank of the first account whose code is known, or "".
func bankName(accounts []domain.OFAccount) string {
	for _, a := range accounts {
		if code, err := strconv.Atoi(a.BankCode); err == nil && banks[fmt.Sprintf("%03d", code)] != "" {
			return banks[fmt.Sprintf("%03d", code)] // "0341" and "341" are the same bank
		}
	}
	return ""
}

func applyItem(c *domain.Connection, item *domain.OFItem) {
	if item.Institution != meuPluggy || c.Institution == "" {
		c.Institution = validate.Text(item.Institution, 200) // a MeuPluggy item keeps the bank name sync gave it
	}
	c.ImageURL = validate.Text(item.ImageURL, 500)
	c.Status, c.ConsentExpiresAt = connStatus(item.Status), item.ConsentExpiresAt
	c.Error = ""
	if c.Status != domain.ConnOK && c.Status != domain.ConnSyncing {
		c.Error = validate.Text(item.Error, 300)
	}
}

// sync reads the item, its accounts and transactions into the database.
// It is safe to repeat: everything is an upsert.
func (s *Service) sync(ctx context.Context, connectionID string) (*domain.Connection, error) {
	c, err := s.repo().FindConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if c.Manual() {
		return c, nil // imported data, nothing to read
	}
	item, err := s.OF.Item(ctx, c.ItemID)
	if err != nil {
		return nil, err
	}
	applyItem(c, item)
	if c.Status == domain.ConnSyncing {
		if err := s.repo().SaveConnectionState(ctx, c, false); err != nil {
			return nil, err
		}
		return c, ErrStillUpdating
	}
	// LOGIN_ERROR keeps no fresh data; OUTDATED still serves the last data read.
	if c.Status == domain.ConnLoginRequired {
		return c, s.repo().SaveConnectionState(ctx, c, false)
	}

	from := s.Clock.Now().AddDate(0, 0, -firstSyncDays)
	if c.LastSyncAt != nil {
		from = c.LastSyncAt.AddDate(0, 0, -resyncDays)
	}
	if err := s.repo().EnsureCategories(ctx, c.UserID, DefaultCategories); err != nil {
		return nil, err
	}
	cat, err := s.categorizer(ctx, c.UserID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.OF.Accounts(ctx, c.ItemID)
	if err != nil {
		return nil, err
	}
	if name := bankName(accounts); name != "" && item.Institution == meuPluggy {
		c.Institution = name
	}
	inserted := 0
	for _, a := range accounts {
		a.Name, a.Number, a.Subtype = validate.Text(a.Name, 200), validate.Text(a.Number, 60), validate.Text(a.Subtype, 40)
		if a.Type == domain.AccountCredit {
			// A missing bill list only hides the last bill; it must not stop the sync.
			if a.LastBillDue, a.LastBill, err = s.OF.LastBill(ctx, a.ID); err != nil {
				s.Log.Warn("Open finance bills unavailable", slog.String("error", err.Error()))
			}
		}
		acc, err := s.repo().UpsertAccount(ctx, c.UserID, c.ID, a)
		if err != nil {
			return nil, err
		}
		raw, err := s.OF.Transactions(ctx, a.ID, from)
		if err != nil {
			return nil, err
		}
		txs := make([]domain.Transaction, 0, len(raw))
		ids := make([]string, 0, len(raw))
		for _, r := range raw {
			txs = append(txs, cat.apply(normalize(r, acc.Type)))
			ids = append(ids, r.ID)
		}
		n, err := s.repo().UpsertTransactions(ctx, c.UserID, acc.ID, txs)
		if err != nil {
			return nil, err
		}
		inserted += n
		// A pending charge the bank no longer lists was replaced by the posted one.
		if _, err := s.repo().DeleteMissingPending(ctx, acc.ID, from, ids); err != nil {
			return nil, err
		}
	}
	// Investments are optional at the provider: without them the sync still counts.
	if inv, err := s.OF.Investments(ctx, c.ItemID); err != nil {
		s.Log.Warn("Open finance investments unavailable", slog.String("error", err.Error()))
	} else if err := s.repo().UpsertInvestments(ctx, c.UserID, c.ID, cleanInvestments(inv)); err != nil {
		return nil, err
	}
	if err := s.repo().SaveConnectionState(ctx, c, true); err != nil {
		return nil, err
	}
	// Older rows keep the category of an older categorizer; bring them up to date.
	changed, err := s.Recategorize(ctx, c.UserID)
	if err != nil {
		return nil, err
	}
	s.Log.Info(fmt.Sprintf("Open finance sync: %d accounts, %d new transactions, %d recategorized", len(accounts), inserted, changed))
	return c, nil
}

func cleanInvestments(list []domain.OFInvestment) []domain.OFInvestment {
	out := make([]domain.OFInvestment, 0, len(list))
	for _, i := range list {
		if i.ID == "" {
			continue
		}
		i.Name, i.Type = validate.Text(i.Name, 200), validate.Text(i.Type, 40)
		out = append(out, i)
	}
	return out
}

// normalize turns a provider transaction into the app sign: negative leaves the user.
// A card charge comes positive from the provider, so card amounts flip.
func normalize(r domain.OFTransaction, accountType string) domain.Transaction {
	amount := r.Amount
	if accountType == domain.AccountCredit {
		amount = -amount
	}
	return domain.Transaction{
		ExternalID: r.ID, Date: r.Date, Description: validate.Text(strings.Join(strings.Fields(r.Description), " "), 300),
		Amount: amount, Status: validate.Text(r.Status, 20), ProviderCategory: validate.Text(r.Category, 100),
		AccountType: accountType, Installment: r.Installment, Installments: r.Installments,
	}
}

// ---- transactions ----

func (s *Service) Transactions(ctx context.Context, userID string, f domain.TransactionFilter) ([]domain.Transaction, int64, error) {
	f.Search = strings.TrimSpace(f.Search)
	if err := validate.Search(f.Search, maxSearchBytes); err != nil {
		return nil, 0, err
	}
	if f.AccountID != "" {
		if err := validate.ID(f.AccountID); err != nil {
			return nil, 0, err
		}
	}
	if f.CategoryID != "" && f.CategoryID != "none" {
		if err := validate.ID(f.CategoryID); err != nil {
			return nil, 0, err
		}
	}
	f.Page = domain.NewPage(f.Page.Number, f.Page.Limit)
	return s.repo().Transactions(ctx, userID, f)
}

func (s *Service) SetCategory(ctx context.Context, userID, transactionID, categoryID string) (string, error) {
	if err := validate.ID(transactionID); err != nil {
		return "", err
	}
	if categoryID != "" {
		if _, err := s.ownCategory(ctx, userID, categoryID); err != nil {
			return "", err
		}
	}
	desc, err := s.repo().SetTransactionCategory(ctx, userID, transactionID, categoryID)
	if err != nil || categoryID == "" {
		return "", err
	}
	return suggestKeyword(desc), nil
}

// ---- categories ----

func (s *Service) ownCategory(ctx context.Context, userID, id string) (*domain.Category, error) {
	if err := validate.ID(id); err != nil {
		return nil, err
	}
	cats, err := s.repo().Categories(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, c := range cats {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, apperror.New(apperror.CodeNotFound, "category not found").WithDetails("resource", "category")
}

func (s *Service) Categories(ctx context.Context, userID string) ([]domain.Category, error) {
	if err := s.repo().EnsureCategories(ctx, userID, DefaultCategories); err != nil {
		return nil, err
	}
	return s.repo().Categories(ctx, userID)
}

func categoryName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > 40 || !utf8.ValidString(name) {
		return "", apperror.New(apperror.CodeInvalidInput, "name must have 1 to 40 characters").WithField("name")
	}
	return name, nil
}

func checkLimit(limit domain.Money) error {
	if limit < 0 || limit > 1_000_000_000_00 {
		return apperror.New(apperror.CodeInvalidInput, "invalid limit").WithField("limit")
	}
	return nil
}

func (s *Service) CreateCategory(ctx context.Context, userID string, c domain.Category) (*domain.Category, error) {
	var err error
	if c.Name, err = categoryName(c.Name); err != nil {
		return nil, err
	}
	if c.Kind != domain.KindExpense && c.Kind != domain.KindIncome && c.Kind != domain.KindTransfer {
		return nil, apperror.New(apperror.CodeInvalidInput, "kind must be expense, income or transfer").WithField("kind")
	}
	if err := checkLimit(c.Limit); err != nil {
		return nil, err
	}
	if err := s.repo().CreateCategory(ctx, userID, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, userID, id string, name *string, limit *domain.Money) (*domain.Category, error) {
	c, err := s.ownCategory(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		if c.Name, err = categoryName(*name); err != nil {
			return nil, err
		}
	}
	if limit != nil {
		if err := checkLimit(*limit); err != nil {
			return nil, err
		}
		c.Limit = *limit
	}
	if err := s.repo().UpdateCategory(ctx, userID, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().DeleteCategory(ctx, userID, id)
}

// ---- rules ----

func ruleKeyword(k string) (string, error) {
	k = strings.Join(strings.Fields(k), " ")
	if n := utf8.RuneCountInString(k); n < 2 || n > 60 || !utf8.ValidString(k) {
		return "", apperror.New(apperror.CodeInvalidInput, "keyword must have 2 to 60 characters").WithField("keyword")
	}
	return k, nil
}

func (s *Service) Rules(ctx context.Context, userID string) ([]domain.Rule, error) {
	return s.repo().Rules(ctx, userID)
}

func (s *Service) PreviewRule(ctx context.Context, userID, keyword string) (int64, error) {
	keyword, err := ruleKeyword(keyword)
	if err != nil {
		return 0, err
	}
	return s.repo().CountMatches(ctx, userID, keyword)
}

func (s *Service) CreateRule(ctx context.Context, userID, keyword, categoryID string) (*domain.Rule, int64, error) {
	keyword, err := ruleKeyword(keyword)
	if err != nil {
		return nil, 0, err
	}
	cat, err := s.ownCategory(ctx, userID, categoryID)
	if err != nil {
		return nil, 0, err
	}
	rule := &domain.Rule{Keyword: keyword, CategoryID: cat.ID, CategoryName: cat.Name}
	var changed int64
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if err := uow.Finance().CreateRule(ctx, userID, rule); err != nil {
			return err
		}
		changed, err = uow.Finance().ApplyRule(ctx, userID, keyword, cat.ID)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	rule.Hits = changed
	return rule, changed, nil
}

func (s *Service) DeleteRule(ctx context.Context, userID, id string) error {
	if err := validate.ID(id); err != nil {
		return err
	}
	return s.repo().DeleteRule(ctx, userID, id)
}
