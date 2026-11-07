import Foundation

/// What `new` starts a package with.
///
/// Each kind includes the library.
enum Kind: String, CaseIterable {
  case library
  case executable
  case app

  var summary: String {
    switch self {
    case .library: "a library with Swift Testing tests"
    case .executable: "a command-line program that uses the library"
    case .app: "a macOS SwiftUI app in an Xcode project, with XCTest UI tests"
    }
  }
}

/// `raw`, such as a directory name, as a Swift type name: `my-tool` becomes `MyTool`.
func typeName(_ raw: String) -> String {
  raw.split { !$0.isLetter && !$0.isNumber }.map { $0.prefix(1).uppercased() + $0.dropFirst() }
    .joined()
}

func isTypeName(_ name: String) -> Bool {
  guard let first = name.unicodeScalars.first, CharacterSet.letters.contains(first) else {
    return false
  }
  return name.allSatisfy { $0.isASCII && ($0.isLetter || $0.isNumber) }
}

/// The Package.swift for a new package, before the managed blocks.
func newManifest(name: String, kinds: Set<Kind>, cxx: Bool) -> String {
  var products = [#".library(name: "\#(name)", targets: ["\#(name)"])"#]
  var targets = [
    cxx
      ? #".target(name: "\#(name)", dependencies: ["\#(name)Cxx"])"#
      : #".target(name: "\#(name)")"#
  ]
  if cxx {
    targets.append(#".target(name: "\#(name)Cxx")"#)
  }
  if kinds.contains(.executable) {
    products.append(#".executable(name: "\#(name.lowercased())", targets: ["\#(name)CLI"])"#)
    targets.append(#".executableTarget(name: "\#(name)CLI", dependencies: ["\#(name)"])"#)
  }
  targets.append(#".testTarget(name: "\#(name)Tests", dependencies: ["\#(name)"])"#)
  let list = { (items: [String]) in items.map { "    \($0),\n" }.joined() }
  return """
    // swift-tools-version: 6.4
    import PackageDescription

    let package = Package(
      name: "\(name)",
      platforms: [.macOS(.v12)],
      products: [
    \(list(products))  ],
      targets: [
    \(list(targets))  ]
    )

    """
}

/// The starter files for `kinds`, with `__Name__` and `__name__` replaced.
func starterFiles(name: String, kinds: Set<Kind>, cxx: Bool) -> [Step] {
  var trees = ["library"]
  trees += Kind.allCases.filter { $0 != .library && kinds.contains($0) }.map(\.rawValue)
  if cxx {
    trees.append("cxx")
  }
  let fill = { (text: String) in
    text.replacingOccurrences(of: "__Name__", with: name)
      .replacingOccurrences(of: "__name__", with: name.lowercased())
  }
  var steps = trees.flatMap { tree in
    Files.tree("kinds/" + tree).map { Step.write(fill($0.path), fill($0.content)) }
  }
  steps.append(
    .write(
      "Package.swift",
      Manifest.adopted(newManifest(name: name, kinds: kinds, cxx: cxx), cxx: cxx)
    )
  )
  steps.append(.write("README.md", fill(Files.read("new/README.md"))))
  return steps
}
