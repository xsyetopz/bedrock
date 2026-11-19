package xctest

import (
	"strings"
)

const operatorChars = "+*/%<>=&|^~:,;"

// isCompound reports whether the top-level character at index makes the
// text a compound expression.
func isCompound(b scalars, index int) bool {
	c := b[index]

	next := ' '
	if index+1 < len(b) {
		next = b[index+1]
	}

	if strings.ContainsRune(operatorChars, c) {
		return true
	}

	switch c {
	case '-':
		return index != 0
	case '!':
		return index != 0 && next == '='
	case '?':
		return !strings.ContainsRune(".([)", next) && index+1 != len(b)
	case '.':
		return next == '.' || (index > 0 && b[index-1] == '.')
	default:
		return isSpace(c)
	}
}

func needsParens(text string) bool {
	b := toScalars(text)
	indices, _ := b.topLevel()

	for _, index := range indices {
		if isCompound(b, index) {
			return true
		}
	}

	return false
}

// wrap parenthesizes text that is a compound expression.
func wrap(text string) string {
	if needsParens(text) {
		return "(" + text + ")"
	}

	return text
}

// splitArguments returns the top-level, trimmed arguments, or false when a
// comment sits inside them.
func splitArguments(b scalars) ([]string, bool) {
	tops, ok := b.topLevel()
	if !ok {
		return nil, false
	}

	cuts := []int{-1}

	for _, index := range tops {
		if b[index] == ',' {
			cuts = append(cuts, index)
		}
	}

	cuts = append(cuts, len(b))

	parts := make([]string, 0, len(cuts)-1)
	for i := 1; i < len(cuts); i++ {
		parts = append(parts, strings.TrimSpace(string(b[cuts[i-1]+1:cuts[i]])))
	}

	if last := len(parts) - 1; parts[last] == "" {
		parts = parts[:last]
	}

	return parts, true
}

// labelled is an argument label with the rest of the argument.
type labelled struct {
	name  string
	value string
}

// label returns the `file:`, `line:`, or `accuracy:` label that starts an
// argument, with the rest of it.
func label(part string) (labelled, bool) {
	name := prefixWhile(part, isWord)
	after := strings.TrimLeft(part[len(name):], " ")

	if name != "file" && name != "line" && name != "accuracy" {
		return labelled{}, false
	}

	if !strings.HasPrefix(after, ":") || strings.HasPrefix(after, "::") {
		return labelled{}, false
	}

	return labelled{name: name, value: strings.TrimSpace(after[1:])}, true
}

// arguments are the arguments of one assertion call, without the `file:`
// and `line:` labels.
type arguments struct {
	accuracy    string
	positional  []string
	hasAccuracy bool
}

func parseArguments(parts []string, rewrite func(string) string) arguments {
	var parsed arguments

	for _, text := range parts {
		part := rewrite(text)

		found, labeled := label(part)

		switch {
		case labeled && found.name == "accuracy":
			parsed.accuracy = found.value
			parsed.hasAccuracy = true
		case !labeled:
			parsed.positional = append(parsed.positional, part)
		}
	}

	return parsed
}
