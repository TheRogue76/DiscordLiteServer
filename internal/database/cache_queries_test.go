package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Cache Metadata Tests
// ============================================================================

func TestSetCacheMetadata_GlobalCache(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set cache without userID (global cache)
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild123", nil, 1*time.Hour)
	require.NoError(t, err)

	// Verify cache was set
	cache, err := db.GetCacheMetadata(ctx, models.CacheTypeGuild, "guild123", nil)
	require.NoError(t, err)
	assert.Equal(t, models.CacheTypeGuild, cache.CacheType)
	assert.Equal(t, "guild123", cache.EntityID)
	assert.False(t, cache.UserID.Valid)
	assert.WithinDuration(t, time.Now().UTC().Add(1*time.Hour), cache.ExpiresAt, 5*time.Second)
}

func TestSetCacheMetadata_UserSpecificCache(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Set user-specific cache
	err = db.SetCacheMetadata(ctx, models.CacheTypeMessage, "channel123", &user.ID, 5*time.Minute)
	require.NoError(t, err)

	// Verify cache was set
	cache, err := db.GetCacheMetadata(ctx, models.CacheTypeMessage, "channel123", &user.ID)
	require.NoError(t, err)
	assert.Equal(t, models.CacheTypeMessage, cache.CacheType)
	assert.Equal(t, "channel123", cache.EntityID)
	assert.True(t, cache.UserID.Valid)
	assert.Equal(t, user.ID, cache.UserID.Int64)
	assert.WithinDuration(t, time.Now().UTC().Add(5*time.Minute), cache.ExpiresAt, 5*time.Second)
}

func TestSetCacheMetadata_Upsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set cache with 1 hour TTL
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", nil, 1*time.Hour)
	require.NoError(t, err)

	cache1, err := db.GetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", nil)
	require.NoError(t, err)
	originalID := cache1.ID

	// Update cache with same key but different TTL (should upsert, not error)
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", nil, 30*time.Minute)
	require.NoError(t, err, "Upsert should not fail on conflict")

	// Verify only one entry exists (ID should be same)
	cache2, err := db.GetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", nil)
	require.NoError(t, err)
	assert.Equal(t, originalID, cache2.ID, "Upsert should update existing row, not create new one")
}

func TestGetCacheMetadata_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	cache, err := db.GetCacheMetadata(ctx, models.CacheTypeGuild, "nonexistent", nil)

	assert.Error(t, err)
	assert.Nil(t, cache)
	assert.Contains(t, err.Error(), "cache metadata not found")
}

// ============================================================================
// Cache Validation Tests
// ============================================================================

func TestIsCacheValid_Valid(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set cache with 1 hour TTL
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild123", nil, 1*time.Hour)
	require.NoError(t, err)

	// Check if valid
	valid, err := db.IsCacheValid(ctx, models.CacheTypeGuild, "guild123", nil)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestIsCacheValid_Expired(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set cache with very short TTL (1 millisecond)
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", nil, 1*time.Millisecond)
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Check if valid (should be expired)
	valid, err := db.IsCacheValid(ctx, models.CacheTypeChannel, "channel123", nil)
	require.NoError(t, err)
	assert.False(t, valid, "Cache should be expired")
}

func TestIsCacheValid_NotExists(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Check non-existent cache
	valid, err := db.IsCacheValid(ctx, models.CacheTypeGuild, "nonexistent", nil)
	require.NoError(t, err)
	assert.False(t, valid, "Non-existent cache should be invalid")
}

// ============================================================================
// Cache Invalidation Tests
// ============================================================================

func TestInvalidateCache_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set cache
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild123", nil, 1*time.Hour)
	require.NoError(t, err)

	// Verify it exists
	valid, err := db.IsCacheValid(ctx, models.CacheTypeGuild, "guild123", nil)
	require.NoError(t, err)
	assert.True(t, valid)

	// Invalidate cache
	err = db.InvalidateCache(ctx, models.CacheTypeGuild, "guild123", nil)
	require.NoError(t, err)

	// Verify it's gone
	valid, err = db.IsCacheValid(ctx, models.CacheTypeGuild, "guild123", nil)
	require.NoError(t, err)
	assert.False(t, valid)
}

func TestInvalidateCache_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Invalidate non-existent cache (should not error, just no-op)
	err = db.InvalidateCache(ctx, models.CacheTypeGuild, "nonexistent", nil)
	require.NoError(t, err, "Invalidating non-existent cache should not error")
}

func TestInvalidateCacheByType(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set multiple guild caches
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild1", nil, 1*time.Hour)
	require.NoError(t, err)
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild2", nil, 1*time.Hour)
	require.NoError(t, err)

	// Set a channel cache (different type)
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel1", nil, 1*time.Hour)
	require.NoError(t, err)

	// Invalidate all guild caches
	err = db.InvalidateCacheByType(ctx, models.CacheTypeGuild)
	require.NoError(t, err)

	// Verify guild caches are gone
	valid, _ := db.IsCacheValid(ctx, models.CacheTypeGuild, "guild1", nil)
	assert.False(t, valid)
	valid, _ = db.IsCacheValid(ctx, models.CacheTypeGuild, "guild2", nil)
	assert.False(t, valid)

	// Verify channel cache still exists
	valid, _ = db.IsCacheValid(ctx, models.CacheTypeChannel, "channel1", nil)
	assert.True(t, valid)
}

func TestInvalidateCacheForUser(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create 2 users
	user1 := generateUser("user1")
	err = db.CreateUser(ctx, user1)
	require.NoError(t, err)

	user2 := generateUser("user2")
	err = db.CreateUser(ctx, user2)
	require.NoError(t, err)

	// Set user-specific caches
	err = db.SetCacheMetadata(ctx, models.CacheTypeMessage, "channel1", &user1.ID, 1*time.Hour)
	require.NoError(t, err)
	err = db.SetCacheMetadata(ctx, models.CacheTypeMessage, "channel2", &user1.ID, 1*time.Hour)
	require.NoError(t, err)
	err = db.SetCacheMetadata(ctx, models.CacheTypeMessage, "channel3", &user2.ID, 1*time.Hour)
	require.NoError(t, err)

	// Invalidate all caches for user1
	err = db.InvalidateCacheForUser(ctx, user1.ID)
	require.NoError(t, err)

	// Verify user1's caches are gone
	valid, _ := db.IsCacheValid(ctx, models.CacheTypeMessage, "channel1", &user1.ID)
	assert.False(t, valid)
	valid, _ = db.IsCacheValid(ctx, models.CacheTypeMessage, "channel2", &user1.ID)
	assert.False(t, valid)

	// Verify user2's cache still exists
	valid, _ = db.IsCacheValid(ctx, models.CacheTypeMessage, "channel3", &user2.ID)
	assert.True(t, valid)
}

// ============================================================================
// Cache Cleanup Tests
// ============================================================================

func TestCleanupExpiredCache(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set expired cache (1 millisecond TTL)
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "expired_guild", nil, 1*time.Millisecond)
	require.NoError(t, err)

	// Set valid cache (1 hour TTL)
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "valid_channel", nil, 1*time.Hour)
	require.NoError(t, err)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Run cleanup
	err = db.CleanupExpiredCache(ctx)
	require.NoError(t, err)

	// Verify expired cache is gone
	valid, _ := db.IsCacheValid(ctx, models.CacheTypeGuild, "expired_guild", nil)
	assert.False(t, valid)

	// Verify valid cache still exists
	valid, _ = db.IsCacheValid(ctx, models.CacheTypeChannel, "valid_channel", nil)
	assert.True(t, valid)
}

func TestCleanupExpiredCache_NoExpiredData(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Set only valid caches
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild1", nil, 1*time.Hour)
	require.NoError(t, err)

	// Run cleanup (should not fail)
	err = db.CleanupExpiredCache(ctx)
	require.NoError(t, err)

	// Verify cache still exists
	valid, _ := db.IsCacheValid(ctx, models.CacheTypeGuild, "guild1", nil)
	assert.True(t, valid)
}

// ============================================================================
// Cache Statistics Tests
// ============================================================================

func TestGetCacheStats(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Initially empty (no stats returned)
	stats, err := db.GetCacheStats(ctx)
	require.NoError(t, err)
	assert.Empty(t, stats)

	// Add caches of different types
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild1", nil, 1*time.Hour)
	require.NoError(t, err)
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, "guild2", nil, 1*time.Hour)
	require.NoError(t, err)
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel1", nil, 1*time.Hour)
	require.NoError(t, err)
	err = db.SetCacheMetadata(ctx, models.CacheTypeMessage, "message1", nil, 1*time.Hour)
	require.NoError(t, err)

	// Get stats (keys are "type_total", "type_valid", "type_expired")
	stats, err = db.GetCacheStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats["guild_total"])
	assert.Equal(t, int64(2), stats["guild_valid"])
	assert.Equal(t, int64(1), stats["channel_total"])
	assert.Equal(t, int64(1), stats["channel_valid"])
	assert.Equal(t, int64(1), stats["message_total"])
	assert.Equal(t, int64(1), stats["message_valid"])
}

// ============================================================================
// Cache Isolation Tests
// ============================================================================

func TestCacheIsolation_GlobalVsUserSpecific(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Set global cache for channel
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", nil, 1*time.Hour)
	require.NoError(t, err)

	// Set user-specific cache for same channel
	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, "channel123", &user.ID, 1*time.Hour)
	require.NoError(t, err)

	// Both should exist independently
	globalValid, _ := db.IsCacheValid(ctx, models.CacheTypeChannel, "channel123", nil)
	assert.True(t, globalValid)

	userValid, _ := db.IsCacheValid(ctx, models.CacheTypeChannel, "channel123", &user.ID)
	assert.True(t, userValid)

	// Invalidate user-specific cache
	err = db.InvalidateCache(ctx, models.CacheTypeChannel, "channel123", &user.ID)
	require.NoError(t, err)

	// Global cache should still exist
	globalValid, _ = db.IsCacheValid(ctx, models.CacheTypeChannel, "channel123", nil)
	assert.True(t, globalValid)

	// User cache should be gone
	userValid, _ = db.IsCacheValid(ctx, models.CacheTypeChannel, "channel123", &user.ID)
	assert.False(t, userValid)
}

func TestCacheDifferentTypes_SameEntity(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	entityID := "123456"

	// Set cache for same entity ID but different types
	err = db.SetCacheMetadata(ctx, models.CacheTypeGuild, entityID, nil, 1*time.Hour)
	require.NoError(t, err)

	err = db.SetCacheMetadata(ctx, models.CacheTypeChannel, entityID, nil, 1*time.Hour)
	require.NoError(t, err)

	// Both should exist independently
	guildValid, _ := db.IsCacheValid(ctx, models.CacheTypeGuild, entityID, nil)
	assert.True(t, guildValid)

	channelValid, _ := db.IsCacheValid(ctx, models.CacheTypeChannel, entityID, nil)
	assert.True(t, channelValid)

	// Invalidate one type
	err = db.InvalidateCache(ctx, models.CacheTypeGuild, entityID, nil)
	require.NoError(t, err)

	// Only guild cache should be gone
	guildValid, _ = db.IsCacheValid(ctx, models.CacheTypeGuild, entityID, nil)
	assert.False(t, guildValid)

	channelValid, _ = db.IsCacheValid(ctx, models.CacheTypeChannel, entityID, nil)
	assert.True(t, channelValid)
}
