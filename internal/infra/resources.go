package infra

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/raulaguila/authenticity/config"
	"github.com/raulaguila/authenticity/internal/adapter/outbound/persistence/postgres"
	"github.com/raulaguila/authenticity/pkg/logger"
)

type Resources struct {
	PostgresDB *gorm.DB
}

func (r *Resources) Shutdown(log *logger.Logger) {
	if sqlDB, err := r.PostgresDB.DB(); err == nil {
		if err := sqlDB.Close(); err != nil && log != nil {
			log.Warn("Failed to close PostgreSQL connection", slog.String("error", err.Error()))
		}
	}

	log.Info("Databases connection closed")
}

func New(cfg *config.Environment, log *logger.Logger, enableCache bool) (*Resources, error) {
	// Connect to PostgreSQL
	log.Info("Connecting to PostgreSQL...")
	postgresDB, err := postgres.Connect(&postgres.Config{Url: cfg.Postgres.URL})
	if err != nil {
		return nil, err
	}
	log.Info("PostgreSQL connected", slog.String("host", cfg.Postgres.Host), slog.String("database", cfg.Postgres.Base))

	return &Resources{
		PostgresDB: postgresDB,
	}, nil
}
