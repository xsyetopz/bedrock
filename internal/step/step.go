// Package step models the changes an adopt makes to a repository. A plan is
// a list of steps; each knows whether the repository already matches it,
// so a second adopt of the same repository plans none.
package step

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// Step is one change, relative to the repository root.
//
//sumtype:decl
type Step interface {
	// Target is the slash-separated path the step leaves changed.
	Target() string
	sealed()
}

// Write puts Content at Path.
type Write struct {
	Path string
	// Content is text, or the bytes of a binary file such as a jar.
	Content []byte
	// Executable marks a program, such as gradlew.
	Executable bool
}

// Remove deletes Path, a directory included.
type Remove struct {
	Path string
}

// Link makes Path a symlink to Dest.
type Link struct {
	Path string
	Dest string
}

// Move renames From to Path, keeping its content and mode.
type Move struct {
	Path string
	From string
}

// Target returns the path written.
func (s Write) Target() string { return s.Path }

// Target returns the path removed.
func (s Remove) Target() string { return s.Path }

// Target returns the link's path.
func (s Link) Target() string { return s.Path }

// Target returns the path moved to.
func (s Move) Target() string { return s.Path }

func (Write) sealed()  {}
func (Remove) sealed() {}
func (Link) sealed()   {}
func (Move) sealed()   {}

const (
	executableBits = 0o111
	executableMode = 0o755
	directoryMode  = 0o755
	fileMode       = 0o644
)

// Describe returns the step's plan line, its verb padded so paths align.
func Describe(root string, s Step) string {
	switch s := s.(type) {
	case Link:
		return "link    " + s.Path + " -> " + s.Dest
	case Remove:
		return "remove  " + s.Path
	case Move:
		return "move    " + s.From + " -> " + s.Path
	case Write:
		if exists(join(root, s.Path)) {
			return "update  " + s.Path
		}

		return "create  " + s.Path
	}

	return s.Target()
}

// Done reports whether root already matches the step.
func Done(root string, s Step) (bool, error) {
	switch s := s.(type) {
	case Remove:
		return !exists(join(root, s.Path)), nil
	case Link:
		dest, err := os.Readlink(join(root, s.Path))

		return err == nil && dest == s.Dest, nil
	case Move:
		return exists(join(root, s.Path)) && !exists(join(root, s.From)), nil
	case Write:
		return writeDone(join(root, s.Path), s)
	}

	return false, nil
}

func writeDone(path string, s Write) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("stat `%s`: %w", s.Path, err)
	}

	// Windows has no execute bit, and Go reports none there, so only content counts.
	unmarked := s.Executable && runtime.GOOS != "windows" && info.Mode()&executableBits == 0
	if !info.Mode().IsRegular() || unmarked {
		return false, nil
	}

	current, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read `%s`: %w", s.Path, err)
	}

	return sameText(current, s.Content), nil
}

// sameText reports whether current holds content, counting a CRLF in a text
// file as the LF bedrock writes: git checks text out with CRLF where
// `.gitattributes` says `eol=crlf`, as it does for `*.bat`, or where
// `core.autocrlf` is on, and the file is still the one bedrock wrote. A binary
// file, one with a NUL byte, must match byte for byte.
func sameText(current, content []byte) bool {
	if bytes.Equal(current, content) {
		return true
	}

	if bytes.IndexByte(content, 0) >= 0 || bytes.IndexByte(current, 0) >= 0 {
		return false
	}

	crlf, lf := []byte("\r\n"), []byte("\n")

	return bytes.Equal(bytes.ReplaceAll(current, crlf, lf), bytes.ReplaceAll(content, crlf, lf))
}

// Pending returns the steps root does not match yet, in order.
func Pending(root string, steps []Step) ([]Step, error) {
	var pending []Step

	for _, s := range steps {
		done, err := Done(root, s)
		if err != nil {
			return nil, err
		}

		if !done {
			pending = append(pending, s)
		}
	}

	return pending, nil
}

// Dedupe keeps the last step for each path, so a language's file replaces
// the base file at the same path, and orders the result by path.
func Dedupe(steps []Step) []Step {
	last := make(map[string]Step, len(steps))
	for _, s := range steps {
		last[s.Target()] = s
	}

	deduped := make([]Step, 0, len(last))
	for _, s := range last {
		deduped = append(deduped, s)
	}

	slices.SortFunc(deduped, func(a, b Step) int { return cmp.Compare(a.Target(), b.Target()) })

	return deduped
}

// Apply makes each step's change. Removals go first, so a file can take the
// place of a directory a removal clears. Nothing is backed up: adopt runs
// only when git holds every file a step touches, so git diff reviews the
// change and git checkout undoes it.
func Apply(root string, steps []Step) error {
	for _, s := range steps {
		if remove, ok := s.(Remove); ok {
			if err := os.RemoveAll(join(root, remove.Path)); err != nil {
				return fmt.Errorf("remove `%s`: %w", remove.Path, err)
			}
		}
	}

	for _, s := range steps {
		if err := apply(root, s); err != nil {
			return err
		}
	}

	return nil
}

func apply(root string, s Step) error {
	switch s := s.(type) {
	case Remove:
		return nil
	case Write:
		return write(join(root, s.Path), s)
	case Link:
		return replace(join(root, s.Path), func(path string) error { return os.Symlink(s.Dest, path) })
	case Move:
		return replace(join(root, s.Path), func(path string) error { return os.Rename(join(root, s.From), path) })
	}

	return nil
}

// replace clears path and its parents' way, then creates it with make.
func replace(path string, create func(path string) error) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("clear `%s`: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), directoryMode); err != nil {
		return fmt.Errorf("create `%s`: %w", filepath.Dir(path), err)
	}

	if err := create(path); err != nil {
		return fmt.Errorf("create `%s`: %w", path, err)
	}

	return nil
}

// write replaces the file atomically: the content goes to a temporary file
// beside it, which takes over the old file's mode and then its name, so an
// interrupted run never leaves a half-written config.
func write(path string, s Write) error {
	mode := fs.FileMode(fileMode)

	if info, statErr := os.Lstat(path); statErr == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	if s.Executable {
		mode = executableMode
	}

	return replace(path, func(path string) error {
		temp, err := os.CreateTemp(filepath.Dir(path), ".bedrock-*")
		if err != nil {
			return fmt.Errorf("temporary file: %w", err)
		}

		defer func() { _ = os.Remove(temp.Name()) }()

		_, err = temp.Write(s.Content)
		err = errors.Join(err, temp.Chmod(mode), temp.Close())

		if err != nil {
			return fmt.Errorf("write `%s`: %w", s.Path, err)
		}

		return os.Rename(temp.Name(), path)
	})
}

func join(root, path string) string {
	return filepath.Join(root, filepath.FromSlash(path))
}

func exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}
