package rust

import (
	"regexp"
	"strings"
)

// gate makes Clippy treat a whole integration test as test code.
const gate = "#![cfg(test)]"

// integrationDoc heads a gated file that had no `//!` lines, since `missing_docs`
// checks each test file as its own crate.
const integrationDoc = "//! Integration tests run against the public API, as another crate would.\n"

func integrationTestPattern() *regexp.Regexp { return regexp.MustCompile(`(?:^|/)tests/[^/]+\.rs$`) }

// isIntegrationTest reports whether path is a top-level integration test of a
// crate: `tests/<name>.rs`.
func isIntegrationTest(path string) bool { return integrationTestPattern().MatchString(path) }

// hasGate reports whether the file has `#![cfg(test)]` as code, not inside a
// comment or string.
func hasGate(source string) bool {
	var joined strings.Builder

	for _, tok := range tokens(source) {
		if tok.kind == kindLiteral {
			joined.WriteString("0")
		} else {
			joined.WriteString(tok.text)
		}
	}

	return strings.Contains(joined.String(), gate)
}

// gated returns the integration test with `#![cfg(test)]` after its leading
// `//!` lines, so Clippy treats the whole file as test code: its
// `tests_outside_test_module` lint rejects each `#[test]` outside a
// `#[cfg(test)]` module. It reports false when the file already has the
// attribute.
func gated(source string) (string, bool) {
	if hasGate(source) {
		return "", false
	}

	lines := strings.SplitAfter(source, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	count := len(lines)

	for index, line := range lines {
		if !strings.HasPrefix(line, "//!") {
			count = index

			break
		}
	}

	head := strings.Join(lines[:count], "")
	tail := strings.Join(lines[count:], "")

	leading := integrationDoc
	if head != "" {
		leading = strings.TrimRight(head, " \t\r\n\v\f") + "\n"
	}

	return leading + gate + "\n\n" + tail, true
}
