package grpc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/auth/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
	"github.com/parsascontentcorner/discordliteserver/internal/testutil"
)

func TestInitAuth_AutoGenerateSessionID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	req := &authv1.InitAuthRequest{
		SessionId: "", // Empty - should auto-generate
	}

	resp, err := server.InitAuth(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should generate UUID session ID
	assert.NotEmpty(t, resp.SessionId)
	assert.Len(t, resp.SessionId, 36) // UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

	// Should return auth URL
	assert.NotEmpty(t, resp.AuthUrl)
	assert.Contains(t, resp.AuthUrl, "discord.com/oauth2/authorize")

	// Should return state
	assert.NotEmpty(t, resp.State)

	// Verify session was created in database
	session, err := db.GetAuthSession(ctx, resp.SessionId)
	require.NoError(t, err)
	assert.Equal(t, models.AuthStatusPending, session.AuthStatus)
}

func TestInitAuth_CustomSessionID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	customSessionID := "my-custom-session-id-123"
	req := &authv1.InitAuthRequest{
		SessionId: customSessionID,
	}

	resp, err := server.InitAuth(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should use provided session ID
	assert.Equal(t, customSessionID, resp.SessionId)

	// Verify session was created in database
	session, err := db.GetAuthSession(ctx, customSessionID)
	require.NoError(t, err)
	assert.Equal(t, models.AuthStatusPending, session.AuthStatus)
}

func TestInitAuth_URLFormat(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	req := &authv1.InitAuthRequest{}

	resp, err := server.InitAuth(ctx, req)

	require.NoError(t, err)

	// Verify URL format
	assert.Contains(t, resp.AuthUrl, "discord.com/oauth2/authorize")
	assert.Contains(t, resp.AuthUrl, "client_id=")
	assert.Contains(t, resp.AuthUrl, "redirect_uri=")
	assert.Contains(t, resp.AuthUrl, "response_type=code")
	assert.Contains(t, resp.AuthUrl, "scope=")
	assert.Contains(t, resp.AuthUrl, "state=") // State is URL-encoded in the URL
}

func TestGetAuthStatus_MissingSessionID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	req := &authv1.GetAuthStatusRequest{
		SessionId: "",
	}

	resp, err := server.GetAuthStatus(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	// Verify gRPC error code
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "session_id is required")
}

func TestGetAuthStatus_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	req := &authv1.GetAuthStatusRequest{
		SessionId: "nonexistent-session-id",
	}

	resp, err := server.GetAuthStatus(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGetAuthStatus_PendingStatus(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	// Create pending session
	sessionID := "test-pending-session"
	session := &models.AuthSession{
		SessionID:  sessionID,
		AuthStatus: models.AuthStatusPending,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	req := &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	}

	resp, err := server.GetAuthStatus(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_PENDING, resp.Status)
	assert.Nil(t, resp.User)
	assert.Nil(t, resp.ErrorMessage)
}

func TestGetAuthStatus_AuthenticatedWithUser(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	// Create user
	user := testutil.GenerateUser("test_discord_id_789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Get user ID
	createdUser, err := db.GetUserByDiscordID(ctx, user.DiscordID)
	require.NoError(t, err)

	// Create authenticated session
	sessionID := "test-authenticated-session"
	session := &models.AuthSession{
		SessionID:  sessionID,
		UserID:     sql.NullInt64{Int64: createdUser.ID, Valid: true},
		AuthStatus: models.AuthStatusAuthenticated,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	req := &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	}

	resp, err := server.GetAuthStatus(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_AUTHENTICATED, resp.Status)
	require.NotNil(t, resp.User)
	assert.Equal(t, user.DiscordID, resp.User.DiscordId)
	assert.Equal(t, user.Username, resp.User.Username)
	assert.Nil(t, resp.ErrorMessage)
}

func TestGetAuthStatus_FailedWithError(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	// Create failed session
	sessionID := "test-failed-session"
	session := &models.AuthSession{
		SessionID:    sessionID,
		AuthStatus:   models.AuthStatusFailed,
		ErrorMessage: sql.NullString{String: "invalid OAuth code", Valid: true},
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	req := &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	}

	resp, err := server.GetAuthStatus(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_FAILED, resp.Status)
	assert.Nil(t, resp.User)
	require.NotNil(t, resp.ErrorMessage)
	assert.Equal(t, "invalid OAuth code", *resp.ErrorMessage)
}

func TestGetAuthStatus_ExpiredSession(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	// Create expired session
	sessionID := "test-expired-session"
	session := &models.AuthSession{
		SessionID:  sessionID,
		AuthStatus: models.AuthStatusPending,
		ExpiresAt:  time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	req := &authv1.GetAuthStatusRequest{
		SessionId: sessionID,
	}

	resp, err := server.GetAuthStatus(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should return FAILED status for expired session
	assert.Equal(t, authv1.AuthStatus_AUTH_STATUS_FAILED, resp.Status)
	require.NotNil(t, resp.ErrorMessage)
	assert.Contains(t, *resp.ErrorMessage, "session has expired")
}

func TestRevokeAuth_MissingSessionID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	req := &authv1.RevokeAuthRequest{
		SessionId: "",
	}

	resp, err := server.RevokeAuth(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "session_id is required")
}

func TestRevokeAuth_SessionNotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	req := &authv1.RevokeAuthRequest{
		SessionId: "nonexistent-session",
	}

	resp, err := server.RevokeAuth(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestRevokeAuth_PendingSession(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	// Create pending session (no user ID, no tokens)
	sessionID := "test-pending-revoke"
	session := &models.AuthSession{
		SessionID:  sessionID,
		AuthStatus: models.AuthStatusPending,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	req := &authv1.RevokeAuthRequest{
		SessionId: sessionID,
	}

	resp, err := server.RevokeAuth(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.True(t, resp.Success)
	assert.Equal(t, "authentication revoked successfully", resp.Message)

	// Verify session was deleted
	_, err = db.GetAuthSession(ctx, sessionID)
	assert.Error(t, err)
}

func TestRevokeAuth_AuthenticatedSessionWithToken(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	server := NewAuthServer(db, discordClient, stateManager, logger, 24)

	// Create user
	user := testutil.GenerateUser("test_discord_revoke")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	createdUser, err := db.GetUserByDiscordID(ctx, user.DiscordID)
	require.NoError(t, err)

	// Store OAuth token
	token := testutil.GenerateOAuthToken(createdUser.ID)
	err = db.StoreOAuthToken(ctx, token)
	require.NoError(t, err)

	// Create authenticated session
	sessionID := "test-authenticated-revoke"
	session := &models.AuthSession{
		SessionID:  sessionID,
		UserID:     sql.NullInt64{Int64: createdUser.ID, Valid: true},
		AuthStatus: models.AuthStatusAuthenticated,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	req := &authv1.RevokeAuthRequest{
		SessionId: sessionID,
	}

	resp, err := server.RevokeAuth(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.True(t, resp.Success)

	// Verify session was deleted
	_, err = db.GetAuthSession(ctx, sessionID)
	assert.Error(t, err)

	// Verify token was deleted
	_, err = db.GetOAuthToken(ctx, createdUser.ID)
	assert.Error(t, err)
}

func TestAuthServer_SessionExpiryConfiguration(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)

	// Create server with custom expiry
	customExpiryHours := 48
	server := NewAuthServer(db, discordClient, stateManager, logger, customExpiryHours)

	req := &authv1.InitAuthRequest{}

	resp, err := server.InitAuth(ctx, req)
	require.NoError(t, err)

	// Verify session expiry
	session, err := db.GetAuthSession(ctx, resp.SessionId)
	require.NoError(t, err)

	expectedExpiry := time.Now().Add(time.Duration(customExpiryHours) * time.Hour)
	timeDiff := session.ExpiresAt.Sub(expectedExpiry)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	assert.Less(t, timeDiff, 5*time.Second, "Expiry should be ~48 hours from now")
}
