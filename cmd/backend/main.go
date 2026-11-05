package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/adapter/inbound/http/rest"
	"github.com/raulaguila/authenticity/internal/composition"
	"github.com/raulaguila/authenticity/internal/infra"
	"github.com/raulaguila/authenticity/pkg/logger"
)

func initLogger(cfg *config.Environment) *logger.Logger {
	return logger.New(logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		TimeFormat: time.DateTime,
		Fields:     map[string]any{"version": cfg.Version, "service": "backend"},
	})
}

func main() {
	cfg := config.MustLoad()
	log := initLogger(cfg)
	log.Info("Configuration loaded", slog.Int("port", cfg.API.Port), slog.String("version", cfg.Version), slog.String("environment", cfg.Environment))

	res, err := infra.New(cfg, log, true)
	if err != nil {
		log.Error("Infrastructure bootstrap failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer res.Shutdown(log)

	app := composition.NewApplication(cfg, res, log)
	fiberApp := rest.NewServer(cfg, app, log)

	addr := fmt.Sprintf(":%d", cfg.API.Port)
	go func() {
		log.Info("HTTP server listening", slog.String("addr", addr), slog.Bool("prefork", cfg.API.EnablePrefork))
		if err := fiberApp.Listen(addr, rest.ListenConfig(cfg.API)); err != nil {
			log.Error("HTTP server stopped", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Info("Shutdown signal received", slog.String("signal", sig.String()))

	if err := fiberApp.ShutdownWithTimeout(cfg.API.ShutdownTimeout); err != nil {
		log.Warn("HTTP shutdown error", slog.String("error", err.Error()))
	}
	log.Info("Backend stopped")
}
