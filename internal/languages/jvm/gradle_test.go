package jvm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func groovyBuild() map[string]string {
	return map[string]string{
		"settings.gradle": "include ':app', ':core'\n",
		"build.gradle":    "group = 'org.acme'\nallprojects {\n  repositories { mavenCentral() }\n}\n",
		"app/build.gradle": `plugins {
  id 'java'
  id 'application'
}
application { mainClass = 'org.acme.Main' }
dependencies {
  implementation 'com.google.guava:guava:33.0.0-jre'
  testImplementation 'junit:junit:4.13'
  implementation project(':core')
}
`,
		"core/build.gradle": "plugins { id 'java-library' }\n",
	}
}

func TestGradleGroovyBuildBecomesKotlinScripts(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, groovyBuild())

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	want := []string{"app/build.gradle", "build.gradle", "core/build.gradle", "settings.gradle"}
	if gone := removed(result.Steps); !slices.Equal(gone, want) {
		t.Errorf("removed: %v", gone)
	}

	app := bedrocktest.Written(result.Steps, "app/build.gradle.kts")
	for _, text := range []string{"libs.guava", `project(":core")`} {
		if !strings.Contains(app, text) {
			t.Errorf("app lacks %s:\n%s", text, app)
		}
	}

	if script := bedrocktest.Written(result.Steps, "build.gradle.kts"); script != "" {
		t.Errorf("a root script is written:\n%s", script)
	}

	if len(result.Notes) == 0 {
		t.Error("no notes")
	}
}

func TestGradleBedrockScriptsAreLeftAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"settings.gradle.kts":  "include(\":lib\")\n",
		"lib/build.gradle.kts": "plugins { id(\"bedrock.java\") }\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	if script := bedrocktest.Written(result.Steps, "lib/build.gradle.kts"); script != "" {
		t.Errorf("the script is rewritten:\n%s", script)
	}
}

func TestGradleAndroidScriptsGetANoteAndNoModules(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"settings.gradle":  "include ':app'\n",
		"app/build.gradle": "plugins { id 'com.android.application' }\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	if script := bedrocktest.Written(result.Steps, "app/build.gradle.kts"); script != "" {
		t.Errorf("an Android module is converted:\n%s", script)
	}

	if len(result.Notes) == 0 {
		t.Error("no notes")
	}
}
