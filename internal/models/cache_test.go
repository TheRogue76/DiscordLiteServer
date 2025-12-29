package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// CacheType Tests
// ============================================================================

func TestCacheType_Constants(t *testing.T) {
	tests := []struct {
		name      string
		cacheType CacheType
		expected  string
	}{
		{"Guild cache type", CacheTypeGuild, "guild"},
		{"Channel cache type", CacheTypeChannel, "channel"},
		{"Message cache type", CacheTypeMessage, "message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.cacheType))
		})
	}
}

// ============================================================================
// CacheMetadata Tests
// ============================================================================

func TestCacheMetadata_IsExpired_NotExpired(t *testing.T) {
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeGuild,
		EntityID:      "guild123",
		UserID:        sql.NullInt64{Valid: false},
		LastFetchedAt: time.Now().Add(-30 * time.Minute),
		ExpiresAt:     time.Now().Add(30 * time.Minute), // Expires in 30 minutes
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.False(t, cache.IsExpired(), "Cache should not be expired")
}

func TestCacheMetadata_IsExpired_Expired(t *testing.T) {
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeChannel,
		EntityID:      "channel456",
		UserID:        sql.NullInt64{Int64: 123, Valid: true},
		LastFetchedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt:     time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		CreatedAt:     time.Now().Add(-3 * time.Hour),
		UpdatedAt:     time.Now().Add(-2 * time.Hour),
	}

	assert.True(t, cache.IsExpired(), "Cache should be expired")
}

func TestCacheMetadata_IsExpired_ExactlyNow(t *testing.T) {
	// Cache that expires exactly now (within 1 second tolerance)
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeMessage,
		EntityID:      "message789",
		UserID:        sql.NullInt64{Valid: false},
		LastFetchedAt: time.Now().Add(-5 * time.Minute),
		ExpiresAt:     time.Now(),
		CreatedAt:     time.Now().Add(-10 * time.Minute),
		UpdatedAt:     time.Now().Add(-5 * time.Minute),
	}

	// Since time.Now() is called at different moments, this should be expired
	// (the ExpiresAt time.Now() happened first, then IsExpired()'s time.Now())
	time.Sleep(10 * time.Millisecond) // Ensure ExpiresAt is in the past
	assert.True(t, cache.IsExpired(), "Cache expiring now should be considered expired")
}

func TestCacheMetadata_IsValid_Valid(t *testing.T) {
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeGuild,
		EntityID:      "guild123",
		UserID:        sql.NullInt64{Valid: false},
		LastFetchedAt: time.Now().Add(-30 * time.Minute),
		ExpiresAt:     time.Now().Add(30 * time.Minute), // Valid for 30 more minutes
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.True(t, cache.IsValid(), "Cache should be valid")
	assert.False(t, cache.IsExpired(), "Cache should not be expired")
}

func TestCacheMetadata_IsValid_Invalid(t *testing.T) {
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeChannel,
		EntityID:      "channel456",
		UserID:        sql.NullInt64{Int64: 123, Valid: true},
		LastFetchedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt:     time.Now().Add(-1 * time.Hour), // Expired
		CreatedAt:     time.Now().Add(-3 * time.Hour),
		UpdatedAt:     time.Now().Add(-2 * time.Hour),
	}

	assert.False(t, cache.IsValid(), "Cache should not be valid")
	assert.True(t, cache.IsExpired(), "Cache should be expired")
}

func TestCacheMetadata_GlobalCache(t *testing.T) {
	// Global cache (no user_id)
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeGuild,
		EntityID:      "guild123",
		UserID:        sql.NullInt64{Valid: false}, // No user ID - global cache
		LastFetchedAt: time.Now(),
		ExpiresAt:     time.Now().Add(1 * time.Hour),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.False(t, cache.UserID.Valid, "Global cache should not have user ID")
	assert.True(t, cache.IsValid(), "Global cache should be valid")
}

func TestCacheMetadata_UserSpecificCache(t *testing.T) {
	// User-specific cache
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeMessage,
		EntityID:      "message789",
		UserID:        sql.NullInt64{Int64: 456, Valid: true}, // User-specific
		LastFetchedAt: time.Now(),
		ExpiresAt:     time.Now().Add(5 * time.Minute),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.True(t, cache.UserID.Valid, "User-specific cache should have user ID")
	assert.Equal(t, int64(456), cache.UserID.Int64)
	assert.True(t, cache.IsValid(), "User-specific cache should be valid")
}

func TestCacheMetadata_DifferentCacheTypes(t *testing.T) {
	tests := []struct {
		name      string
		cacheType CacheType
		entityID  string
		ttl       time.Duration
	}{
		{"Guild cache - 1 hour TTL", CacheTypeGuild, "guild1", 1 * time.Hour},
		{"Channel cache - 30 min TTL", CacheTypeChannel, "channel1", 30 * time.Minute},
		{"Message cache - 5 min TTL", CacheTypeMessage, "message1", 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := &CacheMetadata{
				CacheType:     tt.cacheType,
				EntityID:      tt.entityID,
				LastFetchedAt: time.Now(),
				ExpiresAt:     time.Now().Add(tt.ttl),
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			assert.Equal(t, tt.cacheType, cache.CacheType)
			assert.Equal(t, tt.entityID, cache.EntityID)
			assert.True(t, cache.IsValid(), "Cache should be valid immediately after creation")
		})
	}
}

func TestCacheMetadata_ZeroValue(t *testing.T) {
	// Test zero value behavior
	cache := &CacheMetadata{}

	// Zero time is in the past, so should be expired
	assert.True(t, cache.IsExpired(), "Zero value cache should be expired")
	assert.False(t, cache.IsValid(), "Zero value cache should not be valid")
}

func TestCacheMetadata_FarFutureExpiry(t *testing.T) {
	// Cache that expires far in the future
	cache := &CacheMetadata{
		ID:            1,
		CacheType:     CacheTypeGuild,
		EntityID:      "guild123",
		LastFetchedAt: time.Now(),
		ExpiresAt:     time.Now().Add(365 * 24 * time.Hour), // 1 year
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.False(t, cache.IsExpired(), "Cache with far future expiry should not be expired")
	assert.True(t, cache.IsValid(), "Cache with far future expiry should be valid")
}
