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

// AllKinds names the directory under templates/<id>/platforms whose
// <platform> files an init writes for every kind, such as a CMake preset each
// kind shares; no kind may take the name.
const AllKinds = "all"

// KindPlatforms returns the platforms kind builds for, nil when it takes none.
func (s Spec) KindPlatforms(kind string) []string { return s.Platforms[kind] }

// checkPlatforms rejects a Spec whose Platforms name a kind it lacks or a
// platform outside the shared vocabulary, and a kind named AllKinds.
func checkPlatforms(spec Spec) error {
	if slices.Contains(spec.Kinds, AllKinds) {
		return fmt.Errorf("%w: %s names a kind %q, which means every kind", ErrInvalidSpec, spec.ID, AllKinds)
	}

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
