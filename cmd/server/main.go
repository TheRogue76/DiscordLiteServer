// Package main is the entry point for the Discord Lite Server application.
// It initializes and starts both gRPC and HTTP servers for Discord OAuth authentication.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/config"
	"github.com/parsascontentcorner/discordliteserver/internal/database"
	grpcserver "github.com/parsascontentcorner/discordliteserver/internal/grpc"
	httpserver "github.com/parsascontentcorner/discordliteserver/internal/oauth"
	"github.com/parsascontentcorner/discordliteserver/internal/ratelimit"
	"github.com/parsascontentcorner/discordliteserver/internal/websocket"
	"github.com/parsascontentcorner/discordliteserver/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer func() {
		// Sync errors on stdout/stderr are expected and can be safely ignored
		// for non-syncable file descriptors (pipes, terminals, etc.)
		_ = log.Sync()
	}()

	log.Info("starting Discord Lite Server",
		zap.String("environment", cfg.Server.Env),
		zap.String("http_port", cfg.Server.HTTPPort),
		zap.String("grpc_port", cfg.Server.GRPCPort),
	)

	// Initialize database connection
	db, err := database.NewDB(&cfg.Database, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("failed to close database connection", zap.Error(err))
		}
	}()

	// Run database migrations
	if err := runMigrations(db, log); err != nil {
		log.Fatal("failed to run migrations", zap.Error(err))
	}

	// Start cleanup job for expired sessions
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db.StartCleanupJob(ctx, 30*time.Minute)

	// Initialize auth components
	discordClient := auth.NewDiscordClient(cfg, log)
	stateManager := auth.NewStateManager(db, cfg.Security.StateExpiryMinutes)
	oauthHandler := auth.NewOAuthHandler(db, discordClient, stateManager, log)

	// Initialize rate limiter
	rateLimiter := ratelimit.NewRateLimiter(log)
	discordClient.SetRateLimiter(rateLimiter)

	// Initialize cache manager
	cacheManager := grpcserver.NewCacheManager(db, log)

	// Initialize WebSocket manager
	wsManager := websocket.NewManager(db, discordClient, log, cfg.WebSocket.MaxConnectionsPerUser, cfg.WebSocket.Enabled)

	// Start WebSocket cleanup job (runs every 30 minutes)
	if cfg.WebSocket.Enabled {
		go wsManager.StartCleanupJob(ctx, 30*time.Minute, 1*time.Hour)
	}

	// Start cache cleanup job (runs every 1 hour)
	go db.StartCacheCleanupJob(ctx, 1*time.Hour)

	// Initialize gRPC services
	authService := grpcserver.NewAuthServer(db, discordClient, stateManager, log, cfg.Security.SessionExpiryHours)
	channelService := grpcserver.NewChannelServer(db, discordClient, log, cacheManager)
	messageService := grpcserver.NewMessageServer(db, discordClient, log, cacheManager, wsManager)

	// Initialize gRPC server with all services
	grpcServer, err := grpcserver.NewServer(authService, channelService, messageService, cfg.Server.GRPCPort, log)
	if err != nil {
		log.Fatal("failed to create gRPC server", zap.Error(err))
	}

	// Initialize HTTP server
	httpHandlers := httpserver.NewHandlers(oauthHandler, log)
	httpServer := httpserver.NewServer(httpHandlers, cfg.Server.HTTPPort, log)

	// Start servers in goroutines
	grpcErrChan := make(chan error, 1)
	httpErrChan := make(chan error, 1)

	// Start gRPC server
	go func() {
		log.Info("starting gRPC server", zap.String("port", cfg.Server.GRPCPort))
		if err := grpcServer.Serve(); err != nil {
			grpcErrChan <- err
		}
	}()

	// Start HTTP server
	go func() {
		log.Info("starting HTTP server", zap.String("port", cfg.Server.HTTPPort))
		if err := httpServer.Serve(); err != nil {
			httpErrChan <- err
		}
	}()

	// Wait for shutdown signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-grpcErrChan:
		log.Fatal("gRPC server error", zap.Error(err))
	case err := <-httpErrChan:
		log.Fatal("HTTP server error", zap.Error(err))
	case sig := <-sigChan:
		log.Info("received shutdown signal", zap.String("signal", sig.String()))
	}

	// Graceful shutdown
	log.Info("shutting down servers...")

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown HTTP server gracefully", zap.Error(err))
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown WebSocket manager
	if cfg.WebSocket.Enabled {
		if err := wsManager.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to shutdown WebSocket manager gracefully", zap.Error(err))
		}
	}

	log.Info("servers shut down successfully")
}

// runMigrations runs database migrations using golang-migrate library
func runMigrations(db *database.DB, log *zap.Logger) error {
	log.Info("running database migrations")

	// Path to migrations directory (relative to binary execution location)
	migrationsPath := "internal/database/migrations"

	// Run migrations using the migrate library
	if err := db.RunMigrations(migrationsPath); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Info("database migrations completed successfully")
	return nil
}
