package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

const (
	// Discord Gateway version and encoding
	gatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"

	// Gateway opcodes
	opDispatch            = 0  // Receive: Event dispatch
	opHeartbeat           = 1  // Send/Receive: Heartbeat
	opIdentify            = 2  // Send: Identify (begin session)
	opPresenceUpdate      = 3  // Send: Presence update
	opVoiceStateUpdate    = 4  // Send: Voice state update
	opResume              = 6  // Send: Resume session
	opReconnect           = 7  // Receive: Reconnect
	opRequestGuildMembers = 8  // Send: Request guild members
	opInvalidSession      = 9  // Receive: Invalid session
	opHello               = 10 // Receive: Hello (heartbeat interval)
	opHeartbeatACK        = 11 // Receive: Heartbeat ACK

	// Gateway close codes
	closeNormalClosure       = 1000
	closeGoingAway           = 1001
	closeUnknownError        = 4000
	closeUnknownOpcode       = 4001
	closeDecodeError         = 4002
	closeNotAuthenticated    = 4003
	closeAuthenticationFailed = 4004
	closeAlreadyAuthenticated = 4005
	closeInvalidSeq          = 4007
	closeRateLimited         = 4008
	closeSessionTimedOut     = 4009
	closeInvalidShard        = 4010
	closeShardingRequired    = 4011
	closeInvalidAPIVersion   = 4012
	closeInvalidIntents      = 4013
	closeDisallowedIntents   = 4014
)

// GatewayConnection represents a connection to Discord Gateway
type GatewayConnection struct {
	userID       int64
	accessToken  string
	db           *database.DB
	logger       *zap.Logger

	// WebSocket connection
	conn   *websocket.Conn
	connMu sync.RWMutex

	// Session info
	sessionID     string
	sequenceNum   int64
	sequenceMu    sync.RWMutex

	// Heartbeat
	heartbeatInterval time.Duration
	lastHeartbeatAt   time.Time
	heartbeatMu       sync.RWMutex

	// Control
	closeChan chan struct{}
	closeOnce sync.Once

	// Status
	connected bool
	connectedMu sync.RWMutex
}

// GatewayPayload represents a Discord Gateway message
type GatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s"`
	T  *string         `json:"t"`
}

// HelloPayload represents the HELLO event data
type HelloPayload struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// ReadyPayload represents the READY event data
type ReadyPayload struct {
	SessionID string `json:"session_id"`
	User      struct {
		ID string `json:"id"`
	} `json:"user"`
}

// NewGatewayConnection creates a new Gateway connection
func NewGatewayConnection(userID int64, accessToken string, db *database.DB, logger *zap.Logger) (*GatewayConnection, error) {
	return &GatewayConnection{
		userID:      userID,
		accessToken: accessToken,
		db:          db,
		logger:      logger,
		closeChan:   make(chan struct{}),
	}, nil
}

// Connect establishes and maintains a Gateway connection
func (gc *GatewayConnection) Connect(ctx context.Context, manager *Manager) error {
	gc.logger.Info("connecting to Discord Gateway",
		zap.Int64("user_id", gc.userID),
		zap.String("gateway_url", gatewayURL),
	)

	// Establish WebSocket connection
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(gatewayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial Gateway: %w", err)
	}

	gc.connMu.Lock()
	gc.conn = conn
	gc.connMu.Unlock()

	gc.setConnected(true)
	defer gc.setConnected(false)

	// Start receiving messages
	go gc.receiveLoop(ctx, manager)

	// Wait for close signal or context cancellation
	select {
	case <-gc.closeChan:
		gc.logger.Info("Gateway connection closed by signal")
	case <-ctx.Done():
		gc.logger.Info("Gateway connection closed by context")
	}

	return nil
}

// receiveLoop processes incoming Gateway messages
func (gc *GatewayConnection) receiveLoop(ctx context.Context, manager *Manager) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-gc.closeChan:
			return
		default:
		}

		gc.connMu.RLock()
		conn := gc.conn
		gc.connMu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			gc.logger.Error("failed to read Gateway message", zap.Error(err))
			gc.Close()
			return
		}

		// Parse payload
		var payload GatewayPayload
		if err := json.Unmarshal(message, &payload); err != nil {
			gc.logger.Error("failed to unmarshal Gateway payload", zap.Error(err))
			continue
		}

		// Update sequence number
		if payload.S != nil {
			gc.setSequence(*payload.S)
		}

		// Handle payload by opcode
		if err := gc.handlePayload(ctx, manager, &payload); err != nil {
			gc.logger.Error("failed to handle Gateway payload",
				zap.Int("opcode", payload.Op),
				zap.Error(err),
			)
		}
	}
}

// handlePayload processes a Gateway payload based on opcode
func (gc *GatewayConnection) handlePayload(ctx context.Context, manager *Manager, payload *GatewayPayload) error {
	switch payload.Op {
	case opHello:
		return gc.handleHello(ctx, payload)

	case opHeartbeatACK:
		gc.logger.Debug("received heartbeat ACK")
		gc.updateHeartbeat()
		return nil

	case opDispatch:
		if payload.T == nil {
			return fmt.Errorf("dispatch event missing event type")
		}
		return gc.handleDispatchEvent(ctx, manager, *payload.T, payload.D)

	case opReconnect:
		gc.logger.Warn("received reconnect request from Gateway")
		// TODO: Implement reconnection logic
		return nil

	case opInvalidSession:
		gc.logger.Warn("received invalid session from Gateway")
		// TODO: Implement session invalidation handling
		return nil

	default:
		gc.logger.Debug("received unknown opcode",
			zap.Int("opcode", payload.Op),
		)
		return nil
	}
}

// handleHello processes the HELLO event and starts heartbeat
func (gc *GatewayConnection) handleHello(ctx context.Context, payload *GatewayPayload) error {
	var hello HelloPayload
	if err := json.Unmarshal(payload.D, &hello); err != nil {
		return fmt.Errorf("failed to unmarshal HELLO payload: %w", err)
	}

	gc.heartbeatInterval = time.Duration(hello.HeartbeatInterval) * time.Millisecond
	gc.logger.Info("received HELLO from Gateway",
		zap.Duration("heartbeat_interval", gc.heartbeatInterval),
	)

	// Start heartbeat
	go gc.heartbeatLoop(ctx)

	// Send IDENTIFY
	return gc.sendIdentify()
}

// sendIdentify sends an IDENTIFY payload to begin the session
func (gc *GatewayConnection) sendIdentify() error {
	identify := map[string]interface{}{
		"op": opIdentify,
		"d": map[string]interface{}{
			"token": gc.accessToken,
			"properties": map[string]string{
				"$os":      "linux",
				"$browser": "discord-lite-server",
				"$device":  "discord-lite-server",
			},
			"intents": 1 << 9, // GUILD_MESSAGES intent
		},
	}

	gc.logger.Debug("sending IDENTIFY to Gateway")
	return gc.sendJSON(identify)
}

// handleDispatchEvent processes a dispatch event
func (gc *GatewayConnection) handleDispatchEvent(ctx context.Context, manager *Manager, eventType string, data json.RawMessage) error {
	gc.logger.Debug("received dispatch event",
		zap.String("event_type", eventType),
	)

	switch eventType {
	case "READY":
		return gc.handleReady(ctx, data)

	case "MESSAGE_CREATE":
		return HandleMessageCreate(ctx, manager, gc.db, gc.logger, data)

	case "MESSAGE_UPDATE":
		return HandleMessageUpdate(ctx, manager, gc.db, gc.logger, data)

	case "MESSAGE_DELETE":
		return HandleMessageDelete(ctx, manager, gc.db, gc.logger, data)

	default:
		// Ignore other events
		return nil
	}
}

// handleReady processes the READY event
func (gc *GatewayConnection) handleReady(ctx context.Context, data json.RawMessage) error {
	var ready ReadyPayload
	if err := json.Unmarshal(data, &ready); err != nil {
		return fmt.Errorf("failed to unmarshal READY payload: %w", err)
	}

	gc.sessionID = ready.SessionID
	gc.logger.Info("Gateway session ready",
		zap.String("session_id", gc.sessionID),
	)

	// Store session in database
	session := &models.WebSocketSession{
		SessionID:    gc.sessionID,
		UserID:       gc.userID,
		GatewayURL:   gatewayURL,
		Status:       models.WebSocketStatusConnected,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}

	if err := gc.db.CreateWebSocketSession(ctx, session); err != nil {
		gc.logger.Error("failed to store WebSocket session", zap.Error(err))
	}

	return nil
}

// heartbeatLoop sends periodic heartbeat messages
func (gc *GatewayConnection) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(gc.heartbeatInterval)
	defer ticker.Stop()

	gc.logger.Info("started heartbeat loop",
		zap.Duration("interval", gc.heartbeatInterval),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-gc.closeChan:
			return
		case <-ticker.C:
			if err := gc.sendHeartbeat(); err != nil {
				gc.logger.Error("failed to send heartbeat", zap.Error(err))
				gc.Close()
				return
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the Gateway
func (gc *GatewayConnection) sendHeartbeat() error {
	gc.sequenceMu.RLock()
	seq := gc.sequenceNum
	gc.sequenceMu.RUnlock()

	heartbeat := map[string]interface{}{
		"op": opHeartbeat,
		"d":  seq,
	}

	gc.logger.Debug("sending heartbeat",
		zap.Int64("sequence", seq),
	)

	gc.updateHeartbeat()
	return gc.sendJSON(heartbeat)
}

// sendJSON sends a JSON payload to the Gateway
func (gc *GatewayConnection) sendJSON(v interface{}) error {
	gc.connMu.RLock()
	defer gc.connMu.RUnlock()

	if gc.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	return gc.conn.WriteJSON(v)
}

// Close closes the Gateway connection
func (gc *GatewayConnection) Close() {
	gc.closeOnce.Do(func() {
		close(gc.closeChan)

		gc.connMu.Lock()
		if gc.conn != nil {
			gc.conn.Close()
			gc.conn = nil
		}
		gc.connMu.Unlock()

		gc.setConnected(false)

		gc.logger.Info("Gateway connection closed",
			zap.Int64("user_id", gc.userID),
		)
	})
}

// Helper methods

func (gc *GatewayConnection) setSequence(seq int64) {
	gc.sequenceMu.Lock()
	gc.sequenceNum = seq
	gc.sequenceMu.Unlock()
}

func (gc *GatewayConnection) updateHeartbeat() {
	gc.heartbeatMu.Lock()
	gc.lastHeartbeatAt = time.Now()
	gc.heartbeatMu.Unlock()
}

func (gc *GatewayConnection) setConnected(connected bool) {
	gc.connectedMu.Lock()
	gc.connected = connected
	gc.connectedMu.Unlock()
}

// IsStale checks if the connection is stale (no heartbeat for duration)
func (gc *GatewayConnection) IsStale(staleDuration time.Duration) bool {
	gc.heartbeatMu.RLock()
	lastHeartbeat := gc.lastHeartbeatAt
	gc.heartbeatMu.RUnlock()

	return time.Since(lastHeartbeat) > staleDuration
}

// IsConnected returns whether the connection is active
func (gc *GatewayConnection) IsConnected() bool {
	gc.connectedMu.RLock()
	defer gc.connectedMu.RUnlock()
	return gc.connected
}
