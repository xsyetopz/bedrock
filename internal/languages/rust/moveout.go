package rust

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

const (
	directoryMode os.FileMode = 0o755
	fileMode      os.FileMode = 0o644
)

// edit replaces source[from:to] with text.
type edit struct {
	text string
	from int
	to   int
}

// fileWrite is a file the move creates.
type fileWrite struct {
	path string
	text string
}

// save writes the file, which already has its directory.
func (w fileWrite) save() error {
	return os.WriteFile(w.path, []byte(w.text), fileMode)
}

// movePlan is what moving the test modules of one file needs.
type movePlan struct {
	edits    []edit
	writes   []fileWrite
	removals []string
	problems []string
}

// claimed reports whether destination exists or an earlier module of the plan
// writes it.
func (p *movePlan) claimed(destination string) bool {
	return fsutil.Exists(destination) ||
		slices.ContainsFunc(p.writes, func(write fileWrite) bool { return write.path == destination })
}

func failure(operation, path string, err error) string {
	return fmt.Sprintf("`%s`: cannot %s: %s", path, operation, err)
}

func sameFile(left, right string) bool {
	first, err := filepath.EvalSymlinks(left)
	if err != nil {
		return false
	}

	second, err := filepath.EvalSymlinks(right)

	return err == nil && first == second
}

// pastSpace returns the offset past end and the whitespace that follows it.
func pastSpace(source string, end int) int {
	rest := source[end:]

	return end + len(rest) - len(strings.TrimLeftFunc(rest, unicode.IsSpace))
}

// planPathModule plans the move of a `#[path]` test file to destination.
func (p *movePlan) planPathModule(file, source string, found module, destination string) {
	place := fmt.Sprintf("%s:%d", file, found.line)

	current, exists := childFile(file, found)
	if found.pathAttr == nil || !exists {
		p.problems = append(p.problems, fmt.Sprintf("`%s`: cannot find the file named by `#[path]`", place))

		return
	}

	dropAttribute := edit{from: found.pathAttr.from, to: pastSpace(source, found.pathAttr.to), text: ""}

	if sameFile(current, destination) {
		p.edits = append(p.edits, dropAttribute)

		return
	}

	data, err := os.ReadFile(current)
	if err != nil {
		p.problems = append(p.problems, failure("read", current, err))

		return
	}

	text := string(data)

	if p.claimed(destination) {
		p.problems = append(p.problems, fmt.Sprintf("`%s`: cannot move `%s`: `%s` exists", place, current, destination))

		return
	}

	// A relative path inside the file would resolve from another directory.
	if rewritten, ok := prefixPaths(text, pathPrefixes{include: "moved/", path: "moved/"}); !ok || rewritten != text {
		p.problems = append(p.problems, fmt.Sprintf(
			"`%s`: cannot move `%s`: it names files by relative path, which would stop resolving", place, current))

		return
	}

	p.edits = append(p.edits, dropAttribute)
	p.writes = append(p.writes, fileWrite{path: destination, text: text})
	p.removals = append(p.removals, current)
}

// planModule plans the move of one top-level test module, or reports why it
// cannot move.
func (p *movePlan) planModule(file, source string, found module) {
	place := fmt.Sprintf("%s:%d", file, found.line)

	if found.depth > 0 {
		p.problems = append(p.problems, fmt.Sprintf(
			"`%s`: cannot move `mod %s` out of another module; declare `#[cfg(test)] mod %s;` at the top level of the file",
			place,
			found.name,
			found.name,
		))

		return
	}

	destination := testsFile(file, found.name)

	if found.body == nil {
		p.planPathModule(file, source, found, destination)

		return
	}

	if p.claimed(destination) {
		p.problems = append(p.problems, fmt.Sprintf(
			"`%s`: cannot move `mod %s`: `%s` exists", place, found.name, destination))

		return
	}

	open, closeAt := found.body.from, found.body.to
	body := strings.TrimLeft(source[open+1:closeAt], "\r\n")

	// An include resolves from the file, which may sit a directory deeper now,
	// and a `#[path]` from the module's directory, `name/` until now.
	up := ""
	if rel, err := filepath.Rel(filepath.Dir(destination), filepath.Dir(file)); err == nil && rel != "." {
		up = filepath.ToSlash(rel) + "/"
	}

	text, ok := prefixPaths(body, pathPrefixes{include: up, path: found.name + "/"})
	if !ok {
		p.problems = append(p.problems, fmt.Sprintf(
			"`%s`: cannot move `mod %s`: it names a file by a path that cannot be rewritten", place, found.name))

		return
	}

	p.writes = append(p.writes, fileWrite{path: destination, text: text})

	// The declaration keeps its own spelling, such as `r#type`.
	declaration := strings.TrimRight(source[found.start:open], " \t\r\n\v\f") + ";"
	p.edits = append(p.edits, edit{from: found.start, to: closeAt + 1, text: declaration})
}

// planFile plans every module of source that can move, one at a time: each
// module's check sees the files the earlier ones claim.
func planFile(file, source string) *movePlan {
	plan := &movePlan{edits: nil, writes: nil, removals: nil, problems: nil}

	movable := outermost(slices.DeleteFunc(modules(source), func(found module) bool {
		return !found.testOnly || (found.body == nil && found.pathAttr == nil)
	}))

	for _, found := range movable {
		plan.planModule(file, source, found)
	}

	return plan
}

// writeAll creates the files of writes and returns one message per failure.
func writeAll(writes []fileWrite) []string {
	var failed []string

	for _, write := range writes {
		if err := os.MkdirAll(filepath.Dir(write.path), directoryMode); err != nil {
			failed = append(failed, failure("write", write.path, err))

			continue
		}

		if err := write.save(); err != nil {
			failed = append(failed, failure("write", write.path, err))
		}
	}

	return failed
}

// applyEdits replaces the ranges of edits in source, last first so earlier
// offsets stay valid.
func applyEdits(source string, edits []edit) string {
	ordered := slices.Clone(edits)
	slices.SortFunc(ordered, func(a, b edit) int { return b.from - a.from })

	updated := source
	for _, change := range ordered {
		updated = updated[:change.from] + change.text + updated[change.to:]
	}

	return updated
}

// moveOut moves each test module `name` of file to the `name.rs` where its
// child modules sit and leaves `#[cfg(test)] mod name;`, dropping a `#[path]`
// attribute on the way. It returns one message per module it cannot move, or
// per I/O failure; the other modules still move.
func moveOut(file string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return []string{failure("read", file, err)}
	}

	source := string(data)
	plan := planFile(file, source)

	if len(plan.edits) == 0 {
		return plan.problems
	}

	if written := writeAll(plan.writes); len(written) > 0 {
		return append(plan.problems, written...)
	}

	if saveErr := (fileWrite{path: file, text: applyEdits(source, plan.edits)}).save(); saveErr != nil {
		return append(plan.problems, failure("write", file, saveErr))
	}

	problems := plan.problems

	for _, old := range plan.removals {
		if removeErr := os.Remove(old); removeErr != nil {
			problems = append(problems, failure("remove", old, removeErr))
		}
	}

	return problems
}
