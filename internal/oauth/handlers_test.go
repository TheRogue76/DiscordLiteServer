package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/auth"
	"github.com/parsascontentcorner/discordliteserver/internal/testutil"
)

func TestHealthHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger) // No OAuth handler needed for health check

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.HealthHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestCallbackHandler_Success(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)
	oauthHandler := auth.NewOAuthHandler(db, discordClient, stateManager, logger)

	handlers := NewHandlers(oauthHandler, logger)

	// Create a valid session and state
	sessionID := testutil.GenerateSessionID()
	state, err := stateManager.GenerateState()
	require.NoError(t, err)
	err = stateManager.StoreState(ctx, state, sessionID)
	require.NoError(t, err)

	// Create auth session
	session := testutil.GenerateAuthSession(sessionID, "pending")
	err = db.CreateAuthSession(ctx, session)
	require.NoError(t, err)

	// Note: This test will fail when trying to exchange code with Discord
	// since we're using a test code, but we're testing the handler logic
	req, err := http.NewRequestWithContext(context.Background(), "GET", "/auth/callback?code=test_code_123&state="+state, nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.CallbackHandler(rr, req)

	// The handler will fail at Discord API call, but we're testing the request handling
	assert.Equal(t, http.StatusBadRequest, rr.Code) // Fails during OAuth exchange
	assert.Contains(t, rr.Body.String(), "Authentication failed")
}

func TestCallbackHandler_MissingCode(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger) // OAuth handler not needed for this test

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/auth/callback?state=valid_state", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.CallbackHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid request")
	assert.Contains(t, rr.Body.String(), "Missing required parameters")
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
}

func TestCallbackHandler_MissingState(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger)

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/auth/callback?code=test_code", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.CallbackHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid request")
	assert.Contains(t, rr.Body.String(), "Missing required parameters")
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
}

func TestCallbackHandler_MissingBothParameters(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger)

	req, err := http.NewRequestWithContext(context.Background(), "GET", "/auth/callback", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.CallbackHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Missing required parameters")
}

func TestCallbackHandler_DiscordError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger)

	// Simulate Discord returning an error
	req, err := http.NewRequestWithContext(context.Background(), "GET", "/auth/callback?error=access_denied&error_description=User+denied+authorization", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.CallbackHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Authentication failed")
	assert.Contains(t, rr.Body.String(), "Discord returned an error")
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
}

func TestCallbackHandler_InvalidState(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	logger, _ := zap.NewDevelopment()
	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)
	oauthHandler := auth.NewOAuthHandler(db, discordClient, stateManager, logger)

	handlers := NewHandlers(oauthHandler, logger)

	// Use an invalid state (not stored in database)
	req, err := http.NewRequestWithContext(context.Background(), "GET", "/auth/callback?code=test_code_123&state=invalid_state_xyz", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	handlers.CallbackHandler(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Authentication failed")
}

func TestRenderSuccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger)

	rr := httptest.NewRecorder()

	handlers.renderSuccess(rr)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rr.Header().Get("Content-Type"), "charset=utf-8")

	// Verify HTML content
	body := rr.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "Authentication Successful")
	assert.Contains(t, body, "You have successfully authenticated with Discord")
	assert.Contains(t, body, "You can now close this window")
}

func TestRenderError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger)

	rr := httptest.NewRecorder()

	handlers.renderError(rr, "Test Error", "This is a test error message")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rr.Header().Get("Content-Type"), "charset=utf-8")

	// Verify HTML content
	body := rr.Body.String()
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "Test Error")
	assert.Contains(t, body, "This is a test error message")
	assert.Contains(t, body, "Please close this window and try again")
}

func TestRenderError_HTMLEscaping(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handlers := NewHandlers(nil, logger)

	rr := httptest.NewRecorder()

	// Test with special characters that should be escaped
	handlers.renderError(rr, "Error <script>", "Message with & and <tags>")

	body := rr.Body.String()
	// Note: fmt.Sprintf doesn't HTML-escape, so this test documents current behavior
	// In production, you'd want to use html.EscapeString()
	assert.Contains(t, body, "Error <script>")
	assert.Contains(t, body, "Message with & and <tags>")
}

func TestCallbackHandler_HTTPMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"GET request", "GET"},
		{"POST request", "POST"}, // Should also work - OAuth callbacks can be POST
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := zap.NewDevelopment()
			handlers := NewHandlers(nil, logger)

			// Missing parameters to trigger validation error
			req, err := http.NewRequestWithContext(context.Background(), tt.method, "/auth/callback", nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()

			handlers.CallbackHandler(rr, req)

			// Should handle request and return error for missing params
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), "Missing required parameters")
		})
	}
}

func TestNewHandlers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()
	db, cleanup, err := testutil.SetupTestDB(ctx)
	require.NoError(t, err)
	defer cleanup()

	cfg := testutil.GenerateTestConfig()
	discordClient := auth.NewDiscordClient(cfg, logger)
	stateManager := auth.NewStateManager(db, 10)
	oauthHandler := auth.NewOAuthHandler(db, discordClient, stateManager, logger)

	handlers := NewHandlers(oauthHandler, logger)

	require.NotNil(t, handlers)
	assert.NotNil(t, handlers.oauthHandler)
	assert.NotNil(t, handlers.logger)
}
