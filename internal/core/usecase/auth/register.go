package auth

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/policy"

	"github.com/google/uuid"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/password"
	"github.com/raulaguila/authenticity/pkg/tokenhash"
)

func (s *Service) Register(ctx context.Context, cmd input.RegisterCommand) (*input.RegisterResult, error) {
	name, email, err := policy.ValidateIdentity(cmd.Name, cmd.Email)
	if err != nil {
		return nil, err
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
			if u.Status != domain.UserStatusPending {
				return nil
			}
			if cooling, err := s.emailThrottled(ctx, uow, u.ID, domain.TokenPurposeActivation); cooling || err != nil {
				return err
			}
			// Unactivated account: send a fresh token instead of a dead end.
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
	if err := policy.ValidatePassword(cmd.Password); err != nil {
		return err
	}
	hash := tokenhash.SHA256Bytes(cmd.Token)
	now := s.Clock.Now()

	if tok, _, err := activationTarget(ctx, s.UOW, hash, now); err != nil {
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

	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		tok, user, err := activationTarget(ctx, uow, hash, now)
		if err != nil {
			return err
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
		return uow.Tokens().MarkUsed(ctx, tok.ID, now)
	})
	return err
}

// activationTarget checks an activation token. An expired token comes back with
// the error, for the caller to record outside any rolled-back transaction.
func activationTarget(ctx context.Context, uow output.UnitOfWork, hash []byte, now time.Time) (*domain.EmailToken, *domain.User, error) {
	tok, err := uow.Tokens().FindActiveByHash(ctx, domain.TokenPurposeActivation, hash)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, nil, apperror.New(apperror.CodeNotFound, "invalid or used activation token")
		}
		return nil, nil, err
	}
	if tok.UsedAt != nil {
		return nil, nil, apperror.New(apperror.CodeTokenExpired, "activation token already used")
	}
	if !tok.ExpiresAt.After(now) {
		return tok, nil, apperror.New(apperror.CodeTokenExpired, "activation token expired")
	}
	user, err := uow.Users().FindByID(ctx, tok.UserID)
	if err != nil {
		return nil, nil, err
	}
	if user.Status != domain.UserStatusPending {
		return nil, nil, apperror.New(apperror.CodeConflict, "account cannot be activated")
	}
	return tok, user, nil
}
