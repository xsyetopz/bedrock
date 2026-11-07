import Foundation

/// Rewrites an XCTest file into Swift Testing.
enum XCTestConverter {
  /// Returns the converted text, or nil when `source` does not import XCTest, plus a
  /// note for each construct left for a person, such as `measure`.
  static func convert(_ source: String) -> (text: String, notes: [String])? {
    guard parseLines(source).contains(where: { imports($0, "XCTest") }) else { return nil }
    var text = restructure(rewriteAssertions(source))
    let keep = leftovers(text).contains { $0.needsXCTest }
    text = rewriteImports(text, keepXCTest: keep)
    var notes = leftovers(text).map { ($0.line, $0.message) }
    if keep, let index = parseLines(text).firstIndex(where: { imports($0, "XCTest") }) {
      notes.append((index + 1, "import XCTest kept for the constructs noted here"))
    }
    return (text, notes.sorted { $0.0 < $1.0 }.map { "line \($0.0): \($0.1)" })
  }
}

// MARK: - Lexing

extension XCTestConverter {
  fileprivate typealias Scalars = [Unicode.Scalar]
  fileprivate typealias Replacement = (text: String, end: Int)
  fileprivate typealias Leftover = (line: Int, message: String, needsXCTest: Bool)
  fileprivate typealias Line = (text: String, code: Bool, depth: Int, opens: Int)

  fileprivate static func string(_ scalars: some Sequence<Unicode.Scalar>) -> String {
    String(String.UnicodeScalarView(scalars))
  }

  fileprivate static func has(_ c: Unicode.Scalar, _ set: String) -> Bool {
    set.unicodeScalars.contains(c)
  }

  fileprivate static func isIdent(_ c: Unicode.Scalar) -> Bool {
    c == "_" || c == "$" || ("0"..."9").contains(c) || c.properties.isAlphabetic
  }

  fileprivate static func isIdent(_ c: Character) -> Bool { c == "_" || c.isLetter || c.isNumber }

  /// The index just past the comment or string literal that starts at `i`, if one does.
  fileprivate static func trivia(_ b: Scalars, _ i: Int) -> Int? {
    let n = b.count
    func at(_ k: Int, _ s: String) -> Bool { k < n && b[k...].starts(with: s.unicodeScalars) }
    if at(i, "//") { return b[i...].firstIndex(of: "\n") ?? n }
    if at(i, "/*") {
      var (j, depth) = (i + 2, 1)
      while j < n, depth > 0 {
        let step = at(j, "/*") ? 1 : at(j, "*/") ? -1 : 0
        depth += step
        j += step == 0 ? 1 : 2
      }
      return j
    }
    var q = i
    while q < n, b[q] == "#" { q += 1 }
    guard at(q, "\"") else { return nil }
    let hashes = String(repeating: "#", count: q - i)
    let quotes = at(q, "\"\"\"") ? "\"\"\"" : "\""
    var j = q + quotes.count
    while j < n {
      if at(j, quotes + hashes) { return j + quotes.count + hashes.count }
      if quotes.count == 1, b[j] == "\n" { return j }
      if at(j, "\\" + hashes) {
        j += 1 + hashes.count
        if at(j, "(") { j = matchClose(b, j) ?? n }
      }
      j += 1
    }
    return n
  }

  /// The index of the bracket that closes the one at `open`.
  fileprivate static func matchClose(_ b: Scalars, _ open: Int) -> Int? {
    var depth = 0
    var i = open
    while i < b.count {
      if let j = trivia(b, i) {
        i = j
        continue
      }
      depth += has(b[i], "([{") ? 1 : has(b[i], ")]}") ? -1 : 0
      if depth == 0 { return i }
      i += 1
    }
    return nil
  }

  fileprivate static func codeMask(_ b: Scalars) -> [Bool] {
    var mask = [Bool](repeating: true, count: b.count)
    var i = 0
    while i < b.count {
      let end = trivia(b, i)
      for k in i..<(end ?? i) { mask[k] = false }
      i = end ?? i + 1
    }
    return mask
  }

  /// Indices outside strings, comments and brackets, or nil when a comment is in the way.
  fileprivate static func topLevel(_ b: Scalars) -> [Int]? {
    var found: [Int] = []
    var depth = 0
    var i = 0
    while i < b.count {
      if let j = trivia(b, i) {
        if b[i] == "/" { return nil }
        i = j
        continue
      }
      // The commas of a generic argument list must not split arguments.
      if depth == 0, b[i] == "<", i > 0, isIdent(b[i - 1]),
        let j = (i..<b.count).first(where: { b[$0] == ">" }),
        b[i..<j].allSatisfy({ isIdent($0) || $0.properties.isWhitespace || has($0, "<,.?[]():&") })
      {
        i = j + 1
        continue
      }
      depth -= has(b[i], ")]}") ? 1 : 0
      if depth == 0 { found.append(i) }
      depth += has(b[i], "([{") ? 1 : 0
      i += 1
    }
    return found
  }

  fileprivate static func identifiers(_ b: Scalars) -> [(name: String, start: Int, end: Int)] {
    let mask = codeMask(b)
    var tokens: [(name: String, start: Int, end: Int)] = []
    var i = 0
    while i < b.count {
      guard mask[i], isIdent(b[i]) else {
        i += 1
        continue
      }
      let end = (i..<b.count).first { !isIdent(b[$0]) } ?? b.count
      if !("0"..."9").contains(b[i]) { tokens.append((string(b[i..<end]), i, end)) }
      i = end
    }
    return tokens
  }

  /// The text after `i`, without leading whitespace and cut short.
  fileprivate static func ahead(_ b: Scalars, _ i: Int) -> String {
    string(b[i...].drop { $0.properties.isWhitespace }.prefix(8))
  }

  fileprivate static func behind(_ b: Scalars, _ i: Int) -> Unicode.Scalar? {
    b[..<i].last { !$0.properties.isWhitespace }
  }

  fileprivate static func parseLines(_ s: String) -> [Line] {
    let b = Array(s.unicodeScalars)
    let mask = codeMask(b)
    var lines: [Line] = []
    var depth = 0
    var start = 0
    while start <= b.count {
      let end = b[start...].firstIndex(of: "\n") ?? b.count
      let first = (start..<end).first { !has(b[$0], " \t\r") }
      let braces = (start..<end).filter { mask[$0] && has(b[$0], "{}") }.map { b[$0] }
      let opens = braces.filter { $0 == "{" }.count
      lines.append((string(b[start..<end]), first.map { mask[$0] } ?? false, depth, opens))
      depth = max(0, depth + 2 * opens - braces.count)
      start = end + 1
    }
    return lines
  }
}

// MARK: - Assertions

extension XCTestConverter {
  fileprivate enum Assertion {
    case truth(negated: Bool)
    case compare(String)
    case nilCheck(String)
    case unwrap, fail
    case closure(String)
  }

  fileprivate static let assertions: [String: Assertion] = [
    "XCTAssert": .truth(negated: false), "XCTAssertTrue": .truth(negated: false),
    "XCTAssertFalse": .truth(negated: true), "XCTAssertEqual": .compare("=="),
    "XCTAssertNotEqual": .compare("!="), "XCTAssertGreaterThan": .compare(">"),
    "XCTAssertGreaterThanOrEqual": .compare(">="), "XCTAssertLessThan": .compare("<"),
    "XCTAssertLessThanOrEqual": .compare("<="), "XCTAssertIdentical": .compare("==="),
    "XCTAssertNotIdentical": .compare("!=="), "XCTAssertNil": .nilCheck("=="),
    "XCTAssertNotNil": .nilCheck("!="), "XCTUnwrap": .unwrap, "XCTFail": .fail,
    "XCTAssertThrowsError": .closure("(any Error).self"),
    "XCTAssertNoThrow": .closure("Never.self"),
  ]

  fileprivate static func rewriteAssertions(_ s: String) -> String {
    let b = Array(s.unicodeScalars)
    var out = String.UnicodeScalarView()
    var i = 0
    while i < b.count {
      var j = trivia(b, i) ?? i
      if j == i, isIdent(b[i]) { j = (i..<b.count).first { !isIdent(b[$0]) } ?? b.count }
      let open = b[max(j, i + 1)...].firstIndex { !$0.properties.isWhitespace }
      if j > i, let kind = assertions[string(b[i..<j])], let open, b[open] == "(",
        let (replacement, end) = convertCall(kind, b, open)
      {
        out += replacement.unicodeScalars
        i = end
      } else {
        j = max(j, i + 1)
        out += b[i..<j]
        i = j
      }
    }
    return String(out)
  }

  /// The replacement text and the index after the call whose `(` is at `open`.
  fileprivate static func convertCall(_ k: Assertion, _ b: Scalars, _ open: Int) -> Replacement? {
    guard let close = matchClose(b, open), let parts = split(Array(b[open + 1..<close])) else {
      return nil
    }
    // A trailing closure is an error handler; those calls stay with a person.
    if b[(close + 1)...].first(where: { !$0.properties.isWhitespace }) == "{" { return nil }
    var positional: [String] = []
    var accuracy: String?
    for part in parts.map(rewriteAssertions) {
      switch label(part) {
      case ("file", _)?, ("line", _)?: continue
      case ("accuracy", let value)?: accuracy = value
      default: positional.append(part)
      }
    }
    var effects = Set<String>()
    func operand(_ index: Int, wrapped: Bool = true) -> String {
      var text = positional[index]
      while let word = ["try", "await"].first(where: { text.hasPrefix($0 + " ") }) {
        effects.insert(word)
        text = text.dropFirst(word.count).trimmingCharacters(in: .whitespacesAndNewlines)
      }
      return wrapped ? wrap(text) : text
    }
    func comment(_ index: Int) -> String {
      let text = positional[index]
      let scalars = Array(text.unicodeScalars)
      return trivia(scalars, 0) == scalars.count && scalars.first != "/"
        ? text : "Comment(rawValue: \(text))"
    }
    func call(_ h: String, _ n: Int, _ t: String = ")", _ f: () -> String) -> String? {
      guard positional.count == n || positional.count == n + 1 else { return nil }
      let first = f()
      let lead = ["try", "await"].filter(effects.contains).map { $0 + " " }.joined()
      let message = positional.count > n ? comment(n) : ""
      return h + lead + [first, message].filter { !$0.isEmpty }.joined(separator: ", ") + t
    }
    let text: String? =
      switch k {
      case .truth(let negated):
        call("#expect(", 1) {
          negated ? "!" + operand(0) : operand(0, wrapped: false)
        }
      case .nilCheck(let op): call("#expect(", 1) { "\(operand(0)) \(op) nil" }
      case .unwrap: call("#require(", 1) { positional[0] }
      case .fail: call("Issue.record(", 0) { "" }
      case .closure(let type):
        call("#expect(", 1, ") { \(positional.first ?? "") }") { "throws: \(type)" }
      case .compare(let op) where accuracy != nil && op != "==" && op != "!=": nil
      case .compare(let op):
        call("#expect(", 2) {
          guard let accuracy else { return "\(operand(0)) \(op) \(operand(1))" }
          let bound = "\(op == "==" ? "<=" : ">") \(wrap(accuracy))"
          return "abs(\(operand(0)) - \(operand(1))) \(bound)"
        }
      }
    return text.map { ($0, close + 1) }
  }

  /// The file:, line: or accuracy: label that starts an argument.
  fileprivate static func label(_ part: String) -> (String, String)? {
    let name = part.prefix { isIdent($0) }
    let rest = part.dropFirst(name.count).drop { $0 == " " }
    guard ["file", "line", "accuracy"].contains(name), rest.first == ":", !rest.hasPrefix("::")
    else { return nil }
    return (String(name), rest.dropFirst().trimmingCharacters(in: .whitespacesAndNewlines))
  }

  /// The top-level, trimmed arguments, or nil when a comment sits inside them.
  fileprivate static func split(_ b: Scalars) -> [String]? {
    guard let tops = topLevel(b) else { return nil }
    let cuts = [-1] + tops.filter { b[$0] == "," } + [b.count]
    var parts = zip(cuts, cuts.dropFirst()).map {
      string(b[$0 + 1..<$1]).trimmingCharacters(in: .whitespacesAndNewlines)
    }
    if parts.last?.isEmpty == true { parts.removeLast() }
    return parts
  }

  fileprivate static func wrap(_ s: String) -> String { needsParens(s) ? "(\(s))" : s }

  fileprivate static func needsParens(_ s: String) -> Bool {
    let b = Array(s.unicodeScalars)
    return (topLevel(b) ?? []).contains { i in
      let next = i + 1 < b.count ? b[i + 1] : " "
      switch b[i] {
      case "-": return i != 0
      case "!": return i != 0 && next == "="
      case "?": return !has(next, ".([)") && i + 1 != b.count
      case ".": return next == "." || (i > 0 && b[i - 1] == ".")
      case "+", "*", "/", "%", "<", ">", "=", "&", "|", "^", "~", ":", ",", ";": return true
      default: return b[i].properties.isWhitespace
      }
    }
  }
}

// MARK: - Structure

extension XCTestConverter {
  fileprivate typealias Container = (bodyDepth: Int, seenBody: Bool, hasInit: Bool, hasDeinit: Bool)

  fileprivate static let keywords = Set(
    """
    class enum extension func import init let var struct protocol static default case if else \
    for in is as do guard return switch where while try throw throws true false nil self super \
    repeat break continue defer
    """.split(separator: " ").map(String.init)
  )

  fileprivate static func restructure(_ s: String) -> String {
    let lines = parseLines(s)
    let cr = s.unicodeScalars.contains("\r") ? "\r" : ""
    let classNames = Set(lines.compactMap { $0.code ? rewriteClass($0.text)?.name : nil })
    let used = usedNames(s)
    let tests = Set(lines.compactMap { $0.code ? testName($0.text) : nil })
    let plan = Dictionary(
      uniqueKeysWithValues: tests.map { name in
        let short = deprefixed(name)
        let unique = short.map { s in
          tests.filter { deprefixed($0) == s }.count == 1 && !used.contains(s)
        }
        return (name, unique == true ? short ?? name : name)
      }
    )
    var stack: [Container] = []
    var out: [String] = []
    for line in lines {
      let text = line.text
      if line.code, !stack.isEmpty, isSuperCall(text) { continue }
      guard line.code else {
        out.append(text)
        continue
      }
      while let top = stack.last, top.seenBody, line.depth < top.bodyDepth { stack.removeLast() }
      if let last = stack.indices.last, line.depth >= stack[last].bodyDepth {
        stack[last].seenBody = true
      }
      var emitted = [text]
      let converted = rewriteClass(text)?.text
      if converted != nil || isExtension(text, of: classNames) {
        emitted = [converted ?? text]
        stack.append((line.depth + 1, line.opens > 0, false, false))
      } else if let last = stack.indices.last, line.depth == stack[last].bodyDepth {
        if let name = testName(text), let new = plan[name] {
          let indent = String(text.prefix { $0 == " " || $0 == "\t" })
          emitted = [
            indent + "@Test" + cr,
            text.replacingOccurrences(of: "func \(name)(", with: "func \(new)("),
          ]
        } else if let new = rewriteLifecycle(text, &stack[last]) {
          emitted = [new]
        }
      }
      out += emitted
    }
    return out.joined(separator: "\n")
  }

  /// The rewritten declaration and class name of `class X: XCTestCase`.
  fileprivate static func rewriteClass(_ line: String) -> (text: String, name: String)? {
    let headEnd = line.firstIndex(of: "{") ?? line.endIndex
    let head = line[..<headEnd].trimmingCharacters(in: .whitespacesAndNewlines)
    guard let keyword = head.range(of: "class "),
      keyword.lowerBound == head.startIndex || head[head.index(before: keyword.lowerBound)] == " "
    else { return nil }
    let rest = head[keyword.upperBound...]
    let name = String(rest.prefix { isIdent($0) })
    let after = rest.dropFirst(name.count).trimmingCharacters(in: .whitespacesAndNewlines)
    var bases = after.dropFirst().split(separator: ",").map {
      $0.trimmingCharacters(in: .whitespacesAndNewlines)
    }
    guard after.hasPrefix(":"),
      let found = bases.firstIndex(where: { $0 == "XCTestCase" || $0 == "XCTest.XCTestCase" })
    else { return nil }
    bases.remove(at: found)
    let modifiers = String(head[..<keyword.lowerBound])
    let isFinal = modifiers.split(separator: " ").contains { $0 == "final" || $0 == "open" }
    let indent = line.prefix { $0 == " " || $0 == "\t" }
    let inheritance = bases.isEmpty ? "" : ": " + bases.joined(separator: ", ")
    let tail = line[headEnd...]
    let gap = tail.isEmpty ? (line.hasSuffix("\r") ? "\r" : "") : " "
    let text =
      "\(indent)\(modifiers)\(isFinal ? "" : "final ")class \(name)\(inheritance)\(gap)\(tail)"
    return (text, name)
  }

  fileprivate static func isExtension(_ line: String, of names: Set<String>) -> Bool {
    let words = line.prefix { $0 != "{" }.split { $0 == " " || $0 == "\t" }
    return words.count >= 2 && words[0] == "extension"
      && names.contains(String(words[1].prefix { isIdent($0) }))
  }

  /// The name of a `func testX()` declaration that XCTest would run.
  fileprivate static func testName(_ line: String) -> String? {
    guard let found = line.range(of: "func test") else { return nil }
    let skipped: Set<Substring> = ["override", "private", "fileprivate", "static", "class"]
    guard line[..<found.lowerBound].split(separator: " ").allSatisfy({ !skipped.contains($0) })
    else { return nil }
    let rest = line[line.index(found.lowerBound, offsetBy: 5)...]
    let name = rest.prefix { isIdent($0) }
    return rest.dropFirst(name.count).hasPrefix("()") ? String(name) : nil
  }

  fileprivate static func deprefixed(_ name: String) -> String? {
    let rest = name.dropFirst(4).drop { $0 == "_" }
    guard let first = rest.first, first.isLetter else { return nil }
    let short = first.lowercased() + rest.dropFirst()
    return keywords.contains(short) ? nil : short
  }

  /// Names a test could collide with: declared members and called functions.
  fileprivate static func usedNames(_ s: String) -> Set<String> {
    let b = Array(s.unicodeScalars)
    let tokens = identifiers(b)
    let declarers: Set = ["func", "var", "let", "case", "class", "struct", "enum", "typealias"]
    var used = Set<String>()
    for (k, token) in tokens.enumerated() {
      let declared =
        k > 0 && declarers.contains(tokens[k - 1].name)
        && b[tokens[k - 1].end..<token.start].allSatisfy(\.properties.isWhitespace)
      let called = ahead(b, token.end).hasPrefix("(") && behind(b, token.start) != "."
      if declared || called { used.insert(token.name) }
    }
    return used
  }

  /// `override func setUp…()` becomes `init()`, and `tearDown…()` becomes `deinit`.
  fileprivate static func rewriteLifecycle(_ line: String, _ c: inout Container) -> String? {
    guard let found = line.range(of: "override func ") else { return nil }
    let rest = line[found.upperBound...]
    let name = String(rest.prefix { isIdent($0) })
    let effects = rest.dropFirst(name.count + 2).prefix { $0 != "{" }
    let isSetUp = name == "setUp" || name == "setUpWithError"
    guard isSetUp || name == "tearDown" || name == "tearDownWithError",
      rest.dropFirst(name.count).hasPrefix("()")
    else { return nil }
    if isSetUp ? c.hasInit : c.hasDeinit { return nil }
    if !isSetUp {
      // deinit can neither throw nor be async, and takes no attributes.
      guard !effects.contains("async"), !effects.contains("throws"),
        line[..<found.lowerBound].allSatisfy({ $0 == " " || $0 == "\t" })
      else { return nil }
    }
    c.hasInit = c.hasInit || isSetUp
    c.hasDeinit = c.hasDeinit || !isSetUp
    let end = line.index(rest.startIndex, offsetBy: name.count + (isSetUp ? 0 : 2))
    return line.replacingCharacters(in: found.lowerBound..<end, with: isSetUp ? "init" : "deinit")
  }

  fileprivate static func isSuperCall(_ line: String) -> Bool {
    let calls = ["setUp", "setUpWithError", "tearDown", "tearDownWithError"].map { "super.\($0)()" }
    let trimmed = line.trimmingCharacters(in: CharacterSet(charactersIn: "; \t\r"))
    var words = trimmed.split(separator: " ").map(String.init)
    while let first = words.first, first == "try" || first == "await" { words.removeFirst() }
    return words.count == 1 && calls.contains(words[0])
  }
}

// MARK: - Imports and leftovers

extension XCTestConverter {
  fileprivate static func imports(_ line: Line, _ module: String) -> Bool {
    let code = line.code ? line.text.components(separatedBy: "//")[0] : ""
    let words = code.split { " \t\r".contains($0) }.map(String.init)
    return words.suffix(2) == ["import", module] && words.count < 4 && !words.contains("@testable")
  }

  fileprivate static func rewriteImports(_ s: String, keepXCTest: Bool) -> String {
    let lines = parseLines(s)
    var hasTesting = lines.contains { imports($0, "Testing") }
    var out: [String] = []
    for line in lines {
      guard imports(line, "XCTest") else {
        out.append(line.text)
        continue
      }
      if !hasTesting {
        hasTesting = true
        let indent = line.text.prefix { $0 == " " || $0 == "\t" }
        out.append("\(indent)import Testing\(line.text.hasSuffix("\r") ? "\r" : "")")
      }
      if keepXCTest { out.append(line.text) }
    }
    return out.joined(separator: "\n")
  }

  fileprivate static let confirmation = "use confirmation(...)"
  fileprivate static let manual: [String: (label: String, advice: String)] = [
    "XCTSkip": ("XCTSkip", "use the .enabled(if:) or .disabled trait"),
    "XCTSkipIf": ("XCTSkipIf", "use the .enabled(if:) or .disabled trait"),
    "XCTSkipUnless": ("XCTSkipUnless", "use the .enabled(if:) or .disabled trait"),
    "XCTestExpectation": ("XCTestExpectation", confirmation),
    "expectation": ("expectation(description:)", confirmation),
    "wait": ("wait(for:)", confirmation),
    "waitForExpectations": ("waitForExpectations", confirmation),
    "fulfill": ("fulfill()", confirmation),
    "measure": ("measure", "time the work with ContinuousClock by hand"),
    "XCUIApplication": ("XCUIApplication", "keep UI tests in an XCTest target"),
    "addTeardownBlock": ("addTeardownBlock", "use defer or deinit"),
    "continueAfterFailure": ("continueAfterFailure", "use #require to stop at the first failure"),
    "XCTAssertThrowsError": (
      "XCTAssertThrowsError with an error handler", "use #expect(throws:) and inspect the error"
    ),
  ]

  /// Constructs still left for a person, found in the converted text.
  fileprivate static func leftovers(_ s: String) -> [Leftover] {
    let b = Array(s.unicodeScalars)
    let tokens = identifiers(b)
    var found: [Leftover] = []
    var line = 1
    var position = 0
    for (k, token) in tokens.enumerated() {
      line += b[position..<token.start].filter { $0 == "\n" }.count
      position = token.start
      let name = token.name
      let previous = k > 0 ? tokens[k - 1].name : ""
      let after = ahead(b, token.end)
      let bare = behind(b, token.start) != "." || previous == "self"
      var entry = manual[name]
      if ["expectation", "wait", "measure", "fulfill"].contains(name) {
        let isCall =
          after.hasPrefix(name == "wait" ? "(for:" : "(")
          || (name == "measure" && after.hasPrefix("{"))
        if !isCall || bare != (name != "fulfill") { entry = nil }
      }
      var message = entry.map { "\($0.label) has no direct Swift Testing form; \($0.advice)" }
      var needsXCTest = true
      if message == nil, name.hasPrefix("XCT"), previous != "import" {
        message = "\(name) could not be converted; left as is"
      } else if message == nil, previous == "func",
        name.hasPrefix("setUp") || name.hasPrefix("tearDown")
      {
        message =
          "\(name) left as is; init and deinit appear once, and deinit cannot throw or be async"
        needsXCTest = false
      }
      if let message, !found.contains(where: { $0.line == line && $0.message == message }) {
        found.append((line, message, needsXCTest))
      }
    }
    return found
  }
}
