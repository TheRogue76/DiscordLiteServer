package grpc

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// CacheManager handles cache operations for Discord resources
type CacheManager struct {
	db     *database.DB
	logger *zap.Logger
}

// NewCacheManager creates a new cache manager
func NewCacheManager(db *database.DB, logger *zap.Logger) *CacheManager {
	return &CacheManager{
		db:     db,
		logger: logger,
	}
}

// CheckGuildCache checks if guild data is cached and valid for a user
func (cm *CacheManager) CheckGuildCache(ctx context.Context, userID int64) (bool, error) {
	// For guilds, we check if ANY guild cache for this user is valid
	// This is a simple approach - in production you might want more granular checks
	valid, err := cm.db.IsCacheValid(ctx, models.CacheTypeGuild, "user_guilds", &userID)
	if err != nil {
		cm.logger.Debug("guild cache check failed", zap.Error(err))
		return false, nil
	}

	if valid {
		cm.logger.Debug("guild cache hit", zap.Int64("user_id", userID))
	} else {
		cm.logger.Debug("guild cache miss", zap.Int64("user_id", userID))
	}

	return valid, nil
}

// SetGuildCache marks guild data as cached with 1 hour TTL
func (cm *CacheManager) SetGuildCache(ctx context.Context, userID int64) error {
	err := cm.db.SetCacheMetadata(ctx, models.CacheTypeGuild, "user_guilds", &userID, 1*time.Hour)
	if err != nil {
		return err
	}

	cm.logger.Debug("guild cache set", zap.Int64("user_id", userID))
	return nil
}

// CheckChannelCache checks if channel data is cached and valid for a guild
func (cm *CacheManager) CheckChannelCache(ctx context.Context, guildID string, userID int64) (bool, error) {
	valid, err := cm.db.IsCacheValid(ctx, models.CacheTypeChannel, guildID, &userID)
	if err != nil {
		cm.logger.Debug("channel cache check failed", zap.Error(err))
		return false, nil
	}

	if valid {
		cm.logger.Debug("channel cache hit", zap.String("guild_id", guildID))
	} else {
		cm.logger.Debug("channel cache miss", zap.String("guild_id", guildID))
	}

	return valid, nil
}

// SetChannelCache marks channel data as cached with 30 minute TTL
func (cm *CacheManager) SetChannelCache(ctx context.Context, guildID string, userID int64) error {
	err := cm.db.SetCacheMetadata(ctx, models.CacheTypeChannel, guildID, &userID, 30*time.Minute)
	if err != nil {
		return err
	}

	cm.logger.Debug("channel cache set", zap.String("guild_id", guildID))
	return nil
}

// CheckMessageCache checks if message data is cached and valid for a channel
func (cm *CacheManager) CheckMessageCache(ctx context.Context, channelID string, userID int64) (bool, error) {
	valid, err := cm.db.IsCacheValid(ctx, models.CacheTypeMessage, channelID, &userID)
	if err != nil {
		cm.logger.Debug("message cache check failed", zap.Error(err))
		return false, nil
	}

	if valid {
		cm.logger.Debug("message cache hit", zap.String("channel_id", channelID))
	} else {
		cm.logger.Debug("message cache miss", zap.String("channel_id", channelID))
	}

	return valid, nil
}

// SetMessageCache marks message data as cached with 5 minute TTL
func (cm *CacheManager) SetMessageCache(ctx context.Context, channelID string, userID int64) error {
	err := cm.db.SetCacheMetadata(ctx, models.CacheTypeMessage, channelID, &userID, 5*time.Minute)
	if err != nil {
		return err
	}

	cm.logger.Debug("message cache set", zap.String("channel_id", channelID))
	return nil
}

// InvalidateChannelCache invalidates cache for a specific channel
func (cm *CacheManager) InvalidateChannelCache(ctx context.Context, channelID string) error {
	err := cm.db.InvalidateCache(ctx, models.CacheTypeMessage, channelID, nil)
	if err != nil {
		cm.logger.Warn("failed to invalidate channel cache",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return err
	}

	cm.logger.Debug("invalidated channel cache", zap.String("channel_id", channelID))
	return nil
}

// InvalidateGuildCache invalidates cache for a specific guild
func (cm *CacheManager) InvalidateGuildCache(ctx context.Context, guildID string) error {
	// Invalidate both channel and message caches for this guild
	err := cm.db.InvalidateCache(ctx, models.CacheTypeChannel, guildID, nil)
	if err != nil {
		cm.logger.Warn("failed to invalidate guild channel cache",
			zap.String("guild_id", guildID),
			zap.Error(err),
		)
	}

	cm.logger.Debug("invalidated guild cache", zap.String("guild_id", guildID))
	return nil
}
