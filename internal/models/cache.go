package models

import (
	"database/sql"
	"time"
)

// CacheType represents the type of cached data
type CacheType string

const (
	CacheTypeGuild   CacheType = "guild"
	CacheTypeChannel CacheType = "channel"
	CacheTypeMessage CacheType = "message"
)

// CacheMetadata tracks cache TTL for Discord resources
type CacheMetadata struct {
	ID            int64        `json:"id"`
	CacheType     CacheType    `json:"cache_type"`
	EntityID      string       `json:"entity_id"`
	UserID        sql.NullInt64 `json:"user_id"`
	LastFetchedAt time.Time    `json:"last_fetched_at"`
	ExpiresAt     time.Time    `json:"expires_at"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// IsExpired checks if the cache entry has expired
func (c *CacheMetadata) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// IsValid checks if the cache entry is still valid
func (c *CacheMetadata) IsValid() bool {
	return !c.IsExpired()
}
