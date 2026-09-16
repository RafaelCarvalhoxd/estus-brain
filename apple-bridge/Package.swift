// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "estus-apple-bridge",
    platforms: [.macOS(.v26)],
    targets: [
        .executableTarget(
            name: "estus-apple-bridge",
            path: "Sources/estus-apple-bridge"
        ),
        .testTarget(
            name: "estus-apple-bridgeTests",
            dependencies: ["estus-apple-bridge"],
            path: "Tests/estus-apple-bridgeTests"
        ),
    ]
)
