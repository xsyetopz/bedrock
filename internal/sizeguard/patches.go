package sizeguard

import (
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock"
)

// hook is the sitecustomize module that applies the patches as lizard
// imports the modules they change, so the installed release stays untouched
// and every runner, PATH, uvx, or pipx, measures with the same fixes.
//
//go:embed lizard/sitecustomize.py
var hook []byte

// stageHook writes the hook and bedrock's lizard patches into a new
// directory and returns it with the environment that puts it first on
// PYTHONPATH. The caller removes the directory.
func stageHook() (string, []string, error) {
	dir, err := os.MkdirTemp("", "bedrock-lizard-")
	if err != nil {
		return "", nil, fmt.Errorf("stage the `lizard` patches: %w", err)
	}

	err = writeHook(dir, bedrock.LizardPatches())
	if err != nil {
		_ = os.RemoveAll(dir)

		return "", nil, err
	}

	path := dir
	if existing := os.Getenv("PYTHONPATH"); existing != "" {
		path += string(os.PathListSeparator) + existing
	}

	return dir, []string{"PYTHONPATH=" + path}, nil
}

// writeHook copies the hook and each *.patch in patches into dir.
func writeHook(dir string, patches fs.FS) error {
	const mode = 0o600

	err := os.WriteFile(filepath.Join(dir, "sitecustomize.py"), hook, mode)
	if err != nil {
		return fmt.Errorf("stage the `lizard` hook: %w", err)
	}

	names, err := fs.Glob(patches, "*.patch")
	if err != nil {
		return fmt.Errorf("list the `lizard` patches: %w", err)
	}

	for _, name := range names {
		data, readErr := fs.ReadFile(patches, name)
		if readErr != nil {
			return fmt.Errorf("read `%s`: %w", name, readErr)
		}

		err = os.WriteFile(filepath.Join(dir, name), data, mode)
		if err != nil {
			return fmt.Errorf("stage `%s`: %w", strings.TrimSuffix(name, ".patch"), err)
		}
	}

	return nil
}
