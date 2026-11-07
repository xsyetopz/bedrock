import Foundation

private let usage = """
  usage:
    swift-bedrock new <dir> [--kind library|executable|app]... [--cxx] [--name <Name>]
    swift-bedrock adopt [<dir>] [--cxx] [--dry-run] [--yes] [--no-run]
    swift-bedrock guard [--fix]
    swift-bedrock compile-commands

  new and adopt also take --bedrock-path <path>: depend on a local swift-bedrock,
  given relative to BuildTools/, instead of the tagged release.
  """

struct Options: Equatable {
  var positional: [String] = []
  var kinds: [String] = []
  var name: String?
  var bedrockPath: String?
  var cxx = false
  var dryRun = false
  var yes = false
  var noRun = false
  var fix = false

  var source: Source {
    bedrockPath.map(Source.path) ?? .release
  }
}

func parse(_ arguments: [String]) throws -> Options {
  var options = Options()
  var rest = arguments[...]
  while let argument = rest.popFirst() {
    let value = {
      guard let value = rest.popFirst() else {
        throw Failure("\(argument) needs a value")
      }
      return value
    }
    switch argument {
    case "--kind": options.kinds.append(try value())
    case "--name": options.name = try value()
    case "--bedrock-path": options.bedrockPath = try value()
    case "--cxx": options.cxx = true
    case "--dry-run": options.dryRun = true
    case "--yes": options.yes = true
    case "--no-run": options.noRun = true
    case "--fix": options.fix = true
    case let flag where flag.hasPrefix("-"): throw Failure("unknown option \(flag)")
    default: options.positional.append(argument)
    }
  }
  return options
}

/// Runs the swift-bedrock command line.
///
/// - Parameter arguments: The arguments after the program name.
/// - Returns: The exit status.
public func run(_ arguments: [String]) -> Int32 {
  do {
    guard let command = arguments.first else {
      throw Failure(usage)
    }
    let options = try parse(Array(arguments.dropFirst()))
    let here = FileManager.default.currentDirectoryPath
    switch command {
    case "new": try new(options)
    case "adopt": try adopt(options.positional.first ?? here, options, migrating: true)
    case "guard":
      let problems = try Guard.run(here, source: options.source, fix: options.fix)
      guard problems.isEmpty else {
        throw Failure(problems.joined(separator: "\n"))
      }
    case "compile-commands": try writeCompileCommands(here)
    case "help", "--help", "-h": print(usage)
    default: throw Failure("unknown command \(command)\n\(usage)")
    }
    return 0
  } catch {
    FileHandle.standardError.write(Data("\(error)\n".utf8))
    return 1
  }
}

private func confirm() -> Bool {
  print("Apply these changes? [y/N] ", terminator: "")
  return readLine()?.trimmingCharacters(in: .whitespaces).lowercased() == "y"
}

private func adopt(_ root: String, _ options: Options, migrating: Bool) throws {
  let cxx = options.cxx || usesCxx(root)
  let steps = pending(try plan(root, source: options.source, cxx: cxx), in: root)
  for step in steps {
    print(step.describe(in: root))
  }
  if options.dryRun {
    return
  }
  guard options.yes || isatty(STDIN_FILENO) != 0 && confirm() else {
    throw Failure("Nothing changed. Rerun with --yes to apply.")
  }
  for step in steps {
    try step.apply(in: root)
  }
  if !options.noRun {
    var problems = migrating ? try migrate(root) : []
    problems += try Guard.run(root, source: options.source, fix: true)
    let swiftFiles = try trackedFiles(in: root, ["*.swift"])
    if !swiftFiles.isEmpty {
      try shell(
        ["swift", "format", "format", "--in-place", "--parallel"] + swiftFiles,
        in: root,
        capture: false
      )
    }
    for problem in problems {
      print(problem)
    }
  }
  print(
    """
    Done. Run `just setup`, then `just verify`, and fix what it reports. Compare each *.bak \
    file with its replacement and delete it; move what a removed Makefile, Mintfile, or hook \
    did into project.just.
    """
  )
}

private func new(_ options: Options) throws {
  guard options.positional.count == 1, let directory = options.positional.first else {
    throw Failure("new takes one directory\n\(usage)")
  }
  if exists(directory + "/Package.swift") {
    throw Failure("\(directory) has a Package.swift already: run swift-bedrock adopt")
  }
  var kinds: Set<Kind> = [.library]
  for wanted in options.kinds {
    guard let kind = Kind(rawValue: wanted) else {
      let known = Kind.allCases.map {
        "  \($0.rawValue.padding(toLength: 11, withPad: " ", startingAt: 0))\($0.summary)"
      }
      throw Failure("unknown kind \(wanted); the kinds are:\n\(known.joined(separator: "\n"))")
    }
    kinds.insert(kind)
  }
  let folder = (URL(fileURLWithPath: directory).standardizedFileURL.lastPathComponent)
  let name = options.name ?? typeName(folder)
  guard isTypeName(name) else {
    throw Failure("\(name) is not a Swift type name: pass --name")
  }
  for step in starterFiles(name: name, kinds: kinds, cxx: options.cxx) {
    print(step.describe(in: directory))
    try step.apply(in: directory)
  }
  if !exists(directory + "/.git") {
    try shell(["git", "init", "--quiet"], in: directory)
  }
  var adopting = options
  adopting.yes = true
  try adopt(directory, adopting, migrating: false)
}
