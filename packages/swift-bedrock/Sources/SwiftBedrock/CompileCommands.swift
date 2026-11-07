import Foundation

/// Writes compile_commands.json for clang-tidy and clangd, which SwiftPM does not
/// emit: one entry per C, C++, and Objective-C source in each Clang target, then
/// the entries of each xmake project.
func writeCompileCommands(_ root: String) throws {
  let describe = try json(shell(["swift", "package", "describe", "--type", "json"], in: root))
  let info = try json(shell(["swift", "-print-target-info"], in: root))
  let resources = (info["paths"] as? [String: Any])?["runtimeResourcePath"] as? String ?? ""
  // <swift/bridging> lives in the toolchain's usr/include, beside usr/lib/swift.
  let toolchainInclude =
    URL(fileURLWithPath: resources).appendingPathComponent("../../include")
    .standardizedFileURL.path
  let sdk = (try? shell(["xcrun", "--show-sdk-path"], in: root))?
    .trimmingCharacters(in: .whitespacesAndNewlines)
  let standard = describe["cxx_language_standard"] as? String ?? "c++20"
  let targets = (describe["targets"] as? [[String: Any]] ?? []).filter {
    $0["module_type"] as? String == "ClangTarget"
  }
  let includes = targets.compactMap { $0["path"] as? String }.flatMap {
    ["-I", root + "/" + $0 + "/include"]
  }
  var common = ["-isystem", toolchainInclude, "-mmacosx-version-min=12.0"] + includes
  if let sdk, !sdk.isEmpty {
    common = ["-isysroot", sdk] + common
  }
  var entries: [[String: Any]] = []
  for target in targets {
    let path = target["path"] as? String ?? ""
    for source in target["sources"] as? [String] ?? [] {
      let file = root + "/" + path + "/" + source
      let language: [String] =
        switch URL(fileURLWithPath: source).pathExtension {
        case "c": ["clang", "-std=c17"]
        case "m": ["clang", "-fobjc-arc"]
        case "mm": ["clang++", "-std=\(standard)", "-fobjc-arc"]
        default: ["clang++", "-std=\(standard)"]
        }
      entries.append([
        "directory": root, "file": file, "arguments": language + common + ["-c", file],
      ])
    }
  }
  entries += try xmakeEntries(root)
  let data = try JSONSerialization.data(
    withJSONObject: entries,
    options: [.prettyPrinted, .sortedKeys]
  )
  try data.write(to: URL(fileURLWithPath: root + "/compile_commands.json"))
}

/// The entries xmake writes for each tracked xmake project.
///
/// A nested xmake.lua is skipped, since its parent project `includes` it.
private func xmakeEntries(_ root: String) throws -> [[String: Any]] {
  let projects = try trackedFiles(in: root, ["xmake.lua", "*/xmake.lua"]).map {
    $0.split(separator: "/").dropLast().joined(separator: "/")
  }
  let tops = projects.filter { project in
    !projects.contains { $0 != project && ($0.isEmpty || project.hasPrefix($0 + "/")) }
  }
  let output = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
  defer { try? FileManager.default.removeItem(at: output) }
  return try tops.flatMap { project in
    try shell(
      [
        "xmake", "project", "-q", "-y", "-P", root + "/" + project, "-k", "compile_commands",
        output.path,
      ],
      in: root
    )
    let data = try Data(contentsOf: output.appendingPathComponent("compile_commands.json"))
    return try JSONSerialization.jsonObject(with: data) as? [[String: Any]] ?? []
  }
}

private func json(_ text: String) throws -> [String: Any] {
  guard let object = try JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any]
  else {
    throw Failure("expected a JSON object")
  }
  return object
}
