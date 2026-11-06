package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/composition"
	"github.com/raulaguila/authenticity/internal/infra"
	"github.com/raulaguila/authenticity/pkg/logger"
)

func initLogger(cfg *config.Environment) *logger.Logger {
	return logger.New(logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		TimeFormat: time.DateTime,
		Fields:     map[string]any{"version": cfg.Version, "service": "mailer"},
	})
}

func main() {
	cfg := config.MustLoad()
	log := initLogger(cfg)
	log.Info("Configuration loaded", slog.String("version", cfg.Version), slog.String("environment", cfg.Environment))

	res, err := infra.New(cfg, log)
	if err != nil {
		log.Error("Infrastructure bootstrap failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer res.Shutdown(log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := composition.NewMailer(cfg, res, log).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("Mailer stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
