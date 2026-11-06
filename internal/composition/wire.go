package composition

import (
	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres"
	productrepo "github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/product"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/rbac"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/tokens"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/internal/core/usecase/account"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/internal/core/usecase/auth"
	"github.com/raulaguila/authenticity/internal/core/usecase/authorize"
	"github.com/raulaguila/authenticity/internal/core/usecase/health"
	"github.com/raulaguila/authenticity/internal/core/usecase/product"
	"github.com/raulaguila/authenticity/internal/infra"
)

// NewAdmin wires the operator use cases of cmd/admin.
func NewAdmin(res *infra.Resources) *admin.Service {
	return admin.New(postgres.NewStore(res.PostgresDB), output.RealClock{})
}

// NewApplication wires concrete adapters into use cases / inbound ports.
func NewApplication(cfg *config.Environment, res *infra.Resources) *Application {
	pgHealth := postgres.NewHealthChecker(res.PostgresDB)

	store := postgres.NewStore(res.PostgresDB)
	permReader := rbac.NewRepository(res.PostgresDB)
	issuer := &tokens.Issuer{
		AccessKey:     cfg.Auth.AccessPrivateKey,
		RefreshKey:    cfg.Auth.RefreshPrivateKey,
		AccessExpire:  cfg.Auth.AccessExpiration,
		RefreshExpire: cfg.Auth.RefreshExpiration,
	}

	authSvc := auth.New(store, store, issuer, output.RealClock{}, auth.Config{
		RegistrationTokenTTL:         cfg.Auth.RegistrationTokenTTL,
		PasswordResetTokenTTL:        cfg.Auth.PasswordResetTokenTTL,
		MaxFailedLoginAttempts:       cfg.Auth.MaxFailedLoginAttempts,
		AccountLockDuration:          cfg.Auth.AccountLockDuration,
		AllowManualTokenDelivery:     cfg.Auth.AllowManualTokenDelivery,
		LoginAttemptEmailWindow:      cfg.Auth.LoginAttemptEmailWindow,
		LoginAttemptEmailMaxFailures: cfg.Auth.LoginAttemptEmailMaxFailures,
		LoginAttemptPairMaxFailures:  cfg.Auth.LoginAttemptPairMaxFailures,
		LoginAttemptIPWindow:         cfg.Auth.LoginAttemptIPWindow,
		LoginAttemptIPMaxFailures:    cfg.Auth.LoginAttemptIPMaxFailures,
		EmailTokenCooldown:           cfg.Auth.EmailTokenCooldown,
		EmailTokenHourlyMax:          cfg.Auth.EmailTokenHourlyMax,
		OutboxKey:                    cfg.SMTP.TokenKey(),
	})

	return &Application{
		Health:     health.New(pgHealth),
		Authorizer: authorize.New(permReader),
		Auth:       authSvc,
		Account:    account.New(store, permReader, output.RealClock{}),
		Products:   product.New(productrepo.NewRepository(res.PostgresDB)),
	}
}
