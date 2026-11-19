package jvm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const sbtBuild = `organization := "org.acme"
libraryDependencies += "com.google.guava" % "guava" % "33.0.0-jre"

lazy val core = (project in file("core"))
  .settings(libraryDependencies += "org.slf4j" % "slf4j-api" % "2.0.0" % Provided)

lazy val app = (project in file("app"))
  .dependsOn(core)
  .settings(
    Compile / mainClass := Some("org.acme.Main"),
    libraryDependencies += "org.typelevel" %% "cats-core" % "2.10.0"
  )
`

func TestSbtBuildBecomesModules(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"build.sbt": sbtBuild, "project/plugins.sbt": "\n"})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	if core := bedrocktest.Written(result.Steps, "core/build.gradle.kts"); !strings.Contains(core, "libs.guava") {
		t.Errorf("core:\n%s", core)
	}

	if app := bedrocktest.Written(result.Steps, "app/build.gradle.kts"); !strings.Contains(app, `project(":core")`) {
		t.Errorf("app:\n%s", app)
	}

	if gone := removed(result.Steps); !slices.Equal(gone, []string{"build.sbt", "project"}) {
		t.Errorf("removed: %v", gone)
	}

	if !hasNote(result.Notes, "Scala") {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestSbtSingleProjectBecomesARootScript(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"build.sbt": "organization := \"org.acme\"\nlibraryDependencies += \"com.google.guava\" % \"guava\" % \"33.0.0-jre\"\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	assertRootScript(t, result)
}

func TestAntBuildWithIvyBecomesARootScript(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"build.xml": `<project name="x" default="jar"><javac srcdir="src" destdir="build" source="11" target="11"/></project>`,
		"ivy.xml": `<ivy-module version="2.0"><info organisation="org.acme" module="x"/><dependencies>` +
			`<dependency org="com.google.guava" name="guava" rev="33.0.0-jre" conf="compile->default"/>` +
			`<dependency org="junit" name="junit" rev="4.13" conf="test->default"/></dependencies></ivy-module>`,
	})

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	assertRootScript(t, result)

	if gone := removed(result.Steps); !slices.Equal(gone, []string{"build.xml", "ivy.xml"}) {
		t.Errorf("removed: %v", gone)
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

	if !hasNote(result.Notes, "Java 11") {
		t.Errorf("notes: %v", result.Notes)
	}
}
