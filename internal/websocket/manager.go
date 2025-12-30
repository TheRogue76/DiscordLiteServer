package websocket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	messagev1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/message/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
)

// Manager manages Discord Gateway WebSocket connections and message streaming
type Manager struct {
	db            *database.DB
	discordClient *auth.DiscordClient
	logger        *zap.Logger

	// Map of userID -> GatewayConnection
	connections sync.Map

	// Map of channelID -> set of userIDs subscribed to that channel
	subscriptions sync.Map

	// Map of userID -> map of channelID -> event channel
	eventChannels sync.Map

	// Configuration
	maxConnectionsPerUser int
	enabled               bool
}

// SubscriptionSet represents a set of user IDs subscribed to a channel
type SubscriptionSet struct {
	users map[int64]bool
	mu    sync.RWMutex
}

// NewManager creates a new WebSocket manager
func NewManager(db *database.DB, discordClient *auth.DiscordClient, logger *zap.Logger, maxConnectionsPerUser int, enabled bool) *Manager {
	return &Manager{
		db:                    db,
		discordClient:         discordClient,
		logger:                logger,
		maxConnectionsPerUser: maxConnectionsPerUser,
		enabled:               enabled,
	}
}

// IsEnabled returns whether WebSocket support is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// Subscribe subscribes a user to message events for specific channels
// Returns a channel that will receive MessageEvent notifications
func (m *Manager) Subscribe(ctx context.Context, userID int64, channelIDs []string) (<-chan *messagev1.MessageEvent, error) {
	if !m.enabled {
		return nil, fmt.Errorf("WebSocket support is not enabled")
	}

	m.logger.Info("subscribing user to channels",
		zap.Int64("user_id", userID),
		zap.Strings("channel_ids", channelIDs),
	)

	// Create event channel for this subscription
	eventChan := make(chan *messagev1.MessageEvent, 100) // Buffer 100 events

	// Store event channel
	userChannels, _ := m.eventChannels.LoadOrStore(userID, &sync.Map{})
	userChannelMap := userChannels.(*sync.Map)

	for _, channelID := range channelIDs {
		userChannelMap.Store(channelID, eventChan)

		// Add user to channel subscription set
		subsInterface, _ := m.subscriptions.LoadOrStore(channelID, &SubscriptionSet{
			users: make(map[int64]bool),
		})
		subs := subsInterface.(*SubscriptionSet)

		subs.mu.Lock()
		subs.users[userID] = true
		subs.mu.Unlock()

		m.logger.Debug("user subscribed to channel",
			zap.Int64("user_id", userID),
			zap.String("channel_id", channelID),
		)
	}

	// Ensure Gateway connection exists for this user
	if err := m.ensureConnection(ctx, userID); err != nil {
		m.logger.Error("failed to ensure Gateway connection",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to establish Gateway connection: %w", err)
	}

	return eventChan, nil
}

// Unsubscribe removes a user's subscription to specific channels
func (m *Manager) Unsubscribe(userID int64, channelIDs []string) {
	m.logger.Info("unsubscribing user from channels",
		zap.Int64("user_id", userID),
		zap.Strings("channel_ids", channelIDs),
	)

	// Remove from channel subscriptions
	for _, channelID := range channelIDs {
		subsInterface, ok := m.subscriptions.Load(channelID)
		if !ok {
			continue
		}

		subs := subsInterface.(*SubscriptionSet)
		subs.mu.Lock()
		delete(subs.users, userID)
		isEmpty := len(subs.users) == 0
		subs.mu.Unlock()

		// Remove subscription set if empty
		if isEmpty {
			m.subscriptions.Delete(channelID)
		}

		m.logger.Debug("user unsubscribed from channel",
			zap.Int64("user_id", userID),
			zap.String("channel_id", channelID),
		)
	}

	// Remove event channels
	userChannelsInterface, ok := m.eventChannels.Load(userID)
	if ok {
		userChannels := userChannelsInterface.(*sync.Map)
		for _, channelID := range channelIDs {
			if eventChanInterface, ok := userChannels.Load(channelID); ok {
				eventChan := eventChanInterface.(chan *messagev1.MessageEvent)
				close(eventChan)
				userChannels.Delete(channelID)
			}
		}
	}
}

// BroadcastEvent broadcasts a message event to all users subscribed to the channel
func (m *Manager) BroadcastEvent(channelID string, event *messagev1.MessageEvent) {
	subsInterface, ok := m.subscriptions.Load(channelID)
	if !ok {
		return
	}

	subs := subsInterface.(*SubscriptionSet)
	subs.mu.RLock()
	defer subs.mu.RUnlock()

	m.logger.Debug("broadcasting event to subscribers",
		zap.String("channel_id", channelID),
		zap.Int("subscriber_count", len(subs.users)),
		zap.String("event_type", event.EventType.String()),
	)

	for userID := range subs.users {
		userChannelsInterface, ok := m.eventChannels.Load(userID)
		if !ok {
			continue
		}

		userChannels := userChannelsInterface.(*sync.Map)
		eventChanInterface, ok := userChannels.Load(channelID)
		if !ok {
			continue
		}

		eventChan := eventChanInterface.(chan *messagev1.MessageEvent)

		// Non-blocking send to avoid blocking on slow consumers
		select {
		case eventChan <- event:
			m.logger.Debug("event sent to user",
				zap.Int64("user_id", userID),
				zap.String("channel_id", channelID),
			)
		default:
			m.logger.Warn("event channel full, dropping event",
				zap.Int64("user_id", userID),
				zap.String("channel_id", channelID),
			)
		}
	}
}

// ensureConnection ensures a Gateway connection exists for the user
func (m *Manager) ensureConnection(ctx context.Context, userID int64) error {
	// Check if connection already exists
	if _, ok := m.connections.Load(userID); ok {
		return nil
	}

	// Get OAuth token
	oauthToken, err := m.db.GetOAuthToken(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get OAuth token: %w", err)
	}

	// Refresh token if needed
	accessToken, wasRefreshed, err := m.discordClient.RefreshIfNeeded(ctx, oauthToken)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	if wasRefreshed {
		if err := m.db.StoreOAuthToken(ctx, oauthToken); err != nil {
			m.logger.Error("failed to store refreshed token", zap.Error(err))
		}
	}

	// Create Gateway connection
	conn, err := NewGatewayConnection(userID, accessToken, m.db, m.logger)
	if err != nil {
		return fmt.Errorf("failed to create Gateway connection: %w", err)
	}

	// Store connection
	m.connections.Store(userID, conn)

	// Start connection in background
	go func() {
		if err := conn.Connect(ctx, m); err != nil {
			m.logger.Error("Gateway connection failed",
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
			m.connections.Delete(userID)
		}
	}()

	m.logger.Info("Gateway connection established",
		zap.Int64("user_id", userID),
	)

	return nil
}

// DisconnectUser disconnects a user's Gateway connection
func (m *Manager) DisconnectUser(userID int64) error {
	connInterface, ok := m.connections.Load(userID)
	if !ok {
		return fmt.Errorf("no connection found for user")
	}

	conn := connInterface.(*GatewayConnection)
	conn.Close()
	m.connections.Delete(userID)

	m.logger.Info("Gateway connection closed",
		zap.Int64("user_id", userID),
	)

	return nil
}

// GetConnectionStats returns statistics about active connections
func (m *Manager) GetConnectionStats() map[string]int {
	stats := make(map[string]int)

	connectionCount := 0
	m.connections.Range(func(_, _ interface{}) bool {
		connectionCount++
		return true
	})
	stats["active_connections"] = connectionCount

	subscriptionCount := 0
	m.subscriptions.Range(func(_, _ interface{}) bool {
		subscriptionCount++
		return true
	})
	stats["active_subscriptions"] = subscriptionCount

	return stats
}

// Shutdown gracefully shuts down all Gateway connections
func (m *Manager) Shutdown(_ context.Context) error {
	m.logger.Info("shutting down WebSocket manager")

	// Close all connections
	m.connections.Range(func(key, value interface{}) bool {
		userID := key.(int64)
		conn := value.(*GatewayConnection)
		conn.Close()
		m.logger.Debug("closed Gateway connection", zap.Int64("user_id", userID))
		return true
	})

	// Close all event channels
	m.eventChannels.Range(func(_, value interface{}) bool {
		userChannels := value.(*sync.Map)
		userChannels.Range(func(_, chValue interface{}) bool {
			eventChan := chValue.(chan *messagev1.MessageEvent)
			close(eventChan)
			return true
		})
		return true
	})

	m.logger.Info("WebSocket manager shut down successfully")
	return nil
}

// CleanupStaleConnections removes connections that haven't sent a heartbeat recently
func (m *Manager) CleanupStaleConnections(staleDuration time.Duration) {
	m.logger.Debug("cleaning up stale connections")

	staleCount := 0
	m.connections.Range(func(key, value interface{}) bool {
		userID := key.(int64)
		conn := value.(*GatewayConnection)

		if conn.IsStale(staleDuration) {
			m.logger.Warn("removing stale connection",
				zap.Int64("user_id", userID),
			)
			conn.Close()
			m.connections.Delete(userID)
			staleCount++
		}
		return true
	})

	if staleCount > 0 {
		m.logger.Info("cleaned up stale connections",
			zap.Int("count", staleCount),
		)
	}
}

// StartCleanupJob starts a background job to cleanup stale connections
func (m *Manager) StartCleanupJob(ctx context.Context, interval, staleDuration time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.logger.Info("started WebSocket cleanup job",
		zap.Duration("interval", interval),
		zap.Duration("stale_duration", staleDuration),
	)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("stopping WebSocket cleanup job")
			return
		case <-ticker.C:
			m.CleanupStaleConnections(staleDuration)
		}
	}
}
