package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// OAuthHandler orchestrates the OAuth flow
type OAuthHandler struct {
	db            *database.DB
	discordClient *DiscordClient
	stateManager  *StateManager
	logger        *zap.Logger
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(db *database.DB, discordClient *DiscordClient, stateManager *StateManager, logger *zap.Logger) *OAuthHandler {
	return &OAuthHandler{
		db:            db,
		discordClient: discordClient,
		stateManager:  stateManager,
		logger:        logger,
	}
}

// HandleCallback processes the OAuth callback
func (oh *OAuthHandler) HandleCallback(ctx context.Context, code, state string) error {
	// 1. Validate state
	oh.logger.Debug("validating OAuth state", zap.String("state", state))
	sessionID, err := oh.stateManager.ValidateState(ctx, state)
	if err != nil {
		oh.logger.Error("state validation failed", zap.Error(err))
		return oh.updateSessionFailed(ctx, "", "invalid state")
	}

	oh.logger.Info("state validated successfully", zap.String("session_id", sessionID))

	// 2. Exchange code for token
	oh.logger.Debug("exchanging code for token", zap.String("session_id", sessionID))
	token, err := oh.discordClient.ExchangeCode(ctx, code)
	if err != nil {
		oh.logger.Error("failed to exchange code", zap.String("session_id", sessionID), zap.Error(err))
		return oh.updateSessionFailed(ctx, sessionID, "failed to exchange authorization code")
	}

	// 3. Fetch user info from Discord
	oh.logger.Debug("fetching user info from Discord", zap.String("session_id", sessionID))
	discordUser, err := oh.discordClient.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		oh.logger.Error("failed to fetch user info", zap.String("session_id", sessionID), zap.Error(err))
		return oh.updateSessionFailed(ctx, sessionID, "failed to fetch user information")
	}

	oh.logger.Info("fetched user info",
		zap.String("session_id", sessionID),
		zap.String("discord_id", discordUser.ID),
		zap.String("username", discordUser.Username),
	)

	// 4. Create or update user in database
	user := &models.User{
		DiscordID:     discordUser.ID,
		Username:      discordUser.Username,
		Discriminator: sql.NullString{String: discordUser.Discriminator, Valid: discordUser.Discriminator != ""},
		Avatar:        sql.NullString{String: discordUser.Avatar, Valid: discordUser.Avatar != ""},
		Email:         sql.NullString{String: discordUser.Email, Valid: discordUser.Email != ""},
	}

	if err := oh.db.CreateUser(ctx, user); err != nil {
		oh.logger.Error("failed to create/update user", zap.String("session_id", sessionID), zap.Error(err))
		return oh.updateSessionFailed(ctx, sessionID, "failed to save user data")
	}

	oh.logger.Info("user created/updated",
		zap.String("session_id", sessionID),
		zap.Int64("user_id", user.ID),
	)

	// 5. Encrypt and store OAuth tokens
	encryptedAccess, err := oh.discordClient.EncryptToken(token.AccessToken)
	if err != nil {
		oh.logger.Error("failed to encrypt access token", zap.String("session_id", sessionID), zap.Error(err))
		return oh.updateSessionFailed(ctx, sessionID, "failed to secure tokens")
	}

	encryptedRefresh, err := oh.discordClient.EncryptToken(token.RefreshToken)
	if err != nil {
		oh.logger.Error("failed to encrypt refresh token", zap.String("session_id", sessionID), zap.Error(err))
		return oh.updateSessionFailed(ctx, sessionID, "failed to secure tokens")
	}

	oauthToken := &models.OAuthToken{
		UserID:       user.ID,
		AccessToken:  encryptedAccess,
		RefreshToken: encryptedRefresh,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
		Scope:        strings.Join(oh.discordClient.config.Scopes, " "),
	}

	if err := oh.db.StoreOAuthToken(ctx, oauthToken); err != nil {
		oh.logger.Error("failed to store oauth token", zap.String("session_id", sessionID), zap.Error(err))
		return oh.updateSessionFailed(ctx, sessionID, "failed to store authentication data")
	}

	oh.logger.Info("oauth token stored successfully",
		zap.String("session_id", sessionID),
		zap.Int64("user_id", user.ID),
	)

	// 6. Update session status to authenticated
	if err := oh.db.UpdateAuthSessionStatus(ctx, sessionID, models.AuthStatusAuthenticated, &user.ID, nil); err != nil {
		oh.logger.Error("failed to update session status", zap.String("session_id", sessionID), zap.Error(err))
		return fmt.Errorf("failed to update session status: %w", err)
	}

	oh.logger.Info("authentication completed successfully",
		zap.String("session_id", sessionID),
		zap.Int64("user_id", user.ID),
	)

	return nil
}

// updateSessionFailed updates the session status to failed with an error message
func (oh *OAuthHandler) updateSessionFailed(ctx context.Context, sessionID, errorMessage string) error {
	if sessionID == "" {
		// Cannot update session if we don't have a session ID
		return fmt.Errorf("authentication failed: %s", errorMessage)
	}

	if err := oh.db.UpdateAuthSessionStatus(ctx, sessionID, models.AuthStatusFailed, nil, &errorMessage); err != nil {
		oh.logger.Error("failed to update session to failed status",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
	}

	return fmt.Errorf("authentication failed: %s", errorMessage)
}
