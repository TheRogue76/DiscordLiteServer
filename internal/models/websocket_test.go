package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// WebSocketStatus Tests
// ============================================================================

func TestWebSocketStatus_Constants(t *testing.T) {
	tests := []struct {
		name     string
		status   WebSocketStatus
		expected string
	}{
		{"Connecting status", WebSocketStatusConnecting, "connecting"},
		{"Connected status", WebSocketStatusConnected, "connected"},
		{"Disconnected status", WebSocketStatusDisconnected, "disconnected"},
		{"Failed status", WebSocketStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

// ============================================================================
// WebSocketSession Tests
// ============================================================================

func TestWebSocketSession_IsExpired_NotExpired(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{String: "token", Valid: true},
		SequenceNumber:  100,
		Status:          WebSocketStatusConnected,
		LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
		CreatedAt:       time.Now().Add(-1 * time.Hour),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour), // Expires in 1 hour
	}

	assert.False(t, session.IsExpired(), "Session should not be expired")
}

func TestWebSocketSession_IsExpired_Expired(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{String: "token", Valid: true},
		SequenceNumber:  100,
		Status:          WebSocketStatusConnected,
		LastHeartbeatAt: sql.NullTime{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		CreatedAt:       time.Now().Add(-3 * time.Hour),
		UpdatedAt:       time.Now().Add(-2 * time.Hour),
		ExpiresAt:       time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	assert.True(t, session.IsExpired(), "Session should be expired")
}

func TestWebSocketSession_IsActive_Active(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{String: "token", Valid: true},
		SequenceNumber:  100,
		Status:          WebSocketStatusConnected, // Connected
		LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour), // Not expired
	}

	assert.True(t, session.IsActive(), "Connected and not expired session should be active")
	assert.False(t, session.IsExpired())
	assert.Equal(t, WebSocketStatusConnected, session.Status)
}

func TestWebSocketSession_IsActive_NotConnected(t *testing.T) {
	tests := []struct {
		name   string
		status WebSocketStatus
	}{
		{"Connecting status", WebSocketStatusConnecting},
		{"Disconnected status", WebSocketStatusDisconnected},
		{"Failed status", WebSocketStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &WebSocketSession{
				ID:              1,
				SessionID:       "session123",
				UserID:          456,
				Status:          tt.status, // Not connected
				LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
				ExpiresAt:       time.Now().Add(1 * time.Hour), // Not expired
			}

			assert.False(t, session.IsActive(), "Non-connected session should not be active")
			assert.False(t, session.IsExpired(), "Session is not expired")
		})
	}
}

func TestWebSocketSession_IsActive_ExpiredButConnected(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		Status:          WebSocketStatusConnected, // Connected
		LastHeartbeatAt: sql.NullTime{Time: time.Now().Add(-2 * time.Hour), Valid: true},
		ExpiresAt:       time.Now().Add(-1 * time.Hour), // Expired
	}

	assert.False(t, session.IsActive(), "Expired session should not be active even if connected")
	assert.True(t, session.IsExpired())
}

func TestWebSocketSession_IsActive_ConnectedButNoExpiry(t *testing.T) {
	// Session with zero expiry time
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		Status:          WebSocketStatusConnected,
		LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
		ExpiresAt:       time.Time{}, // Zero time (expired)
	}

	assert.False(t, session.IsActive(), "Session with zero expiry should not be active")
	assert.True(t, session.IsExpired(), "Zero expiry time should be considered expired")
}

func TestWebSocketSession_WithoutHeartbeat(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{String: "token", Valid: true},
		SequenceNumber:  0,
		Status:          WebSocketStatusConnecting,
		LastHeartbeatAt: sql.NullTime{Valid: false}, // No heartbeat yet
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
	}

	assert.False(t, session.LastHeartbeatAt.Valid, "New session should not have heartbeat")
	assert.False(t, session.IsActive(), "Connecting session without heartbeat should not be active")
}

func TestWebSocketSession_WithSessionToken(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{String: "discord_session_token", Valid: true},
		SequenceNumber:  150,
		Status:          WebSocketStatusConnected,
		LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
	}

	assert.True(t, session.SessionToken.Valid, "Session should have token")
	assert.Equal(t, "discord_session_token", session.SessionToken.String)
	assert.True(t, session.IsActive())
}

func TestWebSocketSession_WithoutSessionToken(t *testing.T) {
	// Session without token (perhaps initial connection)
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{Valid: false}, // No token yet
		SequenceNumber:  0,
		Status:          WebSocketStatusConnecting,
		LastHeartbeatAt: sql.NullTime{Valid: false},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
	}

	assert.False(t, session.SessionToken.Valid, "Initial connection may not have token")
	assert.False(t, session.IsActive(), "Session without token should not be active")
}

func TestWebSocketSession_SequenceNumbers(t *testing.T) {
	tests := []struct {
		name           string
		sequenceNumber int64
	}{
		{"Zero sequence", 0},
		{"Small sequence", 10},
		{"Large sequence", 1000},
		{"Very large sequence", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &WebSocketSession{
				ID:              1,
				SessionID:       "session123",
				UserID:          456,
				SequenceNumber:  tt.sequenceNumber,
				Status:          WebSocketStatusConnected,
				LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
				ExpiresAt:       time.Now().Add(1 * time.Hour),
			}

			assert.Equal(t, tt.sequenceNumber, session.SequenceNumber)
			assert.True(t, session.IsActive())
		})
	}
}

func TestWebSocketSession_StatusTransitions(t *testing.T) {
	// Test various status transitions
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		GatewayURL:      "wss://gateway.discord.gg",
		SessionToken:    sql.NullString{String: "token", Valid: true},
		SequenceNumber:  50,
		LastHeartbeatAt: sql.NullTime{Time: time.Now(), Valid: true},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		ExpiresAt:       time.Now().Add(1 * time.Hour),
	}

	// Start connecting
	session.Status = WebSocketStatusConnecting
	assert.False(t, session.IsActive())

	// Become connected
	session.Status = WebSocketStatusConnected
	assert.True(t, session.IsActive())

	// Disconnect
	session.Status = WebSocketStatusDisconnected
	assert.False(t, session.IsActive())

	// Fail
	session.Status = WebSocketStatusFailed
	assert.False(t, session.IsActive())
}

func TestWebSocketSession_RecentHeartbeat(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		Status:          WebSocketStatusConnected,
		LastHeartbeatAt: sql.NullTime{Time: time.Now().Add(-5 * time.Second), Valid: true},
		ExpiresAt:       time.Now().Add(1 * time.Hour),
	}

	assert.True(t, session.IsActive(), "Session with recent heartbeat should be active")
	assert.True(t, session.LastHeartbeatAt.Valid)

	// Verify heartbeat is within last minute
	timeSinceHeartbeat := time.Since(session.LastHeartbeatAt.Time)
	assert.Less(t, timeSinceHeartbeat, 1*time.Minute, "Heartbeat should be recent")
}

func TestWebSocketSession_OldHeartbeat(t *testing.T) {
	session := &WebSocketSession{
		ID:              1,
		SessionID:       "session123",
		UserID:          456,
		Status:          WebSocketStatusConnected,
		LastHeartbeatAt: sql.NullTime{Time: time.Now().Add(-10 * time.Minute), Valid: true},
		ExpiresAt:       time.Now().Add(1 * time.Hour), // Still not expired
	}

	// Session is still active even with old heartbeat (not expired yet)
	assert.True(t, session.IsActive())
	assert.True(t, session.LastHeartbeatAt.Valid)

	// Verify heartbeat is old
	timeSinceHeartbeat := time.Since(session.LastHeartbeatAt.Time)
	assert.Greater(t, timeSinceHeartbeat, 5*time.Minute, "Heartbeat should be old")
}

func TestWebSocketSession_ZeroValue(t *testing.T) {
	// Test zero value behavior
	session := &WebSocketSession{}

	assert.True(t, session.IsExpired(), "Zero value session should be expired")
	assert.False(t, session.IsActive(), "Zero value session should not be active")
	assert.Equal(t, WebSocketStatus(""), session.Status)
}
