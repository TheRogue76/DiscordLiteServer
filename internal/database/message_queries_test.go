package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Helper Functions
// ============================================================================

func generateMessage(discordMessageID string, channelID int64) *models.Message {
	return &models.Message{
		DiscordMessageID:    discordMessageID,
		ChannelID:           channelID,
		AuthorID:            "author_" + discordMessageID,
		AuthorUsername:      "TestUser",
		AuthorAvatar:        sql.NullString{String: "avatar_hash", Valid: true},
		Content:             sql.NullString{String: "Test message content for " + discordMessageID, Valid: true},
		Timestamp:           time.Now().UTC(),
		EditedTimestamp:     sql.NullTime{Valid: false},
		MessageType:         models.MessageTypeDefault,
		ReferencedMessageID: sql.NullString{Valid: false},
	}
}

func generateAttachment(messageID int64, attachmentID string) *models.MessageAttachment {
	return &models.MessageAttachment{
		MessageID:    messageID,
		AttachmentID: attachmentID,
		Filename:     "test_file_" + attachmentID + ".png",
		URL:          "https://cdn.discord.com/attachments/" + attachmentID,
		ProxyURL:     sql.NullString{String: "https://media.discord.net/" + attachmentID, Valid: true},
		SizeBytes:    1024000,
		Width:        sql.NullInt64{Int64: 1920, Valid: true},
		Height:       sql.NullInt64{Int64: 1080, Valid: true},
		ContentType:  sql.NullString{String: "image/png", Valid: true},
	}
}

// ============================================================================
// Message CRUD Tests
// ============================================================================

func TestCreateOrUpdateMessage_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild and channel first
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Create message
	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)

	require.NoError(t, err)
	assert.NotZero(t, message.ID)
	assert.NotZero(t, message.CreatedAt)
	assert.NotZero(t, message.UpdatedAt)
}

func TestCreateOrUpdateMessage_Upsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup guild and channel
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Create initial message
	message1 := generateMessage("message123", channel.ID)
	message1.Content = sql.NullString{String: "Original content", Valid: true}
	err = db.CreateOrUpdateMessage(ctx, message1)
	require.NoError(t, err)

	originalID := message1.ID
	originalCreatedAt := message1.CreatedAt

	time.Sleep(10 * time.Millisecond)

	// Upsert (simulate edit)
	message2 := generateMessage("message123", channel.ID)
	message2.Content = sql.NullString{String: "Edited content", Valid: true}
	message2.EditedTimestamp = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	err = db.CreateOrUpdateMessage(ctx, message2)
	require.NoError(t, err)

	// Verify upsert behavior
	assert.Equal(t, originalID, message2.ID)
	assert.WithinDuration(t, originalCreatedAt, message2.CreatedAt, 1*time.Second)
	assert.True(t, message2.UpdatedAt.After(message2.CreatedAt))

	// Verify edited content
	retrieved, err := db.GetMessageByDiscordID(ctx, "message123")
	require.NoError(t, err)
	assert.Equal(t, "Edited content", retrieved.Content.String)
	assert.True(t, retrieved.EditedTimestamp.Valid)
}

func TestGetMessageByID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup and create message
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Retrieve message
	retrieved, err := db.GetMessageByID(ctx, message.ID)

	require.NoError(t, err)
	assert.Equal(t, message.DiscordMessageID, retrieved.DiscordMessageID)
	assert.Equal(t, message.Content, retrieved.Content)
}

func TestGetMessageByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	message, err := db.GetMessageByID(ctx, 99999)

	assert.Error(t, err)
	assert.Nil(t, message)
	assert.Contains(t, err.Error(), "message not found")
}

func TestGetMessageByDiscordID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup and create message
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Retrieve message
	retrieved, err := db.GetMessageByDiscordID(ctx, "message123")

	require.NoError(t, err)
	assert.Equal(t, message.ID, retrieved.ID)
	assert.Equal(t, message.Content, retrieved.Content)
}

func TestGetMessageByDiscordID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	message, err := db.GetMessageByDiscordID(ctx, "nonexistent_message")

	assert.Error(t, err)
	assert.Nil(t, message)
	assert.Contains(t, err.Error(), "message not found")
}

// ============================================================================
// Message Pagination Tests
// ============================================================================

func TestGetMessagesByChannelID_Empty(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup channel with no messages
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Get messages
	messages, err := db.GetMessagesByChannelID(ctx, channel.ID, 50, "", "")

	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestGetMessagesByChannelID_Pagination(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup channel
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Create 10 messages with increasing timestamps
	messageIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		message := generateMessage("message"+string(rune('0'+i)), channel.ID)
		message.Timestamp = time.Now().UTC().Add(time.Duration(i) * time.Second)
		err = db.CreateOrUpdateMessage(ctx, message)
		require.NoError(t, err)
		messageIDs[i] = message.DiscordMessageID
	}

	// Test 1: Get first 5 messages (most recent)
	messages, err := db.GetMessagesByChannelID(ctx, channel.ID, 5, "", "")
	require.NoError(t, err)
	assert.Len(t, messages, 5)
	// Should be in DESC order (newest first)
	assert.Equal(t, "message9", messages[0].DiscordMessageID)
	assert.Equal(t, "message5", messages[4].DiscordMessageID)

	// Test 2: Get messages before message5 (pagination backward)
	messages, err = db.GetMessagesByChannelID(ctx, channel.ID, 5, "message5", "")
	require.NoError(t, err)
	assert.Len(t, messages, 5)
	assert.Equal(t, "message4", messages[0].DiscordMessageID)
	assert.Equal(t, "message0", messages[4].DiscordMessageID)

	// Test 3: Get messages after message0 (pagination forward)
	messages, err = db.GetMessagesByChannelID(ctx, channel.ID, 5, "", "message0")
	require.NoError(t, err)
	assert.Len(t, messages, 5)
	assert.Equal(t, "message1", messages[0].DiscordMessageID)
	assert.Equal(t, "message5", messages[4].DiscordMessageID)
}

func TestGetMessagesByChannelID_LimitRespected(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup channel with 20 messages
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		message := generateMessage("message"+string(rune('A'+i)), channel.ID)
		message.Timestamp = time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		err = db.CreateOrUpdateMessage(ctx, message)
		require.NoError(t, err)
	}

	// Get with limit=10
	messages, err := db.GetMessagesByChannelID(ctx, channel.ID, 10, "", "")

	require.NoError(t, err)
	assert.Len(t, messages, 10, "Should respect limit of 10")
}

// ============================================================================
// Message Attachment Tests
// ============================================================================

func TestCreateMessageAttachment_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup message
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Create attachment
	attachment := generateAttachment(message.ID, "attachment123")
	err = db.CreateMessageAttachment(ctx, attachment)

	require.NoError(t, err)
	assert.NotZero(t, attachment.ID)
	assert.NotZero(t, attachment.CreatedAt)
}

func TestGetMessageAttachmentsByMessageID_Empty(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup message with no attachments
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Get attachments
	attachments, err := db.GetMessageAttachmentsByMessageID(ctx, message.ID)

	require.NoError(t, err)
	assert.Empty(t, attachments)
}

func TestGetMessageAttachmentsByMessageID_Multiple(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup message
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Create 3 attachments
	for i := 0; i < 3; i++ {
		attachment := generateAttachment(message.ID, "attachment"+string(rune('1'+i)))
		err = db.CreateMessageAttachment(ctx, attachment)
		require.NoError(t, err)
	}

	// Get attachments
	attachments, err := db.GetMessageAttachmentsByMessageID(ctx, message.ID)

	require.NoError(t, err)
	assert.Len(t, attachments, 3)
}

// ============================================================================
// Delete and Cascade Tests
// ============================================================================

func TestDeleteMessage_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup message
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Delete message
	err = db.DeleteMessage(ctx, message.DiscordMessageID)
	require.NoError(t, err)

	// Verify message is gone
	retrieved, err := db.GetMessageByDiscordID(ctx, message.DiscordMessageID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestDeleteMessage_CascadesAttachments(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup message with attachments
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	message := generateMessage("message123", channel.ID)
	err = db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	attachment := generateAttachment(message.ID, "attachment123")
	err = db.CreateMessageAttachment(ctx, attachment)
	require.NoError(t, err)

	// Delete message (should cascade to attachments)
	err = db.DeleteMessage(ctx, message.DiscordMessageID)
	require.NoError(t, err)

	// Verify attachments are also gone
	attachments, err := db.GetMessageAttachmentsByMessageID(ctx, message.ID)
	require.NoError(t, err)
	assert.Empty(t, attachments, "Attachments should be deleted via cascade")
}

func TestDeleteMessage_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.DeleteMessage(ctx, "nonexistent_message")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message not found")
}

// ============================================================================
// Message Count Tests
// ============================================================================

func TestGetMessageCountByChannelID(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Setup channel
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Initially 0 messages
	count, err := db.GetMessageCountByChannelID(ctx, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create 5 messages
	for i := 0; i < 5; i++ {
		message := generateMessage("message"+string(rune('0'+i)), channel.ID)
		err = db.CreateOrUpdateMessage(ctx, message)
		require.NoError(t, err)
	}

	// Count should be 5
	count, err = db.GetMessageCountByChannelID(ctx, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}
