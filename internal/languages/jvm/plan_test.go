package jvm_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const oldSettings = `// >>> jvm-bedrock
pluginManagement { includeBuild("gradle/bedrock") }
plugins { id("bedrock.settings") }
// <<< jvm-bedrock

rootProject.name = "old"
include(":lib")
`

const mavenSingle = `<project><groupId>org.acme</groupId><artifactId>x</artifactId><version>1</version>` +
	`<dependencies><dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId>` +
	`<version>33.0.0-jre</version></dependency></dependencies></project>`

func TestAdoptRemovesWhatOlderReleasesWrote(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"settings.gradle.kts":                                  oldSettings,
		"lib/build.gradle.kts":                                 "plugins { id(\"bedrock.java\") }\n",
		"gradle/bedrock/managed.sha256":                        "abc  gradlew\n",
		"gradle/bedrock/src/main/kotlin/bedrock/core/Guard.kt": "package bedrock.core\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, jvm(t, "", ""))

	// Assert
	for _, gone := range []string{
		"gradle/bedrock/managed.sha256",
		"gradle/bedrock/src/main/kotlin/bedrock/core/Guard.kt",
	} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("%s survives the adopt", gone)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "gradle/bedrock/build.gradle.kts")); err != nil {
		t.Errorf("the managed build is missing: %v", err)
	}

	settings := bedrocktest.Read(t, root, "settings.gradle.kts")
	if !strings.Contains(settings, `rootProject.name = "old"`) {
		t.Errorf("settings lose the root name:\n%s", settings)
	}
}

func TestAdoptNotesAMissingGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"settings.gradle.kts": ""})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	if !slices.ContainsFunc(result.Notes, func(note string) bool {
		return strings.HasPrefix(note, "`gradle.properties`: ")
	}) {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestAdoptNotesScalaSourcesAndStillAdopts(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"src/main/scala/A.scala": "object A\n"})

	// Act
	result := bedrocktest.Adopt(t, root, false, jvm(t, "", ""))

	// Assert
	if !hasNote(result.Notes, "not Scala or Groovy") {
		t.Errorf("notes: %v", result.Notes)
	}

	if _, err := os.Stat(filepath.Join(root, "gradlew")); err != nil {
		t.Errorf("gradlew: %v", err)
	}
}

func TestAdoptNotesAMillBuild(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"build.mill": ""})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	if !hasNote(result.Notes, "adopt cannot read Mill builds") {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestInitWritesEachKindsModuleIncludesItAndPassesTheGuard(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	bedrocktest.Adopt(t, root, true, jvm(t, "java-library,kotlin-native", ""))
	bedrocktest.CommitAll(t, root)

	// Assert
	settings := bedrocktest.Read(t, root, "settings.gradle.kts")
	if !strings.Contains(settings, `include(":lib", ":app")`) {
		t.Errorf("settings:\n%s", settings)
	}

	for _, script := range []string{"lib/build.gradle.kts", "app/build.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(root, script)); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}

	if found := guard(t, root); len(found) != 0 {
		t.Errorf("guard: %v", found)
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)
	bedrocktest.Adopt(t, root, true, jvm(t, "kotlin-library", ""))
	bedrocktest.Write(t, root, "lib/src/main/kotlin/Own.kt", "class Own\n")

	// Act
	again := bedrocktest.Plan(t, root, true, jvm(t, "kotlin-library", ""))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestSecondAdoptOfAMigratedMavenBuildPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"pom.xml": mavenSingle})
	bedrocktest.Adopt(t, root, false, jvm(t, "", ""))

	// Act
	again := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestGroupOptionReplacesTheDeclaredGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"settings.gradle.kts": "", "gradle.properties": "group=org.old\n"})

	// Act
	result := bedrocktest.Adopt(t, root, false, jvm(t, "", "org.chosen"))

	// Assert
	properties := bedrocktest.Read(t, root, "gradle.properties")
	if !strings.Contains(properties, "group=org.chosen\n") || strings.Contains(properties, "org.old") {
		t.Errorf("gradle.properties:\n%s", properties)
	}

	if !hasNote(result.Notes, "`group` org.old becomes org.chosen") {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestInitDerivesAGroupWhenNoneIsChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, true, jvm(t, "java-library", ""))

	// Assert
	if properties := bedrocktest.Written(
		result.Steps,
		"gradle.properties",
	); !strings.Contains(
		properties,
		"group=com.example.",
	) {
		t.Errorf("gradle.properties:\n%s", properties)
	}
}
