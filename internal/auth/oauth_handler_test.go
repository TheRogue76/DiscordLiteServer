package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
	"github.com/parsascontentcorner/discordliteserver/internal/testutil"
)

func TestHandleCallback_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup mock Discord server
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	// Create Discord client and OAuth handler
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Handle callback with valid code
	err = handler.HandleCallback(ctx, "valid_code", state)

	// Verify success
	require.NoError(t, err)

	// Verify session status updated to authenticated
	retrievedSession, err := db.GetAuthSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.AuthStatusAuthenticated, retrievedSession.AuthStatus)
	assert.NotNil(t, retrievedSession.UserID)
	assert.Nil(t, retrievedSession.ErrorMessage)

	// Verify user was created
	user, err := db.GetUserByDiscordID(ctx, "123456789012345678")
	require.NoError(t, err)
	assert.Equal(t, "TestUser", user.Username)
	assert.Equal(t, "testuser@example.com", user.Email.String)

	// Verify OAuth token was stored (encrypted)
	token, err := db.GetOAuthToken(ctx, user.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, token.AccessToken)
	assert.NotEmpty(t, token.RefreshToken)
	assert.Equal(t, "Bearer", token.TokenType)

	// Verify token is encrypted (not plaintext)
	assert.NotEqual(t, "mock_access_token_123", token.AccessToken)

	// Verify state was deleted (single-use)
	_, err = stateManager.ValidateState(ctx, state)
	assert.Error(t, err)

	// Verify mock server was called
	assert.Equal(t, 1, mockServer.TokenCalls)
	assert.Equal(t, 1, mockServer.UserInfoCalls)
}

func TestHandleCallback_InvalidState_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Try to handle callback with non-existent state
	err = handler.HandleCallback(ctx, "valid_code", "nonexistent_state_12345")

	// Should fail with invalid state error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")

	// Verify no users were created
	_, err = db.GetUserByDiscordID(ctx, "123456789012345678")
	assert.Error(t, err)

	// Verify mock server was NOT called
	assert.Equal(t, 0, mockServer.TokenCalls)
	assert.Equal(t, 0, mockServer.UserInfoCalls)
}

func TestHandleCallback_InvalidState_Expired(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Create expired OAuth state
	expiredState := testutil.GenerateOAuthState(sessionID)
	expiredState.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute) // Already expired
	err = db.CreateOAuthState(ctx, expiredState)
	require.NoError(t, err)

	// Try to handle callback with expired state
	err = handler.HandleCallback(ctx, "valid_code", expiredState.State)

	// Should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")

	// Verify no users were created
	_, err = db.GetUserByDiscordID(ctx, "123456789012345678")
	assert.Error(t, err)

	// Verify mock server was NOT called
	assert.Equal(t, 0, mockServer.TokenCalls)
}

func TestHandleCallback_ExchangeCodeFailure(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Handle callback with code that triggers error in mock server
	err = handler.HandleCallback(ctx, "error_code", state)

	// Should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to exchange authorization code")

	// Verify session status updated to failed
	retrievedSession, err := db.GetAuthSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.AuthStatusFailed, retrievedSession.AuthStatus)
	assert.True(t, retrievedSession.ErrorMessage.Valid)
	assert.Contains(t, retrievedSession.ErrorMessage.String, "failed to exchange authorization code")

	// Verify no users were created
	_, err = db.GetUserByDiscordID(ctx, "123456789012345678")
	assert.Error(t, err)

	// Verify token exchange was attempted
	assert.Equal(t, 1, mockServer.TokenCalls)
	// User info should NOT be called
	assert.Equal(t, 0, mockServer.UserInfoCalls)
}

func TestHandleCallback_ServerError(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Handle callback with code that triggers server error
	err = handler.HandleCallback(ctx, "server_error", state)

	// Should fail
	assert.Error(t, err)

	// Verify session status updated to failed
	retrievedSession, err := db.GetAuthSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.AuthStatusFailed, retrievedSession.AuthStatus)
}

func TestHandleCallback_GetUserInfoFailure_Unauthorized(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Handle callback - mock server will return valid token but GetUserInfo will fail
	// We need to use a special code that returns an access token that triggers 401
	// Looking at mock_discord.go, "invalid_token" triggers 401 in GetUserInfo
	err = handler.HandleCallback(ctx, "invalid_token_code", state)

	// Should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch user information")

	// Verify session status updated to failed
	retrievedSession, err := db.GetAuthSession(ctx, sessionID)
	require.NoError(t, err)
	assert.Equal(t, models.AuthStatusFailed, retrievedSession.AuthStatus)
	assert.True(t, retrievedSession.ErrorMessage.Valid)
	assert.Contains(t, retrievedSession.ErrorMessage.String, "failed to fetch user information")

	// Verify both token exchange and user info were attempted
	assert.Equal(t, 1, mockServer.TokenCalls)
	assert.Equal(t, 1, mockServer.UserInfoCalls)

	// Verify no users were created
	_, err = db.GetUserByDiscordID(ctx, "123456789012345678")
	assert.Error(t, err)
}

func TestHandleCallback_UserCreationSuccess_WithNullFields(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Handle callback
	err = handler.HandleCallback(ctx, "valid_code", state)
	require.NoError(t, err)

	// Verify user was created with nullable fields handled correctly
	user, err := db.GetUserByDiscordID(ctx, "123456789012345678")
	require.NoError(t, err)
	assert.Equal(t, "TestUser", user.Username)
	assert.True(t, user.Discriminator.Valid)
	assert.True(t, user.Avatar.Valid)
	assert.True(t, user.Email.Valid)
}

func TestHandleCallback_TokenEncryptionAndStorage(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Create auth session
	sessionID := testutil.GenerateSessionID()
	session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Handle callback
	err = handler.HandleCallback(ctx, "valid_code", state)
	require.NoError(t, err)

	// Get user
	user, err := db.GetUserByDiscordID(ctx, "123456789012345678")
	require.NoError(t, err)

	// Verify OAuth token was stored
	token, err := db.GetOAuthToken(ctx, user.ID)
	require.NoError(t, err)

	// Verify tokens are encrypted (not plaintext from mock)
	assert.NotEqual(t, "mock_access_token_123", token.AccessToken)
	assert.NotEqual(t, "mock_refresh_token_456", token.RefreshToken)

	// Verify token can be decrypted
	decryptedAccess, err := discordClient.DecryptToken(token.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "mock_access_token_123", decryptedAccess)

	decryptedRefresh, err := discordClient.DecryptToken(token.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "mock_refresh_token_456", decryptedRefresh)

	// Verify token metadata
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Contains(t, token.Scope, "identify")
}

func TestHandleCallback_SessionStatusUpdates(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		wantStatus     string
		wantError      bool
		wantErrorInMsg bool
	}{
		{
			name:           "success updates to authenticated",
			code:           "valid_code",
			wantStatus:     models.AuthStatusAuthenticated,
			wantError:      false,
			wantErrorInMsg: false,
		},
		{
			name:           "token exchange failure updates to failed",
			code:           "error_code",
			wantStatus:     models.AuthStatusFailed,
			wantError:      true,
			wantErrorInMsg: true,
		},
		{
			name:           "server error updates to failed",
			code:           "server_error",
			wantStatus:     models.AuthStatusFailed,
			wantError:      true,
			wantErrorInMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db, cleanup, err := testutil.SetupTestDB(ctx)
			require.NoError(t, err)
			defer cleanup()

			mockServer := testutil.NewMockDiscordServer()
			defer mockServer.Close()

			cfg := testutil.GenerateTestConfig()
			logger, _ := zap.NewDevelopment()
			discordClient := NewDiscordClient(cfg, logger)
			discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

			stateManager := NewStateManager(db, 10)
			handler := NewOAuthHandler(db, discordClient, stateManager, logger)

			// Create auth session
			sessionID := testutil.GenerateSessionID()
			session := testutil.GenerateAuthSession(sessionID, models.AuthStatusPending)
			err = db.CreateAuthSession(ctx, session)
			require.NoError(t, err)

			// Store OAuth state
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
			err = stateManager.StoreState(ctx, state, sessionID)
			require.NoError(t, err)

			// Handle callback
			err = handler.HandleCallback(ctx, tt.code, state)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify session status
			retrievedSession, err := db.GetAuthSession(ctx, sessionID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, retrievedSession.AuthStatus)

			// Verify error message presence
			if tt.wantErrorInMsg {
				assert.True(t, retrievedSession.ErrorMessage.Valid)
				assert.NotEmpty(t, retrievedSession.ErrorMessage.String)
			} else {
				assert.False(t, retrievedSession.ErrorMessage.Valid)
			}
		})
	}
}

func TestHandleCallback_UserUpsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)
	discordClient.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// First authentication - creates user
	sessionID1 := testutil.GenerateSessionID()
	session1 := testutil.GenerateAuthSession(sessionID1, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session1)
	require.NoError(t, err)

	state1, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state1, sessionID1)
	require.NoError(t, err)

	err = handler.HandleCallback(ctx, "valid_code", state1)
	require.NoError(t, err)

	// Get created user
	user1, err := db.GetUserByDiscordID(ctx, "123456789012345678")
	require.NoError(t, err)
	originalID := user1.ID
	originalCreatedAt := user1.CreatedAt

	time.Sleep(50 * time.Millisecond)

	// Second authentication - updates same user
	sessionID2 := testutil.GenerateSessionID()
	session2 := testutil.GenerateAuthSession(sessionID2, models.AuthStatusPending)
	err = db.CreateAuthSession(ctx, session2)
	require.NoError(t, err)

	state2, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state2, sessionID2)
	require.NoError(t, err)

	err = handler.HandleCallback(ctx, "valid_code", state2)
	require.NoError(t, err)

	// Get updated user
	user2, err := db.GetUserByDiscordID(ctx, "123456789012345678")
	require.NoError(t, err)

	// Verify user ID didn't change (upsert, not duplicate)
	assert.Equal(t, originalID, user2.ID)

	// Verify created_at didn't change
	assert.WithinDuration(t, originalCreatedAt, user2.CreatedAt, 1*time.Second)

	// Verify updated_at changed
	assert.True(t, user2.UpdatedAt.After(user2.CreatedAt))
}

func TestHandleCallback_EmptySessionID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	discordClient := NewDiscordClient(cfg, logger)

	stateManager := NewStateManager(db, 10)
	handler := NewOAuthHandler(db, discordClient, stateManager, logger)

	// Try to handle callback with completely invalid state (no session)
	err = handler.HandleCallback(ctx, "valid_code", "invalid_state_no_session")

	// Should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
}
