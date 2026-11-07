import Foundation
import Testing

@testable import SwiftBedrock

@Suite
struct Blocks {
  let swift = Block(name: "swift-bedrock", content: "a\n")
  let cxx = Block(name: "swift-bedrock-cxx", content: "b\n")

  @Test
  func createsFromHeader() {
    #expect(
      upsert(nil, header: "root = true\n", syntax: .hash, blocks: [swift])
        == "root = true\n\n# >>> swift-bedrock\na\n# <<< swift-bedrock\n"
    )
  }

  @Test
  func replacesInPlaceAndKeepsTheRest() {
    let current = "x\n\n# >>> swift-bedrock\nold\n# <<< swift-bedrock\n\ny\n"
    #expect(
      upsert(current, header: "", syntax: .hash, blocks: [swift])
        == "x\n\n# >>> swift-bedrock\na\n# <<< swift-bedrock\n\ny\n"
    )
  }

  @Test
  func insertsInBedrockOrder() {
    let base = Block(name: "bedrock", content: "c\n")
    let cxxFirst = upsert(nil, header: "", syntax: .hash, blocks: [cxx])
    let swiftFirst = upsert(nil, header: "", syntax: .hash, blocks: [swift])
    let one = upsert(cxxFirst, header: "", syntax: .hash, blocks: [swift, base])
    let other = upsert(swiftFirst, header: "", syntax: .hash, blocks: [base, cxx])
    #expect(one == other)
    #expect(one.hasPrefix("# >>> bedrock\nc\n# <<< bedrock\n\n# >>> swift-bedrock\n"))
  }

  @Test
  func keepsBlankLinesInContent() {
    let block = Block(name: "bedrock", content: "a\n\nb\n")
    #expect(
      upsert(nil, header: "", syntax: .markdown, blocks: [block])
        == "<!-- bedrock -->\na\n\nb\n<!-- /bedrock -->\n"
    )
  }

  @Test
  func isIdempotent() {
    let once = upsert("x\n", header: "", syntax: .slashes, blocks: [swift, cxx])
    #expect(upsert(once, header: "", syntax: .slashes, blocks: [swift, cxx]) == once)
  }
}

@Suite
struct Steps {
  let root = FileManager.default.temporaryDirectory
    .appendingPathComponent("swift-bedrock-\(UUID().uuidString)").path

  @Test
  func appliesAndBacksUp() throws {
    let file = root + "/a/b.txt"
    try FileManager.default.createDirectory(
      atPath: root + "/a",
      withIntermediateDirectories: true
    )
    try "old".write(toFile: file, atomically: true, encoding: .utf8)
    let step = Step.write(path: "a/b.txt", content: "new", executable: true)
    #expect(!step.isDone(in: root))
    try step.apply(in: root)
    #expect(step.isDone(in: root))
    #expect(try String(contentsOfFile: file + ".bak", encoding: .utf8) == "old")
    try Step.write("a/b.txt", "newer").apply(in: root)
    #expect(try String(contentsOfFile: file + ".bak", encoding: .utf8) == "old")

    let link = Step.link(path: "L.md", target: "missing.md")
    try link.apply(in: root)
    #expect(link.isDone(in: root))
    #expect(exists(root + "/L.md"))
    #expect(pending([link, .remove(path: "gone")], in: root).isEmpty)
    try FileManager.default.removeItem(atPath: root)
  }

  @Test
  func repointsBuildToolsOnlyForAPath() throws {
    try FileManager.default.createDirectory(
      atPath: root + "/BuildTools",
      withIntermediateDirectories: true
    )
    try "import PackageDescription\n".write(
      toFile: root + "/Package.swift",
      atomically: true,
      encoding: .utf8
    )
    try "pinned".write(
      toFile: root + "/BuildTools/Package.swift",
      atomically: true,
      encoding: .utf8
    )
    let buildTools = { (source: Source) in
      try plan(root, source: source, cxx: false).filter {
        $0.path == "BuildTools/Package.swift"
      }
    }
    #expect(try buildTools(.release).isEmpty)
    #expect(try buildTools(.path("../bedrock")).count == 1)
    try FileManager.default.removeItem(atPath: root)
  }

  @Test
  func writesTestAndXcodeConfigsOnlyWhereTheyApply() throws {
    try FileManager.default.createDirectory(atPath: root, withIntermediateDirectories: true)
    try "import PackageDescription\n".write(
      toFile: root + "/Package.swift",
      atomically: true,
      encoding: .utf8
    )
    let configs = {
      try plan(root, source: .release, cxx: false).map(\.path).filter {
        ["Tests/.swiftlint.yml", "Config/Bedrock.xcconfig"].contains($0)
      }
    }
    #expect(try configs().isEmpty)
    for folder in ["Tests", "App.xcodeproj"] {
      try FileManager.default.createDirectory(
        atPath: root + "/" + folder,
        withIntermediateDirectories: true
      )
    }
    #expect(try configs() == ["Tests/.swiftlint.yml", "Config/Bedrock.xcconfig"])
    let cxxConfig = try plan(root, source: .release, cxx: true).first {
      $0.path == "Config/Bedrock.xcconfig"
    }
    guard case .write(_, let xcconfig, _) = try #require(cxxConfig) else {
      Issue.record("the xcconfig is not a write")
      return
    }
    #expect(xcconfig.contains("COMPILATION_CACHE_ENABLE_CACHING = NO\n"))
    try FileManager.default.removeItem(atPath: root)
  }
}

@Suite
struct Manifests {
  @Test(arguments: [
    "// swift-tools-version:5.9\nimport PackageDescription\n",
    "//swift-tools-version: 6.0\nimport PackageDescription\n",
    "import PackageDescription\n",
  ])
  func setsToolsVersion(_ text: String) {
    #expect(
      Manifest.withToolsVersion(text) == "// swift-tools-version: 6.4\nimport PackageDescription\n"
    )
  }

  @Test(arguments: [
    (".macOS(.v10_15), .iOS(.v15)", ".macOS(.v12), .iOS(.v15)"),
    (#".macOS( "11.0.1" )"#, ".macOS(.v12)"),
    (".macOS(.v11)", ".macOS(.v12)"),
    (".macOS(.v13)", ".macOS(.v13)"),
    (".macOS(.v110)", ".macOS(.v110)"),
  ])
  func raisesOldMacOS(_ old: String, _ new: String) {
    #expect(Manifest.withMinimumMacOS("platforms: [\(old)]") == "platforms: [\(new)]")
  }

  @Test
  func stripsBlocksForMigration() {
    let adopted = Manifest.adopted("import PackageDescription\n", cxx: true)
    #expect(adopted.contains("// >>> swift-bedrock-cxx"))
    let migrating = Manifest.forMigration(adopted)
    #expect(!migrating.contains("swift-bedrock"))
    #expect(migrating.hasSuffix("package.swiftLanguageModes = [.v5]\n"))
    #expect(Manifest.adopted(adopted, cxx: true) == adopted)
  }
}

@Suite
struct Guards {
  let target: [String: Any] = [
    "name": "A", "type": "regular",
    "settings": Guard.strict.map { setting -> [String: Any] in
      let parts = setting.split(separator: ":").map(String.init)
      return ["kind": [parts[0]: parts.count > 1 ? ["_0": parts[1]] : [:]]]
    },
  ]

  @Test
  func acceptsStrictPackage() {
    let package: [String: Any] = [
      "toolsVersion": ["_version": "6.4.0"], "swiftLanguageVersions": ["6"],
      "targets": [target, ["name": "S", "type": "system"]],
    ]
    #expect(Guard.manifest(package).isEmpty)
  }

  @Test
  func reportsLaxPackage() {
    let package: [String: Any] = [
      "toolsVersion": ["_version": "5.9.0"], "swiftLanguageVersions": ["5", "6"],
      "targets": [["name": "B", "type": "regular", "settings": []]],
    ]
    #expect(Guard.manifest(package).count == 3)
  }

  @Test
  func findsXCTestImports() {
    #expect(Guard.importsXCTest("import Foundation\n@testable import XCTest\n"))
    #expect(Guard.importsXCTest("  @preconcurrency import class XCTest.XCTestCase\n"))
    #expect(!Guard.importsXCTest("// import XCTest\nimport XCTestDynamicOverlay\n"))
    #expect(Guard.isUITest("AppUITests/LaunchTests.swift"))
    #expect(!Guard.isUITest("Tests/AppTests/UITests.swift"))
  }

  @Test
  func findsFileWideSuppressions() throws {
    let text = try packageFile("Tests/Fixtures/Suppressions.swift.txt")
    #expect(Guard.suppressions(text).map { $0.prefix(2) } == ["1:", "6:"])
  }
}

@Suite
struct Starters {
  @Test(arguments: [
    ("my-tool", "MyTool"), ("swift_kit2", "SwiftKit2"), ("Mixed.Case", "MixedCase"),
  ])
  func namesTypes(_ raw: String, _ name: String) {
    #expect(typeName(raw) == name)
    #expect(isTypeName(name))
  }

  @Test
  func rejectsBadTypeNames() {
    #expect(!isTypeName("2fast"))
    #expect(!isTypeName(""))
    #expect(!isTypeName("Café"))
  }

  @Test
  func writesEveryKind() {
    let steps = starterFiles(name: "Kit", kinds: Set(Kind.allCases), cxx: true)
    let paths = Set(steps.map(\.path))
    #expect(
      paths.isSuperset(of: [
        "Package.swift", "README.md", "Sources/Kit/Kit.swift", "Sources/KitCLI/main.swift",
        "Sources/KitCxx/include/KitCxx.h", "Kit.xcodeproj/project.pbxproj",
      ])
    )
    for case .write(_, let content, _) in steps {
      #expect(!content.contains("__Name__") && !content.contains("__name__"))
    }
  }

  @Test
  func wiresXcodeOnce() throws {
    let steps = starterFiles(name: "Kit", kinds: [.app], cxx: false)
    guard
      case .write(_, let pbxproj, _) = try #require(
        steps.first { $0.path.hasSuffix("project.pbxproj") }
      )
    else {
      Issue.record("pbxproj is not a write")
      return
    }
    #expect(Xcode.wired(pbxproj) == nil)
    #expect(Xcode.overrides(pbxproj).isEmpty)
    let unwired = pbxproj.split(separator: "\n", omittingEmptySubsequences: false)
      .filter { !$0.contains("Bedrock.xcconfig") }.joined(separator: "\n")
    let wired = try #require(Xcode.wired(unwired))
    #expect(wired.components(separatedBy: "Bedrock.xcconfig").count == 7)
    #expect(Xcode.wired(wired) == nil)
  }
}
