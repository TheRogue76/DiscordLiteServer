# Discord Lite Server

A high-performance Golang backend service that handles Discord OAuth authentication with a gRPC API for client applications.

## Features

### Phase 1: Authentication
- **Discord OAuth 2.0 Authentication**: Complete OAuth flow with CSRF protection
- **gRPC API**: Modern gRPC interface for client communication
- **Secure Token Storage**: AES-256-GCM encryption for OAuth tokens
- **PostgreSQL Database**: Persistent storage with connection pooling
- **Concurrent Servers**: HTTP (OAuth callbacks) and gRPC (client API) running together
- **Session Management**: Automatic cleanup of expired sessions
- **Docker Support**: Full containerization with docker-compose
- **Production Ready**: Structured logging, graceful shutdown, health checks

### Phase 2: Discord Integration
- **Guild Browsing**: Fetch user's Discord servers with smart caching
- **Channel Access**: List channels in guilds with permission validation
- **Message History**: Retrieve channel messages with pagination support
- **Real-time Streaming**: WebSocket-based live message updates (server-side streaming RPC)
- **Smart Caching**: Database-backed cache with configurable TTL (guilds: 1h, channels: 30m, messages: 5m)
- **Rate Limiting**: Automatic Discord API rate limit handling
- **Token Refresh**: Transparent OAuth token refresh when expired

## Architecture

```
┌─────────────┐     InitAuth()      ┌──────────────┐
│   Client    │ ───────────────────> │   gRPC       │
│             │                      │   Server     │
│             │ <─────────────────── │   :50051     │
└─────────────┘   AuthURL+SessionID  └──────────────┘
       │                                     │
       │                                     │
       │ User opens AuthURL in browser       │
       │                                     │
       v                                     │
┌─────────────┐                              │
│   Discord   │                              │
│   OAuth     │                              │
└─────────────┘                              │
       │                                     │
       │ Redirect with code + state          │
       │                                     │
       v                                     v
┌─────────────┐                      ┌──────────────┐
│   HTTP      │ HandleCallback()     │  PostgreSQL  │
│   Server    │ ──────────────────> │  Database    │
│   :8080     │                      └──────────────┘
└─────────────┘                              │
                                             │
┌─────────────┐   GetAuthStatus()    ┌──────────────┐
│   Client    │ ───────────────────> │   gRPC       │
│             │                      │   Server     │
│             │ <─────────────────── └──────────────┘
└─────────────┘   Status + UserInfo
```

## Prerequisites

- Go 1.23+
- PostgreSQL 13+
- Docker & Docker Compose (optional)
- Discord Application (see setup below)

## Discord Application Setup

Before running the server, you need to create a Discord application:

### 1. Create Discord Application

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application"
3. Enter a name (e.g., "My Discord Lite App")
4. Click "Create"

### 2. Configure OAuth2

1. In your application, go to "OAuth2" → "General"
2. Under "Redirects", add:
   ```
   http://localhost:8080/auth/callback
   ```
   For production, add your production URL

3. Under "Default Authorization Link", select "In-app Authorization"
4. Under "Scopes", select:
   - `identify` - Read user ID, username, avatar
   - `email` - Read user email
   - `guilds` - Read user's guilds (servers)

### 3. Get Credentials

1. Go to "OAuth2" → "General"
2. Copy your **Client ID**
3. Copy your **Client Secret** (click "Reset Secret" if needed)
4. Save these for configuration

### 4. Optional: Add Bot (for Phase 2)

If you plan to add channel/message features later:
1. Go to "Bot" tab
2. Click "Add Bot"
3. Enable required intents (Server Members, Message Content)

## Installation

### Option 1: Docker Compose (Recommended)

1. Clone the repository:
```bash
git clone <repository-url>
cd DiscordLiteServer
```

2. Create `.env` file from example:
```bash
cp .env.example .env
```

3. Edit `.env` with your Discord credentials:
```bash
DISCORD_CLIENT_ID=your_client_id_here
DISCORD_CLIENT_SECRET=your_client_secret_here
TOKEN_ENCRYPTION_KEY=$(openssl rand -hex 32)
```

4. Start services:
```bash
docker-compose up -d
```

The server will be available at:
- HTTP: http://localhost:8080
- gRPC: localhost:50051

### Option 2: Local Development

1. Install dependencies:
```bash
go mod download
```

2. Install protoc tools:
```bash
make install-tools
```

3. Generate protobuf code:
```bash
make proto
```

4. Start PostgreSQL:
```bash
# Using Docker
docker run -d \
  --name postgres \
  -e POSTGRES_USER=discordlite \
  -e POSTGRES_PASSWORD=your_password \
  -e POSTGRES_DB=discordlite_db \
  -p 5432:5432 \
  postgres:15-alpine
```

5. Create `.env` file:
```bash
cp .env.example .env
# Edit with your credentials
```

6. Run the server (migrations run automatically):
```bash
# Migrations will run automatically on server startup
go run cmd/server/main.go
```

Alternatively, using Make:
```bash
make run
```

## Configuration

All configuration is done via environment variables. See `.env.example` for all options.

### Required Variables

```bash
# Discord OAuth
DISCORD_CLIENT_ID=             # From Discord Developer Portal
DISCORD_CLIENT_SECRET=         # From Discord Developer Portal
DISCORD_REDIRECT_URI=          # Must match Discord settings

# Database
DB_PASSWORD=                   # PostgreSQL password

# Security
TOKEN_ENCRYPTION_KEY=          # 32-byte hex key (64 chars)
```

### Generate Encryption Key

```bash
openssl rand -hex 32
```

## API Usage

### gRPC Service

The service exposes three gRPC methods:

#### 1. InitAuth - Start OAuth Flow

```protobuf
rpc InitAuth(InitAuthRequest) returns (InitAuthResponse);
```

**Example (Go):**
```go
resp, err := client.InitAuth(ctx, &authpb.InitAuthRequest{})
// Open resp.AuthUrl in browser
// Store resp.SessionId for polling
```

#### 2. GetAuthStatus - Poll Authentication Status

```protobuf
rpc GetAuthStatus(GetAuthStatusRequest) returns (GetAuthStatusResponse);
```

**Example (Go):**
```go
resp, err := client.GetAuthStatus(ctx, &authpb.GetAuthStatusRequest{
    SessionId: sessionId,
})

switch resp.Status {
case authpb.AuthStatus_AUTH_STATUS_PENDING:
    // Still waiting for user
case authpb.AuthStatus_AUTH_STATUS_AUTHENTICATED:
    // Success! Access resp.User
case authpb.AuthStatus_AUTH_STATUS_FAILED:
    // Authentication failed
}
```

#### 3. RevokeAuth - Revoke Authentication

```protobuf
rpc RevokeAuth(RevokeAuthRequest) returns (RevokeAuthResponse);
```

**Example (Go):**
```go
resp, err := client.RevokeAuth(ctx, &authpb.RevokeAuthRequest{
    SessionId: sessionId,
})
```

### Phase 2: Channel and Message Services

After authentication, you can access Discord guilds, channels, and messages.

#### 4. GetGuilds - Fetch User's Discord Servers

```protobuf
rpc GetGuilds(GetGuildsRequest) returns (GetGuildsResponse);
```

**Example (Go):**
```go
resp, err := channelClient.GetGuilds(ctx, &channelpb.GetGuildsRequest{
    SessionId:    sessionId,
    ForceRefresh: false,  // Set true to bypass cache
})

for _, guild := range resp.Guilds {
    fmt.Printf("Guild: %s (ID: %s)\n", guild.Name, guild.DiscordGuildId)
}
```

#### 5. GetChannels - Fetch Channels for a Guild

```protobuf
rpc GetChannels(GetChannelsRequest) returns (GetChannelsResponse);
```

**Example (Go):**
```go
resp, err := channelClient.GetChannels(ctx, &channelpb.GetChannelsRequest{
    SessionId:    sessionId,
    GuildId:      guildId,      // Discord guild ID
    ForceRefresh: false,
})

for _, channel := range resp.Channels {
    fmt.Printf("Channel: #%s (Type: %v)\n", channel.Name, channel.Type)
}
```

#### 6. GetMessages - Fetch Messages from a Channel

```protobuf
rpc GetMessages(GetMessagesRequest) returns (GetMessagesResponse);
```

**Example (Go):**
```go
resp, err := messageClient.GetMessages(ctx, &messagepb.GetMessagesRequest{
    SessionId:    sessionId,
    ChannelId:    channelId,    // Discord channel ID
    Limit:        50,            // Max 100, default 50
    Before:       "",            // Message ID for pagination
    ForceRefresh: false,
})

for _, msg := range resp.Messages {
    fmt.Printf("[%s] %s: %s\n",
        time.UnixMilli(msg.Timestamp).Format("15:04"),
        msg.Author.Username,
        msg.Content,
    )
}

// Pagination: fetch older messages
if resp.HasMore {
    oldestMsgId := resp.Messages[len(resp.Messages)-1].DiscordMessageId
    nextResp, err := messageClient.GetMessages(ctx, &messagepb.GetMessagesRequest{
        SessionId: sessionId,
        ChannelId: channelId,
        Limit:     50,
        Before:    oldestMsgId,  // Get messages before this ID
    })
}
```

#### 7. StreamMessages - Real-time Message Updates (Server-side streaming)

```protobuf
rpc StreamMessages(StreamMessagesRequest) returns (stream MessageEvent);
```

**Example (Go):**
```go
stream, err := messageClient.StreamMessages(ctx, &messagepb.StreamMessagesRequest{
    SessionId:  sessionId,
    ChannelIds: []string{channelId1, channelId2},  // Subscribe to multiple channels
})

for {
    event, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }

    switch event.EventType {
    case messagepb.MessageEventType_MESSAGE_EVENT_TYPE_CREATE:
        fmt.Printf("New message: %s\n", event.Message.Content)
    case messagepb.MessageEventType_MESSAGE_EVENT_TYPE_UPDATE:
        fmt.Printf("Message edited: %s\n", event.Message.Content)
    case messagepb.MessageEventType_MESSAGE_EVENT_TYPE_DELETE:
        fmt.Printf("Message deleted: %s\n", event.Message.DiscordMessageId)
    }
}
```

### Swift Client (iOS/macOS)

A Swift Package Manager package is available for iOS and macOS applications at the repository root:

**Installation:**

Add to your Xcode project or `Package.swift`:
```swift
dependencies: [
    .package(url: "https://github.com/parsascontentcorner/discordliteserver", from: "1.0.0")
]
```

Or for local development:
```swift
dependencies: [
    .package(path: "../DiscordLiteServer")
]
```

**Usage:**

```swift
import DiscordLiteAPI
import Connect

// Create client
let client = ProtocolClient(
    httpClient: URLSessionHTTPClient(),
    config: ProtocolClientConfig(
        host: "http://localhost:50051",
        networkProtocol: .connect
    )
)
let authService = Discord_Auth_V1_AuthServiceClient(client: client)

// Initiate auth
let response = try await authService.initAuth(request: Discord_Auth_V1_InitAuthRequest())
UIApplication.shared.open(URL(string: response.authURL)!)

// Poll status
var statusRequest = Discord_Auth_V1_GetAuthStatusRequest()
statusRequest.sessionID = response.sessionID
let status = try await authService.getAuthStatus(request: statusRequest)
```

### Testing with grpcurl

```bash
# List services
grpcurl -plaintext localhost:50051 list

# Phase 1: Authentication
grpcurl -plaintext -d '{}' \
  localhost:50051 discord.auth.AuthService/InitAuth

grpcurl -plaintext -d '{"session_id":"your-session-id"}' \
  localhost:50051 discord.auth.AuthService/GetAuthStatus

# Phase 2: Guilds & Channels
grpcurl -plaintext -d '{"session_id":"your-session-id"}' \
  localhost:50051 discord.channel.v1.ChannelService/GetGuilds

grpcurl -plaintext -d '{"session_id":"your-session-id","guild_id":"123456789"}' \
  localhost:50051 discord.channel.v1.ChannelService/GetChannels

# Phase 2: Messages
grpcurl -plaintext -d '{"session_id":"your-session-id","channel_id":"123456789","limit":50}' \
  localhost:50051 discord.message.v1.MessageService/GetMessages

# Phase 2: Real-time Streaming
grpcurl -plaintext -d '{"session_id":"your-session-id","channel_ids":["123456789"]}' \
  localhost:50051 discord.message.v1.MessageService/StreamMessages
```

## Development

### Project Structure

```
DiscordLiteServer/
├── cmd/server/           # Application entry point
├── internal/
│   ├── auth/            # OAuth logic (Discord, state, handler)
│   ├── config/          # Configuration management
│   ├── database/        # Database connection & queries
│   ├── grpc/            # gRPC server & service
│   ├── http/            # HTTP server & handlers
│   └── models/          # Data models
├── api/
│   ├── proto/           # Protobuf definitions (versioned)
│   │   ├── buf.yaml     # Buf module config
│   │   ├── buf.gen.yaml # Code generation config
│   │   └── discord/auth/v1/  # v1 API definitions
│   └── gen/             # Generated code (committed to git)
│       ├── go/          # Go gRPC code
│       └── swift/       # Swift Connect code (SPM sources)
├── pkg/logger/          # Logging utilities
├── scripts/             # Helper scripts
└── Package.swift        # Swift Package Manager manifest
```

### Make Commands

```bash
make help           # Show all commands
make proto          # Generate Go and Swift protobuf code
make proto-go       # Generate only Go code
make proto-swift    # Generate only Swift code
make proto-check    # Validate protobuf definitions (lint + breaking)
make proto-clean    # Remove generated code
make build          # Build binary
make run            # Run locally
make test           # Run tests
make docker-build   # Build Docker image
make docker-up      # Start with docker-compose
make docker-down    # Stop docker-compose
```

### Adding New Features

For Phase 2 (channels/messages):

1. Add new methods to `api/proto/auth.proto`
2. Regenerate proto: `make proto`
3. Implement in `internal/grpc/auth_service.go`
4. Add Discord API calls in `internal/auth/discord.go`

## Security

### Token Encryption

- OAuth tokens are encrypted at rest using AES-256-GCM
- Encryption key must be 32 bytes (64 hex characters)
- Never commit encryption keys to version control

### CSRF Protection

- OAuth state tokens are cryptographically secure (32 bytes)
- States are single-use and expire after 10 minutes
- Stored in database for validation

### Session Management

- Sessions expire after 24 hours (configurable)
- Background job cleans expired sessions every 30 minutes
- Session IDs are UUIDs for unpredictability

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Logs

The server uses structured logging (zap). Configure via environment:

```bash
LOG_LEVEL=debug     # debug, info, warn, error
LOG_FORMAT=console  # console or json
```

### Database Health

```bash
# Check database connection
docker exec discordlite_postgres pg_isready
```

## Troubleshooting

### "Failed to connect to database"

- Check PostgreSQL is running: `docker ps`
- Verify credentials in `.env`
- Test connection: `psql -h localhost -U discordlite -d discordlite_db`

### "Invalid state" error

- State tokens expire after 10 minutes
- Ensure system clocks are synchronized
- Clear expired states: check database cleanup job

### "Discord returned an error"

- Verify Discord credentials in `.env`
- Check redirect URI matches Discord settings exactly
- Ensure scopes are enabled in Discord application

### gRPC Connection Failed

- Check gRPC port is exposed: `netstat -an | grep 50051`
- Verify firewall allows port 50051
- Use `-plaintext` flag for non-TLS connections

## Production Deployment

### Environment Variables

Update for production:

```bash
ENVIRONMENT=production
SERVER_HOST=0.0.0.0
DISCORD_REDIRECT_URI=https://your-domain.com/auth/callback
DB_SSLMODE=require
LOG_LEVEL=info
LOG_FORMAT=json
```

### TLS/HTTPS

Add TLS certificates:

1. For HTTP server:
   - Update `internal/http/server.go` to use `ListenAndServeTLS`
   - Mount certificates in Docker

2. For gRPC server:
   - Add TLS credentials to gRPC server options
   - Update client connections

### Scaling

- Database: Use connection pooling (already configured)
- App: Run multiple instances behind load balancer
- Sessions: All state is in PostgreSQL (stateless app servers)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[Your License Here]

## Support

For issues and questions:
- GitHub Issues: [Link]
- Discord: [Link]
- Email: [Email]

## Roadmap

### Phase 1 (Complete)
- ✅ Discord OAuth authentication
- ✅ gRPC API (InitAuth, GetAuthStatus, RevokeAuth)
- ✅ Token encryption and storage
- ✅ Session management
- ✅ Docker support

### Phase 2 (Planned)
- ✅ Get user's Discord guilds/servers
- ✅ Get guild channels
- ✅ Get channel messages
- ✅ Real-time updates (WebSocket)
- [ ] Redis caching layer (We will see if it ends up being needed)
- ✅ Rate limiting

### Future
- [ ] Message sending
- [ ] File uploads
- [ ] Voice channel support
- [ ] Admin dashboard
- [ ] Metrics & monitoring (Prometheus)
