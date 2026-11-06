package composition

import (
	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/outbox"
	productrepo "github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/product"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/rbac"
	smtpmail "github.com/raulaguila/authenticity/internal/adapter/outbound/smtp"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/tokens"
	"github.com/raulaguila/authenticity/internal/core/port/input"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/internal/core/usecase/account"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/internal/core/usecase/auth"
	"github.com/raulaguila/authenticity/internal/core/usecase/authorize"
	"github.com/raulaguila/authenticity/internal/core/usecase/health"
	"github.com/raulaguila/authenticity/internal/core/usecase/mailer"
	"github.com/raulaguila/authenticity/internal/core/usecase/product"
	rolesvc "github.com/raulaguila/authenticity/internal/core/usecase/role"
	usersvc "github.com/raulaguila/authenticity/internal/core/usecase/user"
	"github.com/raulaguila/authenticity/internal/infra"
	"github.com/raulaguila/authenticity/pkg/logger"
)

// NewAdmin wires the operator use cases of cmd/admin.
func NewAdmin(res *infra.Resources) *admin.Service {
	return admin.New(postgres.NewStore(res.PostgresDB), output.RealClock{})
}

// NewMailer wires the worker of cmd/mailer.
func NewMailer(cfg *config.Environment, res *infra.Resources, log *logger.Logger) *mailer.Worker {
	return &mailer.Worker{
		Outbox: outbox.NewRepository(res.PostgresDB),
		Mail:   smtpmail.NewSender(cfg.SMTP),
		Log:    log,
		Clock:  output.RealClock{},
		Key:    cfg.SMTP.TokenKey(),
	}
}

// NewApplication wires concrete adapters into use cases / inbound ports.
func NewApplication(cfg *config.Environment, res *infra.Resources, log *logger.Logger) *input.Application {
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
		SessionMaxLifetime:           cfg.Auth.SessionMaxLifetime,
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

	authSvc.Log = log

	authorizer := authorize.New(permReader)

	return &input.Application{
		Health:     health.New(pgHealth),
		Authorizer: authorizer,
		Resources:  authorizer,
		Roles:      rolesvc.New(store, store, permReader),
		Users:      usersvc.New(store, store, output.RealClock{}),
		Auth:       authSvc,
		Account:    account.New(store, permReader, output.RealClock{}),
		Products:   product.New(productrepo.NewRepository(res.PostgresDB)),
	}
}
