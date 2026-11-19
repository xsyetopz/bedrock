package editorconfig

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Widths in runes of the glob constructs that span more than one.
const (
	escapeWidth        = 2
	doubleStarWidth    = 2
	doubleStarDirWidth = 3
	minAlternatives    = 2
)

// numRange is the bounds of one {num1..num2} in a glob, in capture-group order.
type numRange struct {
	from, to float64
}

// token is the regular expression for one glob construct and the index of the
// rune after it.
type token struct {
	regex string
	next  int
}

// Glob is a compiled section glob.
type Glob struct {
	re     *regexp.Regexp
	ranges []numRange
}

// Compile translates a section glob as the EditorConfig spec reads it: `*`,
// `**`, `?`, `[name]`, `[!name]`, `{s1,s2}`, and `{num1..num2}`. A glob with
// no `/` matches at any depth, and one with a `/` matches from the
// directory of its .editorconfig.
func Compile(glob string) (*Glob, error) {
	anchored := "**/" + glob
	if strings.Contains(glob, "/") {
		anchored = strings.TrimPrefix(glob, "/")
	}

	t := &translator{ranges: nil}
	source := t.translate([]rune(anchored))

	// (?s) lets `**` cross a line break in a path, as it crosses any other character.
	re, err := regexp.Compile("(?s)^" + source + "$")
	if err != nil {
		return nil, fmt.Errorf("invalid glob `%s`: %w", glob, err)
	}

	return &Glob{re: re, ranges: t.ranges}, nil
}

// Match reports whether path, relative to the .editorconfig directory, fits
// the glob.
func (g *Glob) Match(path string) bool {
	match := g.re.FindStringSubmatchIndex(path)
	if match == nil {
		return false
	}

	for i, bounds := range g.ranges {
		start, end := match[2*(i+1)], match[2*(i+1)+1]
		if start < 0 {
			// The alternative holding this range did not take part.
			continue
		}

		value, err := strconv.ParseFloat(path[start:end], 64)
		if err != nil || value < min(bounds.from, bounds.to) || value > max(bounds.from, bounds.to) {
			return false
		}
	}

	return true
}

type translator struct {
	ranges []numRange
}

// translate returns the regular expression source for glob, recording each
// {num1..num2} in order.
func (t *translator) translate(glob []rune) string {
	var source strings.Builder

	for index := 0; index < len(glob); {
		tok := t.token(glob, index)
		source.WriteString(tok.regex)

		index = tok.next
	}

	return source.String()
}

func (t *translator) token(glob []rune, index int) token {
	switch glob[index] {
	case '\\':
		escaped := `\`
		if index+1 < len(glob) {
			escaped = string(glob[index+1])
		}

		return token{regex: regexp.QuoteMeta(escaped), next: index + escapeWidth}
	case '*':
		return star(glob, index)
	case '?':
		return token{regex: "[^/]", next: index + 1}
	case '[':
		return bracket(glob, index)
	case '{':
		return t.brace(glob, index)
	default:
		return token{regex: regexp.QuoteMeta(string(glob[index])), next: index + 1}
	}
}

func star(glob []rune, index int) token {
	if index+1 >= len(glob) || glob[index+1] != '*' {
		return token{regex: "[^/]*", next: index + 1}
	}

	if index+2 < len(glob) && glob[index+2] == '/' {
		return token{regex: "(?:.*/)?", next: index + doubleStarDirWidth}
	}

	return token{regex: ".*", next: index + doubleStarWidth}
}

// bracket reads [name] or [!name]; a [ with no ], or one around a /, is literal.
func bracket(glob []rune, open int) token {
	literal := token{regex: `\[`, next: open + 1}

	end := -1

	for i := open + 1; i < len(glob); i++ {
		if glob[i] == ']' {
			end = i

			break
		}
	}

	if end == -1 {
		return literal
	}

	body := string(glob[open+1 : end])
	if body == "" || body == "!" || strings.Contains(body, "/") {
		return literal
	}

	negated := strings.HasPrefix(body, "!")
	body = strings.TrimPrefix(body, "!")

	var class strings.Builder

	for _, char := range body {
		if strings.ContainsRune(`\[]^`, char) {
			class.WriteRune('\\')
		}

		class.WriteRune(char)
	}

	prefix := "["
	if negated {
		prefix = "[^"
	}

	return token{regex: prefix + class.String() + "]", next: end + 1}
}

// brace reads {num1..num2}, {s1,s2}, or a literal {s}, which has no comma.
func (t *translator) brace(glob []rune, open int) token {
	end := closing(glob, open)
	if end == -1 {
		return token{regex: `\{`, next: open + 1}
	}

	body := glob[open+1 : end]
	if bounds, ok := parseRange(string(body)); ok {
		t.ranges = append(t.ranges, bounds)

		return token{regex: `([+-]?\d+)`, next: end + 1}
	}

	parts := options(body)
	if len(parts) < minAlternatives {
		return token{regex: `\{` + t.translate(body) + `\}`, next: end + 1}
	}

	alternatives := make([]string, len(parts))
	for i, part := range parts {
		alternatives[i] = t.translate(part)
	}

	return token{regex: "(?:" + strings.Join(alternatives, "|") + ")", next: end + 1}
}

// closing returns the index of the } that closes the { at open, or -1.
func closing(glob []rune, open int) int {
	depth := 0

	for index := open; index < len(glob); index++ {
		switch glob[index] {
		case '\\':
			index++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}

	return -1
}

// options splits body at the commas outside any nested braces.
func options(body []rune) [][]rune {
	parts := [][]rune{nil}
	depth := 0

	for index := 0; index < len(body); index++ {
		char := body[index]
		last := len(parts) - 1

		switch {
		case char == '\\' && index+1 < len(body):
			parts[last] = append(parts[last], char, body[index+1])
			index++
		case char == ',' && depth == 0:
			parts = append(parts, nil)
		default:
			switch char {
			case '{':
				depth++
			case '}':
				depth--
			}

			parts[last] = append(parts[last], char)
		}
	}

	return parts
}

// parseRange reads "num1..num2", each an optionally signed integer.
func parseRange(body string) (numRange, bool) {
	from, to, found := strings.Cut(body, "..")
	if !found || !isInteger(from) || !isInteger(to) {
		return numRange{}, false
	}

	low, errLow := strconv.ParseFloat(from, 64)
	high, errHigh := strconv.ParseFloat(to, 64)

	return numRange{from: low, to: high}, errLow == nil && errHigh == nil
}

func isInteger(text string) bool {
	digits := text
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+") {
		digits = text[1:]
	}

	if digits == "" {
		return false
	}

	for _, char := range digits {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}
