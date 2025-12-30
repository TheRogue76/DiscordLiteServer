package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// CreateWebSocketSession creates a new WebSocket session
func (db *DB) CreateWebSocketSession(ctx context.Context, session *models.WebSocketSession) error {
	query := `
		INSERT INTO websocket_sessions (
			session_id, user_id, gateway_url, session_token, sequence_number,
			status, last_heartbeat_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id) DO UPDATE
		SET gateway_url = EXCLUDED.gateway_url,
		    session_token = EXCLUDED.session_token,
		    sequence_number = EXCLUDED.sequence_number,
		    status = EXCLUDED.status,
		    last_heartbeat_at = EXCLUDED.last_heartbeat_at,
		    expires_at = EXCLUDED.expires_at,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := db.QueryRowContext(
		ctx,
		query,
		session.SessionID,
		session.UserID,
		session.GatewayURL,
		session.SessionToken,
		session.SequenceNumber,
		session.Status,
		session.LastHeartbeatAt,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create WebSocket session: %w", err)
	}

	return nil
}

// GetWebSocketSessionByID retrieves a WebSocket session by its internal ID
func (db *DB) GetWebSocketSessionByID(ctx context.Context, id int64) (*models.WebSocketSession, error) {
	query := `
		SELECT id, session_id, user_id, gateway_url, session_token, sequence_number,
		       status, last_heartbeat_at, created_at, updated_at, expires_at
		FROM websocket_sessions
		WHERE id = $1
	`

	var session models.WebSocketSession
	err := db.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&session.SessionID,
		&session.UserID,
		&session.GatewayURL,
		&session.SessionToken,
		&session.SequenceNumber,
		&session.Status,
		&session.LastHeartbeatAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("WebSocket session not found")
		}
		return nil, fmt.Errorf("failed to get WebSocket session: %w", err)
	}

	return &session, nil
}

// GetWebSocketSessionBySessionID retrieves a WebSocket session by its session ID
func (db *DB) GetWebSocketSessionBySessionID(ctx context.Context, sessionID string) (*models.WebSocketSession, error) {
	query := `
		SELECT id, session_id, user_id, gateway_url, session_token, sequence_number,
		       status, last_heartbeat_at, created_at, updated_at, expires_at
		FROM websocket_sessions
		WHERE session_id = $1
	`

	var session models.WebSocketSession
	err := db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID,
		&session.SessionID,
		&session.UserID,
		&session.GatewayURL,
		&session.SessionToken,
		&session.SequenceNumber,
		&session.Status,
		&session.LastHeartbeatAt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&session.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("WebSocket session not found")
		}
		return nil, fmt.Errorf("failed to get WebSocket session: %w", err)
	}

	return &session, nil
}

// GetWebSocketSessionsByUserID retrieves all WebSocket sessions for a user
func (db *DB) GetWebSocketSessionsByUserID(ctx context.Context, userID int64) ([]*models.WebSocketSession, error) {
	query := `
		SELECT id, session_id, user_id, gateway_url, session_token, sequence_number,
		       status, last_heartbeat_at, created_at, updated_at, expires_at
		FROM websocket_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query WebSocket sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []*models.WebSocketSession
	for rows.Next() {
		var session models.WebSocketSession
		err := rows.Scan(
			&session.ID,
			&session.SessionID,
			&session.UserID,
			&session.GatewayURL,
			&session.SessionToken,
			&session.SequenceNumber,
			&session.Status,
			&session.LastHeartbeatAt,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan WebSocket session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating WebSocket sessions: %w", err)
	}

	return sessions, nil
}

// UpdateWebSocketSessionStatus updates the status of a WebSocket session
func (db *DB) UpdateWebSocketSessionStatus(ctx context.Context, sessionID string, status models.WebSocketStatus) error {
	query := `
		UPDATE websocket_sessions
		SET status = $1, updated_at = NOW()
		WHERE session_id = $2
	`

	result, err := db.ExecContext(ctx, query, status, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update WebSocket session status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("WebSocket session not found")
	}

	return nil
}

// UpdateWebSocketSessionHeartbeat updates the last heartbeat timestamp
func (db *DB) UpdateWebSocketSessionHeartbeat(ctx context.Context, sessionID string) error {
	query := `
		UPDATE websocket_sessions
		SET last_heartbeat_at = NOW(), updated_at = NOW()
		WHERE session_id = $1
	`

	result, err := db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update WebSocket session heartbeat: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("WebSocket session not found")
	}

	return nil
}

// UpdateWebSocketSessionSequence updates the sequence number for resume capability
func (db *DB) UpdateWebSocketSessionSequence(ctx context.Context, sessionID string, sequenceNumber int64) error {
	query := `
		UPDATE websocket_sessions
		SET sequence_number = $1, updated_at = NOW()
		WHERE session_id = $2
	`

	result, err := db.ExecContext(ctx, query, sequenceNumber, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update WebSocket session sequence: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("WebSocket session not found")
	}

	return nil
}

// DeleteWebSocketSession removes a WebSocket session
func (db *DB) DeleteWebSocketSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM websocket_sessions WHERE session_id = $1`

	result, err := db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete WebSocket session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("WebSocket session not found")
	}

	return nil
}

// GetActiveWebSocketSessions retrieves all active (connected) WebSocket sessions
func (db *DB) GetActiveWebSocketSessions(ctx context.Context) ([]*models.WebSocketSession, error) {
	query := `
		SELECT id, session_id, user_id, gateway_url, session_token, sequence_number,
		       status, last_heartbeat_at, created_at, updated_at, expires_at
		FROM websocket_sessions
		WHERE status = $1 AND expires_at > NOW()
		ORDER BY created_at DESC
	`

	rows, err := db.QueryContext(ctx, query, models.WebSocketStatusConnected)
	if err != nil {
		return nil, fmt.Errorf("failed to query active WebSocket sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []*models.WebSocketSession
	for rows.Next() {
		var session models.WebSocketSession
		err := rows.Scan(
			&session.ID,
			&session.SessionID,
			&session.UserID,
			&session.GatewayURL,
			&session.SessionToken,
			&session.SequenceNumber,
			&session.Status,
			&session.LastHeartbeatAt,
			&session.CreatedAt,
			&session.UpdatedAt,
			&session.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan WebSocket session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating WebSocket sessions: %w", err)
	}

	return sessions, nil
}

// CleanupExpiredWebSocketSessions removes expired WebSocket sessions
func (db *DB) CleanupExpiredWebSocketSessions(ctx context.Context) error {
	query := `DELETE FROM websocket_sessions WHERE expires_at < NOW()`

	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired WebSocket sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		fmt.Printf("Cleaned up %d expired WebSocket sessions\n", rowsAffected)
	}

	return nil
}

// CleanupStaleWebSocketSessions removes sessions with no heartbeat for a given duration
func (db *DB) CleanupStaleWebSocketSessions(ctx context.Context, staleDuration time.Duration) error {
	staleThreshold := time.Now().Add(-staleDuration)

	query := `
		UPDATE websocket_sessions
		SET status = $1, updated_at = NOW()
		WHERE status = $2 AND last_heartbeat_at < $3
	`

	result, err := db.ExecContext(ctx, query, models.WebSocketStatusDisconnected, models.WebSocketStatusConnected, staleThreshold)
	if err != nil {
		return fmt.Errorf("failed to cleanup stale WebSocket sessions: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		fmt.Printf("Marked %d stale WebSocket sessions as disconnected\n", rowsAffected)
	}

	return nil
}

// StartWebSocketCleanupJob starts a background job that periodically cleans up expired/stale WebSocket sessions
func (db *DB) StartWebSocketCleanupJob(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping WebSocket cleanup job")
			return
		case <-ticker.C:
			// Cleanup expired sessions
			err := db.CleanupExpiredWebSocketSessions(ctx)
			if err != nil {
				fmt.Printf("Error during WebSocket cleanup: %v\n", err)
			}

			// Mark stale sessions (no heartbeat for 5 minutes) as disconnected
			err = db.CleanupStaleWebSocketSessions(ctx, 5*time.Minute)
			if err != nil {
				fmt.Printf("Error during stale WebSocket cleanup: %v\n", err)
			}
		}
	}
}
