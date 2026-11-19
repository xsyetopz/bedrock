package jvm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const bedrockScript = "plugins { id(\"bedrock.java\") }\n"

// locations returns the path, or path and line, each finding starts with.
func locations(found []string, parts int) []string {
	var heads []string

	for _, finding := range found {
		heads = append(heads, strings.Join(strings.SplitN(finding, ":", parts+1)[:parts], ":"))
	}

	slices.Sort(heads)

	return heads
}

func TestGuardAcceptsACleanProject(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"settings.gradle.kts":      "rootProject.name = \"x\"\n",
		"lib/build.gradle.kts":     bedrockScript,
		"lib/src/main/java/A.java": "class A {}\n",
	})

	// Act
	found := guard(t, root)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings: %v", found)
	}
}

func TestGuardRejectsForeignBuildFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"pom.xml":                   "<project/>",
		"mvnw":                      "#!/bin/sh\n",
		".mvn/wrapper/x.properties": "a=b\n",
		"build.sbt":                 "\n",
		"app/build.gradle":          "\n",
	})

	// Act
	found := guard(t, root)

	// Assert
	want := []string{".mvn/wrapper/x.properties", "app/build.gradle", "build.sbt", "mvnw", "pom.xml"}
	if heads := locations(found, 1); !slices.Equal(heads, want) {
		t.Errorf("findings: %v", found)
	}
}

func TestGuardRejectsAScriptWithoutABedrockPlugin(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"lib/build.gradle.kts": "plugins { java }\n"})

	// Act
	found := guard(t, root)

	// Assert
	if len(found) != 1 || !strings.HasPrefix(found[0], "lib/build.gradle.kts: ") {
		t.Errorf("findings: %v", found)
	}
}

func TestGuardRejectsWeakProguardRulesExceptTheManagedFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"app/proguard-rules.pro": "-keep class a.B\n-dontshrink\n-dontoptimize\n",
		"gradle/bedrock.pro":     "-dontshrink\n",
	})

	// Act
	found := guard(t, root)

	// Assert
	want := []string{"app/proguard-rules.pro:2", "app/proguard-rules.pro:3"}
	if heads := locations(found, 2); !slices.Equal(heads, want) {
		t.Errorf("findings: %v", found)
	}
}

func TestGuardRejectsSuppressionsWithoutAReason(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/A.java": "@SuppressWarnings(\"x\") // why\nclass A {}\n",
		"src/B.java": "@SuppressWarnings(\"x\")\nclass B {}\n",
		"src/C.kt":   "@Suppress(\"x\")\nclass C\n",
	})

	// Act
	found := guard(t, root)

	// Assert
	if heads := locations(found, 2); !slices.Equal(heads, []string{"src/B.java:1", "src/C.kt:1"}) {
		t.Errorf("findings: %v", found)
	}
}

func TestGuardRejectsJUnit4ExceptInAndroidModules(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"droid/build.gradle.kts": "plugins { id(\"bedrock.android\") }\n",
		"droid/src/test/A.java":  "import org.junit.Test;\n",
		"lib/build.gradle.kts":   bedrockScript,
		"lib/src/test/A.java":    "import org.junit.Test;\n",
		"lib/src/test/B.java":    "import org.junit.jupiter.api.Test;\n",
	})

	// Act
	found := guard(t, root)

	// Assert
	if heads := locations(found, 2); !slices.Equal(heads, []string{"lib/src/test/A.java:1"}) {
		t.Errorf("findings: %v", found)
	}
}

func TestGuardSkipsOpenSpecAndTheManagedBuild(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"openspec/pom.xml":                "<project/>",
		"gradle/bedrock/src/B.java":       "@SuppressWarnings(\"x\")\nclass B {}\n",
		"gradle/bedrock/build.gradle.kts": "plugins { java }\n",
	})

	// Act
	found := guard(t, root)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings: %v", found)
	}
}
