package language

import (
	"fmt"
	"slices"
	"strings"
)

// Platforms returns every platform name a Spec may list: the vocabulary
// that makes `@ios` mean the same target in each language that offers it.
// The names follow the operating system or runtime a build runs on, not a
// toolchain's own spelling (Go's `js`, .NET's `net10.0-ios`, Rust's
// triples), which each language maps from these.
func Platforms() []string {
	return []string{
		"android", "freebsd", "ios", "linux", "maccatalyst", "macos",
		"tvos", "visionos", "wasi", "wasm", "watchos", "windows",
	}
}

// KindPlatforms returns the platforms kind builds for, nil when it takes none.
func (s Spec) KindPlatforms(kind string) []string { return s.Platforms[kind] }

// checkPlatforms rejects a Spec whose Platforms name a kind it lacks or a
// platform outside the shared vocabulary.
func checkPlatforms(spec Spec) error {
	for kind, names := range spec.Platforms {
		if !slices.Contains(spec.Kinds, kind) {
			return fmt.Errorf(
				"%w: %s lists platforms for kind %q, which it does not have",
				ErrInvalidSpec,
				spec.ID,
				kind,
			)
		}

		for _, name := range names {
			if !slices.Contains(Platforms(), name) {
				return fmt.Errorf("%w: %s kind %q names platform %q; known: %s",
					ErrInvalidSpec, spec.ID, kind, name, strings.Join(Platforms(), ", "))
			}
		}
	}

	return nil
}
