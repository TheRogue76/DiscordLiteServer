package database

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
)

// setupTestDB creates a PostgreSQL TestContainer for database package tests.
// This is separate from testutil to avoid import cycles.
func setupTestDB(ctx context.Context) (*DB, func(), error) {
	// Create PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get connection details
	host, err := pgContainer.Host(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	// Create logger for database
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create database config
	cfg := &config.DatabaseConfig{
		Host:         host,
		Port:         mappedPort.Port(),
		User:         "testuser",
		Password:     "testpass",
		Name:         "testdb",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	// Connect to database
	db, err := NewDB(cfg, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations from the migrations directory (relative to package)
	if err := db.RunMigrations("migrations"); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Cleanup function
	cleanup := func() {
		if db != nil {
			err := db.Close()
			if err != nil {
				logger.Error("failed to close db", zap.Error(err))
			}
		}
		if pgContainer != nil {
			if err := pgContainer.Terminate(ctx); err != nil {
				logger.Error("failed to terminate container", zap.Error(err))
			}
		}
	}

	return db, cleanup, nil
}
