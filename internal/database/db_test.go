package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
)

// setupPostgresContainer starts a PostgreSQL container for testing
func setupPostgresContainer(ctx context.Context) (testcontainers.Container, *config.DatabaseConfig, error) {
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
		return nil, nil, err
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		return nil, nil, err
	}

	mappedPort, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, err
	}

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

	return pgContainer, cfg, nil
}

func TestNewDB_Success(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)

	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify connection is active
	err = db.PingContext(ctx)
	assert.NoError(t, err)

	// Verify connection pool settings
	stats := db.Stats()
	assert.Equal(t, 5, stats.MaxOpenConnections)
}

func TestNewDB_InvalidCredentials(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Use wrong password
	cfg.Password = "wrong_password"

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)

	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestNewDB_InvalidHost(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:         "nonexistent-host-12345",
		Port:         "5432",
		User:         "testuser",
		Password:     "testpass",
		Name:         "testdb",
		SSLMode:      "disable",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)

	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "failed to ping database")
}

func TestDBHealth_Healthy(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	// Health check should pass
	err = db.Health(ctx)
	assert.NoError(t, err)
}

func TestDBHealth_ClosedConnection(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)

	// Close the connection
	err = db.Close()
	require.NoError(t, err)

	// Health check should fail on closed connection
	err = db.Health(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database health check failed")
}

func TestDBClose(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)

	// Verify connection is active before close
	err = db.PingContext(ctx)
	assert.NoError(t, err)

	// Close the connection
	err = db.Close()
	assert.NoError(t, err)

	// Verify connection is closed - ping should fail
	err = db.PingContext(ctx)
	assert.Error(t, err)
}

func TestRunMigrations_Success(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	// Run migrations - try multiple paths
	migrationPaths := []string{
		"migrations",
		"../database/migrations",
		"internal/database/migrations",
	}

	var migrationErr error
	migrated := false
	for _, path := range migrationPaths {
		if err := db.RunMigrations(path); err == nil {
			migrated = true
			break
		} else {
			migrationErr = err
		}
	}

	require.True(t, migrated, "Failed to run migrations from any path: %v", migrationErr)

	// Verify tables were created
	var tableCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name IN ('users', 'oauth_tokens', 'auth_sessions', 'oauth_states')
	`).Scan(&tableCount)

	require.NoError(t, err)
	assert.Equal(t, 4, tableCount, "All 4 tables should be created")

	// Verify migration tracking table exists
	var migrationTableExists bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = 'schema_migrations'
		)
	`).Scan(&migrationTableExists)

	require.NoError(t, err)
	assert.True(t, migrationTableExists, "Migration tracking table should exist")
}

func TestRunMigrations_Idempotent(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	// Run migrations first time - try multiple paths
	migrationPaths := []string{
		"migrations",
		"../database/migrations",
		"internal/database/migrations",
	}

	var successfulPath string
	for _, path := range migrationPaths {
		if err := db.RunMigrations(path); err == nil {
			successfulPath = path
			break
		}
	}
	require.NotEmpty(t, successfulPath, "Failed to run migrations on first attempt")

	// Run migrations again with same path - should be idempotent (no error)
	err = db.RunMigrations(successfulPath)
	assert.NoError(t, err, "Running migrations twice should not error (ErrNoChange is handled)")

	// Verify table count didn't change
	var tableCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name IN ('users', 'oauth_tokens', 'auth_sessions', 'oauth_states')
	`).Scan(&tableCount)

	require.NoError(t, err)
	assert.Equal(t, 4, tableCount, "Table count should remain 4")
}

func TestRunMigrations_InvalidPath(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	// Try to run migrations with invalid path
	err = db.RunMigrations("/nonexistent/path/to/migrations")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create migrate instance")
}

func TestDatabaseConnectionPool(t *testing.T) {
	ctx := context.Background()

	pgContainer, cfg, err := setupPostgresContainer(ctx)
	require.NoError(t, err)
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Set specific connection pool values
	cfg.MaxOpenConns = 10
	cfg.MaxIdleConns = 5

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	db, err := NewDB(cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	// Verify connection pool settings
	stats := db.Stats()
	assert.Equal(t, 10, stats.MaxOpenConnections)

	// Open some connections by querying
	for i := 0; i < 3; i++ {
		var result int
		err := db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, result)
	}

	// Verify connections were opened
	stats = db.Stats()
	assert.True(t, stats.OpenConnections > 0, "Should have open connections")
	assert.True(t, stats.InUse >= 0, "InUse should be >= 0")
}
