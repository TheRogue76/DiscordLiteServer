# Claude Project Documentation

## Project Overview

**Project Name**: Discord Lite Server
**Purpose**: Golang backend service for Discord OAuth authentication with gRPC API
**Phase**: Phase 1 Complete (Authentication + Swift Client) | Phase 2: 85% Complete (Guilds/Channels/Messages)
**Status**: 321 tests passing, 50%+ coverage, StreamMessages pending implementation
**Last Updated**: 2025-12-30

## Architecture Summary

### Core Components

1. **gRPC Server** (Port 50051)
   - **AuthService** - 3 RPC methods (InitAuth, GetAuthStatus, RevokeAuth)
   - **ChannelService** - 2 RPC methods (GetGuilds, GetChannels)
   - **MessageService** - 2 RPC methods (GetMessages, StreamMessages*)
   - Reflection enabled for development
   - *StreamMessages pending WebSocket integration

2. **HTTP Server** (Port 8080)
   - OAuth callback endpoint (`/auth/callback`)
   - Health check endpoint (`/health`)
   - Serves HTML success/error pages

3. **PostgreSQL Database** (Port 5432)
   - **Phase 1 tables (4)**: users, oauth_tokens, oauth_states, auth_sessions
   - **Phase 2 tables (7)**: guilds, user_guilds, channels, messages, message_attachments, cache_metadata, websocket_sessions
   - Automatic cleanup of expired sessions and cache (30-minute intervals)
   - Connection pooling configured

4. **Discord OAuth Integration**
   - Full OAuth 2.0 flow with automatic token refresh
   - AES-256-GCM token encryption
   - CSRF protection via state tokens
   - Discord API client (guilds, channels, messages)
   - Rate limiting (respects Discord API headers)

5. **Caching Layer**
   - Database-backed caching (no Redis)
   - TTL strategy: Guilds (1hr), Channels (30min), Messages (5min)
   - Global and user-specific cache support
   - Automatic cache invalidation on events

6. **WebSocket Manager** ⚠️
   - Discord Gateway connection handling
   - Event processing (MESSAGE_CREATE, UPDATE, DELETE)
   - Session management and heartbeat
   - *Not yet wired to StreamMessages RPC

### Authentication Flow

```
1. Client → gRPC: InitAuth()
   └─> Server generates session_id, state, Discord OAuth URL

2. Client opens OAuth URL in browser
   └─> User authenticates on Discord

3. Discord redirects to HTTP callback
   └─> Server validates state, exchanges code for token
   └─> Server encrypts and stores tokens in PostgreSQL
   └─> Server updates session status to "authenticated"

4. Client → gRPC: GetAuthStatus() (polling)
   └─> Server returns status + user info if authenticated
```

## Implementation Status

### ✅ Completed (100%)

#### Phase 1: Project Initialization
- [x] Go module initialized (`go.mod`, `go.sum`)
- [x] Directory structure created (cmd, internal, api, pkg, scripts)
- [x] `.env.example` with all configuration options
- [x] Makefile with targets: proto, build, run, test, docker-*

#### Phase 2: Configuration & Logging
- [x] `internal/config/config.go` - Environment-based configuration
- [x] `pkg/logger/logger.go` - Structured logging (zap)
- [x] Configuration validation
- [x] Multiple log formats (json, console)

#### Phase 3: Database Layer
- [x] `internal/database/migrations/001_initial.sql` - Complete schema
  - Users table with Discord info
  - OAuth tokens table (encrypted)
  - OAuth states table (CSRF protection)
  - Auth sessions table (session tracking)
  - Automatic updated_at triggers
- [x] `internal/database/db.go` - Connection pooling
- [x] `internal/models/auth.go` - All data models
- [x] `internal/database/queries.go` - 15+ database operations
  - CreateUser, GetUserByDiscordID, GetUserByID
  - StoreOAuthToken, GetOAuthToken, DeleteOAuthToken
  - CreateAuthSession, GetAuthSession, UpdateAuthSessionStatus, DeleteAuthSession
  - CreateOAuthState, ValidateAndDeleteOAuthState
  - CleanupExpiredSessions, StartCleanupJob

#### Phase 4: gRPC Service
- [x] `api/proto/auth.proto` - Service definition
  - InitAuth RPC
  - GetAuthStatus RPC
  - RevokeAuth RPC
  - UserInfo, AuthStatus enums
- [x] Generated protobuf code (`auth.pb.go`, `auth_grpc.pb.go`)
- [x] `internal/grpc/auth_service.go` - Service implementation
- [x] `internal/grpc/server.go` - Server setup with interceptors

#### Phase 5: OAuth Implementation
- [x] `internal/auth/state_manager.go`
  - Cryptographically secure state generation (32 bytes)
  - Database-backed state validation
  - Single-use state tokens
- [x] `internal/auth/discord.go`
  - OAuth2 client configuration
  - Code-to-token exchange
  - User info fetching from Discord API
  - AES-256-GCM encryption/decryption
- [x] `internal/auth/oauth_handler.go`
  - Complete callback flow orchestration
  - Error handling and session status updates

#### Phase 6: HTTP Server
- [x] `internal/http/handlers.go`
  - HealthHandler
  - CallbackHandler with HTML responses
  - Success/error page rendering
- [x] `internal/http/server.go`
  - Server setup with timeouts
  - Logging middleware
  - Graceful shutdown

#### Phase 7: Main Application
- [x] `cmd/server/main.go`
  - Concurrent server startup (HTTP + gRPC)
  - Signal handling (SIGINT, SIGTERM)
  - Graceful shutdown
  - Migration runner
  - Cleanup job starter

#### Phase 8: Docker Support
- [x] `Dockerfile` - Multi-stage build (builder + alpine runtime)
- [x] `docker-compose.yml` - PostgreSQL + App services
- [x] `.dockerignore` - Optimized builds

#### Phase 9: Documentation & Tools
- [x] `README.md` - Comprehensive documentation
  - Discord app setup guide
  - Installation instructions (Docker + Local)
  - API usage examples
  - Troubleshooting guide
  - Production deployment tips
  - Phase 2 roadmap

#### Phase 10: Build & Dependencies
- [x] All Go dependencies installed
- [x] Compatible gRPC version (v1.65.0 for Go 1.23.1)
- [x] Protobuf plugins installed
- [x] Binary successfully compiled (17MB)
- [x] No compilation errors

#### Phase 11: Multi-Language API Support (Swift Client)
- [x] **Buf CLI Integration** - Modern protobuf tooling
  - Replaced raw `protoc` commands with Buf
  - Created `api/proto/buf.yaml` (module config, linting)
  - Created `api/proto/buf.gen.yaml` (code generation config)
  - Added Makefile targets: proto-go, proto-swift, proto-check, proto-clean
- [x] **API Versioning (v1)** - Future-proof API structure
  - Migrated proto files to `api/proto/discord/auth/v1/`
  - Package changed from `discord.auth` to `discord.auth.v1`
  - Go import path: `github.com/parsascontentcorner/discordliteserver/api/gen/go/discord/auth/v1`
- [x] **Generated Code Management** - Committed to git
  - Go code: `api/gen/go/discord/auth/v1/` (auth.pb.go, auth_grpc.pb.go)
  - Swift code: `api/gen/swift/discord/auth/v1/` (auth.pb.swift, auth.connect.swift)
  - Separated generated code from source proto files
- [x] **Swift Package Manager Integration** - iOS/macOS client
  - Created `Package.swift` at repository root (remote-installable)
  - Dependencies: swift-protobuf (1.27.0+), connect-swift (1.0.0+)
  - Platforms: iOS 15+, macOS 12+
  - Target path points directly to `api/gen/swift` (no symlink needed)
  - Successfully builds with Swift Package Manager
  - Remote installation: `https://github.com/parsascontentcorner/discordliteserver`
- [x] **Go Import Path Updates** - Migrated to new structure
  - Updated `internal/grpc/auth_service.go` (authpb → authv1)
  - Updated `internal/grpc/server.go` (authpb → authv1)
  - Updated `internal/grpc/auth_service_test.go` (authpb → authv1)
  - All 115+ tests passing after migration
- [x] **Documentation** - Complete usage guides
  - Created `ios-client/README.md` - Full Swift client documentation
  - Updated `README.md` - Added Swift client usage examples
  - Updated Makefile help text with new proto targets
  - Updated project structure diagrams

**Key Technologies:**
- **Buf** (v1.60.0) - Modern protobuf build tool
- **Connect-Swift** (1.0.0+) - Modern HTTP/1.1 & HTTP/2 RPC framework
- **Swift Protobuf** (1.27.0+) - Swift protocol buffers runtime

**Breaking Changes:**
- Go import path changed from `api/proto` to `api/gen/go/discord/auth/v1`
- Package alias changed from `authpb` to `authv1`
- Proto package changed from `discord.auth` to `discord.auth.v1`

**Benefits:**
- Multi-language support (Go + Swift, extensible to TypeScript, Python, etc.)
- API versioning for backward compatibility
- Modern tooling with Buf (linting, breaking change detection)
- Type-safe Swift client with async/await
- Generated code committed to git (reproducible builds)

#### Phase 12: Comprehensive Test Suite
- [x] **Database Query Tests** (85 tests)
  - Phase 1 tests: User, OAuth token, auth session CRUD (27 tests)
  - Guild tests: CRUD, user-guild membership (23 tests)
  - Channel tests: CRUD, guild relationships, access validation (17 tests)
  - Message tests: CRUD, pagination, attachments, cascade deletion (18 tests)
  - Cache tests: TTL management, invalidation, isolation (19 tests)
- [x] **gRPC Service Tests** (23 tests)
  - ChannelService: GetGuilds, GetChannels with cache testing (12 tests)
  - MessageService: GetMessages with pagination and attachments (11 tests)
  - Mock Discord API integration using httptest
  - Cache hit/miss scenarios validated
- [x] **Configuration Tests** (18 tests)
  - Environment variable loading and validation
  - Default value handling
  - Error scenarios
- [x] **Rate Limiting Tests** (10 tests)
  - Bucket-based rate limiting
  - Discord API header parsing
  - Concurrent access safety
- [x] **Test Infrastructure**
  - testcontainers-go for PostgreSQL 15-alpine
  - Table-driven test patterns
  - Mock HTTP servers for Discord API
  - Test database setup helpers

**Current Test Suite (2025-12-30):**
- **321 passing tests** (Phase 1 + Phase 2)
- **50%+ overall code coverage**
- **Zero flaky tests** - Fixed rate limiter timing issues
- All model tests added (100% model coverage)
- Phase 1 integration tests passing (8 tests)

**Coverage by Package:**
- models: 100.0% (excellent) - 82 tests
- config: 97.2% (excellent) - 18 tests
- logger: 94.7% (excellent)
- integration: 80.6% (good) - 8 tests
- grpc: 69.2% (good) - 47 tests (Phase 1 + Phase 2)
- database: 64.1% (good) - 104 tests (Phase 1 + Phase 2)
- ratelimit: 56.9% (moderate) - 10 tests
- oauth: 50.0% (moderate)
- auth: 43.9% (moderate)
- websocket: 0.0% (not yet tested)

### ✅ Phase 2: Guilds, Channels, Messages (85% Complete)

#### Phase 2A: Database Schema & Models ✅ COMPLETE
- [x] Migration `000002_add_phase2_tables.up.sql` (167 lines, 7 tables)
- [x] Models: `guild.go`, `channel.go`, `message.go`, `cache.go`, `websocket.go`
- [x] Query files: `guild_queries.go`, `channel_queries.go`, `message_queries.go`, `cache_queries.go`, `websocket_queries.go`
- [x] **100% model test coverage** (82 tests covering all Discord types and edge cases)

#### Phase 2B: Discord API Client Extensions ✅ COMPLETE
- [x] `RefreshToken()` and `RefreshIfNeeded()` - Automatic token refresh
- [x] `GetUserGuilds()` - Fetch user's Discord servers
- [x] `GetGuildChannels()` - Fetch guild channels
- [x] `GetChannelMessages()` - Fetch messages with pagination
- [x] Rate limiter with Discord API header parsing
- [x] **10 rate limiter tests passing** (fixed flaky timing test)

#### Phase 2C: Protobuf Definitions ✅ COMPLETE
- [x] `api/proto/discord/channel/v1/channel.proto` - Guild and Channel services
- [x] `api/proto/discord/message/v1/message.proto` - Message service with StreamMessages
- [x] Generated Go code with Buf CLI
- [x] Swift client generation (multi-language support)

#### Phase 2D: gRPC Services ✅ COMPLETE
- [x] `ChannelServer` - GetGuilds, GetChannels RPCs
- [x] `MessageServer` - GetMessages RPC
- [x] `CacheManager` - Cache hit/miss logic
- [x] All services registered in `server.go`
- [x] **24 gRPC service tests passing**
  - Cache hit/miss scenarios
  - Pagination (before/after cursors)
  - Access control validation
  - Error handling

#### Phase 2E: WebSocket Implementation ⚠️ PARTIAL (70% Complete)
- [x] `internal/websocket/manager.go` - Connection manager (9.7KB)
- [x] `internal/websocket/gateway.go` - Gateway WebSocket logic (10.8KB)
- [x] `internal/websocket/events.go` - Event handlers (10.6KB)
- [x] WebSocket manager integrated in `main.go`
- [x] Cleanup jobs configured (session and cache)
- [ ] **StreamMessages RPC returns `Unimplemented`** ⚠️
- [ ] WebSocket events not wired to gRPC streaming

**What's Left:**
- Wire WebSocket manager to StreamMessages RPC (~3 hours)
- Subscribe to channels, forward MESSAGE_CREATE/UPDATE/DELETE events
- Handle context cancellation and cleanup

#### Phase 2F: Configuration & Integration ⚠️ PARTIAL (60% Complete)
- [x] `CacheConfig` and `WebSocketConfig` added
- [x] Environment variables in `.env.example`
- [x] Services initialized in `main.go`
- [x] Cache and WebSocket cleanup jobs running
- [ ] **No Phase 2 integration tests** ❌
- [ ] **Documentation not updated** (README, API examples) ❌

**What's Left:**
- Create `internal/integration/phase2_flow_test.go` (~4 hours)
- Update README.md with Phase 2 API examples (~2 hours)
- Manual testing with real Discord API (~6 hours)

#### 📊 Phase 2 Summary

| Component | Status | Tests | Coverage |
|-----------|--------|-------|----------|
| Database & Models | ✅ Complete | 104 tests | 64.1% |
| Discord API Client | ✅ Complete | 10 tests | 43.9% |
| Protobuf Definitions | ✅ Complete | N/A | N/A |
| gRPC Get* Services | ✅ Complete | 24 tests | 69.2% |
| Cache Manager | ✅ Complete | 19 tests | 100% |
| WebSocket Infrastructure | ⚠️ 70% | 0 tests | 0% |
| **StreamMessages RPC** | ❌ **Stub only** | **0 tests** | **0%** |
| **Phase 2 Integration** | ❌ **Missing** | **0 tests** | **0%** |
| **Documentation** | ❌ **Not updated** | **N/A** | **N/A** |

**Overall Phase 2 Progress: 85% Complete**

**Remaining Work (Estimated 16-20 hours):**
1. Implement StreamMessages RPC (~3 hours)
2. Create Phase 2 integration test (~4 hours)
3. Manual testing with real Discord API (~6 hours)
4. Update documentation (README, examples) (~3 hours)

### 🚧 Known Issues & TODOs

#### Pre-Testing Setup Required

1. **Discord Application**
   - Must create app at https://discord.com/developers/applications
   - Configure OAuth2 redirect URI: `http://localhost:8080/auth/callback`
   - Enable scopes: identify, email, guilds
   - Copy Client ID and Secret to `.env`

2. **Environment Configuration**
   - Create `.env` from `.env.example`
   - Set `DISCORD_CLIENT_ID` and `DISCORD_CLIENT_SECRET`
   - Generate encryption key: `openssl rand -hex 32`
   - Set `TOKEN_ENCRYPTION_KEY`

3. **Database Setup**
   - PostgreSQL must be running (Docker or local)
   - Migrations run automatically when server starts (using golang-migrate)
   - Verify tables created: `psql -U discordlite -d discordlite_db -c '\dt'`

#### Potential Issues to Watch

1. **OAuth Redirect URI**
   - Must match exactly between Discord settings and `.env`
   - Case-sensitive
   - Trailing slash matters

2. **Token Encryption Key**
   - Must be exactly 32 bytes (64 hex characters)
   - Should be randomly generated
   - Never commit to version control

3. **Session Expiry**
   - Default: 24 hours
   - May need adjustment for testing
   - Cleanup job runs every 30 minutes

4. **CORS/Network**
   - HTTP server needs to be accessible from browser
   - gRPC server needs to be accessible from client
   - Docker networking may require host network mode

## Testing Plan

### Unit Testing

**Priority: High**

```bash
# Test files to create:
internal/auth/state_manager_test.go
internal/auth/discord_test.go (encryption/decryption)
internal/config/config_test.go
internal/database/queries_test.go (use testcontainers)
```

**Focus Areas:**
- State generation randomness
- Token encryption/decryption roundtrip
- Configuration validation
- Database query edge cases

### Integration Testing

**Priority: High**

**Test Scenarios:**

1. **Happy Path - Full OAuth Flow**
   ```
   1. Call InitAuth → Get session_id and auth_url
   2. Open auth_url in browser
   3. Authenticate with Discord
   4. Verify callback success
   5. Poll GetAuthStatus → Verify authenticated status
   6. Verify user info returned
   ```

2. **State Validation**
   ```
   1. Initiate auth flow
   2. Tamper with state parameter
   3. Verify callback rejects invalid state
   4. Verify session marked as failed
   ```

3. **Session Expiry**
   ```
   1. Create session with short expiry (1 minute)
   2. Wait for expiry
   3. Call GetAuthStatus
   4. Verify session expired error
   ```

4. **Token Encryption**
   ```
   1. Complete auth flow
   2. Query database directly
   3. Verify tokens are encrypted (not plaintext)
   4. Decrypt and verify structure
   ```

5. **Concurrent Sessions**
   ```
   1. Initiate multiple auth flows
   2. Complete them in different orders
   3. Verify no cross-session contamination
   ```

### Manual Testing

**Priority: Medium**

**Tools:**
- grpcurl (for gRPC testing)
- Postman/curl (for HTTP testing)
- psql (for database inspection)

**Test Script:**
```bash
# 1. Start services
docker-compose up -d

# 2. Test health check
curl http://localhost:8080/health

# 3. Test gRPC InitAuth
grpcurl -plaintext -d '{}' \
  localhost:50051 discord.auth.AuthService/InitAuth

# 4. Open returned auth_url in browser
# (Manual step)

# 5. Poll GetAuthStatus
grpcurl -plaintext -d '{"session_id":"<your-session-id>"}' \
  localhost:50051 discord.auth.AuthService/GetAuthStatus

# 6. Verify database state
docker exec -it discordlite_postgres \
  psql -U discordlite -d discordlite_db -c \
  "SELECT session_id, auth_status FROM auth_sessions;"
```

### Load Testing

**Priority: Low** (for Phase 2)

- Concurrent auth flows
- Database connection pool behavior
- Session cleanup under load
- gRPC server performance

## File Structure

```
DiscordLiteServer/
├── cmd/
│   └── server/
│       └── main.go                   # Entry point (113 lines)
├── internal/
│   ├── auth/
│   │   ├── discord.go                # OAuth client (166 lines)
│   │   ├── oauth_handler.go          # Flow orchestration (132 lines)
│   │   └── state_manager.go          # State generation (66 lines)
│   ├── config/
│   │   └── config.go                 # Configuration (176 lines)
│   ├── database/
│   │   ├── db.go                     # Connection (81 lines)
│   │   ├── queries.go                # CRUD operations (349 lines)
│   │   └── migrations/
│   │       └── 001_initial.sql       # Schema (73 lines)
│   ├── grpc/
│   │   ├── auth_service.go           # gRPC implementation (168 lines)
│   │   └── server.go                 # Server setup (86 lines)
│   ├── http/
│   │   ├── handlers.go               # HTTP handlers (189 lines)
│   │   └── server.go                 # Server setup (89 lines)
│   └── models/
│       └── auth.go                   # Data models (69 lines)
├── api/
│   └── proto/
│       ├── auth.proto                # gRPC definition (69 lines)
│       ├── auth.pb.go                # Generated (auto)
│       └── auth_grpc.pb.go           # Generated (auto)
├── pkg/
│   └── logger/
│       └── logger.go                 # Logging (51 lines)
├── bin/
│   └── server                        # Compiled binary (17MB)
├── docker-compose.yml                # Docker orchestration
├── Dockerfile                        # Multi-stage build
├── .dockerignore                     # Build optimization
├── .env.example                      # Configuration template
├── Makefile                          # Build automation
├── README.md                         # User documentation
├── claude.md                         # This file
├── go.mod                            # Go dependencies
└── go.sum                            # Dependency checksums

Total Lines of Code (excluding generated): ~1,917 lines
Generated Code: ~500 lines (protobuf)
Documentation: ~450 lines (README + claude.md)
```

## Key Dependencies

```go
// Core
google.golang.org/grpc v1.65.0          // gRPC framework
google.golang.org/protobuf v1.34.1      // Protocol buffers

// Database
github.com/lib/pq v1.10.9               // PostgreSQL driver

// OAuth
golang.org/x/oauth2 v0.20.0             // OAuth2 client

// Utilities
github.com/joho/godotenv v1.5.1         // .env loading
github.com/google/uuid v1.6.0           // UUID generation
go.uber.org/zap v1.27.1                 // Structured logging
golang.org/x/crypto                     // AES encryption

// Testing
github.com/stretchr/testify v1.9.0                          // Testing assertions
github.com/testcontainers/testcontainers-go v0.27.0         // Integration testing
```

## Testing Requirements

### Mandatory Testing Policy

**ALL new features and bug fixes MUST include tests before being considered complete.**

#### Requirements:
1. **Minimum Coverage:** 80% for new code
2. **Test Types Required:**
   - Unit tests for all new functions
   - Integration tests for database operations
   - End-to-end tests for new gRPC/HTTP endpoints

#### Test File Conventions:
- Test files named `*_test.go` alongside source files
- Use `testify/assert` for assertions
- Use `testify/require` for setup failures
- Use TestContainers for database integration tests

#### Running Tests:
```bash
# All tests
make test

# Unit tests only (fast)
make test-unit

# Integration tests only
make test-integration

# Coverage report
make test-coverage
make test-coverage-html  # Open coverage.html in browser
```

#### Test Structure:
```go
func TestFeatureName(t *testing.T) {
    // Arrange: Setup test data
    // Act: Execute the code under test
    // Assert: Verify results
}
```

#### Table-Driven Tests:
Use table-driven tests for multiple scenarios:
```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    bool
        wantErr error
    }{
        // test cases...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic...
        })
    }
}
```

#### Integration Test Guidelines:
- Use `testutil.SetupTestDB()` for database tests
- Always call cleanup function via `defer cleanup()`
- Truncate tables between tests or use transactions
- Mock external APIs (Discord) with `testutil.MockDiscordServer()`

#### Pre-Commit Checklist:
- [ ] Tests written for all new/modified functions
- [ ] All tests pass: `make test`
- [ ] Coverage ≥ 80%: `make test-coverage`
- [ ] No race conditions: Tests run with `-race` flag
- [ ] Integration tests use TestContainers, not local DB

#### Exemptions:
- Generated code (`*.pb.go`)
- `main.go` (test via integration tests)
- Simple getters/setters (if truly trivial)

**Failure to include tests will result in PR rejection.**

## Environment Variables Reference

### Required
```bash
DISCORD_CLIENT_ID              # Discord app client ID
DISCORD_CLIENT_SECRET          # Discord app client secret
DISCORD_REDIRECT_URI           # OAuth callback URL
DB_PASSWORD                    # PostgreSQL password
TOKEN_ENCRYPTION_KEY           # 32-byte hex key (64 chars)
```

### Optional (with defaults)
```bash
HTTP_PORT=8080                 # HTTP server port
GRPC_PORT=50051                # gRPC server port
DB_HOST=localhost              # Database host
DB_PORT=5432                   # Database port
DB_USER=discordlite            # Database user
DB_NAME=discordlite_db         # Database name
SESSION_EXPIRY_HOURS=24        # Session TTL
STATE_EXPIRY_MINUTES=10        # OAuth state TTL
LOG_LEVEL=info                 # debug|info|warn|error
LOG_FORMAT=json                # json|console
```

## Database Schema

### Tables

**users** (Discord user information)
- id, discord_id (unique), username, discriminator, avatar, email
- created_at, updated_at

**oauth_tokens** (Encrypted OAuth credentials)
- id, user_id (unique FK), access_token, refresh_token, token_type
- expiry, scope, created_at, updated_at

**oauth_states** (CSRF protection)
- state (PK), session_id, created_at, expires_at

**auth_sessions** (Session tracking)
- session_id (PK), user_id (nullable FK), auth_status
- error_message, created_at, updated_at, expires_at

### Indexes
- users.discord_id (unique)
- oauth_tokens.user_id, oauth_tokens.expiry
- oauth_states.expires_at, oauth_states.session_id
- auth_sessions.user_id, auth_sessions.status, auth_sessions.expires_at

## Security Considerations

### Implemented
- ✅ AES-256-GCM encryption for OAuth tokens
- ✅ Cryptographically secure state tokens (32 bytes)
- ✅ State validation and single-use enforcement
- ✅ Session expiry (configurable, default 24h)
- ✅ Automatic cleanup of expired sessions/states
- ✅ Prepared statements (Go's sql package default)
- ✅ Non-root Docker user
- ✅ Environment-based secrets (no hardcoding)

### Future Enhancements
- [ ] TLS/HTTPS for production
- [ ] Rate limiting
- [ ] PKCE (Proof Key for Code Exchange)
- [ ] Token refresh logic
- [ ] Audit logging
- [ ] IP-based session validation
- [ ] Secrets management (Vault, AWS Secrets Manager)

## Phase 2: Implemented Features (85% Complete)

### ✅ Implemented RPCs
1. **GetGuilds RPC** ✅
   - Fetches user's Discord servers
   - Database caching with 1-hour TTL
   - Automatic token refresh
   - Force refresh option

2. **GetChannels RPC** ✅
   - Fetches guild channels
   - Database caching with 30-minute TTL
   - Guild access validation
   - Force refresh option

3. **GetMessages RPC** ✅
   - Fetches channel messages with attachments
   - Pagination (before/after cursors, limit)
   - Database caching with 5-minute TTL
   - Channel access validation
   - Force refresh option

4. **StreamMessages RPC** ⚠️ **Stub only**
   - Server-side streaming defined in proto
   - Returns `Unimplemented` error
   - WebSocket infrastructure exists but not wired

### ✅ Implemented Infrastructure
- **Database caching** (PostgreSQL, not Redis) ✅
- **Automatic OAuth token refresh** ✅
- **Rate limiting** (Discord API header parsing) ✅
- **WebSocket manager** (not yet connected to StreamMessages) ⚠️
- **Cache invalidation** (TTL-based and manual) ✅

### 🚧 Remaining Work
- [ ] Wire WebSocket to StreamMessages RPC
- [ ] Phase 2 integration tests
- [ ] Manual testing with real Discord API
- [ ] Update README with Phase 2 examples
- [ ] Load testing and optimization

## Common Commands

### Development
```bash
# Generate protobuf code
make proto

# Build binary
make build

# Run locally
make run

# Run tests (when written)
make test

# Format code
make fmt
```

### Docker
```bash
# Build image
make docker-build

# Start services
make docker-up

# Stop services
make docker-down

# View logs
docker-compose logs -f app
```

### Database
```bash
# Migrations run automatically on server startup
# To check migration status:
docker exec discordlite_postgres psql -U discordlite -d discordlite_db -c "SELECT * FROM schema_migrations;"

# Connect to database
docker exec -it discordlite_postgres psql -U discordlite -d discordlite_db

# View tables
\dt

# View sessions
SELECT * FROM auth_sessions;
```

### Testing gRPC
```bash
# List services
grpcurl -plaintext localhost:50051 list

# Describe service
grpcurl -plaintext localhost:50051 describe discord.auth.AuthService

# Call InitAuth
grpcurl -plaintext -d '{}' localhost:50051 discord.auth.AuthService/InitAuth

# Call GetAuthStatus
grpcurl -plaintext -d '{"session_id":"xxx"}' localhost:50051 discord.auth.AuthService/GetAuthStatus
```

## Troubleshooting

### "failed to connect to database"
- Verify PostgreSQL is running: `docker ps | grep postgres`
- Check credentials in `.env`
- Test connection: `psql -h localhost -U discordlite -d discordlite_db`

### "invalid state"
- State tokens expire after 10 minutes
- Verify system clock is correct
- Check `oauth_states` table for expired entries

### "DISCORD_CLIENT_ID is required"
- Copy `.env.example` to `.env`
- Fill in Discord credentials
- Verify no typos in variable names

### "gRPC connection refused"
- Check server is running: `netstat -an | grep 50051`
- Verify port is not in use by another process
- Use `-plaintext` flag for non-TLS connections

## Next Steps

### Immediate (Before First Run)
1. Create Discord application and get credentials
2. Set up `.env` file with all required variables
3. Start PostgreSQL (Docker or local)
4. Run database migrations
5. Start the server

### Short Term (Testing)
1. Write unit tests for critical components
2. Test full OAuth flow end-to-end
3. Verify token encryption works correctly
4. Test session expiry and cleanup
5. Load test with concurrent requests

### Medium Term (Phase 2)
1. Implement Discord API client for guilds/channels
2. Add channel and message fetching
3. Implement caching layer (Redis)
4. Add WebSocket support for real-time updates
5. Implement rate limiting

### Long Term (Production)
1. Add TLS support for both servers
2. Implement monitoring (Prometheus/Grafana)
3. Add distributed tracing (OpenTelemetry)
4. Create admin dashboard
5. Write comprehensive test suite
6. Set up CI/CD pipeline
7. Deploy to production environment

## Success Metrics

### Phase 1 Complete ✅
- [x] All planned features implemented
- [x] Code compiles without errors
- [x] Documentation complete
- [x] Docker support added
- [x] **321 tests passing** (Phase 1 + Phase 2 unit tests)
- [x] **Phase 1 integration tests passing** (8 tests)
- [x] **Multi-language support** (Go + Swift client)

### Phase 2: 85% Complete ⚠️
- [x] GetGuilds RPC working (with tests)
- [x] GetChannels RPC working (with tests)
- [x] GetMessages RPC working with pagination (with tests)
- [x] Database caching implemented (TTL-based)
- [x] Token refresh working automatically
- [x] Rate limiting implemented
- [x] **104 database tests** for Phase 2 tables
- [x] **24 gRPC service tests** for Phase 2 RPCs
- [x] **82 model tests** (100% model coverage)
- [ ] **StreamMessages RPC** (stub only, pending WebSocket wiring)
- [ ] **Phase 2 integration tests** (not created)
- [ ] **Manual testing** with real Discord API
- [ ] **Documentation** updated with Phase 2 examples
- [ ] Cache hit rate validation (target: >70%)
- [ ] API response time benchmarking (target: <100ms)

## Contact & Support

For questions or issues during development:
- Review this document first
- Check README.md for user-facing docs
- Review plan file: `/Users/parsascontentcorner/.claude/plans/compressed-baking-locket.md`
- Test incrementally (don't test everything at once)

---

**Last Updated**: 2025-12-30
**Status**: Phase 2: 85% Complete (321 tests passing, 50%+ coverage)
**Next Milestone**: Complete StreamMessages RPC + Phase 2 Integration Tests + Documentation
