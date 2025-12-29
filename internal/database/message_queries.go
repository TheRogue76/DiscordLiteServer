package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// CreateOrUpdateMessage inserts or updates a message in the database
func (db *DB) CreateOrUpdateMessage(ctx context.Context, message *models.Message) error {
	query := `
		INSERT INTO messages (
			discord_message_id, channel_id, author_id, author_username, author_avatar,
			content, timestamp, edited_timestamp, message_type, referenced_message_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (discord_message_id) DO UPDATE
		SET content = EXCLUDED.content,
		    edited_timestamp = EXCLUDED.edited_timestamp,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRowContext(
		ctx,
		query,
		message.DiscordMessageID,
		message.ChannelID,
		message.AuthorID,
		message.AuthorUsername,
		message.AuthorAvatar,
		message.Content,
		message.Timestamp,
		message.EditedTimestamp,
		message.MessageType,
		message.ReferencedMessageID,
	).Scan(&message.ID, &message.CreatedAt, &message.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create/update message: %w", err)
	}

	return nil
}

// CreateMessageAttachment inserts a message attachment
func (db *DB) CreateMessageAttachment(ctx context.Context, attachment *models.MessageAttachment) error {
	query := `
		INSERT INTO message_attachments (
			message_id, attachment_id, filename, url, proxy_url,
			size_bytes, width, height, content_type
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (message_id, attachment_id) DO UPDATE
		SET filename = EXCLUDED.filename,
		    url = EXCLUDED.url,
		    proxy_url = EXCLUDED.proxy_url,
		    size_bytes = EXCLUDED.size_bytes,
		    width = EXCLUDED.width,
		    height = EXCLUDED.height,
		    content_type = EXCLUDED.content_type
		RETURNING id, created_at
	`

	err := db.QueryRowContext(
		ctx,
		query,
		attachment.MessageID,
		attachment.AttachmentID,
		attachment.Filename,
		attachment.URL,
		attachment.ProxyURL,
		attachment.SizeBytes,
		attachment.Width,
		attachment.Height,
		attachment.ContentType,
	).Scan(&attachment.ID, &attachment.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create message attachment: %w", err)
	}

	return nil
}

// GetMessageByID retrieves a message by its internal ID
func (db *DB) GetMessageByID(ctx context.Context, id int64) (*models.Message, error) {
	query := `
		SELECT id, discord_message_id, channel_id, author_id, author_username, author_avatar,
		       content, timestamp, edited_timestamp, message_type, referenced_message_id,
		       created_at, updated_at
		FROM messages
		WHERE id = $1
	`

	var message models.Message
	err := db.QueryRowContext(ctx, query, id).Scan(
		&message.ID,
		&message.DiscordMessageID,
		&message.ChannelID,
		&message.AuthorID,
		&message.AuthorUsername,
		&message.AuthorAvatar,
		&message.Content,
		&message.Timestamp,
		&message.EditedTimestamp,
		&message.MessageType,
		&message.ReferencedMessageID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &message, nil
}

// GetMessageByDiscordID retrieves a message by its Discord message ID
func (db *DB) GetMessageByDiscordID(ctx context.Context, discordMessageID string) (*models.Message, error) {
	query := `
		SELECT id, discord_message_id, channel_id, author_id, author_username, author_avatar,
		       content, timestamp, edited_timestamp, message_type, referenced_message_id,
		       created_at, updated_at
		FROM messages
		WHERE discord_message_id = $1
	`

	var message models.Message
	err := db.QueryRowContext(ctx, query, discordMessageID).Scan(
		&message.ID,
		&message.DiscordMessageID,
		&message.ChannelID,
		&message.AuthorID,
		&message.AuthorUsername,
		&message.AuthorAvatar,
		&message.Content,
		&message.Timestamp,
		&message.EditedTimestamp,
		&message.MessageType,
		&message.ReferencedMessageID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &message, nil
}

// GetMessagesByChannelID retrieves messages for a channel with pagination
// Pagination: limit (max 100), before (older than message ID), after (newer than message ID)
func (db *DB) GetMessagesByChannelID(ctx context.Context, channelID int64, limit int, before, after string) ([]*models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var query string
	var args []interface{}

	if before != "" {
		// Get messages older than 'before' message ID
		query = `
			SELECT id, discord_message_id, channel_id, author_id, author_username, author_avatar,
			       content, timestamp, edited_timestamp, message_type, referenced_message_id,
			       created_at, updated_at
			FROM messages
			WHERE channel_id = $1 AND timestamp < (
				SELECT timestamp FROM messages WHERE discord_message_id = $2
			)
			ORDER BY timestamp DESC
			LIMIT $3
		`
		args = []interface{}{channelID, before, limit}
	} else if after != "" {
		// Get messages newer than 'after' message ID
		query = `
			SELECT id, discord_message_id, channel_id, author_id, author_username, author_avatar,
			       content, timestamp, edited_timestamp, message_type, referenced_message_id,
			       created_at, updated_at
			FROM messages
			WHERE channel_id = $1 AND timestamp > (
				SELECT timestamp FROM messages WHERE discord_message_id = $2
			)
			ORDER BY timestamp ASC
			LIMIT $3
		`
		args = []interface{}{channelID, after, limit}
	} else {
		// Get most recent messages
		query = `
			SELECT id, discord_message_id, channel_id, author_id, author_username, author_avatar,
			       content, timestamp, edited_timestamp, message_type, referenced_message_id,
			       created_at, updated_at
			FROM messages
			WHERE channel_id = $1
			ORDER BY timestamp DESC
			LIMIT $2
		`
		args = []interface{}{channelID, limit}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var message models.Message
		err := rows.Scan(
			&message.ID,
			&message.DiscordMessageID,
			&message.ChannelID,
			&message.AuthorID,
			&message.AuthorUsername,
			&message.AuthorAvatar,
			&message.Content,
			&message.Timestamp,
			&message.EditedTimestamp,
			&message.MessageType,
			&message.ReferencedMessageID,
			&message.CreatedAt,
			&message.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, &message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// GetMessageAttachmentsByMessageID retrieves all attachments for a message
func (db *DB) GetMessageAttachmentsByMessageID(ctx context.Context, messageID int64) ([]*models.MessageAttachment, error) {
	query := `
		SELECT id, message_id, attachment_id, filename, url, proxy_url,
		       size_bytes, width, height, content_type, created_at
		FROM message_attachments
		WHERE message_id = $1
		ORDER BY id ASC
	`

	rows, err := db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*models.MessageAttachment
	for rows.Next() {
		var attachment models.MessageAttachment
		err := rows.Scan(
			&attachment.ID,
			&attachment.MessageID,
			&attachment.AttachmentID,
			&attachment.Filename,
			&attachment.URL,
			&attachment.ProxyURL,
			&attachment.SizeBytes,
			&attachment.Width,
			&attachment.Height,
			&attachment.ContentType,
			&attachment.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}
		attachments = append(attachments, &attachment)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attachments: %w", err)
	}

	return attachments, nil
}

// DeleteMessage removes a message and its attachments (cascade)
func (db *DB) DeleteMessage(ctx context.Context, discordMessageID string) error {
	query := `DELETE FROM messages WHERE discord_message_id = $1`

	result, err := db.ExecContext(ctx, query, discordMessageID)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message not found")
	}

	return nil
}

// GetMessageCountByChannelID returns the total number of messages in a channel
func (db *DB) GetMessageCountByChannelID(ctx context.Context, channelID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM messages WHERE channel_id = $1`

	var count int64
	err := db.QueryRowContext(ctx, query, channelID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}

	return count, nil
}
