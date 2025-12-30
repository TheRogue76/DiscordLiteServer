package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEnv sets up environment variables for testing and returns a cleanup function
func setupTestEnv(t *testing.T, envVars map[string]string) func() {
	// Store original values
	original := make(map[string]string)
	for key := range envVars {
		original[key] = os.Getenv(key)
	}

	// Set test values
	for key, value := range envVars {
		if value == "" {
			err := os.Unsetenv(key)
			if err != nil {
				t.Error(err)
			}
		} else {
			err := os.Setenv(key, value)
			if err != nil {
				t.Error(err)
			}
		}
	}

	// Return cleanup function
	return func() {
		for key, value := range original {
			if value == "" {
				err := os.Unsetenv(key)
				if err != nil {
					t.Error(err)
				}
			} else {
				err := os.Setenv(key, value)
				if err != nil {
					t.Error(err)
				}
			}
		}
	}
}

func TestLoadConfigSuccess(t *testing.T) {
	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "test_client_id_123",
		"DISCORD_CLIENT_SECRET": "test_client_secret_456",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/auth/callback",
		"DB_PASSWORD":           "test_db_password",
		"TOKEN_ENCRYPTION_KEY":  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", // 64 hex chars = 32 bytes
		"HTTP_PORT":             "9090",
		"GRPC_PORT":             "50052",
		"LOG_LEVEL":             "debug",
		"LOG_FORMAT":            "console",
	})
	defer cleanup()

	cfg, err := Load()

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify Discord config
	assert.Equal(t, "test_client_id_123", cfg.Discord.ClientID)
	assert.Equal(t, "test_client_secret_456", cfg.Discord.ClientSecret)
	assert.Equal(t, "http://localhost:8080/auth/callback", cfg.Discord.RedirectURI)
	assert.Equal(t, []string{"identify", "email", "guilds"}, cfg.Discord.Scopes)

	// Verify Server config
	assert.Equal(t, "9090", cfg.Server.HTTPPort)
	assert.Equal(t, "50052", cfg.Server.GRPCPort)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, "development", cfg.Server.Env)

	// Verify Database config
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "5432", cfg.Database.Port)
	assert.Equal(t, "discordlite", cfg.Database.User)
	assert.Equal(t, "test_db_password", cfg.Database.Password)
	assert.Equal(t, "discordlite_db", cfg.Database.Name)
	assert.Equal(t, "disable", cfg.Database.SSLMode)

	// Verify Security config
	assert.Equal(t, 32, len(cfg.Security.TokenEncryptionKey))
	assert.Equal(t, 24, cfg.Security.SessionExpiryHours)
	assert.Equal(t, 10, cfg.Security.StateExpiryMinutes)

	// Verify Logging config
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "console", cfg.Logging.Format)
}

func TestLoadConfigMissingRequired(t *testing.T) {
	validEncryptionKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		envVars     map[string]string
		expectedErr string
	}{
		{
			name: "missing DISCORD_CLIENT_ID",
			envVars: map[string]string{
				"DISCORD_CLIENT_ID":     "",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validEncryptionKey,
			},
			expectedErr: "DISCORD_CLIENT_ID is required",
		},
		{
			name: "missing DISCORD_CLIENT_SECRET",
			envVars: map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validEncryptionKey,
			},
			expectedErr: "DISCORD_CLIENT_SECRET is required",
		},
		{
			name: "missing DISCORD_REDIRECT_URI",
			envVars: map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validEncryptionKey,
			},
			expectedErr: "DISCORD_REDIRECT_URI is required",
		},
		{
			name: "missing DB_PASSWORD",
			envVars: map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "",
				"TOKEN_ENCRYPTION_KEY":  validEncryptionKey,
			},
			expectedErr: "DB_PASSWORD is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, tt.envVars)
			defer cleanup()

			cfg, err := Load()

			assert.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestLoadConfigInvalidEncryptionKey(t *testing.T) {
	tests := []struct {
		name           string
		encryptionKey  string
		expectedErrMsg string
	}{
		{
			name:           "non-hex characters",
			encryptionKey:  "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			expectedErrMsg: "invalid TOKEN_ENCRYPTION_KEY: must be a hex-encoded string",
		},
		{
			name:           "too short - 16 bytes",
			encryptionKey:  "0123456789abcdef0123456789abcdef",
			expectedErrMsg: "TOKEN_ENCRYPTION_KEY must be exactly 32 bytes",
		},
		{
			name:           "too long - 40 bytes",
			encryptionKey:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expectedErrMsg: "TOKEN_ENCRYPTION_KEY must be exactly 32 bytes",
		},
		{
			name:           "empty encryption key",
			encryptionKey:  "",
			expectedErrMsg: "TOKEN_ENCRYPTION_KEY must be exactly 32 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  tt.encryptionKey,
			})
			defer cleanup()

			cfg, err := Load()

			assert.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}

func TestValidateEncryptionKey(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "client_id",
		"DISCORD_CLIENT_SECRET": "secret",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
		"DB_PASSWORD":           "password",
		"TOKEN_ENCRYPTION_KEY":  validKey,
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	// Key should be exactly 32 bytes
	assert.Equal(t, 32, len(cfg.Security.TokenEncryptionKey))
}

func TestValidateSessionExpiry(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		hours       string
		shouldError bool
		expectedErr string
	}{
		{
			name:        "positive hours",
			hours:       "48",
			shouldError: false,
		},
		{
			name:        "zero hours",
			hours:       "0",
			shouldError: true,
			expectedErr: "SESSION_EXPIRY_HOURS must be positive",
		},
		{
			name:        "negative hours",
			hours:       "-1",
			shouldError: true,
			expectedErr: "SESSION_EXPIRY_HOURS must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validKey,
				"SESSION_EXPIRY_HOURS":  tt.hours,
			})
			defer cleanup()

			cfg, err := Load()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestValidateStateExpiry(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		minutes     string
		shouldError bool
		expectedErr string
	}{
		{
			name:        "positive minutes",
			minutes:     "15",
			shouldError: false,
		},
		{
			name:        "zero minutes",
			minutes:     "0",
			shouldError: true,
			expectedErr: "STATE_EXPIRY_MINUTES must be positive",
		},
		{
			name:        "negative minutes",
			minutes:     "-5",
			shouldError: true,
			expectedErr: "STATE_EXPIRY_MINUTES must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validKey,
				"STATE_EXPIRY_MINUTES":  tt.minutes,
			})
			defer cleanup()

			cfg, err := Load()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		level       string
		shouldError bool
	}{
		{name: "debug level", level: "debug", shouldError: false},
		{name: "info level", level: "info", shouldError: false},
		{name: "warn level", level: "warn", shouldError: false},
		{name: "error level", level: "error", shouldError: false},
		{name: "invalid level", level: "trace", shouldError: true},
		{name: "invalid uppercase", level: "DEBUG", shouldError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validKey,
				"LOG_LEVEL":             tt.level,
			})
			defer cleanup()

			cfg, err := Load()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), "LOG_LEVEL must be one of")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, tt.level, cfg.Logging.Level)
			}
		})
	}
}

func TestValidateLogFormat(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		format      string
		shouldError bool
	}{
		{name: "json format", format: "json", shouldError: false},
		{name: "console format", format: "console", shouldError: false},
		{name: "invalid format", format: "xml", shouldError: true},
		{name: "invalid uppercase", format: "JSON", shouldError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validKey,
				"LOG_FORMAT":            tt.format,
			})
			defer cleanup()

			cfg, err := Load()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), "LOG_FORMAT must be one of")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, tt.format, cfg.Logging.Format)
			}
		})
	}
}

func TestGetDSN(t *testing.T) {
	dbConfig := DatabaseConfig{
		Host:     "testhost",
		Port:     "5433",
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "require",
	}

	dsn := dbConfig.GetDSN()

	expected := "host=testhost port=5433 user=testuser password=testpass dbname=testdb sslmode=require"
	assert.Equal(t, expected, dsn)
}

func TestDefaultValues(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	// Only set required fields, let defaults apply
	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "client_id",
		"DISCORD_CLIENT_SECRET": "secret",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
		"DB_PASSWORD":           "password",
		"TOKEN_ENCRYPTION_KEY":  validKey,
		// Unset all optional fields to test defaults
		"HTTP_PORT":            "",
		"GRPC_PORT":            "",
		"SERVER_HOST":          "",
		"ENVIRONMENT":          "",
		"DB_HOST":              "",
		"DB_PORT":              "",
		"DB_USER":              "",
		"DB_NAME":              "",
		"DB_SSLMODE":           "",
		"DB_MAX_OPEN_CONNS":    "",
		"DB_MAX_IDLE_CONNS":    "",
		"SESSION_EXPIRY_HOURS": "",
		"STATE_EXPIRY_MINUTES": "",
		"LOG_LEVEL":            "",
		"LOG_FORMAT":           "",
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify Server defaults
	assert.Equal(t, "8080", cfg.Server.HTTPPort)
	assert.Equal(t, "50051", cfg.Server.GRPCPort)
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, "development", cfg.Server.Env)

	// Verify Database defaults
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "5432", cfg.Database.Port)
	assert.Equal(t, "discordlite", cfg.Database.User)
	assert.Equal(t, "discordlite_db", cfg.Database.Name)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.Equal(t, 5, cfg.Database.MaxIdleConns)

	// Verify Security defaults
	assert.Equal(t, 24, cfg.Security.SessionExpiryHours)
	assert.Equal(t, 10, cfg.Security.StateExpiryMinutes)

	// Verify Logging defaults
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)

	// Verify Discord default scopes
	assert.Equal(t, []string{"identify", "email", "guilds"}, cfg.Discord.Scopes)
}

func TestCustomDatabaseConnections(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "client_id",
		"DISCORD_CLIENT_SECRET": "secret",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
		"DB_PASSWORD":           "password",
		"TOKEN_ENCRYPTION_KEY":  validKey,
		"DB_MAX_OPEN_CONNS":     "50",
		"DB_MAX_IDLE_CONNS":     "10",
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
}

func TestCustomScopes(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "client_id",
		"DISCORD_CLIENT_SECRET": "secret",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
		"DB_PASSWORD":           "password",
		"TOKEN_ENCRYPTION_KEY":  validKey,
		"DISCORD_OAUTH_SCOPES":  "identify guilds guilds.members.read",
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, []string{"identify", "guilds", "guilds.members.read"}, cfg.Discord.Scopes)
}

// Phase 2 Tests: Cache Configuration

func TestCacheConfigDefaults(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "client_id",
		"DISCORD_CLIENT_SECRET": "secret",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
		"DB_PASSWORD":           "password",
		"TOKEN_ENCRYPTION_KEY":  validKey,
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	// Verify Cache defaults
	assert.Equal(t, 1, cfg.Cache.GuildTTLHours)
	assert.Equal(t, 30, cfg.Cache.ChannelTTLMinutes)
	assert.Equal(t, 5, cfg.Cache.MessageTTLMinutes)
}

func TestCacheConfigCustomValues(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":         "client_id",
		"DISCORD_CLIENT_SECRET":     "secret",
		"DISCORD_REDIRECT_URI":      "http://localhost:8080/callback",
		"DB_PASSWORD":               "password",
		"TOKEN_ENCRYPTION_KEY":      validKey,
		"CACHE_GUILD_TTL_HOURS":     "2",
		"CACHE_CHANNEL_TTL_MINUTES": "60",
		"CACHE_MESSAGE_TTL_MINUTES": "10",
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 2, cfg.Cache.GuildTTLHours)
	assert.Equal(t, 60, cfg.Cache.ChannelTTLMinutes)
	assert.Equal(t, 10, cfg.Cache.MessageTTLMinutes)
}

func TestValidateCacheTTL(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		guildTTL    string
		channelTTL  string
		messageTTL  string
		shouldError bool
		expectedErr string
	}{
		{
			name:        "all positive values",
			guildTTL:    "2",
			channelTTL:  "45",
			messageTTL:  "10",
			shouldError: false,
		},
		{
			name:        "zero guild TTL",
			guildTTL:    "0",
			channelTTL:  "30",
			messageTTL:  "5",
			shouldError: true,
			expectedErr: "CACHE_GUILD_TTL_HOURS must be positive",
		},
		{
			name:        "negative channel TTL",
			guildTTL:    "1",
			channelTTL:  "-1",
			messageTTL:  "5",
			shouldError: true,
			expectedErr: "CACHE_CHANNEL_TTL_MINUTES must be positive",
		},
		{
			name:        "zero message TTL",
			guildTTL:    "1",
			channelTTL:  "30",
			messageTTL:  "0",
			shouldError: true,
			expectedErr: "CACHE_MESSAGE_TTL_MINUTES must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":         "client_id",
				"DISCORD_CLIENT_SECRET":     "secret",
				"DISCORD_REDIRECT_URI":      "http://localhost:8080/callback",
				"DB_PASSWORD":               "password",
				"TOKEN_ENCRYPTION_KEY":      validKey,
				"CACHE_GUILD_TTL_HOURS":     tt.guildTTL,
				"CACHE_CHANNEL_TTL_MINUTES": tt.channelTTL,
				"CACHE_MESSAGE_TTL_MINUTES": tt.messageTTL,
			})
			defer cleanup()

			cfg, err := Load()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

// Phase 2 Tests: WebSocket Configuration

func TestWebSocketConfigDefaults(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":     "client_id",
		"DISCORD_CLIENT_SECRET": "secret",
		"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
		"DB_PASSWORD":           "password",
		"TOKEN_ENCRYPTION_KEY":  validKey,
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	// Verify WebSocket defaults
	assert.Equal(t, true, cfg.WebSocket.Enabled)
	assert.Equal(t, 5, cfg.WebSocket.MaxConnectionsPerUser)
	assert.Equal(t, 30, cfg.WebSocket.HeartbeatInterval)
	assert.Equal(t, 3, cfg.WebSocket.ReconnectAttempts)
	assert.Equal(t, 5, cfg.WebSocket.ReconnectDelay)
}

func TestWebSocketConfigCustomValues(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	cleanup := setupTestEnv(t, map[string]string{
		"DISCORD_CLIENT_ID":                  "client_id",
		"DISCORD_CLIENT_SECRET":              "secret",
		"DISCORD_REDIRECT_URI":               "http://localhost:8080/callback",
		"DB_PASSWORD":                        "password",
		"TOKEN_ENCRYPTION_KEY":               validKey,
		"WEBSOCKET_ENABLED":                  "false",
		"WEBSOCKET_MAX_CONNECTIONS_PER_USER": "10",
		"WEBSOCKET_HEARTBEAT_INTERVAL":       "60",
		"WEBSOCKET_RECONNECT_ATTEMPTS":       "5",
		"WEBSOCKET_RECONNECT_DELAY":          "10",
	})
	defer cleanup()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, false, cfg.WebSocket.Enabled)
	assert.Equal(t, 10, cfg.WebSocket.MaxConnectionsPerUser)
	assert.Equal(t, 60, cfg.WebSocket.HeartbeatInterval)
	assert.Equal(t, 5, cfg.WebSocket.ReconnectAttempts)
	assert.Equal(t, 10, cfg.WebSocket.ReconnectDelay)
}

func TestValidateWebSocketConfig(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name        string
		maxConns    string
		heartbeat   string
		attempts    string
		delay       string
		shouldError bool
		expectedErr string
	}{
		{
			name:        "all valid values",
			maxConns:    "10",
			heartbeat:   "45",
			attempts:    "5",
			delay:       "10",
			shouldError: false,
		},
		{
			name:        "zero max connections",
			maxConns:    "0",
			heartbeat:   "30",
			attempts:    "3",
			delay:       "5",
			shouldError: true,
			expectedErr: "WEBSOCKET_MAX_CONNECTIONS_PER_USER must be positive",
		},
		{
			name:        "negative heartbeat interval",
			maxConns:    "5",
			heartbeat:   "-1",
			attempts:    "3",
			delay:       "5",
			shouldError: true,
			expectedErr: "WEBSOCKET_HEARTBEAT_INTERVAL must be positive",
		},
		{
			name:        "negative reconnect attempts (should allow)",
			maxConns:    "5",
			heartbeat:   "30",
			attempts:    "-1",
			delay:       "5",
			shouldError: true,
			expectedErr: "WEBSOCKET_RECONNECT_ATTEMPTS must be non-negative",
		},
		{
			name:        "zero reconnect attempts (should allow)",
			maxConns:    "5",
			heartbeat:   "30",
			attempts:    "0",
			delay:       "5",
			shouldError: false,
		},
		{
			name:        "zero reconnect delay",
			maxConns:    "5",
			heartbeat:   "30",
			attempts:    "3",
			delay:       "0",
			shouldError: true,
			expectedErr: "WEBSOCKET_RECONNECT_DELAY must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, map[string]string{
				"DISCORD_CLIENT_ID":                  "client_id",
				"DISCORD_CLIENT_SECRET":              "secret",
				"DISCORD_REDIRECT_URI":               "http://localhost:8080/callback",
				"DB_PASSWORD":                        "password",
				"TOKEN_ENCRYPTION_KEY":               validKey,
				"WEBSOCKET_MAX_CONNECTIONS_PER_USER": tt.maxConns,
				"WEBSOCKET_HEARTBEAT_INTERVAL":       tt.heartbeat,
				"WEBSOCKET_RECONNECT_ATTEMPTS":       tt.attempts,
				"WEBSOCKET_RECONNECT_DELAY":          tt.delay,
			})
			defer cleanup()

			cfg, err := Load()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestWebSocketEnabledParsing(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{name: "true", value: "true", expected: true},
		{name: "false", value: "false", expected: false},
		{name: "TRUE", value: "TRUE", expected: false}, // case-sensitive
		{name: "1", value: "1", expected: false},       // not "true"
		{name: "empty defaults to true", value: "", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := map[string]string{
				"DISCORD_CLIENT_ID":     "client_id",
				"DISCORD_CLIENT_SECRET": "secret",
				"DISCORD_REDIRECT_URI":  "http://localhost:8080/callback",
				"DB_PASSWORD":           "password",
				"TOKEN_ENCRYPTION_KEY":  validKey,
			}
			if tt.value != "" {
				envVars["WEBSOCKET_ENABLED"] = tt.value
			}

			cleanup := setupTestEnv(t, envVars)
			defer cleanup()

			cfg, err := Load()
			require.NoError(t, err)

			assert.Equal(t, tt.expected, cfg.WebSocket.Enabled)
		})
	}
}
