package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/validate"
)

const (
	defaultBatch   = 10
	defaultPoll    = 2 * time.Second
	maxAttempts    = 14 // about 75 minutes of backoff, to outlast an SMTP outage
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
	Heartbeat func() // optional liveness signal after a successful queue poll or send
	Outbox    output.OutboxRepository
	Mail      MailSender
	Log       output.Logger
	Clock     output.Clock
	Codec     output.EmailTokenCodec
	Batch     int
	Poll      time.Duration
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

	output.Log(w.Log).Info("Mailer worker started", slog.Int("batch", w.Batch), slog.Duration("poll", w.Poll))
	for {
		if err := w.tick(ctx); err != nil {
			output.Log(w.Log).Warn("Mailer tick error", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			output.Log(w.Log).Info("Mailer worker stopped")
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
	if w.Heartbeat != nil {
		w.Heartbeat()
	}
	for i := range msgs {
		if ctx.Err() != nil { // shutting down: unsent rows return when the lease ends
			return ctx.Err()
		}
		w.process(ctx, &msgs[i])
		if w.Heartbeat != nil {
			w.Heartbeat()
		}
	}
	return nil
}

func (w *Worker) process(ctx context.Context, msg *domain.OutboxMessage) {
	to, subject, body, err := render(msg, w.Codec)
	if err != nil {
		w.fail(ctx, msg, err.Error(), true)
		return
	}
	if !w.Mail.Enabled() {
		w.fail(ctx, msg, "smtp_disabled", true)
		return
	}
	if err := w.Mail.Send(to, subject, body); err != nil {
		attempts := msg.Attempts + 1
		terminal := attempts >= maxAttempts || err.Error() == "smtp_misconfigured"
		w.fail(ctx, msg, validate.Text(err.Error(), maxErrorBytes), terminal)
		return
	}
	now := w.Clock.Now()
	mark, cancel := markContext(ctx)
	defer cancel()
	if err := w.Outbox.MarkSent(mark, msg.ID, msg.ClaimID, now); err != nil {
		output.Log(w.Log).Warn("MarkSent failed", slog.String("outbox_id", msg.ID), slog.String("error", err.Error()))
		return
	}
	output.Log(w.Log).Info("Email sent", slog.String("outbox_id", msg.ID), slog.String("purpose", msg.Purpose))
}

func (w *Worker) fail(ctx context.Context, msg *domain.OutboxMessage, lastErr string, terminal bool) {
	now := w.Clock.Now()
	attempts := msg.Attempts + 1
	backoff := time.Duration(math.Min(math.Pow(2, float64(attempts)), float64(maxBackoffSecs))) * time.Second
	available := now.Add(backoff)
	var processed *time.Time
	if terminal {
		processed = &now
	}
	mark, cancel := markContext(ctx)
	defer cancel()
	if err := w.Outbox.MarkFailed(mark, msg.ID, msg.ClaimID, attempts, available, processed, lastErr, terminal); err != nil {
		output.Log(w.Log).Warn("MarkFailed failed", slog.String("outbox_id", msg.ID), slog.String("error", err.Error()))
		return
	}
	output.Log(w.Log).Warn("Email send failed",
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

func render(msg *domain.OutboxMessage, codec output.EmailTokenCodec) (to, subject, body string, err error) {
	p, err := codec.Decode(msg.Payload)
	if err != nil {
		return "", "", "", err
	}
	switch msg.Purpose {
	case domain.OutboxPurposeActivation:
		subject = "Activate your account"
		body = "Hello,\n\nUse this activation token:\n\n" + p.Token + "\n"
	case domain.OutboxPurposePasswordReset:
		subject = "Password reset"
		body = "Hello,\n\nUse this password reset token:\n\n" + p.Token + "\n"
	default:
		return "", "", "", fmt.Errorf("unknown_purpose")
	}
	return p.Email, subject, body, nil
}
