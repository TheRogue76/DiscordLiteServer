package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// MessageType Tests
// ============================================================================

func TestMessageType_Constants(t *testing.T) {
	tests := []struct {
		name        string
		messageType MessageType
		expected    int
	}{
		{"Default message", MessageTypeDefault, 0},
		{"Recipient add", MessageTypeRecipientAdd, 1},
		{"Recipient remove", MessageTypeRecipientRemove, 2},
		{"Call", MessageTypeCall, 3},
		{"Channel name change", MessageTypeChannelNameChange, 4},
		{"Channel icon change", MessageTypeChannelIconChange, 5},
		{"Channel pinned message", MessageTypeChannelPinnedMessage, 6},
		{"Guild member join", MessageTypeGuildMemberJoin, 7},
		{"User premium guild subscription", MessageTypeUserPremiumGuildSubscription, 8},
		{"Premium tier 1", MessageTypeUserPremiumGuildSubscriptionTier1, 9},
		{"Premium tier 2", MessageTypeUserPremiumGuildSubscriptionTier2, 10},
		{"Premium tier 3", MessageTypeUserPremiumGuildSubscriptionTier3, 11},
		{"Channel follow add", MessageTypeChannelFollowAdd, 12},
		{"Guild discovery disqualified", MessageTypeGuildDiscoveryDisqualified, 14},
		{"Guild discovery requalified", MessageTypeGuildDiscoveryRequalified, 15},
		{"Discovery grace period initial warning", MessageTypeGuildDiscoveryGracePeriodInitialWarning, 16},
		{"Discovery grace period final warning", MessageTypeGuildDiscoveryGracePeriodFinalWarning, 17},
		{"Thread created", MessageTypeThreadCreated, 18},
		{"Reply", MessageTypeReply, 19},
		{"Chat input command", MessageTypeChatInputCommand, 20},
		{"Thread starter message", MessageTypeThreadStarterMessage, 21},
		{"Guild invite reminder", MessageTypeGuildInviteReminder, 22},
		{"Context menu command", MessageTypeContextMenuCommand, 23},
		{"Auto moderation action", MessageTypeAutoModerationAction, 24},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.messageType))
		})
	}
}

func TestMessageType_DiscordAPICompliance(t *testing.T) {
	// Verify that message types match Discord API specification
	// Note: Type 13 is missing (reserved/deprecated by Discord)
	assert.Equal(t, 0, int(MessageTypeDefault))
	assert.Equal(t, 14, int(MessageTypeGuildDiscoveryDisqualified)) // Note the jump from 12 to 14
	assert.Equal(t, 24, int(MessageTypeAutoModerationAction))
}

// ============================================================================
// Message Tests
// ============================================================================

func TestMessage_Creation(t *testing.T) {
	now := time.Now()
	msg := &Message{
		ID:                  1,
		DiscordMessageID:    "1234567890",
		ChannelID:           100,
		AuthorID:            "author123",
		AuthorUsername:      "TestUser",
		AuthorAvatar:        sql.NullString{String: "avatar_hash", Valid: true},
		Content:             sql.NullString{String: "Hello, world!", Valid: true},
		Timestamp:           now,
		EditedTimestamp:     sql.NullTime{Valid: false},
		MessageType:         MessageTypeDefault,
		ReferencedMessageID: sql.NullString{Valid: false},
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	assert.Equal(t, int64(1), msg.ID)
	assert.Equal(t, "1234567890", msg.DiscordMessageID)
	assert.Equal(t, int64(100), msg.ChannelID)
	assert.Equal(t, "author123", msg.AuthorID)
	assert.Equal(t, "TestUser", msg.AuthorUsername)
	assert.True(t, msg.AuthorAvatar.Valid)
	assert.Equal(t, "avatar_hash", msg.AuthorAvatar.String)
	assert.True(t, msg.Content.Valid)
	assert.Equal(t, "Hello, world!", msg.Content.String)
	assert.Equal(t, MessageTypeDefault, msg.MessageType)
	assert.False(t, msg.EditedTimestamp.Valid)
	assert.False(t, msg.ReferencedMessageID.Valid)
}

func TestMessage_WithoutAvatar(t *testing.T) {
	msg := &Message{
		ID:               1,
		DiscordMessageID: "1234567890",
		ChannelID:        100,
		AuthorID:         "author123",
		AuthorUsername:   "TestUser",
		AuthorAvatar:     sql.NullString{Valid: false}, // No avatar
		Content:          sql.NullString{String: "Test", Valid: true},
		Timestamp:        time.Now(),
		MessageType:      MessageTypeDefault,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.False(t, msg.AuthorAvatar.Valid, "Message can have no avatar")
}

func TestMessage_WithoutContent(t *testing.T) {
	// Message with no content (e.g., only attachments)
	msg := &Message{
		ID:               1,
		DiscordMessageID: "1234567890",
		ChannelID:        100,
		AuthorID:         "author123",
		AuthorUsername:   "TestUser",
		Content:          sql.NullString{Valid: false}, // No content
		Timestamp:        time.Now(),
		MessageType:      MessageTypeDefault,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.False(t, msg.Content.Valid, "Message can have no content (attachment-only)")
}

func TestMessage_EditedMessage(t *testing.T) {
	now := time.Now()
	edited := now.Add(5 * time.Minute)

	msg := &Message{
		ID:               1,
		DiscordMessageID: "1234567890",
		ChannelID:        100,
		AuthorID:         "author123",
		AuthorUsername:   "TestUser",
		Content:          sql.NullString{String: "Edited message", Valid: true},
		Timestamp:        now,
		EditedTimestamp:  sql.NullTime{Time: edited, Valid: true}, // Edited
		MessageType:      MessageTypeDefault,
		CreatedAt:        now,
		UpdatedAt:        edited,
	}

	assert.True(t, msg.EditedTimestamp.Valid, "Edited message should have edit timestamp")
	assert.True(t, msg.EditedTimestamp.Time.After(msg.Timestamp), "Edit time should be after original timestamp")
}

func TestMessage_ReplyMessage(t *testing.T) {
	msg := &Message{
		ID:                  1,
		DiscordMessageID:    "1234567890",
		ChannelID:           100,
		AuthorID:            "author123",
		AuthorUsername:      "TestUser",
		Content:             sql.NullString{String: "This is a reply", Valid: true},
		Timestamp:           time.Now(),
		MessageType:         MessageTypeReply,
		ReferencedMessageID: sql.NullString{String: "9876543210", Valid: true}, // Replying to this message
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	assert.Equal(t, MessageTypeReply, msg.MessageType)
	assert.True(t, msg.ReferencedMessageID.Valid, "Reply should reference another message")
	assert.Equal(t, "9876543210", msg.ReferencedMessageID.String)
}

func TestMessage_DifferentTypes(t *testing.T) {
	tests := []struct {
		name        string
		messageType MessageType
	}{
		{"Default message", MessageTypeDefault},
		{"Guild member join", MessageTypeGuildMemberJoin},
		{"User boost", MessageTypeUserPremiumGuildSubscription},
		{"Reply", MessageTypeReply},
		{"Command", MessageTypeChatInputCommand},
		{"Thread created", MessageTypeThreadCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				ID:               1,
				DiscordMessageID: "1234567890",
				ChannelID:        100,
				AuthorID:         "author123",
				AuthorUsername:   "TestUser",
				Content:          sql.NullString{String: "Content", Valid: true},
				Timestamp:        time.Now(),
				MessageType:      tt.messageType,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			assert.Equal(t, tt.messageType, msg.MessageType)
		})
	}
}

// ============================================================================
// MessageAttachment Tests
// ============================================================================

func TestMessageAttachment_Creation(t *testing.T) {
	attachment := &MessageAttachment{
		ID:           1,
		MessageID:    100,
		AttachmentID: "attachment123",
		Filename:     "image.png",
		URL:          "https://cdn.discord.com/attachments/123/image.png",
		ProxyURL:     sql.NullString{String: "https://media.discord.net/attachments/123/image.png", Valid: true},
		SizeBytes:    1024000,
		Width:        sql.NullInt64{Int64: 1920, Valid: true},
		Height:       sql.NullInt64{Int64: 1080, Valid: true},
		ContentType:  sql.NullString{String: "image/png", Valid: true},
		CreatedAt:    time.Now(),
	}

	assert.Equal(t, int64(1), attachment.ID)
	assert.Equal(t, int64(100), attachment.MessageID)
	assert.Equal(t, "attachment123", attachment.AttachmentID)
	assert.Equal(t, "image.png", attachment.Filename)
	assert.Equal(t, 1024000, attachment.SizeBytes)
	assert.True(t, attachment.Width.Valid)
	assert.Equal(t, int64(1920), attachment.Width.Int64)
	assert.True(t, attachment.Height.Valid)
	assert.Equal(t, int64(1080), attachment.Height.Int64)
	assert.True(t, attachment.ContentType.Valid)
	assert.Equal(t, "image/png", attachment.ContentType.String)
}

func TestMessageAttachment_WithoutDimensions(t *testing.T) {
	// Non-image attachment (e.g., PDF, text file)
	attachment := &MessageAttachment{
		ID:           1,
		MessageID:    100,
		AttachmentID: "attachment123",
		Filename:     "document.pdf",
		URL:          "https://cdn.discord.com/attachments/123/document.pdf",
		SizeBytes:    500000,
		Width:        sql.NullInt64{Valid: false}, // No width
		Height:       sql.NullInt64{Valid: false}, // No height
		ContentType:  sql.NullString{String: "application/pdf", Valid: true},
		CreatedAt:    time.Now(),
	}

	assert.False(t, attachment.Width.Valid, "Non-image attachment should not have width")
	assert.False(t, attachment.Height.Valid, "Non-image attachment should not have height")
	assert.Equal(t, "application/pdf", attachment.ContentType.String)
}

func TestMessageAttachment_WithoutProxyURL(t *testing.T) {
	attachment := &MessageAttachment{
		ID:           1,
		MessageID:    100,
		AttachmentID: "attachment123",
		Filename:     "file.txt",
		URL:          "https://cdn.discord.com/attachments/123/file.txt",
		ProxyURL:     sql.NullString{Valid: false}, // No proxy URL
		SizeBytes:    1000,
		ContentType:  sql.NullString{String: "text/plain", Valid: true},
		CreatedAt:    time.Now(),
	}

	assert.False(t, attachment.ProxyURL.Valid, "Attachment may not have proxy URL")
}

func TestMessageAttachment_WithoutContentType(t *testing.T) {
	attachment := &MessageAttachment{
		ID:           1,
		MessageID:    100,
		AttachmentID: "attachment123",
		Filename:     "unknown.bin",
		URL:          "https://cdn.discord.com/attachments/123/unknown.bin",
		SizeBytes:    5000,
		ContentType:  sql.NullString{Valid: false}, // Unknown content type
		CreatedAt:    time.Now(),
	}

	assert.False(t, attachment.ContentType.Valid, "Attachment may not have content type")
}

func TestMessageAttachment_VariousContentTypes(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		contentType string
		hasSize     bool
	}{
		{"PNG image", "image.png", "image/png", true},
		{"JPEG image", "photo.jpg", "image/jpeg", true},
		{"GIF image", "animation.gif", "image/gif", true},
		{"PDF document", "doc.pdf", "application/pdf", false},
		{"Text file", "notes.txt", "text/plain", false},
		{"Video file", "video.mp4", "video/mp4", true},
		{"Audio file", "song.mp3", "audio/mpeg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attachment := &MessageAttachment{
				ID:           1,
				MessageID:    100,
				AttachmentID: "attachment123",
				Filename:     tt.filename,
				URL:          "https://cdn.discord.com/attachments/123/" + tt.filename,
				SizeBytes:    100000,
				ContentType:  sql.NullString{String: tt.contentType, Valid: true},
				CreatedAt:    time.Now(),
			}

			if tt.hasSize {
				attachment.Width = sql.NullInt64{Int64: 800, Valid: true}
				attachment.Height = sql.NullInt64{Int64: 600, Valid: true}
			}

			assert.Equal(t, tt.filename, attachment.Filename)
			assert.Equal(t, tt.contentType, attachment.ContentType.String)
			assert.Equal(t, tt.hasSize, attachment.Width.Valid)
			assert.Equal(t, tt.hasSize, attachment.Height.Valid)
		})
	}
}

func TestMessageAttachment_LargeFile(t *testing.T) {
	// Discord limit is typically 8MB for free users, 50MB for Nitro
	attachment := &MessageAttachment{
		ID:           1,
		MessageID:    100,
		AttachmentID: "attachment123",
		Filename:     "large_video.mp4",
		URL:          "https://cdn.discord.com/attachments/123/large_video.mp4",
		SizeBytes:    50 * 1024 * 1024,                        // 50MB
		Width:        sql.NullInt64{Int64: 3840, Valid: true}, // 4K
		Height:       sql.NullInt64{Int64: 2160, Valid: true},
		ContentType:  sql.NullString{String: "video/mp4", Valid: true},
		CreatedAt:    time.Now(),
	}

	assert.Equal(t, 50*1024*1024, attachment.SizeBytes)
	assert.Equal(t, int64(3840), attachment.Width.Int64)
	assert.Equal(t, int64(2160), attachment.Height.Int64)
}

func TestMessageAttachment_SmallFile(t *testing.T) {
	attachment := &MessageAttachment{
		ID:           1,
		MessageID:    100,
		AttachmentID: "attachment123",
		Filename:     "tiny.txt",
		URL:          "https://cdn.discord.com/attachments/123/tiny.txt",
		SizeBytes:    100, // 100 bytes
		ContentType:  sql.NullString{String: "text/plain", Valid: true},
		CreatedAt:    time.Now(),
	}

	assert.Equal(t, 100, attachment.SizeBytes)
}
