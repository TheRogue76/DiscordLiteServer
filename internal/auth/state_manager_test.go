package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/parsascontentcorner/discordliteserver/internal/testutil"
)

func TestGenerateState(t *testing.T) {
	manager := &StateManager{}

	state, err := manager.GenerateState()

	require.NoError(t, err)
	assert.NotEmpty(t, state)

	// State should be 32 bytes (64 hex chars)
	assert.Equal(t, 64, len(state))
}

func TestGenerateState_Uniqueness(t *testing.T) {
	manager := &StateManager{}

	// Generate 100 states and verify all are unique
	states := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state, err := manager.GenerateState()
		require.NoError(t, err)

		// Check uniqueness
		assert.False(t, states[state], "State should be unique")
		states[state] = true
	}

	assert.Equal(t, 100, len(states))
}

func TestGenerateState_URLSafe(t *testing.T) {
	manager := &StateManager{}

	for i := 0; i < 10; i++ {
		state, err := manager.GenerateState()
		require.NoError(t, err)

		// State should be hex-encoded (only contains 0-9, a-f)
		for _, char := range state {
			assert.True(t,
				(char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'),
				"State should only contain hex characters: %s", state,
			)
		}
	}
}

func TestStoreState(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	manager := NewStateManager(db, 10) // 10 minute expiry
	sessionID := testutil.GenerateSessionID()

	// Generate state first
	state, err := manager.GenerateState()
	require.NoError(t, err)
	require.NotEmpty(t, state)

	// Store the state
	err = manager.StoreState(ctx, state, sessionID)

	require.NoError(t, err)
	assert.NotEmpty(t, state)
	assert.True(t, len(state) > 40) // base64 encoded 32 bytes

	// Verify state was stored in database
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM oauth_states WHERE state = $1", state).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify session_id is correct
	var storedSessionID string
	err = db.QueryRowContext(ctx, "SELECT session_id FROM oauth_states WHERE state = $1", state).Scan(&storedSessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, storedSessionID)

	// Verify expiry is set (should be ~10 minutes in future)
	var expiresAt time.Time
	err = db.QueryRowContext(ctx, "SELECT expires_at FROM oauth_states WHERE state = $1", state).Scan(&expiresAt)
	require.NoError(t, err)

	expectedExpiry := time.Now().UTC().Add(10 * time.Minute)
	timeDiff := expiresAt.Sub(expectedExpiry)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	assert.Less(t, timeDiff, 5*time.Second, "Expiry should be ~10 minutes from now")
}

func TestValidateState_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	manager := NewStateManager(db, 10)
	sessionID := testutil.GenerateSessionID()

	// Store state
	state, err := manager.GenerateState()
	require.NoError(t, err)
	err = manager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Validate state
	retrievedSessionID, err := manager.ValidateState(ctx, state)
	require.NoError(t, err)
	assert.Equal(t, sessionID, retrievedSessionID)

	// Verify state was deleted (single-use)
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM oauth_states WHERE state = $1", state).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestValidateState_InvalidState(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	manager := NewStateManager(db, 10)

	// Try to validate non-existent state
	sessionID, err := manager.ValidateState(ctx, "nonexistent_state_12345")

	assert.Error(t, err)
	assert.Empty(t, sessionID)
	assert.Contains(t, err.Error(), "invalid or expired state")
}

func TestValidateState_ExpiredState(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create manager with very short expiry (1 second)
	manager := NewStateManager(db, 0) // 0 minutes = immediate expiry
	sessionID := testutil.GenerateSessionID()

	// Manually insert expired state
	state, _ := manager.GenerateState()
	expiresAt := time.Now().UTC().Add(-1 * time.Minute) // Already expired
	_, err = db.ExecContext(ctx,
		"INSERT INTO oauth_states (state, session_id, created_at, expires_at) VALUES ($1, $2, $3, $4)",
		state, sessionID, time.Now().UTC(), expiresAt,
	)
	require.NoError(t, err)

	// Try to validate expired state
	retrievedSessionID, err := manager.ValidateState(ctx, state)

	assert.Error(t, err)
	assert.Empty(t, retrievedSessionID)
	assert.Contains(t, err.Error(), "invalid or expired state")
}

func TestValidateState_SingleUseEnforcement(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	manager := NewStateManager(db, 10)
	sessionID := testutil.GenerateSessionID()

	// Store state
	state, err := manager.GenerateState()
	require.NoError(t, err)
	err = manager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// First validation should succeed
	retrievedSessionID1, err1 := manager.ValidateState(ctx, state)
	require.NoError(t, err1)
	assert.Equal(t, sessionID, retrievedSessionID1)

	// Second validation should fail (state already used)
	retrievedSessionID2, err2 := manager.ValidateState(ctx, state)
	assert.Error(t, err2)
	assert.Empty(t, retrievedSessionID2)
	assert.Contains(t, err2.Error(), "invalid or expired state")
}

func TestValidateState_ConcurrentValidation(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	manager := NewStateManager(db, 10)
	sessionID := testutil.GenerateSessionID()

	// Store state
	state, err := manager.GenerateState()
	require.NoError(t, err)
	err = manager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Try to validate same state concurrently from 2 goroutines
	var wg sync.WaitGroup
	results := make([]error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := manager.ValidateState(ctx, state)
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

func TestStateExpiry_EdgeCase(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create manager with 2-second expiry
	manager := NewStateManager(db, 0) // Will manually set expiry
	sessionID := testutil.GenerateSessionID()

	// Manually insert state that expires in 2 seconds
	state, _ := manager.GenerateState()
	expiresAt := time.Now().UTC().Add(2 * time.Second)
	_, err = db.ExecContext(ctx,
		"INSERT INTO oauth_states (state, session_id, created_at, expires_at) VALUES ($1, $2, $3, $4)",
		state, sessionID, time.Now().UTC(), expiresAt,
	)
	require.NoError(t, err)

	// Validate immediately (should succeed)
	retrievedSessionID, err := manager.ValidateState(ctx, state)
	require.NoError(t, err)
	assert.Equal(t, sessionID, retrievedSessionID)
}
