package auth

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/clientip"
	"github.com/raulaguila/authenticity/pkg/password"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

// refreshRaceWindow tells two tabs refreshing together from a replayed token.
const refreshRaceWindow = 10 * time.Second

const settleTimeout = 2 * time.Second

func (s *Service) Login(ctx context.Context, cmd input.LoginCommand) (*domain.TokenPair, error) {
	email := normalizeEmail(cmd.Email)
	now := s.Clock.Now()

	attemptID, err := s.reserveAttempt(ctx, email, cmd.IPAddress, cmd.UserAgent, now)
	if err != nil {
		return nil, err
	}

	pair, failure, err := s.authenticate(ctx, email, cmd, now)

	// The request may be out of time, and the outcome must still land.
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	switch {
	case pair != nil:
		_ = s.UOW.Attempts().Settle(settleCtx, attemptID, true, nil)
	case failure != "":
		_ = s.UOW.Attempts().Settle(settleCtx, attemptID, false, &failure)
	default:
		_ = s.UOW.Attempts().Release(settleCtx, attemptID)
	}
	return pair, err
}

// authenticate returns the failure kind to store for a judged attempt; none
// means no verdict was reached, as on a timeout.
func (s *Service) authenticate(ctx context.Context, email string, cmd input.LoginCommand, now time.Time) (*domain.TokenPair, string, error) {
	// Every refusal without a password check costs one Argon2 run and says the
	// same thing, so neither the answer nor its timing reveals the account state.
	refuse := func(kind string) (*domain.TokenPair, string, error) {
		password.Waste(ctx, cmd.Password)
		if ctx.Err() != nil {
			return nil, "", requestTimedOut()
		}
		return nil, kind, invalidCredentials()
	}

	user, err := s.UOW.Users().FindByEmail(ctx, email)
	if err != nil {
		if apperror.IsNotFound(err) {
			return refuse(domain.FailureUserNotFound)
		}
		return nil, "", err
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
			return nil, "", err
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
		return nil, "", err
	}
	if cred.LockedUntil != nil && cred.LockedUntil.After(now) {
		return refuse(domain.FailureAccountLocked)
	}
	if cred.PasswordHash == nil {
		return refuse(domain.FailureBadPassword)
	}

	ok, err := password.Verify(ctx, *cred.PasswordHash, cmd.Password)
	if ctx.Err() != nil { // running out of time says nothing about the password
		return nil, "", requestTimedOut()
	}
	if err != nil || !ok {
		lockedUntil, err := s.UOW.Credentials().RecordFailure(ctx, user.ID, s.Cfg.MaxFailedLoginAttempts, now.Add(s.Cfg.AccountLockDuration), now)
		if err != nil {
			return nil, "", err
		}
		if lockedUntil != nil {
			if err := s.UOW.Users().UpdateStatus(ctx, user.ID, domain.UserStatusLocked); err != nil {
				return nil, domain.FailureBadPassword, err
			}
		}
		return nil, domain.FailureBadPassword, invalidCredentials()
	}

	// The hash may have been replaced by a reset during the slow verify above.
	applied, err := s.UOW.Credentials().RecordSuccess(ctx, user.ID, *cred.PasswordHash, now)
	if err != nil {
		return nil, "", err
	}
	if !applied {
		return nil, domain.FailureBadPassword, invalidCredentials()
	}

	pair, err := s.issueSession(ctx, user.ID, cmd.IPAddress, cmd.UserAgent, now)
	return pair, "", err
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

// reserveAttempt checks the limits and stores the attempt as failed before any
// password is tested, under a lock per client and account: parallel requests see
// each other, so none passes a limit because the others are still hashing.
func (s *Service) reserveAttempt(ctx context.Context, email string, ip, ua *string, now time.Time) (string, error) {
	var id, limited string
	err := s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		attempts := uow.Attempts()
		// Always this order: two logins never wait on each other's lock.
		if ip != nil && *ip != "" {
			first, _ := clientip.Range(*ip)
			if err := attempts.Lock(ctx, "login:ip:"+first); err != nil {
				return err
			}
		}
		if err := attempts.Lock(ctx, "login:email:"+email); err != nil {
			return err
		}

		var err error
		if limited, err = s.overLimit(ctx, attempts, email, ip, now); err != nil {
			return err
		}
		kind := domain.FailureBadPassword
		if limited != "" {
			kind = limited
		}
		a := &domain.LoginAttempt{Email: email, IPAddress: ip, UserAgent: ua, FailureKind: &kind}
		if err := attempts.Create(ctx, a); err != nil {
			return err
		}
		id = a.ID
		return nil
	})
	if err != nil {
		return "", err
	}
	if limited != "" {
		return "", apperror.New(apperror.CodeTooManyRequests, "too many failed attempts")
	}
	return id, nil
}

// overLimit names the limit this login is over, if any.
func (s *Service) overLimit(ctx context.Context, attempts output.LoginAttemptRepository, email string, ip *string, now time.Time) (string, error) {
	n, err := attempts.CountRecentFailuresByEmail(ctx, email, now.Add(-s.Cfg.LoginAttemptEmailWindow))
	if err != nil {
		return "", err
	}
	if n >= s.Cfg.LoginAttemptEmailMaxFailures {
		return domain.FailureRateLimitedEmail, nil
	}
	if ip == nil || *ip == "" {
		return "", nil
	}
	n, err = attempts.CountRecentFailuresByEmailAndIP(ctx, email, *ip, now.Add(-s.Cfg.LoginAttemptEmailWindow))
	if err != nil {
		return "", err
	}
	if n >= s.Cfg.LoginAttemptPairMaxFailures {
		return domain.FailureRateLimitedEmail, nil
	}
	n, err = attempts.CountRecentFailuresByIP(ctx, *ip, now.Add(-s.Cfg.LoginAttemptIPWindow))
	if err != nil {
		return "", err
	}
	if n >= s.Cfg.LoginAttemptIPMaxFailures {
		return domain.FailureRateLimitedIP, nil
	}
	return "", nil
}

func invalidCredentials() error {
	return apperror.New(apperror.CodeInvalidCredentials, "invalid credentials")
}
