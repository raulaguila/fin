package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

// Config holds auth policy from environment (no config package import).
type Config struct {
	SessionMaxLifetime           time.Duration // a session ends this long after login, however often it refreshes
	RegistrationTokenTTL         time.Duration
	PasswordResetTokenTTL        time.Duration
	MaxFailedLoginAttempts       int
	AccountLockDuration          time.Duration
	AllowManualTokenDelivery     bool
	LoginAttemptEmailWindow      time.Duration
	LoginAttemptEmailMaxFailures int // all clients together; high, or anyone could lock an account out
	LoginAttemptPairMaxFailures  int // one client against one account
	LoginAttemptIPWindow         time.Duration
	LoginAttemptIPMaxFailures    int
	EmailTokenCooldown           time.Duration // minimum gap between emails of one purpose to one user
	EmailTokenHourlyMax          int           // 0 means no cap
}

// Service implements identity auth flows.
type Service struct {
	Tx     output.TxManager
	UOW    output.UnitOfWork
	Tokens output.TokenIssuer
	Clock  output.Clock
	Cfg    Config
	Codec  output.EmailTokenCodec
	Log    output.Logger // optional: a nil logger drops the message
}

var _ input.AuthService = (*Service)(nil)

func New(
	tx output.TxManager,
	uow output.UnitOfWork,
	tokens output.TokenIssuer,
	clock output.Clock,
	cfg Config,
	codec output.EmailTokenCodec,
) *Service {
	return &Service{Tx: tx, UOW: uow, Tokens: tokens, Clock: clock, Cfg: cfg, Codec: codec}
}

// warn reports a side step that failed without failing the request.
func (s *Service) warn(msg string, err error) {
	if err != nil {
		output.Log(s.Log).Warn(msg, slog.String("error", err.Error()))
	}
}

// emailThrottled limits emails per account: the per-IP limit does not stop many
// addresses flooding one mailbox. The hourly cap also protects the link already
// sent, since each new token kills the one before it.
func (s *Service) emailThrottled(ctx context.Context, uow output.UnitOfWork, userID, purpose string) (bool, error) {
	if err := uow.Lock(ctx, "email:"+userID+":"+purpose); err != nil {
		return false, err
	}
	now := s.Clock.Now()
	recent, err := uow.Tokens().IssuedSince(ctx, userID, purpose, now.Add(-s.Cfg.EmailTokenCooldown))
	if recent || err != nil || s.Cfg.EmailTokenHourlyMax <= 0 {
		return recent, err
	}
	n, err := uow.Tokens().CountIssuedSince(ctx, userID, purpose, now.Add(-time.Hour))
	return n >= s.Cfg.EmailTokenHourlyMax, err
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
		CreatedAt: s.Clock.Now(),
	}
	if err := uow.Tokens().Create(ctx, tok); err != nil {
		return err
	}
	payload, err := s.Codec.Encode(output.EmailToken{Email: u.Email, Token: plain})
	if err != nil {
		return apperror.New(apperror.CodeInternal, "email token encoding failed").WithCause(err)
	}

	return uow.Outbox().Enqueue(ctx, purpose, payload)
}
