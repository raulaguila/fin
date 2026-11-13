package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	got, err := assistant.Ask(context.Background(), "sys", "pergunta", nil)
	if err != nil || got != "resposta" {
		t.Fatalf("got %q, %v", got, err)
	}

	cfg.OpenAIKey = "wrong"
	if _, err := NewOpenAI(cfg).Ask(context.Background(), "sys", "x", nil); err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("want status error, got %v", err)
	}
}

func TestNewDisabled(t *testing.T) {
	if _, err := New(config.AIConfig{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("want ErrDisabled, got %v", err)
	}
}

func TestOpenAIStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !body.Stream {
			http.Error(w, "want stream", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"res\"}}]}\n\n" +
			": keep-alive\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"posta\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()
	var parts []string
	got, err := NewOpenAI(config.AIConfig{Model: "m", OpenAIURL: srv.URL}).Ask(context.Background(), "", "p", func(d string) { parts = append(parts, d) })
	if err != nil || got != "resposta" || strings.Join(parts, "|") != "res|posta" {
		t.Fatalf("got %q %v %v", got, parts, err)
	}
}

func TestClaudeStream(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "claude")
	script := `#!/bin/sh
cat >/dev/null
echo '{"type":"system","subtype":"init"}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"o"}}}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"i"}}}'
echo '{"type":"result","subtype":"success","is_error":false,"result":"oi"}'
`
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	var parts []string
	got, err := Claude{cfg: config.AIConfig{ClaudeBin: bin}}.Ask(context.Background(), "s", "p", func(d string) { parts = append(parts, d) })
	if err != nil || got != "oi" || strings.Join(parts, "") != "oi" {
		t.Fatalf("got %q %v %v", got, parts, err)
	}
}

func TestOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream  bool `json:"stream"`
			Think   bool `json:"think"`
			Options struct {
				NumCtx int `json:"num_ctx"`
			} `json:"options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/api/chat" || body.Think || body.Options.NumCtx != 8192 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body.Stream {
			_, _ = w.Write([]byte(`{"message":{"content":"res"},"done":false}` + "\n" + `{"message":{"content":"posta"},"done":true}` + "\n"))
			return
		}
		_, _ = w.Write([]byte(`{"message":{"content":""},"done":true,"done_reason":"length"}`))
	}))
	defer srv.Close()
	o := NewOllama(config.AIConfig{Model: "m", OllamaURL: srv.URL + "/", Context: 8192})
	var parts []string
	got, err := o.Ask(context.Background(), "s", "p", func(d string) { parts = append(parts, d) })
	if err != nil || got != "resposta" || strings.Join(parts, "|") != "res|posta" {
		t.Fatalf("got %q %v %v", got, parts, err)
	}
	// A thinking model that used the whole context answers nothing: say so.
	if _, err := o.Ask(context.Background(), "s", "p", nil); err == nil || !strings.Contains(err.Error(), "AI_CONTEXT") {
		t.Fatalf("want context error, got %v", err)
	}
}
