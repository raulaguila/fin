package rest

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/raulaguila/authenticity/config"
	authhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/auth"
	healthhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/health"
	notificationhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/notification"
	permissionhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/permission"
	preferencehttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/preference"
	producthttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/product"
	rolehttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/role"
	sessionhttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/session"
	usershttp "github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/handler/user"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/mapper"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest/middleware"
	"github.com/raulaguila/authenticity/internal/core/port/input"
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
		ErrorHandler:       mapper.ErrorHandler,
		EnableIPValidation: true, // c.IP() goes into an inet column
	}
	if api.TrustedProxies != "" {
		cfg.ProxyHeader = api.ProxyHeader
		cfg.TrustProxy = true
		cfg.TrustProxyConfig.Proxies = splitList(api.TrustedProxies)
	}
	return cfg
}

func splitList(list string) []string {
	return strings.Split(strings.ReplaceAll(list, " ", ""), ",")
}

// New builds the Fiber app with middleware and routes.
func New(cfg *config.Environment, app *input.Application, log *logger.Logger) *fiber.App {
	fiberApp := fiber.New(serverConfig(cfg.API))

	if l := middleware.RequestLog(log, cfg.API.EnableLogger); l != nil {
		fiberApp.Use(l)
	}

	fiberApp.Use(
		recover.New(recover.Config{EnableStackTrace: true}),
		cors.New(cors.Config{
			AllowOrigins:        splitList(cfg.API.CorsOrigins),
			AllowMethods:        []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete, fiber.MethodOptions},
			AllowHeaders:        []string{fiber.HeaderAuthorization, fiber.HeaderContentType, fiber.HeaderAccept, fiber.HeaderAcceptLanguage, fiber.HeaderXRequestID, fiber.HeaderOrigin},
			ExposeHeaders:       []string{fiber.HeaderContentType, fiber.HeaderContentLength, fiber.HeaderXRequestID},
			AllowPrivateNetwork: true,
			MaxAge:              3600,
		}),
		middleware.Timeout(cfg.API.RequestTimeout),
	)

	fiberApp.Get("/health", healthhttp.NewHandler(app.Health).Get) // before the limiter: probes must always answer
	if !cfg.Protected() {
		mountSwagger(fiberApp, cfg.Version) // never in production: it maps every route
	}

	fiberApp.Use(middleware.RateLimit(cfg.API.GlobalRateLimit))
	bearer := middleware.Bearer(app.Auth)

	v1 := fiberApp.Group("/v1")

	auth := v1.Group("/auth")
	authhttp.New(app.Auth, app.Account).Routes(auth, bearer, middleware.RateLimit(cfg.API.AuthRateLimit), middleware.RateLimit(cfg.API.PasswordRateLimit))
	sessionhttp.New(app.Account).Routes(auth.Group("/sessions", bearer))

	user := v1.Group("/user", bearer)
	preferencehttp.New(app.Account).Routes(user.Group("/preferences"))
	notificationhttp.New(app.Account).Routes(user.Group("/notifications"))

	permissionhttp.New(app.Resources).Routes(v1.Group("/permissions", bearer), app.Authorizer)
	rolehttp.New(app.Roles).Routes(v1.Group("/roles", bearer), app.Authorizer)
	usershttp.New(app.Users).Routes(v1.Group("/users", bearer), app.Authorizer)
	producthttp.New(app.Products).Routes(v1.Group("/products", bearer), app.Authorizer)

	return fiberApp
}
