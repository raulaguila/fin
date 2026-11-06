package mailer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/aescrypt"
	"github.com/raulaguila/authenticity/pkg/logger"
	"github.com/raulaguila/authenticity/pkg/validate"
)

const (
	defaultBatch   = 10
	defaultPoll    = 2 * time.Second
	maxAttempts    = 8
	maxBackoffSecs = 15 * 60
	maxErrorBytes  = 500
	// claimLease must outlast a full batch of sends (10 x the 30s SMTP deadline).
	claimLease  = 10 * time.Minute
	markTimeout = 5 * time.Second
)

// MailSender sends a rendered email.
type MailSender interface {
	Enabled() bool
	Send(to, subject, body string) error
}

// Worker claims outbox rows and sends email.
type Worker struct {
	Outbox output.OutboxRepository
	Mail   MailSender
	Log    *logger.Logger
	Clock  output.Clock
	Key    []byte // AES-256 key opening the sealed token in each payload
	Batch  int
	Poll   time.Duration
}

type emailPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Token  string `json:"token"`
}

func (w *Worker) Run(ctx context.Context) error {
	if w.Batch <= 0 {
		w.Batch = defaultBatch
	}
	if w.Poll <= 0 {
		w.Poll = defaultPoll
	}

	ticker := time.NewTicker(w.Poll)
	defer ticker.Stop()

	w.Log.Info("Mailer worker started", slog.Int("batch", w.Batch), slog.Duration("poll", w.Poll))
	for {
		if err := w.tick(ctx); err != nil {
			w.Log.Warn("Mailer tick error", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			w.Log.Info("Mailer worker stopped")
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	now := w.Clock.Now()
	msgs, err := w.Outbox.ClaimPending(ctx, w.Batch, now, claimLease)
	if err != nil {
		return err
	}
	for i := range msgs {
		if ctx.Err() != nil { // shutting down: unsent rows return when the lease ends
			return ctx.Err()
		}
		w.process(ctx, &msgs[i], now)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, msg *domain.OutboxMessage, now time.Time) {
	to, subject, body, err := render(msg, w.Key)
	if err != nil {
		w.fail(ctx, msg, now, err.Error(), true)
		return
	}
	if !w.Mail.Enabled() {
		w.fail(ctx, msg, now, "smtp_disabled", true)
		return
	}
	if err := w.Mail.Send(to, subject, body); err != nil {
		attempts := msg.Attempts + 1
		terminal := attempts >= maxAttempts || err.Error() == "smtp_disabled" || err.Error() == "smtp_misconfigured"
		w.fail(ctx, msg, now, validate.Text(err.Error(), maxErrorBytes), terminal)
		return
	}
	mark, cancel := markContext(ctx)
	defer cancel()
	if err := w.Outbox.MarkSent(mark, msg.ID, now); err != nil {
		w.Log.Warn("MarkSent failed", slog.String("outbox_id", msg.ID), slog.String("error", err.Error()))
		return
	}
	w.Log.Info("Email sent", slog.String("outbox_id", msg.ID), slog.String("purpose", msg.Purpose))
}

func (w *Worker) fail(ctx context.Context, msg *domain.OutboxMessage, now time.Time, lastErr string, terminal bool) {
	attempts := msg.Attempts + 1
	backoff := time.Duration(math.Min(math.Pow(2, float64(attempts)), float64(maxBackoffSecs))) * time.Second
	available := now.Add(backoff)
	var processed *time.Time
	if terminal {
		processed = &now
	}
	mark, cancel := markContext(ctx)
	defer cancel()
	if err := w.Outbox.MarkFailed(mark, msg.ID, attempts, available, processed, lastErr, terminal); err != nil {
		w.Log.Warn("MarkFailed failed", slog.String("outbox_id", msg.ID), slog.String("error", err.Error()))
		return
	}
	w.Log.Warn("Email send failed",
		slog.String("outbox_id", msg.ID),
		slog.String("purpose", msg.Purpose),
		slog.Int("attempts", attempts),
		slog.Bool("terminal", terminal),
		slog.String("error", lastErr),
	)
}

// markContext starts after the send: a result must be recorded even when the
// send was slow or shutdown cancelled ctx meanwhile.
func markContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), markTimeout)
}

func render(msg *domain.OutboxMessage, key []byte) (to, subject, body string, err error) {
	var p emailPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return "", "", "", fmt.Errorf("invalid_payload")
	}
	if p.Email == "" || p.Token == "" {
		return "", "", "", fmt.Errorf("invalid_payload")
	}
	token, err := aescrypt.Decrypt(key, p.Token)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid_payload")
	}
	switch msg.Purpose {
	case domain.OutboxPurposeActivation:
		subject = "Activate your account"
		body = fmt.Sprintf("Hello%s,\n\nUse this activation token:\n\n%s\n", nameSuffix(p.Name), token)
	case domain.OutboxPurposePasswordReset:
		subject = "Password reset"
		body = fmt.Sprintf("Hello%s,\n\nUse this password reset token:\n\n%s\n", nameSuffix(p.Name), token)
	default:
		return "", "", "", fmt.Errorf("unknown_purpose")
	}
	return p.Email, subject, body, nil
}

func nameSuffix(name string) string {
	if name == "" {
		return ""
	}
	return " " + name
}
