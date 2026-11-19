package rust

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// span is a range of byte offsets in a source file.
type span struct{ from, to int }

// module is a `mod name;` or `mod name { ... }` declaration.
type module struct {
	// body is the offsets of the `{` and `}` of an inline body, or nil for
	// `mod name;`.
	body *span
	// pathAttr is the offsets of the `#[path = "..."]` attribute, or nil.
	pathAttr *span
	// name is the module name without any `r#` prefix.
	name string
	// path is the `#[path = "..."]` value when pathAttr is set.
	path string
	// line is where the declaration starts.
	line int
	// start is the offset of `mod`.
	start int
	// depth is how many braces enclose the declaration.
	depth int
	// testOnly is set when a `#[cfg(test)]`-style attribute gates it.
	testOnly bool
}

// tokenAt returns the token at index, or a zero token past either end, so
// lookahead needs no bounds checks.
func tokenAt(all []token, index int) token {
	if index < 0 || index >= len(all) {
		return token{kind: kindNone, text: "", start: 0, end: 0, line: 0}
	}

	return all[index]
}

// closing returns the index of the bracket that closes the `[` or `(` at
// openAt.
func closing(all []token, openAt int) (int, bool) {
	open := tokenAt(all, openAt).text

	closer := ")"
	if open == "[" {
		closer = "]"
	} else if open != "(" {
		return 0, false
	}

	depth := 0

	for index := openAt; index < len(all); index++ {
		switch all[index].text {
		case open:
			depth++
		case closer:
			depth--
		}

		if depth == 0 {
			return index, true
		}
	}

	return 0, false
}

// splitArguments splits the tokens inside `( ... )` at top-level commas.
func splitArguments(group []token) [][]token {
	parts := [][]token{nil}
	depth := 0

	for _, tok := range group {
		switch tok.text {
		case "(":
			depth++
		case ")":
			depth--
		}

		if tok.text == "," && depth == 0 {
			parts = append(parts, nil)
		} else {
			parts[len(parts)-1] = append(parts[len(parts)-1], tok)
		}
	}

	return slices.DeleteFunc(parts, func(part []token) bool { return len(part) == 0 })
}

// impliesTest reports whether the cfg predicate holds only when compiling
// tests: `all` needs one such part, `any` needs every part to be one, and
// `not` never is.
func impliesTest(predicate []token) bool {
	if len(predicate) == 1 {
		return predicate[0].text == "test"
	}

	if len(predicate) < 3 || predicate[1].text != "(" || predicate[len(predicate)-1].text != ")" {
		return false
	}

	parts := splitArguments(predicate[2 : len(predicate)-1])

	switch predicate[0].text {
	case "all":
		return slices.ContainsFunc(parts, impliesTest)
	case "any":
		return len(parts) > 0 && !slices.ContainsFunc(parts, func(part []token) bool { return !impliesTest(part) })
	default:
		return false
	}
}

func isTestCfg(inner []token) bool {
	return len(inner) >= 4 && inner[0].text == "cfg" && inner[1].text == "(" &&
		inner[len(inner)-1].text == ")" && impliesTest(inner[2:len(inner)-1])
}

// pathValue returns the string of `path = "..."`.
func pathValue(inner []token) (string, bool) {
	if len(inner) != 3 || inner[0].text != "path" || inner[1].text != "=" ||
		inner[2].kind != kindLiteral || !strings.HasPrefix(inner[2].text, `"`) {
		return "", false
	}

	text := inner[2].text

	return text[1 : len(text)-1], true
}

// attrs are the attributes in front of an item.
type attrs struct {
	pathAttr *span
	path     string
	// next is the index of the item's first token.
	next     int
	testOnly bool
}

func attributesAt(all []token, from int) attrs {
	found := attrs{pathAttr: nil, path: "", next: from, testOnly: false}

	for tokenAt(all, found.next).text == "#" {
		end, ok := closing(all, found.next+1)
		if !ok {
			break
		}

		inner := all[found.next+2 : end]
		found.testOnly = found.testOnly || isTestCfg(inner)

		if value, isPath := pathValue(inner); isPath && found.pathAttr == nil {
			found.path = value
			found.pathAttr = &span{from: all[found.next].start, to: all[end].end}
		}

		found.next = end + 1
	}

	return found
}

// skipModifiers returns the index after `pub`, `pub(crate)`, and `unsafe`.
func skipModifiers(all []token, from int) int {
	cursor := from

	if tokenAt(all, cursor).text == "pub" {
		cursor++

		if end, ok := closing(all, cursor); ok {
			cursor = end + 1
		}
	}

	if tokenAt(all, cursor).text == "unsafe" {
		return cursor + 1
	}

	return cursor
}

// bracesEnd returns the index of the `}` that closes the `{` at openAt.
func bracesEnd(all []token, openAt int) (int, bool) {
	depth := 0

	for index := openAt; index < len(all); index++ {
		switch all[index].text {
		case "{":
			depth++
		case "}":
			depth--
		}

		if depth == 0 {
			return index, true
		}
	}

	return 0, false
}

// depths returns how many braces enclose each token.
func depths(all []token) []int {
	result := make([]int, len(all))
	depth := 0

	for index, tok := range all {
		if tok.text == "}" {
			depth = max(0, depth-1)
		}

		result[index] = depth

		if tok.text == "{" {
			depth++
		}
	}

	return result
}

// A module declaration is `mod name` and then `;` or `{`: the delimiter sits two
// tokens after the keyword, and the declaration spans three.
const (
	nameToDelimiter = 2
	moduleTokens    = 3
)

// modules returns every `mod name;` and `mod name { ... }` in source, in
// order, outside comments and strings.
func modules(source string) []module {
	all := tokens(source)
	enclosing := depths(all)

	var found []module

	for at := 0; at < len(all); {
		attributes := attributesAt(all, at)
		cursor := skipModifiers(all, attributes.next)
		keyword, name, after := tokenAt(all, cursor), tokenAt(all, cursor+1), tokenAt(all, cursor+nameToDelimiter)

		var body *span

		if after.text == "{" {
			if end, ok := bracesEnd(all, cursor+nameToDelimiter); ok {
				body = &span{from: after.start, to: all[end].start}
			}
		}

		if keyword.text == "mod" && name.kind == kindIdent && (after.text == ";" || body != nil) {
			found = append(found, module{
				name:     strings.TrimPrefix(name.text, "r#"),
				line:     keyword.line,
				path:     attributes.path,
				pathAttr: attributes.pathAttr,
				testOnly: attributes.testOnly,
				start:    keyword.start,
				body:     body,
				depth:    enclosing[cursor],
			})
			at = cursor + moduleTokens
		} else {
			at = max(cursor, at+1)
		}
	}

	return found
}

// ownsDirectory reports whether file is a crate root or a `mod.rs`, whose
// child modules sit beside it rather than in a directory named after it. A
// target root counts too: `tests/x.rs`, `examples/x.rs`, `benches/x.rs`, or
// `src/bin/x.rs` in a package. A directory merely named `tests` inside `src/`
// is an ordinary module directory.
func ownsDirectory(file string) bool {
	switch filepath.Base(file) {
	case "lib.rs", "main.rs", "mod.rs", "build.rs":
		return true
	}

	parent := filepath.Dir(file)

	var owner string

	switch filepath.Base(parent) {
	case "tests", "examples", "benches":
		owner = filepath.Dir(parent)
	case "bin": // src/bin, Cargo's directory of extra binaries
		owner = filepath.Dir(filepath.Dir(parent))
	default:
		return false
	}

	return fsutil.Exists(filepath.Join(owner, "Cargo.toml"))
}

// moduleDirectory returns where the child modules of file sit: beside a
// `mod.rs`, `lib.rs`, or `main.rs`, and in a directory named for the file
// otherwise.
func moduleDirectory(file string) string {
	if ownsDirectory(file) {
		return filepath.Dir(file)
	}

	return filepath.Join(filepath.Dir(file), strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
}

// childFile returns the file that holds the body of declaration, declared in
// file, or false when none exists.
func childFile(file string, declaration module) (string, bool) {
	if declaration.pathAttr != nil {
		candidate := filepath.Join(filepath.Dir(file), declaration.path)

		return candidate, fsutil.Exists(candidate)
	}

	base := moduleDirectory(file)

	for _, candidate := range []string{
		filepath.Join(base, declaration.name+".rs"),
		filepath.Join(base, declaration.name, "mod.rs"),
	} {
		if fsutil.Exists(candidate) {
			return candidate, true
		}
	}

	return "", false
}
