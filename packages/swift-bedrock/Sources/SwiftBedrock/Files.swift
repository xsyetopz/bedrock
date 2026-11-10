import Foundation

/// The release `new` and `adopt` pin BuildTools to.
let version = "0.1.1"
let repository = "https://github.com/xsyetopz/bedrock"

/// The files swift-bedrock writes, copied into its bundle at build time.
enum Files {
  static let root: String = {
    guard let url = Bundle.module.url(forResource: "files", withExtension: nil) else {
      fatalError("the swift-bedrock bundle has no files directory")
    }
    return url.path
  }()

  static func read(_ path: String) -> String {
    guard let text = try? String(contentsOfFile: root + "/" + path, encoding: .utf8) else {
      fatalError("the swift-bedrock bundle has no \(path)")
    }
    return text
  }

  /// The file at `directory/path`, written to `path`.
  static func managed(_ directory: String, _ path: String, executable: Bool = false) -> Step {
    .write(path: path, content: read(directory + "/" + path), executable: executable)
  }

  /// Every file under `directory`, by path relative to it.
  static func tree(_ directory: String) -> [(path: String, content: String)] {
    let folder = root + "/" + directory
    let paths = FileManager.default.enumerator(atPath: folder)?.compactMap { $0 as? String } ?? []
    return paths.sorted().compactMap { path in
      (try? String(contentsOfFile: folder + "/" + path, encoding: .utf8)).map { (path, $0) }
    }
  }

  static let base: [Step] = [
    managed("base", "justfile"),
    managed("base", ".githooks/commit-msg", executable: true),
    managed("base", ".githooks/pre-commit", executable: true),
    managed("base", ".markdownlint.jsonc"),
    managed("base", ".github/workflows/base.yml"),
  ]

  /// typescript-bedrock writes its own Markdown config, with the custom rules it installs.
  static let markdown = managed("base", ".markdownlint-cli2.jsonc")

  /// The Swift files, with the test config only where `Tests` exists and the xcconfig
  /// only beside an Xcode project.
  static func swift(cxx: Bool, tests: Bool, xcode: Bool) -> [Step] {
    var steps = [
      managed("swift", ".just/swift.just"),
      managed("swift", ".github/workflows/swift.yml"),
      managed("swift", ".swiftlint.yml"),
      managed("swift", ".swift-format"),
      managed("swift", ".swift-version"),
    ]
    if tests {
      steps.append(managed("swift", "Tests/.swiftlint.yml"))
    }
    if xcode {
      let xcconfig = read("swift/Config/Bedrock.xcconfig")
      steps.append(
        .write(
          "Config/Bedrock.xcconfig",
          cxx
            ? xcconfig.replacingOccurrences(
              of: "BEDROCK_CXX_FLAGS =\n",
              with: "BEDROCK_CXX_FLAGS = $(BEDROCK_CXX_INTEROP)\n"
            )
            .replacingOccurrences(
              of: "COMPILATION_CACHE_ENABLE_CACHING = YES\n",
              with: "COMPILATION_CACHE_ENABLE_CACHING = NO\n"
            )
            : xcconfig
        )
      )
    }
    return steps
  }

  static let cpp: [Step] = [
    managed("cpp", ".just/cpp.just"),
    managed("cpp", ".clang-tidy"),
    managed("cpp", ".clang-format"),
  ]

  /// The bedrock block and this bedrock's block for a file other bedrocks share.
  static func blocks(_ file: String, cxx: Bool = false) -> [Block] {
    var blocks = [
      Block(name: "bedrock", content: read("base/blocks/" + file)),
      Block(name: "swift-bedrock", content: read("blocks/" + file)),
    ]
    if cxx, FileManager.default.fileExists(atPath: root + "/cxx-blocks/" + file) {
      blocks.append(Block(name: "swift-bedrock-cxx", content: read("cxx-blocks/" + file)))
    }
    return blocks
  }

  /// The Package.swift blocks.
  static func manifest(cxx: Bool) -> [Block] {
    [Block(name: "swift-bedrock", content: read("blocks/Package.swift"))]
      + (cxx ? [Block(name: "swift-bedrock-cxx", content: read("cxx-blocks/Package.swift"))] : [])
  }
}
