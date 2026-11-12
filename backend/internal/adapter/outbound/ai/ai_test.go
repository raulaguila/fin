package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/raulaguila/fin/config"
)

func TestParseClaude(t *testing.T) {
	if got, err := parseClaude([]byte(`{"subtype":"success","is_error":false,"result":"oi"}`)); err != nil || got != "oi" {
		t.Fatalf("got %q, %v", got, err)
	}
	// Real shape of an auth failure: subtype success, is_error true.
	_, err := parseClaude([]byte(`{"subtype":"success","is_error":true,"result":"Failed to authenticate."}`))
	if err == nil || !strings.Contains(err.Error(), "Failed to authenticate") {
		t.Fatalf("want auth error, got %v", err)
	}
	if _, err := parseClaude([]byte(`not json`)); err == nil {
		t.Fatal("want error on garbage")
	}
}

func TestClaudeEnvDropsSecrets(t *testing.T) {
	t.Setenv("POSTGRES_PASS", "secret")
	t.Setenv("HOME", "/home/x")
	env := strings.Join(claudeEnv("tok"), "\n")
	if strings.Contains(env, "POSTGRES_PASS") {
		t.Fatal("backend secret leaked to claude")
	}
	if !strings.Contains(env, "HOME=/home/x") || !strings.Contains(env, "CLAUDE_CODE_OAUTH_TOKEN=tok") {
		t.Fatalf("missing env: %s", env)
	}
}

func TestOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var body struct {
			Model    string        `json:"model"`
			Messages []chatMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "m" || len(body.Messages) != 2 || body.Messages[0].Role != "system" {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"resposta"}}]}`))
	}))
	defer srv.Close()

	cfg := config.AIConfig{Provider: "openai", Model: "m", Timeout: time.Second, OpenAIURL: srv.URL + "/v1/", OpenAIKey: "k"}
	assistant, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err := assistant.Ask(context.Background(), "sys", "pergunta")
	if err != nil || got != "resposta" {
		t.Fatalf("got %q, %v", got, err)
	}

	cfg.OpenAIKey = "wrong"
	if _, err := NewOpenAI(cfg).Ask(context.Background(), "sys", "x"); err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("want status error, got %v", err)
	}
}

func TestNewDisabled(t *testing.T) {
	if _, err := New(config.AIConfig{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("want ErrDisabled, got %v", err)
	}
}
