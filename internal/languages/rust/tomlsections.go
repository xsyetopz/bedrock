package rust

// Line-oriented TOML editing that keeps comments and formatting. A file splits
// into sections, and a section into units: one `key = value` entry (however
// many lines its value spans), one comment line, or one blank line. Parsing
// values is left to a TOML library; editing text is done here because no
// library keeps the comments.

import (
	"regexp"
	"slices"
	"strings"
)

// section is one table of a TOML file with the comments that introduce it.
type section struct {
	// name is the dotted table name, such as `workspace.lints.rust`; empty
	// before the first header, where named reports false.
	name string
	// header is the header line, or "" for the text before the first header.
	header string
	// leading are the comment lines directly above the header.
	leading []string
	units   []string
}

// named reports whether the section has a header; the text before the first
// header does not.
func (s section) named() bool { return s.header != "" }

// lineState is where a scan of TOML lines stands: inside brackets or inside a
// multi-line string.
type lineState struct {
	// multi is the delimiter of a multi-line string that is still open.
	multi string
	depth int
}

// skipQuoted returns the index after the quoted string that starts at from.
func skipQuoted(line string, from int) int {
	quote := line[from]
	at := from + 1

	for at < len(line) && line[at] != quote {
		if quote == '"' && line[at] == '\\' {
			at += 2
		} else {
			at++
		}
	}

	return at + 1
}

// tripleWidth is the length of a `"""` or `”'` delimiter.
const tripleWidth = 3

// step consumes the text at index at and returns where to go on, or false to
// stop at a comment.
func (s *lineState) step(line string, at int) (int, bool) {
	char := line[at]
	triple := line[at:min(at+tripleWidth, len(line))]

	switch {
	case s.multi != "":
		closeAt := strings.Index(line[at:], s.multi)
		if closeAt == -1 {
			return len(line), true
		}

		after := at + closeAt + len(s.multi)
		s.multi = ""

		return after, true
	case char == '#':
		return at, false
	case triple == `"""` || triple == "'''":
		s.multi = triple

		return at + len(triple), true
	case char == '"' || char == '\'':
		return skipQuoted(line, at), true
	}

	switch char {
	case '[', '{':
		s.depth++
	case ']', '}':
		s.depth--
	}

	return at + 1, true
}

func (s *lineState) scan(line string) {
	for at, more := 0, true; at < len(line) && more; {
		at, more = s.step(line, at)
	}
}

// open reports whether the scanned text stops inside an array, table, or
// string.
func (s *lineState) open() bool { return s.depth > 0 || s.multi != "" }

// splitLines splits text after each newline, keeping it.
func splitLines(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// toUnits groups physical lines into units that start outside any array,
// table, or string.
func toUnits(text string) []string {
	var (
		units []string
		state lineState
		open  bool
	)

	for _, line := range splitLines(text) {
		if open {
			units[len(units)-1] += line
		} else {
			units = append(units, line)
		}

		state.scan(line)
		open = state.open()
	}

	return units
}

func isHeader(unit string) bool { return strings.HasPrefix(strings.TrimLeft(unit, " \t\r\n"), "[") }

func isComment(unit string) bool { return strings.HasPrefix(strings.TrimLeft(unit, " \t\r\n"), "#") }

// tableName returns `workspace.lints.a b` from the header line
// `[workspace.lints."a b"]`.
func tableName(header string) string {
	trailing := regexp.MustCompile(`\s+#.*$`)
	brackets := regexp.MustCompile(`^\[\[?|\]\]?$`)
	part := regexp.MustCompile(`"[^"]*"|'[^']*'|[^.\s"']+`)
	quotes := regexp.MustCompile(`^["']|["']$`)

	inner := brackets.ReplaceAllString(trailing.ReplaceAllString(strings.TrimSpace(header), ""), "")
	found := part.FindAllString(inner, -1)
	names := make([]string, 0, len(found))

	for _, name := range found {
		names = append(names, quotes.ReplaceAllString(name, ""))
	}

	return strings.Join(names, ".")
}

// parseSections splits TOML text into sections; the first holds what precedes
// any header.
func parseSections(text string) []section {
	sections := []section{{name: "", header: "", leading: nil, units: nil}}

	for _, unit := range toUnits(text) {
		last := &sections[len(sections)-1]

		if !isHeader(unit) {
			last.units = append(last.units, unit)

			continue
		}

		var leading []string

		for len(last.units) > 0 && isComment(last.units[len(last.units)-1]) {
			leading = append([]string{last.units[len(last.units)-1]}, leading...)
			last.units = last.units[:len(last.units)-1]
		}

		sections = append(sections, section{name: tableName(unit), header: unit, leading: leading, units: nil})
	}

	return sections
}

// renderSections returns the text of sections, with a newline after every unit
// but possibly the last.
func renderSections(sections []section) string {
	var parts []string

	for _, entry := range sections {
		parts = append(parts, entry.leading...)

		if entry.header != "" {
			parts = append(parts, entry.header)
		}

		parts = append(parts, entry.units...)
	}

	var out strings.Builder

	for index, part := range parts {
		out.WriteString(part)

		if !strings.HasSuffix(part, "\n") && index != len(parts)-1 {
			out.WriteString("\n")
		}
	}

	return out.String()
}

// withUnits returns entry with other units.
func withUnits(entry section, units []string) section {
	entry.units = units

	return entry
}

// keyOf returns the key at the start of an entry, such as `edition.workspace`,
// or false for comments and blanks.
func keyOf(unit string) (string, bool) {
	part := `(?:"[^"]*"|'[^']*'|[\w-]+)`
	pattern := regexp.MustCompile(`^\s*(` + part + `(?:\s*\.\s*` + part + `)*)\s*=`)

	match := pattern.FindStringSubmatch(unit)
	if match == nil {
		return "", false
	}

	dots := regexp.MustCompile(`\s*\.\s*`)
	quotes := regexp.MustCompile(`["']`)

	return quotes.ReplaceAllString(dots.ReplaceAllString(match[1], "."), ""), true
}

func isEntry(unit string) bool {
	_, ok := keyOf(unit)

	return ok
}

func isBlank(unit string) bool { return strings.TrimSpace(unit) == "" }

// hasNoEntries reports whether the section holds no entry, only comments and
// blank lines.
func hasNoEntries(entry section) bool {
	return !slices.ContainsFunc(entry.units, isEntry)
}

// withoutKeys returns entry without the entries whose key drop accepts.
func withoutKeys(entry section, drop func(key string) bool) section {
	units := slices.DeleteFunc(slices.Clone(entry.units), func(unit string) bool {
		key, ok := keyOf(unit)

		return ok && drop(key)
	})

	return withUnits(entry, units)
}

// withKey returns entry with `key = value`: replaced in place, else after the
// last entry.
func withKey(entry section, key, value string) section {
	line := key + " = " + value + "\n"
	units := slices.Clone(entry.units)

	for index, unit := range units {
		if found, ok := keyOf(unit); ok && found == key {
			units[index] = line

			return withUnits(entry, units)
		}
	}

	last := -1

	for index, unit := range units {
		if isEntry(unit) {
			last = index
		}
	}

	return withUnits(entry, slices.Insert(units, last+1, line))
}

// makeSection builds a section from a header name and entry lines.
func makeSection(name string, entries []string) section {
	units := make([]string, 0, len(entries))
	for _, entry := range entries {
		units = append(units, entry+"\n")
	}

	return section{name: name, header: "[" + name + "]\n", leading: nil, units: units}
}

// tidy returns sections with one blank line between neighbors and none at the
// end, which keeps an insert or removal from leaving gaps or piling blank lines
// up.
func tidy(sections []section) []section {
	tidied := make([]section, 0, len(sections))

	for index, entry := range sections {
		units := slices.Clone(entry.units)
		for len(units) > 0 && isBlank(units[len(units)-1]) {
			units = units[:len(units)-1]
		}

		followed := index < len(sections)-1
		filled := entry.header != "" || len(units) > 0

		if followed && filled {
			units = append(units, "\n")
		}

		tidied = append(tidied, withUnits(entry, units))
	}

	return tidied
}

// removeSections removes every section for which drop is true and returns
// where the first one sat, or -1 when none did.
func removeSections(sections []section, drop func(name string) bool) ([]section, int) {
	first := slices.IndexFunc(sections, func(entry section) bool { return entry.named() && drop(entry.name) })
	kept := slices.DeleteFunc(slices.Clone(sections), func(entry section) bool {
		return entry.named() && drop(entry.name)
	})

	return kept, first
}

// findSection returns the section called name, if the file has one.
func findSection(sections []section, name string) (section, bool) {
	index := slices.IndexFunc(sections, func(entry section) bool { return entry.named() && entry.name == name })
	if index == -1 {
		return section{}, false
	}

	return sections[index], true
}

// upsertSection returns sections with entry replacing the one of the same
// name, or appended.
func upsertSection(sections []section, entry section) []section {
	result := slices.Clone(sections)

	index := slices.IndexFunc(result, func(other section) bool { return other.named() && other.name == entry.name })
	if index == -1 {
		return append(result, entry)
	}

	result[index] = entry

	return result
}
