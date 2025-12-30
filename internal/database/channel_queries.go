package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// CreateOrUpdateChannel inserts or updates a channel in the database
func (db *DB) CreateOrUpdateChannel(ctx context.Context, channel *models.Channel) error {
	query := `
		INSERT INTO channels (discord_channel_id, guild_id, name, type, position, parent_id, topic, nsfw, last_message_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (discord_channel_id) DO UPDATE
		SET guild_id = EXCLUDED.guild_id,
		    name = EXCLUDED.name,
		    type = EXCLUDED.type,
		    position = EXCLUDED.position,
		    parent_id = EXCLUDED.parent_id,
		    topic = EXCLUDED.topic,
		    nsfw = EXCLUDED.nsfw,
		    last_message_id = EXCLUDED.last_message_id,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRowContext(
		ctx,
		query,
		channel.DiscordChannelID,
		channel.GuildID,
		channel.Name,
		channel.Type,
		channel.Position,
		channel.ParentID,
		channel.Topic,
		channel.NSFW,
		channel.LastMessageID,
	).Scan(&channel.ID, &channel.CreatedAt, &channel.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create/update channel: %w", err)
	}

	return nil
}

// GetChannelByID retrieves a channel by its internal ID
func (db *DB) GetChannelByID(ctx context.Context, id int64) (*models.Channel, error) {
	query := `
		SELECT id, discord_channel_id, guild_id, name, type, position, parent_id, topic, nsfw, last_message_id, created_at, updated_at
		FROM channels
		WHERE id = $1
	`

	var channel models.Channel
	err := db.QueryRowContext(ctx, query, id).Scan(
		&channel.ID,
		&channel.DiscordChannelID,
		&channel.GuildID,
		&channel.Name,
		&channel.Type,
		&channel.Position,
		&channel.ParentID,
		&channel.Topic,
		&channel.NSFW,
		&channel.LastMessageID,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("channel not found")
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return &channel, nil
}

// GetChannelByDiscordID retrieves a channel by its Discord channel ID
func (db *DB) GetChannelByDiscordID(ctx context.Context, discordChannelID string) (*models.Channel, error) {
	query := `
		SELECT id, discord_channel_id, guild_id, name, type, position, parent_id, topic, nsfw, last_message_id, created_at, updated_at
		FROM channels
		WHERE discord_channel_id = $1
	`

	var channel models.Channel
	err := db.QueryRowContext(ctx, query, discordChannelID).Scan(
		&channel.ID,
		&channel.DiscordChannelID,
		&channel.GuildID,
		&channel.Name,
		&channel.Type,
		&channel.Position,
		&channel.ParentID,
		&channel.Topic,
		&channel.NSFW,
		&channel.LastMessageID,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("channel not found")
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return &channel, nil
}

// GetChannelsByGuildID retrieves all channels for a guild
func (db *DB) GetChannelsByGuildID(ctx context.Context, guildID int64) ([]*models.Channel, error) {
	query := `
		SELECT id, discord_channel_id, guild_id, name, type, position, parent_id, topic, nsfw, last_message_id, created_at, updated_at
		FROM channels
		WHERE guild_id = $1
		ORDER BY position ASC, name ASC
	`

	rows, err := db.QueryContext(ctx, query, guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var channels []*models.Channel
	for rows.Next() {
		var channel models.Channel
		err := rows.Scan(
			&channel.ID,
			&channel.DiscordChannelID,
			&channel.GuildID,
			&channel.Name,
			&channel.Type,
			&channel.Position,
			&channel.ParentID,
			&channel.Topic,
			&channel.NSFW,
			&channel.LastMessageID,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}
		channels = append(channels, &channel)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

// GetChannelsByDiscordGuildID retrieves all channels for a guild by Discord guild ID
func (db *DB) GetChannelsByDiscordGuildID(ctx context.Context, discordGuildID string) ([]*models.Channel, error) {
	query := `
		SELECT c.id, c.discord_channel_id, c.guild_id, c.name, c.type, c.position, c.parent_id, c.topic, c.nsfw, c.last_message_id, c.created_at, c.updated_at
		FROM channels c
		INNER JOIN guilds g ON c.guild_id = g.id
		WHERE g.discord_guild_id = $1
		ORDER BY c.position ASC, c.name ASC
	`

	rows, err := db.QueryContext(ctx, query, discordGuildID)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var channels []*models.Channel
	for rows.Next() {
		var channel models.Channel
		err := rows.Scan(
			&channel.ID,
			&channel.DiscordChannelID,
			&channel.GuildID,
			&channel.Name,
			&channel.Type,
			&channel.Position,
			&channel.ParentID,
			&channel.Topic,
			&channel.NSFW,
			&channel.LastMessageID,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}
		channels = append(channels, &channel)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

// DeleteChannel removes a channel and all associated messages (cascade)
func (db *DB) DeleteChannel(ctx context.Context, channelID int64) error {
	query := `DELETE FROM channels WHERE id = $1`

	result, err := db.ExecContext(ctx, query, channelID)
	if err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("channel not found")
	}

	return nil
}

// UserHasChannelAccess checks if a user has access to a channel (via guild membership)
func (db *DB) UserHasChannelAccess(ctx context.Context, userID int64, discordChannelID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_guilds ug
			INNER JOIN channels c ON ug.guild_id = c.guild_id
			WHERE ug.user_id = $1 AND c.discord_channel_id = $2
		)
	`

	var exists bool
	err := db.QueryRowContext(ctx, query, userID, discordChannelID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check channel access: %w", err)
	}

	return exists, nil
}
