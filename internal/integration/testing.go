package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/auth/v1"
	channelv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/channel/v1"
	messagev1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/message/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	grpcserver "github.com/parsascontentcorner/discordliteserver/internal/grpc"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
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

// mockWebSocketManager is a mock implementation of WebSocketManager for testing
type mockWebSocketManager struct{}

func (m *mockWebSocketManager) IsEnabled() bool {
	return false
}

func (m *mockWebSocketManager) Subscribe(ctx context.Context, userID int64, channelIDs []string) (<-chan *messagev1.MessageEvent, error) {
	return nil, fmt.Errorf("WebSocket is disabled in tests")
}

func (m *mockWebSocketManager) Unsubscribe(userID int64, channelIDs []string) {}

// TestSuite is the test suite for Phase 2 integration tests
type TestSuite struct {
	db             *database.DB
	discordClient  *auth.DiscordClient
	cleanup        func()
	grpcServer     *grpc.Server
	grpcAddr       string
	mockDiscordAPI *httptest.Server
	authClient     authv1.AuthServiceClient
	channelClient  channelv1.ChannelServiceClient
	messageClient  messagev1.MessageServiceClient
}

// setupTestSuite creates a complete test environment for Phase 2 integration tests
func setupTestSuite(t *testing.T) *TestSuite {
	t.Helper()

	ctx := context.Background()

	// Setup database
	db, dbCleanup, err := setupTestDB(ctx)
	require.NoError(t, err)

	// Create mock Discord API
	mockDiscord := httptest.NewServer(nil)

	// Create config
	cfg := &config.Config{
		Discord: config.DiscordConfig{
			ClientID:     "test_client",
			ClientSecret: "test_secret",
			RedirectURI:  mockDiscord.URL + "/callback",
			Scopes:       []string{"identify", "guilds", "messages.read"},
		},
		Security: config.SecurityConfig{
			TokenEncryptionKey: []byte("12345678901234567890123456789012"),
			SessionExpiryHours: 24,
			StateExpiryMinutes: 10,
		},
		Cache: config.CacheConfig{
			GuildTTLHours:     1,
			ChannelTTLMinutes: 30,
			MessageTTLMinutes: 5,
		},
		WebSocket: config.WebSocketConfig{
			Enabled:               false, // Disabled for integration tests
			MaxConnectionsPerUser: 5,
		},
	}

	logger := zap.NewNop()

	// Create Discord client
	discordClient := auth.NewDiscordClient(cfg, logger)
	discordClient.SetBaseURL(mockDiscord.URL)

	// Create state manager
	stateManager := auth.NewStateManager(db, cfg.Security.StateExpiryMinutes)

	// Create cache manager
	cacheManager := grpcserver.NewCacheManager(db, logger)

	// Create auth service
	authService := grpcserver.NewAuthServer(db, discordClient, stateManager, logger, cfg.Security.SessionExpiryHours)

	// Create channel service
	channelService := grpcserver.NewChannelServer(db, discordClient, logger, cacheManager)

	// Create message service with mock WebSocket manager
	mockWSManager := &mockWebSocketManager{}
	messageService := grpcserver.NewMessageServer(db, discordClient, logger, cacheManager, mockWSManager)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcServer, authService)
	channelv1.RegisterChannelServiceServer(grpcServer, channelService)
	messagev1.RegisterMessageServiceServer(grpcServer, messageService)

	// Start gRPC server
	listener, err := StartTestServer(grpcServer)
	require.NoError(t, err)

	// Create gRPC clients
	grpcConn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	authClient := authv1.NewAuthServiceClient(grpcConn)
	channelClient := channelv1.NewChannelServiceClient(grpcConn)
	messageClient := messagev1.NewMessageServiceClient(grpcConn)

	// Cleanup function
	cleanup := func() {
		grpcConn.Close()
		grpcServer.Stop()
		mockDiscord.Close()
		dbCleanup()
	}

	return &TestSuite{
		db:             db,
		discordClient:  discordClient,
		cleanup:        cleanup,
		grpcServer:     grpcServer,
		grpcAddr:       listener.Addr().String(),
		mockDiscordAPI: mockDiscord,
		authClient:     authClient,
		channelClient:  channelClient,
		messageClient:  messageClient,
	}
}

// authenticateUser performs Phase 1 OAuth authentication and returns a session ID
func (ts *TestSuite) authenticateUser(ctx context.Context, t *testing.T) string {
	t.Helper()

	// Create user in database
	user := &models.User{
		DiscordID:     "discord123",
		Username:      "testuser",
		Discriminator: sql.NullString{String: "1234", Valid: true},
		Email:         sql.NullString{String: "test@example.com", Valid: true},
	}
	err := ts.db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Encrypt test tokens
	encryptedAccessToken, err := ts.discordClient.EncryptToken("test_access_token")
	require.NoError(t, err)

	encryptedRefreshToken, err := ts.discordClient.EncryptToken("test_refresh_token")
	require.NoError(t, err)

	// Store OAuth token
	oauthToken := &models.OAuthToken{
		UserID:       user.ID,
		AccessToken:  encryptedAccessToken,
		RefreshToken: encryptedRefreshToken,
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(24 * time.Hour),
		Scope:        "identify guilds messages.read",
	}
	err = ts.db.StoreOAuthToken(ctx, oauthToken)
	require.NoError(t, err)

	// Create authenticated session
	sessionID := "test_session_" + fmt.Sprintf("%d", time.Now().Unix())
	session := &models.AuthSession{
		SessionID:  sessionID,
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		AuthStatus: models.AuthStatusAuthenticated,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	err = ts.db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	return sessionID
}
