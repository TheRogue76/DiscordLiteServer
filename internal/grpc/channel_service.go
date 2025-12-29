package grpc

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	channelv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/channel/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// ChannelServer implements the ChannelService gRPC server
type ChannelServer struct {
	channelv1.UnimplementedChannelServiceServer
	db            *database.DB
	discordClient *auth.DiscordClient
	logger        *zap.Logger
	cacheManager  *CacheManager
}

// NewChannelServer creates a new channel service server
func NewChannelServer(db *database.DB, discordClient *auth.DiscordClient, logger *zap.Logger, cacheManager *CacheManager) *ChannelServer {
	return &ChannelServer{
		db:            db,
		discordClient: discordClient,
		logger:        logger,
		cacheManager:  cacheManager,
	}
}

// GetGuilds returns all guilds the authenticated user is a member of
func (s *ChannelServer) GetGuilds(ctx context.Context, req *channelv1.GetGuildsRequest) (*channelv1.GetGuildsResponse, error) {
	s.logger.Debug("GetGuilds called", zap.String("session_id", req.SessionId))

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

	// 2. Check cache unless force refresh
	fromCache := false
	if !req.ForceRefresh {
		cacheValid, err := s.cacheManager.CheckGuildCache(ctx, userID)
		if err == nil && cacheValid {
			// Serve from cache
			guilds, err := s.db.GetGuildsByUserID(ctx, userID)
			if err == nil && len(guilds) > 0 {
				return &channelv1.GetGuildsResponse{
					Guilds:    convertGuildsToProto(guilds),
					FromCache: true,
				}, nil
			}
		}
	}

	// 3. Get OAuth token and refresh if needed
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

	// If token was refreshed, update in database
	if wasRefreshed {
		if err := s.db.StoreOAuthToken(ctx, oauthToken); err != nil {
			s.logger.Error("failed to update refreshed token", zap.Error(err))
		}
	}

	// 4. Fetch guilds from Discord API
	discordGuilds, err := s.discordClient.GetUserGuilds(ctx, accessToken)
	if err != nil {
		s.logger.Error("failed to fetch guilds from Discord", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to fetch guilds from Discord API")
	}

	// 5. Store guilds in database
	var storedGuilds []*models.Guild
	for _, dg := range discordGuilds {
		// Parse permissions
		permissions, _ := strconv.ParseInt(dg.Permissions, 10, 64)

		guild := &models.Guild{
			DiscordGuildID: dg.ID,
			Name:           dg.Name,
			Icon:           sql.NullString{String: dg.Icon, Valid: dg.Icon != ""},
			Permissions:    permissions,
			Features:       dg.Features,
		}

		// Create or update guild
		if err := s.db.CreateOrUpdateGuild(ctx, guild); err != nil {
			s.logger.Error("failed to store guild", zap.Error(err), zap.String("guild_id", dg.ID))
			continue
		}

		// Link user to guild
		if err := s.db.CreateUserGuild(ctx, userID, guild.ID); err != nil {
			s.logger.Error("failed to link user to guild", zap.Error(err))
		}

		storedGuilds = append(storedGuilds, guild)
	}

	// 6. Update cache metadata
	if err := s.cacheManager.SetGuildCache(ctx, userID); err != nil {
		s.logger.Warn("failed to set guild cache", zap.Error(err))
	}

	s.logger.Info("fetched guilds",
		zap.Int64("user_id", userID),
		zap.Int("guild_count", len(storedGuilds)),
		zap.Bool("from_cache", fromCache),
	)

	return &channelv1.GetGuildsResponse{
		Guilds:    convertGuildsToProto(storedGuilds),
		FromCache: fromCache,
	}, nil
}

// GetChannels returns all channels in a specific guild
func (s *ChannelServer) GetChannels(ctx context.Context, req *channelv1.GetChannelsRequest) (*channelv1.GetChannelsResponse, error) {
	s.logger.Debug("GetChannels called",
		zap.String("session_id", req.SessionId),
		zap.String("guild_id", req.GuildId),
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

	// 2. Verify user has access to this guild
	hasAccess, err := s.db.UserHasGuildAccess(ctx, userID, req.GuildId)
	if err != nil {
		s.logger.Error("failed to check guild access", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to verify guild access")
	}

	if !hasAccess {
		return nil, status.Errorf(codes.PermissionDenied, "you don't have access to this guild")
	}

	// 3. Check cache unless force refresh
	fromCache := false
	if !req.ForceRefresh {
		cacheValid, err := s.cacheManager.CheckChannelCache(ctx, req.GuildId, userID)
		if err == nil && cacheValid {
			// Serve from cache
			channels, err := s.db.GetChannelsByDiscordGuildID(ctx, req.GuildId)
			if err == nil && len(channels) > 0 {
				return &channelv1.GetChannelsResponse{
					Channels:  convertChannelsToProto(channels),
					FromCache: true,
				}, nil
			}
		}
	}

	// 4. Get OAuth token and refresh if needed
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

	// 5. Fetch channels from Discord API
	discordChannels, err := s.discordClient.GetGuildChannels(ctx, accessToken, req.GuildId)
	if err != nil {
		s.logger.Error("failed to fetch channels from Discord", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to fetch channels from Discord API")
	}

	// 6. Get guild internal ID
	guild, err := s.db.GetGuildByDiscordID(ctx, req.GuildId)
	if err != nil {
		s.logger.Error("failed to get guild", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "guild not found in database")
	}

	// 7. Store channels in database
	var storedChannels []*models.Channel
	for _, dc := range discordChannels {
		channel := &models.Channel{
			DiscordChannelID: dc.ID,
			GuildID:          guild.ID,
			Name:             dc.Name,
			Type:             models.ChannelType(dc.Type),
			Position:         dc.Position,
			ParentID:         sql.NullString{String: dc.ParentID, Valid: dc.ParentID != ""},
			Topic:            sql.NullString{String: dc.Topic, Valid: dc.Topic != ""},
			NSFW:             dc.NSFW,
			LastMessageID:    sql.NullString{String: dc.LastMessageID, Valid: dc.LastMessageID != ""},
		}

		if err := s.db.CreateOrUpdateChannel(ctx, channel); err != nil {
			s.logger.Error("failed to store channel", zap.Error(err), zap.String("channel_id", dc.ID))
			continue
		}

		storedChannels = append(storedChannels, channel)
	}

	// 8. Update cache metadata
	if err := s.cacheManager.SetChannelCache(ctx, req.GuildId, userID); err != nil {
		s.logger.Warn("failed to set channel cache", zap.Error(err))
	}

	s.logger.Info("fetched channels",
		zap.String("guild_id", req.GuildId),
		zap.Int("channel_count", len(storedChannels)),
		zap.Bool("from_cache", fromCache),
	)

	return &channelv1.GetChannelsResponse{
		Channels:  convertChannelsToProto(storedChannels),
		FromCache: fromCache,
	}, nil
}

// Helper functions to convert models to proto

func convertGuildsToProto(guilds []*models.Guild) []*channelv1.Guild {
	result := make([]*channelv1.Guild, 0, len(guilds))
	for _, g := range guilds {
		result = append(result, &channelv1.Guild{
			DiscordGuildId: g.DiscordGuildID,
			Name:           g.Name,
			Icon:           g.Icon.String,
			Owner:          false, // We don't store owner info currently
			Permissions:    g.Permissions,
			Features:       g.Features,
		})
	}
	return result
}

func convertChannelsToProto(channels []*models.Channel) []*channelv1.Channel {
	result := make([]*channelv1.Channel, 0, len(channels))
	for _, c := range channels {
		// Get guild Discord ID (we need to fetch it or pass it differently)
		// For now, we'll leave it empty as we'd need to join with guilds table
		result = append(result, &channelv1.Channel{
			DiscordChannelId: c.DiscordChannelID,
			GuildId:          fmt.Sprintf("%d", c.GuildID), // This should be Discord guild ID, not internal ID
			Name:             c.Name,
			Type:             channelv1.ChannelType(c.Type),
			Position:         int32(c.Position),
			ParentId:         c.ParentID.String,
			Topic:            c.Topic.String,
			Nsfw:             c.NSFW,
			LastMessageId:    c.LastMessageID.String,
		})
	}
	return result
}
