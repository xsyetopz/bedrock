package rust

import (
	"slices"
	"strings"
)

// pathPrefixes are the prefixes each kind of relative path gains.
type pathPrefixes struct {
	// include is for `include!`, `include_str!`, and `include_bytes!`, which
	// resolve from the file.
	include string
	// path is for `#[path = "..."]` on a `mod x;`, which resolves from the
	// module directory.
	path string
}

// The tokens of `include!("...")` count from the macro name: `!`, the opening
// bracket, the literal, and the closing bracket.
const (
	callOpen    = 2
	callLiteral = 3
	callClose   = 4
)

// includeLiteral returns the literal of an `include!("...")`-style call whose
// name is at index.
func includeLiteral(all []token, index int) (token, bool) {
	opens := slices.Contains([]string{"(", "[", "{"}, tokenAt(all, index+callOpen).text)
	closes := slices.Contains([]string{")", "]", "}"}, tokenAt(all, index+callClose).text)
	literal := tokenAt(all, index+callLiteral)

	return literal, opens && closes && literal.kind == kindLiteral
}

// precedesInlineModule reports whether the item an attribute ending at end
// decorates is an inline module.
func precedesInlineModule(all []token, end int) bool {
	for _, tok := range all[end+1:] {
		if tok.text == ";" || tok.text == "{" {
			return tok.text == "{"
		}
	}

	return false
}

// literalPath is the path a string literal holds. Valid is false for one with
// escapes or that is no string; skip is true for an absolute one, which a
// rename does not change.
type literalPath struct {
	value string
	skip  bool
	valid bool
}

// relative returns the path in a plain string literal.
func relative(literal token) literalPath {
	text := literal.text
	if len(text) < 2 || !strings.HasPrefix(text, `"`) || !strings.HasSuffix(text, `"`) || strings.Contains(text, `\`) {
		return literalPath{}
	}

	if strings.HasPrefix(text, `"/`) {
		return literalPath{skip: true, valid: true}
	}

	return literalPath{value: text[1 : len(text)-1], valid: true}
}

// pathLiteral returns the `#[path = "..."]` literal, at depth 0 only, of the
// attribute whose `#` is at index, and where it ends. It fails for a path that
// cannot be rewritten.
func pathLiteral(all []token, index, depth int) (int, *token, bool) {
	open := index + 1
	if tokenAt(all, open).text == "!" {
		open++
	}

	if tokenAt(all, open).text != "[" {
		return index, nil, true
	}

	end, found := closing(all, open)
	if !found {
		return 0, nil, false
	}

	inner := all[open+1 : end]
	names := false

	for at, part := range inner {
		if part.text == "path" && tokenAt(inner, at+1).text == "=" {
			names = true
		}
	}

	if !names || depth > 0 {
		return end, nil, true
	}

	if len(inner) != 3 || precedesInlineModule(all, end) {
		return 0, nil, false
	}

	return end, &inner[2], true
}

// pathSite is a path literal found at a token, with where to resume.
type pathSite struct {
	literal *token
	next    int
	include bool
}

// literalAt returns the path literal the token at index introduces, if any,
// and the index to resume from. It fails when the token names a path that
// cannot be rewritten.
func literalAt(all []token, index, depth int) (pathSite, bool) {
	tok := tokenAt(all, index)

	if tok.kind == kindIdent && slices.Contains([]string{"include", "include_str", "include_bytes"}, tok.text) &&
		tokenAt(all, index+1).text == "!" {
		literal, ok := includeLiteral(all, index)

		return pathSite{literal: &literal, next: index, include: true}, ok
	}

	if tok.text == "#" {
		end, literal, ok := pathLiteral(all, index, depth)

		return pathSite{literal: literal, next: end, include: false}, ok
	}

	return pathSite{literal: nil, next: index, include: false}, true
}

// prefixPaths rewrites the relative paths in source for a file in another
// place: each `#[path = "..."]` on a `mod x;` and each `include!`-style literal
// gains its prefix. A path inside an inline `mod x { }` keeps its text, since
// rustc resolves it through the module's own directory, which does not move.
// It fails when a path cannot be rewritten safely: one that is computed,
// escaped, set through `cfg_attr`, or given to an inline module.
func prefixPaths(source string, prefixes pathPrefixes) (string, bool) {
	all := tokens(source)

	type edit struct {
		value string
		start int
		end   int
	}

	var edits []edit

	depth := 0

	for index := 0; index < len(all); index++ {
		switch all[index].text {
		case "{":
			depth++
		case "}":
			depth = max(0, depth-1)
		}

		site, ok := literalAt(all, index, depth)
		if !ok {
			return "", false
		}

		index = site.next

		if site.literal == nil {
			continue
		}

		found := relative(*site.literal)
		if !found.valid {
			return "", false
		}

		if found.skip {
			continue
		}

		added := prefixes.path
		if site.include {
			added = prefixes.include
		}

		edits = append(edits, edit{start: site.literal.start, end: site.literal.end, value: added + found.value})
	}

	text := source
	for _, change := range slices.Backward(edits) {
		text = text[:change.start] + `"` + change.value + `"` + text[change.end:]
	}

	return text, true
}
