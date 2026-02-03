// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "Glimpse",
    platforms: [
        .macOS(.v14)
    ],
    products: [
        .executable(name: "Glimpse", targets: ["Glimpse"])
    ],
    targets: [
        .executableTarget(
            name: "Glimpse",
            path: "Sources"
        )
    ]
)
