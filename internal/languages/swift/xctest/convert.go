package xctest

import (
	"fmt"
	"slices"
	"strings"
)

// Converted is the result of converting one file.
type Converted struct {
	// Text is the converted source.
	Text string
	// Notes is what is left for a person, each as `line N: message`.
	Notes []string
}

func importsModule(l line, module string) bool {
	code := ""
	if l.code {
		code, _, _ = strings.Cut(l.text, "//")
	}

	parts := strings.FieldsFunc(code, func(c rune) bool { return c == ' ' || c == '\t' || c == '\r' })
	count := len(parts)

	return count >= 2 && count < 4 &&
		parts[count-2] == kwImport && parts[count-1] == module &&
		!slices.Contains(parts, "@testable")
}

func importsIn(lines []line, module string) bool {
	return slices.ContainsFunc(lines, func(l line) bool { return importsModule(l, module) })
}

// rewriteImports adds `import Testing` where `import XCTest` stood and
// drops the XCTest import unless a leftover still needs it.
func rewriteImports(source string, keepXCTest bool) string {
	lines := parseLines(source)
	hasTesting := importsIn(lines, "Testing")
	out := make([]string, 0, len(lines))

	for _, l := range lines {
		if !importsModule(l, "XCTest") {
			out = append(out, l.text)

			continue
		}

		if !hasTesting {
			hasTesting = true
			cr := ""

			if strings.HasSuffix(l.text, "\r") {
				cr = "\r"
			}

			out = append(out, prefixWhile(l.text, isIndent)+"import Testing"+cr)
		}

		if keepXCTest {
			out = append(out, l.text)
		}
	}

	return strings.Join(out, "\n")
}

// ImportsXCTest reports whether the file imports XCTest.
func ImportsXCTest(source string) bool {
	return importsIn(parseLines(source), "XCTest")
}

type note struct {
	message string
	line    int
}

// Convert rewrites an XCTest file into Swift Testing. It reports false when
// source does not import XCTest.
func Convert(source string) (Converted, bool) {
	if !ImportsXCTest(source) {
		return Converted{}, false
	}

	restructured := restructure(rewriteAssertions(source))
	keep := slices.ContainsFunc(leftovers(restructured), func(l leftover) bool { return l.needsXCTest })
	text := rewriteImports(restructured, keep)

	var notes []note
	for _, entry := range leftovers(text) {
		notes = append(notes, note{line: entry.line, message: entry.message})
	}

	kept := slices.IndexFunc(parseLines(text), func(l line) bool { return importsModule(l, "XCTest") })
	if keep && kept != -1 {
		notes = append(notes, note{line: kept + 1, message: "`import XCTest` kept for the constructs noted here"})
	}

	slices.SortStableFunc(notes, func(left, right note) int { return left.line - right.line })

	lines := make([]string, len(notes))
	for i, n := range notes {
		lines[i] = fmt.Sprintf("line %d: %s", n.line, n.message)
	}

	return Converted{Text: text, Notes: lines}, true
}
