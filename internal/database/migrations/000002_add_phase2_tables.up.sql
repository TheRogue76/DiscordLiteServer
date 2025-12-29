-- Phase 2: Add tables for guilds, channels, messages, cache, and WebSocket sessions

-- 1. Guilds table - stores Discord guild (server) information
CREATE TABLE guilds (
    id BIGSERIAL PRIMARY KEY,
    discord_guild_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    icon VARCHAR(255),
    owner_id VARCHAR(255),
    permissions BIGINT DEFAULT 0,
    features TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. User-Guild membership table (many-to-many)
CREATE TABLE user_guilds (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id BIGINT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, guild_id)
);

-- 3. Channels table - stores Discord channel information
CREATE TABLE channels (
    id BIGSERIAL PRIMARY KEY,
    discord_channel_id VARCHAR(255) UNIQUE NOT NULL,
    guild_id BIGINT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type INT NOT NULL,
    position INT DEFAULT 0,
    parent_id VARCHAR(255),
    topic TEXT,
    nsfw BOOLEAN DEFAULT FALSE,
    last_message_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 4. Messages table - stores Discord messages
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    discord_message_id VARCHAR(255) UNIQUE NOT NULL,
    channel_id BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    author_id VARCHAR(255) NOT NULL,
    author_username VARCHAR(255) NOT NULL,
    author_avatar VARCHAR(255),
    content TEXT,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    edited_timestamp TIMESTAMP WITH TIME ZONE,
    message_type INT DEFAULT 0,
    referenced_message_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 5. Message attachments table
CREATE TABLE message_attachments (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    attachment_id VARCHAR(255) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    proxy_url TEXT,
    size_bytes INT DEFAULT 0,
    width INT,
    height INT,
    content_type VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(message_id, attachment_id)
);

-- 6. Cache metadata table - tracks cache TTL for guilds/channels/messages
CREATE TABLE cache_metadata (
    id BIGSERIAL PRIMARY KEY,
    cache_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    last_fetched_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(cache_type, entity_id, user_id)
);

-- 7. WebSocket sessions table - tracks active Discord Gateway connections
CREATE TABLE websocket_sessions (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(255) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    gateway_url TEXT NOT NULL,
    session_token TEXT,
    sequence_number BIGINT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'connecting',
    last_heartbeat_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Indexes for performance optimization

-- Guilds indexes
CREATE INDEX idx_guilds_discord_id ON guilds(discord_guild_id);

-- User-Guilds indexes
CREATE INDEX idx_user_guilds_user_id ON user_guilds(user_id);
CREATE INDEX idx_user_guilds_guild_id ON user_guilds(guild_id);

-- Channels indexes
CREATE INDEX idx_channels_guild_id ON channels(guild_id);
CREATE INDEX idx_channels_discord_id ON channels(discord_channel_id);

-- Messages indexes (critical for pagination)
CREATE INDEX idx_messages_channel_id ON messages(channel_id);
CREATE INDEX idx_messages_timestamp ON messages(channel_id, timestamp DESC);
CREATE INDEX idx_messages_discord_id ON messages(discord_message_id);

-- Message attachments indexes
CREATE INDEX idx_message_attachments_message_id ON message_attachments(message_id);

-- Cache metadata indexes (critical for cache lookup and cleanup)
CREATE INDEX idx_cache_metadata_lookup ON cache_metadata(cache_type, entity_id, user_id);
CREATE INDEX idx_cache_metadata_expires_at ON cache_metadata(expires_at);
CREATE INDEX idx_cache_metadata_user_id ON cache_metadata(user_id);

-- WebSocket sessions indexes
CREATE INDEX idx_websocket_sessions_user_id ON websocket_sessions(user_id);
CREATE INDEX idx_websocket_sessions_status ON websocket_sessions(status);
CREATE INDEX idx_websocket_sessions_expires_at ON websocket_sessions(expires_at);

-- Updated_at triggers for automatic timestamp updates

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_guilds_updated_at
    BEFORE UPDATE ON guilds
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_channels_updated_at
    BEFORE UPDATE ON channels
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_messages_updated_at
    BEFORE UPDATE ON messages
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_cache_metadata_updated_at
    BEFORE UPDATE ON cache_metadata
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_websocket_sessions_updated_at
    BEFORE UPDATE ON websocket_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
