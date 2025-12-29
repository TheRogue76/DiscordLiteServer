package websocket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	messagev1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/message/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// DiscordMessage represents a message from Discord Gateway
type DiscordMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
	Author    struct {
		ID            string `json:"id"`
		Username      string `json:"username"`
		Discriminator string `json:"discriminator"`
		Avatar        string `json:"avatar"`
	} `json:"author"`
	Content         string    `json:"content"`
	Timestamp       string    `json:"timestamp"`
	EditedTimestamp *string   `json:"edited_timestamp"`
	Type            int       `json:"type"`
	Attachments     []Attachment `json:"attachments"`
	MessageReference *struct {
		MessageID string `json:"message_id"`
	} `json:"message_reference"`
}

// Attachment represents a Discord message attachment
type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url"`
	Height      *int   `json:"height"`
	Width       *int   `json:"width"`
	ContentType string `json:"content_type"`
}

// DiscordMessageDelete represents a MESSAGE_DELETE event
type DiscordMessageDelete struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
}

// HandleMessageCreate processes a MESSAGE_CREATE event
func HandleMessageCreate(ctx context.Context, manager *Manager, db *database.DB, logger *zap.Logger, data json.RawMessage) error {
	var discordMsg DiscordMessage
	if err := json.Unmarshal(data, &discordMsg); err != nil {
		return fmt.Errorf("failed to unmarshal MESSAGE_CREATE: %w", err)
	}

	logger.Debug("received MESSAGE_CREATE event",
		zap.String("message_id", discordMsg.ID),
		zap.String("channel_id", discordMsg.ChannelID),
	)

	// Get channel from database
	channel, err := db.GetChannelByDiscordID(ctx, discordMsg.ChannelID)
	if err != nil {
		logger.Debug("channel not in database, skipping message",
			zap.String("channel_id", discordMsg.ChannelID),
		)
		return nil // Not an error, just not tracking this channel
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, discordMsg.Timestamp)
	if err != nil {
		logger.Warn("failed to parse timestamp", zap.Error(err))
		timestamp = time.Now()
	}

	// Parse edited timestamp if present
	var editedTimestamp sql.NullTime
	if discordMsg.EditedTimestamp != nil {
		editedTime, err := time.Parse(time.RFC3339, *discordMsg.EditedTimestamp)
		if err == nil {
			editedTimestamp = sql.NullTime{Time: editedTime, Valid: true}
		}
	}

	// Get referenced message ID if present
	var referencedMessageID sql.NullString
	if discordMsg.MessageReference != nil {
		referencedMessageID = sql.NullString{String: discordMsg.MessageReference.MessageID, Valid: true}
	}

	// Store message in database
	message := &models.Message{
		DiscordMessageID:    discordMsg.ID,
		ChannelID:           channel.ID,
		AuthorID:            discordMsg.Author.ID,
		AuthorUsername:      discordMsg.Author.Username,
		AuthorAvatar:        sql.NullString{String: discordMsg.Author.Avatar, Valid: discordMsg.Author.Avatar != ""},
		Content:             sql.NullString{String: discordMsg.Content, Valid: discordMsg.Content != ""},
		Timestamp:           timestamp,
		EditedTimestamp:     editedTimestamp,
		MessageType:         models.MessageType(discordMsg.Type),
		ReferencedMessageID: referencedMessageID,
	}

	if err := db.CreateOrUpdateMessage(ctx, message); err != nil {
		logger.Error("failed to store message", zap.Error(err))
		return err
	}

	// Store attachments
	for _, att := range discordMsg.Attachments {
		attachment := &models.MessageAttachment{
			MessageID:    message.ID,
			AttachmentID: att.ID,
			Filename:     att.Filename,
			URL:          att.URL,
			ProxyURL:     sql.NullString{String: att.ProxyURL, Valid: att.ProxyURL != ""},
			SizeBytes:    att.Size,
			Width:        sql.NullInt64{Int64: int64(*att.Width), Valid: att.Width != nil},
			Height:       sql.NullInt64{Int64: int64(*att.Height), Valid: att.Height != nil},
			ContentType:  sql.NullString{String: att.ContentType, Valid: att.ContentType != ""},
		}

		if err := db.CreateMessageAttachment(ctx, attachment); err != nil {
			logger.Error("failed to store attachment", zap.Error(err))
		}
	}

	// Convert to proto and broadcast
	protoMsg := convertToProtoMessage(&discordMsg, message)
	event := &messagev1.MessageEvent{
		EventType: messagev1.MessageEventType_MESSAGE_EVENT_TYPE_CREATE,
		Message:   protoMsg,
		Timestamp: time.Now().UnixMilli(),
	}

	manager.BroadcastEvent(discordMsg.ChannelID, event)

	logger.Info("processed MESSAGE_CREATE event",
		zap.String("message_id", discordMsg.ID),
		zap.String("channel_id", discordMsg.ChannelID),
	)

	return nil
}

// HandleMessageUpdate processes a MESSAGE_UPDATE event
func HandleMessageUpdate(ctx context.Context, manager *Manager, db *database.DB, logger *zap.Logger, data json.RawMessage) error {
	var discordMsg DiscordMessage
	if err := json.Unmarshal(data, &discordMsg); err != nil {
		return fmt.Errorf("failed to unmarshal MESSAGE_UPDATE: %w", err)
	}

	logger.Debug("received MESSAGE_UPDATE event",
		zap.String("message_id", discordMsg.ID),
		zap.String("channel_id", discordMsg.ChannelID),
	)

	// Get existing message from database
	existingMsg, err := db.GetMessageByDiscordID(ctx, discordMsg.ID)
	if err != nil {
		logger.Debug("message not in database, skipping update",
			zap.String("message_id", discordMsg.ID),
		)
		return nil
	}

	// Get channel from database (verify channel exists and we're tracking it)
	_, err = db.GetChannelByDiscordID(ctx, discordMsg.ChannelID)
	if err != nil {
		logger.Debug("channel not in database, skipping message update",
			zap.String("channel_id", discordMsg.ChannelID),
		)
		return nil
	}

	// Parse edited timestamp
	var editedTimestamp sql.NullTime
	if discordMsg.EditedTimestamp != nil {
		editedTime, err := time.Parse(time.RFC3339, *discordMsg.EditedTimestamp)
		if err == nil {
			editedTimestamp = sql.NullTime{Time: editedTime, Valid: true}
		}
	}

	// Update message
	existingMsg.Content = sql.NullString{String: discordMsg.Content, Valid: discordMsg.Content != ""}
	existingMsg.EditedTimestamp = editedTimestamp

	if err := db.CreateOrUpdateMessage(ctx, existingMsg); err != nil {
		logger.Error("failed to update message", zap.Error(err))
		return err
	}

	// Convert to proto and broadcast
	protoMsg := convertToProtoMessage(&discordMsg, existingMsg)
	event := &messagev1.MessageEvent{
		EventType: messagev1.MessageEventType_MESSAGE_EVENT_TYPE_UPDATE,
		Message:   protoMsg,
		Timestamp: time.Now().UnixMilli(),
	}

	manager.BroadcastEvent(discordMsg.ChannelID, event)

	logger.Info("processed MESSAGE_UPDATE event",
		zap.String("message_id", discordMsg.ID),
		zap.String("channel_id", discordMsg.ChannelID),
	)

	return nil
}

// HandleMessageDelete processes a MESSAGE_DELETE event
func HandleMessageDelete(ctx context.Context, manager *Manager, db *database.DB, logger *zap.Logger, data json.RawMessage) error {
	var deleteEvent DiscordMessageDelete
	if err := json.Unmarshal(data, &deleteEvent); err != nil {
		return fmt.Errorf("failed to unmarshal MESSAGE_DELETE: %w", err)
	}

	logger.Debug("received MESSAGE_DELETE event",
		zap.String("message_id", deleteEvent.ID),
		zap.String("channel_id", deleteEvent.ChannelID),
	)

	// Get message before deleting (for broadcasting)
	existingMsg, err := db.GetMessageByDiscordID(ctx, deleteEvent.ID)
	if err != nil {
		logger.Debug("message not in database, skipping delete",
			zap.String("message_id", deleteEvent.ID),
		)
		return nil
	}

	// Delete message from database
	if err := db.DeleteMessage(ctx, deleteEvent.ID); err != nil {
		logger.Error("failed to delete message", zap.Error(err))
		return err
	}

	// Convert to proto and broadcast
	protoMsg := &messagev1.Message{
		DiscordMessageId: existingMsg.DiscordMessageID,
		ChannelId:        deleteEvent.ChannelID,
		Author: &messagev1.MessageAuthor{
			DiscordId: existingMsg.AuthorID,
			Username:  existingMsg.AuthorUsername,
			Avatar:    existingMsg.AuthorAvatar.String,
		},
		Content:   existingMsg.Content.String,
		Timestamp: existingMsg.Timestamp.UnixMilli(),
		Type:      messagev1.MessageType(existingMsg.MessageType),
	}

	event := &messagev1.MessageEvent{
		EventType: messagev1.MessageEventType_MESSAGE_EVENT_TYPE_DELETE,
		Message:   protoMsg,
		Timestamp: time.Now().UnixMilli(),
	}

	manager.BroadcastEvent(deleteEvent.ChannelID, event)

	logger.Info("processed MESSAGE_DELETE event",
		zap.String("message_id", deleteEvent.ID),
		zap.String("channel_id", deleteEvent.ChannelID),
	)

	return nil
}

// convertToProtoMessage converts a Discord message to proto format
func convertToProtoMessage(discordMsg *DiscordMessage, dbMsg *models.Message) *messagev1.Message {
	protoMsg := &messagev1.Message{
		DiscordMessageId: discordMsg.ID,
		ChannelId:        discordMsg.ChannelID,
		Author: &messagev1.MessageAuthor{
			DiscordId:     discordMsg.Author.ID,
			Username:      discordMsg.Author.Username,
			Discriminator: discordMsg.Author.Discriminator,
			Avatar:        discordMsg.Author.Avatar,
		},
		Content:   discordMsg.Content,
		Timestamp: dbMsg.Timestamp.UnixMilli(),
		Type:      messagev1.MessageType(discordMsg.Type),
	}

	// Add edited timestamp if present
	if dbMsg.EditedTimestamp.Valid {
		editedMs := dbMsg.EditedTimestamp.Time.UnixMilli()
		protoMsg.EditedTimestamp = &editedMs
	}

	// Add referenced message if present
	if dbMsg.ReferencedMessageID.Valid {
		protoMsg.ReferencedMessageId = &dbMsg.ReferencedMessageID.String
	}

	// Add attachments
	protoAttachments := make([]*messagev1.MessageAttachment, 0, len(discordMsg.Attachments))
	for _, att := range discordMsg.Attachments {
		protoAtt := &messagev1.MessageAttachment{
			AttachmentId: att.ID,
			Filename:     att.Filename,
			Url:          att.URL,
			ProxyUrl:     att.ProxyURL,
			SizeBytes:    int32(att.Size),
			ContentType:  att.ContentType,
		}

		if att.Width != nil {
			width := int32(*att.Width)
			protoAtt.Width = &width
		}
		if att.Height != nil {
			height := int32(*att.Height)
			protoAtt.Height = &height
		}

		protoAttachments = append(protoAttachments, protoAtt)
	}
	protoMsg.Attachments = protoAttachments

	return protoMsg
}
