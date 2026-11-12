package ai

import (
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

const maxResponse = 4 << 20

// OpenAI talks to any /chat/completions endpoint: Ollama, LM Studio, vLLM.
type OpenAI struct {
	cfg    config.AIConfig
	client *http.Client
}

func NewOpenAI(cfg config.AIConfig) OpenAI {
	return OpenAI{cfg: cfg, client: &http.Client{}} // the context carries the deadline
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (o OpenAI) Ask(ctx context.Context, system, prompt string) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}
	messages = append(messages, chatMessage{Role: "user", Content: prompt})
	body, err := json.Marshal(map[string]any{"model": o.cfg.Model, "messages": messages})
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(o.cfg.OpenAIURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.cfg.OpenAIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.cfg.OpenAIKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data[:min(len(data), 512)])))
	}

	var r struct {
		Choices []struct {
			Message chatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("openai: unreadable response: %w", err)
	}
	if len(r.Choices) == 0 {
		return "", errors.New("openai: no choices in response")
	}
	return r.Choices[0].Message.Content, nil
}
