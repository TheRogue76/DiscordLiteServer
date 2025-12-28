package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
)

// SetupTestDB creates a PostgreSQL TestContainer, runs migrations, and returns a database connection.
// Returns the DB connection, a cleanup function, and any error encountered.
//
// Usage:
//
//	db, cleanup, err := testutil.SetupTestDB(ctx)
//	require.NoError(t, err)
//	defer cleanup()
func SetupTestDB(ctx context.Context) (*database.DB, func(), error) {
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
	db, err := database.NewDB(cfg, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations - try multiple paths since tests run from different directories
	migrationPaths := []string{
		"internal/database/migrations", // From project root
		"../database/migrations",       // From internal/auth or similar
		"../../database/migrations",    // From nested packages
		"database/migrations",          // From internal/
		"migrations",                   // From internal/database itself
	}

	var migrationErr error
	migrated := false
	for _, path := range migrationPaths {
		if err := db.RunMigrations(path); err == nil {
			migrated = true
			break
		}
		migrationErr = err
	}

	if !migrated {
		return nil, nil, fmt.Errorf("failed to run migrations from any path: %w", migrationErr)
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

// TruncateTables removes all data from all tables (except schema_migrations).
// Useful for cleaning up between tests without recreating the entire database.
func TruncateTables(ctx context.Context, db *database.DB) error {
	tables := []string{
		"auth_sessions",
		"oauth_states",
		"oauth_tokens",
		"users",
	}

	for _, table := range tables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}

	return nil
}

// SeedTestData inserts common test data into the database.
// This is optional and can be used for tests that need pre-populated data.
func SeedTestData(ctx context.Context, db *database.DB) error {
	// Create a test user
	user := GenerateUser("test_discord_id_123")
	if err := db.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to seed test user: %w", err)
	}

	return nil
}
