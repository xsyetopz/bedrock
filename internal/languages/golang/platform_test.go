package golang_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
)

func TestSecondInitWithPlatformsPlansNothing(t *testing.T) {
	t.Parallel()

	for kind, platform := range map[string]string{"command": "wasm,wasi", "library": "ios", "service": "linux"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			root := bedrocktest.Repo(t, nil)
			selected := bedrocktest.Select(t, "go", config.Options{"kind": kind, "platform": platform})
			bedrocktest.Adopt(t, root, true, selected)

			again := bedrocktest.Plan(t, root, true, selected)

			if len(again.Steps) != 0 {
				t.Errorf("second init plans %v", bedrocktest.Paths(again.Steps))
			}
		})
	}
}

func TestInitBuildsEachPlatformThroughTheBuildRecipe(t *testing.T) {
	t.Parallel()

	for kind, platforms := range map[string][]string{
		"command": {"freebsd", "linux", "macos", "wasi", "wasm", "windows"},
		"library": {"freebsd", "linux", "macos", "wasi", "wasm", "windows"},
		"service": {"freebsd", "linux", "macos", "windows"},
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			requireTools(t, "go", "just")

			root := initRepo(t, kind, platforms...)

			for _, platform := range platforms {
				if ok, output := run(t, root, "just", "go::build", platform); !ok {
					t.Errorf("`just go::build %s`:\n%s", platform, output)
				}
			}

			if kind != "library" {
				requireArtifacts(t, root, platforms)
			}
		})
	}
}

func TestInitWasmBuildCarriesTheRuntimeAndPage(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "just")

	root := initRepo(t, "command", "wasm")

	if ok, output := run(t, root, "just", "go::build", "wasm"); !ok {
		t.Fatalf("`just go::build wasm`:\n%s", output)
	}

	for _, file := range []string{"wasm_exec.js", "index.html"} {
		if _, err := os.Stat(filepath.Join(root, "dist", "wasm", file)); err != nil {
			t.Errorf("dist/wasm: %v", err)
		}
	}
}

func TestInitWasiBuildRunsUnderWasmtime(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "just", "wasmtime")

	root := initRepo(t, "command", "wasi")

	if ok, output := run(t, root, "just", "go::build", "wasi"); !ok {
		t.Fatalf("`just go::build wasi`:\n%s", output)
	}

	matches, _ := filepath.Glob(filepath.Join(root, "dist", "wasi", "*.wasm"))
	if len(matches) != 1 {
		t.Fatalf("dist/wasi: %v", matches)
	}

	if ok, output := run(t, root, "wasmtime", matches[0]); !ok || output != "Hello, world!\n" {
		t.Errorf("wasmtime output %q", output)
	}
}

func TestInitBindsALibraryForIOS(t *testing.T) {
	t.Parallel()
	requireTools(t, "go", "just", "xcrun")

	if ok, _ := run(t, t.TempDir(), "xcrun", "--sdk", "iphoneos", "--show-sdk-path"); !ok {
		t.Skip("the iOS SDK is not installed")
	}

	root := initRepo(t, "library", "ios")

	// bedrock init runs this after it writes the files; the test helper does not.
	if ok, output := run(t, root, "go", "mod", "tidy"); !ok {
		t.Fatalf("go mod tidy:\n%s", output)
	}

	if ok, output := run(t, root, "just", "go::build", "ios"); !ok {
		t.Fatalf("`just go::build ios`:\n%s", output)
	}

	if ok, output := run(t, root, "go", "mod", "tidy", "-diff"); !ok {
		t.Errorf("go.mod is not tidy:\n%s", output)
	}
}

// requireArtifacts fails for each platform whose build left no file with its extension.
func requireArtifacts(t *testing.T, root string, platforms []string) {
	t.Helper()

	for platform, artifact := range map[string]string{"wasi": ".wasm", "wasm": ".wasm", "windows": ".exe"} {
		if !slices.Contains(platforms, platform) {
			continue
		}

		matches, _ := filepath.Glob(filepath.Join(root, "dist", platform, "*"+artifact))
		if len(matches) == 0 {
			t.Errorf("dist/%s has no %s file", platform, artifact)
		}
	}
}
