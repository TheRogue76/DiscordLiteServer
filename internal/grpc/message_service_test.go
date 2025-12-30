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

	messagev1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/message/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ============================================================================
// Test Setup & Helpers
// ============================================================================

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

// mockWebSocketManager is a mock implementation of WebSocketManager for testing
type mockWebSocketManager struct {
	enabled bool
}

func (m *mockWebSocketManager) IsEnabled() bool {
	return m.enabled
}

func (m *mockWebSocketManager) Subscribe(ctx context.Context, userID int64, channelIDs []string) (<-chan *messagev1.MessageEvent, error) {
	// Return a channel that never sends anything
	ch := make(chan *messagev1.MessageEvent)
	return ch, nil
}

func (m *mockWebSocketManager) Unsubscribe(userID int64, channelIDs []string) {
	// No-op
}

type testMessageService struct {
	db            *database.DB
	cleanup       func()
	server        *MessageServer
	mockDiscord   *httptest.Server
	discordClient *auth.DiscordClient
	cacheManager  *CacheManager
}

func setupMessageServiceTest(t *testing.T) *testMessageService {
	t.Helper()

	// Setup test database
	ctx := context.Background()
	db, cleanup, err := setupTestDB(ctx)
	require.NoError(t, err)

	// Create mock Discord API server
	mockDiscord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	// Create Discord client with mock server
	cfg := &config.Config{
		Discord: config.DiscordConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
			RedirectURI:  "http://localhost:8080/callback",
			Scopes:       []string{"identify", "guilds", "messages.read"},
		},
		Security: config.SecurityConfig{
			TokenEncryptionKey: []byte("12345678901234567890123456789012"), // 32 bytes
		},
	}

	logger := zap.NewNop()
	discordClient := auth.NewDiscordClient(cfg, logger)
	discordClient.SetBaseURL(mockDiscord.URL)

	// Create cache manager
	cacheManager := NewCacheManager(db, logger)

	// Create mock WebSocket manager
	mockWSManager := &mockWebSocketManager{enabled: false}

	// Create message server
	server := NewMessageServer(db, discordClient, logger, cacheManager, mockWSManager)

	return &testMessageService{
		db:            db,
		cleanup:       func() { cleanup(); mockDiscord.Close() },
		server:        server,
		mockDiscord:   mockDiscord,
		discordClient: discordClient,
		cacheManager:  cacheManager,
	}
}

func (ts *testMessageService) createAuthenticatedSessionWithChannel(ctx context.Context, t *testing.T) (string, int64, *models.Channel) {
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
		Scope:        "identify guilds messages.read",
	}
	err = ts.db.StoreOAuthToken(ctx, oauthToken)
	require.NoError(t, err)

	// Create guild
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Test Guild",
		Permissions:    123456,
	}
	err = ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	// Link user to guild
	err = ts.db.CreateUserGuild(ctx, user.ID, guild.ID)
	require.NoError(t, err)

	// Create channel
	channel := &models.Channel{
		DiscordChannelID: "channel123",
		GuildID:          guild.ID,
		Name:             "test-channel",
		Type:             models.ChannelTypeGuildText,
		Position:         0,
	}
	err = ts.db.CreateOrUpdateChannel(ctx, channel)
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

	return session.SessionID, user.ID, channel
}

func (ts *testMessageService) setupMockMessagesResponse(channelID string, messages []*auth.DiscordMessage) {
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/channels/"+channelID+"/messages" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(messages)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

// ============================================================================
// GetMessages Tests
// ============================================================================

func TestGetMessages_Success_CacheMiss(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Setup mock Discord response
	timestamp := time.Now().UTC().Format(time.RFC3339)
	mockMessages := []*auth.DiscordMessage{
		{
			ID:        "msg1",
			ChannelID: channel.DiscordChannelID,
			Author: auth.DiscordUser{
				ID:       "author1",
				Username: "testauthor",
				Avatar:   "avatar123",
			},
			Content:   "Hello, world!",
			Timestamp: timestamp,
			Type:      0,
			Attachments: []auth.DiscordAttachment{
				{
					ID:          "att1",
					Filename:    "image.png",
					Size:        1024,
					URL:         "https://cdn.discord.com/attachments/123/456/image.png",
					ProxyURL:    "https://media.discord.net/attachments/123/456/image.png",
					ContentType: "image/png",
					Width:       intPtr(800),
					Height:      intPtr(600),
				},
			},
		},
		{
			ID:        "msg2",
			ChannelID: channel.DiscordChannelID,
			Author: auth.DiscordUser{
				ID:       "author2",
				Username: "anotheruser",
			},
			Content:   "Second message",
			Timestamp: timestamp,
			Type:      0,
		},
	}
	ts.setupMockMessagesResponse(channel.DiscordChannelID, mockMessages)

	// Call GetMessages
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: sessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     50,
	})

	// Verify response
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Messages, 2)
	assert.False(t, resp.FromCache, "First call should be from API")

	// Verify message data
	assert.Equal(t, "msg1", resp.Messages[0].DiscordMessageId)
	assert.Equal(t, "Hello, world!", resp.Messages[0].Content)
	assert.Equal(t, "author1", resp.Messages[0].Author.DiscordId)
	assert.Equal(t, "testauthor", resp.Messages[0].Author.Username)
	assert.Len(t, resp.Messages[0].Attachments, 1)
	assert.Equal(t, "image.png", resp.Messages[0].Attachments[0].Filename)

	// Verify messages were stored in database
	storedMsg, err := ts.db.GetMessageByDiscordID(ctx, "msg1")
	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", storedMsg.Content.String)

	// Verify attachments were stored
	attachments, err := ts.db.GetMessageAttachmentsByMessageID(ctx, storedMsg.ID)
	require.NoError(t, err)
	assert.Len(t, attachments, 1)
	assert.Equal(t, "image.png", attachments[0].Filename)
}

func TestGetMessages_Success_CacheHit(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Pre-populate database with messages
	message := &models.Message{
		DiscordMessageID: "cached_msg",
		ChannelID:        channel.ID,
		AuthorID:         "author123",
		AuthorUsername:   "cachedauthor",
		Content:          sql.NullString{String: "Cached message", Valid: true},
		Timestamp:        time.Now().UTC(),
		MessageType:      models.MessageTypeDefault,
	}
	err := ts.db.CreateOrUpdateMessage(ctx, message)
	require.NoError(t, err)

	// Set cache as valid
	err = ts.cacheManager.SetMessageCache(ctx, channel.DiscordChannelID, userID)
	require.NoError(t, err)

	// Setup mock to fail (should NOT be called)
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Discord API should not be called when cache is valid")
		w.WriteHeader(http.StatusInternalServerError)
	})

	// Call GetMessages (should use cache)
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: sessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     50,
	})

	// Verify response came from cache
	require.NoError(t, err)
	assert.True(t, resp.FromCache, "Should be served from cache")
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "Cached message", resp.Messages[0].Content)
}

func TestGetMessages_Pagination_Before(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Setup mock response
	timestamp := time.Now().UTC().Format(time.RFC3339)
	mockMessages := []*auth.DiscordMessage{
		{
			ID:        "msg10",
			ChannelID: channel.DiscordChannelID,
			Author:    auth.DiscordUser{ID: "author1", Username: "user1"},
			Content:   "Message 10",
			Timestamp: timestamp,
			Type:      0,
		},
	}
	ts.setupMockMessagesResponse(channel.DiscordChannelID, mockMessages)

	// Call GetMessages with before cursor
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: sessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     10,
		Before:    "msg20", // Get messages before msg20
	})

	// Verify response
	require.NoError(t, err)
	assert.False(t, resp.FromCache, "Paginated requests should not use cache")
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "msg10", resp.Messages[0].DiscordMessageId)
}

func TestGetMessages_Pagination_After(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Setup mock response
	timestamp := time.Now().UTC().Format(time.RFC3339)
	mockMessages := []*auth.DiscordMessage{
		{
			ID:        "msg30",
			ChannelID: channel.DiscordChannelID,
			Author:    auth.DiscordUser{ID: "author1", Username: "user1"},
			Content:   "Message 30",
			Timestamp: timestamp,
			Type:      0,
		},
	}
	ts.setupMockMessagesResponse(channel.DiscordChannelID, mockMessages)

	// Call GetMessages with after cursor
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: sessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     10,
		After:     "msg20", // Get messages after msg20
	})

	// Verify response
	require.NoError(t, err)
	assert.False(t, resp.FromCache, "Paginated requests should not use cache")
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "msg30", resp.Messages[0].DiscordMessageId)
}

func TestGetMessages_ForceRefresh_BypassesCache(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, userID, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Set cache
	err := ts.cacheManager.SetMessageCache(ctx, channel.DiscordChannelID, userID)
	require.NoError(t, err)

	// Setup mock with new data
	timestamp := time.Now().UTC().Format(time.RFC3339)
	mockMessages := []*auth.DiscordMessage{
		{
			ID:        "fresh_msg",
			ChannelID: channel.DiscordChannelID,
			Author:    auth.DiscordUser{ID: "author1", Username: "user1"},
			Content:   "Fresh message from API",
			Timestamp: timestamp,
			Type:      0,
		},
	}
	ts.setupMockMessagesResponse(channel.DiscordChannelID, mockMessages)

	// Call GetMessages with force_refresh
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId:    sessionID,
		ChannelId:    channel.DiscordChannelID,
		Limit:        50,
		ForceRefresh: true,
	})

	// Verify response came from API
	require.NoError(t, err)
	assert.False(t, resp.FromCache, "force_refresh should bypass cache")
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "Fresh message from API", resp.Messages[0].Content)
}

func TestGetMessages_InvalidSession(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Call with invalid session
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: "invalid_session",
		ChannelId: "channel123",
		Limit:     50,
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "invalid session")
}

func TestGetMessages_NoChannelAccess(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Create user and session but NO channel access
	user := &models.User{
		DiscordID:     "discord123",
		Username:      "testuser",
		Discriminator: sql.NullString{String: "1234", Valid: true},
	}
	err := ts.db.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create OAuth token
	accessToken, _ := ts.discordClient.EncryptToken("test_token")
	refreshToken, _ := ts.discordClient.EncryptToken("test_refresh")
	oauthToken := &models.OAuthToken{
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
		Scope:        "identify",
	}
	err = ts.db.StoreOAuthToken(ctx, oauthToken)
	require.NoError(t, err)

	// Create session
	session := &models.AuthSession{
		SessionID:  "test_session",
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		AuthStatus: "authenticated",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}
	err = ts.db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Create guild and channel (but user is NOT in guild)
	guild := &models.Guild{
		DiscordGuildID: "guild123",
		Name:           "Restricted Guild",
	}
	err = ts.db.CreateOrUpdateGuild(ctx, guild)
	require.NoError(t, err)

	channel := &models.Channel{
		DiscordChannelID: "channel123",
		GuildID:          guild.ID,
		Name:             "restricted-channel",
		Type:             models.ChannelTypeGuildText,
	}
	err = ts.db.CreateOrUpdateChannel(ctx, channel)
	require.NoError(t, err)

	// Call GetMessages (user has no access)
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: session.SessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     50,
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "don't have access")
}

func TestGetMessages_ChannelNotFound(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	// Create user and session
	user := &models.User{
		DiscordID: "discord123",
		Username:  "testuser",
	}
	err := ts.db.CreateUser(ctx, user)
	require.NoError(t, err)

	accessToken, _ := ts.discordClient.EncryptToken("test_token")
	refreshToken, _ := ts.discordClient.EncryptToken("test_refresh")
	oauthToken := &models.OAuthToken{
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour),
		Scope:        "identify",
	}
	err = ts.db.StoreOAuthToken(ctx, oauthToken)
	require.NoError(t, err)

	session := &models.AuthSession{
		SessionID:  "test_session",
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		AuthStatus: "authenticated",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}
	err = ts.db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Call GetMessages for non-existent channel
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: session.SessionID,
		ChannelId: "nonexistent_channel",
		Limit:     50,
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	// Could be PermissionDenied (no access) or NotFound (doesn't exist)
	assert.Contains(t, []codes.Code{codes.PermissionDenied, codes.NotFound}, st.Code())
}

func TestGetMessages_DiscordAPIError(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Setup mock to return error
	ts.mockDiscord.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "Internal Server Error"}`))
	})

	// Call GetMessages
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: sessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     50,
	})

	// Verify error
	assert.Nil(t, resp)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to fetch messages from Discord API")
}

func TestGetMessages_HasMoreFlag(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()
	ctx := context.Background()

	sessionID, _, channel := ts.createAuthenticatedSessionWithChannel(ctx, t)

	// Setup mock to return exactly 10 messages (the limit)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	mockMessages := make([]*auth.DiscordMessage, 10)
	for i := 0; i < 10; i++ {
		mockMessages[i] = &auth.DiscordMessage{
			ID:        "msg" + string(rune('0'+i)),
			ChannelID: channel.DiscordChannelID,
			Author:    auth.DiscordUser{ID: "author1", Username: "user1"},
			Content:   "Message content",
			Timestamp: timestamp,
			Type:      0,
		}
	}
	ts.setupMockMessagesResponse(channel.DiscordChannelID, mockMessages)

	// Call GetMessages with limit 10
	resp, err := ts.server.GetMessages(ctx, &messagev1.GetMessagesRequest{
		SessionId: sessionID,
		ChannelId: channel.DiscordChannelID,
		Limit:     10,
	})

	// Verify HasMore is true when we got exactly limit messages
	require.NoError(t, err)
	assert.Len(t, resp.Messages, 10)
	assert.True(t, resp.HasMore, "HasMore should be true when message count equals limit")
}

// ============================================================================
// StreamMessages Tests
// ============================================================================

func TestStreamMessages_WebSocketDisabled(t *testing.T) {
	ts := setupMessageServiceTest(t)
	defer ts.cleanup()

	// WebSocket is disabled in test setup (wsManager returns nil from IsEnabled)
	// Verify that StreamMessages returns Unavailable error

	err := ts.server.StreamMessages(
		&messagev1.StreamMessagesRequest{
			SessionId:  "test_session",
			ChannelIds: []string{"channel1", "channel2"},
		},
		nil, // stream is nil for this basic test
	)

	// Verify error
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	assert.Contains(t, st.Message(), "WebSocket support is not enabled")
}
