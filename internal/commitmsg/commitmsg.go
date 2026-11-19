// Package commitmsg checks a commit message against Conventional Commits.
//
// It ports compilerla/conventional-pre-commit v4.5.0 (Apache-2.0) with its
// default options:
// https://github.com/compilerla/conventional-pre-commit/blob/v4.5.0/conventional_pre_commit/format.py
// Changed: the subject must be the first line, where upstream accepts a
// conventional line anywhere in the message. Each line holds at most
// MaxLine characters, the limit of @commitlint/config-conventional, counted
// as graphemes so accents and emoji count once.
package commitmsg

import (
	"regexp"
	"slices"
	"strings"

	"github.com/rivo/uniseg"
)

// MaxLine is the most characters one line of a message holds.
const MaxLine = 100

// Types are the commit types a subject may start with.
func Types() []string {
	return []string{"build", "chore", "ci", "docs", "feat", "fix", "perf", "refactor", "revert", "style", "test"}
}

// footerLine is the index of the first line after the subject and the blank
// line that separates it.
const footerLine = 2

const scissors = "# ------------------------ >8 ------------------------"

func subject() *regexp.Regexp {
	return regexp.MustCompile(`^(?:` + strings.Join(Types(), "|") + `)(?:\([a-z0-9_ :,./#-]+\))?!?: .+$`)
}

func exempt() *regexp.Regexp   { return regexp.MustCompile(`^(?:amend|fixup|squash)! `) }
func merge() *regexp.Regexp    { return regexp.MustCompile(`^merge(?:[^a-z0-9_]|$)`) }
func breaking() *regexp.Regexp { return regexp.MustCompile(`^BREAKING[ -]CHANGE: `) }

// keptLines are the lines git keeps: those above the scissors line that are
// not comments. A message of nothing but comments keeps every line, as
// upstream does.
func keptLines(text string) []string {
	raw := strings.Split(text, "\n")
	if raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}

	if cut := slices.Index(raw, scissors); cut >= 0 {
		raw = raw[:cut]
	}

	kept := slices.DeleteFunc(slices.Clone(raw), func(line string) bool { return strings.HasPrefix(line, "#") })
	if len(kept) == 0 {
		return raw
	}

	return kept
}

// Conventional reports whether the commit-msg hook accepts text.
func Conventional(text string) bool {
	lines := keptLines(text)

	line := func(index int) string {
		if index < len(lines) {
			return lines[index]
		}

		return ""
	}

	first := line(0)
	if exempt().MatchString(first) || merge().MatchString(strings.ToLower(first)) {
		return true
	}

	if !subject().MatchString(strings.ToLower(first)) {
		return false
	}

	if slices.ContainsFunc(lines, func(l string) bool { return uniseg.GraphemeClusterCount(l) > MaxLine }) {
		return false
	}

	if line(1) != "" {
		return false
	}

	// A breaking-change footer must be upper case, as the spec writes it.
	footer := line(footerLine)

	return !breaking().MatchString(strings.ToUpper(footer)) || breaking().MatchString(footer)
}
