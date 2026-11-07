import Foundation

/// A change adopt makes, which it can print instead of applying.
enum Step: Equatable {
  case write(path: String, content: String, executable: Bool)
  case remove(path: String)
  case link(path: String, target: String)

  static func write(_ path: String, _ content: String) -> Self {
    .write(path: path, content: content, executable: false)
  }

  var path: String {
    switch self {
    case .write(let path, _, _), .remove(let path), .link(let path, _): path
    }
  }

  func describe(in root: String) -> String {
    switch self {
    case .write(let path, _, _) where exists(root + "/" + path):
      "update  \(path) (kept as \(path).bak)"
    case .write(let path, _, _): "create  \(path)"
    case .remove(let path): "remove  \(path) (kept as \(path).bak)"
    case .link(let path, let target): "link    \(path) -> \(target)"
    }
  }

  /// Whether `root` already matches this step.
  func isDone(in root: String) -> Bool {
    let full = root + "/" + path
    let files = FileManager.default
    switch self {
    case .write(_, let content, let executable):
      return (try? String(contentsOfFile: full, encoding: .utf8)) == content
        && (!executable || files.isExecutableFile(atPath: full))
    case .remove: return !exists(full)
    case .link(_, let target): return (try? files.destinationOfSymbolicLink(atPath: full)) == target
    }
  }

  /// Applies the step, keeping a file it replaces or removes as `<file>.bak`.
  ///
  /// An existing backup stays, since it holds the file from before the first adopt.
  func apply(in root: String) throws {
    let files = FileManager.default
    let full = root + "/" + path
    if exists(full) {
      let backup = full + ".bak"
      if exists(backup) {
        try files.removeItem(atPath: full)
      } else {
        try files.moveItem(atPath: full, toPath: backup)
      }
    }
    try files.createDirectory(
      atPath: URL(fileURLWithPath: full).deletingLastPathComponent().path,
      withIntermediateDirectories: true
    )
    switch self {
    case .write(_, let content, let executable):
      try content.write(toFile: full, atomically: true, encoding: .utf8)
      if executable {
        try files.setAttributes([.posixPermissions: 0o755], ofItemAtPath: full)
      }
    case .remove: break
    case .link(_, let target):
      try files.createSymbolicLink(atPath: full, withDestinationPath: target)
    }
  }
}

/// Whether anything, a dangling symlink included, sits at `path`.
func exists(_ path: String) -> Bool {
  (try? FileManager.default.attributesOfItem(atPath: path)) != nil
}

/// Drops the steps `root` already matches.
func pending(_ steps: [Step], in root: String) -> [Step] {
  steps.filter { !$0.isDone(in: root) }
}
