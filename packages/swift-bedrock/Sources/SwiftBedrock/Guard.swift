import Foundation

/// What SwiftLint cannot see: the settings the manifest ends up with, XCTest outside
/// UI tests, file-wide suppressions, foreign tools, and drift from what adopt writes.
enum Guard {
  static let strict = [
    "enableUpcomingFeature:ExistentialAny",
    "enableUpcomingFeature:InternalImportsByDefault",
    "enableUpcomingFeature:MemberImportVisibility",
    "enableUpcomingFeature:InferIsolatedConformances",
    "enableUpcomingFeature:NonisolatedNonsendingByDefault",
    "enableUpcomingFeature:ImmutableWeakCaptures",
    "strictMemorySafety",
    "treatAllWarnings:error",
  ]

  static let foreign = [
    "Podfile": "CocoaPods: move each pod to a SwiftPM dependency in Package.swift",
    "Cartfile": "Carthage: move each framework to a SwiftPM dependency in Package.swift",
    ".swiftformat": "SwiftFormat: swift-format and .swift-format replaced it",
  ]

  static func run(_ root: String, source: Source, fix: Bool) throws -> [String] {
    var problems: [String] = []
    if exists(root + "/Package.swift") {
      problems += try manifest(dumped(root))
    }
    for path in try trackedFiles(in: root, ["*.swift"]) {
      problems += try check(path, in: root, fix: fix)
    }
    for (path, problem) in foreign.sorted(by: { $0.key < $1.key }) where exists(root + "/" + path) {
      problems.append("\(path): \(problem)")
    }
    for project in Xcode.projects(in: root) {
      let path = project + "/project.pbxproj"
      guard let text = read(root, path) else {
        continue
      }
      if !text.contains("Bedrock.xcconfig") {
        problems.append(
          """
          \(path): set Config/Bedrock.xcconfig as the base configuration of the project's \
          Debug and Release
          """
        )
      }
      problems += Set(Xcode.overrides(text)).sorted().map { "\(path): \($0); remove it" }
    }
    let cxx = usesCxx(root)
    for step in pending(try plan(root, source: source, cxx: cxx), in: root) {
      problems.append(
        """
        \(step.path): differs from what swift-bedrock \(version) writes; run swift-bedrock \
        adopt --dry-run to see the change
        """
      )
    }
    return problems
  }

  private static func dumped(_ root: String) throws -> [String: Any] {
    let output = try shell(["swift", "package", "dump-package"], in: root)
    guard let json = try JSONSerialization.jsonObject(with: Data(output.utf8)) as? [String: Any]
    else {
      throw Failure("swift package dump-package printed no package")
    }
    return json
  }

  /// Problems with the package swift package dump-package describes.
  static func manifest(_ package: [String: Any]) -> [String] {
    var problems: [String] = []
    let tools = (package["toolsVersion"] as? [String: Any])?["_version"] as? String ?? ""
    if !tools.hasPrefix("6.4") {
      problems.append("Package.swift: swift-tools-version is \(tools); use 6.4")
    }
    let modes = package["swiftLanguageVersions"] as? [String] ?? []
    if modes != ["6"] {
      problems.append("Package.swift: the language modes are \(modes); use [.v6] alone")
    }
    for target in package["targets"] as? [[String: Any]] ?? [] {
      let name = target["name"] as? String ?? "?"
      guard !["system", "binary", "plugin"].contains(target["type"] as? String) else {
        continue
      }
      let settings = (target["settings"] as? [[String: Any]] ?? []).compactMap {
        $0["kind"] as? [String: Any]
      }
      let found = Set(
        settings.flatMap { kind in
          kind.map { key, value in
            ((value as? [String: Any])?["_0"] as? String).map { "\(key):\($0)" } ?? key
          }
        }
      )
      let missing = strict.filter { !found.contains($0) }
      if !missing.isEmpty {
        problems.append(
          """
          Package.swift: target \(name) lacks \(missing.joined(separator: ", ")); keep the \
          swift-bedrock block last
          """
        )
      }
      if settings.contains(where: { $0["unsafeFlags"] != nil }) {
        problems.append(
          """
          Package.swift: target \(name) uses unsafeFlags, which keeps others from depending \
          on the package
          """
        )
      }
    }
    return problems
  }

  static func isUITest(_ path: String) -> Bool {
    path.split(separator: "/").dropLast().contains {
      $0.hasSuffix("UITests") || $0.hasSuffix("UITest")
    }
  }

  static func importsXCTest(_ text: String) -> Bool {
    text.range(
      of: #"(?m)^\s*(?:@\w+\s+)*import\s+(?:\w+\s+)?XCTest\b"#,
      options: .regularExpression
    )
      != nil
  }

  private static func check(_ path: String, in root: String, fix: Bool) throws -> [String] {
    var problems: [String] = []
    guard var text = read(root, path) else {
      return []
    }
    if !isUITest(path), importsXCTest(text) {
      if fix, let converted = XCTestConverter.convert(text) {
        try converted.text.write(toFile: root + "/" + path, atomically: true, encoding: .utf8)
        text = converted.text
        problems += converted.notes.map { "\(path): \($0)" }
      }
      if importsXCTest(text) {
        problems.append(
          """
          \(path): imports XCTest outside a *UITests target; use Swift Testing \
          (just lint-fix converts it)
          """
        )
      }
    }
    problems += suppressions(text).map { "\(path):\($0)" }
    return problems
  }

  /// Lines that turn a SwiftLint rule off for the rest of the file, or swift-format off
  /// for the whole file.
  static func suppressions(_ text: String) -> [String] {
    let directives = text.split(separator: "\n", omittingEmptySubsequences: false).map(directive)
    var found: [String] = []
    for (index, directive) in directives.enumerated() {
      if directive.hasPrefix("swift-format-ignore-file") {
        found.append("\(index + 1): swift-format-ignore-file; ignore the one declaration instead")
      }
      let disable = "swiftlint:disable"
      guard directive.hasPrefix(disable), !directive.dropFirst(disable.count).hasPrefix(":") else {
        continue
      }
      let rules = directive.dropFirst(disable.count).split(separator: " ")
        .prefix { !$0.hasPrefix("//") }
      let reenabled = directives[(index + 1)...].contains { later in
        let enable = "swiftlint:enable"
        return later.hasPrefix(enable)
          && rules.allSatisfy(Set(later.dropFirst(enable.count).split(separator: " ")).contains)
      }
      if !reenabled {
        found.append(
          """
          \(index + 1): swiftlint:disable without a matching swiftlint:enable; use \
          swiftlint:disable:next
          """
        )
      }
    }
    return found
  }

  /// The text of a line's `//` comment, where SwiftLint and swift-format read directives.
  private static func directive(_ line: Substring) -> Substring {
    guard let start = line.range(of: "//") else {
      return ""
    }
    return line[start.lowerBound...].drop { $0 == "/" || $0 == " " }
  }
}
