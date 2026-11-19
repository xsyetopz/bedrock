// Package editorconfig reads .editorconfig files as the EditorConfig spec
// does, for the bedrock settings and the size bounds they record.
package editorconfig

import (
	"strings"
)

// Pair is one key = value line. Keys are lower case.
type Pair struct {
	Key   string
	Value string
}

// Section is one [glob] section, with its pairs in file order.
type Section struct {
	Glob  string
	Pairs []Pair
}

// Parse returns the sections of an .editorconfig: full-line # and ;
// comments are skipped, keys are lowercased, and the pairs before the first
// section, such as root, are dropped.
func Parse(text string) []Section {
	var sections []Section

	for raw := range strings.Lines(text) {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, Section{Glob: line[1 : len(line)-1], Pairs: nil})

			continue
		}

		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)

		if !found || key == "" || len(sections) == 0 {
			continue
		}

		last := &sections[len(sections)-1]
		last.Pairs = append(last.Pairs, Pair{
			Key:   strings.ToLower(key),
			Value: strings.TrimSpace(value),
		})
	}

	return sections
}
