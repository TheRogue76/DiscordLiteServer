// Package auth provides Discord OAuth2 authentication functionality including
// token exchange, user information retrieval, and token encryption/decryption.
package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
	"github.com/parsascontentcorner/discordliteserver/internal/ratelimit"
)

const (
	discordAPIEndpoint = "https://discord.com/api/v10"
	discordAuthURL     = "https://discord.com/oauth2/authorize"
	discordTokenURL    = "https://discord.com/api/oauth2/token" //nolint:gosec // Not a hardcoded credential, just an API endpoint URL
)

// DiscordUser represents a Discord user from the API
type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
}

// DiscordGuild represents a Discord guild (server) from the API
type DiscordGuild struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Icon        string   `json:"icon"`
	Owner       bool     `json:"owner"`
	Permissions string   `json:"permissions"`
	Features    []string `json:"features"`
}

// DiscordChannel represents a Discord channel from the API
type DiscordChannel struct {
	ID            string `json:"id"`
	Type          int    `json:"type"`
	GuildID       string `json:"guild_id"`
	Position      int    `json:"position"`
	Name          string `json:"name"`
	Topic         string `json:"topic"`
	NSFW          bool   `json:"nsfw"`
	LastMessageID string `json:"last_message_id"`
	ParentID      string `json:"parent_id"`
}

// DiscordMessage represents a Discord message from the API
type DiscordMessage struct {
	ID               string                   `json:"id"`
	ChannelID        string                   `json:"channel_id"`
	Author           DiscordUser              `json:"author"`
	Content          string                   `json:"content"`
	Timestamp        string                   `json:"timestamp"`
	EditedTimestamp  *string                  `json:"edited_timestamp"`
	Type             int                      `json:"type"`
	MessageReference *DiscordMessageReference `json:"message_reference"`
	Attachments      []DiscordAttachment      `json:"attachments"`
}

// DiscordMessageReference represents a message reference (for replies)
type DiscordMessageReference struct {
	MessageID string `json:"message_id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
}

// DiscordAttachment represents a file attachment in a message
type DiscordAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url"`
	Height      *int   `json:"height"`
	Width       *int   `json:"width"`
	ContentType string `json:"content_type"`
}

// DiscordClient handles Discord OAuth operations
type DiscordClient struct {
	config        *oauth2.Config
	encryptionKey []byte
	logger        *zap.Logger
	baseURL       string // Discord API base URL (configurable for testing)
	rateLimiter   *ratelimit.RateLimiter
	botToken      string // Bot token for Discord API access (guild channels, messages, gateway)
}

// NewDiscordClient creates a new Discord OAuth client
func NewDiscordClient(cfg *config.Config, logger *zap.Logger) *DiscordClient {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.Discord.ClientID,
		ClientSecret: cfg.Discord.ClientSecret,
		RedirectURL:  cfg.Discord.RedirectURI,
		Scopes:       cfg.Discord.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  discordAuthURL,
			TokenURL: discordTokenURL,
		},
	}

	return &DiscordClient{
		config:        oauthConfig,
		encryptionKey: cfg.Security.TokenEncryptionKey,
		logger:        logger,
		baseURL:       discordAPIEndpoint,
		botToken:      cfg.Discord.BotToken,
	}
}

// GetAuthURL constructs the Discord OAuth authorization URL
func (dc *DiscordClient) GetAuthURL(state string) string {
	return dc.config.AuthCodeURL(state)
}

// ExchangeCode exchanges an authorization code for an access token
func (dc *DiscordClient) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := dc.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	dc.logger.Debug("successfully exchanged code for token",
		zap.String("token_type", token.TokenType),
		zap.Time("expiry", token.Expiry),
	)

	return token, nil
}

// GetUserInfo fetches user information from Discord API
func (dc *DiscordClient) GetUserInfo(ctx context.Context, accessToken string) (*DiscordUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", dc.baseURL+"/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			dc.logger.Warn("failed to close response body", zap.Error(err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API returned status %d: %s", resp.StatusCode, string(body))
	}

	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	dc.logger.Debug("fetched user info from Discord",
		zap.String("discord_id", user.ID),
		zap.String("username", user.Username),
	)

	return &user, nil
}

// EncryptToken encrypts a token using AES-256-GCM
func (dc *DiscordClient) EncryptToken(plaintext string) (string, error) {
	block, err := aes.NewCipher(dc.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken decrypts a token using AES-256-GCM
func (dc *DiscordClient) DecryptToken(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(dc.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// SetRateLimiter sets the rate limiter for the Discord client
func (dc *DiscordClient) SetRateLimiter(rl *ratelimit.RateLimiter) {
	dc.rateLimiter = rl
}

// SetBaseURL sets the base URL for the Discord API (used for testing)
func (dc *DiscordClient) SetBaseURL(url string) {
	dc.baseURL = url
	// Also update OAuth token endpoint for testing
	dc.config.Endpoint.TokenURL = url + "/oauth2/token"
}

// RefreshToken refreshes an OAuth token using the refresh token
func (dc *DiscordClient) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := dc.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	dc.logger.Debug("successfully refreshed OAuth token",
		zap.Time("new_expiry", newToken.Expiry),
	)

	return newToken, nil
}

// RefreshIfNeeded checks if token is expiring soon and refreshes if needed
// Returns: (accessToken, wasRefreshed, error)
func (dc *DiscordClient) RefreshIfNeeded(ctx context.Context, oauthToken *models.OAuthToken) (string, bool, error) {
	// Check if token expires within 5 minutes
	expiryBuffer := 5 * time.Minute
	if time.Now().Add(expiryBuffer).After(oauthToken.Expiry) {
		dc.logger.Info("OAuth token expiring soon, refreshing",
			zap.Time("expiry", oauthToken.Expiry),
		)

		// Decrypt refresh token
		refreshToken, err := dc.DecryptToken(oauthToken.RefreshToken)
		if err != nil {
			return "", false, fmt.Errorf("failed to decrypt refresh token: %w", err)
		}

		// Refresh the token
		newToken, err := dc.RefreshToken(ctx, refreshToken)
		if err != nil {
			return "", false, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Encrypt new tokens
		encryptedAccessToken, err := dc.EncryptToken(newToken.AccessToken)
		if err != nil {
			return "", false, fmt.Errorf("failed to encrypt access token: %w", err)
		}

		encryptedRefreshToken, err := dc.EncryptToken(newToken.RefreshToken)
		if err != nil {
			return "", false, fmt.Errorf("failed to encrypt refresh token: %w", err)
		}

		// Update the oauth token struct (caller should save to database)
		oauthToken.AccessToken = encryptedAccessToken
		oauthToken.RefreshToken = encryptedRefreshToken
		oauthToken.Expiry = newToken.Expiry

		return newToken.AccessToken, true, nil
	}

	// Token is still valid, decrypt and return
	accessToken, err := dc.DecryptToken(oauthToken.AccessToken)
	if err != nil {
		return "", false, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	return accessToken, false, nil
}

// makeAPIRequest makes a rate-limited HTTP request to Discord API
func (dc *DiscordClient) makeAPIRequest(ctx context.Context, method, endpoint, accessToken string) (*http.Response, error) {
	// Wait for rate limit if limiter is set
	if dc.rateLimiter != nil {
		if err := dc.rateLimiter.Wait(endpoint); err != nil {
			return nil, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, dc.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	// Update rate limit info from headers
	if dc.rateLimiter != nil {
		dc.rateLimiter.UpdateFromHeaders(endpoint, resp.Header)
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		defer func() { _ = resp.Body.Close() }()
		if dc.rateLimiter != nil {
			_ = dc.rateLimiter.HandleRateLimitResponse(endpoint, resp.Header)
		}
		return nil, fmt.Errorf("rate limited by Discord API")
	}

	return resp, nil
}

// GetUserGuilds fetches the user's guilds from Discord API
func (dc *DiscordClient) GetUserGuilds(ctx context.Context, accessToken string) ([]*DiscordGuild, error) {
	resp, err := dc.makeAPIRequest(ctx, "GET", "/users/@me/guilds", accessToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API returned status %d: %s", resp.StatusCode, string(body))
	}

	var guilds []*DiscordGuild
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return nil, fmt.Errorf("failed to decode guilds: %w", err)
	}

	dc.logger.Debug("fetched user guilds from Discord",
		zap.Int("guild_count", len(guilds)),
	)

	return guilds, nil
}

// GetGuildChannels fetches channels for a guild from Discord API
func (dc *DiscordClient) GetGuildChannels(ctx context.Context, accessToken, guildID string) ([]*DiscordChannel, error) {
	endpoint := "/guilds/" + guildID + "/channels"
	resp, err := dc.makeAPIRequestWithBot(ctx, "GET", endpoint)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API returned status %d: %s", resp.StatusCode, string(body))
	}

	var channels []*DiscordChannel
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		return nil, fmt.Errorf("failed to decode channels: %w", err)
	}

	dc.logger.Debug("fetched guild channels from Discord",
		zap.String("guild_id", guildID),
		zap.Int("channel_count", len(channels)),
	)

	return channels, nil
}

// GetChannelMessages fetches messages from a channel with pagination
func (dc *DiscordClient) GetChannelMessages(ctx context.Context, accessToken, channelID string, limit int, before, after string) ([]*DiscordMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Build query parameters
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	if before != "" {
		params.Set("before", before)
	}
	if after != "" {
		params.Set("after", after)
	}

	endpoint := "/channels/" + channelID + "/messages?" + params.Encode()
	resp, err := dc.makeAPIRequest(ctx, "GET", endpoint, accessToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API returned status %d: %s", resp.StatusCode, string(body))
	}

	var messages []*DiscordMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	dc.logger.Debug("fetched channel messages from Discord",
		zap.String("channel_id", channelID),
		zap.Int("message_count", len(messages)),
	)

	return messages, nil
}

// makeAPIRequestWithBot makes a rate-limited HTTP request using bot token
// This method is similar to makeAPIRequest but uses the bot token instead of user OAuth token
func (dc *DiscordClient) makeAPIRequestWithBot(ctx context.Context, method, endpoint string) (*http.Response, error) {
	if dc.botToken == "" {
		return nil, fmt.Errorf("bot token is not configured")
	}

	// Wait for rate limit if limiter is set
	if dc.rateLimiter != nil {
		if err := dc.rateLimiter.Wait(endpoint); err != nil {
			return nil, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, dc.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// CRITICAL: Bot tokens use "Bot" prefix, not "Bearer"
	req.Header.Set("Authorization", "Bot "+dc.botToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	// Update rate limit info from headers
	if dc.rateLimiter != nil {
		dc.rateLimiter.UpdateFromHeaders(endpoint, resp.Header)
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		defer func() { _ = resp.Body.Close() }()
		if dc.rateLimiter != nil {
			_ = dc.rateLimiter.HandleRateLimitResponse(endpoint, resp.Header)
		}
		return nil, fmt.Errorf("rate limited by Discord API")
	}

	return resp, nil
}
