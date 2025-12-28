// Package database provides PostgreSQL database connection and migration management.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // File source driver for migrations
	_ "github.com/lib/pq"                                // PostgreSQL driver
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
)

// DB wraps the database connection
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// NewDB creates a new database connection with connection pooling
func NewDB(cfg *config.DatabaseConfig, logger *zap.Logger) (*DB, error) {
	// Build connection string
	dsn := cfg.GetDSN()

	// Open database connection
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Verify connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established",
		zap.String("host", cfg.Host),
		zap.String("port", cfg.Port),
		zap.String("database", cfg.Name),
	)

	return &DB{
		DB:     sqlDB,
		logger: logger,
	}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	db.logger.Info("closing database connection")
	return db.DB.Close()
}

// Health checks the database health
func (db *DB) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

// RunMigrations runs database migrations using golang-migrate library
func (db *DB) RunMigrations(migrationsPath string) error {
	db.logger.Info("running database migrations with golang-migrate", zap.String("path", migrationsPath))

	// Create postgres driver instance from existing connection
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver instance: %w", err)
	}

	// Create migrate instance with file source
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	// Run all pending migrations
	if err := m.Up(); err != nil {
		// ErrNoChange is not an error - it means we're already up to date
		if errors.Is(err, migrate.ErrNoChange) {
			db.logger.Info("database schema is already up to date")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		db.logger.Warn("failed to get migration version", zap.Error(err))
	} else if errors.Is(err, migrate.ErrNilVersion) {
		db.logger.Info("no migrations have been applied yet")
	} else {
		db.logger.Info("database migrations completed successfully",
			zap.Uint("version", version),
			zap.Bool("dirty", dirty),
		)
	}

	return nil
}
