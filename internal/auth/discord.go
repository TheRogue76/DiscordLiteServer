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

	"golang.org/x/oauth2"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/config"
)

const (
	discordAPIEndpoint = "https://discord.com/api/v10"
	discordAuthURL     = "https://discord.com/oauth2/authorize"
	discordTokenURL    = "https://discord.com/api/oauth2/token"
)

// DiscordUser represents a Discord user from the API
type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
}

// DiscordClient handles Discord OAuth operations
type DiscordClient struct {
	config        *oauth2.Config
	encryptionKey []byte
	logger        *zap.Logger
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
	req, err := http.NewRequestWithContext(ctx, "GET", discordAPIEndpoint+"/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

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
