package infra

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/cache"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres"
	"github.com/raulaguila/authenticity/pkg/logger"
)

type Resources struct {
	PostgresDB *gorm.DB
	Cache      *cache.Client
}

func (r *Resources) Shutdown(log *logger.Logger) {
	if r.Cache != nil {
		if err := r.Cache.Close(); err != nil && log != nil {
			log.Warn("Failed to close cache connection", slog.String("error", err.Error()))
		}
	}

	if r.PostgresDB != nil {
		if sqlDB, err := r.PostgresDB.DB(); err == nil {
			if err := sqlDB.Close(); err != nil && log != nil {
				log.Warn("Failed to close PostgreSQL connection", slog.String("error", err.Error()))
			}
		}
	}

	if log != nil {
		log.Info("Infrastructure connections closed")
	}
}

func New(cfg *config.Environment, log *logger.Logger, enableCache bool) (*Resources, error) {
	log.Info("Connecting to PostgreSQL...")
	postgresDB, err := postgres.Connect(&postgres.Config{Url: cfg.Postgres.URL, MaxConns: cfg.Postgres.MaxConns})
	if err != nil {
		return nil, err
	}
	log.Info("PostgreSQL connected", slog.String("host", cfg.Postgres.Host), slog.String("database", cfg.Postgres.Base))

	res := &Resources{PostgresDB: postgresDB}

	if enableCache {
		log.Info("Connecting to cache...", slog.String("provider", cfg.Cache.Backend), slog.String("host", cfg.Cache.Host))
		client, err := cache.Connect(&cfg.Cache)
		if err != nil {
			if sqlDB, dbErr := postgresDB.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
			return nil, fmt.Errorf("cache: %w", err)
		}
		res.Cache = client
		log.Info("Cache connected", slog.String("provider", cfg.Cache.Backend), slog.String("host", cfg.Cache.Host))
	}

	return res, nil
}
