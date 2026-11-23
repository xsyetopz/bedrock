package jvm_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/step"
)

const conventions = "gradle/bedrock/src/main/kotlin/"

// written returns the paths the steps write.
func written(steps []step.Step) []string {
	var paths []string

	for _, each := range steps {
		if write, ok := each.(step.Write); ok {
			paths = append(paths, write.Path)
		}
	}

	return paths
}

func TestAdoptWritesOnlyTheConventionPluginsTheBuildApplies(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"settings.gradle.kts":                            "rootProject.name = \"ide\"\n",
		"packages/jetbrains-plugin/build.gradle.kts":     "plugins { id(\"bedrock.kotlin\") }\n",
		"packages/vscode-extension/package.json":         "{}\n",
		"packages/jetbrains-plugin/src/main/kotlin/A.kt": "package a\n",
	})

	// Act
	paths := written(bedrocktest.Plan(t, root, false, jvm(t, "", "")).Steps)

	// Assert
	for _, kept := range []string{
		conventions + "bedrock.settings.settings.gradle.kts",
		conventions + "bedrock.kotlin.gradle.kts",
		conventions + "bedrock.jvm.gradle.kts",
		conventions + "bedrock/Catalog.kt",
		"gradle/bedrock.versions.toml",
	} {
		if !slices.Contains(paths, kept) {
			t.Errorf("the plan leaves out %s", kept)
		}
	}

	for _, left := range []string{
		conventions + "bedrock.android.gradle.kts",
		conventions + "bedrock.kmp.gradle.kts",
		conventions + "bedrock.scala.gradle.kts",
		conventions + "bedrock.native-image.gradle.kts",
		"gradle/bedrock.pro",
		"gradle/bedrock-android.pro",
		"gradle/bedrock.scalafmt.conf",
	} {
		if slices.Contains(paths, left) {
			t.Errorf("the plan writes %s, which no build applies", left)
		}
	}
}

func TestAdoptRemovesUnusedConventionPluginsAndSettles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"settings.gradle.kts":                      "rootProject.name = \"x\"\n",
		"app/build.gradle.kts":                     "plugins { id(\"bedrock.kotlin\") }\n",
		conventions + "bedrock.scala.gradle.kts":   "// Managed by bedrock.\n",
		conventions + "bedrock.android.gradle.kts": "// Managed by bedrock.\n",
		"gradle/bedrock-android.pro":               "# Managed by bedrock.\n",
	})

	// Act
	first := bedrocktest.Adopt(t, root, false, jvm(t, "", ""))
	second := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	want := []string{
		"gradle/bedrock-android.pro",
		conventions + "bedrock.android.gradle.kts",
		conventions + "bedrock.scala.gradle.kts",
	}
	if got := removed(first.Steps); !slices.Equal(got, want) {
		t.Errorf("removed %v, want %v", got, want)
	}

	if len(second.Steps) > 0 {
		t.Errorf("a second adopt plans %d steps: %v", len(second.Steps), second.Steps)
	}
}

func TestInitWritesThePluginsAnAppliedPluginApplies(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{})

	// Act
	bedrocktest.Adopt(t, root, true, jvm(t, "cmp", ""))

	// Assert: cmp applies kmp, and kmp applies kmp-android by name.
	for _, plugin := range []string{"bedrock.cmp", "bedrock.kmp", "bedrock.kmp-android"} {
		if _, err := os.Stat(filepath.Join(root, conventions+plugin+".gradle.kts")); err != nil {
			t.Errorf("%s is missing: %v", plugin, err)
		}
	}

	if _, err := os.Stat(filepath.Join(root, conventions+"bedrock.scala.gradle.kts")); err == nil {
		t.Error("bedrock.scala is written, though no module applies it")
	}
}
