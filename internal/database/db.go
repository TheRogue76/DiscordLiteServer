package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
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

// RunMigrations runs database migrations from the migrations folder
func (db *DB) RunMigrations(migrationPath string) error {
	db.logger.Info("running database migrations", zap.String("path", migrationPath))

	// Read migration file
	content, err := readMigrationFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Execute migration
	if _, err := db.Exec(content); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	db.logger.Info("database migrations completed successfully")
	return nil
}

// readMigrationFile reads a migration file
// This is a simple implementation; in production, consider using a migration library
func readMigrationFile(path string) (string, error) {
	// This is a placeholder - in the actual implementation,
	// we'll read from the file system or use embedded files
	// For now, this will be called from main.go after reading the file
	return "", fmt.Errorf("not implemented: use migration script")
}
