package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Server wraps the HTTP server
type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
}

// NewServer creates a new HTTP server
func NewServer(handlers *Handlers, port string, logger *zap.Logger) *Server {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", handlers.HealthHandler)
	mux.HandleFunc("/auth/callback", handlers.CallbackHandler)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      loggingMiddleware(mux, logger),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("HTTP server configured", zap.String("port", port))

	return &Server{
		httpServer: httpServer,
		logger:     logger,
	}
}

// Serve starts the HTTP server
func (s *Server) Serve() error {
	s.logger.Info("starting HTTP server", zap.String("address", s.httpServer.Addr))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to serve HTTP: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	return nil
}

// loggingMiddleware logs all HTTP requests
func loggingMiddleware(next http.Handler, logger *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		logger.Debug("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
		)

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start)

		logger.Info("HTTP request completed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrappedWriter.statusCode),
			zap.Duration("duration", duration),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
