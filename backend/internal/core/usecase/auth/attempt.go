package auth

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/clientip"
)

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
			if err := uow.Lock(ctx, "login:ip:"+first); err != nil {
				return err
			}
		}
		if err := uow.Lock(ctx, "login:email:"+email); err != nil {
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
		a := &domain.LoginAttempt{CreatedAt: now, Email: email, IPAddress: ip, UserAgent: ua, FailureKind: &kind}
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
		// The window is an upper bound: older failures may leave it sooner.
		wait := s.Cfg.LoginAttemptEmailWindow
		if limited == domain.FailureRateLimitedIP {
			wait = s.Cfg.LoginAttemptIPWindow
		}
		return "", apperror.New(apperror.CodeTooManyRequests, "too many failed attempts").
			WithDetails(apperror.DetailRetryAfter, int(wait.Seconds()))
	}
	return id, nil
}

// overLimit names the limit this login is over, if any.
func (s *Service) overLimit(ctx context.Context, attempts output.LoginAttemptRepository, email string, ip *string, now time.Time) (string, error) {
	n, err := attempts.CountRecentFailures(ctx, email, ip, now.Add(-s.Cfg.LoginAttemptEmailWindow), now.Add(-s.Cfg.LoginAttemptIPWindow))
	switch {
	case err != nil:
		return "", err
	case n.Email >= s.Cfg.LoginAttemptEmailMaxFailures, n.Pair >= s.Cfg.LoginAttemptPairMaxFailures:
		return domain.FailureRateLimitedEmail, nil
	case n.IP >= s.Cfg.LoginAttemptIPMaxFailures:
		return domain.FailureRateLimitedIP, nil
	}
	return "", nil
}
