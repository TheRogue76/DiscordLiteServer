package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// SetCacheMetadata creates or updates cache metadata for an entity
func (db *DB) SetCacheMetadata(ctx context.Context, cacheType models.CacheType, entityID string, userID *int64, ttlDuration time.Duration) error {
	now := time.Now()
	expiresAt := now.Add(ttlDuration)

	query := `
		INSERT INTO cache_metadata (cache_type, entity_id, user_id, last_fetched_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (cache_type, entity_id, user_id) DO UPDATE
		SET last_fetched_at = EXCLUDED.last_fetched_at,
		    expires_at = EXCLUDED.expires_at,
		    updated_at = NOW()
	`

	var userIDVal interface{}
	if userID != nil {
		userIDVal = *userID
	} else {
		userIDVal = nil
	}

	_, err := db.ExecContext(ctx, query, cacheType, entityID, userIDVal, now, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to set cache metadata: %w", err)
	}

	return nil
}

// GetCacheMetadata retrieves cache metadata for an entity
func (db *DB) GetCacheMetadata(ctx context.Context, cacheType models.CacheType, entityID string, userID *int64) (*models.CacheMetadata, error) {
	query := `
		SELECT id, cache_type, entity_id, user_id, last_fetched_at, expires_at, created_at, updated_at
		FROM cache_metadata
		WHERE cache_type = $1 AND entity_id = $2 AND (user_id = $3 OR (user_id IS NULL AND $3 IS NULL))
	`

	var userIDVal interface{}
	if userID != nil {
		userIDVal = *userID
	} else {
		userIDVal = nil
	}

	var cache models.CacheMetadata
	err := db.QueryRowContext(ctx, query, cacheType, entityID, userIDVal).Scan(
		&cache.ID,
		&cache.CacheType,
		&cache.EntityID,
		&cache.UserID,
		&cache.LastFetchedAt,
		&cache.ExpiresAt,
		&cache.CreatedAt,
		&cache.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cache metadata not found")
		}
		return nil, fmt.Errorf("failed to get cache metadata: %w", err)
	}

	return &cache, nil
}

// IsCacheValid checks if cache is valid (exists and not expired)
func (db *DB) IsCacheValid(ctx context.Context, cacheType models.CacheType, entityID string, userID *int64) (bool, error) {
	cache, err := db.GetCacheMetadata(ctx, cacheType, entityID, userID)
	if err != nil {
		// Cache doesn't exist
		return false, nil
	}

	return cache.IsValid(), nil
}

// InvalidateCache removes cache metadata for an entity
func (db *DB) InvalidateCache(ctx context.Context, cacheType models.CacheType, entityID string, userID *int64) error {
	query := `
		DELETE FROM cache_metadata
		WHERE cache_type = $1 AND entity_id = $2 AND (user_id = $3 OR (user_id IS NULL AND $3 IS NULL))
	`

	var userIDVal interface{}
	if userID != nil {
		userIDVal = *userID
	} else {
		userIDVal = nil
	}

	_, err := db.ExecContext(ctx, query, cacheType, entityID, userIDVal)
	if err != nil {
		return fmt.Errorf("failed to invalidate cache: %w", err)
	}

	return nil
}

// InvalidateCacheByType removes all cache metadata of a specific type
func (db *DB) InvalidateCacheByType(ctx context.Context, cacheType models.CacheType) error {
	query := `DELETE FROM cache_metadata WHERE cache_type = $1`

	result, err := db.ExecContext(ctx, query, cacheType)
	if err != nil {
		return fmt.Errorf("failed to invalidate cache by type: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		fmt.Printf("Invalidated %d cache entries of type %s\n", rowsAffected, cacheType)
	}

	return nil
}

// InvalidateCacheForUser removes all cache metadata for a specific user
func (db *DB) InvalidateCacheForUser(ctx context.Context, userID int64) error {
	query := `DELETE FROM cache_metadata WHERE user_id = $1`

	result, err := db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to invalidate cache for user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		fmt.Printf("Invalidated %d cache entries for user %d\n", rowsAffected, userID)
	}

	return nil
}

// CleanupExpiredCache removes expired cache entries
func (db *DB) CleanupExpiredCache(ctx context.Context) error {
	query := `DELETE FROM cache_metadata WHERE expires_at < NOW()`

	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired cache: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		fmt.Printf("Cleaned up %d expired cache entries\n", rowsAffected)
	}

	return nil
}

// StartCacheCleanupJob starts a background job that periodically cleans up expired cache
func (db *DB) StartCacheCleanupJob(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping cache cleanup job")
			return
		case <-ticker.C:
			err := db.CleanupExpiredCache(ctx)
			if err != nil {
				fmt.Printf("Error during cache cleanup: %v\n", err)
			}
		}
	}
}

// GetCacheStats returns statistics about the cache
func (db *DB) GetCacheStats(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT
			cache_type,
			COUNT(*) as count,
			SUM(CASE WHEN expires_at > NOW() THEN 1 ELSE 0 END) as valid_count,
			SUM(CASE WHEN expires_at <= NOW() THEN 1 ELSE 0 END) as expired_count
		FROM cache_metadata
		GROUP BY cache_type
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var cacheType string
		var count, validCount, expiredCount int64
		err := rows.Scan(&cacheType, &count, &validCount, &expiredCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan cache stats: %w", err)
		}
		stats[cacheType+"_total"] = count
		stats[cacheType+"_valid"] = validCount
		stats[cacheType+"_expired"] = expiredCount
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cache stats: %w", err)
	}

	return stats, nil
}
