package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/auth/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	grpcserver "github.com/parsascontentcorner/discordliteserver/internal/grpc"
	"github.com/parsascontentcorner/discordliteserver/internal/oauth"
)

// ============================================================================
// Test Setup & Helpers
// ============================================================================

type testOAuthServer struct {
	db              *database.DB
	cleanup         func()
	grpcServer      *grpc.Server
	grpcAddr        string
	httpServer      *httptest.Server
	mockDiscord     *httptest.Server
	discordClient   *auth.DiscordClient
	stateManager    *auth.StateManager
	oauthHandlers   *oauth.Handlers
	authServiceImpl *grpcserver.AuthServer
}

func setupOAuthIntegrationTest(t *testing.T) *testOAuthServer {
	t.Helper()

	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)

	// Create mock Discord OAuth/API server
	mockDiscord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			// Mock token exchange
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "mock_access_token",
				"token_type":    "Bearer",
				"expires_in":    604800,
				"refresh_token": "mock_refresh_token",
				"scope":         "identify email guilds",
			})
		case "/users/@me":
			// Mock user info
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":            "123456789",
				"username":      "testuser",
				"discriminator": "1234",
				"avatar":        "avatar_hash",
				"email":         "test@example.com",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Create config
	cfg := &config.Config{
		Server: config.ServerConfig{
			HTTPPort: "0", // Random port
			GRPCPort: "0", // Random port
		},
		Discord: config.DiscordConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
			RedirectURI:  "http://localhost:8080/auth/callback",
			Scopes:       []string{"identify", "email", "guilds"},
		},
		Security: config.SecurityConfig{
			TokenEncryptionKey: []byte("12345678901234567890123456789012"),
			SessionExpiryHours: 24,
			StateExpiryMinutes: 10,
		},
	}

	logger := zap.NewNop()

	// Create Discord client
	discordClient := auth.NewDiscordClient(cfg, logger)
	// Override Discord API endpoints to use mock
	discordClient.SetBaseURL(mockDiscord.URL)

	// Create state manager
	stateManager := auth.NewStateManager(db, cfg.Security.StateExpiryMinutes)

	// Create OAuth handler
	oauthHandler := auth.NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create OAuth HTTP handlers
	oauthHandlers := oauth.NewHandlers(oauthHandler, logger)

	// Create gRPC server
	authServiceImpl := grpcserver.NewAuthServer(db, discordClient, stateManager, logger, cfg.Security.SessionExpiryHours)
	grpcSrv := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcSrv, authServiceImpl)

	// Start gRPC server on random port
	listener, err := StartTestServer(grpcSrv)
	require.NoError(t, err)
	grpcAddr := listener.Addr().String()

	// Create HTTP server with OAuth callback
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", oauthHandlers.CallbackHandler)
	mux.HandleFunc("/health", oauthHandlers.HealthHandler)
	httpSrv := httptest.NewServer(mux)

	// Update redirect URI to match test server
	cfg.Discord.RedirectURI = httpSrv.URL + "/auth/callback"

	cleanupFunc := func() {
		httpSrv.Close()
		mockDiscord.Close()
		grpcSrv.Stop()
		cleanup()
	}

	return &testOAuthServer{
		db:              db,
		cleanup:         cleanupFunc,
		grpcServer:      grpcSrv,
		grpcAddr:        grpcAddr,
		httpServer:      httpSrv,
		mockDiscord:     mockDiscord,
		discordClient:   discordClient,
		stateManager:    stateManager,
		oauthHandlers:   oauthHandlers,
		authServiceImpl: authServiceImpl,
	}
}

func (ts *testOAuthServer) createGRPCClient(t *testing.T) authv1.AuthServiceClient {
	t.Helper()

	conn, err := grpc.NewClient(
		ts.grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return authv1.NewAuthServiceClient(conn)
}

// ============================================================================
// Integration Tests - Complete OAuth Flow
// ============================================================================

func TestCompleteOAuthFlow_Success(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Step 1: Call InitAuth to get OAuth URL and session
	initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, initResp.SessionId)
	assert.NotEmpty(t, initResp.AuthUrl)
	assert.Contains(t, initResp.AuthUrl, "discord.com/oauth2/authorize")
	assert.Contains(t, initResp.AuthUrl, "state=")

	sessionID := initResp.SessionId

	// Step 2: Verify session is in pending state
	statusResp, err := client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	})
	require.NoError(t, err)
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_PENDING, statusResp.Status)
	assert.Nil(t, statusResp.User)

	// Step 3: Extract state from OAuth URL
	parsedURL, err := url.Parse(initResp.AuthUrl)
	require.NoError(t, err)
	state := parsedURL.Query().Get("state")
	assert.NotEmpty(t, state)

	// Step 4: Simulate OAuth callback (user authenticated with Discord)
	callbackURL := fmt.Sprintf("%s/auth/callback?code=test_auth_code&state=%s", ts.httpServer.URL, state)
	resp, err := http.Get(callbackURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify callback returned success page
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Step 5: Poll GetAuthStatus - should now be authenticated
	var finalStatus *authv1.GetAuthStatusResponse
	for i := 0; i < 10; i++ {
		finalStatus, err = client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
			SessionId: sessionID,
		})
		require.NoError(t, err)

		if finalStatus.Status == authv1.AuthStatus_AUTH_STATUS_AUTHENTICATED {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify authenticated status
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_AUTHENTICATED, finalStatus.Status)
	assert.NotNil(t, finalStatus.User)
	assert.Equal(t, "123456789", finalStatus.User.DiscordId)
	assert.Equal(t, "testuser", finalStatus.User.Username)
	assert.Equal(t, "test@example.com", finalStatus.User.Email)

	// Step 6: Verify user and token stored in database
	user, err := ts.db.GetUserByDiscordID(ctx, "123456789")
	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)

	oauthToken, err := ts.db.GetOAuthToken(ctx, user.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, oauthToken.AccessToken)
	assert.NotEmpty(t, oauthToken.RefreshToken)

	// Step 7: Revoke auth
	_, err = client.RevokeAuth(ctx, &authv1.RevokeAuthRequest{
		SessionId: sessionID,
	})
	require.NoError(t, err)

	// Step 8: Verify session and token are deleted
	_, err = ts.db.GetAuthSession(ctx, sessionID)
	assert.Error(t, err) // Should not exist

	_, err = ts.db.GetOAuthToken(ctx, user.ID)
	assert.Error(t, err) // Should not exist
}

func TestOAuthFlow_InvalidState(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Step 1: Initialize auth
	initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{})
	require.NoError(t, err)
	sessionID := initResp.SessionId

	// Step 2: Try callback with invalid state
	callbackURL := fmt.Sprintf("%s/auth/callback?code=test_code&state=invalid_state_token", ts.httpServer.URL)
	resp, err := http.Get(callbackURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify callback returned error page (400 Bad Request)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Step 3: Verify session is still pending (invalid state prevents update)
	time.Sleep(200 * time.Millisecond) // Give it time to update
	statusResp, err := client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	})
	require.NoError(t, err)
	// Session should still be pending (callback failed so status wasn't updated)
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_PENDING, statusResp.Status)
}

func TestOAuthFlow_ExpiredState(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Step 1: Initialize auth
	initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{})
	require.NoError(t, err)

	// Extract state
	parsedURL, err := url.Parse(initResp.AuthUrl)
	require.NoError(t, err)
	state := parsedURL.Query().Get("state")

	// Step 2: Manually expire the state in database
	// (In production, states expire after 10 minutes)
	_, err = ts.db.ExecContext(ctx, `
		UPDATE oauth_states
		SET expires_at = NOW() - INTERVAL '1 hour'
		WHERE state = $1
	`, state)
	require.NoError(t, err)

	// Step 3: Try callback with expired state
	callbackURL := fmt.Sprintf("%s/auth/callback?code=test_code&state=%s", ts.httpServer.URL, state)
	resp, err := http.Get(callbackURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return error page (400 Bad Request for expired state)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOAuthFlow_MultipleSimultaneousSessions(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Create 3 simultaneous auth sessions
	sessions := make([]string, 3)
	states := make([]string, 3)

	for i := 0; i < 3; i++ {
		initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{})
		require.NoError(t, err)
		sessions[i] = initResp.SessionId

		parsedURL, err := url.Parse(initResp.AuthUrl)
		require.NoError(t, err)
		states[i] = parsedURL.Query().Get("state")
	}

	// Complete OAuth for session 2 only
	callbackURL := fmt.Sprintf("%s/auth/callback?code=test_code&state=%s", ts.httpServer.URL, states[1])
	resp, err := http.Get(callbackURL)
	require.NoError(t, err)
	resp.Body.Close()

	time.Sleep(200 * time.Millisecond)

	// Verify session 2 is authenticated
	statusResp, err := client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
		SessionId: sessions[1],
	})
	require.NoError(t, err)
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_AUTHENTICATED, statusResp.Status)

	// Verify sessions 0 and 2 are still pending
	for _, idx := range []int{0, 2} {
		statusResp, err = client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
			SessionId: sessions[idx],
		})
		require.NoError(t, err)
		assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_PENDING, statusResp.Status)
	}
}

func TestOAuthFlow_SessionExpiry(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Step 1: Create session
	initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{})
	require.NoError(t, err)
	sessionID := initResp.SessionId

	// Step 2: Manually expire the session
	_, err = ts.db.ExecContext(ctx, `
		UPDATE auth_sessions
		SET expires_at = NOW() - INTERVAL '1 hour'
		WHERE session_id = $1
	`, sessionID)
	require.NoError(t, err)

	// Step 3: Try to get status of expired session
	statusResp, err := client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	})
	require.NoError(t, err)
	// Expired sessions return FAILED status with "session has expired" error message
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_FAILED, statusResp.Status)
	assert.NotNil(t, statusResp.ErrorMessage)
	assert.Contains(t, *statusResp.ErrorMessage, "expired")
}

func TestOAuthFlow_RevokeUnauthenticatedSession(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Step 1: Create pending session
	initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{})
	require.NoError(t, err)
	sessionID := initResp.SessionId

	// Step 2: Try to revoke pending session (should succeed but do nothing)
	_, err = client.RevokeAuth(ctx, &authv1.RevokeAuthRequest{
		SessionId: sessionID,
	})
	require.NoError(t, err)

	// Step 3: Verify session is deleted
	_, err = ts.db.GetAuthSession(ctx, sessionID)
	assert.Error(t, err) // Should not exist
}

func TestOAuthFlow_CustomSessionID(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	client := ts.createGRPCClient(t)

	// Step 1: Create auth with custom session ID
	customSessionID := "my-custom-session-id-12345"
	initResp, err := client.InitAuth(ctx, &authv1.InitAuthRequest{
		SessionId: customSessionID,
	})
	require.NoError(t, err)
	assert.Equal(t, customSessionID, initResp.SessionId)

	// Step 2: Verify can query status with custom session ID
	statusResp, err := client.GetAuthStatus(ctx, &authv1.GetAuthStatusRequest{
		SessionId: customSessionID,
	})
	require.NoError(t, err)
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_PENDING, statusResp.Status)
}

func TestOAuthFlow_HealthCheck(t *testing.T) {
	ts := setupOAuthIntegrationTest(t)
	defer ts.cleanup()

	// Test HTTP health endpoint
	resp, err := http.Get(ts.httpServer.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
