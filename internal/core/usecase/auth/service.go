package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/aescrypt"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/password"
	"github.com/raulaguila/authenticity/pkg/pointer"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

const (
	minPasswordBytes = 8
	maxPasswordBytes = 128

	// refreshRaceWindow tells two tabs refreshing together from a replayed token.
	refreshRaceWindow = 10 * time.Second
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

func (s *Service) Register(ctx context.Context, cmd input.RegisterCommand) (*input.RegisterResult, error) {
	name := strings.TrimSpace(cmd.Name)
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	if n := utf8.RuneCountInString(name); n < 2 || n > 80 || strings.ContainsFunc(name, unicode.IsControl) {
		return nil, apperror.New(apperror.CodeInvalidInput, "name must be 2-80 characters").WithField("name")
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return nil, apperror.New(apperror.CodeInvalidInput, "invalid email").WithField("email")
	}

	plain, err := tokenhash.RandomURLSafe()
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token generation failed").WithCause(err)
	}

	// Known and new emails get the same answer, so this route reveals no account.
	res := &input.RegisterResult{UserID: uuid.NewString(), Status: domain.UserStatusPending}
	issued := false
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		u, findErr := uow.Users().FindByEmail(ctx, email)
		switch {
		case findErr == nil:
			if u.Status != domain.UserStatusPending && u.Status != domain.UserStatusExpired {
				return nil
			}
			if cooling, err := s.coolingDown(ctx, uow, u.ID, domain.TokenPurposeActivation); cooling || err != nil {
				return err
			}
			// Unactivated account: send a fresh token instead of a dead end.
			if err := uow.Users().UpdateStatus(ctx, u.ID, domain.UserStatusPending); err != nil {
				return err
			}
		case apperror.IsNotFound(findErr):
			u = &domain.User{Name: name, Email: email, Status: domain.UserStatusPending}
			if err := uow.Users().Create(ctx, u); err != nil {
				return err
			}
			if err := uow.Credentials().Create(ctx, u.ID); err != nil {
				return err
			}
			res.UserID = u.ID
		default:
			return findErr
		}
		issued = true
		return s.issueEmailToken(ctx, uow, u, domain.TokenPurposeActivation, plain, s.Cfg.RegistrationTokenTTL)
	})
	if apperror.IsCode(err, apperror.CodeAlreadyExists) { // lost a race with the same signup
		return res, nil
	}
	if err != nil {
		return nil, err
	}

	if issued && s.Cfg.AllowManualTokenDelivery {
		res.ActivationToken = plain
		res.ManualDelivery = true
	}
	return res, nil
}

func (s *Service) Activate(ctx context.Context, cmd input.ActivateCommand) error {
	if err := validatePassword(cmd.Password); err != nil {
		return err
	}
	hash := tokenhash.SHA256Bytes(cmd.Token)
	now := s.Clock.Now()
	var userID string
	var expired *domain.EmailToken

	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		tok, err := uow.Tokens().FindActiveByHash(ctx, domain.TokenPurposeActivation, hash)
		if err != nil {
			if apperror.IsNotFound(err) {
				return apperror.New(apperror.CodeNotFound, "invalid or used activation token")
			}
			return err
		}
		if tok.UsedAt != nil {
			return apperror.New(apperror.CodeTokenExpired, "activation token already used")
		}
		if !tok.ExpiresAt.After(now) {
			expired = tok
			return apperror.New(apperror.CodeTokenExpired, "activation token expired")
		}

		user, err := uow.Users().FindByID(ctx, tok.UserID)
		if err != nil {
			return err
		}
		if user.Status != domain.UserStatusPending && user.Status != domain.UserStatusExpired {
			return apperror.New(apperror.CodeConflict, "account cannot be activated")
		}

		ph, err := password.Hash(cmd.Password)
		if err != nil {
			return apperror.New(apperror.CodeInternal, "password hash failed").WithCause(err)
		}
		if err := uow.Credentials().SetPassword(ctx, user.ID, ph, domain.PasswordAlgorithmArgon2id); err != nil {
			return err
		}
		if err := uow.Users().UpdateStatus(ctx, user.ID, domain.UserStatusActive); err != nil {
			return err
		}
		roleID, err := uow.Roles().FindDefaultRoleID(ctx)
		if err != nil {
			return err
		}
		if err := uow.Roles().Assign(ctx, user.ID, roleID); err != nil {
			return err
		}
		userID = user.ID
		return uow.Tokens().MarkUsed(ctx, tok.ID, now)
	})
	if expired != nil {
		// Written outside the transaction, which rolled back on the expiry error.
		_ = s.UOW.Users().UpdateStatus(ctx, expired.UserID, domain.UserStatusExpired)
		_, _ = s.UOW.Tokens().IncrementAttempts(ctx, expired.ID)
	}
	if err != nil {
		return err
	}
	if s.Authz != nil && userID != "" {
		_ = s.Authz.Invalidate(ctx, userID)
	}
	return nil
}

func (s *Service) Login(ctx context.Context, cmd input.LoginCommand) (*domain.TokenPair, error) {
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	now := s.Clock.Now()

	if err := s.checkRateLimits(ctx, email, cmd.IPAddress, now); err != nil {
		return nil, err
	}

	// Every refusal without a password check costs one Argon2 run and says the
	// same thing, so neither the answer nor its timing reveals the account state.
	refuse := func(kind string) (*domain.TokenPair, error) {
		password.Waste(cmd.Password)
		_ = s.recordAttempt(ctx, email, cmd.IPAddress, cmd.UserAgent, false, kind)
		return nil, apperror.New(apperror.CodeInvalidCredentials, "invalid credentials")
	}

	user, err := s.UOW.Users().FindByEmail(ctx, email)
	if err != nil {
		if apperror.IsNotFound(err) {
			return refuse(domain.FailureUserNotFound)
		}
		return nil, err
	}

	switch user.Status {
	case domain.UserStatusPending:
		return refuse(domain.FailureAccountPending)
	case domain.UserStatusExpired:
		return refuse(domain.FailureAccountExpired)
	case domain.UserStatusDisabled:
		return refuse(domain.FailureAccountDisabled)
	case domain.UserStatusLocked:
		isLocked, err := s.settleLock(ctx, user, now)
		if err != nil {
			return nil, err
		}
		if isLocked {
			return refuse(domain.FailureAccountLocked)
		}
	case domain.UserStatusActive:
	default:
		return refuse(domain.FailureAccountDisabled)
	}

	cred, err := s.UOW.Credentials().FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if cred.LockedUntil != nil && cred.LockedUntil.After(now) {
		return refuse(domain.FailureAccountLocked)
	}
	if cred.PasswordHash == nil {
		return refuse(domain.FailureBadPassword)
	}

	ok, err := password.Verify(*cred.PasswordHash, cmd.Password)
	if err != nil || !ok {
		lockedUntil, err := s.UOW.Credentials().RecordFailure(ctx, user.ID, s.Cfg.MaxFailedLoginAttempts, now.Add(s.Cfg.AccountLockDuration), now)
		if err != nil {
			return nil, err
		}
		_ = s.recordAttempt(ctx, email, cmd.IPAddress, cmd.UserAgent, false, domain.FailureBadPassword)
		if lockedUntil != nil {
			if err := s.UOW.Users().UpdateStatus(ctx, user.ID, domain.UserStatusLocked); err != nil {
				return nil, err
			}
		}
		return nil, apperror.New(apperror.CodeInvalidCredentials, "invalid credentials")
	}

	// The hash may have been replaced by a reset during the slow verify above.
	applied, err := s.UOW.Credentials().RecordSuccess(ctx, user.ID, *cred.PasswordHash, now)
	if err != nil {
		return nil, err
	}
	if !applied {
		_ = s.recordAttempt(ctx, email, cmd.IPAddress, cmd.UserAgent, false, domain.FailureBadPassword)
		return nil, apperror.New(apperror.CodeInvalidCredentials, "invalid credentials")
	}

	pair, err := s.issueSession(ctx, user.ID, cmd.IPAddress, cmd.UserAgent, now)
	if err != nil {
		return nil, err
	}
	_ = s.recordAttempt(ctx, email, cmd.IPAddress, cmd.UserAgent, true, "")
	return pair, nil
}

func (s *Service) Refresh(ctx context.Context, cmd input.RefreshCommand) (*domain.TokenPair, error) {
	claims, err := s.Tokens.ParseRefresh(cmd.RefreshToken)
	if err != nil {
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid refresh token").WithCause(err)
	}
	jtiHash := tokenhash.SHA256Bytes(claims.JTI)
	now := s.Clock.Now()

	sess, err := s.UOW.Sessions().FindByID(ctx, claims.SessionID)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, apperror.New(apperror.CodeUnauthorized, "invalid refresh token")
		}
		return nil, err
	}
	if sess.UserID != claims.UserID {
		return nil, apperror.New(apperror.CodeUnauthorized, "invalid refresh token")
	}
	if sess.RevokedAt != nil {
		return nil, apperror.New(apperror.CodeUnauthorized, "session revoked")
	}
	if !sess.ExpiresAt.After(now) {
		if err := s.UOW.Sessions().Revoke(ctx, sess.ID, now, domain.RevokeExpired); err != nil {
			return nil, err
		}
		return nil, apperror.New(apperror.CodeTokenExpired, "session expired")
	}

	// A genuinely signed token for this session that is not the live one has
	// already been spent, however many rotations ago.
	if !bytes.Equal(sess.RefreshTokenHash, jtiHash) {
		if bytes.Equal(sess.PreviousRefreshTokenHash, jtiHash) && now.Sub(sess.LastSeenAt) < refreshRaceWindow {
			return nil, refreshRaceLost()
		}
		return nil, s.revokeForReuse(ctx, sess.ID, now)
	}

	user, err := s.UOW.Users().FindByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	// A lock stops password guessing, not a session already proven: otherwise
	// anyone who knows the email could log the owner out at will.
	if user.Status != domain.UserStatusActive && user.Status != domain.UserStatusLocked {
		return nil, apperror.New(apperror.CodeForbidden, "login not allowed")
	}

	newRefresh, newJTI, err := s.Tokens.IssueRefresh(user.ID, sess.ID, now)
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token issue failed").WithCause(err)
	}
	access, err := s.Tokens.IssueAccess(user.ID, sess.ID, now)
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token issue failed").WithCause(err)
	}

	rotated, err := s.UOW.Sessions().Rotate(ctx, sess.ID, jtiHash, tokenhash.SHA256Bytes(newJTI), now.Add(s.Tokens.RefreshTTL()), now)
	if err != nil {
		return nil, err
	}
	if !rotated {
		return nil, refreshRaceLost()
	}

	return &domain.TokenPair{
		AccessToken:  access,
		RefreshToken: newRefresh,
		ExpiresIn:    int64(s.Tokens.AccessTTL().Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// refreshRaceLost answers the loser of two simultaneous refreshes. It gets no
// token, so a thief gains nothing; the client reads the winner's token and retries.
func refreshRaceLost() error {
	return apperror.New(apperror.CodeConflict, "refresh already in progress")
}

func (s *Service) revokeForReuse(ctx context.Context, sessionID string, now time.Time) error {
	if err := s.UOW.Sessions().Revoke(ctx, sessionID, now, domain.RevokeTokenReuse); err != nil {
		return err
	}
	return apperror.New(apperror.CodeUnauthorized, "refresh token reuse detected")
}

func (s *Service) Logout(ctx context.Context, cmd input.LogoutCommand) error {
	claims, err := s.Tokens.ParseAccess(cmd.AccessToken)
	if err != nil {
		return apperror.New(apperror.CodeUnauthorized, "invalid access token").WithCause(err)
	}
	return s.UOW.Sessions().Revoke(ctx, claims.SessionID, s.Clock.Now(), domain.RevokeUserLogout)
}

func (s *Service) RequestPasswordReset(ctx context.Context, cmd input.RequestPasswordResetCommand) (*input.RequestPasswordResetResult, error) {
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	res := &input.RequestPasswordResetResult{}

	user, err := s.UOW.Users().FindByEmail(ctx, email)
	if err != nil {
		if apperror.IsNotFound(err) {
			// do not reveal existence
			return res, nil
		}
		return nil, err
	}
	if user.Status != domain.UserStatusActive && user.Status != domain.UserStatusLocked {
		return res, nil
	}

	plain, err := tokenhash.RandomURLSafe()
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token generation failed").WithCause(err)
	}

	issued := false
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if cooling, err := s.coolingDown(ctx, uow, user.ID, domain.TokenPurposePasswordReset); cooling || err != nil {
			return err
		}
		issued = true
		return s.issueEmailToken(ctx, uow, user, domain.TokenPurposePasswordReset, plain, s.Cfg.PasswordResetTokenTTL)
	})
	if apperror.IsCode(err, apperror.CodeAlreadyExists) { // lost a race with the same request
		return res, nil
	}
	if err != nil {
		return nil, err
	}

	if issued && s.Cfg.AllowManualTokenDelivery {
		res.ResetToken = plain
		res.ManualDelivery = true
	}
	return res, nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, cmd input.ConfirmPasswordResetCommand) error {
	if err := validatePassword(cmd.Password); err != nil {
		return err
	}
	hash := tokenhash.SHA256Bytes(cmd.Token)
	now := s.Clock.Now()
	var refused *domain.EmailToken

	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		tok, err := uow.Tokens().FindActiveByHash(ctx, domain.TokenPurposePasswordReset, hash)
		if err != nil {
			if apperror.IsNotFound(err) {
				return apperror.New(apperror.CodeNotFound, "invalid or used reset token")
			}
			return err
		}
		if tok.UsedAt != nil || !tok.ExpiresAt.After(now) {
			refused = tok
			return apperror.New(apperror.CodeTokenExpired, "reset token expired or used")
		}

		ph, err := password.Hash(cmd.Password)
		if err != nil {
			return apperror.New(apperror.CodeInternal, "password hash failed").WithCause(err)
		}
		if err := uow.Credentials().SetPassword(ctx, tok.UserID, ph, domain.PasswordAlgorithmArgon2id); err != nil {
			return err
		}
		user, err := uow.Users().FindByID(ctx, tok.UserID)
		if err != nil {
			return err
		}
		if user.Status == domain.UserStatusLocked {
			if err := uow.Users().UpdateStatus(ctx, user.ID, domain.UserStatusActive); err != nil {
				return err
			}
		}
		if err := uow.Sessions().RevokeAllForUser(ctx, tok.UserID, now, domain.RevokePasswordChanged); err != nil {
			return err
		}
		return uow.Tokens().MarkUsed(ctx, tok.ID, now)
	})
	if refused != nil {
		// Written outside the transaction, which rolled back on the refusal.
		_, _ = s.UOW.Tokens().IncrementAttempts(ctx, refused.ID)
	}
	return err
}

// settleLock reports whether user is under a running lock. A lapsed lock turns
// the account active again, so it needs no login to recover.
func (s *Service) settleLock(ctx context.Context, user *domain.User, now time.Time) (bool, error) {
	if user.Status != domain.UserStatusLocked {
		return false, nil
	}
	cred, err := s.UOW.Credentials().FindByUserID(ctx, user.ID)
	if err != nil {
		return false, err
	}
	if cred.LockedUntil != nil && cred.LockedUntil.After(now) {
		return true, nil
	}
	if err := s.UOW.Users().UpdateStatus(ctx, user.ID, domain.UserStatusActive); err != nil {
		return false, err
	}
	user.Status = domain.UserStatusActive
	return false, nil
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

func validatePassword(plain string) error {
	if len(plain) < minPasswordBytes {
		return apperror.New(apperror.CodeInvalidInput, "password too short").WithField("password")
	}
	if len(plain) > maxPasswordBytes {
		return apperror.New(apperror.CodeInvalidInput, "password too long").WithField("password")
	}
	return nil
}

func (s *Service) issueSession(ctx context.Context, userID string, ip, ua *string, now time.Time) (*domain.TokenPair, error) {
	// Pre-generate session id so JWT sid matches the row on insert.
	sessionID := uuid.NewString()
	refresh, jti, err := s.Tokens.IssueRefresh(userID, sessionID, now)
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token issue failed").WithCause(err)
	}
	access, err := s.Tokens.IssueAccess(userID, sessionID, now)
	if err != nil {
		return nil, apperror.New(apperror.CodeInternal, "token issue failed").WithCause(err)
	}

	sess := &domain.Session{
		ID:               sessionID,
		UserID:           userID,
		ExpiresAt:        now.Add(s.Tokens.RefreshTTL()),
		RefreshTokenHash: tokenhash.SHA256Bytes(jti),
		IPAddress:        ip,
		UserAgent:        ua,
		LastSeenAt:       now,
	}
	if err := s.UOW.Sessions().Create(ctx, sess); err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.Tokens.AccessTTL().Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func (s *Service) checkRateLimits(ctx context.Context, email string, ip *string, now time.Time) error {
	n, err := s.UOW.Attempts().CountRecentFailuresByEmail(ctx, email, now.Add(-s.Cfg.LoginAttemptEmailWindow))
	if err != nil {
		return err
	}
	if n >= s.Cfg.LoginAttemptEmailMaxFailures {
		_ = s.recordAttempt(ctx, email, ip, nil, false, domain.FailureRateLimitedEmail)
		return apperror.New(apperror.CodeTooManyRequests, "too many failed attempts")
	}
	if ip != nil && *ip != "" {
		n, err = s.UOW.Attempts().CountRecentFailuresByIP(ctx, *ip, now.Add(-s.Cfg.LoginAttemptIPWindow))
		if err != nil {
			return err
		}
		if n >= s.Cfg.LoginAttemptIPMaxFailures {
			_ = s.recordAttempt(ctx, email, ip, nil, false, domain.FailureRateLimitedIP)
			return apperror.New(apperror.CodeTooManyRequests, "too many failed attempts")
		}
	}
	return nil
}

func (s *Service) recordAttempt(ctx context.Context, email string, ip, ua *string, success bool, kind string) error {
	a := &domain.LoginAttempt{
		Email:     email,
		IPAddress: ip,
		UserAgent: ua,
		Success:   success,
	}
	if !success {
		a.FailureKind = pointer.Of(kind)
	}
	return s.UOW.Attempts().Create(ctx, a)
}
