package grpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	channelv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/channel/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Test Setup & Helpers
// ============================================================================

type testChannelService struct {
	db            *database.DB
	cleanup       func()
	server        *ChannelServer
	mockDiscord   *httptest.Server
	discordClient *auth.DiscordClient
	cacheManager  *CacheManager
}

func setupChannelServiceTest(t *testing.T) *testChannelService {
	t.Helper()

	// Setup test database
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)

	// Create mock Discord API server
	mockDiscord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default handler - can be overridden in tests
		w.WriteHeader(http.StatusNotFound)
	}))

	// Create Discord client with mock server
	cfg := &config.Config{
		Discord: config.DiscordConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
			RedirectURI:  "http://localhost:8080/callback",
			Scopes:       []string{"identify", "guilds"},
		},
		Security: config.SecurityConfig{
			TokenEncryptionKey: []byte("12345678901234567890123456789012"), // 32 bytes
		},
	}

	logger := zap.NewNop()
	discordClient := auth.NewDiscordClient(cfg, logger)
	// Override base URL to use mock server
	discordClient.SetBaseURL(mockDiscord.URL)

	// Create cache manager
	cacheManager := NewCacheManager(db, logger)

	// Create channel server
	server := NewChannelServer(db, discordClient, logger, cacheManager)

	return &testChannelService{
		db:            db,
		cleanup:       func() { cleanup(); mockDiscord.Close() },
		server:        server,
		mockDiscord:   mockDiscord,
		discordClient: discordClient,
		cacheManager:  cacheManager,
	}
}

func (ts *testChannelService) createAuthenticatedSession(ctx context.Context, t *testing.T) (string, int64) {
	t.Helper()

	// Create user
	user := &models.User{
		DiscordID:     "discord123",
		Username:      "testuser",
		Discriminator: sql.NullString{String: "1234", Valid: true},
		Email:         sql.NullString{String: "test@example.com", Valid: true},
	}
	err := ts.db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create OAuth token
	accessToken, err := ts.discordClient.EncryptToken("test_access_token")
	require.NoError(t, err)
	refreshToken, err := ts.discordClient.EncryptToken("test_refresh_token")
	require.NoError(t, err)

	oauthToken := &models.OAuthToken{
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(7 * 24 * time.Hour),
		Scope:        "identify guilds",
	}
	err = ts.db.StoreOAuthToken(ctx, oauthToken)
	require.NoError(t, err)

	// Create authenticated session
	session := &models.AuthSession{
		SessionID:  "test_session_123",
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		AuthStatus: "authenticated",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}
	err = ts.db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	return session.SessionID, user.ID
}

func (ts *testChannelService) setupMockGuildsResponse(guilds []*auth.DiscordGuild) {
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/@me/guilds" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(guilds)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func (ts *testChannelService) setupMockChannelsResponse(guildID string, channels []*auth.DiscordChannel) {
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/guilds/"+guildID+"/channels" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(channels)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

// ============================================================================
// GetGuilds Tests
// ============================================================================

func TestGetGuilds_Success_CacheMiss(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _ := ts.createAuthenticatedSession(ctx, t)

	// Setup mock Discord response
	mockGuilds := []*auth.DiscordGuild{
		{
			ID:          "guild1",
			Name:        "Test Guild 1",
			Icon:        "icon1",
			Owner:       true,
			Permissions: "2147483647",
			Features:    []string{"ANIMATED_ICON"},
		},
		{
			ID:          "guild2",
			Name:        "Test Guild 2",
			Icon:        "icon2",
			Owner:       false,
			Permissions: "104324673",
			Features:    []string{},
		},
	}
	ts.setupMockGuildsResponse(mockGuilds)

	// Call GetGuilds
	resp, err := ts.server.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId: sessionID,
	})

	// Verify response
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Guilds, 2)
	assert.False(t, resp.FromCache, "First call should be from API, not cache")

	// Verify guild data
	assert.Equal(t, "guild1", resp.Guilds[0].DiscordGuildId)
	assert.Equal(t, "Test Guild 1", resp.Guilds[0].Name)
	assert.Equal(t, int64(2147483647), resp.Guilds[0].Permissions)
	assert.Contains(t, resp.Guilds[0].Features, "ANIMATED_ICON")

	// Verify guilds were stored in database
	storedGuild, err := ts.db.GetGuildByDiscordID(ctx, "guild1")
	require.NoError(t, err)
	assert.Equal(t, "Test Guild 1", storedGuild.Name)
}

func TestGetGuilds_Success_CacheHit(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID := ts.createAuthenticatedSession(ctx, t)

	// Pre-populate database with guilds
	guild := &models.Guild{
		DiscordGuildID: "guild1",
		Name:           "Cached Guild",
		Permissions:    123456,
		Features:       []string{"COMMUNITY"},
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)
	err = ts.db.CreateUserGuild(ctx, userID, guild.ID)
	require.NoError(t, err)

	// Set cache as valid
	err = ts.cacheManager.SetGuildCache(ctx, userID)
	require.NoError(t, err)

	// Setup mock to return error (should NOT be called)
	ts.setupMockGuildsResponse(nil)
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Discord API should not be called when cache is valid")
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Call GetGuilds (should use cache)
	resp, err := ts.server.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId: sessionID,
	})

	// Verify response came from cache
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.FromCache, "Should be served from cache")
	assert.Len(t, resp.Guilds, 1)
	assert.Equal(t, "Cached Guild", resp.Guilds[0].Name)
}

func TestGetGuilds_ForceRefresh_BypassesCache(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID := ts.createAuthenticatedSession(ctx, t)

	// Pre-populate cache
	guild := &models.Guild{
		DiscordGuildID: "guild_old",
		Name:           "Old Cached Guild",
		Permissions:    123456,
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)
	err = ts.db.CreateUserGuild(ctx, userID, guild.ID)
	require.NoError(t, err)
	err = ts.cacheManager.SetGuildCache(ctx, userID)
	require.NoError(t, err)

	// Setup mock with new data
	mockGuilds := []*auth.DiscordGuild{
		{
			ID:          "guild_new",
			Name:        "New Fresh Guild",
			Permissions: "999999",
		},
	}
	ts.setupMockGuildsResponse(mockGuilds)

	// Call GetGuilds with force_refresh
	resp, err := ts.server.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId:    sessionID,
		ForceRefresh: true,
	})

	// Verify response came from API
	require.NoError(t, err)
	assert.False(t, resp.FromCache, "force_refresh should bypass cache")

	// When force refresh, we get only what Discord API returns (not cached data)
	// Discord API returned only the new guild, so we should get only 1 guild
	assert.Len(t, resp.Guilds, 1)
	assert.Equal(t, "guild_new", resp.Guilds[0].DiscordGuildId)
	assert.Equal(t, "New Fresh Guild", resp.Guilds[0].Name)

	// Verify new guild was stored in database
	storedGuild, err := ts.db.GetGuildByDiscordID(ctx, "guild_new")
	require.NoError(t, err)
	assert.Equal(t, "New Fresh Guild", storedGuild.Name)
}

func TestGetGuilds_InvalidSession(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Call with non-existent session
	resp, err := ts.server.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId: "invalid_session_id",
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "invalid session")
}

func TestGetGuilds_SessionNotAuthenticated(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Create pending session (not authenticated)
	session := &models.AuthSession{
		SessionID:  "pending_session",
		AuthStatus: "pending",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}
	err := ts.db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Call GetGuilds
	resp, err := ts.server.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId: session.SessionID,
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "not authenticated")
}

func TestGetGuilds_DiscordAPIError(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _ := ts.createAuthenticatedSession(ctx, t)

	// Setup mock to return error
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "Internal Server Error"}`))
	})

	// Call GetGuilds
	resp, err := ts.server.GetGuilds(ctx, &channelv1.GetGuildsRequest{
		SessionId: sessionID,
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to fetch guilds from Discord API")
}

// ============================================================================
// GetChannels Tests
// ============================================================================

func TestGetChannels_Success_CacheMiss(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID := ts.createAuthenticatedSession(ctx, t)

	// Create guild and link user
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Test Guild",
		Permissions:    123456,
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)
	err = ts.db.CreateUserGuild(ctx, userID, guild.ID)
	require.NoError(t, err)

	// Setup mock Discord response
	mockChannels := []*auth.DiscordChannel{
		{
			ID:       "channel1",
			Type:     0, // GUILD_TEXT
			GuildID:  "guild123",
			Position: 0,
			Name:     "general",
			Topic:    "General discussion",
			NSFW:     false,
		},
		{
			ID:       "channel2",
			Type:     4, // GUILD_CATEGORY
			GuildID:  "guild123",
			Position: 1,
			Name:     "Voice Channels",
		},
	}
	ts.setupMockChannelsResponse("guild123", mockChannels)

	// Call GetChannels
	resp, err := ts.server.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId: sessionID,
		GuildId:   "guild123",
	})

	// Verify response
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Channels, 2)
	assert.False(t, resp.FromCache, "First call should be from API")

	// Verify channel data
	assert.Equal(t, "channel1", resp.Channels[0].DiscordChannelId)
	assert.Equal(t, "general", resp.Channels[0].Name)
	assert.Equal(t, "General discussion", resp.Channels[0].Topic)
	assert.False(t, resp.Channels[0].Nsfw)

	// Verify channels were stored in database
	storedChannel, err := ts.db.GetChannelByDiscordID(ctx, "channel1")
	require.NoError(t, err)
	assert.Equal(t, "general", storedChannel.Name)
}

func TestGetChannels_Success_CacheHit(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID := ts.createAuthenticatedSession(ctx, t)

	// Create guild and link user
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Test Guild",
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)
	err = ts.db.CreateUserGuild(ctx, userID, guild.ID)
	require.NoError(t, err)

	// Pre-populate database with channels
	channel := &models.Channel{
		DiscordChannelID: "channel1",
		GuildID:          guild.ID,
		Name:             "cached-channel",
		Type:             models.ChannelTypeGuildText,
		Position:         0,
	}
	err = ts.db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Set cache as valid
	err = ts.cacheManager.SetChannelCache(ctx, "guild123", userID)
	require.NoError(t, err)

	// Setup mock to fail (should NOT be called)
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Discord API should not be called when cache is valid")
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Call GetChannels (should use cache)
	resp, err := ts.server.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId: sessionID,
		GuildId:   "guild123",
	})

	// Verify response came from cache
	require.NoError(t, err)
	assert.True(t, resp.FromCache, "Should be served from cache")
	assert.Len(t, resp.Channels, 1)
	assert.Equal(t, "cached-channel", resp.Channels[0].Name)
}

func TestGetChannels_NoGuildAccess(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _ := ts.createAuthenticatedSession(ctx, t)

	// Create guild but DON'T link user to it
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Restricted Guild",
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Call GetChannels (user has no access)
	resp, err := ts.server.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId: sessionID,
		GuildId:   "guild123",
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "don't have access")
}

func TestGetChannels_InvalidSession(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Call with invalid session
	resp, err := ts.server.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId: "invalid_session",
		GuildId:   "guild123",
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetChannels_ForceRefresh_BypassesCache(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID := ts.createAuthenticatedSession(ctx, t)

	// Create guild and link user
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Test Guild",
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)
	err = ts.db.CreateUserGuild(ctx, userID, guild.ID)
	require.NoError(t, err)

	// Set cache
	err = ts.cacheManager.SetChannelCache(ctx, "guild123", userID)
	require.NoError(t, err)

	// Setup mock with new data
	mockChannels := []*auth.DiscordChannel{
		{
			ID:       "new_channel",
			Type:     0,
			GuildID:  "guild123",
			Name:     "fresh-channel",
			Position: 0,
		},
	}
	ts.setupMockChannelsResponse("guild123", mockChannels)

	// Call GetChannels with force_refresh
	resp, err := ts.server.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId:    sessionID,
		GuildId:      "guild123",
		ForceRefresh: true,
	})

	// Verify response came from API
	require.NoError(t, err)
	assert.False(t, resp.FromCache, "force_refresh should bypass cache")
	assert.Len(t, resp.Channels, 1)
	assert.Equal(t, "fresh-channel", resp.Channels[0].Name)
}

func TestGetChannels_DiscordAPIError(t *testing.T) {
	ts := setupChannelServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID := ts.createAuthenticatedSession(ctx, t)

	// Create guild and link user
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Test Guild",
	}
	err := ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)
	err = ts.db.CreateUserGuild(ctx, userID, guild.ID)
	require.NoError(t, err)

	// Setup mock to return error
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "Missing Access"}`))
	})

	// Call GetChannels
	resp, err := ts.server.GetChannels(ctx, &channelv1.GetChannelsRequest{
		SessionId: sessionID,
		GuildId:   "guild123",
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to fetch channels from Discord API")
}
