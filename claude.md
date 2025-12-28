# Claude Project Documentation

## Project Overview

**Project Name**: Discord Lite Server
**Purpose**: Golang backend service for Discord OAuth authentication with gRPC API
**Phase**: Phase 1 Complete (Authentication) | Phase 2 Planned (Channels/Messages)
**Status**: Implementation Complete, Testing Pending
**Last Updated**: 2025-12-27

## Architecture Summary

### Core Components

1. **gRPC Server** (Port 50051)
   - AuthService with 3 RPC methods
   - Handles client authentication requests
   - Reflection enabled for development

2. **HTTP Server** (Port 8080)
   - OAuth callback endpoint (`/auth/callback`)
   - Health check endpoint (`/health`)
   - Serves HTML success/error pages

3. **PostgreSQL Database** (Port 5432)
   - 4 tables: users, oauth_tokens, oauth_states, auth_sessions
   - Automatic cleanup of expired sessions (30-minute interval)
   - Connection pooling configured

4. **Discord OAuth Integration**
   - Full OAuth 2.0 flow
   - AES-256-GCM token encryption
   - CSRF protection via state tokens

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

### ⏳ Testing Status

#### ✅ Verified
- [x] Code compiles without errors
- [x] Directory structure is correct
- [x] All files are in place
- [x] Dependencies are resolved
- [x] Protobuf generation works

#### ❌ Not Yet Tested

**Database Operations**
- [ ] Database connection and pooling
- [ ] Schema migration execution
- [ ] CRUD operations for all tables
- [ ] Cleanup job functionality
- [ ] Transaction handling in state validation

**gRPC Service**
- [ ] InitAuth RPC endpoint
- [ ] GetAuthStatus RPC endpoint
- [ ] RevokeAuth RPC endpoint
- [ ] Session ID generation
- [ ] Error handling and status codes

**OAuth Flow**
- [ ] State generation and validation
- [ ] Discord OAuth URL construction
- [ ] Code-to-token exchange
- [ ] User info fetching from Discord API
- [ ] Token encryption/decryption
- [ ] Callback processing end-to-end

**HTTP Server**
- [ ] Health check endpoint
- [ ] OAuth callback handler
- [ ] HTML page rendering
- [ ] Middleware logging

**Integration**
- [ ] Full OAuth flow (browser-based)
- [ ] Concurrent server operation
- [ ] Graceful shutdown
- [ ] Session expiry and cleanup
- [ ] Error scenarios (invalid state, expired session, etc.)

**Docker**
- [ ] Docker image build
- [ ] docker-compose stack startup
- [ ] Container networking
- [ ] Volume persistence
- [ ] Environment variable injection

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

## Phase 2 Roadmap

### Planned Features
1. **GetChannels RPC**
   - Fetch user's Discord guilds
   - Fetch guild channels
   - Cache guild/channel data

2. **GetMessages RPC**
   - Fetch channel messages
   - Pagination support
   - Real-time updates (WebSocket)

3. **Caching Layer**
   - Redis integration
   - Cache guild/channel metadata
   - Reduce Discord API calls

4. **Rate Limiting**
   - Respect Discord rate limits
   - Client-side rate limiting
   - Queue system for requests

### Implementation Notes
- Will require Discord API client library (discordgo)
- Need to handle OAuth token refresh
- Consider WebSocket for real-time updates
- Phase 2 will add 3-5 new RPC methods

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
- [ ] Manual testing successful
- [ ] Unit tests written
- [ ] Integration tests passing

### Phase 2 Goals
- [ ] Channel/message fetching works
- [ ] Cache hit rate > 80%
- [ ] API response time < 100ms
- [ ] Zero token leaks
- [ ] 99% uptime

## Contact & Support

For questions or issues during development:
- Review this document first
- Check README.md for user-facing docs
- Review plan file: `/Users/parsascontentcorner/.claude/plans/compressed-baking-locket.md`
- Test incrementally (don't test everything at once)

---

**Last Updated**: 2025-12-27
**Status**: Phase 1 Implementation Complete, Testing Pending
**Next Milestone**: First Successful OAuth Flow
