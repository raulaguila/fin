package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/infra"
	"github.com/raulaguila/authenticity/pkg/logger"
)

// initLogger creates and configures the application logger
func initLogger(cfg *config.Environment) *logger.Logger {
	return logger.New(logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		TimeFormat: time.DateTime,
		Fields:     map[string]any{"version": cfg.Version, "service": "backend"},
	})
}

// @title 						Authenticity Golang API
// @description 				Backend API in hexagonal structure.

// @contact.name				Raul del Aguila
// @contact.url 				https://github.com/raulaguila
// @contact.email				admin@admin.com

// @BasePath					/v1

// @securityDefinitions.apiKey	Bearer
// @in							header
// @name						Authorization
// @description 				Type "Bearer" followed by a space and the JWT token.
func main() {
	// Load configuration, panic if error
	cfg := config.MustLoad()

	// Initialize logger
	log := initLogger(cfg)
	log.Info("Configuration loaded", slog.Int("port", cfg.API.Port), slog.String("version", cfg.Version), slog.String("environment", cfg.Environment))

	// Initialize database connections
	_, err := infra.New(cfg, log, true)
	if err != nil {
		log.Error("Infrastructure bootstrap failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
