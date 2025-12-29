// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "DiscordLiteAPI",
    platforms: [
        .iOS(.v15),
        .macOS(.v12),
    ],
    products: [
        .library(
            name: "DiscordLiteAPI",
            targets: ["DiscordLiteAPI"]
        ),
    ],
    dependencies: [
        .package(
            url: "https://github.com/apple/swift-protobuf.git",
            from: "1.27.0"
        ),
        .package(
            url: "https://github.com/connectrpc/connect-swift.git",
            from: "1.0.0"
        ),
    ],
    targets: [
        .target(
            name: "DiscordLiteAPI",
            dependencies: [
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
                .product(name: "Connect", package: "connect-swift"),
            ],
            path: "api/gen/swift"
        ),
    ]
)
