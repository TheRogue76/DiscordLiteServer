// Package config provides application configuration management using environment variables.
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Discord  DiscordConfig
	Database DatabaseConfig
	Security SecurityConfig
	Logging  LoggingConfig
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	HTTPPort string
	GRPCPort string
	Host     string
	Env      string
}

// DiscordConfig holds Discord OAuth configuration
type DiscordConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	TokenEncryptionKey []byte
	SessionExpiryHours int
	StateExpiryMinutes int
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string
	Format string
}

// Load loads configuration from environment variables
// It optionally loads from a .env file if it exists
func Load() (*Config, error) {
	// Try to load .env file (optional, ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{}

	// Load Server Config
	cfg.Server = ServerConfig{
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		GRPCPort: getEnv("GRPC_PORT", "50051"),
		Host:     getEnv("SERVER_HOST", "localhost"),
		Env:      getEnv("ENVIRONMENT", "development"),
	}

	// Load Discord Config
	cfg.Discord = DiscordConfig{
		ClientID:     getEnv("DISCORD_CLIENT_ID", ""),
		ClientSecret: getEnv("DISCORD_CLIENT_SECRET", ""),
		RedirectURI:  getEnv("DISCORD_REDIRECT_URI", ""),
		Scopes:       strings.Split(getEnv("DISCORD_OAUTH_SCOPES", "identify email guilds"), " "),
	}

	// Load Database Config
	maxOpenConns, _ := strconv.Atoi(getEnv("DB_MAX_OPEN_CONNS", "25"))
	maxIdleConns, _ := strconv.Atoi(getEnv("DB_MAX_IDLE_CONNS", "5"))

	cfg.Database = DatabaseConfig{
		Host:         getEnv("DB_HOST", "localhost"),
		Port:         getEnv("DB_PORT", "5432"),
		User:         getEnv("DB_USER", "discordlite"),
		Password:     getEnv("DB_PASSWORD", ""),
		Name:         getEnv("DB_NAME", "discordlite_db"),
		SSLMode:      getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns: maxOpenConns,
		MaxIdleConns: maxIdleConns,
	}

	// Load Security Config
	sessionExpiryHours, _ := strconv.Atoi(getEnv("SESSION_EXPIRY_HOURS", "24"))
	stateExpiryMinutes, _ := strconv.Atoi(getEnv("STATE_EXPIRY_MINUTES", "10"))

	encryptionKeyHex := getEnv("TOKEN_ENCRYPTION_KEY", "")
	encryptionKey, err := hex.DecodeString(encryptionKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid TOKEN_ENCRYPTION_KEY: must be a hex-encoded string: %w", err)
	}

	cfg.Security = SecurityConfig{
		TokenEncryptionKey: encryptionKey,
		SessionExpiryHours: sessionExpiryHours,
		StateExpiryMinutes: stateExpiryMinutes,
	}

	// Load Logging Config
	cfg.Logging = LoggingConfig{
		Level:  getEnv("LOG_LEVEL", "info"),
		Format: getEnv("LOG_FORMAT", "json"),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Discord Config
	if c.Discord.ClientID == "" {
		return fmt.Errorf("DISCORD_CLIENT_ID is required")
	}
	if c.Discord.ClientSecret == "" {
		return fmt.Errorf("DISCORD_CLIENT_SECRET is required")
	}
	if c.Discord.RedirectURI == "" {
		return fmt.Errorf("DISCORD_REDIRECT_URI is required")
	}

	// Validate Database Config
	if c.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	// Validate Security Config
	if len(c.Security.TokenEncryptionKey) != 32 {
		return fmt.Errorf("TOKEN_ENCRYPTION_KEY must be exactly 32 bytes (64 hex characters) for AES-256")
	}
	if c.Security.SessionExpiryHours <= 0 {
		return fmt.Errorf("SESSION_EXPIRY_HOURS must be positive")
	}
	if c.Security.StateExpiryMinutes <= 0 {
		return fmt.Errorf("STATE_EXPIRY_MINUTES must be positive")
	}

	// Validate Logging Config
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error")
	}
	validLogFormats := map[string]bool{"json": true, "console": true}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("LOG_FORMAT must be one of: json, console")
	}

	return nil
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// getEnv retrieves an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
