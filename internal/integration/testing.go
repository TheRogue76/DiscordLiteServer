package integration

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
)

// setupTestDB creates a PostgreSQL TestContainer for integration tests.
func setupTestDB(ctx context.Context) (*database.DB, func(), error) {
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
	logger := zap.NewNop() // Use nop logger for tests

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

	// Run migrations from the migrations directory
	// Try multiple paths relative to integration package
	migrationPaths := []string{
		"../database/migrations",
		"internal/database/migrations",
		"../../internal/database/migrations",
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
		return nil, nil, fmt.Errorf("failed to run migrations: %w", migrationErr)
	}

	// Cleanup function
	cleanup := func() {
		if db != nil {
			_ = db.Close()
		}
		if pgContainer != nil {
			_ = pgContainer.Terminate(ctx)
		}
	}

	return db, cleanup, nil
}

// StartTestServer starts a gRPC server on a random port for testing.
// Returns the listener so the test can get the address.
func StartTestServer(srv *grpc.Server) (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		if err := srv.Serve(listener); err != nil {
			// Server stopped or error - ignore in test
		}
	}()

	return listener, nil
}
