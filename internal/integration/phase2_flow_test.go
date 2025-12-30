package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	channelv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/channel/v1"
	messagev1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/message/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Phase 2 Integration Tests - Full Flow
// ============================================================================

func TestPhase2_GetGuilds_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Phase 1: Authenticate user (reuse existing OAuth flow)
	sessionID := ts.authenticateUser(ctx, t)

	// Setup mock Discord API responses for guilds
	guildsResponse := []map[string]interface{}{
		{
			"id":          "guild1",
			"name":        "Test Guild 1",
			"icon":        "icon_hash_1",
			"owner":       false,
			"permissions": "2147483647",
			"features":    []string{"COMMUNITY", "NEWS"},
		},
		{
			"id":          "guild2",
			"name":        "Test Guild 2",
			"icon":        nil,
			"owner":       true,
			"permissions": "2147483647",
			"features":    []string{"ANIMATED_ICON"},
		},
	}

	apiCallCount := 0
	ts.mockDiscordAPI.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/@me/guilds" {
			apiCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "50")
			w.Header().Set("X-RateLimit-Remaining", "49")
			w.Header().Set("X-RateLimit-Reset", time.Now().Add(5*time.Minute).Format(time.RFC3339))
			_ = json.NewEncoder(w).Encode(guildsResponse)
			return
		}
		http.NotFound(w, r)
	})

	// First call - cache miss, should call Discord API
	resp1, err := ts.channelClient.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId:    sessionID,
		ForceRefresh: false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp1)
	assert.Len(t, resp1.Guilds, 2)
	assert.Equal(t, "guild1", resp1.Guilds[0].DiscordGuildId)
	assert.Equal(t, "Test Guild 1", resp1.Guilds[0].Name)
	assert.Len(t, resp1.Guilds[0].Features, 2)
	assert.Equal(t, 1, apiCallCount, "First call should hit API")

	// Second call - cache hit, should NOT call Discord API
	resp2, err := ts.channelClient.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId:    sessionID,
		ForceRefresh: false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.Len(t, resp2.Guilds, 2)
	assert.Equal(t, "guild1", resp2.Guilds[0].DiscordGuildId)
	assert.Equal(t, 1, apiCallCount, "Second call should use cache")

	// Force refresh - should call API again
	resp3, err := ts.channelClient.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId:    sessionID,
		ForceRefresh: true,
	})
	require.NoError(t, err)
	require.NotNil(t, resp3)
	assert.Len(t, resp3.Guilds, 2)
	assert.Equal(t, 2, apiCallCount, "Force refresh should hit API")
}

func TestPhase2_GetChannels_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Phase 1: Authenticate user
	sessionID := ts.authenticateUser(ctx, t)

	guildID := "guild123"

	// Setup mock Discord API responses
	channelsResponse := []map[string]interface{}{
		{
			"id":       "channel1",
			"guild_id": guildID,
			"name":     "general",
			"type":     1, // GUILD_TEXT
			"position": 0,
			"topic":    "General discussion",
			"nsfw":     false,
		},
		{
			"id":       "channel2",
			"guild_id": guildID,
			"name":     "announcements",
			"type":     5, // GUILD_NEWS
			"position": 1,
			"topic":    "Server announcements",
			"nsfw":     false,
		},
	}

	// First setup guilds so user has access
	ts.setupGuildMembership(ctx, t, sessionID, guildID)

	apiCallCount := 0
	ts.mockDiscordAPI.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/guilds/"+guildID+"/channels" {
			apiCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "50")
			w.Header().Set("X-RateLimit-Remaining", "49")
			w.Header().Set("X-RateLimit-Reset", time.Now().Add(5*time.Minute).Format(time.RFC3339))
			_ = json.NewEncoder(w).Encode(channelsResponse)
			return
		}
		http.NotFound(w, r)
	})

	// First call - cache miss
	resp1, err := ts.channelClient.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId:    sessionID,
		GuildId:      guildID,
		ForceRefresh: false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp1)
	assert.Len(t, resp1.Channels, 2)
	assert.Equal(t, "channel1", resp1.Channels[0].DiscordChannelId)
	assert.Equal(t, "general", resp1.Channels[0].Name)
	assert.Equal(t, channelv1.ChannelType_CHANNEL_TYPE_GUILD_TEXT, resp1.Channels[0].Type)
	assert.Equal(t, 1, apiCallCount, "First call should hit API")

	// Second call - cache hit
	resp2, err := ts.channelClient.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId:    sessionID,
		GuildId:      guildID,
		ForceRefresh: false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.Len(t, resp2.Channels, 2)
	assert.Equal(t, 1, apiCallCount, "Second call should use cache")
}

func TestPhase2_GetMessages_WithPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Phase 1: Authenticate user
	sessionID := ts.authenticateUser(ctx, t)

	guildID := "guild123"
	channelID := "channel456"

	// Setup guild and channel membership
	ts.setupChannelAccess(ctx, t, sessionID, guildID, channelID)

	// Setup mock Discord API responses for messages
	messagesPage1 := []map[string]interface{}{
		{
			"id":         "msg1",
			"channel_id": channelID,
			"author": map[string]interface{}{
				"id":            "user1",
				"username":      "TestUser1",
				"discriminator": "0001",
				"avatar":        "avatar1",
			},
			"content":   "First message",
			"timestamp": time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
			"type":      0,
		},
		{
			"id":         "msg2",
			"channel_id": channelID,
			"author": map[string]interface{}{
				"id":            "user2",
				"username":      "TestUser2",
				"discriminator": "0002",
			},
			"content":   "Second message",
			"timestamp": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			"type":      0,
		},
	}

	messagesPage2 := []map[string]interface{}{
		{
			"id":         "msg3",
			"channel_id": channelID,
			"author": map[string]interface{}{
				"id":            "user3",
				"username":      "TestUser3",
				"discriminator": "0003",
			},
			"content":   "Third message",
			"timestamp": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			"type":      0,
		},
	}

	apiCallCount := 0
	ts.mockDiscordAPI.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/channels/"+channelID+"/messages" {
			apiCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "50")
			w.Header().Set("X-RateLimit-Remaining", "49")
			w.Header().Set("X-RateLimit-Reset", time.Now().Add(5*time.Minute).Format(time.RFC3339))

			// Pagination: check for before parameter
			beforeID := r.URL.Query().Get("before")
			if beforeID == "" {
				_ = json.NewEncoder(w).Encode(messagesPage1)
			} else {
				_ = json.NewEncoder(w).Encode(messagesPage2)
			}
			return
		}
		http.NotFound(w, r)
	})

	// First page
	resp1, err := ts.messageClient.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId:    sessionID,
		ChannelId:    channelID,
		Limit:        2,
		ForceRefresh: false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp1)
	assert.Len(t, resp1.Messages, 2)
	assert.Equal(t, "msg1", resp1.Messages[0].DiscordMessageId)
	assert.Equal(t, "First message", resp1.Messages[0].Content)
	assert.True(t, resp1.HasMore, "Should have more messages")
	assert.Equal(t, 1, apiCallCount)

	// Second page (pagination with before)
	resp2, err := ts.messageClient.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId:    sessionID,
		ChannelId:    channelID,
		Limit:        10,
		Before:       "msg2",
		ForceRefresh: false,
	})
	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.Len(t, resp2.Messages, 1)
	assert.Equal(t, "msg3", resp2.Messages[0].DiscordMessageId)
	assert.Equal(t, 2, apiCallCount)
}

// Helper function to setup guild membership
func (ts *TestSuite) setupGuildMembership(ctx context.Context, t *testing.T, sessionID, guildID string) {
	t.Helper()

	// Get user from session
	session, err := ts.db.GetAuthSession(ctx, sessionID)
	require.NoError(t, err)
	require.True(t, session.UserID.Valid)

	// Create guild
	guild := &models.Guild{
		DiscordGuildID: guildID,
		Name:           "Test Guild",
		Permissions:    2147483647,
		Features:       pq.StringArray{},
	}
	err = ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Create user-guild membership
	err = ts.db.CreateUserGuild(ctx, session.UserID.Int64, guild.ID)
	require.NoError(t, err)
}

// Helper function to setup channel access
func (ts *TestSuite) setupChannelAccess(ctx context.Context, t *testing.T, sessionID, guildID, channelID string) {
	t.Helper()

	// First setup guild membership
	ts.setupGuildMembership(ctx, t, sessionID, guildID)

	// Get guild
	guild, err := ts.db.GetGuildByDiscordID(ctx, guildID)
	require.NoError(t, err)

	// Create channel
	channel := &models.Channel{
		DiscordChannelID: channelID,
		GuildID:          guild.ID,
		Name:             "test-channel",
		Type:             models.ChannelTypeGuildText,
		Position:         0,
	}
	err = ts.db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)
}
