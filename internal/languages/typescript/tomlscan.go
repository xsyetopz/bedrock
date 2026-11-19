package typescript

import (
	"regexp"
	"strings"
)

// tomlPath is the keys from the root to a value; an int indexes an array of
// tables.
type tomlPath []any

// tomlEntry is one key and the line range its value covers.
type tomlEntry struct {
	key  string
	path tomlPath
	// from is the first line of the comments right above the key.
	from  int
	start int
	end   int
}

// tomlSection is the root or one table, with the line range of every key.
type tomlSection struct {
	path    tomlPath
	entries []tomlEntry
	from    int
	end     int
	array   bool
}

func tomlHeader() *regexp.Regexp {
	return regexp.MustCompile(`^\s*(\[\[?)\s*([^\]]+?)\s*\]\]?\s*(?:#.*)?$`)
}

func tomlKey() *regexp.Regexp { return regexp.MustCompile(`^\s*([^\s#=\[][^=]*?)\s*=`) }

func tomlComment() *regexp.Regexp { return regexp.MustCompile(`^\s*#`) }

// unquote strips one pair of matching quotes.
func unquote(part string) string {
	if len(part) >= 2 && (part[0] == '"' || part[0] == '\'') && part[len(part)-1] == part[0] {
		return part[1 : len(part)-1]
	}

	return part
}

func keyPath(text string) tomlPath {
	var path tomlPath
	for part := range strings.SplitSeq(text, ".") {
		path = append(path, unquote(strings.TrimSpace(part)))
	}

	return path
}

// stripComment returns the line without a trailing `#` comment.
func stripComment(line string) string {
	quote := rune(0)

	for i, char := range line {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '"' || char == '\'':
			quote = char
		case char == '#':
			return line[:i]
		}
	}

	return line
}

// brackets counts the arrays the line opens minus the ones it closes,
// outside strings and comments.
func brackets(line string) int {
	depth := 0

	for _, char := range stripComment(line) {
		switch char {
		case '[':
			depth++
		case ']':
			depth--
		}
	}

	return depth
}

// valueEnd is the last line of the value that starts on start.
func valueEnd(lines []string, start, keyLength int) int {
	end := start
	depth := brackets(lines[start][keyLength:])

	for depth > 0 && end+1 < len(lines) {
		end++
		depth += brackets(lines[end])
	}

	return end
}

func startSection(header []string, from, start int, counts map[string]int) *tomlSection {
	name := header[2]
	array := header[1] == "[["
	count := counts[name]
	counts[name] = count + 1

	path := keyPath(name)
	if array {
		path = append(path, count)
	}

	return &tomlSection{path: path, entries: nil, from: from, end: start, array: array}
}

// scanToml splits a TOML file into its root and its tables, each with the
// line range of every key.
func scanToml(lines []string) []*tomlSection {
	section := &tomlSection{path: nil, entries: nil, from: 0, end: -1, array: false}
	sections := []*tomlSection{section}
	counts := map[string]int{}
	from := -1

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if tomlComment().MatchString(line) {
			if from < 0 {
				from = index
			}

			continue
		}

		first := index
		if from >= 0 {
			first = from
		}

		if header := tomlHeader().FindStringSubmatch(line); header != nil {
			section = startSection(header, first, index, counts)
			sections = append(sections, section)
		} else if key := tomlKey().FindStringSubmatch(line); key != nil {
			end := valueEnd(lines, index, len(key[0]))
			path := append(append(tomlPath{}, section.path...), keyPath(key[1])...)
			section.entries = append(
				section.entries,
				tomlEntry{key: key[1], path: path, from: first, start: index, end: end},
			)
			section.end = end
			index = end
		}

		from = -1
	}

	return sections
}
