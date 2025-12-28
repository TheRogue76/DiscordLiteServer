// Package models defines data structures for authentication and user management.
package models

import (
	"database/sql"
	"time"
)

// User represents a Discord user
type User struct {
	ID            int64          `json:"id"`
	DiscordID     string         `json:"discord_id"`
	Username      string         `json:"username"`
	Discriminator sql.NullString `json:"discriminator"`
	Avatar        sql.NullString `json:"avatar"`
	Email         sql.NullString `json:"email"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// OAuthToken represents an encrypted OAuth token
type OAuthToken struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	AccessToken  string    `json:"access_token"`  // Encrypted
	RefreshToken string    `json:"refresh_token"` // Encrypted
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OAuthState represents a temporary OAuth state for CSRF protection
type OAuthState struct {
	State     string    `json:"state"`
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AuthSession represents a user authentication session
type AuthSession struct {
	SessionID    string         `json:"session_id"`
	UserID       sql.NullInt64  `json:"user_id"`
	AuthStatus   string         `json:"auth_status"` // 'pending', 'authenticated', 'failed'
	ErrorMessage sql.NullString `json:"error_message"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	ExpiresAt    time.Time      `json:"expires_at"`
}

// AuthStatus constants
const (
	AuthStatusPending       = "pending"
	AuthStatusAuthenticated = "authenticated"
	AuthStatusFailed        = "failed"
)

// IsExpired checks if the auth session has expired
func (a *AuthSession) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// IsExpired checks if the OAuth state has expired
func (s *OAuthState) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsExpired checks if the OAuth token has expired
func (t *OAuthToken) IsExpired() bool {
	return time.Now().After(t.Expiry)
}
