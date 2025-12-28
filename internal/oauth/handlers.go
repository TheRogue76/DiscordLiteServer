// Package oauth provides HTTP server handlers for OAuth callbacks and health checks.
package oauth

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/parsascontentcorner/discordliteserver/internal/auth"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	oauthHandler *auth.OAuthHandler
	logger       *zap.Logger
}

// NewHandlers creates a new handlers instance
func NewHandlers(oauthHandler *auth.OAuthHandler, logger *zap.Logger) *Handlers {
	return &Handlers{
		oauthHandler: oauthHandler,
		logger:       logger,
	}
}

// HealthHandler handles health check requests
func (h *Handlers) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		h.logger.Error("failed to write health check response", zap.Error(err))
	}
}

// CallbackHandler handles the OAuth callback from Discord
func (h *Handlers) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Get code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	// Check for error from Discord
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		errDesc := r.URL.Query().Get("error_description")
		h.logger.Error("oauth error from discord",
			zap.String("error", errParam),
			zap.String("description", errDesc),
		)
		h.renderError(w, "Authentication failed", fmt.Sprintf("Discord returned an error: %s", errDesc))
		return
	}

	// Validate parameters
	if code == "" || state == "" {
		h.logger.Error("missing required parameters", zap.String("code", code), zap.String("state", state))
		h.renderError(w, "Invalid request", "Missing required parameters (code or state)")
		return
	}

	h.logger.Info("received oauth callback",
		zap.String("state", state),
		zap.Bool("has_code", code != ""),
	)

	// Process the OAuth callback
	if err := h.oauthHandler.HandleCallback(r.Context(), code, state); err != nil {
		h.logger.Error("failed to handle oauth callback", zap.Error(err))
		h.renderError(w, "Authentication failed", "Failed to complete authentication. Please try again.")
		return
	}

	// Render success page
	h.renderSuccess(w)
}

// renderSuccess renders a success HTML page
func (h *Handlers) renderSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	html := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Authentication Successful</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .container {
            background: white;
            padding: 3rem;
            border-radius: 10px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            text-align: center;
            max-width: 400px;
        }
        .checkmark {
            width: 80px;
            height: 80px;
            margin: 0 auto 1rem;
            border-radius: 50%;
            background: #4caf50;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .checkmark svg {
            width: 50px;
            height: 50px;
            stroke: white;
            stroke-width: 3;
            fill: none;
        }
        h1 {
            color: #333;
            margin: 0 0 1rem;
        }
        p {
            color: #666;
            margin: 0;
            line-height: 1.6;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="checkmark">
            <svg viewBox="0 0 52 52">
                <path d="M14 27l7.5 7.5L38 18"/>
            </svg>
        </div>
        <h1>Authentication Successful!</h1>
        <p>You have successfully authenticated with Discord.</p>
        <p style="margin-top: 1rem;">You can now close this window and return to the application.</p>
    </div>
</body>
</html>
`
	if _, err := w.Write([]byte(html)); err != nil {
		h.logger.Error("failed to write success response", zap.Error(err))
	}
}

// renderError renders an error HTML page
func (h *Handlers) renderError(w http.ResponseWriter, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
        }
        .container {
            background: white;
            padding: 3rem;
            border-radius: 10px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            text-align: center;
            max-width: 400px;
        }
        .error-icon {
            width: 80px;
            height: 80px;
            margin: 0 auto 1rem;
            border-radius: 50%%;
            background: #f44336;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .error-icon svg {
            width: 50px;
            height: 50px;
            stroke: white;
            stroke-width: 3;
            fill: none;
        }
        h1 {
            color: #333;
            margin: 0 0 1rem;
        }
        p {
            color: #666;
            margin: 0;
            line-height: 1.6;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="error-icon">
            <svg viewBox="0 0 52 52">
                <path d="M16 16l20 20M36 16l-20 20"/>
            </svg>
        </div>
        <h1>%s</h1>
        <p>%s</p>
        <p style="margin-top: 1rem;">Please close this window and try again.</p>
    </div>
</body>
</html>
`, title, title, message)

	if _, err := w.Write([]byte(html)); err != nil {
		h.logger.Error("failed to write error response", zap.Error(err))
	}
}
