package testutil

import (
	"testing"
	"time"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
	"github.com/stretchr/testify/assert"
)

// AssertUserEqual performs a deep comparison of two User objects.
// Ignores timestamps (CreatedAt, UpdatedAt) as they may vary slightly.
func AssertUserEqual(t *testing.T, expected, actual *models.User) {
	t.Helper()

	assert.Equal(t, expected.DiscordID, actual.DiscordID, "DiscordID should match")
	assert.Equal(t, expected.Username, actual.Username, "Username should match")
	assert.Equal(t, expected.Discriminator, actual.Discriminator, "Discriminator should match")
	assert.Equal(t, expected.Avatar, actual.Avatar, "Avatar should match")
	assert.Equal(t, expected.Email, actual.Email, "Email should match")

	// Optionally check timestamps if needed
	if !expected.CreatedAt.IsZero() {
		AssertTimeAlmostEqual(t, expected.CreatedAt, actual.CreatedAt, 2*time.Second)
	}
	if !expected.UpdatedAt.IsZero() {
		AssertTimeAlmostEqual(t, expected.UpdatedAt, actual.UpdatedAt, 2*time.Second)
	}
}

// AssertTokenEqual performs a deep comparison of two OAuthToken objects.
// Ignores timestamps as they may vary slightly.
func AssertTokenEqual(t *testing.T, expected, actual *models.OAuthToken) {
	t.Helper()

	assert.Equal(t, expected.UserID, actual.UserID, "UserID should match")
	assert.Equal(t, expected.AccessToken, actual.AccessToken, "AccessToken should match")
	assert.Equal(t, expected.RefreshToken, actual.RefreshToken, "RefreshToken should match")
	assert.Equal(t, expected.TokenType, actual.TokenType, "TokenType should match")
	assert.Equal(t, expected.Scope, actual.Scope, "Scope should match")

	// Check expiry with tolerance
	AssertTimeAlmostEqual(t, expected.Expiry, actual.Expiry, 2*time.Second)
}

// AssertSessionEqual performs a deep comparison of two AuthSession objects.
// Ignores exact timestamp matching but checks key fields.
func AssertSessionEqual(t *testing.T, expected, actual *models.AuthSession) {
	t.Helper()

	assert.Equal(t, expected.SessionID, actual.SessionID, "SessionID should match")
	assert.Equal(t, expected.UserID, actual.UserID, "UserID should match")
	assert.Equal(t, expected.AuthStatus, actual.AuthStatus, "AuthStatus should match")
	assert.Equal(t, expected.ErrorMessage, actual.ErrorMessage, "ErrorMessage should match")

	// Check expiry with tolerance
	AssertTimeAlmostEqual(t, expected.ExpiresAt, actual.ExpiresAt, 2*time.Second)
}

// AssertStateEqual performs a deep comparison of two OAuthState objects.
func AssertStateEqual(t *testing.T, expected, actual *models.OAuthState) {
	t.Helper()

	assert.Equal(t, expected.State, actual.State, "State should match")
	assert.Equal(t, expected.SessionID, actual.SessionID, "SessionID should match")

	// Check expiry with tolerance
	AssertTimeAlmostEqual(t, expected.ExpiresAt, actual.ExpiresAt, 2*time.Second)
}

// AssertTimeAlmostEqual checks if two times are within a specified delta.
// Useful for timestamp comparisons where exact equality isn't expected.
func AssertTimeAlmostEqual(t *testing.T, expected, actual time.Time, delta time.Duration) {
	t.Helper()

	diff := expected.Sub(actual)
	if diff < 0 {
		diff = -diff
	}

	assert.True(t,
		diff <= delta,
		"Times should be within %v of each other. Expected: %v, Actual: %v, Diff: %v",
		delta, expected, actual, diff,
	)
}
