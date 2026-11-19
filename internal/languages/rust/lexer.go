package rust

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokenKind says what a token is; punctuation carries its character as the
// token text.
type tokenKind int

const (
	kindNone tokenKind = iota
	kindIdent
	kindLiteral
	kindPunct
)

// token is one Rust token: its place in the source and its text.
type token struct {
	text string
	// start and end are byte offsets of the token in the source.
	start int
	end   int
	// line is the 1-based line of the first character.
	line int
	kind tokenKind
}

// isIdentByte treats every byte of a non-ASCII character as part of an
// identifier, as rustc does for Unicode identifiers.
func isIdentByte(b byte) bool {
	return b == '_' || b >= 0x80 || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// scanner is a cursor that counts lines as it moves.
type scanner struct {
	source string
	at     int
	line   int
}

func (s *scanner) done() bool { return s.at >= len(s.source) }

// peek returns the byte offset bytes ahead, or 0 past the end.
func (s *scanner) peek(offset int) byte {
	if s.at+offset >= len(s.source) {
		return 0
	}

	return s.source[s.at+offset]
}

func (s *scanner) bump(count int) {
	for range count {
		if s.done() {
			return
		}

		if s.peek(0) == '\n' {
			s.line++
		}

		s.at++
	}
}

func (s *scanner) skipWhile(keep func(b byte) bool) {
	for !s.done() && keep(s.peek(0)) {
		s.bump(1)
	}
}

// blockComment skips a block comment, which may nest, from its opening slash.
func (s *scanner) blockComment() {
	depth := 0

	for !s.done() {
		switch {
		case s.peek(0) == '/' && s.peek(1) == '*':
			depth++

			s.bump(1)
		case s.peek(0) == '*' && s.peek(1) == '/':
			depth--

			s.bump(1)

			if depth == 0 {
				s.bump(1)

				return
			}
		}

		s.bump(1)
	}
}

// quoted skips a quoted literal from its opening quote, honoring backslash
// escapes.
func (s *scanner) quoted(quote byte) {
	s.bump(1)

	for !s.done() {
		switch s.peek(0) {
		case '\\':
			s.bump(1)
		case quote:
			s.bump(1)

			return
		}

		s.bump(1)
	}
}

// rawString skips `#*"..."#*` from the first `#` or `"`, after the `r` prefix.
func (s *scanner) rawString() {
	hashes := 0
	for s.peek(0) == '#' {
		hashes++

		s.bump(1)
	}

	s.bump(1)

	closing := `"` + strings.Repeat("#", hashes)

	for !s.done() {
		if strings.HasPrefix(s.source[s.at:], closing) {
			s.bump(len(closing))

			return
		}

		s.bump(1)
	}
}

// quoteWidth is the two quotes around a char literal; rawIdentPrefix is the `r#` of a
// raw identifier.
const (
	quoteWidth     = 2
	rawIdentPrefix = 2
)

// rawPrefix returns the length of a raw string prefix (`r`, `br`, `cr`) at
// the cursor, or 0.
func (s *scanner) rawPrefix() int {
	var prefix int

	switch first := s.peek(0); {
	case first == 'r':
		prefix = 1
	case (first == 'b' || first == 'c') && s.peek(1) == 'r':
		prefix = 2
	default:
		return 0
	}

	offset := prefix
	for s.peek(offset) == '#' {
		offset++
	}

	if s.peek(offset) == '"' {
		return prefix
	}

	return 0
}

// quote skips a char literal or a lifetime from its `'`.
func (s *scanner) quote() tokenKind {
	if s.peek(1) == '\\' {
		s.quoted('\'')

		return kindLiteral
	}

	_, width := utf8.DecodeRuneInString(s.source[min(s.at+1, len(s.source)):])
	if s.peek(1+width) == '\'' {
		s.bump(width + quoteWidth)

		return kindLiteral
	}

	s.bump(1)
	s.skipWhile(isIdentByte)

	return kindIdent
}

// token skips one token that is not a comment or whitespace and returns its
// kind.
func (s *scanner) token() tokenKind {
	char := s.peek(0)
	next := s.peek(1)

	if prefix := s.rawPrefix(); prefix > 0 {
		s.bump(prefix)
		s.rawString()

		return kindLiteral
	}

	switch {
	case (char == 'b' || char == 'c') && next == '"':
		s.bump(1)
		s.quoted('"')
	case char == 'b' && next == '\'':
		s.bump(1)
		s.quoted('\'')
	case char == '"':
		s.quoted('"')
	default:
		return s.simpleToken(char, next)
	}

	return kindLiteral
}

func (s *scanner) simpleToken(char, next byte) tokenKind {
	switch {
	case char == '\'':
		return s.quote()
	case char == 'r' && next == '#':
		s.bump(rawIdentPrefix)
		s.skipWhile(isIdentByte)

		return kindIdent
	case isDigit(char):
		s.skipWhile(isIdentByte)

		return kindLiteral
	case isIdentByte(char):
		s.skipWhile(isIdentByte)

		return kindIdent
	default:
		s.bump(1)

		return kindPunct
	}
}

// tokens splits Rust source into tokens, skipping whitespace and comments and
// keeping strings, raw strings, and char literals whole, so text inside them
// never looks like code.
func tokens(source string) []token {
	scan := &scanner{source: source, at: 0, line: 1}

	var found []token

	for !scan.done() {
		first, width := utf8.DecodeRuneInString(source[scan.at:])

		switch {
		case unicode.IsSpace(first):
			scan.bump(width)
		case scan.peek(0) == '/' && scan.peek(1) == '/':
			scan.skipWhile(func(b byte) bool { return b != '\n' })
		case scan.peek(0) == '/' && scan.peek(1) == '*':
			scan.blockComment()
		default:
			start, line := scan.at, scan.line
			kind := scan.token()
			found = append(found, token{
				kind: kind, text: source[start:scan.at], start: start, end: scan.at, line: line,
			})
		}
	}

	return found
}
