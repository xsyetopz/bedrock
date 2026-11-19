// Package bedrock holds what every bedrock build embeds: the templates each
// adopt renders and the release version from package.json, so a binary
// carries everything it writes and needs no checkout beside it.
package bedrock

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
)

// templateFS holds templates/<part>/<dir>/**/*.tmpl. The all: prefix keeps
// dotfiles and __name__ paths, which plain patterns drop.
//
//go:embed all:templates
var templateFS embed.FS

// manifest is the root package.json, whose version field is the release.
//
//go:embed package.json
var manifest []byte

// ErrNoVersion reports a package.json without a version string.
var ErrNoVersion = errors.New("`package.json`: no version")

// Templates returns the template tree, rooted so "base/files" names
// templates/base/files.
func Templates() fs.FS {
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		// fs.Sub fails only on an invalid directory name, and "templates" is valid.
		panic(err)
	}

	return sub
}

// Version returns the release this build is, from package.json.
func Version() (string, error) {
	var parsed struct {
		Version string `json:"version"`
	}

	err := json.Unmarshal(manifest, &parsed)
	if err != nil {
		return "", fmt.Errorf("`package.json`: %w", err)
	}

	if parsed.Version == "" {
		return "", ErrNoVersion
	}

	return parsed.Version, nil
}
