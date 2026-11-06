package auth

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/password"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

const (
	minPasswordBytes = 8
	maxPasswordBytes = 128
)

func (s *Service) RequestPasswordReset(ctx context.Context, cmd input.RequestPasswordResetCommand) (*input.RequestPasswordResetResult, error) {
	email := NormalizeEmail(cmd.Email)
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
		if cooling, err := s.emailThrottled(ctx, uow, user.ID, domain.TokenPurposePasswordReset); cooling || err != nil {
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
	if err := ValidatePassword(cmd.Password); err != nil {
		return err
	}
	hash := tokenhash.SHA256Bytes(cmd.Token)
	now := s.Clock.Now()

	if tok, err := resetToken(ctx, s.UOW, hash, now); err != nil {
		if tok != nil {
			if _, err := s.UOW.Tokens().IncrementAttempts(ctx, tok.ID); err != nil {
				s.warn("Token attempt not counted", err)
			}
		}
		return err
	}

	// Hashed before the transaction: Argon2 must not hold a database connection.
	ph, err := password.Hash(ctx, cmd.Password)
	if err != nil {
		return hashError(ctx, err)
	}

	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		tok, err := resetToken(ctx, uow, hash, now)
		if err != nil {
			return err
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
		if err := uow.Notifications().Create(ctx, &domain.Notification{
			UserID:  tok.UserID,
			Level:   domain.NotificationWarning,
			Title:   "Password changed",
			Message: "Your password was changed and every session was ended.",
		}); err != nil {
			return err
		}
		return uow.Tokens().MarkUsed(ctx, tok.ID, now)
	})
}

// resetToken checks a reset token. A refused token comes back with the error,
// for the caller to count outside any rolled-back transaction.
func resetToken(ctx context.Context, uow output.UnitOfWork, hash []byte, now time.Time) (*domain.EmailToken, error) {
	tok, err := uow.Tokens().FindActiveByHash(ctx, domain.TokenPurposePasswordReset, hash)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, apperror.New(apperror.CodeNotFound, "invalid or used reset token")
		}
		return nil, err
	}
	if tok.UsedAt != nil || !tok.ExpiresAt.After(now) {
		return tok, apperror.New(apperror.CodeTokenExpired, "reset token expired or used")
	}
	return tok, nil
}

// hashError tells a request that ran out of time from a real hashing failure.
func hashError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return requestTimedOut()
	}
	return apperror.New(apperror.CodeInternal, "password hash failed").WithCause(err)
}

func requestTimedOut() error {
	return apperror.New(apperror.CodeTimedOut, "request timed out")
}

// ValidatePassword applies the password policy.
func ValidatePassword(plain string) error {
	if len(plain) < minPasswordBytes {
		return apperror.New(apperror.CodeInvalidInput, "password too short").WithField("password")
	}
	if len(plain) > maxPasswordBytes {
		return apperror.New(apperror.CodeInvalidInput, "password too long").WithField("password")
	}
	return nil
}
