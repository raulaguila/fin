package output

import "context"

// Assistant sends one prompt to the configured model and returns its answer.
// A non-nil onDelta gets the answer in pieces as the model writes it.
type Assistant interface {
	Ask(ctx context.Context, system, prompt string, onDelta func(string)) (string, error)
}
