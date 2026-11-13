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

func (o OpenAI) Ask(ctx context.Context, system, prompt string, onDelta func(string)) (string, error) {
	messages := make([]chatMessage, 0, 2)
	if system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}
	messages = append(messages, chatMessage{Role: "user", Content: prompt})
	body, err := json.Marshal(map[string]any{"model": o.cfg.Model, "messages": messages, "stream": onDelta != nil})
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
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("openai: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if onDelta != nil {
		return readOpenAIStream(io.LimitReader(resp.Body, maxResponse), onDelta)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
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

// readOpenAIStream reads the server-sent events of a streamed completion:
// "data: {chunk}" lines, then "data: [DONE]".
func readOpenAIStream(r io.Reader, onDelta func(string)) (string, error) {
	var answer strings.Builder
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), maxResponse)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		data = strings.TrimSpace(data)
		if !ok || data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", fmt.Errorf("openai: unreadable chunk: %w", err)
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				answer.WriteString(c.Delta.Content)
				onDelta(c.Delta.Content)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	if answer.Len() == 0 {
		return "", errors.New("openai: empty answer")
	}
	return answer.String(), nil
}
