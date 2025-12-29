// Package grpc provides gRPC server implementation for authentication services.
package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/auth/v1"
	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	"github.com/parsascontentcorner/discordliteserver/internal/models"
)

// AuthServer implements the gRPC AuthService
type AuthServer struct {
	authv1.UnimplementedAuthServiceServer
	db                 *database.DB
	discordClient      *auth.DiscordClient
	stateManager       *auth.StateManager
	logger             *zap.Logger
	sessionExpiryHours int
}

// NewAuthServer creates a new gRPC auth server
func NewAuthServer(
	db *database.DB,
	discordClient *auth.DiscordClient,
	stateManager *auth.StateManager,
	logger *zap.Logger,
	sessionExpiryHours int,
) *AuthServer {
	return &AuthServer{
		db:                 db,
		discordClient:      discordClient,
		stateManager:       stateManager,
		logger:             logger,
		sessionExpiryHours: sessionExpiryHours,
	}
}

// InitAuth initiates the OAuth flow
func (s *AuthServer) InitAuth(ctx context.Context, req *authv1.InitAuthRequest) (*authv1.InitAuthResponse, error) {
	// Generate or use provided session ID
	sessionID := req.SessionId
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	s.logger.Info("initiating auth flow", zap.String("session_id", sessionID))

	// Generate OAuth state
	state, err := s.stateManager.GenerateState()
	if err != nil {
		s.logger.Error("failed to generate state", zap.String("session_id", sessionID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to generate state")
	}

	// Store state in database
	if err := s.stateManager.StoreState(ctx, state, sessionID); err != nil {
		s.logger.Error("failed to store state", zap.String("session_id", sessionID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to store state")
	}

	// Create auth session with pending status
	expiresAt := time.Now().Add(time.Duration(s.sessionExpiryHours) * time.Hour)
	session := &models.AuthSession{
		SessionID:  sessionID,
		AuthStatus: models.AuthStatusPending,
		ExpiresAt:  expiresAt,
	}

	if err := s.db.CreateAuthSession(ctx, session); err != nil {
		s.logger.Error("failed to create auth session", zap.String("session_id", sessionID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to create session")
	}

	// Get Discord OAuth URL
	authURL := s.discordClient.GetAuthURL(state)

	s.logger.Info("auth flow initiated successfully",
		zap.String("session_id", sessionID),
		zap.String("auth_url", authURL),
	)

	return &authv1.InitAuthResponse{
		AuthUrl:   authURL,
		SessionId: sessionID,
		State:     state,
	}, nil
}

// GetAuthStatus checks the authentication status
func (s *AuthServer) GetAuthStatus(ctx context.Context, req *authv1.GetAuthStatusRequest) (*authv1.GetAuthStatusResponse, error) {
	sessionID := req.SessionId
	if sessionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "session_id is required")
	}

	s.logger.Debug("checking auth status", zap.String("session_id", sessionID))

	// Get session from database
	session, err := s.db.GetAuthSession(ctx, sessionID)
	if err != nil {
		s.logger.Error("failed to get auth session", zap.String("session_id", sessionID), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "session not found")
	}

	// Check if session has expired
	if session.IsExpired() {
		s.logger.Warn("session has expired", zap.String("session_id", sessionID))
		return &authv1.GetAuthStatusResponse{
			Status:       authv1.AuthStatus_AUTH_STATUS_FAILED,
			ErrorMessage: stringPtr("session has expired"),
		}, nil
	}

	// Build response based on status
	resp := &authv1.GetAuthStatusResponse{}

	switch session.AuthStatus {
	case models.AuthStatusPending:
		resp.Status = authv1.AuthStatus_AUTH_STATUS_PENDING

	case models.AuthStatusAuthenticated:
		resp.Status = authv1.AuthStatus_AUTH_STATUS_AUTHENTICATED

		// Get user info if authenticated
		if session.UserID.Valid {
			user, err := s.db.GetUserByID(ctx, session.UserID.Int64)
			if err != nil {
				s.logger.Error("failed to get user", zap.Int64("user_id", session.UserID.Int64), zap.Error(err))
				return nil, status.Errorf(codes.Internal, "failed to retrieve user information")
			}

			resp.User = &authv1.UserInfo{
				DiscordId:     user.DiscordID,
				Username:      user.Username,
				Discriminator: user.Discriminator.String,
				Avatar:        user.Avatar.String,
				Email:         user.Email.String,
			}
		}

	case models.AuthStatusFailed:
		resp.Status = authv1.AuthStatus_AUTH_STATUS_FAILED
		if session.ErrorMessage.Valid {
			resp.ErrorMessage = stringPtr(session.ErrorMessage.String)
		}

	default:
		resp.Status = authv1.AuthStatus_AUTH_STATUS_UNSPECIFIED
	}

	s.logger.Debug("returning auth status",
		zap.String("session_id", sessionID),
		zap.String("status", session.AuthStatus),
	)

	return resp, nil
}

// RevokeAuth revokes authentication for a session
func (s *AuthServer) RevokeAuth(ctx context.Context, req *authv1.RevokeAuthRequest) (*authv1.RevokeAuthResponse, error) {
	sessionID := req.SessionId
	if sessionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "session_id is required")
	}

	s.logger.Info("revoking auth", zap.String("session_id", sessionID))

	// Get session to find user ID
	session, err := s.db.GetAuthSession(ctx, sessionID)
	if err != nil {
		s.logger.Error("failed to get auth session", zap.String("session_id", sessionID), zap.Error(err))
		return nil, status.Errorf(codes.NotFound, "session not found")
	}

	// Delete OAuth tokens if user exists
	if session.UserID.Valid {
		if err := s.db.DeleteOAuthToken(ctx, session.UserID.Int64); err != nil {
			s.logger.Warn("failed to delete oauth token",
				zap.String("session_id", sessionID),
				zap.Int64("user_id", session.UserID.Int64),
				zap.Error(err),
			)
		}
	}

	// Delete session
	if err := s.db.DeleteAuthSession(ctx, sessionID); err != nil {
		s.logger.Error("failed to delete auth session", zap.String("session_id", sessionID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to revoke authentication")
	}

	s.logger.Info("auth revoked successfully", zap.String("session_id", sessionID))

	return &authv1.RevokeAuthResponse{
		Success: true,
		Message: "authentication revoked successfully",
	}, nil
}

// stringPtr returns a pointer to a string (helper for optional fields)
func stringPtr(s string) *string {
	return &s
}
