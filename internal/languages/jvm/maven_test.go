package jvm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/plan"
)

const parentPom = `<project><groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version>
<packaging>pom</packaging>
<modules><module>core</module><module>app</module></modules>
<properties><guava.version>33.0.0-jre</guava.version></properties>
<dependencyManagement><dependencies><dependency><groupId>com.google.guava</groupId>
<artifactId>guava</artifactId><version>${guava.version}</version></dependency></dependencies></dependencyManagement>
</project>`

const corePom = `<project><parent><groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version></parent>
<artifactId>core</artifactId>
<dependencies>
<dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId></dependency>
<dependency><groupId>org.slf4j</groupId><artifactId>slf4j-api</artifactId><version>2.0.0</version><scope>provided</scope></dependency>
</dependencies></project>`

const appPom = `<project><parent><groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version></parent>
<artifactId>app</artifactId>
<properties><maven.compiler.release>17</maven.compiler.release><start-class>org.acme.Main</start-class></properties>
<dependencies><dependency><groupId>org.acme</groupId><artifactId>core</artifactId><version>1</version></dependency></dependencies>
</project>`

func mavenReactor() map[string]string {
	return map[string]string{
		"pom.xml":                               parentPom,
		"core/pom.xml":                          corePom,
		"app/pom.xml":                           appPom,
		"mvnw":                                  "#!/bin/sh\n",
		".mvn/wrapper/maven-wrapper.properties": "distributionUrl=x\n",
	}
}

func TestMavenReactorBecomesModuleScripts(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, mavenReactor())

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	core := bedrocktest.Written(result.Steps, "core/build.gradle.kts")
	for _, want := range []string{"api(libs.guava)", "compileOnly(libs.slf4j.api)"} {
		if !strings.Contains(core, want) {
			t.Errorf("core lacks %s:\n%s", want, core)
		}
	}

	if app := bedrocktest.Written(
		result.Steps,
		"app/build.gradle.kts",
	); !strings.Contains(
		app,
		`implementation(project(":core"))`,
	) {
		t.Errorf("app:\n%s", app)
	}

	catalog := bedrocktest.Written(result.Steps, "gradle/libs.versions.toml")
	if !strings.Contains(catalog, `guava = { module = "com.google.guava:guava", version = "33.0.0-jre" }`) {
		t.Errorf("catalog:\n%s", catalog)
	}

	if gone := removed(
		result.Steps,
	); !slices.Equal(
		gone,
		[]string{".mvn", "app/pom.xml", "core/pom.xml", "mvnw", "pom.xml"},
	) {
		t.Errorf("removed: %v", gone)
	}

	if !hasNote(result.Notes, "Java 17") {
		t.Errorf("notes: %v", result.Notes)
	}

	if properties := bedrocktest.Written(
		result.Steps,
		"gradle.properties",
	); !strings.Contains(
		properties,
		"group=org.acme",
	) {
		t.Errorf("gradle.properties:\n%s", properties)
	}
}

func TestMavenModuleWithoutAPomFails(t *testing.T) {
	t.Parallel()

	// Arrange
	files := mavenReactor()
	delete(files, "core/pom.xml")
	root := bedrocktest.Repo(t, files)

	// Act
	err := planError(t, root, jvm(t, "", ""))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "core/pom.xml") {
		t.Errorf("error: %v", err)
	}
}

func TestMavenSinglePomBecomesARootScript(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"pom.xml": mavenSingle})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	assertRootScript(t, result)

	if !slices.Contains(removed(result.Steps), "pom.xml") {
		t.Errorf("removed: %v", removed(result.Steps))
	}

	if properties := bedrocktest.Written(
		result.Steps,
		"gradle.properties",
	); !strings.Contains(
		properties,
		"group=org.acme",
	) {
		t.Errorf("gradle.properties:\n%s", properties)
	}
}

func TestMavenPomWithADoctypeFails(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"pom.xml": "<!DOCTYPE project []><project/>"})

	// Act
	err := planError(t, root, jvm(t, "", ""))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "DOCTYPE") {
		t.Errorf("error: %v", err)
	}
}

// assertRootScript checks that a migrated single-module build wrote a root
// script using the guava alias.
func assertRootScript(t *testing.T, result plan.Plan) {
	t.Helper()

	if script := bedrocktest.Written(result.Steps, "build.gradle.kts"); !strings.Contains(script, "libs.guava") {
		t.Errorf("build.gradle.kts:\n%s", script)
	}

	if !result.ProjectFiles["build.gradle.kts"] {
		t.Errorf("project files: %v", result.ProjectFiles)
	}
}
