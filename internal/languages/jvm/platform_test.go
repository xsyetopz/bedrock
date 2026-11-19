package jvm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/step"
)

// jvmOn selects the JVM language with a kind built for platforms.
func jvmOn(t *testing.T, kind, platforms string) plan.Selected {
	t.Helper()

	return bedrocktest.Select(t, "jvm", config.Options{"kind": kind, "platform": platforms})
}

// writtenPaths returns the sorted paths the steps write.
func writtenPaths(steps []step.Step) []string {
	var paths []string

	for _, each := range steps {
		if write, ok := each.(step.Write); ok {
			paths = append(paths, write.Path)
		}
	}

	slices.Sort(paths)

	return paths
}

func TestPlatformsChooseTheTargetsInTheModuleProperties(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ kind, module, platforms string }{
		{"kmp", "shared", "android,ios,linux,macos,wasi,wasm,windows"},
		{"kmp", "shared", "linux,wasm"},
		{"cmp", "shared", "android,ios,macos,linux,windows,wasm"},
		{"kotlin-native", "app", "linux,windows"},
	} {
		t.Run(test.kind+"@"+test.platforms, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)

			// Act
			result := bedrocktest.Plan(t, root, true, jvmOn(t, test.kind, test.platforms))

			// Assert
			got := bedrocktest.Written(result.Steps, test.module+"/gradle.properties")
			if !strings.HasSuffix(got, "\nbedrock.platforms="+test.platforms+"\n") {
				t.Errorf("%s/gradle.properties:\n%s", test.module, got)
			}
		})
	}
}

func TestWithoutPlatformsAKindWritesNoPropertiesAndKeepsItsModules(t *testing.T) {
	t.Parallel()

	for kind, modules := range map[string][]string{
		"kmp":           {"shared"},
		"cmp":           {"androidApp", "shared"},
		"kotlin-native": {"app"},
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)

			// Act
			result := bedrocktest.Plan(t, root, true, jvm(t, kind, ""))

			// Assert
			paths := writtenPaths(result.Steps)
			settings := bedrocktest.Written(result.Steps, "settings.gradle.kts")

			for _, module := range modules {
				if slices.Contains(paths, module+"/gradle.properties") {
					t.Errorf("%s writes %s/gradle.properties without platforms", kind, module)
				}

				if !strings.Contains(settings, `":`+module+`"`) {
					t.Errorf("%s: settings lack %s:\n%s", kind, module, settings)
				}
			}
		})
	}
}

func TestCmpWithoutAndroidLeavesOutTheAndroidApp(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, true, jvmOn(t, "cmp", "ios,wasm"))

	// Assert
	for _, path := range writtenPaths(result.Steps) {
		if strings.HasPrefix(path, "androidApp/") {
			t.Errorf("writes %s without android", path)
		}
	}

	if settings := bedrocktest.Written(result.Steps, "settings.gradle.kts"); strings.Contains(settings, "androidApp") {
		t.Errorf("settings include androidApp:\n%s", settings)
	}
}

func TestCmpWithAndroidKeepsTheAndroidApp(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, true, jvmOn(t, "cmp", "android,wasm"))

	// Assert
	if !slices.Contains(writtenPaths(result.Steps), "androidApp/build.gradle.kts") {
		t.Errorf("no androidApp: %v", writtenPaths(result.Steps))
	}
}

func TestPlatformPlansAreIdempotent(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)
	bedrocktest.Adopt(t, root, true, jvmOn(t, "kmp", "linux,wasm"))
	bedrocktest.CommitAll(t, root)

	// Act
	again := bedrocktest.Plan(t, root, false, jvmOn(t, "kmp", "linux,wasm"))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("a second adopt plans %d steps", len(again.Steps))
	}
}

func TestScalaAndGradlePluginKindsWriteTheirModuleAndPlanIdempotently(t *testing.T) {
	t.Parallel()

	for kind, module := range map[string]string{
		"scala-application":    "app",
		"scala-library":        "lib",
		"java-gradle-plugin":   "plugin",
		"kotlin-gradle-plugin": "plugin",
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			bedrocktest.Adopt(t, root, true, jvm(t, kind, ""))
			bedrocktest.CommitAll(t, root)

			// Act
			again := bedrocktest.Plan(t, root, false, jvm(t, kind, ""))

			// Assert
			if !strings.Contains(bedrocktest.Read(t, root, "settings.gradle.kts"), `":`+module+`"`) {
				t.Errorf("%s: settings lack %s", kind, module)
			}

			if len(again.Steps) != 0 {
				t.Errorf("%s: a second adopt plans %d steps", kind, len(again.Steps))
			}
		})
	}
}
