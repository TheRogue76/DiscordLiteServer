-- Initial schema for Discord Lite Server
-- PostgreSQL 13+

-- Users table: stores basic Discord user information
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    discord_id VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255) NOT NULL,
    discriminator VARCHAR(10),
    avatar VARCHAR(255),
    email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_discord_id ON users(discord_id);

-- OAuth tokens table: stores encrypted OAuth credentials
CREATE TABLE oauth_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token TEXT NOT NULL,           -- Encrypted with AES-256-GCM
    refresh_token TEXT NOT NULL,          -- Encrypted with AES-256-GCM
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP WITH TIME ZONE NOT NULL,
    scope TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id)                       -- One token per user
);

CREATE INDEX idx_oauth_tokens_user_id ON oauth_tokens(user_id);
CREATE INDEX idx_oauth_tokens_expiry ON oauth_tokens(expiry);

-- OAuth states table: temporary storage for OAuth state validation (CSRF protection)
CREATE TABLE oauth_states (
    state VARCHAR(255) PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_oauth_states_expires_at ON oauth_states(expires_at);
CREATE INDEX idx_oauth_states_session_id ON oauth_states(session_id);

-- Auth sessions table: maps session IDs to users for gRPC clients
CREATE TABLE auth_sessions (
    session_id VARCHAR(255) PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    auth_status VARCHAR(50) NOT NULL,     -- 'pending', 'authenticated', 'failed'
    error_message TEXT,                   -- Error message if status is 'failed'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_auth_sessions_user_id ON auth_sessions(user_id);
CREATE INDEX idx_auth_sessions_status ON auth_sessions(auth_status);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions(expires_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers to automatically update updated_at
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_oauth_tokens_updated_at
    BEFORE UPDATE ON oauth_tokens
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_auth_sessions_updated_at
    BEFORE UPDATE ON auth_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
