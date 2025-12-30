package models

import (
	"database/sql"
	"time"
)

// WebSocketStatus represents the status of a WebSocket connection
type WebSocketStatus string

// WebSocket connection status constants
const (
	WebSocketStatusConnecting   WebSocketStatus = "connecting"
	WebSocketStatusConnected    WebSocketStatus = "connected"
	WebSocketStatusDisconnected WebSocketStatus = "disconnected"
	WebSocketStatusFailed       WebSocketStatus = "failed"
)

// WebSocketSession represents an active Discord Gateway WebSocket connection
type WebSocketSession struct {
	ID              int64           `json:"id"`
	SessionID       string          `json:"session_id"`
	UserID          int64           `json:"user_id"`
	GatewayURL      string          `json:"gateway_url"`
	SessionToken    sql.NullString  `json:"session_token"`
	SequenceNumber  int64           `json:"sequence_number"`
	Status          WebSocketStatus `json:"status"`
	LastHeartbeatAt sql.NullTime    `json:"last_heartbeat_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ExpiresAt       time.Time       `json:"expires_at"`
}

// IsExpired checks if the WebSocket session has expired
func (w *WebSocketSession) IsExpired() bool {
	return time.Now().After(w.ExpiresAt)
}

// IsActive checks if the WebSocket session is active
func (w *WebSocketSession) IsActive() bool {
	return w.Status == WebSocketStatusConnected && !w.IsExpired()
}
