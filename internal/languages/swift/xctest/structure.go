package xctest

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reserved are words a test name cannot become once its `test` prefix is gone.
func reserved(word string) bool {
	switch word {
	case "class", "enum", "extension", kwFunc, kwImport, "init", "let", "var", "struct", "protocol",
		"static", "default", "case", "if", "else", "for", "in", "is", "as", "do", "guard", "return",
		"switch", "where", "while", kwTry, "throw", "throws", "true", "false", "nil", "self", "super",
		"repeat", "break", "continue", "defer":
		return true
	default:
		return false
	}
}

func indentOf(text string) string { return prefixWhile(text, isIndent) }

func words(text, separators string) []string {
	return strings.FieldsFunc(text, func(c rune) bool { return strings.ContainsRune(separators, c) })
}

func hasWord(text, separators string, wanted ...string) bool {
	for _, word := range words(text, separators) {
		if slices.Contains(wanted, word) {
			return true
		}
	}

	return false
}

// declaration is a rewritten class declaration with the class's name.
type declaration struct {
	text string
	name string
}

// rewriteClass returns the rewritten declaration and class name of
// `class X: XCTestCase`.
func rewriteClass(text string) (declaration, bool) {
	headEnd := len(text)
	if brace := strings.IndexByte(text, '{'); brace != -1 {
		headEnd = brace
	}

	head := strings.TrimSpace(text[:headEnd])

	keyword := strings.Index(head, "class ")
	if keyword == -1 || (keyword > 0 && head[keyword-1] != ' ') {
		return declaration{}, false
	}

	afterKeyword := head[keyword+len("class "):]
	name := prefixWhile(afterKeyword, isWord)
	after := strings.TrimSpace(afterKeyword[len(name):])

	if !strings.HasPrefix(after, ":") {
		return declaration{}, false
	}

	var bases []string

	found := false

	for base := range strings.SplitSeq(after[1:], ",") {
		base = strings.TrimSpace(base)

		switch {
		case base == "":
		case !found && (base == "XCTestCase" || base == "XCTest.XCTestCase"):
			found = true
		default:
			bases = append(bases, base)
		}
	}

	if !found {
		return declaration{}, false
	}

	modifiers := head[:keyword]

	final := "final "
	if hasWord(modifiers, " ", "final", "open") {
		final = ""
	}

	inheritance := ""
	if len(bases) > 0 {
		inheritance = ": " + strings.Join(bases, ", ")
	}

	tail := text[headEnd:]

	gap := " "
	if tail == "" {
		gap = ""
		if strings.HasSuffix(text, "\r") {
			gap = "\r"
		}
	}

	rewritten := indentOf(text) + modifiers + final + "class " + name + inheritance + gap + tail

	return declaration{text: rewritten, name: name}, true
}

func isExtension(text string, names map[string]bool) bool {
	head, _, _ := strings.Cut(text, "{")

	parts := words(head, " \t")
	if len(parts) < 2 || parts[0] != "extension" {
		return false
	}

	return names[prefixWhile(parts[1], isWord)]
}

// testName returns the name of a `func testX()` declaration that XCTest
// would run.
func testName(text string) (string, bool) {
	found := strings.Index(text, "func test")
	if found == -1 || hasWord(text[:found], " ", "override", "private", "fileprivate", "static", "class") {
		return "", false
	}

	afterFunc := text[found+len("func "):]
	name := prefixWhile(afterFunc, isWord)

	if !strings.HasPrefix(afterFunc[len(name):], "()") {
		return "", false
	}

	return name, true
}

// deprefixed returns the name without its `test` prefix, or false when
// that is not a valid name.
func deprefixed(name string) (string, bool) {
	tail := strings.TrimLeft(name[len("test"):], "_")

	first, size := utf8.DecodeRuneInString(tail)
	if size == 0 || !unicode.IsLetter(first) {
		return "", false
	}

	short := string(unicode.ToLower(first)) + tail[size:]

	return short, !reserved(short)
}

// usedNames returns the names a test could collide with: declared members
// and called functions.
func usedNames(source string) map[string]bool {
	b := toScalars(source)
	tokens := b.identifiers()
	used := map[string]bool{}

	for k, tok := range tokens {
		declared := false

		if k > 0 {
			previous := tokens[k-1]
			switch previous.name {
			case kwFunc, "var", "let", "case", "class", "struct", "enum", "typealias":
				declared = allSpace(b[previous.end:tok.start])
			}
		}

		last, hasLast := b.behind(tok.start)
		called := strings.HasPrefix(b.ahead(tok.end), "(") && (!hasLast || last != '.')

		if declared || called {
			used[tok.name] = true
		}
	}

	return used
}

func allSpace(b scalars) bool {
	for _, c := range b {
		if !isSpace(c) {
			return false
		}
	}

	return true
}

// renames says how each test is named after conversion: without `test`
// unless that collides.
func renames(source string, lines []line) map[string]string {
	used := usedNames(source)

	var tests []string

	seen := map[string]bool{}

	for _, l := range lines {
		if !l.code {
			continue
		}

		if name, ok := testName(l.text); ok && !seen[name] {
			seen[name] = true

			tests = append(tests, name)
		}
	}

	counts := map[string]int{}

	for _, name := range tests {
		if short, ok := deprefixed(name); ok {
			counts[short]++
		}
	}

	plan := make(map[string]string, len(tests))

	for _, name := range tests {
		short, valid := deprefixed(name)
		if valid && counts[short] == 1 && !used[short] {
			plan[name] = short
		} else {
			plan[name] = name
		}
	}

	return plan
}

func testLine(text, name, renamed, cr string) []string {
	return []string{
		indentOf(text) + "@Test" + cr,
		strings.Replace(text, "func "+name+"(", "func "+renamed+"(", 1),
	}
}

func classNamesOf(lines []line) map[string]bool {
	names := map[string]bool{}

	for _, l := range lines {
		if !l.code {
			continue
		}

		if class, ok := rewriteClass(l.text); ok {
			names[class.name] = true
		}
	}

	return names
}

// enter drops the containers the line has left and marks the current one as
// entered.
func enter(stack []*container, l line) ([]*container, *container) {
	for len(stack) > 0 && stack[len(stack)-1].seenBody && l.depth < stack[len(stack)-1].bodyDepth {
		stack = stack[:len(stack)-1]
	}

	if len(stack) == 0 {
		return stack, nil
	}

	top := stack[len(stack)-1]
	if l.depth >= top.bodyDepth {
		top.seenBody = true
	}

	return stack, top
}

// rewriteMember rewrites a member line of a test container.
func rewriteMember(text string, top *container, plan map[string]string, cr string) []string {
	if name, ok := testName(text); ok {
		if renamed, planned := plan[name]; planned {
			return testLine(text, name, renamed, cr)
		}
	}

	if rewritten, ok := rewriteLifecycle(text, top); ok {
		return []string{rewritten}
	}

	return []string{text}
}

// restructure rewrites test classes into plain classes with `@Test`
// functions and `init`/`deinit`.
func restructure(source string) string {
	lines := parseLines(source)

	cr := ""
	if strings.Contains(source, "\r") {
		cr = "\r"
	}

	classNames := classNamesOf(lines)
	plan := renames(source, lines)

	var (
		stack []*container
		out   []string
	)

	for _, l := range lines {
		if !l.code || (len(stack) > 0 && isSuperCall(l.text)) {
			if !l.code {
				out = append(out, l.text)
			}

			continue
		}

		var top *container

		stack, top = enter(stack, l)
		class, isClass := rewriteClass(l.text)
		converted := class.text

		switch {
		case isClass || isExtension(l.text, classNames):
			if !isClass {
				converted = l.text
			}

			out = append(out, converted)
			stack = append(stack, &container{bodyDepth: l.depth + 1, seenBody: l.opens > 0})
		case top != nil && l.depth == top.bodyDepth:
			out = append(out, rewriteMember(l.text, top, plan, cr)...)
		default:
			out = append(out, l.text)
		}
	}

	return strings.Join(out, "\n")
}
