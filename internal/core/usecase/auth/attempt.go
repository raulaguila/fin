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
