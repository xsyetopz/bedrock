package cpp_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// kindFiles are the paths each kind's starter writes, for a project named
// p001, the temporary directory's name.
func kindFiles() map[string][]string {
	return map[string][]string{
		"executable": {
			"CMakeLists.txt", "CMakePresets.json",
			"src/greeting.cpp", "src/greeting.hpp", "src/main.cpp", "tests/greeting_test.cpp",
		},
		"header-only": {
			"CMakeLists.txt", "CMakePresets.json",
			"include/p001/p001.hpp", "tests/p001_test.cpp",
		},
		"library": {
			"CMakeLists.txt", "CMakePresets.json",
			"include/p001/p001.hpp", "src/p001.cpp", "tests/p001_test.cpp",
		},
	}
}

func cppSpec(t *testing.T) language.Spec {
	t.Helper()

	return bedrocktest.Language(t, "cpp").Spec()
}

func initPlan(t *testing.T, kind, platform string) []step.Step {
	t.Helper()

	options := config.Options{"kind": kind}
	if platform != "" {
		options["platform"] = platform
	}

	root := bedrocktest.Repo(t, nil)

	return bedrocktest.Plan(t, root, true, bedrocktest.Select(t, "cpp", options)).Steps
}

func TestInitWritesEachKindsCMakeStarter(t *testing.T) {
	t.Parallel()

	for kind, want := range kindFiles() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Act
			paths := bedrocktest.Paths(initPlan(t, kind, ""))

			// Assert
			for _, path := range want {
				if !slices.Contains(paths, path) {
					t.Errorf("plan misses `%s`: %v", path, paths)
				}
			}

			if slices.Contains(paths, "cmake/presets/macos.json") {
				t.Errorf("a plan without platforms writes a platform preset: %v", paths)
			}
		})
	}
}

func TestInitFillsEveryPlaceholder(t *testing.T) {
	t.Parallel()

	for kind := range kindFiles() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			platform := strings.Join(cppSpec(t).KindPlatforms(kind), ",")

			// Act
			steps := initPlan(t, kind, platform)

			// Assert
			for _, written := range steps {
				write, ok := written.(step.Write)
				if !ok {
					continue
				}

				if strings.Contains(string(write.Content), "@@") {
					t.Errorf("`%s` keeps a placeholder:\n%s", write.Path, write.Content)
				}
			}
		})
	}
}

// checkPresetFile checks that the platform's preset file names one configure
// and one build preset after the platform.
func checkPresetFile(t *testing.T, steps []step.Step, platform string) {
	t.Helper()

	path := "cmake/presets/" + platform + ".json"

	var preset struct {
		ConfigurePresets []struct {
			Name string `json:"name"`
		} `json:"configurePresets"`
		BuildPresets []struct {
			Name string `json:"name"`
		} `json:"buildPresets"`
	}

	if err := json.Unmarshal([]byte(bedrocktest.Written(steps, path)), &preset); err != nil {
		t.Fatalf("`%s` is not JSON: %v", path, err)
	}

	if len(preset.ConfigurePresets) != 1 || preset.ConfigurePresets[0].Name != platform ||
		len(preset.BuildPresets) != 1 || preset.BuildPresets[0].Name != platform {
		t.Errorf("`%s` names presets %+v, want one configure and one build preset %q", path, preset, platform)
	}
}

func TestInitWritesEachPlatformsPresetAndIncludesIt(t *testing.T) {
	t.Parallel()

	for kind, platforms := range cppSpec(t).Platforms {
		for _, platform := range platforms {
			t.Run(kind+"@"+platform, func(t *testing.T) {
				t.Parallel()

				// Arrange
				// Act
				steps := initPlan(t, kind, platform)

				// Assert
				path := "cmake/presets/" + platform + ".json"
				if !slices.Contains(bedrocktest.Paths(steps), path) {
					t.Fatalf("plan misses `%s`: %v", path, bedrocktest.Paths(steps))
				}

				var root struct {
					Include []string `json:"include"`
				}

				if err := json.Unmarshal([]byte(bedrocktest.Written(steps, "CMakePresets.json")), &root); err != nil {
					t.Fatalf("CMakePresets.json is not JSON: %v", err)
				}

				if !slices.Equal(root.Include, []string{path}) {
					t.Errorf("CMakePresets.json includes %v, want [%s]", root.Include, path)
				}

				checkPresetFile(t, steps, platform)
			})
		}
	}
}

func TestInitWithTwoPlatformsIncludesBothInOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	// Act
	steps := initPlan(t, "library", "wasm,ios")

	// Assert
	var root struct {
		Include []string `json:"include"`
	}

	if err := json.Unmarshal([]byte(bedrocktest.Written(steps, "CMakePresets.json")), &root); err != nil {
		t.Fatal(err)
	}

	if want := []string{"cmake/presets/ios.json", "cmake/presets/wasm.json"}; !slices.Equal(root.Include, want) {
		t.Errorf("includes = %v, want %v", root.Include, want)
	}
}

func TestPlatformPresetsAreTheSameInEveryKind(t *testing.T) {
	t.Parallel()

	// Arrange
	platforms := cppSpec(t).Platforms
	first := map[string]string{}

	// Act
	for _, kind := range []string{"executable", "header-only", "library"} {
		for _, platform := range platforms[kind] {
			path := "cmake/presets/" + platform + ".json"
			text := bedrocktest.Written(initPlan(t, kind, platform), path)

			// Assert
			if want, seen := first[platform]; seen && want != text {
				t.Errorf("%s's `%s` differs from another kind's", kind, path)
			}

			first[platform] = text
		}
	}
}

func TestAnExecutableTakesNoAppBundlePlatform(t *testing.T) {
	t.Parallel()

	// Arrange
	platforms := cppSpec(t).KindPlatforms("executable")

	// Act
	// Assert
	for _, bundle := range []string{"ios", "maccatalyst", "tvos", "visionos", "watchos"} {
		if slices.Contains(platforms, bundle) {
			t.Errorf("executable offers %s, which launches app bundles", bundle)
		}
	}
}

func TestSecondAdoptOfAnInitializedProjectPlansNothing(t *testing.T) {
	t.Parallel()

	for kind := range kindFiles() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			options := config.Options{"kind": kind, "platform": "linux,macos"}
			bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "cpp", options))
			bedrocktest.CommitAll(t, root)

			// Act
			again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "cpp", options))

			// Assert
			if len(again.Steps) != 0 {
				t.Errorf("a second adopt plans %v", bedrocktest.Paths(again.Steps))
			}
		})
	}
}
