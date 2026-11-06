package rest

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/raulaguila/authenticity/config"
	authhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/auth"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/health"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	notificationhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/notification"
	preferencehttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/preference"
	producthttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/product"
	sessionhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/session"
	"github.com/raulaguila/authenticity/internal/composition"
	"github.com/raulaguila/authenticity/pkg/logger"
)

// maxBodyBytes bounds what a client makes the server buffer: the body is read
// before any middleware runs, rate limit included. No route needs more.
const maxBodyBytes = 16 * 1024

func serverConfig(api config.APIConfig) fiber.Config {
	cfg := fiber.Config{
		AppName:            "authenticity",
		ReadTimeout:        api.ReadTimeout,
		WriteTimeout:       api.WriteTimeout,
		IdleTimeout:        api.IdleTimeout,
		BodyLimit:          maxBodyBytes,
		EnableIPValidation: true, // c.IP() goes into an inet column
	}
	if api.TrustedProxies != "" {
		cfg.ProxyHeader = api.ProxyHeader
		cfg.TrustProxy = true
		cfg.TrustProxyConfig.Proxies = splitList(api.TrustedProxies)
	}
	return cfg
}

// ListenConfig returns the startup options of Listen.
func ListenConfig(api config.APIConfig) fiber.ListenConfig {
	return fiber.ListenConfig{EnablePrefork: api.EnablePrefork, DisableStartupMessage: false}
}

func splitList(list string) []string {
	return strings.Split(strings.ReplaceAll(list, " ", ""), ",")
}

// NewServer builds the Fiber app with middleware and routes.
func NewServer(cfg *config.Environment, app *composition.Application, log *logger.Logger) *fiber.App {
	fiberApp := fiber.New(serverConfig(cfg.API))

	fiberApp.Use(middleware.RequestLog(log, cfg.API.EnableLogger))
	fiberApp.Use(recoverer.New(recoverer.Config{EnableStackTrace: true}))
	fiberApp.Use(cors.New(cors.Config{
		AllowOrigins: splitList(cfg.API.CorsOrigins),
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))
	fiberApp.Use(middleware.Timeout(cfg.API.RequestTimeout))

	healthHandler := health.NewHandler(app.Health)

	fiberApp.Get("/health", healthHandler.Get) // before the limiter: probes must always answer
	if !cfg.Protected() {
		mountSwagger(fiberApp, cfg.Version) // never in production: it maps every route
	}

	fiberApp.Use(middleware.RateLimit(cfg.API.GlobalRateLimit))
	bearer := middleware.Bearer(app.Auth)

	v1 := fiberApp.Group("/v1")
	healthHandler.Routes(v1)

	auth := v1.Group("/auth")
	authhttp.NewHandler(app.Auth, app.Account).Routes(auth, bearer,
		middleware.RateLimit(cfg.API.AuthRateLimit), middleware.RateLimit(cfg.API.PasswordRateLimit))
	sessionhttp.NewHandler(app.Account).Routes(auth.Group("/sessions", bearer))

	user := v1.Group("/user", bearer)
	preferencehttp.NewHandler(app.Account).Routes(user.Group("/preferences"))
	notificationhttp.NewHandler(app.Account).Routes(user.Group("/notifications"))

	producthttp.NewHandler(app.Products).Routes(v1.Group("/products", bearer), app.Authorizer)

	return fiberApp
}
