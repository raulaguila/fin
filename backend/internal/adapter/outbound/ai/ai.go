// Package ai implements output.Assistant: Claude Code headless on a Claude
// subscription, or any OpenAI-compatible API such as a local Ollama.
package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/raulaguila/fin/config"
	"github.com/raulaguila/fin/internal/core/port/output"
)

// ErrDisabled means AI_PROVIDER is empty.
var ErrDisabled = errors.New("assistant disabled: AI_PROVIDER is empty")

// New returns the assistant AI_PROVIDER names, each call bounded by AI_TIMEOUT.
func New(cfg config.AIConfig) (output.Assistant, error) {
	switch cfg.Provider {
	case "claude":
		return timed{Claude{cfg: cfg}, cfg}, nil
	case "openai":
		return timed{NewOpenAI(cfg), cfg}, nil
	case "":
		return nil, ErrDisabled
	default:
		return nil, fmt.Errorf("unknown AI_PROVIDER %q", cfg.Provider)
	}
}

type timed struct {
	next output.Assistant
	cfg  config.AIConfig
}

func (t timed) Ask(ctx context.Context, system, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.cfg.Timeout)
	defer cancel()
	return t.next.Ask(ctx, system, prompt)
}
