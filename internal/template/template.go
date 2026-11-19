// Package template renders bedrock's embedded templates. A template's text
// fills @@name@@ placeholders, and its path fills __name__ ones, since globs
// and shells read underscores literally. Every template ends in .tmpl, so no
// tool reads one as live config.
package template

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Suffix ends every template file.
const Suffix = ".tmpl"

// Variables are values for placeholders, such as the package name.
type Variables map[string]string

// With returns a copy of v with extra laid over it.
func (v Variables) With(extra Variables) Variables {
	merged := make(Variables, len(v)+len(extra))
	maps.Copy(merged, v)
	maps.Copy(merged, extra)

	return merged
}

// MissingValueError reports a placeholder without a value, a defect in
// bedrock itself rather than in the repository.
type MissingValueError struct {
	From        string
	Placeholder string
}

func (e *MissingValueError) Error() string {
	return fmt.Sprintf("%s: no value for %s", e.From, e.Placeholder)
}

func textPlaceholder() *regexp.Regexp { return regexp.MustCompile(`@@([a-z][a-zA-Z]*)@@`) }

func pathPlaceholder() *regexp.Regexp { return regexp.MustCompile(`__([a-z][a-zA-Z]*)__`) }

// Render fills each @@name@@ placeholder in text; from names the template
// in the error.
func Render(text string, vars Variables, from string) (string, error) {
	return fill(textPlaceholder(), text, vars, from)
}

// RenderPath fills each __name__ placeholder in a template's target path.
func RenderPath(target string, vars Variables) (string, error) {
	return fill(pathPlaceholder(), target, vars, target)
}

func fill(pattern *regexp.Regexp, text string, vars Variables, from string) (string, error) {
	var missing error

	filled := pattern.ReplaceAllStringFunc(text, func(placeholder string) string {
		name := pattern.FindStringSubmatch(placeholder)[1]

		value, ok := vars[name]
		if !ok && missing == nil {
			missing = &MissingValueError{From: from, Placeholder: placeholder}
		}

		return value
	})

	return filled, missing
}

// Rendered is one template as a repository receives it.
type Rendered struct {
	// Path is the target path, slash-separated, with placeholders filled.
	Path string
	// Content is the filled text, or the bytes of a binary file unchanged.
	Content []byte
}

// Dir is a read-only view of one template directory, such as rust/files.
type Dir struct {
	fsys fs.FS
	dir  string
}

// NewDir returns the templates under dir in fsys.
func NewDir(fsys fs.FS, dir string) Dir {
	return Dir{fsys: fsys, dir: dir}
}

// Name returns the directory, such as rust/files.
func (d Dir) Name() string {
	return d.dir
}

// Paths returns the target paths, without Suffix and before placeholders
// are filled, of every template under the directory, sorted. A missing
// directory has none.
func (d Dir) Paths() ([]string, error) {
	var paths []string

	err := fs.WalkDir(d.fsys, d.dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && strings.HasSuffix(name, Suffix) {
			paths = append(paths, strings.TrimSuffix(strings.TrimPrefix(name, d.dir+"/"), Suffix))
		}

		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("templates `%s`: %w", d.dir, err)
	}

	slices.Sort(paths)

	return paths, nil
}

// Read returns the rendered template for a target path, or false when the
// directory has none.
func (d Dir) Read(target string, vars Variables) (string, bool, error) {
	data, err := fs.ReadFile(d.fsys, path.Join(d.dir, target+Suffix))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("template `%s/%s`: %w", d.dir, target, err)
	}

	text, err := Render(string(data), vars, d.dir+"/"+target)

	return text, err == nil, err
}

// Files returns every template, with path and text filled from vars and
// binary files as their bytes, sorted by template path.
func (d Dir) Files(vars Variables) ([]Rendered, error) {
	paths, err := d.Paths()
	if err != nil {
		return nil, err
	}

	files := make([]Rendered, 0, len(paths))

	for _, target := range paths {
		file, renderErr := d.render(target, vars)
		if renderErr != nil {
			return nil, renderErr
		}

		files = append(files, file)
	}

	return files, nil
}

// render fills one template. A file holding a NUL byte, as a jar or an
// image does, is binary and stays as it is; git tells the two apart the
// same way.
func (d Dir) render(target string, vars Variables) (Rendered, error) {
	from := d.dir + "/" + target

	data, err := fs.ReadFile(d.fsys, path.Join(d.dir, target+Suffix))
	if err != nil {
		return Rendered{}, fmt.Errorf("template `%s`: %w", from, err)
	}

	filledPath, err := RenderPath(target, vars)
	if err != nil {
		return Rendered{}, err
	}

	if bytes.IndexByte(data, 0) >= 0 {
		return Rendered{Path: filledPath, Content: data}, nil
	}

	text, err := Render(string(data), vars, from)
	if err != nil {
		return Rendered{}, err
	}

	return Rendered{Path: filledPath, Content: []byte(text)}, nil
}
