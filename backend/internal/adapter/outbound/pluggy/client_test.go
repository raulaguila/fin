package pluggy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/raulaguila/fin/config"
)

// fake serves the Pluggy routes the client uses, with the shapes of the docs.
func fake(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var auths atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["clientId"] != "id" || body["clientSecret"] != "secret" {
			http.Error(w, `{"codeDescription":"CLIENT_KEYS_UNAUTHORIZED"}`, http.StatusUnauthorized)
			return
		}
		auths.Add(1)
		_, _ = w.Write([]byte(`{"apiKey":"key-` + string(rune('0'+auths.Load())) + `"}`))
	})
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.Header.Get("X-API-KEY"), "key-") {
				http.Error(w, "{}", http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("POST /connect_token", authed(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ItemID  string         `json:"itemId"`
			Options map[string]any `json:"options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Options["clientUserId"] != "user-1" || body.Options["webhookUrl"] != "https://fin.example.io/v1/finance/webhook?token=s3cr3t-s3cr3t-s3" {
			http.Error(w, "bad options", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"accessToken":"connect-` + body.ItemID + `"}`))
	}))
	mux.HandleFunc("GET /items/{id}", authed(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"` + r.PathValue("id") + `","status":"LOGIN_ERROR","executionStatus":"ERROR",
			"clientUserId":"user-1","consentExpiresAt":"2027-01-02T03:04:05.000Z",
			"error":{"code":"INVALID_CREDENTIALS","message":"wrong password"},
			"connector":{"id":2,"name":"Pluggy Bank","imageUrl":"https://cdn/x.svg"}}`))
	}))
	mux.HandleFunc("GET /accounts", authed(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"total":2,"totalPages":1,"results":[
			{"id":"a1","type":"BANK","subtype":"CHECKING_ACCOUNT","name":"Conta","number":"0001/1","balance":1234.56},
			{"id":"a2","type":"CREDIT","subtype":"CREDIT_CARD","name":"Card","marketingName":"Platinum","balance":99.9,
			 "creditData":{"creditLimit":5000,"availableCreditLimit":4000.1,"balanceCloseDate":"2026-11-28","balanceDueDate":"2026-12-05"}}]}`))
	}))
	mux.HandleFunc("GET /v2/transactions", authed(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("after") == "" {
			if r.URL.Query().Get("dateFrom") != "2026-01-01" {
				http.Error(w, "bad dateFrom", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"t1","description":" UBER *TRIP ","amount":-23.5,"date":"2026-11-10T00:00:00.000Z","status":"POSTED","type":"DEBIT","category":"Taxi and ride-hailing"}],
				"next":"?accountId=a1&after=abc"}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"t2","description":"LOJA 2/10","amount":159.2,"date":"2026-11-11T00:00:00.000Z","status":"PENDING","type":"CREDIT",
			"creditCardMetadata":{"installmentNumber":2,"totalInstallments":10}}],"next":null}`))
	}))
	mux.HandleFunc("GET /bills", authed(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":"b1","dueDate":"2026-10-05T00:00:00.000Z","totalAmount":800.5},
			{"id":"b2","dueDate":"2026-11-05T00:00:00.000Z","totalAmount":1173.78}]}`))
	}))
	mux.HandleFunc("GET /investments", authed(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("itemId") != "item-9" {
			http.Error(w, "bad itemId", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"page":1,"totalPages":1,"results":[{"id":"i1","name":" CDB Banco X ","type":"FIXED_INCOME","balance":10500.25}]}`))
	}))
	mux.HandleFunc("PATCH /items/{id}", authed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"MeuPluggy item cant be updated","code":400}`, http.StatusBadRequest)
	}))
	mux.HandleFunc("DELETE /items/{id}", authed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "{}", http.StatusNotFound)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &auths
}

func client(srv *httptest.Server) *Client {
	return New(config.PluggyConfig{BaseURL: srv.URL, ClientID: "id", ClientSecret: "secret",
		WebhookURL: "https://fin.example.io/v1/finance/webhook", WebhookSecret: "s3cr3t-s3cr3t-s3"})
}

func TestClientFlow(t *testing.T) {
	srv, auths := fake(t)
	c := client(srv)
	ctx := context.Background()

	token, err := c.ConnectToken(ctx, "user-1", "item-9")
	if err != nil || token != "connect-item-9" {
		t.Fatalf("connect token: %q, %v", token, err)
	}

	item, err := c.Item(ctx, "item-9")
	if err != nil {
		t.Fatal(err)
	}
	if item.ClientUserID != "user-1" || item.Institution != "Pluggy Bank" || item.Status != "LOGIN_ERROR" ||
		item.Error != "INVALID_CREDENTIALS: wrong password" || item.ConsentExpiresAt == nil || item.ConsentExpiresAt.Year() != 2027 {
		t.Fatalf("item: %+v", item)
	}

	accounts, err := c.Accounts(ctx, "item-9")
	if err != nil || len(accounts) != 2 {
		t.Fatalf("accounts: %+v, %v", accounts, err)
	}
	if accounts[0].Balance != 123456 || accounts[1].Name != "Platinum" || *accounts[1].CreditLimit != 500000 ||
		*accounts[1].Available != 400010 || accounts[1].BillDueDate.Format(time.DateOnly) != "2026-12-05" ||
		accounts[0].Subtype != "CHECKING_ACCOUNT" {
		t.Fatalf("account fields: %+v %+v", accounts[0], accounts[1])
	}

	due, amount, err := c.LastBill(ctx, "a2")
	if err != nil || due == nil || due.Format(time.DateOnly) != "2026-11-05" || *amount != 117378 {
		t.Fatalf("last bill: %v %v %v", due, amount, err)
	}

	txs, err := c.Transactions(ctx, "a1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(txs) != 2 {
		t.Fatalf("transactions: %+v, %v", txs, err)
	}
	if txs[0].Amount != -2350 || txs[0].Description != "UBER *TRIP" || txs[0].Category != "Taxi and ride-hailing" ||
		txs[1].Amount != 15920 || txs[1].Installment != 2 || txs[1].Installments != 10 {
		t.Fatalf("transaction fields: %+v", txs)
	}

	inv, err := c.Investments(ctx, "item-9")
	if err != nil || len(inv) != 1 || inv[0].Name != "CDB Banco X" || inv[0].Balance != 1050025 || inv[0].Type != "FIXED_INCOME" {
		t.Fatalf("investments: %+v, %v", inv, err)
	}

	if err := c.RefreshItem(ctx, "meupluggy"); err != nil {
		t.Fatalf("a MeuPluggy item refreshes itself; asking must not fail the sync: %v", err)
	}
	if err := c.DeleteItem(ctx, "gone"); err != nil {
		t.Fatalf("delete of a missing item must succeed: %v", err)
	}
	if auths.Load() != 1 {
		t.Fatalf("the API key must be reused, got %d auth calls", auths.Load())
	}
}

func TestClientRejectsForeignCursor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"apiKey":"key-1"}`)) })
	mux.HandleFunc("GET /v2/transactions", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[],"next":"/items/x"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if _, err := client(srv).Transactions(context.Background(), "a1", time.Now()); err == nil {
		t.Fatal("a next cursor that is not a query must be refused")
	}
}

func TestClientDisabled(t *testing.T) {
	c := New(config.PluggyConfig{})
	if c.Enabled() {
		t.Fatal("no client id must disable the client")
	}
	if _, err := c.Item(context.Background(), "x"); err != ErrDisabled {
		t.Fatalf("want ErrDisabled, got %v", err)
	}
}
