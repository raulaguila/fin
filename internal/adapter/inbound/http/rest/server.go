package rest

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/raulaguila/authenticity/config"
	authhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/auth"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/health"
	mehttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/me"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	producthttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/product"
	"github.com/raulaguila/authenticity/internal/composition"
	"github.com/raulaguila/authenticity/internal/core/domain"
	"github.com/raulaguila/authenticity/pkg/logger"
)

func serverConfig(api config.APIConfig) fiber.Config {
	cfg := fiber.Config{
		AppName:            "authenticity",
		ReadTimeout:        api.ReadTimeout,
		WriteTimeout:       api.WriteTimeout,
		IdleTimeout:        api.IdleTimeout,
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
	return fiber.ListenConfig{EnablePrefork: api.EnablePrefork, DisableStartupMessage: true}
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
	authHandler := authhttp.NewHandler(app.Auth)
	meHandler := mehttp.NewHandler(app.Authorizer)
	productHandler := producthttp.NewHandler(app.Products)

	bearer := middleware.Bearer(app.Tokens, app.Sessions)
	requirePermRead := middleware.Require(app.Authorizer, "permissions", domain.ActionRead)
	requireProductRead := middleware.Require(app.Authorizer, "products", domain.ActionRead)
	requireProductCreate := middleware.Require(app.Authorizer, "products", domain.ActionCreate)
	requireProductUpdate := middleware.Require(app.Authorizer, "products", domain.ActionUpdate)
	requireProductDelete := middleware.Require(app.Authorizer, "products", domain.ActionDelete)

	fiberApp.Get("/health", healthHandler.Get) // before the limiter: probes must always answer

	fiberApp.Use(middleware.RateLimit(cfg.API.GlobalRateLimit))
	authLimit := middleware.RateLimit(cfg.API.AuthRateLimit)
	passwordLimit := middleware.RateLimit(cfg.API.PasswordRateLimit)

	v1 := fiberApp.Group("/v1")
	v1.Get("/health", healthHandler.Get)

	auth := v1.Group("/auth")
	auth.Post("/register", authLimit, authHandler.Register)
	auth.Post("/activate", authLimit, authHandler.Activate)
	auth.Post("/login", authLimit, authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Post("/logout", bearer, authHandler.Logout)
	auth.Post("/password-reset/request", passwordLimit, authHandler.RequestPasswordReset)
	auth.Post("/password-reset/confirm", passwordLimit, authHandler.ConfirmPasswordReset)

	v1.Get("/me", bearer, meHandler.Get)
	v1.Get("/me/permissions", bearer, requirePermRead, meHandler.Permissions)

	products := v1.Group("/products", bearer)
	products.Get("/", requireProductRead, productHandler.List)
	products.Get("/:id", requireProductRead, productHandler.Get)
	products.Post("/", requireProductCreate, productHandler.Create)
	products.Patch("/:id", requireProductUpdate, productHandler.Update)
	products.Delete("/:id", requireProductDelete, productHandler.Delete)

	return fiberApp
}
