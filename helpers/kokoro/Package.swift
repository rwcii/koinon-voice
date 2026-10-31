// swift-tools-version: 6.2

// The Kokoro helper: Kokoro-82M on MLX Swift through kokoro-ios, with MisakiSwift for text to
// phonemes, behind the bridge's sidecar protocol (ADR 002, Decisions 1 and 4). MLX's Metal
// shaders build only with xcodebuild: use scripts/build-kokoro-helper.sh.
import PackageDescription

let package = Package(
    name: "koinon-voice-kokoro",
    // SidecarProtocol's package needs macOS 26.4.
    platforms: [.macOS("26.4")],
    dependencies: [
        .package(path: "../apple"),
        .package(url: "https://github.com/mlalma/kokoro-ios", exact: "1.0.11"),
        // The versions kokoro-ios 1.0.11 pins. MisakiSwift 1.0.6 is a local copy with the
        // patches in patches/, which fetch-misaki.sh makes; kokoro-ios uses it too.
        .package(path: ".misaki/MisakiSwift"),
        .package(url: "https://github.com/ml-explore/mlx-swift", exact: "0.30.2"),
    ],
    targets: [
        // What the helper does without the model, so its tests need no model or Metal.
        .target(name: "KokoroText"),
        .executableTarget(
            name: "koinon-voice-kokoro",
            dependencies: [
                "KokoroText",
                .product(name: "SidecarProtocol", package: "apple"),
                .product(name: "KokoroSwift", package: "kokoro-ios"),
                .product(name: "MisakiSwift", package: "MisakiSwift"),
                .product(name: "MLX", package: "mlx-swift"),
            ],
            swiftSettings: [.swiftLanguageMode(.v5)],
            linkerSettings: [
                // The build script puts the frameworks beside the helper (helpers/kokoro/README.md).
                .unsafeFlags(["-Xlinker", "-rpath", "-Xlinker", "@executable_path/Frameworks"]),
            ]
        ),
        .testTarget(name: "KokoroTextTests", dependencies: ["KokoroText"]),
    ]
)
