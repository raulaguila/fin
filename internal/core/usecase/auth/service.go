package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/aescrypt"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

// Config holds auth policy from environment (no config package import).
type Config struct {
	RegistrationTokenTTL         time.Duration
	PasswordResetTokenTTL        time.Duration
	MaxFailedLoginAttempts       int
	AccountLockDuration          time.Duration
	AllowManualTokenDelivery     bool
	LoginAttemptEmailWindow      time.Duration
	LoginAttemptEmailMaxFailures int
	LoginAttemptIPWindow         time.Duration
	LoginAttemptIPMaxFailures    int
	EmailTokenCooldown           time.Duration // minimum gap between emails of one purpose to one user
	OutboxKey                    []byte        // AES-256 key sealing email tokens at rest in sys.outbox
}

// Service implements identity auth flows.
type Service struct {
	Tx     output.TxManager
	UOW    output.UnitOfWork
	Tokens output.TokenIssuer
	Clock  output.Clock
	Cfg    Config
	Authz  input.Authorizer // optional; invalidates effective-permission cache
}

var _ input.AuthService = (*Service)(nil)

func New(
	tx output.TxManager,
	uow output.UnitOfWork,
	tokens output.TokenIssuer,
	clock output.Clock,
	cfg Config,
	authz input.Authorizer,
) *Service {
	if clock == nil {
		clock = output.RealClock{}
	}
	return &Service{Tx: tx, UOW: uow, Tokens: tokens, Clock: clock, Cfg: cfg, Authz: authz}
}

// coolingDown limits emails per account: the per-IP limit does not stop many
// addresses flooding one mailbox and killing its link with each new token.
func (s *Service) coolingDown(ctx context.Context, uow output.UnitOfWork, userID, purpose string) (bool, error) {
	return uow.Tokens().IssuedSince(ctx, userID, purpose, s.Clock.Now().Add(-s.Cfg.EmailTokenCooldown))
}

// issueEmailToken replaces the user's live token and queues the email. The
// outbox only ever holds the token sealed; the mailer opens it to send.
func (s *Service) issueEmailToken(ctx context.Context, uow output.UnitOfWork, u *domain.User, purpose, plain string, ttl time.Duration) error {
	if err := uow.Tokens().InvalidateActive(ctx, u.ID, purpose); err != nil {
		return err
	}
	tok := &domain.EmailToken{
		UserID:    u.ID,
		Purpose:   purpose,
		TokenHash: tokenhash.SHA256Bytes(plain),
		ExpiresAt: s.Clock.Now().Add(ttl),
	}
	if err := uow.Tokens().Create(ctx, tok); err != nil {
		return err
	}
	sealed, err := aescrypt.Encrypt(s.Cfg.OutboxKey, plain)
	if err != nil {
		return apperror.New(apperror.CodeInternal, "token sealing failed").WithCause(err)
	}
	payload, err := json.Marshal(map[string]any{
		"user_id": u.ID,
		"email":   u.Email,
		"name":    u.Name,
		"token":   sealed,
	})
	if err != nil {
		return apperror.New(apperror.CodeInternal, "payload encoding failed").WithCause(err)
	}
	return uow.Outbox().Enqueue(ctx, purpose, payload)
}
