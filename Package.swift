// swift-tools-version: 6.4
import PackageDescription

let package = Package(
  name: "bedrock",
  platforms: [.macOS(.v12)],
  products: [
    .executable(name: "swift-bedrock", targets: ["swift-bedrock"])
  ],
  targets: [
    .target(
      name: "SwiftBedrock",
      path: "packages/swift-bedrock/Sources/SwiftBedrock",
      resources: [.copy("files")]
    ),
    .executableTarget(
      name: "swift-bedrock",
      dependencies: ["SwiftBedrock"],
      path: "packages/swift-bedrock/Sources/swift-bedrock"
    ),
    .testTarget(
      name: "SwiftBedrockTests",
      dependencies: ["SwiftBedrock"],
      path: "packages/swift-bedrock/Tests/SwiftBedrockTests"
    ),
  ]
)

// >>> swift-bedrock
// Managed by swift-bedrock: Swift 6, the upcoming features, strict memory safety,
// and warnings as errors, for every target.
package.swiftLanguageModes = [.v6]
for target in package.targets where ![.system, .binary, .plugin].contains(target.type) {
  target.swiftSettings =
    (target.swiftSettings ?? []) + [
      .enableUpcomingFeature("ExistentialAny"),
      .enableUpcomingFeature("InternalImportsByDefault"),
      .enableUpcomingFeature("MemberImportVisibility"),
      .enableUpcomingFeature("InferIsolatedConformances"),
      .enableUpcomingFeature("NonisolatedNonsendingByDefault"),
      .enableUpcomingFeature("ImmutableWeakCaptures"),
      .strictMemorySafety(),
      .treatAllWarnings(as: .error),
    ]
}
// <<< swift-bedrock

// >>> swift-bedrock-cxx
// Managed by swift-bedrock: C++20, and C++ interop for every Swift target, since a
// target that imports a C++ module, even through another target, needs it.
package.cxxLanguageStandard = .cxx20
for target in package.targets where [.regular, .executable, .test].contains(target.type) {
  target.swiftSettings = (target.swiftSettings ?? []) + [.interoperabilityMode(.Cxx)]
}
// <<< swift-bedrock-cxx
