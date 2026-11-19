package editorconfig

import (
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// Name is the file name the EditorConfig spec reads in each directory.
const Name = ".editorconfig"

// File is one .editorconfig on disk.
type File struct {
	// Path is the file's path, relative to the repository root.
	Path string
	// Dir is its directory, relative to the repository root; "" for the root itself.
	Dir string
	// Sections are its [glob] sections, in file order.
	Sections []Section
	// Root is true when its preamble sets root = true, which ends the search upward.
	Root bool
}

// Properties returns the properties that apply to path, relative to the
// .editorconfig the sections come from: each matching section in order, a
// later pair overriding an earlier one, and unset removing a pair.
func Properties(sections []Section, path string) (map[string]string, error) {
	properties := map[string]string{}

	if err := apply(sections, path, properties, map[string]*Glob{}); err != nil {
		return nil, err
	}

	return properties, nil
}

func apply(sections []Section, path string, properties map[string]string, globs map[string]*Glob) error {
	for _, section := range sections {
		glob, ok := globs[section.Glob]
		if !ok {
			compiled, err := Compile(section.Glob)
			if err != nil {
				return err
			}

			glob = compiled
			globs[section.Glob] = glob
		}

		if !glob.Match(path) {
			continue
		}

		for _, pair := range section.Pairs {
			if strings.EqualFold(pair.Value, "unset") {
				delete(properties, pair.Key)
			} else {
				properties[pair.Key] = pair.Value
			}
		}
	}

	return nil
}

// Tree reads the .editorconfig files under one root as the paths that need
// them ask, each once. It is not safe for concurrent use.
type Tree struct {
	// loaded holds each directory read so far; a nil file is a directory
	// with no .editorconfig.
	loaded map[string]*File
	globs  map[string]*Glob
	root   string
}

// NewTree returns a tree over the repository at root.
func NewTree(root string) *Tree {
	return &Tree{loaded: map[string]*File{}, globs: map[string]*Glob{}, root: root}
}

// Properties returns the properties of path, relative to the repository
// root. They come from the .editorconfig files in its directory and each one
// above it, up to the nearest that sets root = true or the repository root,
// applied from the top down, so a nested file overrides the root one.
func (t *Tree) Properties(file string) (map[string]string, error) {
	var files []*File

	for _, dir := range ancestors(file) {
		found, err := t.load(dir)
		if err != nil {
			return nil, err
		}

		if found != nil {
			files = append(files, found)
		}
	}

	properties := map[string]string{}

	top := 0

	for i, found := range files {
		if found.Root {
			top = i
		}
	}

	for _, found := range files[top:] {
		relative := file
		if found.Dir != "" {
			relative = file[len(found.Dir)+1:]
		}

		if err := apply(found.Sections, relative, properties, t.globs); err != nil {
			return nil, err
		}
	}

	return properties, nil
}

// Files returns every .editorconfig read so far, the root one always among
// them, ordered by path.
func (t *Tree) Files() ([]File, error) {
	if _, err := t.load(""); err != nil {
		return nil, err
	}

	var files []File

	for _, found := range t.loaded {
		if found != nil {
			files = append(files, *found)
		}
	}

	slices.SortFunc(files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })

	return files, nil
}

// load reads the .editorconfig in dir, or returns nil when it has none.
func (t *Tree) load(dir string) (*File, error) {
	if found, ok := t.loaded[dir]; ok {
		return found, nil
	}

	name := path.Join(dir, Name)

	text, ok, err := fsutil.ReadText(filepath.Join(t.root, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}

	var found *File
	if ok {
		found = &File{Path: name, Dir: dir, Sections: Parse(text), Root: DeclaresRoot(text)}
	}

	t.loaded[dir] = found

	return found, nil
}

// ancestors returns "" and each directory above file, from the top down.
func ancestors(file string) []string {
	parts := strings.Split(file, "/")
	parts = parts[:len(parts)-1]
	dirs := []string{""}

	for i := range parts {
		dirs = append(dirs, strings.Join(parts[:i+1], "/"))
	}

	return dirs
}

// DeclaresRoot reports whether the preamble of an .editorconfig, the lines
// before its first section, sets root = true.
func DeclaresRoot(text string) bool {
	preamble := text
	if loc := regexp.MustCompile(`(?m)^\s*\[`).FindStringIndex(text); loc != nil {
		preamble = text[:loc[0]]
	}

	return regexp.MustCompile(`(?im)^\s*root\s*=\s*true\s*$`).MatchString(preamble)
}
