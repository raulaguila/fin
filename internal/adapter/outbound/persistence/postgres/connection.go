package postgres

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds database configuration
type Config struct {
	Url string
}

// MustConnect establishes a connection or panics
func MustConnect(cfg *Config) *gorm.DB {
	db, err := Connect(cfg)
	if err != nil {
		panic(err)
	}
	return db
}

// Connect establishes a connection to the PostgreSQL database
func Connect(cfg *Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Url), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Silent),
		NowFunc:     time.Now,
		PrepareStmt: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SetMaxOpenConns sets the maximum number of open connections to the database.
	sqlDB.SetMaxOpenConns(50)

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDB.SetMaxIdleConns(50)

	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
