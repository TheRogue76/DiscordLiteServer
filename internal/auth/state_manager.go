package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// StateManager handles OAuth state generation and validation
type StateManager struct {
	db                 *database.DB
	stateExpiryMinutes int
}

// NewStateManager creates a new state manager
func NewStateManager(db *database.DB, stateExpiryMinutes int) *StateManager {
	return &StateManager{
		db:                 db,
		stateExpiryMinutes: stateExpiryMinutes,
	}
}

// GenerateState generates a cryptographically secure random state
func (sm *StateManager) GenerateState() (string, error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}

	// Encode to base64 URL-safe string
	state := base64.URLEncoding.EncodeToString(b)
	return state, nil
}

// StoreState stores a state in the database with an expiry time
func (sm *StateManager) StoreState(ctx context.Context, state, sessionID string) error {
	expiresAt := time.Now().Add(time.Duration(sm.stateExpiryMinutes) * time.Minute)

	oauthState := &models.OAuthState{
		State:     state,
		SessionID: sessionID,
		ExpiresAt: expiresAt,
	}

	if err := sm.db.CreateOAuthState(ctx, oauthState); err != nil {
		return fmt.Errorf("failed to store state: %w", err)
	}

	return nil
}

// ValidateState validates and deletes a state (single-use)
func (sm *StateManager) ValidateState(ctx context.Context, state string) (string, error) {
	oauthState, err := sm.db.ValidateAndDeleteOAuthState(ctx, state)
	if err != nil {
		return "", fmt.Errorf("state validation failed: %w", err)
	}

	return oauthState.SessionID, nil
}
