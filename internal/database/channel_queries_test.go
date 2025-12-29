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

func generateChannel(discordChannelID string, guildID int64) *models.Channel {
	return &models.Channel{
		DiscordChannelID: discordChannelID,
		GuildID:          guildID,
		Name:             "test-channel-" + discordChannelID,
		Type:             models.ChannelTypeGuildText,
		Position:         0,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{String: "Test channel topic", Valid: true},
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false},
	}
}

func assertChannelEqual(t *testing.T, expected, actual *models.Channel) {
	t.Helper()
	assert.Equal(t, expected.DiscordChannelID, actual.DiscordChannelID)
	assert.Equal(t, expected.GuildID, actual.GuildID)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.Position, actual.Position)
	assert.Equal(t, expected.ParentID, actual.ParentID)
	assert.Equal(t, expected.Topic, actual.Topic)
	assert.Equal(t, expected.NSFW, actual.NSFW)
	assert.Equal(t, expected.LastMessageID, actual.LastMessageID)
}

// ============================================================================
// Channel CRUD Tests
// ============================================================================

func TestCreateOrUpdateChannel_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild first
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create channel
	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)

	require.NoError(t, err)
	assert.NotZero(t, channel.ID)
	assert.NotZero(t, channel.CreatedAt)
	assert.NotZero(t, channel.UpdatedAt)
	assert.WithinDuration(t, time.Now(), channel.CreatedAt, 2*time.Second)
}

func TestCreateOrUpdateChannel_Upsert(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create initial channel
	channel1 := generateChannel("channel123", guild.ID)
	channel1.Name = "original-name"
	channel1.Position = 1
	err = db.CreateOrUpdateChannel(ctx, channel1)
	require.NoError(t, err)

	originalID := channel1.ID
	originalCreatedAt := channel1.CreatedAt

	time.Sleep(10 * time.Millisecond)

	// Upsert with same discord_channel_id but different data
	channel2 := generateChannel("channel123", guild.ID)
	channel2.Name = "updated-name"
	channel2.Position = 5
	channel2.NSFW = true
	err = db.CreateOrUpdateChannel(ctx, channel2)
	require.NoError(t, err)

	// ID should remain the same
	assert.Equal(t, originalID, channel2.ID)

	// Created_at should not change
	assert.WithinDuration(t, originalCreatedAt, channel2.CreatedAt, 1*time.Second)

	// Updated_at should be newer
	assert.True(t, channel2.UpdatedAt.After(channel2.CreatedAt))

	// Verify updated data
	retrieved, err := db.GetChannelByDiscordID(ctx, "channel123")
	require.NoError(t, err)
	assert.Equal(t, "updated-name", retrieved.Name)
	assert.Equal(t, 5, retrieved.Position)
	assert.True(t, retrieved.NSFW)
}

func TestGetChannelByID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild and channel
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Retrieve channel
	retrieved, err := db.GetChannelByID(ctx, channel.ID)

	require.NoError(t, err)
	assertChannelEqual(t, channel, retrieved)
}

func TestGetChannelByID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	channel, err := db.GetChannelByID(ctx, 99999)

	assert.Error(t, err)
	assert.Nil(t, channel)
	assert.Contains(t, err.Error(), "channel not found")
}

func TestGetChannelByDiscordID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild and channel
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Retrieve channel
	retrieved, err := db.GetChannelByDiscordID(ctx, "channel123")

	require.NoError(t, err)
	assertChannelEqual(t, channel, retrieved)
}

func TestGetChannelByDiscordID_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	channel, err := db.GetChannelByDiscordID(ctx, "nonexistent_channel")

	assert.Error(t, err)
	assert.Nil(t, channel)
	assert.Contains(t, err.Error(), "channel not found")
}

func TestGetChannelsByGuildID_Empty(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild with no channels
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Get channels
	channels, err := db.GetChannelsByGuildID(ctx, guild.ID)

	require.NoError(t, err)
	assert.Empty(t, channels)
}

func TestGetChannelsByGuildID_MultipleChannels(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create 3 channels with different positions
	channel1 := generateChannel("channel1", guild.ID)
	channel1.Name = "general"
	channel1.Position = 2
	err = db.CreateOrUpdateChannel(ctx, channel1)
	require.NoError(t, err)

	channel2 := generateChannel("channel2", guild.ID)
	channel2.Name = "announcements"
	channel2.Position = 0
	err = db.CreateOrUpdateChannel(ctx, channel2)
	require.NoError(t, err)

	channel3 := generateChannel("channel3", guild.ID)
	channel3.Name = "random"
	channel3.Position = 1
	err = db.CreateOrUpdateChannel(ctx, channel3)
	require.NoError(t, err)

	// Get channels (should be ordered by position ASC, then name ASC)
	channels, err := db.GetChannelsByGuildID(ctx, guild.ID)

	require.NoError(t, err)
	assert.Len(t, channels, 3)
	assert.Equal(t, "announcements", channels[0].Name)
	assert.Equal(t, 0, channels[0].Position)
	assert.Equal(t, "random", channels[1].Name)
	assert.Equal(t, 1, channels[1].Position)
	assert.Equal(t, "general", channels[2].Name)
	assert.Equal(t, 2, channels[2].Position)
}

func TestGetChannelsByDiscordGuildID_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create channels
	channel1 := generateChannel("channel1", guild.ID)
	channel1.Position = 0
	err = db.CreateOrUpdateChannel(ctx, channel1)
	require.NoError(t, err)

	channel2 := generateChannel("channel2", guild.ID)
	channel2.Position = 1
	err = db.CreateOrUpdateChannel(ctx, channel2)
	require.NoError(t, err)

	// Get channels by Discord guild ID
	channels, err := db.GetChannelsByDiscordGuildID(ctx, guild.DiscordGuildID)

	require.NoError(t, err)
	assert.Len(t, channels, 2)
}

func TestDeleteChannel_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild and channel
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Delete channel
	err = db.DeleteChannel(ctx, channel.ID)
	require.NoError(t, err)

	// Verify channel is gone
	retrieved, err := db.GetChannelByID(ctx, channel.ID)
	assert.Error(t, err)
	assert.Nil(t, retrieved)
}

func TestDeleteChannel_NotFound(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	err = db.DeleteChannel(ctx, 99999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel not found")
}

// ============================================================================
// Access Control Tests
// ============================================================================

func TestUserHasChannelAccess_True(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user, guild, and channel
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	err = db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Check access
	hasAccess, err := db.UserHasChannelAccess(ctx, user.ID, channel.DiscordChannelID)

	require.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestUserHasChannelAccess_False(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create user, guild, and channel (no user-guild relationship)
	user := generateUser("user123")
	err = db.CreateUser(ctx, user)
	require.NoError(t, err)

	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := generateChannel("channel123", guild.ID)
	err = db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Check access (should be false)
	hasAccess, err := db.UserHasChannelAccess(ctx, user.ID, channel.DiscordChannelID)

	require.NoError(t, err)
	assert.False(t, hasAccess)
}

func TestChannelCategoryHierarchy(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	// Create guild
	guild := generateGuild("guild123")
	err = db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create category channel
	category := generateChannel("category123", guild.ID)
	category.Name = "General Category"
	category.Type = models.ChannelTypeGuildCategory
	category.Position = 0
	err = db.CreateOrUpdateChannel(ctx, category)
	require.NoError(t, err)

	// Create text channel under category
	textChannel := generateChannel("text123", guild.ID)
	textChannel.Name = "general-chat"
	textChannel.Type = models.ChannelTypeGuildText
	textChannel.ParentID = sql.NullString{String: category.DiscordChannelID, Valid: true}
	textChannel.Position = 1
	err = db.CreateOrUpdateChannel(ctx, textChannel)
	require.NoError(t, err)

	// Verify parent-child relationship
	retrieved, err := db.GetChannelByDiscordID(ctx, textChannel.DiscordChannelID)
	require.NoError(t, err)
	assert.True(t, retrieved.ParentID.Valid)
	assert.Equal(t, category.DiscordChannelID, retrieved.ParentID.String)
}
