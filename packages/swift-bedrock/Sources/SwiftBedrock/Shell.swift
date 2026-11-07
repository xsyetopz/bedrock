import Foundation

/// A failure to report to the user, as it reads.
struct Failure: Error, CustomStringConvertible {
  let description: String

  init(_ description: String) {
    self.description = description
  }
}

/// Runs a program found on PATH in `directory` and returns its standard output.
///
/// Standard error goes to the terminal.
@discardableResult
func shell(_ arguments: [String], in directory: String, capture: Bool = true) throws -> String {
  let process = Process()
  process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
  process.arguments = arguments
  process.currentDirectoryURL = URL(fileURLWithPath: directory)
  let output = Pipe()
  if capture {
    process.standardOutput = output
  }
  try process.run()
  let data = capture ? output.fileHandleForReading.readDataToEndOfFile() : Data()
  process.waitUntilExit()
  guard process.terminationStatus == 0 else {
    throw Failure("\(arguments.joined(separator: " ")) failed")
  }
  return String(bytes: data, encoding: .utf8) ?? ""
}

func read(_ root: String, _ path: String) -> String? {
  try? String(contentsOfFile: root + "/" + path, encoding: .utf8)
}

/// The files git tracks or would track that match `patterns`.
func trackedFiles(in root: String, _ patterns: [String]) throws -> [String] {
  let output = try shell(
    ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"] + patterns,
    in: root
  )
  return output.split(separator: "\0").map(String.init).filter {
    FileManager.default.fileExists(atPath: root + "/" + $0)
  }
}
