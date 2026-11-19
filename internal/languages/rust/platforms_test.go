package rust_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/rust"
	"github.com/xsyetopz/bedrock/internal/plan"
)

// platformTriples is the rustup target set each platform pins.
func platformTriples() map[string][]string {
	return map[string][]string{
		"android": {"aarch64-linux-android", "x86_64-linux-android"},
		"ios":     {"aarch64-apple-ios", "aarch64-apple-ios-sim"},
		"linux":   {"x86_64-unknown-linux-gnu"},
		"macos":   {"aarch64-apple-darwin", "x86_64-apple-darwin"},
		"wasi":    {"wasm32-wasip2"},
		"wasm":    {"wasm32-unknown-unknown"},
		"windows": {"x86_64-pc-windows-msvc"},
	}
}

func TestSpecPlatformsAreInTheSharedVocabularyForKnownKinds(t *testing.T) {
	t.Parallel()

	// Act
	spec := rust.New().Spec()

	// Assert
	for kind, names := range spec.Platforms {
		if !slices.Contains(spec.Kinds, kind) {
			t.Errorf("platforms for unknown kind %s", kind)
		}

		for _, name := range names {
			if !slices.Contains(language.Platforms(), name) || platformTriples()[name] == nil {
				t.Errorf("kind %s names unmapped platform %s", kind, name)
			}
		}
	}
}

func TestInitPinsTheTargetsOfEachPlatformTheKindBuildsFor(t *testing.T) {
	t.Parallel()

	spec := rust.New().Spec()

	for kind, names := range spec.Platforms {
		for _, name := range names {
			t.Run(kind+"@"+name, func(t *testing.T) {
				t.Parallel()

				// Arrange
				root := bedrocktest.Repo(t, nil)

				// Act
				result := bedrocktest.Plan(t, root, true,
					selectRust(t, config.Options{"kind": kind, "platform": name}))

				// Assert
				toolchain := bedrocktest.Written(result.Steps, "rust-toolchain.toml")
				for _, triple := range platformTriples()[name] {
					if !strings.Contains(toolchain, `"`+triple+`"`) {
						t.Errorf("toolchain lacks %s:\n%s", triple, toolchain)
					}
				}
			})
		}
	}
}

func TestInitWithoutPlatformsPinsNoTargets(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, true, selectRust(t, config.Options{"kind": "lib"}))

	// Assert
	if toolchain := bedrocktest.Written(result.Steps, "rust-toolchain.toml"); strings.Contains(toolchain, "targets") {
		t.Errorf("toolchain pins targets:\n%s", toolchain)
	}
}

func TestInitKeepsTheWasmKindOnItsTargetAndAddsRequestedPlatforms(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, true,
		selectRust(t, config.Options{"kind": "wasm,lib", "platform": "wasi"}))

	// Assert
	toolchain := bedrocktest.Written(result.Steps, "rust-toolchain.toml")
	for _, triple := range []string{"wasm32-unknown-unknown", "wasm32-wasip2"} {
		if !strings.Contains(toolchain, triple) {
			t.Errorf("toolchain lacks %s:\n%s", triple, toolchain)
		}
	}
}

func TestInitRejectsAPlatformNoWantedKindBuildsFor(t *testing.T) {
	t.Parallel()

	for _, options := range []config.Options{
		{"kind": "service", "platform": "wasi"},
		{"kind": "proc-macro", "platform": "ios"},
		{"kind": "lib", "platform": "tvos"},
	} {
		t.Run(options["kind"]+"@"+options["platform"], func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)

			// Act
			_, err := bedrocktest.Engine(t).Build(t.Context(), root,
				[]plan.Selected{selectRust(t, options)}, plan.Options{Version: bedrocktest.Version, Init: true})

			// Assert
			if err == nil {
				t.Error("plan succeeded")
			}
		})
	}
}

// TestInitBuildsEachKindForEachPlatform runs `cargo build --target` for each
// target a platform pins. A build that fails at the link step skips: ios needs
// Xcode's SDK, android the NDK's clang, and linux or windows a cross linker.
func TestInitBuildsEachKindForEachPlatform(t *testing.T) {
	t.Parallel()

	spec := rust.New().Spec()
	targetDir := t.TempDir()

	for kind, names := range spec.Platforms {
		for _, name := range names {
			t.Run(kind+"@"+name, func(t *testing.T) {
				t.Parallel()
				requireCargo(t)

				// Arrange
				root := bedrocktest.Repo(t, nil)
				bedrocktest.Adopt(t, root, true, selectRust(t, config.Options{"kind": kind, "platform": name}))

				for _, triple := range platformTriples()[name] {
					// Act
					ok, output := cargoSharing(t, root, targetDir, "build", "--workspace", "--target", triple)

					// Assert
					switch {
					case ok:
					case missingToolchainPart(output):
						t.Logf("skip %s: %s", triple, firstLine(output, "linker"))
					default:
						t.Errorf("cargo build --target %s failed:\n%s", triple, output)
					}
				}
			})
		}
	}
}

// missingToolchainPart reports whether a failed build lacked a linker, SDK, or
// downloadable target here, not whether the crate is wrong.
func missingToolchainPart(output string) bool {
	for _, marker := range []string{
		"linker `", "linking with", "could not find", "is not installed",
		"error: toolchain", "unable to update", "failed to download", "error sending request",
	} {
		if strings.Contains(output, marker) {
			return true
		}
	}

	return false
}

func firstLine(output, part string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, part) {
			return strings.TrimSpace(line)
		}
	}

	return "unavailable"
}
