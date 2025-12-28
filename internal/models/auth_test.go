package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthSession_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expired - 1 hour ago",
			expiresAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "expired - 1 second ago",
			expiresAt: time.Now().Add(-1 * time.Second),
			want:      true,
		},
		{
			name:      "not expired - 1 hour from now",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "not expired - 1 day from now",
			expiresAt: time.Now().Add(24 * time.Hour),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &AuthSession{
				SessionID: "test_session",
				ExpiresAt: tt.expiresAt,
			}

			got := session.IsExpired()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOAuthState_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expired - 10 minutes ago",
			expiresAt: time.Now().Add(-10 * time.Minute),
			want:      true,
		},
		{
			name:      "expired - 1 minute ago",
			expiresAt: time.Now().Add(-1 * time.Minute),
			want:      true,
		},
		{
			name:      "not expired - 5 minutes from now",
			expiresAt: time.Now().Add(5 * time.Minute),
			want:      false,
		},
		{
			name:      "not expired - 10 minutes from now",
			expiresAt: time.Now().Add(10 * time.Minute),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &OAuthState{
				State:     "test_state",
				SessionID: "test_session",
				ExpiresAt: tt.expiresAt,
			}

			got := state.IsExpired()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOAuthToken_IsExpired(t *testing.T) {
	tests := []struct {
		name   string
		expiry time.Time
		want   bool
	}{
		{
			name:   "expired - 1 day ago",
			expiry: time.Now().Add(-24 * time.Hour),
			want:   true,
		},
		{
			name:   "expired - 1 hour ago",
			expiry: time.Now().Add(-1 * time.Hour),
			want:   true,
		},
		{
			name:   "not expired - 1 hour from now",
			expiry: time.Now().Add(1 * time.Hour),
			want:   false,
		},
		{
			name:   "not expired - 7 days from now",
			expiry: time.Now().Add(7 * 24 * time.Hour),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &OAuthToken{
				UserID: 123,
				Expiry: tt.expiry,
			}

			got := token.IsExpired()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAuthStatus_Constants(t *testing.T) {
	// Verify auth status constants exist and have expected values
	assert.Equal(t, "pending", AuthStatusPending)
	assert.Equal(t, "authenticated", AuthStatusAuthenticated)
	assert.Equal(t, "failed", AuthStatusFailed)
}

func TestAuthSession_IsExpired_EdgeCase(t *testing.T) {
	// Test edge case: expiry exactly at current time
	// Due to timing, this should be expired (After returns true when equal)
	session := &AuthSession{
		SessionID: "edge_case_session",
		ExpiresAt: time.Now(),
	}

	// Sleep a tiny bit to ensure we're past the expiry time
	time.Sleep(1 * time.Millisecond)

	got := session.IsExpired()
	assert.True(t, got, "Session with expiry at current time should be expired after a millisecond")
}

func TestOAuthState_IsExpired_EdgeCase(t *testing.T) {
	state := &OAuthState{
		State:     "edge_state",
		SessionID: "edge_session",
		ExpiresAt: time.Now(),
	}

	time.Sleep(1 * time.Millisecond)

	got := state.IsExpired()
	assert.True(t, got, "State with expiry at current time should be expired after a millisecond")
}

func TestOAuthToken_IsExpired_EdgeCase(t *testing.T) {
	token := &OAuthToken{
		UserID: 456,
		Expiry: time.Now(),
	}

	time.Sleep(1 * time.Millisecond)

	got := token.IsExpired()
	assert.True(t, got, "Token with expiry at current time should be expired after a millisecond")
}

func TestAuthSession_IsExpired_FarFuture(t *testing.T) {
	// Test with a very far future date
	session := &AuthSession{
		SessionID: "far_future_session",
		ExpiresAt: time.Now().Add(100 * 365 * 24 * time.Hour), // 100 years
	}

	got := session.IsExpired()
	assert.False(t, got, "Session expiring in 100 years should not be expired")
}

func TestAuthSession_IsExpired_FarPast(t *testing.T) {
	// Test with a very far past date
	session := &AuthSession{
		SessionID: "far_past_session",
		ExpiresAt: time.Now().Add(-100 * 365 * 24 * time.Hour), // 100 years ago
	}

	got := session.IsExpired()
	assert.True(t, got, "Session expired 100 years ago should be expired")
}
