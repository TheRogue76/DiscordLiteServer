package auth

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/testutil"
)

func TestNewDiscordClient(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()

	client := NewDiscordClient(cfg, logger)

	assert.NotNil(t, client)
	assert.NotNil(t, client.config)
	assert.Equal(t, cfg.Discord.ClientID, client.config.ClientID)
	assert.Equal(t, cfg.Discord.ClientSecret, client.config.ClientSecret)
	assert.Equal(t, cfg.Discord.RedirectURI, client.config.RedirectURL)
	assert.NotNil(t, client.encryptionKey)
	assert.Equal(t, 32, len(client.encryptionKey))
}

func TestGetAuthURL(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	state := "test_state_123"
	authURL := client.GetAuthURL(state)

	// Verify URL contains required parameters
	assert.Contains(t, authURL, "discord.com/oauth2/authorize")
	assert.Contains(t, authURL, "client_id="+cfg.Discord.ClientID)
	assert.Contains(t, authURL, "redirect_uri=")
	assert.Contains(t, authURL, "response_type=code")
	assert.Contains(t, authURL, "state="+state)
	assert.Contains(t, authURL, "scope=")
}

func TestGetAuthURL_MultipleStates(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	state1 := "state_1"
	state2 := "state_2"

	url1 := client.GetAuthURL(state1)
	url2 := client.GetAuthURL(state2)

	assert.Contains(t, url1, "state="+state1)
	assert.Contains(t, url2, "state="+state2)
	assert.NotEqual(t, url1, url2)
}

func TestExchangeCode_Success(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Override endpoints to use mock server
	client.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	ctx := context.Background()
	token, err := client.ExchangeCode(ctx, "valid_code")

	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "mock_access_token_123", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, "mock_refresh_token_456", token.RefreshToken)
	assert.Equal(t, 1, mockServer.TokenCalls)
}

func TestExchangeCode_InvalidCode(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	client.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	ctx := context.Background()
	token, err := client.ExchangeCode(ctx, "error_code")

	assert.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "oauth2")
}

func TestExchangeCode_ServerError(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	client.config.Endpoint.TokenURL = mockServer.GetTokenURL()

	ctx := context.Background()
	token, err := client.ExchangeCode(ctx, "server_error")

	assert.Error(t, err)
	assert.Nil(t, token)
}

func TestGetUserInfo_Success(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Override base URL to use mock server
	client.baseURL = mockServer.Server.URL + "/api/v10"

	ctx := context.Background()
	user, err := client.GetUserInfo(ctx, "mock_access_token_123")

	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "123456789012345678", user.ID)
	assert.Equal(t, "TestUser", user.Username)
	assert.Equal(t, "1234", user.Discriminator)
	assert.Equal(t, "avatar_hash_123", user.Avatar)
	assert.Equal(t, "testuser@example.com", user.Email)
	assert.Equal(t, 1, mockServer.UserInfoCalls)
}

func TestGetUserInfo_Unauthorized(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Override base URL to use mock server
	client.baseURL = mockServer.Server.URL + "/api/v10"

	ctx := context.Background()
	user, err := client.GetUserInfo(ctx, "invalid_token")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "401")
}

func TestGetUserInfo_NotFound(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Override base URL to use mock server
	client.baseURL = mockServer.Server.URL + "/api/v10"

	ctx := context.Background()
	user, err := client.GetUserInfo(ctx, "not_found")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "404")
}

func TestGetUserInfo_ServerError(t *testing.T) {
	mockServer := testutil.NewMockDiscordServer()
	defer mockServer.Close()

	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Override base URL to use mock server
	client.baseURL = mockServer.Server.URL + "/api/v10"

	ctx := context.Background()
	user, err := client.GetUserInfo(ctx, "server_error")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "500")
}

func TestEncryptToken(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	plaintext := "my_secret_token_12345"
	encrypted, err := client.EncryptToken(plaintext)

	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)

	// Verify it can be decrypted back to the original
	decrypted, err := client.DecryptToken(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptToken_Success(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Test various token lengths
	tests := []struct {
		name      string
		plaintext string
	}{
		{"short token", "token123"},
		{"medium token", "this_is_a_medium_length_access_token_value"},
		{"long token", "very_long_token_" + strings.Repeat("a", 100)},
		{"unicode token", "token_with_unicode_🔐_chars"},
		{"special chars", "token!@#$%^&*()_+-=[]{}|;:',.<>?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := client.EncryptToken(tt.plaintext)
			require.NoError(t, err)

			decrypted, err := client.DecryptToken(encrypted)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestDecryptToken_InvalidBase64(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	invalidBase64 := "not-valid-base64!!!"
	decrypted, err := client.DecryptToken(invalidBase64)

	assert.Error(t, err)
	assert.Empty(t, decrypted)
	assert.Contains(t, err.Error(), "base64")
}

func TestDecryptToken_WrongKey(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client1 := NewDiscordClient(cfg, logger)

	// Encrypt with client1
	plaintext := "secret_token"
	encrypted, err := client1.EncryptToken(plaintext)
	require.NoError(t, err)

	// Try to decrypt with client2 (different key)
	cfg2 := testutil.GenerateTestConfig()
	client2 := NewDiscordClient(cfg2, logger)

	decrypted, err := client2.DecryptToken(encrypted)

	// Should error or return garbage
	if err == nil {
		// If no error, decrypted should not match original
		assert.NotEqual(t, plaintext, decrypted)
	}
}

func TestDecryptToken_TruncatedCiphertext(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	// Create valid encrypted token
	encrypted, err := client.EncryptToken("test_token")
	require.NoError(t, err)

	// Truncate the ciphertext (remove last few characters)
	decoded, _ := base64.URLEncoding.DecodeString(encrypted)
	if len(decoded) > 10 {
		truncated := base64.URLEncoding.EncodeToString(decoded[:10])

		decrypted, err := client.DecryptToken(truncated)
		assert.Error(t, err)
		assert.Empty(t, decrypted)
	}
}

func TestEncryption_NonceUniqueness(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()
	client := NewDiscordClient(cfg, logger)

	plaintext := "test_token_for_nonce_test"
	encrypted := make([]string, 100)

	// Encrypt same plaintext 100 times
	for i := 0; i < 100; i++ {
		enc, err := client.EncryptToken(plaintext)
		require.NoError(t, err)
		encrypted[i] = enc
	}

	// All ciphertexts should be different (due to unique nonces)
	seen := make(map[string]bool)
	for _, enc := range encrypted {
		assert.False(t, seen[enc], "Nonce should be unique for each encryption")
		seen[enc] = true
	}

	// But all should decrypt to same plaintext
	for _, enc := range encrypted {
		dec, err := client.DecryptToken(enc)
		require.NoError(t, err)
		assert.Equal(t, plaintext, dec)
	}
}

func TestEncryptionKeySize(t *testing.T) {
	cfg := testutil.GenerateTestConfig()
	logger, _ := zap.NewDevelopment()

	// Valid 32-byte key
	validKey := testutil.GenerateEncryptionKey()
	require.Equal(t, 32, len(validKey))

	// Create client with valid key
	cfg.Security.TokenEncryptionKey = validKey
	client := NewDiscordClient(cfg, logger)
	assert.NotNil(t, client)
	assert.Equal(t, 32, len(client.encryptionKey))

	// Test encryption works
	encrypted, err := client.EncryptToken("test")
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
}
