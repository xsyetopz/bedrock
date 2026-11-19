// Package blocks edits marked regions of shared files, such as .gitignore or
// AGENTS.md, so the base and each language own one block apiece while the
// project keeps the rest of the file.
package blocks

import (
	"slices"
	"strings"
)

// Syntax is the comment syntax of a file that holds marked blocks.
type Syntax int

const (
	// Hash marks blocks with "# >>> name" and "# <<< name".
	Hash Syntax = iota
	// Slash marks blocks with "// >>> name" and "// <<< name".
	Slash
	// Markdown marks blocks with "<!-- name -->" and "<!-- /name -->".
	Markdown
)

// Block is one named region and the text between its markers, which ends
// in a newline.
type Block struct {
	Name    string
	Content string
}

// markerLines are the lines that open and close one block.
type markerLines struct {
	Open  string
	Close string
}

// markers returns the marker lines of the block name.
func markers(syntax Syntax, name string) markerLines {
	switch syntax {
	case Markdown:
		return markerLines{Open: "<!-- " + name + " -->", Close: "<!-- /" + name + " -->"}
	case Slash:
		return markerLines{Open: "// >>> " + name, Close: "// <<< " + name}
	case Hash:
		return markerLines{Open: "# >>> " + name, Close: "# <<< " + name}
	}

	return markerLines{Open: "# >>> " + name, Close: "# <<< " + name}
}

// Has reports whether text holds the opening marker of block name.
func Has(text string, syntax Syntax, name string) bool {
	open := markers(syntax, name).Open

	return lineStart(text, open) >= 0
}

// Remove returns text without the block name, or text when it has none.
// The blank line that separated the block from the next one goes with it,
// or, for the last block, the one that separated it from the block before.
func Remove(text string, syntax Syntax, name string) string {
	lines := markers(syntax, name)
	open, closing := lines.Open, lines.Close

	start := lineStart(text, open)
	if start < 0 {
		return text
	}

	closeAt := strings.Index(text[start:], closing)
	if closeAt < 0 {
		return text
	}

	end := start + closeAt + len(closing)
	end += newlineAt(text, end)

	separated := start == 0 || (start >= 2 && text[start-2:start] == "\n\n")
	if separated {
		end += newlineAt(text, end)
	}

	from := start
	if separated && start > 0 && end == len(text) {
		from = start - 1
	}

	return text[:from] + text[end:]
}

// Upsert returns text with each block replaced in place or, when missing,
// inserted before the first block that order lists after it, else appended.
// order names every block a file may hold, in the order files list them.
func Upsert(text string, syntax Syntax, order []string, blocks ...Block) string {
	for _, block := range blocks {
		text = upsertOne(text, syntax, order, block)
	}

	return text
}

func upsertOne(text string, syntax Syntax, order []string, block Block) string {
	lines := markers(syntax, block.Name)
	open, closing := lines.Open, lines.Close
	marked := open + "\n" + block.Content + closing + "\n"

	start := lineStart(text, open)
	if start >= 0 {
		if closeAt := strings.Index(text[start:], closing); closeAt >= 0 {
			end := start + closeAt + len(closing)

			return text[:start] + marked + text[end+newlineAt(text, end):]
		}
	}

	if next := laterBlock(text, syntax, order, block.Name); next >= 0 {
		return text[:next] + marked + "\n" + text[next:]
	}

	switch {
	case text == "", strings.HasSuffix(text, "\n\n"):
	case strings.HasSuffix(text, "\n"):
		text += "\n"
	default:
		text += "\n\n"
	}

	return text + marked
}

// lineStart returns where the first line starting with marker begins, or -1.
func lineStart(text, marker string) int {
	if strings.HasPrefix(text, marker) {
		return 0
	}

	if index := strings.Index(text, "\n"+marker); index >= 0 {
		return index + 1
	}

	return -1
}

// laterBlock returns where the first block that order lists after name
// starts in text, or -1.
func laterBlock(text string, syntax Syntax, order []string, name string) int {
	at := slices.Index(order, name)
	if at < 0 {
		return -1
	}

	first := -1

	for _, later := range order[at+1:] {
		open := markers(syntax, later).Open
		if index := lineStart(text, open); index >= 0 && (first < 0 || index < first) {
			first = index
		}
	}

	return first
}

func newlineAt(text string, index int) int {
	if index < len(text) && text[index] == '\n' {
		return 1
	}

	return 0
}
