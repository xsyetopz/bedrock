package swift

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	suppress = "swiftlint:disable"
	enable   = "swiftlint:enable"
)

// directive returns the text of a line's `//` comment, where SwiftLint and
// swift-format read directives.
func directive(text string) string {
	start := strings.Index(text, "//")
	if start == -1 {
		return ""
	}

	return strings.TrimLeft(text[start:], "/ ")
}

// rulesOf returns the rule names before any trailing comment.
func rulesOf(text string) []string {
	var rules []string

	for word := range strings.SplitSeq(text, " ") {
		if strings.HasPrefix(word, "//") {
			break
		}

		if word != "" {
			rules = append(rules, word)
		}
	}

	return rules
}

// isReenabled reports whether a later directive enables every rule.
func isReenabled(directives []string, index int, rules []string) bool {
	for _, later := range directives[index+1:] {
		if !strings.HasPrefix(later, enable) {
			continue
		}

		enabled := strings.Split(later[len(enable):], " ")

		if !slices.ContainsFunc(rules, func(rule string) bool { return !slices.Contains(enabled, rule) }) {
			return true
		}
	}

	return false
}

// suppressions returns the lines that turn a SwiftLint rule off for the rest
// of the file, or swift-format off for the whole file, each as `N: message`.
func suppressions(text string) []string {
	var directives []string
	for line := range strings.SplitSeq(text, "\n") {
		directives = append(directives, directive(line))
	}

	var found []string

	for index, comment := range directives {
		number := strconv.Itoa(index + 1)

		if strings.HasPrefix(comment, "swift-format-ignore-file") {
			found = append(found, number+": `swift-format-ignore-file`; ignore the one declaration instead")
		}

		if !strings.HasPrefix(comment, suppress) {
			continue
		}

		tail := comment[len(suppress):]
		if strings.HasPrefix(tail, ":") || isReenabled(directives, index, rulesOf(tail)) {
			continue
		}

		found = append(found, number+": `swiftlint:disable` without a matching `swiftlint:enable`; "+
			"use `swiftlint:disable:next`")
	}

	return found
}

// isUITest reports whether a path sits in a `*UITests` or `*UITest`
// directory, where XCTest stays.
func isUITest(path string) bool {
	parts := strings.Split(path, "/")

	for _, part := range parts[:len(parts)-1] {
		if strings.HasSuffix(part, "UITests") || strings.HasSuffix(part, "UITest") {
			return true
		}
	}

	return false
}

func importXCTest() *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*(?:@\w+\s+)*import\s+(?:\w+\s+)?XCTest\b`)
}

// importsXCTest reports whether the text imports XCTest, `@testable import
// XCTest` forms included.
func importsXCTest(text string) bool {
	return importXCTest().MatchString(text)
}
