package typescript

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// bunfigMerge is the state of one bunfig.toml merge: the template's lines,
// the repo's lines and parsed data, and what to add where.
type bunfigMerge struct {
	base     any
	repo     any
	after    map[int][]string
	lines    []string
	own      []string
	appended []string
	notes    []string
}

// tomlGet follows path through parsed TOML, or returns nil.
func tomlGet(value any, path tomlPath) any {
	node := value

	for _, part := range path {
		switch key := part.(type) {
		case int:
			list, _ := node.([]any)
			if key >= len(list) {
				return nil
			}

			node = list[key]
		case string:
			table, _ := node.(map[string]any)
			node = table[key]
		}
	}

	return node
}

func asList(value any) []any {
	list, _ := value.([]any)

	return list
}

func containsValue(list []any, value any) bool {
	return slices.ContainsFunc(list, func(item any) bool { return reflect.DeepEqual(item, value) })
}

// tomlText renders a value as the JSON that TOML accepts for strings,
// numbers, booleans, and arrays.
func tomlText(value any) string {
	switch v := value.(type) {
	case string:
		return quote(v)
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, tomlText(item))
		}

		return "[" + strings.Join(items, ", ") + "]"
	case map[string]any:
		keys := slices.Sorted(func(yield func(string) bool) {
			for key := range v {
				if !yield(key) {
					return
				}
			}
		})
		items := make([]string, 0, len(keys))

		for _, key := range keys {
			items = append(items, quote(key)+" = "+tomlText(v[key]))
		}

		return "{ " + strings.Join(items, ", ") + " }"
	default:
		return fmt.Sprint(v)
	}
}

func parseToml(text string) (any, error) {
	var parsed map[string]any
	if err := toml.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}

	return parsed, nil
}

// tomlOrNil parses text, or returns nil when it is not TOML. Callers that
// treat a broken copy as absent use it instead of handling the error.
func tomlOrNil(text string) any {
	parsed, err := parseToml(text)
	if err != nil {
		return nil
	}

	return parsed
}

// valuesOn parses the array entries one line holds, or none when the line
// is only part of an entry.
func valuesOn(line string) []any {
	// The newline keeps a trailing comment from swallowing the `]`.
	parsed, err := parseToml("v = [" + line + "\n]")
	if err != nil {
		return nil
	}

	return asList(tomlGet(parsed, tomlPath{"v"}))
}

func (m *bunfigMerge) insert(line int, text []string) {
	if len(text) > 0 {
		m.after[line] = append(m.after[line], text...)
	}
}

func (m *bunfigMerge) raw(from, end int) string {
	return strings.Join(m.own[from:end+1], "\n")
}

// carriedLines are the repo's lines that hold an entry the template array
// lacks, with the comments right above them.
func (m *bunfigMerge) carriedLines(entry tomlEntry, mine []any) ([]string, []any) {
	var (
		lines    []string
		values   []any
		comments []string
	)

	for _, line := range m.own[entry.start+1 : entry.end] {
		if tomlComment().MatchString(line) {
			comments = append(comments, line)

			continue
		}

		var fresh []any

		for _, value := range valuesOn(line) {
			if !containsValue(mine, value) {
				fresh = append(fresh, value)
			}
		}

		if len(fresh) > 0 {
			lines = append(append(lines, comments...), line)
			values = append(values, fresh...)
		}

		comments = nil
	}

	return lines, values
}

// withCommas makes every entry line but the last end in a comma, and the
// last in none, since bedrock writes no trailing comma where a format allows it.
func withCommas(lines []string) []string {
	last := -1

	for i, line := range lines {
		if !tomlComment().MatchString(line) && strings.TrimSpace(line) != "" {
			last = i
		}
	}

	out := slices.Clone(lines)

	for i, line := range out {
		if tomlComment().MatchString(line) || strings.TrimSpace(line) == "" {
			continue
		}

		code := strings.TrimRight(stripComment(line), " \t")
		switch {
		case i < last && !strings.HasSuffix(code, ","):
			out[i] = code + "," + line[len(code):]
		case i == last && strings.HasSuffix(code, ","):
			out[i] = strings.TrimSuffix(code, ",") + line[len(code):]
		}
	}

	return out
}

// ensureComma gives the array's last template entry a comma, so entries
// inserted after it stay valid TOML.
func (m *bunfigMerge) ensureComma(at tomlEntry) {
	for i := at.end - 1; i > at.start; i-- {
		line := m.lines[i]
		if tomlComment().MatchString(line) || strings.TrimSpace(line) == "" {
			continue
		}

		code := strings.TrimRight(stripComment(line), " \t")
		if !strings.HasSuffix(code, ",") {
			m.lines[i] = code + "," + line[len(code):]
		}

		return
	}
}

func (m *bunfigMerge) mergeArray(entry, at tomlEntry, mine []any) {
	var extra []any

	for _, value := range asList(tomlGet(m.repo, entry.path)) {
		if !containsValue(mine, value) {
			extra = append(extra, value)
		}
	}

	if len(extra) == 0 {
		return
	}

	if at.start == at.end {
		values := make([]string, 0, len(mine)+len(extra))
		for _, value := range append(slices.Clone(mine), extra...) {
			values = append(values, tomlText(value))
		}

		m.lines[at.start] = at.key + " = [" + strings.Join(values, ", ") + "]"

		return
	}

	indent := m.lines[at.end-1][:len(m.lines[at.end-1])-len(strings.TrimLeft(m.lines[at.end-1], " \t"))]
	carried, carriedValues := m.carriedLines(entry, mine)

	var generated []string

	for _, value := range extra {
		if !containsValue(carriedValues, value) {
			generated = append(generated, indent+tomlText(value))
		}
	}

	if len(carried)+len(generated) == 0 {
		return
	}

	m.ensureComma(at)
	m.insert(at.end-1, withCommas(append(carried, generated...)))
}

func (m *bunfigMerge) mergeTable(section, table *tomlSection) {
	keys := map[string]tomlEntry{}
	for _, entry := range table.entries {
		keys[fmt.Sprintf("%#v", entry.path)] = entry
	}

	var added []string

	for _, entry := range section.entries {
		mine := tomlGet(m.base, entry.path)
		theirs := tomlGet(m.repo, entry.path)
		at, found := keys[fmt.Sprintf("%#v", entry.path)]

		switch {
		case mine == nil:
			added = append(added, m.raw(entry.from, entry.end))
		case found && asList(mine) != nil && asList(theirs) != nil:
			m.mergeArray(entry, at, asList(mine))
		case !reflect.DeepEqual(mine, theirs):
			name := joinPath(entry.path)
			m.notes = append(m.notes, "`bunfig.toml`: kept the template's `"+name+" = "+tomlText(mine)+
				"`; the repo had `"+name+" = "+tomlText(theirs)+"`.")
		}
	}

	m.insert(table.end, added)
}

func joinPath(path tomlPath) string {
	parts := make([]string, 0, len(path))
	for _, part := range path {
		parts = append(parts, fmt.Sprint(part))
	}

	return strings.Join(parts, ".")
}

func (m *bunfigMerge) mergeSection(section *tomlSection, tables map[string]*tomlSection) {
	table := tables[fmt.Sprintf("%#v", section.path)]

	isNew := table == nil
	if section.array {
		isNew = !containsValue(
			asList(tomlGet(m.base, section.path[:len(section.path)-1])),
			tomlGet(m.repo, section.path),
		)
	}

	switch {
	case isNew:
		m.appended = append(m.appended, m.raw(section.from, section.end))
	case !section.array:
		m.mergeTable(section, table)
	}
}

func (m *bunfigMerge) render() string {
	out := slices.Clone(m.after[-1])

	for index, line := range m.lines {
		out = append(out, line)
		out = append(out, m.after[index]...)
	}

	var text strings.Builder

	text.WriteString(strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n")

	for _, block := range m.appended {
		text.WriteString("\n" + block + "\n")
	}

	return text.String()
}

// mergeBunfig writes the template's bunfig.toml, then adds what the repo's
// copy adds to it: each key a table lacks, each entry a template array
// lacks, and each table or `[[…]]` element the template lacks, with the
// comments right above them. Where both set a value, the template's wins,
// since bunfig.toml cannot extend a shared file; each such value gets a
// note. A nil current means the repo has no bunfig.toml.
func mergeBunfig(template string, current *string) (string, []string, error) {
	if current == nil {
		return template, nil, nil
	}

	repo := tomlOrNil(*current)
	if repo == nil {
		return template, []string{
			"`bunfig.toml`: the repo's copy is not valid TOML, so adopt wrote the template. " +
				"Its settings are in git history.",
		}, nil
	}

	base, err := parseToml(template)
	if err != nil {
		return "", nil, fmt.Errorf("bedrock's `bunfig.toml`: %w", err)
	}

	merge := &bunfigMerge{
		base: base, repo: repo, after: map[int][]string{},
		lines: strings.Split(template, "\n"), own: strings.Split(*current, "\n"),
		appended: nil, notes: nil,
	}

	tables := map[string]*tomlSection{}
	for _, section := range scanToml(merge.lines) {
		tables[fmt.Sprintf("%#v", section.path)] = section
	}

	for _, section := range scanToml(merge.own) {
		merge.mergeSection(section, tables)
	}

	return merge.render(), merge.notes, nil
}
