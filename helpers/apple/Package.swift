// swift-tools-version: 6.2

// The Apple helper: Foundation Models, SpeechAnalyzer and AVSpeechSynthesizer behind the
// bridge's sidecar protocol (ADR 002, Decision 1). Build with `swift build -c release`.
import PackageDescription

let package = Package(
    name: "koinon-voice-apple",
    // tokenCount(for:) needs macOS 26.4.
    platforms: [.macOS("26.4")],
    products: [
        .executable(name: "koinon-voice-apple", targets: ["koinon-voice-apple"]),
        // The Kokoro helper (helpers/kokoro) shares this protocol implementation.
        .library(name: "SidecarProtocol", targets: ["SidecarProtocol"]),
    ],
    targets: [
        .target(name: "SidecarProtocol"),
        .executableTarget(
            name: "koinon-voice-apple",
            dependencies: ["SidecarProtocol"],
            swiftSettings: [.swiftLanguageMode(.v5)],
            linkerSettings: [
                // A command-line tool has no bundle; the usage strings for the microphone and
                // speech recognition live in an embedded Info.plist.
                .unsafeFlags(["-Xlinker", "-sectcreate", "-Xlinker", "__TEXT", "-Xlinker", "__info_plist", "-Xlinker", Context.packageDirectory + "/Info.plist"]),
            ]
        ),
        .testTarget(name: "SidecarProtocolTests", dependencies: ["SidecarProtocol"]),
    ]
)
