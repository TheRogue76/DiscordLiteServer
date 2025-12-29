package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// CreateOrUpdateGuild inserts or updates a guild in the database
func (db *DB) CreateOrUpdateGuild(ctx context.Context, guild *models.Guild) error {
	query := `
		INSERT INTO guilds (discord_guild_id, name, icon, owner_id, permissions, features)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (discord_guild_id) DO UPDATE
		SET name = EXCLUDED.name,
		    icon = EXCLUDED.icon,
		    owner_id = EXCLUDED.owner_id,
		    permissions = EXCLUDED.permissions,
		    features = EXCLUDED.features,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRowContext(
		ctx,
		query,
		guild.DiscordGuildID,
		guild.Name,
		guild.Icon,
		guild.OwnerID,
		guild.Permissions,
		guild.Features,
	).Scan(&guild.ID, &guild.CreatedAt, &guild.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create/update guild: %w", err)
	}

	return nil
}

// GetGuildByID retrieves a guild by its internal ID
func (db *DB) GetGuildByID(ctx context.Context, id int64) (*models.Guild, error) {
	query := `
		SELECT id, discord_guild_id, name, icon, owner_id, permissions, features, created_at, updated_at
		FROM guilds
		WHERE id = $1
	`

	var guild models.Guild
	err := db.QueryRowContext(ctx, query, id).Scan(
		&guild.ID,
		&guild.DiscordGuildID,
		&guild.Name,
		&guild.Icon,
		&guild.OwnerID,
		&guild.Permissions,
		&guild.Features,
		&guild.CreatedAt,
		&guild.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("guild not found")
		}
		return nil, fmt.Errorf("failed to get guild: %w", err)
	}

	return &guild, nil
}

// GetGuildByDiscordID retrieves a guild by its Discord guild ID
func (db *DB) GetGuildByDiscordID(ctx context.Context, discordGuildID string) (*models.Guild, error) {
	query := `
		SELECT id, discord_guild_id, name, icon, owner_id, permissions, features, created_at, updated_at
		FROM guilds
		WHERE discord_guild_id = $1
	`

	var guild models.Guild
	err := db.QueryRowContext(ctx, query, discordGuildID).Scan(
		&guild.ID,
		&guild.DiscordGuildID,
		&guild.Name,
		&guild.Icon,
		&guild.OwnerID,
		&guild.Permissions,
		&guild.Features,
		&guild.CreatedAt,
		&guild.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("guild not found")
		}
		return nil, fmt.Errorf("failed to get guild: %w", err)
	}

	return &guild, nil
}

// GetGuildsByUserID retrieves all guilds for a user
func (db *DB) GetGuildsByUserID(ctx context.Context, userID int64) ([]*models.Guild, error) {
	query := `
		SELECT g.id, g.discord_guild_id, g.name, g.icon, g.owner_id, g.permissions, g.features, g.created_at, g.updated_at
		FROM guilds g
		INNER JOIN user_guilds ug ON g.id = ug.guild_id
		WHERE ug.user_id = $1
		ORDER BY g.name ASC
	`

	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query guilds: %w", err)
	}
	defer rows.Close()

	var guilds []*models.Guild
	for rows.Next() {
		var guild models.Guild
		err := rows.Scan(
			&guild.ID,
			&guild.DiscordGuildID,
			&guild.Name,
			&guild.Icon,
			&guild.OwnerID,
			&guild.Permissions,
			&guild.Features,
			&guild.CreatedAt,
			&guild.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan guild: %w", err)
		}
		guilds = append(guilds, &guild)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating guilds: %w", err)
	}

	return guilds, nil
}

// CreateUserGuild adds a user to a guild (many-to-many relationship)
func (db *DB) CreateUserGuild(ctx context.Context, userID, guildID int64) error {
	query := `
		INSERT INTO user_guilds (user_id, guild_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, guild_id) DO NOTHING
	`

	_, err := db.ExecContext(ctx, query, userID, guildID)
	if err != nil {
		return fmt.Errorf("failed to create user-guild relationship: %w", err)
	}

	return nil
}

// DeleteUserGuild removes a user from a guild
func (db *DB) DeleteUserGuild(ctx context.Context, userID, guildID int64) error {
	query := `DELETE FROM user_guilds WHERE user_id = $1 AND guild_id = $2`

	result, err := db.ExecContext(ctx, query, userID, guildID)
	if err != nil {
		return fmt.Errorf("failed to delete user-guild relationship: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user-guild relationship not found")
	}

	return nil
}

// DeleteGuild removes a guild and all associated data (cascades to user_guilds, channels, messages)
func (db *DB) DeleteGuild(ctx context.Context, guildID int64) error {
	query := `DELETE FROM guilds WHERE id = $1`

	result, err := db.ExecContext(ctx, query, guildID)
	if err != nil {
		return fmt.Errorf("failed to delete guild: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guild not found")
	}

	return nil
}

// UserHasGuildAccess checks if a user has access to a guild
func (db *DB) UserHasGuildAccess(ctx context.Context, userID int64, discordGuildID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_guilds ug
			INNER JOIN guilds g ON ug.guild_id = g.id
			WHERE ug.user_id = $1 AND g.discord_guild_id = $2
		)
	`

	var exists bool
	err := db.QueryRowContext(ctx, query, userID, discordGuildID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check guild access: %w", err)
	}

	return exists, nil
}
