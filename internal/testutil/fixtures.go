package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// GenerateUser creates a test user with the given Discord ID.
// Optional fields (discriminator, avatar, email) are set to test values.
func GenerateUser(discordID string) *models.User {
	return &models.User{
		DiscordID:     discordID,
		Username:      fmt.Sprintf("testuser_%s", discordID),
		Discriminator: sql.NullString{String: "1234", Valid: true},
		Avatar:        sql.NullString{String: "test_avatar_hash", Valid: true},
		Email:         sql.NullString{String: fmt.Sprintf("%s@test.com", discordID), Valid: true},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
}

// GenerateUserWithNulls creates a test user with null optional fields.
func GenerateUserWithNulls(discordID string) *models.User {
	return &models.User{
		DiscordID:     discordID,
		Username:      fmt.Sprintf("testuser_%s", discordID),
		Discriminator: sql.NullString{Valid: false},
		Avatar:        sql.NullString{Valid: false},
		Email:         sql.NullString{Valid: false},
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
}

// GenerateOAuthToken creates a test OAuth token for the given user ID.
// The token is encrypted (not plaintext) and has a future expiry.
func GenerateOAuthToken(userID int64) *models.OAuthToken {
	return &models.OAuthToken{
		UserID:       userID,
		AccessToken:  "encrypted_access_token_test_value",
		RefreshToken: "encrypted_refresh_token_test_value",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(24 * time.Hour),
		Scope:        "identify email guilds",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

// GenerateExpiredOAuthToken creates a test OAuth token that is already expired.
func GenerateExpiredOAuthToken(userID int64) *models.OAuthToken {
	return &models.OAuthToken{
		UserID:       userID,
		AccessToken:  "expired_access_token",
		RefreshToken: "expired_refresh_token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(-24 * time.Hour), // Expired
		Scope:        "identify email",
		CreatedAt:    time.Now().UTC().Add(-48 * time.Hour),
		UpdatedAt:    time.Now().UTC().Add(-48 * time.Hour),
	}
}

// GenerateAuthSession creates a test auth session with the given session ID and status.
func GenerateAuthSession(sessionID string, status string) *models.AuthSession {
	return &models.AuthSession{
		SessionID:    sessionID,
		UserID:       sql.NullInt64{Valid: false}, // Null by default
		AuthStatus:   status,
		ErrorMessage: sql.NullString{Valid: false},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(24 * time.Hour),
	}
}

// GenerateAuthSessionWithUser creates an authenticated session with a user ID.
func GenerateAuthSessionWithUser(sessionID string, userID int64) *models.AuthSession {
	return &models.AuthSession{
		SessionID:    sessionID,
		UserID:       sql.NullInt64{Int64: userID, Valid: true},
		AuthStatus:   "authenticated",
		ErrorMessage: sql.NullString{Valid: false},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(24 * time.Hour),
	}
}

// GenerateExpiredAuthSession creates an auth session that is already expired.
func GenerateExpiredAuthSession(sessionID string) *models.AuthSession {
	return &models.AuthSession{
		SessionID:    sessionID,
		UserID:       sql.NullInt64{Valid: false},
		AuthStatus:   "pending",
		ErrorMessage: sql.NullString{Valid: false},
		CreatedAt:    time.Now().UTC().Add(-48 * time.Hour),
		UpdatedAt:    time.Now().UTC().Add(-48 * time.Hour),
		ExpiresAt:    time.Now().UTC().Add(-24 * time.Hour), // Expired
	}
}

// GenerateOAuthState creates a test OAuth state for the given session ID.
func GenerateOAuthState(sessionID string) *models.OAuthState {
	return &models.OAuthState{
		State:     GenerateRandomState(),
		SessionID: sessionID,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute),
	}
}

// GenerateExpiredOAuthState creates an OAuth state that is already expired.
func GenerateExpiredOAuthState(sessionID string) *models.OAuthState {
	return &models.OAuthState{
		State:     GenerateRandomState(),
		SessionID: sessionID,
		CreatedAt: time.Now().UTC().Add(-15 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-5 * time.Minute), // Expired
	}
}

// GenerateRandomState generates a random state string (32 bytes, base64 URL-encoded).
func GenerateRandomState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random state: %v", err))
	}
	return hex.EncodeToString(b)
}

// GenerateSessionID generates a random session ID (UUID).
func GenerateSessionID() string {
	return uuid.New().String()
}

// GenerateEncryptionKey generates a 32-byte encryption key for testing.
func GenerateEncryptionKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("failed to generate encryption key: %v", err))
	}
	return key
}

// GenerateTestConfig creates a test configuration with valid values.
// Uses a generated encryption key and test database credentials.
func GenerateTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			HTTPPort: "8080",
			GRPCPort: "50051",
			Host:     "localhost",
			Env:      "test",
		},
		Discord: config.DiscordConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
			RedirectURI:  "http://localhost:8080/auth/callback",
			Scopes:       []string{"identify", "email", "guilds"},
		},
		Database: config.DatabaseConfig{
			Host:         "localhost",
			Port:         "5432",
			User:         "testuser",
			Password:     "testpass",
			Name:         "testdb",
			SSLMode:      "disable",
			MaxOpenConns: 5,
			MaxIdleConns: 2,
		},
		Security: config.SecurityConfig{
			TokenEncryptionKey: GenerateEncryptionKey(),
			SessionExpiryHours: 24,
			StateExpiryMinutes: 10,
		},
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "console",
		},
	}
}
