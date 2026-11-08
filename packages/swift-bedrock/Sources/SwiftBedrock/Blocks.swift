/// A marked block in a file that several bedrocks write to, such as AGENTS.md.
struct Block: Equatable {
  let name: String
  let content: String
}

/// How a file marks the start and end of a block.
enum Syntax {
  case markdown
  case hash
  case slashes

  func open(_ name: String) -> String {
    switch self {
    case .markdown: "<!-- \(name) -->"
    case .hash: "# >>> \(name)"
    case .slashes: "// >>> \(name)"
    }
  }

  func close(_ name: String) -> String {
    switch self {
    case .markdown: "<!-- /\(name) -->"
    case .hash: "# <<< \(name)"
    case .slashes: "// <<< \(name)"
    }
  }
}

/// Every bedrock's block, in the order a file lists them, so a file comes out the
/// same whichever bedrock adopts first.
private let order = [
  "bedrock", "typescript-bedrock", "rust-bedrock", "swift-bedrock", "swift-bedrock-cxx",
  "jvm-bedrock",
]

/// Returns `current` with each block replaced or, when missing, added in bedrock order.
///
/// A missing file starts as `header`.
func upsert(_ current: String?, header: String, syntax: Syntax, blocks: [Block]) -> String {
  var lines = (current ?? header).split(separator: "\n", omittingEmptySubsequences: false)
    .map(String.init)
  for block in blocks {
    let open = syntax.open(block.name)
    let close = syntax.close(block.name)
    var body = block.content.split(separator: "\n", omittingEmptySubsequences: false)
      .map(String.init)
    if body.last?.isEmpty == true {
      body.removeLast()
    }
    let marked = [open] + body + [close]
    if let start = lines.firstIndex(of: open),
      let end = lines[start...].firstIndex(of: close)
    {
      lines.replaceSubrange(start...end, with: marked)
      continue
    }
    let rank = order.firstIndex(of: block.name) ?? order.count
    let later = order.dropFirst(rank + 1).map(syntax.open)
    if let next = lines.firstIndex(where: later.contains) {
      lines.insert(contentsOf: marked + [""], at: next)
      continue
    }
    while lines.last?.isEmpty == true {
      lines.removeLast()
    }
    lines += (lines.isEmpty ? [] : [""]) + marked + [""]
  }
  return lines.joined(separator: "\n")
}
