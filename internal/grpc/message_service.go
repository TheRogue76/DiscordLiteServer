package grpc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagev1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/message/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// MessageServer implements the MessageService gRPC server
type MessageServer struct {
	messagev1.UnimplementedMessageServiceServer
	db            *database.DB
	discordClient *auth.DiscordClient
	logger        *zap.Logger
	cacheManager  *CacheManager
	wsManager     interface{} // Using interface{} to avoid import cycle with websocket package
}

// NewMessageServer creates a new message service server
func NewMessageServer(db *database.DB, discordClient *auth.DiscordClient, logger *zap.Logger, cacheManager *CacheManager, wsManager interface{}) *MessageServer {
	return &MessageServer{
		db:            db,
		discordClient: discordClient,
		logger:        logger,
		cacheManager:  cacheManager,
		wsManager:     wsManager,
	}
}

// GetMessages returns messages from a channel with pagination support
func (s *MessageServer) GetMessages(ctx context.Context, req *messagev1.GetMessagesRequest) (*messagev1.GetMessagesResponse, error) {
	s.logger.Debug("GetMessages called",
		zap.String("session_id", req.SessionId),
		zap.String("channel_id", req.ChannelId),
		zap.Int32("limit", req.Limit),
	)

	// 1. Validate session and get user
	session, err := s.db.GetAuthSession(ctx, req.SessionId)
	if err != nil {
		s.logger.Error("failed to get auth session", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "invalid session")
	}

	if session.AuthStatus != "authenticated" {
		return nil, status.Errorf(codes.Unauthenticated, "session not authenticated")
	}

	if !session.UserID.Valid {
		return nil, status.Errorf(codes.Internal, "session has no user")
	}

	userID := session.UserID.Int64

	// 2. Verify user has access to this channel
	hasAccess, err := s.db.UserHasChannelAccess(ctx, userID, req.ChannelId)
	if err != nil {
		s.logger.Error("failed to check channel access", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to verify channel access")
	}

	if !hasAccess {
		return nil, status.Errorf(codes.PermissionDenied, "you don't have access to this channel")
	}

	// 3. Get channel internal ID
	channel, err := s.db.GetChannelByDiscordID(ctx, req.ChannelId)
	if err != nil {
		s.logger.Error("failed to get channel", zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "channel not found")
	}

	// 4. Check cache (only if no pagination and no force refresh)
	fromCache := false
	if !req.ForceRefresh && req.Before == "" && req.After == "" {
		cacheValid, err := s.cacheManager.CheckMessageCache(ctx, req.ChannelId, userID)
		if err == nil && cacheValid {
			// Serve from cache
			messages, err := s.db.GetMessagesByChannelID(ctx, channel.ID, int(req.Limit), "", "")
			if err == nil && len(messages) > 0 {
				protoMessages, err := s.convertMessagesToProto(ctx, messages)
				if err != nil {
					s.logger.Error("failed to convert messages to proto", zap.Error(err))
				} else {
					return &messagev1.GetMessagesResponse{
						Messages:  protoMessages,
						FromCache: true,
						HasMore:   len(messages) == int(req.Limit),
					}, nil
				}
			}
		}
	}

	// 5. Get OAuth token and refresh if needed
	oauthToken, err := s.db.GetOAuthToken(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get OAuth token", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to get OAuth token")
	}

	accessToken, wasRefreshed, err := s.discordClient.RefreshIfNeeded(ctx, oauthToken)
	if err != nil {
		s.logger.Error("failed to refresh token", zap.Error(err))
		return nil, status.Errorf(codes.Unauthenticated, "failed to refresh OAuth token")
	}

	if wasRefreshed {
		if err := s.db.StoreOAuthToken(ctx, oauthToken); err != nil {
			s.logger.Error("failed to update refreshed token", zap.Error(err))
		}
	}

	// 6. Fetch messages from Discord API
	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	discordMessages, err := s.discordClient.GetChannelMessages(ctx, accessToken, req.ChannelId, limit, req.Before, req.After)
	if err != nil {
		s.logger.Error("failed to fetch messages from Discord", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to fetch messages from Discord API")
	}

	// 7. Store messages in database
	var storedMessages []*models.Message
	for _, dm := range discordMessages {
		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, dm.Timestamp)
		if err != nil {
			s.logger.Warn("failed to parse message timestamp", zap.Error(err))
			timestamp = time.Now()
		}

		// Parse edited timestamp if present
		var editedTimestamp sql.NullTime
		if dm.EditedTimestamp != nil {
			editedTime, err := time.Parse(time.RFC3339, *dm.EditedTimestamp)
			if err == nil {
				editedTimestamp = sql.NullTime{Time: editedTime, Valid: true}
			}
		}

		// Get referenced message ID if present
		var referencedMessageID sql.NullString
		if dm.MessageReference != nil {
			referencedMessageID = sql.NullString{String: dm.MessageReference.MessageID, Valid: true}
		}

		message := &models.Message{
			DiscordMessageID:    dm.ID,
			ChannelID:           channel.ID,
			AuthorID:            dm.Author.ID,
			AuthorUsername:      dm.Author.Username,
			AuthorAvatar:        sql.NullString{String: dm.Author.Avatar, Valid: dm.Author.Avatar != ""},
			Content:             sql.NullString{String: dm.Content, Valid: dm.Content != ""},
			Timestamp:           timestamp,
			EditedTimestamp:     editedTimestamp,
			MessageType:         models.MessageType(dm.Type),
			ReferencedMessageID: referencedMessageID,
		}

		if err := s.db.CreateOrUpdateMessage(ctx, message); err != nil {
			s.logger.Error("failed to store message", zap.Error(err), zap.String("message_id", dm.ID))
			continue
		}

		// Store attachments
		for _, att := range dm.Attachments {
			attachment := &models.MessageAttachment{
				MessageID:    message.ID,
				AttachmentID: att.ID,
				Filename:     att.Filename,
				URL:          att.URL,
				ProxyURL:     sql.NullString{String: att.ProxyURL, Valid: att.ProxyURL != ""},
				SizeBytes:    att.Size,
				Width:        sql.NullInt64{Int64: int64(*att.Width), Valid: att.Width != nil},
				Height:       sql.NullInt64{Int64: int64(*att.Height), Valid: att.Height != nil},
				ContentType:  sql.NullString{String: att.ContentType, Valid: att.ContentType != ""},
			}

			if err := s.db.CreateMessageAttachment(ctx, attachment); err != nil {
				s.logger.Error("failed to store attachment", zap.Error(err))
			}
		}

		storedMessages = append(storedMessages, message)
	}

	// 8. Update cache metadata (only for non-paginated requests)
	if req.Before == "" && req.After == "" {
		if err := s.cacheManager.SetMessageCache(ctx, req.ChannelId, userID); err != nil {
			s.logger.Warn("failed to set message cache", zap.Error(err))
		}
	}

	// 9. Convert to proto
	protoMessages, err := s.convertMessagesToProto(ctx, storedMessages)
	if err != nil {
		s.logger.Error("failed to convert messages to proto", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to convert messages")
	}

	s.logger.Info("fetched messages",
		zap.String("channel_id", req.ChannelId),
		zap.Int("message_count", len(storedMessages)),
		zap.Bool("from_cache", fromCache),
	)

	return &messagev1.GetMessagesResponse{
		Messages:  protoMessages,
		FromCache: fromCache,
		HasMore:   len(discordMessages) == limit,
	}, nil
}

// StreamMessages streams real-time message events for subscribed channels
// This is a server-side streaming RPC that will be fully implemented in Phase 2E
func (s *MessageServer) StreamMessages(req *messagev1.StreamMessagesRequest, stream messagev1.MessageService_StreamMessagesServer) error {
	s.logger.Debug("StreamMessages called",
		zap.String("session_id", req.SessionId),
		zap.Strings("channel_ids", req.ChannelIds),
	)

	// TODO: Phase 2E - Implement WebSocket integration
	// This is a placeholder that returns an error indicating it's not yet implemented

	return status.Errorf(codes.Unimplemented, "StreamMessages will be implemented in Phase 2E with WebSocket support")

	/* Phase 2E implementation will look like:
	1. Validate session
	2. Verify user has access to all requested channels
	3. Subscribe to WebSocket manager for these channels
	4. Stream MessageEvent as they arrive
	5. Handle context cancellation for cleanup

	ctx := stream.Context()
	userID := ... // from session

	// Subscribe to channels
	eventChan, err := s.wsManager.Subscribe(ctx, userID, req.ChannelIds)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to subscribe to channels")
	}
	defer s.wsManager.Unsubscribe(userID, req.ChannelIds)

	// Stream events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-eventChan:
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
	*/
}

// Helper functions

func (s *MessageServer) convertMessagesToProto(ctx context.Context, messages []*models.Message) ([]*messagev1.Message, error) {
	result := make([]*messagev1.Message, 0, len(messages))

	for _, m := range messages {
		// Get attachments
		attachments, err := s.db.GetMessageAttachmentsByMessageID(ctx, m.ID)
		if err != nil {
			s.logger.Warn("failed to get attachments", zap.Error(err))
			attachments = []*models.MessageAttachment{}
		}

		protoAttachments := make([]*messagev1.MessageAttachment, 0, len(attachments))
		for _, att := range attachments {
			protoAtt := &messagev1.MessageAttachment{
				AttachmentId: att.AttachmentID,
				Filename:     att.Filename,
				Url:          att.URL,
				ProxyUrl:     att.ProxyURL.String,
				SizeBytes:    int32(att.SizeBytes),
				ContentType:  att.ContentType.String,
			}

			if att.Width.Valid {
				width := int32(att.Width.Int64)
				protoAtt.Width = &width
			}
			if att.Height.Valid {
				height := int32(att.Height.Int64)
				protoAtt.Height = &height
			}

			protoAttachments = append(protoAttachments, protoAtt)
		}

		protoMsg := &messagev1.Message{
			DiscordMessageId: m.DiscordMessageID,
			ChannelId:        fmt.Sprintf("%d", m.ChannelID), // Should be Discord channel ID
			Author: &messagev1.MessageAuthor{
				DiscordId:     m.AuthorID,
				Username:      m.AuthorUsername,
				Discriminator: "", // We don't store discriminator currently
				Avatar:        m.AuthorAvatar.String,
			},
			Content:     m.Content.String,
			Timestamp:   m.Timestamp.UnixMilli(),
			Type:        messagev1.MessageType(m.MessageType),
			Attachments: protoAttachments,
		}

		if m.EditedTimestamp.Valid {
			editedMs := m.EditedTimestamp.Time.UnixMilli()
			protoMsg.EditedTimestamp = &editedMs
		}

		if m.ReferencedMessageID.Valid {
			protoMsg.ReferencedMessageId = &m.ReferencedMessageID.String
		}

		result = append(result, protoMsg)
	}

	return result, nil
}
