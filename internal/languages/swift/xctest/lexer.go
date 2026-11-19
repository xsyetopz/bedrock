// Package xctest rewrites XCTest unit tests into Swift Testing. It reads
// Swift with a small lexer rather than a parser: enough to skip strings and
// comments, match brackets, and track brace depth, which is all the
// rewrites need and keeps a file it cannot read exactly as it was.
package xctest

import (
	"strings"
	"unicode"
)

// scalars is source text as Unicode code points, so indices match Swift's
// scalars and a multi-byte character is one position.
type scalars []rune

// line is one source line with the brace depth where it starts and how many
// braces it opens.
type line struct {
	text string
	// code is whether the first non-blank character is code, not inside a
	// comment or string.
	code  bool
	depth int
	opens int
}

// token is an identifier in code, with the offsets of its first and
// one-past-last scalar.
type token struct {
	name  string
	start int
	end   int
}

const (
	lineComment  = "//"
	blockOpen    = "/*"
	blockClose   = "*/"
	tripleQuote  = `"""`
	openBrackets = "([{"
	closeBracket = ")]}"
	genericRest  = "<,.?[]():&"
)

func toScalars(text string) scalars { return scalars(text) }

func isSpace(c rune) bool { return unicode.IsSpace(c) || c == '\uFEFF' }

// isIdent reports whether c can be part of an identifier, `$0` included.
func isIdent(c rune) bool {
	return c == '_' || c == '$' || unicode.IsLetter(c) || unicode.IsNumber(c)
}

// isWord reports whether c can be part of a declared name.
func isWord(c rune) bool {
	return c == '_' || unicode.IsLetter(c) || unicode.IsNumber(c)
}

// at returns the scalar at index, or 0 beyond either end.
func (b scalars) at(index int) rune {
	if index < 0 || index >= len(b) {
		return 0
	}

	return b[index]
}

// prefixWhile returns the longest prefix of text whose characters satisfy test.
func prefixWhile(text string, test func(rune) bool) string {
	for i, c := range text {
		if !test(c) {
			return text[:i]
		}
	}

	return text
}

func (b scalars) startsAt(index int, text string) bool {
	for offset, c := range []rune(text) {
		if index+offset >= len(b) || b[index+offset] != c {
			return false
		}
	}

	return true
}

func (b scalars) indexFrom(c rune, from int) int {
	for i := from; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}

	return -1
}

func (b scalars) blockCommentEnd(from int) int {
	index := from + len("/*")
	depth := 1

	for index < len(b) && depth > 0 {
		switch {
		case b.startsAt(index, blockOpen):
			depth++
			index += 2
		case b.startsAt(index, blockClose):
			depth--
			index += 2
		default:
			index++
		}
	}

	return index
}

func (b scalars) stringEnd(from, quote int) int {
	hashes := strings.Repeat("#", quote-from)

	quotes := `"`
	if b.startsAt(quote, tripleQuote) {
		quotes = tripleQuote
	}

	index := quote + len(quotes)
	for index < len(b) {
		if b.startsAt(index, quotes+hashes) {
			return index + len(quotes) + len(hashes)
		}

		if len(quotes) == 1 && b[index] == '\n' {
			return index
		}

		if b.startsAt(index, `\`+hashes) {
			index += 1 + len(hashes)
			if b.startsAt(index, "(") {
				index = b.interpolationEnd(index)
			}
		}

		index++
	}

	return len(b)
}

// interpolationEnd returns the index of the bracket closing the `(` of a
// string interpolation, or the end of the text when it never closes.
func (b scalars) interpolationEnd(open int) int {
	if end, ok := b.matchClose(open); ok {
		return end
	}

	return len(b)
}

// trivia returns the index just past the comment or string literal that
// starts at from, if one does.
func (b scalars) trivia(from int) (int, bool) {
	if b.startsAt(from, lineComment) {
		if newline := b.indexFrom('\n', from); newline != -1 {
			return newline, true
		}

		return len(b), true
	}

	if b.startsAt(from, blockOpen) {
		return b.blockCommentEnd(from), true
	}

	quote := from
	for b.at(quote) == '#' && quote < len(b) {
		quote++
	}

	if b.startsAt(quote, `"`) {
		return b.stringEnd(from, quote), true
	}

	return 0, false
}

// matchClose returns the index of the bracket that closes the one at open.
func (b scalars) matchClose(open int) (int, bool) {
	depth := 0
	index := open

	for index < len(b) {
		if skip, ok := b.trivia(index); ok {
			index = skip

			continue
		}

		if strings.ContainsRune(openBrackets, b[index]) {
			depth++
		}

		if strings.ContainsRune(closeBracket, b[index]) {
			depth--
		}

		if depth == 0 {
			return index, true
		}

		index++
	}

	return 0, false
}

// codeMask says for each scalar whether it is code, not inside a comment or
// string.
func (b scalars) codeMask() []bool {
	mask := make([]bool, len(b))
	for i := range mask {
		mask[i] = true
	}

	index := 0
	for index < len(b) {
		end, ok := b.trivia(index)
		if !ok {
			index++

			continue
		}

		for i := index; i < end && i < len(b); i++ {
			mask[i] = false
		}

		index = max(end, index+1)
	}

	return mask
}

// genericEnd returns where the generic argument list that `<` at index
// opens ends, if one does.
func (b scalars) genericEnd(index int) (int, bool) {
	if b.at(index) != '<' || !isIdent(b.at(index-1)) {
		return 0, false
	}

	end := b.indexFrom('>', index)
	if end == -1 {
		return 0, false
	}

	for _, c := range b[index:end] {
		if !isIdent(c) && !isSpace(c) && !strings.ContainsRune(genericRest, c) {
			return 0, false
		}
	}

	return end, true
}

// topLevel returns the indices outside strings, comments, and brackets, or
// false when a comment is in the way.
func (b scalars) topLevel() ([]int, bool) {
	var found []int

	depth := 0
	index := 0

	for index < len(b) {
		if skip, ok := b.trivia(index); ok {
			if b[index] == '/' {
				return nil, false
			}

			index = skip

			continue
		}

		// The commas of a generic argument list must not split arguments.
		if depth == 0 {
			if end, ok := b.genericEnd(index); ok {
				index = end + 1

				continue
			}
		}

		if strings.ContainsRune(closeBracket, b[index]) {
			depth--
		}

		if depth == 0 {
			found = append(found, index)
		}

		if strings.ContainsRune(openBrackets, b[index]) {
			depth++
		}

		index++
	}

	return found, true
}

// identifiers returns every identifier outside comments and strings.
func (b scalars) identifiers() []token {
	mask := b.codeMask()

	var tokens []token

	index := 0
	for index < len(b) {
		if !mask[index] || !isIdent(b[index]) {
			index++

			continue
		}

		end := index
		for end < len(b) && isIdent(b[end]) {
			end++
		}

		if b[index] < '0' || b[index] > '9' {
			tokens = append(tokens, token{name: string(b[index:end]), start: index, end: end})
		}

		index = end
	}

	return tokens
}

// ahead returns the text after index, without leading white space and cut
// short.
func (b scalars) ahead(index int) string {
	start := index
	for start < len(b) && isSpace(b[start]) {
		start++
	}

	const reach = 8

	return string(b[start:min(start+reach, len(b))])
}

// behind returns the last non-blank scalar before index.
func (b scalars) behind(index int) (rune, bool) {
	for i := min(index, len(b)) - 1; i >= 0; i-- {
		if !isSpace(b[i]) {
			return b[i], true
		}
	}

	return 0, false
}

// parseLines splits source into lines, tracking brace depth outside strings
// and comments.
func parseLines(source string) []line {
	b := toScalars(source)
	mask := b.codeMask()

	var lines []line

	depth := 0
	start := 0

	for start <= len(b) {
		end := len(b)
		if newline := b.indexFrom('\n', start); newline != -1 {
			end = newline
		}

		first := -1
		opens := 0
		braces := 0

		for k := start; k < end; k++ {
			if first == -1 && b[k] != ' ' && b[k] != '\t' && b[k] != '\r' {
				first = k
			}

			if mask[k] && (b[k] == '{' || b[k] == '}') {
				braces++

				if b[k] == '{' {
					opens++
				}
			}
		}

		lines = append(lines, line{
			text:  string(b[start:end]),
			code:  first != -1 && mask[first],
			depth: depth,
			opens: opens,
		})
		depth = max(0, depth+2*opens-braces)
		start = end + 1
	}

	return lines
}
