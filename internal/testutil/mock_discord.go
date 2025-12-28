package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockDiscordServer represents a mock Discord API server for testing.
type MockDiscordServer struct {
	Server       *httptest.Server
	TokenCalls   int
	UserInfoCalls int
}

// DiscordTokenResponse represents the OAuth token response from Discord.
type DiscordTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// DiscordUserResponse represents the user info response from Discord.
type DiscordUserResponse struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
}

// DiscordErrorResponse represents an error response from Discord.
type DiscordErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// NewMockDiscordServer creates a new mock Discord API server.
// The server handles token exchange and user info endpoints.
func NewMockDiscordServer() *MockDiscordServer {
	mds := &MockDiscordServer{}

	mux := http.NewServeMux()

	// Token exchange endpoint
	mux.HandleFunc("/api/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		mds.TokenCalls++

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")

		// Simulate different responses based on the code
		switch code {
		case "valid_code":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordTokenResponse{
				AccessToken:  "mock_access_token_123",
				TokenType:    "Bearer",
				ExpiresIn:    604800, // 7 days
				RefreshToken: "mock_refresh_token_456",
				Scope:        "identify email guilds",
			})

		case "error_code":
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordErrorResponse{
				Error:            "invalid_grant",
				ErrorDescription: "Invalid authorization code",
			})

		case "server_error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))

		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordErrorResponse{
				Error:            "invalid_request",
				ErrorDescription: "Unknown code",
			})
		}
	})

	// User info endpoint
	mux.HandleFunc("/api/v10/users/@me", func(w http.ResponseWriter, r *http.Request) {
		mds.UserInfoCalls++

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordErrorResponse{
				Error:            "unauthorized",
				ErrorDescription: "Missing or invalid authorization header",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Simulate different responses based on the token
		switch token {
		case "mock_access_token_123":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordUserResponse{
				ID:            "123456789012345678",
				Username:      "TestUser",
				Discriminator: "1234",
				Avatar:        "avatar_hash_123",
				Email:         "testuser@example.com",
			})

		case "invalid_token":
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordErrorResponse{
				Error:            "unauthorized",
				ErrorDescription: "Invalid token",
			})

		case "not_found":
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordErrorResponse{
				Error:            "not_found",
				ErrorDescription: "User not found",
			})

		case "server_error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))

		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DiscordErrorResponse{
				Error:            "invalid_request",
				ErrorDescription: "Unknown token",
			})
		}
	})

	mds.Server = httptest.NewServer(mux)
	return mds
}

// Close closes the mock server.
func (mds *MockDiscordServer) Close() {
	if mds.Server != nil {
		mds.Server.Close()
	}
}

// GetTokenURL returns the token exchange endpoint URL.
func (mds *MockDiscordServer) GetTokenURL() string {
	return fmt.Sprintf("%s/api/oauth2/token", mds.Server.URL)
}

// GetUserInfoURL returns the user info endpoint URL.
func (mds *MockDiscordServer) GetUserInfoURL() string {
	return fmt.Sprintf("%s/api/v10/users/@me", mds.Server.URL)
}

// ResetCallCounts resets the call counters.
func (mds *MockDiscordServer) ResetCallCounts() {
	mds.TokenCalls = 0
	mds.UserInfoCalls = 0
}
