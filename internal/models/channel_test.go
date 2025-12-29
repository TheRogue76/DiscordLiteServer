package models

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// ChannelType Tests
// ============================================================================

func TestChannelType_Constants(t *testing.T) {
	tests := []struct {
		name        string
		channelType ChannelType
		expected    int
	}{
		{"Guild text channel", ChannelTypeGuildText, 0},
		{"DM channel", ChannelTypeDM, 1},
		{"Guild voice channel", ChannelTypeGuildVoice, 2},
		{"Group DM channel", ChannelTypeGroupDM, 3},
		{"Guild category", ChannelTypeGuildCategory, 4},
		{"Guild news channel", ChannelTypeGuildNews, 5},
		{"Guild store channel", ChannelTypeGuildStore, 6},
		{"Guild news thread", ChannelTypeGuildNewsThread, 10},
		{"Guild public thread", ChannelTypeGuildPublicThread, 11},
		{"Guild private thread", ChannelTypeGuildPrivateThread, 12},
		{"Guild stage voice", ChannelTypeGuildStageVoice, 13},
		{"Guild forum", ChannelTypeGuildForum, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.channelType))
		})
	}
}

func TestChannelType_DiscordAPICompliance(t *testing.T) {
	// Verify that channel types match Discord API specification
	// Note: Types 7, 8, 9, 14 are missing (reserved/deprecated by Discord)
	assert.Equal(t, 0, int(ChannelTypeGuildText))
	assert.Equal(t, 6, int(ChannelTypeGuildStore))
	assert.Equal(t, 10, int(ChannelTypeGuildNewsThread)) // Note the jump from 6 to 10
	assert.Equal(t, 15, int(ChannelTypeGuildForum))
}

// ============================================================================
// Channel Tests
// ============================================================================

func TestChannel_TextChannel(t *testing.T) {
	now := time.Now()
	channel := &Channel{
		ID:               1,
		DiscordChannelID: "1234567890",
		GuildID:          100,
		Name:             "general",
		Type:             ChannelTypeGuildText,
		Position:         0,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{String: "General discussion", Valid: true},
		NSFW:             false,
		LastMessageID:    sql.NullString{String: "9999999999", Valid: true},
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	assert.Equal(t, int64(1), channel.ID)
	assert.Equal(t, "1234567890", channel.DiscordChannelID)
	assert.Equal(t, int64(100), channel.GuildID)
	assert.Equal(t, "general", channel.Name)
	assert.Equal(t, ChannelTypeGuildText, channel.Type)
	assert.Equal(t, 0, channel.Position)
	assert.False(t, channel.ParentID.Valid)
	assert.True(t, channel.Topic.Valid)
	assert.Equal(t, "General discussion", channel.Topic.String)
	assert.False(t, channel.NSFW)
	assert.True(t, channel.LastMessageID.Valid)
}

func TestChannel_VoiceChannel(t *testing.T) {
	channel := &Channel{
		ID:               2,
		DiscordChannelID: "2234567890",
		GuildID:          100,
		Name:             "Voice Channel",
		Type:             ChannelTypeGuildVoice,
		Position:         1,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{Valid: false}, // Voice channels typically don't have topics
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false}, // Voice channels don't have messages
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeGuildVoice, channel.Type)
	assert.False(t, channel.Topic.Valid, "Voice channel should not have topic")
	assert.False(t, channel.LastMessageID.Valid, "Voice channel should not have messages")
}

func TestChannel_CategoryChannel(t *testing.T) {
	channel := &Channel{
		ID:               3,
		DiscordChannelID: "3234567890",
		GuildID:          100,
		Name:             "Text Channels",
		Type:             ChannelTypeGuildCategory,
		Position:         0,
		ParentID:         sql.NullString{Valid: false}, // Categories don't have parents
		Topic:            sql.NullString{Valid: false},
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeGuildCategory, channel.Type)
	assert.False(t, channel.ParentID.Valid, "Category should not have parent")
}

func TestChannel_WithParentCategory(t *testing.T) {
	channel := &Channel{
		ID:               4,
		DiscordChannelID: "4234567890",
		GuildID:          100,
		Name:             "announcements",
		Type:             ChannelTypeGuildText,
		Position:         2,
		ParentID:         sql.NullString{String: "3234567890", Valid: true}, // Under a category
		Topic:            sql.NullString{String: "Server announcements", Valid: true},
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.True(t, channel.ParentID.Valid, "Channel should have parent category")
	assert.Equal(t, "3234567890", channel.ParentID.String)
}

func TestChannel_NSFWChannel(t *testing.T) {
	channel := &Channel{
		ID:               5,
		DiscordChannelID: "5234567890",
		GuildID:          100,
		Name:             "nsfw",
		Type:             ChannelTypeGuildText,
		Position:         10,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{String: "NSFW content", Valid: true},
		NSFW:             true, // NSFW channel
		LastMessageID:    sql.NullString{Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.True(t, channel.NSFW, "Channel should be marked as NSFW")
}

func TestChannel_ThreadChannel(t *testing.T) {
	tests := []struct {
		name        string
		channelType ChannelType
	}{
		{"Public thread", ChannelTypeGuildPublicThread},
		{"Private thread", ChannelTypeGuildPrivateThread},
		{"News thread", ChannelTypeGuildNewsThread},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{
				ID:               6,
				DiscordChannelID: "6234567890",
				GuildID:          100,
				Name:             "Thread Discussion",
				Type:             tt.channelType,
				Position:         0,
				ParentID:         sql.NullString{String: "1234567890", Valid: true}, // Parent channel
				Topic:            sql.NullString{Valid: false},
				NSFW:             false,
				LastMessageID:    sql.NullString{String: "8888888888", Valid: true},
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			assert.Equal(t, tt.channelType, channel.Type)
			assert.True(t, channel.ParentID.Valid, "Thread should have parent channel")
		})
	}
}

func TestChannel_ForumChannel(t *testing.T) {
	channel := &Channel{
		ID:               7,
		DiscordChannelID: "7234567890",
		GuildID:          100,
		Name:             "Help Forum",
		Type:             ChannelTypeGuildForum,
		Position:         5,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{String: "Ask for help here", Valid: true},
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeGuildForum, channel.Type)
}

func TestChannel_StageChannel(t *testing.T) {
	channel := &Channel{
		ID:               8,
		DiscordChannelID: "8234567890",
		GuildID:          100,
		Name:             "Town Hall",
		Type:             ChannelTypeGuildStageVoice,
		Position:         1,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{String: "Weekly community meetings", Valid: true},
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeGuildStageVoice, channel.Type)
	assert.True(t, channel.Topic.Valid, "Stage channel can have topic")
}

func TestChannel_NewsChannel(t *testing.T) {
	channel := &Channel{
		ID:               9,
		DiscordChannelID: "9234567890",
		GuildID:          100,
		Name:             "news",
		Type:             ChannelTypeGuildNews,
		Position:         0,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{String: "Official news", Valid: true},
		NSFW:             false,
		LastMessageID:    sql.NullString{String: "7777777777", Valid: true},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeGuildNews, channel.Type)
}

func TestChannel_DMChannel(t *testing.T) {
	// DM channels work differently but can be stored similarly
	channel := &Channel{
		ID:               10,
		DiscordChannelID: "1034567890",
		GuildID:          0, // DMs don't belong to a guild
		Name:             "DM",
		Type:             ChannelTypeDM,
		Position:         0,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{Valid: false},
		NSFW:             false,
		LastMessageID:    sql.NullString{String: "6666666666", Valid: true},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeDM, channel.Type)
	assert.Equal(t, int64(0), channel.GuildID, "DM channel should not have guild ID")
}

func TestChannel_GroupDMChannel(t *testing.T) {
	channel := &Channel{
		ID:               11,
		DiscordChannelID: "1134567890",
		GuildID:          0, // Group DMs don't belong to a guild
		Name:             "Friend Group",
		Type:             ChannelTypeGroupDM,
		Position:         0,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{Valid: false},
		NSFW:             false,
		LastMessageID:    sql.NullString{String: "5555555555", Valid: true},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, ChannelTypeGroupDM, channel.Type)
	assert.Equal(t, int64(0), channel.GuildID, "Group DM should not have guild ID")
}

func TestChannel_WithoutTopic(t *testing.T) {
	channel := &Channel{
		ID:               12,
		DiscordChannelID: "1234567890",
		GuildID:          100,
		Name:             "random",
		Type:             ChannelTypeGuildText,
		Position:         5,
		ParentID:         sql.NullString{Valid: false},
		Topic:            sql.NullString{Valid: false}, // No topic
		NSFW:             false,
		LastMessageID:    sql.NullString{Valid: false},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.False(t, channel.Topic.Valid, "Channel may not have topic")
}

func TestChannel_Position(t *testing.T) {
	tests := []struct {
		name     string
		position int
	}{
		{"First channel", 0},
		{"Second channel", 1},
		{"Fifth channel", 5},
		{"Last channel", 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{
				ID:               1,
				DiscordChannelID: "1234567890",
				GuildID:          100,
				Name:             "channel",
				Type:             ChannelTypeGuildText,
				Position:         tt.position,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			assert.Equal(t, tt.position, channel.Position)
		})
	}
}

func TestChannel_LongName(t *testing.T) {
	// Discord allows up to 100 characters for channel names
	longName := "this-is-a-very-long-channel-name-that-discord-allows-up-to-one-hundred-characters-for-channel-names"

	channel := &Channel{
		ID:               1,
		DiscordChannelID: "1234567890",
		GuildID:          100,
		Name:             longName,
		Type:             ChannelTypeGuildText,
		Position:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, longName, channel.Name)
	assert.LessOrEqual(t, len(channel.Name), 100, "Channel name should be within Discord limits")
}

func TestChannel_LongTopic(t *testing.T) {
	// Discord allows up to 1024 characters for channel topics
	longTopic := "This is a very long topic that can contain up to 1024 characters in Discord. " +
		"Topics are used to describe what the channel is about and can include URLs, emojis, " +
		"and other formatting. " + string(make([]byte, 800))

	channel := &Channel{
		ID:               1,
		DiscordChannelID: "1234567890",
		GuildID:          100,
		Name:             "channel",
		Type:             ChannelTypeGuildText,
		Position:         0,
		Topic:            sql.NullString{String: longTopic, Valid: true},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.True(t, channel.Topic.Valid)
	assert.LessOrEqual(t, len(channel.Topic.String), 1024, "Topic should be within Discord limits")
}
