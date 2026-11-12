// Package pluggy implements output.OpenFinance on the Pluggy API.
package pluggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/raulaguila/fin/config"
	"github.com/raulaguila/fin/internal/core/domain"
	"github.com/raulaguila/fin/internal/core/port/output"
)

const (
	// The API key lasts 2 hours; renew it well before.
	keyLifetime  = 100 * time.Minute
	maxBody      = 16 << 20
	maxPages     = 200 // 500 transactions each: a loop guard, far above one account
	requestLimit = 60 * time.Second
)

// ErrDisabled means PLUGGY_CLIENT_ID is empty.
var ErrDisabled = errors.New("open finance disabled: PLUGGY_CLIENT_ID is empty")

type Client struct {
	base       string
	id, secret string
	webhookURL string
	http       *http.Client

	mu     sync.Mutex
	key    string
	keyExp time.Time
}

var _ output.OpenFinance = (*Client)(nil)

func New(cfg config.PluggyConfig) *Client {
	c := &Client{
		base:   strings.TrimRight(cfg.BaseURL, "/"),
		id:     cfg.ClientID,
		secret: cfg.ClientSecret,
		http:   &http.Client{Timeout: requestLimit},
	}
	if cfg.WebhookURL != "" && cfg.WebhookSecret != "" {
		c.webhookURL = cfg.WebhookURL + "?token=" + url.QueryEscape(cfg.WebhookSecret)
	}
	return c
}

func (c *Client) Enabled() bool { return c.id != "" }

func (c *Client) apiKey(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key != "" && time.Now().Before(c.keyExp) {
		return c.key, nil
	}
	var out struct {
		APIKey string `json:"apiKey"`
	}
	body := map[string]string{"clientId": c.id, "clientSecret": c.secret}
	if err := c.send(ctx, http.MethodPost, "/auth", "", body, &out); err != nil {
		return "", err
	}
	if out.APIKey == "" {
		return "", errors.New("pluggy: auth returned no apiKey")
	}
	c.key, c.keyExp = out.APIKey, time.Now().Add(keyLifetime)
	return c.key, nil
}

// do calls the API with the key, renewing it once on 401.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if !c.Enabled() {
		return ErrDisabled
	}
	for attempt := 0; ; attempt++ {
		key, err := c.apiKey(ctx)
		if err != nil {
			return err
		}
		err = c.send(ctx, method, path, key, body, out)
		var status statusError
		if attempt == 0 && errors.As(err, &status) && status.code == http.StatusUnauthorized {
			c.mu.Lock()
			c.key = ""
			c.mu.Unlock()
			continue
		}
		return err
	}
}

type statusError struct {
	code int
	body string
}

func (e statusError) Error() string { return fmt.Sprintf("pluggy: status %d: %s", e.code, e.body) }

func (c *Client) send(ctx context.Context, method, path, key string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("X-API-KEY", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("pluggy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("pluggy: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return statusError{code: resp.StatusCode, body: strings.TrimSpace(string(data[:min(len(data), 300)]))}
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("pluggy: unreadable response of %s: %w", path, err)
	}
	return nil
}

func (c *Client) ConnectToken(ctx context.Context, clientUserID, itemID string) (string, error) {
	// No avoidDuplicates: MeuPluggy shares one bank per item under the same sign-in, so
	// Pluggy took a second bank for a duplicate. AddConnection already dedupes by item id.
	options := map[string]any{"clientUserId": clientUserID}
	if c.webhookURL != "" {
		options["webhookUrl"] = c.webhookURL
	}
	body := map[string]any{"options": options}
	if itemID != "" {
		body["itemId"] = itemID
	}
	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if err := c.do(ctx, http.MethodPost, "/connect_token", body, &out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

type itemJSON struct {
	ID               string     `json:"id"`
	Status           string     `json:"status"`
	ExecutionStatus  string     `json:"executionStatus"`
	ClientUserID     *string    `json:"clientUserId"`
	ConsentExpiresAt *time.Time `json:"consentExpiresAt"`
	Error            *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Connector struct {
		Name     string `json:"name"`
		ImageURL string `json:"imageUrl"`
	} `json:"connector"`
}

func (c *Client) Item(ctx context.Context, id string) (*domain.OFItem, error) {
	var it itemJSON
	if err := c.do(ctx, http.MethodGet, "/items/"+url.PathEscape(id), nil, &it); err != nil {
		return nil, err
	}
	out := &domain.OFItem{
		ID: it.ID, Status: it.Status, ExecutionStatus: it.ExecutionStatus,
		Institution: it.Connector.Name, ImageURL: it.Connector.ImageURL, ConsentExpiresAt: it.ConsentExpiresAt,
	}
	if it.ClientUserID != nil {
		out.ClientUserID = *it.ClientUserID
	}
	if it.Error != nil {
		out.Error = strings.TrimSpace(it.Error.Code + ": " + it.Error.Message)
	}
	return out, nil
}

func (c *Client) RefreshItem(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPatch, "/items/"+url.PathEscape(id), map[string]any{}, nil)
}

func (c *Client) DeleteItem(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/items/"+url.PathEscape(id), nil, nil)
	var status statusError
	if errors.As(err, &status) && status.code == http.StatusNotFound {
		return nil // already gone at the provider
	}
	return err
}

type accountJSON struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	Subtype       string      `json:"subtype"`
	Name          string      `json:"name"`
	MarketingName string      `json:"marketingName"`
	Number        string      `json:"number"`
	Balance       json.Number `json:"balance"`
	CreditData    *struct {
		CreditLimit          json.Number `json:"creditLimit"`
		AvailableCreditLimit json.Number `json:"availableCreditLimit"`
		BalanceCloseDate     string      `json:"balanceCloseDate"`
		BalanceDueDate       string      `json:"balanceDueDate"`
	} `json:"creditData"`
}

func (c *Client) Accounts(ctx context.Context, itemID string) ([]domain.OFAccount, error) {
	var out struct {
		Results []accountJSON `json:"results"`
	}
	if err := c.do(ctx, http.MethodGet, "/accounts?itemId="+url.QueryEscape(itemID), nil, &out); err != nil {
		return nil, err
	}
	accounts := make([]domain.OFAccount, 0, len(out.Results))
	for _, a := range out.Results {
		acc := domain.OFAccount{ID: a.ID, Type: a.Type, Subtype: a.Subtype, Name: a.Name, Number: a.Number, Balance: cents(a.Balance)}
		if a.MarketingName != "" {
			acc.Name = a.MarketingName
		}
		if cd := a.CreditData; cd != nil {
			acc.CreditLimit = optionalCents(cd.CreditLimit)
			acc.Available = optionalCents(cd.AvailableCreditLimit)
			acc.BillCloseDate = date(cd.BalanceCloseDate)
			acc.BillDueDate = date(cd.BalanceDueDate)
		}
		accounts = append(accounts, acc)
	}
	return accounts, nil
}

func (c *Client) LastBill(ctx context.Context, accountID string) (*time.Time, *domain.Money, error) {
	var out struct {
		Results []struct {
			DueDate     string      `json:"dueDate"`
			TotalAmount json.Number `json:"totalAmount"`
		} `json:"results"`
	}
	if err := c.do(ctx, http.MethodGet, "/bills?accountId="+url.QueryEscape(accountID), nil, &out); err != nil {
		return nil, nil, err
	}
	var due *time.Time
	var amount *domain.Money
	for _, b := range out.Results {
		d := date(b.DueDate)
		if d != nil && (due == nil || d.After(*due)) {
			due, amount = d, optionalCents(b.TotalAmount)
		}
	}
	return due, amount, nil
}

type transactionJSON struct {
	ID                 string      `json:"id"`
	Description        string      `json:"description"`
	Amount             json.Number `json:"amount"`
	Date               time.Time   `json:"date"`
	Status             string      `json:"status"`
	Category           *string     `json:"category"`
	CreditCardMetadata *struct {
		InstallmentNumber int `json:"installmentNumber"`
		TotalInstallments int `json:"totalInstallments"`
	} `json:"creditCardMetadata"`
}

func (c *Client) Transactions(ctx context.Context, accountID string, from time.Time) ([]domain.OFTransaction, error) {
	path := "/v2/transactions?" + url.Values{"accountId": {accountID}, "dateFrom": {from.Format(time.DateOnly)}}.Encode()
	var all []domain.OFTransaction
	for range maxPages {
		var page struct {
			Results []transactionJSON `json:"results"`
			Next    *string           `json:"next"`
		}
		if err := c.do(ctx, http.MethodGet, path, nil, &page); err != nil {
			return nil, err
		}
		for _, t := range page.Results {
			tx := domain.OFTransaction{
				ID: t.ID, Date: t.Date, Description: strings.TrimSpace(t.Description),
				Amount: cents(t.Amount), Status: t.Status,
			}
			if t.Category != nil {
				tx.Category = *t.Category
			}
			if m := t.CreditCardMetadata; m != nil {
				tx.Installment, tx.Installments = m.InstallmentNumber, m.TotalInstallments
			}
			all = append(all, tx)
		}
		if page.Next == nil || *page.Next == "" {
			return all, nil
		}
		// next is the query string of the following page; never let it name another path.
		if !strings.HasPrefix(*page.Next, "?") {
			return nil, fmt.Errorf("pluggy: unexpected next cursor %q", *page.Next)
		}
		path = "/v2/transactions" + *page.Next
	}
	return nil, errors.New("pluggy: too many transaction pages")
}

// Investments reads the item's positions; balance is the current gross value.
func (c *Client) Investments(ctx context.Context, itemID string) ([]domain.OFInvestment, error) {
	var all []domain.OFInvestment
	for page := 1; page <= maxPages; page++ {
		var out struct {
			Results []struct {
				ID      string      `json:"id"`
				Name    string      `json:"name"`
				Type    string      `json:"type"`
				Balance json.Number `json:"balance"`
			} `json:"results"`
			TotalPages int `json:"totalPages"`
		}
		q := url.Values{"itemId": {itemID}, "pageSize": {"500"}, "page": {strconv.Itoa(page)}}
		if err := c.do(ctx, http.MethodGet, "/investments?"+q.Encode(), nil, &out); err != nil {
			return nil, err
		}
		for _, i := range out.Results {
			all = append(all, domain.OFInvestment{ID: i.ID, Name: strings.TrimSpace(i.Name), Type: i.Type, Balance: cents(i.Balance)})
		}
		if page >= out.TotalPages {
			return all, nil
		}
	}
	return nil, errors.New("pluggy: too many investment pages")
}

// cents turns a decimal amount into integer cents; an absent value is 0.
func cents(n json.Number) domain.Money {
	f, err := strconv.ParseFloat(n.String(), 64)
	if err != nil {
		return 0
	}
	return domain.Money(math.Round(f * 100))
}

func optionalCents(n json.Number) *domain.Money {
	if n == "" {
		return nil
	}
	v := cents(n)
	return &v
}

func date(s string) *time.Time {
	if len(s) < len(time.DateOnly) {
		return nil
	}
	t, err := time.Parse(time.DateOnly, s[:len(time.DateOnly)])
	if err != nil {
		return nil
	}
	return &t
}
