package grpc

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	authv1 "github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/auth/v1"
)

// Server wraps the gRPC server
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *zap.Logger
	port       string
}

// NewServer creates a new gRPC server
func NewServer(authService *AuthServer, port string, logger *zap.Logger) (*Server, error) {
	// Create listener - net.Listen is standard for gRPC server setup
	lis, err := net.Listen("tcp", ":"+port) //nolint:noctx // Server initialization doesn't require context
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	// Create gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(logger)),
	)

	// Register auth service
	authv1.RegisterAuthServiceServer(grpcServer, authService)

	// Register reflection service for development (allows tools like grpcurl)
	reflection.Register(grpcServer)

	logger.Info("gRPC server configured", zap.String("port", port))

	return &Server{
		grpcServer: grpcServer,
		listener:   lis,
		logger:     logger,
		port:       port,
	}, nil
}

// Serve starts the gRPC server
func (s *Server) Serve() error {
	s.logger.Info("starting gRPC server", zap.String("address", s.listener.Addr().String()))

	if err := s.grpcServer.Serve(s.listener); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}

	return nil
}

// GracefulStop gracefully stops the gRPC server
func (s *Server) GracefulStop() {
	s.logger.Info("gracefully stopping gRPC server")
	s.grpcServer.GracefulStop()
}

// Stop immediately stops the gRPC server
func (s *Server) Stop() {
	s.logger.Info("stopping gRPC server")
	s.grpcServer.Stop()
}

// loggingInterceptor logs all gRPC requests
func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Debug("gRPC request",
			zap.String("method", info.FullMethod),
		)

		resp, err := handler(ctx, req)

		if err != nil {
			logger.Error("gRPC request failed",
				zap.String("method", info.FullMethod),
				zap.Error(err),
			)
		} else {
			logger.Debug("gRPC request completed",
				zap.String("method", info.FullMethod),
			)
		}

		return resp, err
	}
}
