package database

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/google/uuid"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Test Helper Functions
// ============================================================================

func generateSessionID() string {
	return uuid.New().String()
}

func generateUser(discordID string) *models.User {
	return &models.User{
		DiscordID:     discordID,
		Username:      "testuser_" + discordID,
		Discriminator: sql.NullString{String: "1234", Valid: true},
		Avatar:        sql.NullString{String: "avatar_hash", Valid: true},
		Email:         sql.NullString{String: "test@example.com", Valid: true},
	}
}

func generateOAuthToken(userID int64) *models.OAuthToken {
	return &models.OAuthToken{
		UserID:       userID,
		AccessToken:  "encrypted_access_token_" + time.Now().Format("20060102150405"),
		RefreshToken: "encrypted_refresh_token_" + time.Now().Format("20060102150405"),
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(7 * 24 * time.Hour),
		Scope:        "identify email guilds",
	}
}

func generateAuthSession(sessionID, status string) *models.AuthSession {
	return &models.AuthSession{
		SessionID:    sessionID,
		UserID:       sql.NullInt64{Valid: false},
		AuthStatus:   status,
		ErrorMessage: sql.NullString{Valid: false},
		ExpiresAt:    time.Now().UTC().Add(24 * time.Hour),
	}
}

func generateOAuthState(sessionID string) *models.OAuthState {
	return &models.OAuthState{
		State:     uuid.New().String(),
		SessionID: sessionID,
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}
}

func assertUserEqual(t *testing.T, expected, actual *models.User) {
	t.Helper()
	assert.Equal(t, expected.DiscordID, actual.DiscordID)
	assert.Equal(t, expected.Username, actual.Username)
	assert.Equal(t, expected.Discriminator, actual.Discriminator)
	assert.Equal(t, expected.Avatar, actual.Avatar)
	assert.Equal(t, expected.Email, actual.Email)
}

func assertTokenEqual(t *testing.T, expected, actual *models.OAuthToken) {
	t.Helper()
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.AccessToken, actual.AccessToken)
	assert.Equal(t, expected.RefreshToken, actual.RefreshToken)
	assert.Equal(t, expected.TokenType, actual.TokenType)
	assert.Equal(t, expected.Scope, actual.Scope)
	assert.WithinDuration(t, expected.Expiry, actual.Expiry, 2*time.Second)
}

func assertSessionEqual(t *testing.T, expected, actual *models.AuthSession) {
	t.Helper()
	assert.Equal(t, expected.SessionID, actual.SessionID)
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.AuthStatus, actual.AuthStatus)
	assert.Equal(t, expected.ErrorMessage, actual.ErrorMessage)
	assert.WithinDuration(t, expected.ExpiresAt, actual.ExpiresAt, 2*time.Second)
}

func assertStateEqual(t *testing.T, expected, actual *models.OAuthState) {
	t.Helper()
	assert.Equal(t, expected.State, actual.State)
	assert.Equal(t, expected.SessionID, actual.SessionID)
	assert.WithinDuration(t, expected.ExpiresAt, actual.ExpiresAt, 2*time.Second)
}

// ============================================================================
// User Tests
// ============================================================================

func TestCreateUser_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	user := generateUser("123456789")

	err = db.CreateUser(ctx, user)

	require.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.NotZero(t, user.CreatedAt)
	assert.NotZero(t, user.UpdatedAt)
	assert.WithinDuration(t, time.Now(), user.CreatedAt, 2*time.Second)
	assert.WithinDuration(t, time.Now(), user.UpdatedAt, 2*time.Second)
}

func TestCreateUser_Upsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create initial user
	user1 := generateUser("123456789")
	user1.Username = "OriginalName"
	err = db.CreateUser(ctx, user1)
	require.NoError(t, err)

	originalID := user1.ID
	originalCreatedAt := user1.CreatedAt

	// Wait a bit to ensure updated_at differs
	time.Sleep(10 * time.Millisecond)

	// Upsert with same discord_id but different data
	user2 := generateUser("123456789")
	user2.Username = "UpdatedName"
	err = db.CreateUser(ctx, user2)
	require.NoError(t, err)

	// ID should remain the same (upsert, not duplicate)
	assert.Equal(t, originalID, user2.ID)

	// Created_at should not change
	assert.WithinDuration(t, originalCreatedAt, user2.CreatedAt, 1*time.Second)

	// Updated_at should be newer
	assert.True(t, user2.UpdatedAt.After(user2.CreatedAt))

	// Verify updated data in database
	retrieved, err := db.GetUserByDiscordID(ctx, "123456789")
	require.NoError(t, err)
	assert.Equal(t, "UpdatedName", retrieved.Username)
}

func TestGetUserByDiscordID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Retrieve user
	retrieved, err := db.GetUserByDiscordID(ctx, "123456789")

	require.NoError(t, err)
	assertUserEqual(t, user, retrieved)
}

func TestGetUserByDiscordID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	user, err := db.GetUserByDiscordID(ctx, "nonexistent_id")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "user not found")
}

func TestGetUserByID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Retrieve by database ID
	retrieved, err := db.GetUserByID(ctx, user.ID)

	require.NoError(t, err)
	assertUserEqual(t, user, retrieved)
}

func TestGetUserByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	user, err := db.GetUserByID(ctx, 99999)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "user not found")
}

// ============================================================================
// OAuth Token Tests
// ============================================================================

func TestStoreOAuthToken_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user first
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Store token
	token := generateOAuthToken(user.ID)
	err = db.StoreOAuthToken(ctx, token)

	require.NoError(t, err)
	assert.NotZero(t, token.ID)
	assert.NotZero(t, token.CreatedAt)
	assert.NotZero(t, token.UpdatedAt)
}

func TestStoreOAuthToken_Upsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Store initial token
	token1 := generateOAuthToken(user.ID)
	token1.AccessToken = "original_access_token"
	err = db.StoreOAuthToken(ctx, token1)
	require.NoError(t, err)

	originalID := token1.ID
	originalCreatedAt := token1.CreatedAt

	time.Sleep(10 * time.Millisecond)

	// Upsert with same user_id but different token
	token2 := generateOAuthToken(user.ID)
	token2.AccessToken = "updated_access_token"
	err = db.StoreOAuthToken(ctx, token2)
	require.NoError(t, err)

	// ID should remain the same
	assert.Equal(t, originalID, token2.ID)

	// Created_at should not change
	assert.WithinDuration(t, originalCreatedAt, token2.CreatedAt, 1*time.Second)

	// Updated_at should be newer
	assert.True(t, token2.UpdatedAt.After(token2.CreatedAt))

	// Verify updated token in database
	retrieved, err := db.GetOAuthToken(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated_access_token", retrieved.AccessToken)
}

func TestGetOAuthToken_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and token
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	token := generateOAuthToken(user.ID)
	err = db.StoreOAuthToken(ctx, token)
	require.NoError(t, err)

	// Retrieve token
	retrieved, err := db.GetOAuthToken(ctx, user.ID)

	require.NoError(t, err)
	assertTokenEqual(t, token, retrieved)
}

func TestGetOAuthToken_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	token, err := db.GetOAuthToken(ctx, 99999)

	assert.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "oauth token not found")
}

func TestDeleteOAuthToken_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and token
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	token := generateOAuthToken(user.ID)
	err = db.StoreOAuthToken(ctx, token)
	require.NoError(t, err)

	// Delete token
	err = db.DeleteOAuthToken(ctx, user.ID)
	require.NoError(t, err)

	// Verify token is gone
	retrieved, err := db.GetOAuthToken(ctx, user.ID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestDeleteOAuthToken_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.DeleteOAuthToken(ctx, 99999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oauth token not found")
}

// ============================================================================
// Auth Session Tests
// ============================================================================

func TestCreateAuthSession_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	session := generateAuthSession(generateSessionID(), "pending")

	err = db.CreateAuthSession(ctx, session)

	require.NoError(t, err)
	assert.NotZero(t, session.CreatedAt)
	assert.NotZero(t, session.UpdatedAt)
	assert.WithinDuration(t, time.Now(), session.CreatedAt, 2*time.Second)
}

func TestCreateAuthSession_WithUserID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create session with user_id
	session := generateAuthSession(generateSessionID(), "authenticated")
	session.UserID = sql.NullInt64{Int64: user.ID, Valid: true}

	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Verify user_id is stored
	retrieved, err := db.GetAuthSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.True(t, retrieved.UserID.Valid)
	assert.Equal(t, user.ID, retrieved.UserID.Int64)
}

func TestGetAuthSession_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create session
	session := generateAuthSession(generateSessionID(), "pending")
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Retrieve session
	retrieved, err := db.GetAuthSession(ctx, session.SessionID)

	require.NoError(t, err)
	assertSessionEqual(t, session, retrieved)
}

func TestGetAuthSession_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	session, err := db.GetAuthSession(ctx, "nonexistent_session_id")

	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "auth session not found")
}

func TestUpdateAuthSessionStatus_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user and session
	user := generateUser("123456789")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	session := generateAuthSession(generateSessionID(), "pending")
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	originalUpdatedAt := session.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	// Update session status to authenticated
	err = db.UpdateAuthSessionStatus(ctx, session.SessionID, "authenticated", &user.ID, nil)
	require.NoError(t, err)

	// Verify updates
	retrieved, err := db.GetAuthSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, "authenticated", retrieved.AuthStatus)
	assert.True(t, retrieved.UserID.Valid)
	assert.Equal(t, user.ID, retrieved.UserID.Int64)
	assert.True(t, retrieved.UpdatedAt.After(originalUpdatedAt))
}

func TestUpdateAuthSessionStatus_WithError(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create session
	session := generateAuthSession(generateSessionID(), "pending")
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Update to failed status with error message
	errorMsg := "OAuth exchange failed"
	err = db.UpdateAuthSessionStatus(ctx, session.SessionID, "failed", nil, &errorMsg)
	require.NoError(t, err)

	// Verify error message stored
	retrieved, err := db.GetAuthSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, "failed", retrieved.AuthStatus)
	assert.True(t, retrieved.ErrorMessage.Valid)
	assert.Equal(t, errorMsg, retrieved.ErrorMessage.String)
	assert.False(t, retrieved.UserID.Valid)
}

func TestUpdateAuthSessionStatus_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.UpdateAuthSessionStatus(ctx, "nonexistent_session", "authenticated", nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth session not found")
}

func TestDeleteAuthSession_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create session
	session := generateAuthSession(generateSessionID(), "pending")
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Delete session
	err = db.DeleteAuthSession(ctx, session.SessionID)
	require.NoError(t, err)

	// Verify session is gone
	retrieved, err := db.GetAuthSession(ctx, session.SessionID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestDeleteAuthSession_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.DeleteAuthSession(ctx, "nonexistent_session")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth session not found")
}

// ============================================================================
// OAuth State Tests
// ============================================================================

func TestCreateOAuthState_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	state := generateOAuthState(generateSessionID())

	err = db.CreateOAuthState(ctx, state)

	require.NoError(t, err)
	assert.NotZero(t, state.CreatedAt)
	assert.WithinDuration(t, time.Now(), state.CreatedAt, 2*time.Second)
}

func TestValidateAndDeleteOAuthState_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create state
	state := generateOAuthState(generateSessionID())
	err = db.CreateOAuthState(ctx, state)
	require.NoError(t, err)

	// Validate and delete state
	retrieved, err := db.ValidateAndDeleteOAuthState(ctx, state.State)

	require.NoError(t, err)
	assertStateEqual(t, state, retrieved)

	// Verify state is deleted (single-use)
	_, err = db.ValidateAndDeleteOAuthState(ctx, state.State)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
}

func TestValidateAndDeleteOAuthState_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	_, err = db.ValidateAndDeleteOAuthState(ctx, "nonexistent_state_12345")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
}

func TestValidateAndDeleteOAuthState_Expired(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create expired state
	state := generateOAuthState(generateSessionID())
	state.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute) // Already expired

	err = db.CreateOAuthState(ctx, state)
	require.NoError(t, err)

	// Try to validate expired state
	_, err = db.ValidateAndDeleteOAuthState(ctx, state.State)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestValidateAndDeleteOAuthState_SingleUse(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create state
	state := generateOAuthState(generateSessionID())
	err = db.CreateOAuthState(ctx, state)
	require.NoError(t, err)

	// First validation should succeed
	retrieved1, err1 := db.ValidateAndDeleteOAuthState(ctx, state.State)
	require.NoError(t, err1)
	assert.Equal(t, state.State, retrieved1.State)

	// Second validation should fail (already used)
	_, err2 := db.ValidateAndDeleteOAuthState(ctx, state.State)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "invalid state")
}

func TestValidateAndDeleteOAuthState_Concurrent(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create state
	state := generateOAuthState(generateSessionID())
	err = db.CreateOAuthState(ctx, state)
	require.NoError(t, err)

	// Try to validate same state concurrently from 2 goroutines
	var wg sync.WaitGroup
	results := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := db.ValidateAndDeleteOAuthState(ctx, state.State)
			results[index] = err
		}(i)
	}

	wg.Wait()

	// Exactly one should succeed, one should fail
	successCount := 0
	failCount := 0
	for _, err := range results {
		if err == nil {
			successCount++
		} else {
			failCount++
		}
	}

	assert.Equal(t, 1, successCount, "Exactly one validation should succeed")
	assert.Equal(t, 1, failCount, "Exactly one validation should fail")
}

// ============================================================================
// Cleanup Tests
// ============================================================================

func TestCleanupExpiredSessions_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create expired session
	expiredSession := generateAuthSession(generateSessionID(), "pending")
	expiredSession.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour) // Expired 1 hour ago
	err = db.CreateAuthSession(ctx, expiredSession)
	require.NoError(t, err)

	// Create valid session
	validSession := generateAuthSession(generateSessionID(), "pending")
	validSession.ExpiresAt = time.Now().UTC().Add(1 * time.Hour) // Expires in 1 hour
	err = db.CreateAuthSession(ctx, validSession)
	require.NoError(t, err)

	// Create expired state
	expiredState := generateOAuthState(generateSessionID())
	expiredState.ExpiresAt = time.Now().UTC().Add(-1 * time.Minute) // Expired 1 minute ago
	err = db.CreateOAuthState(ctx, expiredState)
	require.NoError(t, err)

	// Create valid state
	validState := generateOAuthState(generateSessionID())
	validState.ExpiresAt = time.Now().UTC().Add(10 * time.Minute) // Expires in 10 minutes
	err = db.CreateOAuthState(ctx, validState)
	require.NoError(t, err)

	// Run cleanup
	err = db.CleanupExpiredSessions(ctx)
	require.NoError(t, err)

	// Verify expired session is deleted
	_, err = db.GetAuthSession(ctx, expiredSession.SessionID)
	assert.Error(t, err)

	// Verify valid session still exists
	retrieved, err := db.GetAuthSession(ctx, validSession.SessionID)
	require.NoError(t, err)
	assert.Equal(t, validSession.SessionID, retrieved.SessionID)

	// Verify expired state is deleted (should return "invalid state")
	_, err = db.ValidateAndDeleteOAuthState(ctx, expiredState.State)
	assert.Error(t, err)

	// Verify valid state still exists
	retrievedState, err := db.ValidateAndDeleteOAuthState(ctx, validState.State)
	require.NoError(t, err)
	assert.Equal(t, validState.State, retrievedState.State)
}

func TestCleanupExpiredSessions_NoExpiredData(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create only valid sessions
	validSession := generateAuthSession(generateSessionID(), "pending")
	validSession.ExpiresAt = time.Now().UTC().Add(1 * time.Hour)
	err = db.CreateAuthSession(ctx, validSession)
	require.NoError(t, err)

	// Run cleanup (should not fail even if nothing to clean)
	err = db.CleanupExpiredSessions(ctx)
	require.NoError(t, err)

	// Verify valid session still exists
	retrieved, err := db.GetAuthSession(ctx, validSession.SessionID)
	require.NoError(t, err)
	assert.Equal(t, validSession.SessionID, retrieved.SessionID)
}

func TestCleanupExpiredSessions_EmptyDatabase(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Run cleanup on empty database
	err = db.CleanupExpiredSessions(ctx)

	// Should not fail
	require.NoError(t, err)
}
