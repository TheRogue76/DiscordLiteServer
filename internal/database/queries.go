package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// CreateUser creates a new user or updates if exists
func (db *DB) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (discord_id, username, discriminator, avatar, email)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (discord_id)
		DO UPDATE SET
			username = EXCLUDED.username,
			discriminator = EXCLUDED.discriminator,
			avatar = EXCLUDED.avatar,
			email = EXCLUDED.email,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRowContext(ctx, query,
		user.DiscordID,
		user.Username,
		user.Discriminator,
		user.Avatar,
		user.Email,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByDiscordID retrieves a user by their Discord ID
func (db *DB) GetUserByDiscordID(ctx context.Context, discordID string) (*models.User, error) {
	query := `
		SELECT id, discord_id, username, discriminator, avatar, email, created_at, updated_at
		FROM users
		WHERE discord_id = $1
	`

	user := &models.User{}
	err := db.QueryRowContext(ctx, query, discordID).Scan(
		&user.ID,
		&user.DiscordID,
		&user.Username,
		&user.Discriminator,
		&user.Avatar,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by their database ID
func (db *DB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	query := `
		SELECT id, discord_id, username, discriminator, avatar, email, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user := &models.User{}
	err := db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.DiscordID,
		&user.Username,
		&user.Discriminator,
		&user.Avatar,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// StoreOAuthToken stores or updates an OAuth token (encrypted)
func (db *DB) StoreOAuthToken(ctx context.Context, token *models.OAuthToken) error {
	query := `
		INSERT INTO oauth_tokens (user_id, access_token, refresh_token, token_type, expiry, scope)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			scope = EXCLUDED.scope,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRowContext(ctx, query,
		token.UserID,
		token.AccessToken,
		token.RefreshToken,
		token.TokenType,
		token.Expiry,
		token.Scope,
	).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to store oauth token: %w", err)
	}

	return nil
}

// GetOAuthToken retrieves an OAuth token by user ID
func (db *DB) GetOAuthToken(ctx context.Context, userID int64) (*models.OAuthToken, error) {
	query := `
		SELECT id, user_id, access_token, refresh_token, token_type, expiry, scope, created_at, updated_at
		FROM oauth_tokens
		WHERE user_id = $1
	`

	token := &models.OAuthToken{}
	err := db.QueryRowContext(ctx, query, userID).Scan(
		&token.ID,
		&token.UserID,
		&token.AccessToken,
		&token.RefreshToken,
		&token.TokenType,
		&token.Expiry,
		&token.Scope,
		&token.CreatedAt,
		&token.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("oauth token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth token: %w", err)
	}

	return token, nil
}

// DeleteOAuthToken deletes an OAuth token
func (db *DB) DeleteOAuthToken(ctx context.Context, userID int64) error {
	query := `DELETE FROM oauth_tokens WHERE user_id = $1`

	result, err := db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete oauth token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("oauth token not found")
	}

	return nil
}

// CreateAuthSession creates a new authentication session
func (db *DB) CreateAuthSession(ctx context.Context, session *models.AuthSession) error {
	query := `
		INSERT INTO auth_sessions (session_id, user_id, auth_status, error_message, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`

	err := db.QueryRowContext(ctx, query,
		session.SessionID,
		session.UserID,
		session.AuthStatus,
		session.ErrorMessage,
		session.ExpiresAt,
	).Scan(&session.CreatedAt, &session.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create auth session: %w", err)
	}

	return nil
}

// GetAuthSession retrieves an authentication session by session ID
func (db *DB) GetAuthSession(ctx context.Context, sessionID string) (*models.AuthSession, error) {
	query := `
		SELECT session_id, user_id, auth_status, error_message, created_at, updated_at, expires_at
		FROM auth_sessions
		WHERE session_id = $1
	`

	session := &models.AuthSession{}
	err := db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.SessionID,
		&session.UserID,
		&session.AuthStatus,
		&session.ErrorMessage,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("auth session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get auth session: %w", err)
	}

	return session, nil
}

// UpdateAuthSessionStatus updates the status of an authentication session
func (db *DB) UpdateAuthSessionStatus(ctx context.Context, sessionID string, status string, userID *int64, errorMessage *string) error {
	query := `
		UPDATE auth_sessions
		SET auth_status = $2, user_id = $3, error_message = $4, updated_at = NOW()
		WHERE session_id = $1
	`

	var userIDVal sql.NullInt64
	if userID != nil {
		userIDVal = sql.NullInt64{Int64: *userID, Valid: true}
	}

	var errorMsgVal sql.NullString
	if errorMessage != nil {
		errorMsgVal = sql.NullString{String: *errorMessage, Valid: true}
	}

	result, err := db.ExecContext(ctx, query, sessionID, status, userIDVal, errorMsgVal)
	if err != nil {
		return fmt.Errorf("failed to update auth session status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("auth session not found")
	}

	return nil
}

// DeleteAuthSession deletes an authentication session
func (db *DB) DeleteAuthSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM auth_sessions WHERE session_id = $1`

	result, err := db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete auth session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("auth session not found")
	}

	return nil
}

// CreateOAuthState creates a new OAuth state for CSRF protection
func (db *DB) CreateOAuthState(ctx context.Context, state *models.OAuthState) error {
	query := `
		INSERT INTO oauth_states (state, session_id, expires_at)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`

	err := db.QueryRowContext(ctx, query,
		state.State,
		state.SessionID,
		state.ExpiresAt,
	).Scan(&state.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create oauth state: %w", err)
	}

	return nil
}

// ValidateAndDeleteOAuthState validates and deletes an OAuth state (single-use)
func (db *DB) ValidateAndDeleteOAuthState(ctx context.Context, state string) (*models.OAuthState, error) {
	// Start a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the state
	query := `
		SELECT state, session_id, created_at, expires_at
		FROM oauth_states
		WHERE state = $1
	`

	oauthState := &models.OAuthState{}
	err = tx.QueryRowContext(ctx, query, state).Scan(
		&oauthState.State,
		&oauthState.SessionID,
		&oauthState.CreatedAt,
		&oauthState.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid state: not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to validate oauth state: %w", err)
	}

	// Check if expired
	if oauthState.IsExpired() {
		return nil, fmt.Errorf("state has expired")
	}

	// Delete the state (single-use)
	deleteQuery := `DELETE FROM oauth_states WHERE state = $1`
	_, err = tx.ExecContext(ctx, deleteQuery, state)
	if err != nil {
		return nil, fmt.Errorf("failed to delete oauth state: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return oauthState, nil
}

// CleanupExpiredSessions deletes expired sessions and states
func (db *DB) CleanupExpiredSessions(ctx context.Context) error {
	// Delete expired auth sessions
	query1 := `DELETE FROM auth_sessions WHERE expires_at < NOW()`
	_, err := db.ExecContext(ctx, query1)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired auth sessions: %w", err)
	}

	// Delete expired OAuth states
	query2 := `DELETE FROM oauth_states WHERE expires_at < NOW()`
	_, err = db.ExecContext(ctx, query2)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired oauth states: %w", err)
	}

	db.logger.Debug("cleaned up expired sessions and states")
	return nil
}

// StartCleanupJob starts a background job to periodically cleanup expired sessions
func (db *DB) StartCleanupJob(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := db.CleanupExpiredSessions(ctx); err != nil {
					db.logger.Error("failed to cleanup expired sessions", zap.Error(err))
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	db.logger.Info("started cleanup job", zap.Duration("interval", interval))
}
