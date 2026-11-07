import Foundation

/// Package.swift edits: the tools version, and the blocks at the end that make
/// every target strict.
enum Manifest {
  static let toolsVersion = "// swift-tools-version: 6.4"

  /// The upcoming features `swift package migrate` can rewrite code for.
  ///
  /// The other two, InternalImportsByDefault and ImmutableWeakCaptures, need a person.
  static let migratable = [
    "ExistentialAny", "MemberImportVisibility", "InferIsolatedConformances",
    "NonisolatedNonsendingByDefault", "StrictMemorySafety",
  ]

  static func withToolsVersion(_ text: String) -> String {
    var lines = text.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
    let first = lines.first?.filter { !$0.isWhitespace }.lowercased() ?? ""
    if first.hasPrefix("//swift-tools-version") {
      lines[0] = toolsVersion
    } else {
      lines.insert(toolsVersion, at: 0)
    }
    return lines.joined(separator: "\n")
  }

  /// Raises a macOS target below 12, the oldest the template builds for, to `.v12`.
  static func withMinimumMacOS(_ text: String) -> String {
    // Regex needs macOS 13, newer than swift-bedrock targets.
    text.replacingOccurrences(
      of: #"\.macOS\(\s*(?:\.v10_\d+|\.v11|"(?:10\.\d+|11)(?:\.\d+)*")\s*\)"#,
      with: ".macOS(.v12)",
      options: .regularExpression
    )
  }

  static func adopted(_ text: String, cxx: Bool) -> String {
    upsert(
      withMinimumMacOS(withToolsVersion(text)),
      header: "",
      syntax: .slashes,
      blocks: Files.manifest(cxx: cxx)
    )
  }

  /// The manifest `swift package migrate` runs on: the new tools version, without the
  /// strict blocks, since migrate stops at a warning that the blocks make an error,
  /// and in Swift 5 mode, so code that is not yet Swift 6 still builds.
  static func forMigration(_ text: String) -> String {
    var kept: [Substring] = []
    var inBlock = false
    let manifest = withMinimumMacOS(withToolsVersion(text))
    for line in manifest.split(separator: "\n", omittingEmptySubsequences: false) {
      if line.hasPrefix("// >>> swift-bedrock") {
        inBlock = true
      } else if line.hasPrefix("// <<< swift-bedrock") {
        inBlock = false
      } else if !inBlock {
        kept.append(line)
      }
    }
    return kept.joined(separator: "\n") + "\npackage.swiftLanguageModes = [.v5]\n"
  }
}
