// Package admin holds operator actions that no HTTP route exposes yet.
package admin

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/internal/core/usecase/auth"
	"github.com/raulaguila/authenticity/pkg/apperror"
	"github.com/raulaguila/authenticity/pkg/password"
)

const adminRole = "admin"

// Service implements the commands of cmd/admin.
type Service struct {
	Tx    output.TxManager
	Clock output.Clock
}

func New(tx output.TxManager, clock output.Clock) *Service {
	return &Service{Tx: tx, Clock: clock}
}

// BootstrapCommand describes the first admin.
type BootstrapCommand struct {
	Name     string
	Email    string
	Password string
}

// Bootstrap creates the first admin, active and ready to log in, with no email
// involved. Once any admin exists it does nothing and returns false. An email
// already in use is an error: someone else may have registered it.
func (s *Service) Bootstrap(ctx context.Context, cmd BootstrapCommand) (bool, error) {
	name, email, err := auth.ValidateIdentity(cmd.Name, cmd.Email)
	if err != nil {
		return false, err
	}
	if err := auth.ValidatePassword(cmd.Password); err != nil {
		return false, err
	}
	// Hashed before the transaction: Argon2 must not hold a database connection.
	hash, err := password.Hash(ctx, cmd.Password)
	if err != nil {
		return false, apperror.New(apperror.CodeInternal, "password hash failed").WithCause(err)
	}
	created := false
	err = s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		if err := uow.Attempts().Lock(ctx, "admin:bootstrap"); err != nil { // two runs at once make one admin
			return err
		}
		adminID, err := uow.Roles().FindIDByName(ctx, adminRole)
		if err != nil {
			return err
		}
		exists, err := uow.Roles().HasMembers(ctx, adminID)
		if exists || err != nil {
			return err
		}
		switch _, err := uow.Users().FindByEmail(ctx, email); {
		case err == nil:
			return apperror.New(apperror.CodeConflict, "email already in use").WithField("email")
		case !apperror.IsNotFound(err):
			return err
		}
		user := &domain.User{Name: name, Email: email, Status: domain.UserStatusActive}
		if err := uow.Users().Create(ctx, user); err != nil {
			return err
		}
		if err := uow.Credentials().Create(ctx, user.ID); err != nil {
			return err
		}
		if err := uow.Credentials().SetPassword(ctx, user.ID, hash, domain.PasswordAlgorithmArgon2id); err != nil {
			return err
		}
		defaultID, err := uow.Roles().FindDefaultRoleID(ctx)
		if err != nil {
			return err
		}
		if err := uow.Roles().Assign(ctx, user.ID, defaultID); err != nil {
			return err
		}
		created = true
		return uow.Roles().Assign(ctx, user.ID, adminID)
	})
	return created && err == nil, err
}

// Promote gives an active user the admin role.
func (s *Service) Promote(ctx context.Context, email string) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		user, err := uow.Users().FindByEmail(ctx, auth.NormalizeEmail(email))
		if err != nil {
			return err
		}
		if user.Status != domain.UserStatusActive {
			return apperror.New(apperror.CodeConflict, "user is "+user.Status+", not active")
		}
		roleID, err := uow.Roles().FindIDByName(ctx, adminRole)
		if err != nil {
			return err
		}
		return uow.Roles().Assign(ctx, user.ID, roleID)
	})
}

// Disable blocks a user and revokes their sessions, so access tokens stop at
// the next request: Bearer checks the session every time.
func (s *Service) Disable(ctx context.Context, email string) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context, uow output.UnitOfWork) error {
		user, err := uow.Users().FindByEmail(ctx, auth.NormalizeEmail(email))
		if err != nil {
			return err
		}
		if err := uow.Users().UpdateStatus(ctx, user.ID, domain.UserStatusDisabled); err != nil {
			return err
		}
		return uow.Sessions().RevokeAllForUser(ctx, user.ID, s.Clock.Now(), domain.RevokeAccountDisabled)
	})
}
