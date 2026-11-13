package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/raulaguila/fin/config"
)

// Ollama talks to Ollama's own /api/chat. Its OpenAI-compatible endpoint keeps
// the server's context size (4096 by default) and lets a thinking model spend
// it all on reasoning: the assistant's facts alone fill most of that.
type Ollama struct {
	cfg    config.AIConfig
	client *http.Client
}

func NewOllama(cfg config.AIConfig) Ollama {
	return Ollama{cfg: cfg, client: &http.Client{}} // the context carries the deadline
}

type ollamaChunk struct {
	Message    chatMessage `json:"message"`
	Done       bool        `json:"done"`
	DoneReason string      `json:"done_reason"`
	Error      string      `json:"error"`
}

func (o Ollama) Ask(ctx context.Context, system, prompt string, onDelta func(string)) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}
	messages = append(messages, chatMessage{Role: "user", Content: prompt})
	body, err := json.Marshal(map[string]any{
		"model": o.cfg.Model, "messages": messages, "stream": onDelta != nil,
		"think": false, "options": map[string]any{"num_ctx": o.cfg.Context},
	})
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(o.cfg.OllamaURL, "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ollama: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	// Streamed or not, the body is JSON objects one per line; unstreamed it is one.
	var answer strings.Builder
	var last ollamaChunk
	sc := bufio.NewScanner(io.LimitReader(resp.Body, maxResponse))
	sc.Buffer(make([]byte, 64<<10), maxResponse)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var c ollamaChunk
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			return "", fmt.Errorf("ollama: unreadable response: %w", err)
		}
		if c.Error != "" {
			return "", fmt.Errorf("ollama: %s", c.Error)
		}
		if c.Message.Content != "" {
			answer.WriteString(c.Message.Content)
			if onDelta != nil {
				onDelta(c.Message.Content)
			}
		}
		last = c
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	if answer.Len() == 0 {
		if last.DoneReason == "length" {
			return "", errors.New("ollama: the answer did not fit in AI_CONTEXT")
		}
		return "", errors.New("ollama: empty answer")
	}
	return answer.String(), nil
}
