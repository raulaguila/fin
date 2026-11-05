package input

import (
	"context"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

type RegisterCommand struct {
	Name  string
	Email string
}

type RegisterResult struct {
	UserID          string
	Status          string
	ActivationToken string // only when AllowManualTokenDelivery
	ManualDelivery  bool
}

type ActivateCommand struct {
	Token    string
	Password string
}

type LoginCommand struct {
	Email     string
	Password  string
	IPAddress *string
	UserAgent *string
}

type RefreshCommand struct {
	RefreshToken string
}

type LogoutCommand struct {
	AccessToken string
}

type RequestPasswordResetCommand struct {
	Email string
}

type RequestPasswordResetResult struct {
	ResetToken     string
	ManualDelivery bool
}

type ConfirmPasswordResetCommand struct {
	Token    string
	Password string
}

// AuthService is the inbound port for identity flows.
type AuthService interface {
	Register(ctx context.Context, cmd RegisterCommand) (*RegisterResult, error)
	Activate(ctx context.Context, cmd ActivateCommand) error
	Login(ctx context.Context, cmd LoginCommand) (*domain.TokenPair, error)
	Refresh(ctx context.Context, cmd RefreshCommand) (*domain.TokenPair, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
	RequestPasswordReset(ctx context.Context, cmd RequestPasswordResetCommand) (*RequestPasswordResetResult, error)
	ConfirmPasswordReset(ctx context.Context, cmd ConfirmPasswordResetCommand) error
}
