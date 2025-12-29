# Swift Client for DiscordLiteAPI

Complete guide for using the DiscordLiteAPI Swift package in your iOS or macOS applications.

## Overview

This Swift package provides a type-safe, modern Swift interface to the Discord Lite Server's gRPC API. It uses Connect-Swift for networking and supports both iOS 15+ and macOS 12+.

## Installation

### Remote Installation (Recommended)

Add to your Xcode project:
1. File → Add Package Dependencies
2. Enter: `https://github.com/parsascontentcorner/discordliteserver`
3. Select version rule (e.g., "Up to Next Major: 1.0.0")
4. Add to your target

Or add to `Package.swift`:
```swift
dependencies: [
    .package(url: "https://github.com/parsascontentcorner/discordliteserver", from: "1.0.0")
]
```

### Local Development

If you have the repository cloned locally:
```swift
dependencies: [
    .package(path: "../DiscordLiteServer")
]
```

## Quick Start

### 1. Import the Module

```swift
import DiscordLiteAPI
import Connect
```

### 2. Create a Client

```swift
let client = ProtocolClient(
    httpClient: URLSessionHTTPClient(),
    config: ProtocolClientConfig(
        host: "http://localhost:50051",
        networkProtocol: .connect
    )
)

let authService = Discord_Auth_V1_AuthServiceClient(client: client)
```

### 3. Authenticate

```swift
// Initiate auth flow
let initRequest = Discord_Auth_V1_InitAuthRequest()
let initResponse = try await authService.initAuth(request: initRequest)

// Open auth URL in browser
if let url = URL(string: initResponse.authURL) {
    #if os(iOS)
    await UIApplication.shared.open(url)
    #elseif os(macOS)
    NSWorkspace.shared.open(url)
    #endif
}

// Poll for authentication status
while true {
    var statusRequest = Discord_Auth_V1_GetAuthStatusRequest()
    statusRequest.sessionID = initResponse.sessionID

    let statusResponse = try await authService.getAuthStatus(request: statusRequest)

    switch statusResponse.status {
    case .pending:
        try await Task.sleep(nanoseconds: 2_000_000_000) // Wait 2 seconds

    case .authStatusAuthenticated:
        print("Authenticated as: \(statusResponse.user?.username ?? "Unknown")")
        return statusResponse.user

    case .authStatusFailed:
        throw AuthError.failed(statusResponse.errorMessage ?? "Unknown error")

    default:
        throw AuthError.unknown
    }
}
```

## Complete Example

Here's a full authentication manager:

```swift
import DiscordLiteAPI
import Connect
import Foundation

@MainActor
class DiscordAuthManager: ObservableObject {
    @Published var authState: AuthState = .unauthenticated
    @Published var user: Discord_Auth_V1_UserInfo?

    private let authService: Discord_Auth_V1_AuthServiceClient
    private var currentSessionID: String?

    enum AuthState {
        case unauthenticated
        case authenticating
        case authenticated
        case failed(String)
    }

    init(serverURL: String = "http://localhost:50051") {
        let client = ProtocolClient(
            httpClient: URLSessionHTTPClient(),
            config: ProtocolClientConfig(
                host: serverURL,
                networkProtocol: .connect
            )
        )
        self.authService = Discord_Auth_V1_AuthServiceClient(client: client)
    }

    func startAuth() async {
        authState = .authenticating

        do {
            let request = Discord_Auth_V1_InitAuthRequest()
            let response = try await authService.initAuth(request: request)

            currentSessionID = response.sessionID

            // Open auth URL
            if let url = URL(string: response.authURL) {
                #if os(iOS)
                await UIApplication.shared.open(url)
                #elseif os(macOS)
                NSWorkspace.shared.open(url)
                #endif
            }

            // Start polling
            await pollAuthStatus()

        } catch {
            authState = .failed(error.localizedDescription)
        }
    }

    private func pollAuthStatus() async {
        guard let sessionID = currentSessionID else { return }

        while authState == .authenticating {
            do {
                var request = Discord_Auth_V1_GetAuthStatusRequest()
                request.sessionID = sessionID

                let response = try await authService.getAuthStatus(request: request)

                switch response.status {
                case .authStatusPending:
                    try await Task.sleep(nanoseconds: 2_000_000_000)

                case .authStatusAuthenticated:
                    user = response.user
                    authState = .authenticated
                    return

                case .authStatusFailed:
                    authState = .failed(response.errorMessage ?? "Authentication failed")
                    return

                default:
                    authState = .failed("Unknown status")
                    return
                }
            } catch {
                authState = .failed(error.localizedDescription)
                return
            }
        }
    }

    func logout() async {
        guard let sessionID = currentSessionID else { return }

        do {
            var request = Discord_Auth_V1_RevokeAuthRequest()
            request.sessionID = sessionID

            _ = try await authService.revokeAuth(request: request)

            authState = .unauthenticated
            user = nil
            currentSessionID = nil
        } catch {
            print("Logout failed: \(error)")
        }
    }
}
```

## SwiftUI Integration

```swift
import SwiftUI
import DiscordLiteAPI

struct ContentView: View {
    @StateObject private var authManager = DiscordAuthManager()

    var body: some View {
        VStack(spacing: 20) {
            switch authManager.authState {
            case .unauthenticated:
                Button("Login with Discord") {
                    Task {
                        await authManager.startAuth()
                    }
                }

            case .authenticating:
                ProgressView("Authenticating...")

            case .authenticated:
                if let user = authManager.user {
                    VStack {
                        Text("Welcome, \(user.username)!")
                        Text("Discord ID: \(user.discordID)")
                            .font(.caption)

                        Button("Logout") {
                            Task {
                                await authManager.logout()
                            }
                        }
                    }
                }

            case .failed(let error):
                VStack {
                    Text("Authentication Failed")
                        .foregroundColor(.red)
                    Text(error)
                        .font(.caption)

                    Button("Try Again") {
                        Task {
                            await authManager.startAuth()
                        }
                    }
                }
            }
        }
        .padding()
    }
}
```

## API Reference

### Services

#### AuthService

The main authentication service.

**Methods:**
- `initAuth(request:)` - Start OAuth flow
- `getAuthStatus(request:)` - Check authentication status
- `revokeAuth(request:)` - Revoke authentication

### Types

#### AuthStatus

```swift
public enum Discord_Auth_V1_AuthStatus {
    case authStatusUnspecified  // Default, unknown
    case authStatusPending      // Waiting for user
    case authStatusAuthenticated // Successfully authenticated
    case authStatusFailed       // Authentication failed
}
```

#### UserInfo

```swift
public struct Discord_Auth_V1_UserInfo {
    public var discordID: String      // Discord user ID
    public var username: String       // Discord username
    public var discriminator: String  // Discord discriminator
    public var avatar: String         // Avatar hash
    public var email: String          // User email
}
```

#### InitAuthRequest

```swift
public struct Discord_Auth_V1_InitAuthRequest {
    public var sessionID: String // Optional: provide custom session ID
}
```

#### InitAuthResponse

```swift
public struct Discord_Auth_V1_InitAuthResponse {
    public var authURL: String    // Discord OAuth URL
    public var sessionID: String  // Session ID for polling
    public var state: String      // OAuth state parameter
}
```

#### GetAuthStatusRequest

```swift
public struct Discord_Auth_V1_GetAuthStatusRequest {
    public var sessionID: String // Session ID from InitAuth
}
```

#### GetAuthStatusResponse

```swift
public struct Discord_Auth_V1_GetAuthStatusResponse {
    public var status: Discord_Auth_V1_AuthStatus
    public var user: Discord_Auth_V1_UserInfo?
    public var errorMessage: String?
}
```

#### RevokeAuthRequest

```swift
public struct Discord_Auth_V1_RevokeAuthRequest {
    public var sessionID: String
}
```

#### RevokeAuthResponse

```swift
public struct Discord_Auth_V1_RevokeAuthResponse {
    public var success: Bool
    public var message: String
}
```

## Configuration

### Server URL

For development:
```swift
host: "http://localhost:50051"
```

For production:
```swift
host: "https://your-domain.com:50051"
```

### Network Protocol

Connect-Swift supports multiple protocols:
```swift
networkProtocol: .connect  // Recommended
// or
networkProtocol: .grpcWeb
```

## Error Handling

All service methods throw `ConnectError`:

```swift
do {
    let response = try await authService.initAuth(request: request)
} catch let error as ConnectError {
    switch error.code {
    case .unavailable:
        print("Server unavailable")
    case .unauthenticated:
        print("Not authenticated")
    case .invalidArgument:
        print("Invalid request")
    default:
        print("Error: \(error.message)")
    }
}
```

## Troubleshooting

### Connection Refused

Ensure the server is running:
```bash
cd DiscordLiteServer
docker-compose up -d
# or
make run
```

### Authentication Loop

If polling never completes:
1. Check server logs: `docker-compose logs -f app`
2. Verify Discord OAuth redirect URI matches
3. Check session hasn't expired (default 24 hours)

### Import Errors

If Xcode can't find the module:
1. Clean build folder: Product → Clean Build Folder
2. Reset package cache: File → Packages → Reset Package Caches
3. Verify package is added to your target

## Generated Code

This package contains auto-generated Swift code from Protocol Buffers:

**Location:** `api/gen/swift/discord/auth/v1/`

**Generated files:**
- `auth.pb.swift` - Protocol Buffers messages
- `auth.connect.swift` - Connect-Swift service stubs

To regenerate after proto changes:
```bash
cd DiscordLiteServer
make proto-swift
```

## Requirements

- iOS 15.0+ / macOS 12.0+
- Swift 5.9+
- Xcode 15.0+

## Dependencies

- [swift-protobuf](https://github.com/apple/swift-protobuf) (1.27.0+)
- [connect-swift](https://github.com/connectrpc/connect-swift) (1.0.0+)

## License

See LICENSE in the repository root.

## Support

For issues:
- GitHub Issues: https://github.com/parsascontentcorner/discordliteserver/issues
- Documentation: See README.md in repository root
