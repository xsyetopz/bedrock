package golang_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
)

func TestAdoptWritesRecipesWorkflowAndConfigForTheModule(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, map[string]string{"go.mod": "module github.com/acme/app\n\ngo 1.27\n"})

	result := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "go", nil))

	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{".just/go.just", ".github/workflows/go.yml"} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses %s: %v", want, paths)
		}
	}

	if slices.Contains(paths, "go.mod") {
		t.Error("plan rewrites the project's `go.mod`")
	}

	if lint := bedrocktest.Written(result.Steps, ".golangci.yml"); !strings.Contains(lint, "- github.com/acme/app") {
		t.Errorf("`.golangci.yml` does not name the module:\n%s", lint)
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, map[string]string{"go.mod": "module m\n\ngo 1.27\n"})
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "go", nil))

	again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "go", nil))

	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestAdoptKeepsAGolangciConfigTheProjectChanged(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, map[string]string{"go.mod": "module m\n\ngo 1.27\n"})
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "go", nil))
	bedrocktest.Write(t, root, ".golangci.yml", "version: \"2\"\n")

	again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "go", nil))

	if len(again.Steps) != 0 {
		t.Errorf("adopt overwrites the project's config: %v", bedrocktest.Paths(again.Steps))
	}
}

func TestInitStartsAModuleWithEachKindsStarter(t *testing.T) {
	t.Parallel()

	for kind, starter := range map[string]string{
		"command": "internal/greeting/greeting.go",
		"library": ".golangci.yml",
		"service": "internal/server/server.go",
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			root := bedrocktest.Repo(t, nil)

			result := bedrocktest.Plan(t, root, true, bedrocktest.Select(t, "go", config.Options{"kind": kind}))

			paths := bedrocktest.Paths(result.Steps)
			if !slices.Contains(paths, "go.mod") || !slices.Contains(paths, starter) {
				t.Errorf("init %s misses `go.mod` or %s: %v", kind, starter, paths)
			}
		})
	}
}

func TestInitTakesTheModulePathFromTheRecordedOptions(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, nil)

	result := bedrocktest.Plan(t, root, true,
		bedrocktest.Select(t, "go", config.Options{"kind": "library", "module": "github.com/acme/lib"}))

	if mod := bedrocktest.Written(result.Steps, "go.mod"); !strings.Contains(mod, "module github.com/acme/lib") {
		t.Errorf("go.mod:\n%s", mod)
	}

	if len(result.Notes) != 0 {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestInitWithoutAModulePathNotesThePlaceholder(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, nil)

	result := bedrocktest.Plan(t, root, true, bedrocktest.Select(t, "go", config.Options{"kind": "library"}))

	if !slices.ContainsFunc(
		result.Notes,
		func(note string) bool { return strings.Contains(note, "placeholder module") },
	) {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestInitRejectsAKindListWithATrailingComma(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, nil)
	engine := bedrocktest.Engine(t)

	_, err := engine.Build(t.Context(), root,
		[]plan.Selected{bedrocktest.Select(t, "go", config.Options{"kind": "library,"})},
		plan.Options{Version: bedrocktest.Version, Dropped: nil, Init: true})
	if !errors.Is(err, config.ErrEmptyEntry) {
		t.Errorf("err = %v, want %v", err, config.ErrEmptyEntry)
	}
}

func TestGuardReportsDependencyManagersAndASecondGolangciConfig(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, map[string]string{
		"go.mod":         "module m\n",
		"Gopkg.toml":     "",
		"glide.yaml":     "",
		".golangci.yaml": "",
	})
	guarder := bedrocktest.Language(t, "go").(language.Guarder)

	found, err := guarder.Guard(t.Context(), bedrocktest.Context(t, root, bedrocktest.Select(t, "go", nil)), false)
	if err != nil {
		t.Fatal(err)
	}

	var paths []string
	for _, finding := range found {
		path, _, _ := strings.Cut(finding, ":")
		paths = append(paths, path)
	}

	if want := []string{".golangci.yaml", "Gopkg.toml", "glide.yaml"}; !slices.Equal(paths, want) {
		t.Errorf("findings = %v, want %v", paths, want)
	}
}

func TestGuardPassesAModuleWithItsConfig(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, map[string]string{"go.mod": "module m\n", ".golangci.yml": ""})
	guarder := bedrocktest.Language(t, "go").(language.Guarder)

	found, err := guarder.Guard(t.Context(), bedrocktest.Context(t, root, bedrocktest.Select(t, "go", nil)), false)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 0 {
		t.Errorf("findings: %v", found)
	}
}

func TestInitAddsTheWebFilesOnlyForTheWasmPlatform(t *testing.T) {
	t.Parallel()

	for platform, want := range map[string]bool{"wasm": true, "wasi": false, "linux": false, "windows": false} {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			root := bedrocktest.Repo(t, nil)

			result := bedrocktest.Plan(t, root, true,
				bedrocktest.Select(t, "go", config.Options{"kind": "command", "platform": platform}))

			paths := bedrocktest.Paths(result.Steps)
			if got := slices.Contains(paths, "web/index.html"); got != want {
				t.Errorf("web/index.html planned = %v, want %v: %v", got, want, paths)
			}

			if !slices.Contains(paths, "cmd/"+filepath.Base(root)+"/main.go") {
				t.Errorf("plan misses the command: %v", paths)
			}
		})
	}
}

func TestInitPinsGomobileForALibraryBuiltForIOSOnly(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		options config.Options
		want    bool
	}{
		{"library ios", config.Options{"kind": "library", "platform": "ios"}, true},
		{"library linux", config.Options{"kind": "library", "platform": "linux"}, false},
		{"library", config.Options{"kind": "library"}, false},
		{"command linux", config.Options{"kind": "command", "platform": "linux"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := bedrocktest.Repo(t, nil)

			result := bedrocktest.Plan(t, root, true, bedrocktest.Select(t, "go", test.options))

			mod := bedrocktest.Written(result.Steps, "go.mod")
			if got := strings.Contains(mod, "golang.org/x/mobile/cmd/gomobile"); got != test.want {
				t.Errorf("go.mod pins gomobile = %v, want %v:\n%s", got, test.want, mod)
			}
		})
	}
}

func TestInitWritesTheBuildRecipeAndIgnoresDist(t *testing.T) {
	t.Parallel()

	root := bedrocktest.Repo(t, nil)

	result := bedrocktest.Plan(t, root, true,
		bedrocktest.Select(t, "go", config.Options{"kind": "command", "platform": "wasi,linux"}))

	if recipe := bedrocktest.Written(
		result.Steps,
		".just/go.just",
	); !strings.Contains(
		recipe,
		"\nbuild platform arch=",
	) {
		t.Errorf("`.just/go.just` has no build recipe:\n%s", recipe)
	}
}

func TestInitRejectsAPlatformTheKindLacks(t *testing.T) {
	t.Parallel()

	spec := bedrocktest.Language(t, "go").Spec()

	for kind, platform := range map[string]string{"service": "wasm", "command": "ios", "library": "android"} {
		if slices.Contains(spec.KindPlatforms(kind), platform) {
			t.Errorf("%s offers %s", kind, platform)
		}
	}

	for _, platform := range spec.KindPlatforms("command") {
		if !slices.Contains(language.Platforms(), platform) {
			t.Errorf("unknown platform %s", platform)
		}
	}
}
