import Foundation

/// Files whose job a managed file took over: hook managers, formatters, Make, and
/// the old SwiftLint config name.
private let replaced = [
  ".husky", ".lefthook.yml", ".pre-commit-config.yaml", ".simple-git-hooks.json",
  ".swiftformat", ".swiftlint.yaml", "Makefile", "Mintfile", "lefthook.yaml", "lefthook.yml",
]

/// C and C++ files, whose presence turns on the C++ setup.
let cxxPatterns = ["*.c", "*.cc", "*.cpp", "*.cxx", "*.h", "*.hh", "*.hpp", "*.hxx", "*.mm"]

/// Where BuildTools gets swift-bedrock from.
enum Source: Equatable {
  /// The tagged release of this version.
  case release
  /// A local checkout, as a path relative to BuildTools/.
  case path(String)

  var buildTools: String {
    let dependency =
      switch self {
      case .release: #".package(url: "\#(repository)", revision: "swift-bedrock-v\#(version)")"#
      case .path(let path): #".package(path: "\#(path)")"#
      }
    return """
      // swift-tools-version: 6.4
      // Runs swift-bedrock for `just`: swift run --package-path BuildTools swift-bedrock
      import PackageDescription

      let package = Package(
        name: "BuildTools",
        platforms: [.macOS(.v12)],
        dependencies: [
          \(dependency)
        ]
      )

      """
  }
}

/// Whether the repository has C or C++ code, or adopted the C++ setup before.
func usesCxx(_ root: String) -> Bool {
  exists(root + "/.just/cpp.just") || !((try? trackedFiles(in: root, cxxPatterns)) ?? []).isEmpty
}

/// AGENTS.md, its links, and the files every bedrock writes a block into.
private func shared(_ root: String, cxx: Bool) -> [Step] {
  var steps: [Step] = []
  var agents = read(root, "AGENTS.md")
  if agents == nil,
    (try? FileManager.default.destinationOfSymbolicLink(atPath: root + "/CLAUDE.md")) == nil
  {
    agents = read(root, "CLAUDE.md")
  }
  steps.append(
    .write(
      "AGENTS.md",
      upsert(agents, header: "# Agents\n", syntax: .markdown, blocks: Files.blocks("AGENTS.md"))
    )
  )
  steps += ["CLAUDE.md", "GEMINI.md"].map { .link(path: $0, target: "AGENTS.md") }
  for (path, file, header) in [
    (".gitignore", "gitignore", ""), (".gitattributes", "gitattributes", ""),
    (".editorconfig", "editorconfig", "root = true\n"),
  ] {
    steps.append(
      .write(
        path,
        upsert(
          read(root, path),
          header: header,
          syntax: .hash,
          blocks: Files.blocks(file, cxx: cxx)
        )
      )
    )
  }
  let dependabot = read(root, ".github/dependabot.yml").flatMap {
    $0.contains("# >>> bedrock") ? $0 : nil
  }
  steps.append(
    .write(
      ".github/dependabot.yml",
      upsert(
        dependabot,
        header: "version: 2\nupdates:\n",
        syntax: .hash,
        blocks: Files.blocks("dependabot.yml")
      )
    )
  )
  return steps
}

/// Every change adopt makes in `root`, including ones already in place.
func plan(_ root: String, source: Source, cxx: Bool) throws -> [Step] {
  let projects = Xcode.projects(in: root)
  var steps =
    Files.base
    + Files.swift(
      cxx: cxx,
      tests: exists(root + "/Tests"),
      xcode: !projects.isEmpty
    )
  if !exists(root + "/package.json") {
    steps.append(Files.markdown)
  }
  if cxx {
    steps += Files.cpp
    steps += ["_clang-format", "_clang-tidy"].filter { exists(root + "/" + $0) }.map {
      .remove(path: $0)
    }
  }
  // A pinned release stays as the repository bumped it; --bedrock-path always takes effect.
  if source != .release || !exists(root + "/BuildTools/Package.swift") {
    steps.append(.write("BuildTools/Package.swift", source.buildTools))
  }
  steps += replaced.filter { exists(root + "/" + $0) }.map { .remove(path: $0) }
  steps += shared(root, cxx: cxx)
  if let manifest = read(root, "Package.swift") {
    steps.append(.write("Package.swift", Manifest.adopted(manifest, cxx: cxx)))
  } else if projects.isEmpty {
    throw Failure("Package.swift and an .xcodeproj are both missing: run swift-bedrock new instead")
  }
  for project in projects {
    let path = project + "/project.pbxproj"
    if let wired = read(root, path).flatMap(Xcode.wired) {
      steps.append(.write(path, wired))
    }
  }
  return steps
}

/// Rewrites code for the upcoming features migrate knows.
///
/// It then puts the adopted manifest back, since migrate also edits it, and returns
/// what is left for a person.
func migrate(_ root: String) throws -> [String] {
  guard let adopted = read(root, "Package.swift") else {
    return []
  }
  let manifest = root + "/Package.swift"
  defer { try? adopted.write(toFile: manifest, atomically: true, encoding: .utf8) }
  try Manifest.forMigration(adopted).write(toFile: manifest, atomically: true, encoding: .utf8)
  let features = Manifest.migratable.joined(separator: ",")
  do {
    try shell(
      ["swift", "package", "migrate", "--to-feature", features],
      in: root,
      capture: false
    )
  } catch {
    return [
      """
      Package.swift: swift package migrate failed, so the code still needs the upcoming \
      features; fix the build, then run swift package migrate --to-feature \(features)
      """
    ]
  }
  return []
}
