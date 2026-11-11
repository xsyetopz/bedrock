// Package scaffold plans and writes the files go-bedrock manages in a Go module.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed files/*
var files embed.FS

const (
	dirMode  = 0o755
	fileMode = 0o644
	modFile  = "go.mod"
	justFile = ".just/go.just"
	lintFile = ".golangci.yml"
	// placeholder is the module path the embedded lint config holds until render swaps it.
	placeholder = "github.com/my/project"
)

// ErrNoModule reports a directory whose go.mod has no module line.
var ErrNoModule = errors.New("go.mod has no module line")

// Action is what Apply does with one path.
type Action string

const (
	// Create writes a file that does not exist yet.
	Create Action = "create"
	// Replace overwrites a managed file that differs from go-bedrock's copy.
	Replace Action = "replace"
	// Keep leaves a file alone, because it is current or the project owns it.
	Keep Action = "keep"
)

// Entry is one file in a plan.
type Entry struct {
	Path    string
	Action  Action
	content []byte
}

// ModulePath reads the module path from the go.mod in dir.
func ModulePath(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, modFile))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}

	return "", ErrNoModule
}

// AdoptPlan lists what adopting dir, a module named module, changes. The lint
// config belongs to the project once it exists; the recipes stay managed.
func AdoptPlan(dir, module string) ([]Entry, error) {
	lint, err := render("files/golangci.yml", module)
	if err != nil {
		return nil, err
	}

	recipes, err := files.ReadFile("files/go.just")
	if err != nil {
		return nil, fmt.Errorf("read embedded recipes: %w", err)
	}

	return []Entry{
		classify(dir, lintFile, lint, false),
		classify(dir, justFile, recipes, true),
	}, nil
}

// NewPlan lists the files of a new module: go.mod, one command under cmd/, and
// the managed files. The project owns everything but the recipes.
func NewPlan(dir, module, name string) ([]Entry, error) {
	managed, err := AdoptPlan(dir, module)
	if err != nil {
		return nil, err
	}

	goMod := fmt.Sprintf("module %s\n\ngo 1.27\n", module)
	mainGo := "package main\n\nfunc main() {}\n"

	return append([]Entry{
		classify(dir, modFile, []byte(goMod), false),
		classify(dir, filepath.Join("cmd", name, "main.go"), []byte(mainGo), false),
	}, managed...), nil
}

// Apply writes every Create and Replace entry of plan under dir.
func Apply(dir string, plan []Entry) error {
	for _, entry := range plan {
		if entry.Action == Keep {
			continue
		}

		path := filepath.Join(dir, entry.Path)

		err := os.MkdirAll(filepath.Dir(path), dirMode)
		if err != nil {
			return fmt.Errorf("create folder for %s: %w", entry.Path, err)
		}

		err = os.WriteFile(path, entry.content, fileMode)
		if err != nil {
			return fmt.Errorf("write %s: %w", entry.Path, err)
		}
	}

	return nil
}

func render(name, module string) ([]byte, error) {
	data, err := files.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", name, err)
	}

	return bytes.ReplaceAll(data, []byte(placeholder), []byte(module)), nil
}

func classify(dir, path string, content []byte, managed bool) Entry {
	current, err := os.ReadFile(filepath.Join(dir, path))

	switch {
	case err != nil:
		return Entry{Path: path, Action: Create, content: content}
	case managed && !bytes.Equal(current, content):
		return Entry{Path: path, Action: Replace, content: content}
	default:
		return Entry{Path: path, Action: Keep, content: content}
	}
}
