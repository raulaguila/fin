package output

import "context"

// Assistant sends one prompt to the configured model and returns its answer.
type Assistant interface {
	Ask(ctx context.Context, system, prompt string) (string, error)
}
