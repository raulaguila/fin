package composition

import (
	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres"
	productrepo "github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/product"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres/rbac"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/tokens"
	"github.com/raulaguila/authenticity/internal/core/port/output"
	"github.com/raulaguila/authenticity/internal/core/usecase/admin"
	"github.com/raulaguila/authenticity/internal/core/usecase/auth"
	"github.com/raulaguila/authenticity/internal/core/usecase/authorize"
	"github.com/raulaguila/authenticity/internal/core/usecase/health"
	"github.com/raulaguila/authenticity/internal/core/usecase/product"
	"github.com/raulaguila/authenticity/internal/infra"
)

// NewAdmin wires the operator use cases of cmd/admin.
func NewAdmin(cfg *config.Environment, res *infra.Resources) *admin.Service {
	authz := authorize.New(rbac.NewRepository(res.PostgresDB), res.Cache, cfg.Cache.TTL)
	return admin.New(postgres.NewStore(res.PostgresDB), output.RealClock{}, authz)
}

// NewApplication wires concrete adapters into use cases / inbound ports.
func NewApplication(cfg *config.Environment, res *infra.Resources) *Application {
	pgHealth := postgres.NewHealthChecker(res.PostgresDB)

	var cachePinger output.Pinger
	var cacheStore output.CacheStore
	if res.Cache != nil {
		cachePinger = res.Cache
		cacheStore = res.Cache
	}

	store := postgres.NewStore(res.PostgresDB)
	permReader := rbac.NewRepository(res.PostgresDB)
	issuer := &tokens.Issuer{
		AccessKey:     cfg.Auth.AccessPrivateKey,
		RefreshKey:    cfg.Auth.RefreshPrivateKey,
		AccessExpire:  cfg.Auth.AccessExpiration,
		RefreshExpire: cfg.Auth.RefreshExpiration,
	}
	authz := authorize.New(permReader, cacheStore, cfg.Cache.TTL)

	authSvc := auth.New(store, store, issuer, output.RealClock{}, auth.Config{
		RegistrationTokenTTL:         cfg.Auth.RegistrationTokenTTL,
		PasswordResetTokenTTL:        cfg.Auth.PasswordResetTokenTTL,
		MaxFailedLoginAttempts:       cfg.Auth.MaxFailedLoginAttempts,
		AccountLockDuration:          cfg.Auth.AccountLockDuration,
		AllowManualTokenDelivery:     cfg.Auth.AllowManualTokenDelivery,
		LoginAttemptEmailWindow:      cfg.Auth.LoginAttemptEmailWindow,
		LoginAttemptEmailMaxFailures: cfg.Auth.LoginAttemptEmailMaxFailures,
		LoginAttemptIPWindow:         cfg.Auth.LoginAttemptIPWindow,
		LoginAttemptIPMaxFailures:    cfg.Auth.LoginAttemptIPMaxFailures,
		EmailTokenCooldown:           cfg.Auth.EmailTokenCooldown,
		OutboxKey:                    cfg.SMTP.TokenKey(),
	}, authz)

	return &Application{
		Health:     health.New(pgHealth, cachePinger),
		Authorizer: authz,
		Auth:       authSvc,
		Products:   product.New(productrepo.NewRepository(res.PostgresDB)),
		Tokens:     issuer,
		Sessions:   store.Sessions(),
	}
}
