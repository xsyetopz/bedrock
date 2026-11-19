// Package jvm is the JVM brick: Java and Kotlin on Gradle, with the
// `bedrock.*` convention plugins and migrations from Maven, sbt, Ant, and
// Gradle's own scripts. Its templates live under templates/jvm.
package jvm

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

// JVM is the Java and Kotlin language.
type JVM struct{}

// New returns the JVM language.
func New() JVM { return JVM{} }

func packagePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
}

// packageError says a group is not a Java package name.
type packageError struct{}

func (packageError) Error() string { return "is no Java package name, such as `com.example.app`" }

func validateGroup(value string) error {
	if !packagePattern().MatchString(value) {
		return packageError{}
	}

	return nil
}

// Spec declares the JVM's templates, markers, flag, and executables.
func (JVM) Spec() language.Spec {
	return language.Spec{
		Tools: nil,
		Flags: []language.Flag{{
			Validate: validateGroup,
			Name:     "group",
			Value:    "<pkg>",
			Usage:    "The JVM group and package, such as `com.example.app`; kept in `.editorconfig`.",
			Requires: true,
		}},
		ID:      "jvm",
		Block:   "jvm-bedrock",
		Aliases: []string{"jvm", nameJava, "kotlin"},
		Markers: []string{
			filePomXML, fileBuildGradle, fileBuildGradleKts, fileSettingsGradle, fileSettingsGradleKts,
			"build.sbt", "build.xml", "ivy.xml", "gradlew",
		},
		Sources: []string{"**/*.java", "**/*.kt", "**/*.kts", "**/*.pro", "gradle/**", fileGradleProperties},
		Kinds: []string{
			"android", "cmp", "java-application", "java-library", "java-native",
			"kmp", "kotlin-application", "kotlin-library", "kotlin-native",
		},
		Executables: []string{"**/gradlew"},
		// javac, kotlinc, Error Prone, and NullAway bound no sizes, so lizard
		// covers files and functions.
		Lizard: []string{nameJava, "kotlin", "scala"},
	}
}

// Variables fill the group and its directory form for the kind templates.
func (JVM) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	group := groupOption(c)
	if group == "" {
		group = "com.example." + packageName(filepath.Base(c.Root))
	}

	return template.Variables{"package": group, "path": regexp.MustCompile(`\.`).ReplaceAllString(group, "/")}, nil
}

// AfterApply runs `./gradlew --version` when a JDK is found, which downloads
// the Gradle distribution. A failure never fails the adopt: the recipes run
// Gradle again and report what is missing.
func (JVM) AfterApply(ctx context.Context, c *language.Context) error {
	found, ok := findJdk(ctx, c.Runner, os.Getenv("JAVA_HOME"))
	if !ok || !fsutil.IsRegularFile(filepath.Join(c.Root, "gradlew")) {
		return nil
	}

	_, _ = c.Runner.Run(ctx, execx.Command{
		Stdin: nil, Dir: c.Root, Name: "./gradlew", Args: []string{"--version"}, Env: found.env(), Stream: false,
	})

	return nil
}

// Runs names the work AfterApply does.
func (JVM) Runs() string { return "`./gradlew --version`, which fetches Gradle" }

// Subcommands returns `jvm j2k`.
func (JVM) Subcommands() []language.Subcommand {
	return []language.Subcommand{{
		Run:     j2k,
		Name:    "j2k",
		Args:    "<path>...",
		Summary: "Convert Java files to Kotlin with IntelliJ IDEA's J2K.",
	}}
}

// testSourceSet matches a Gradle test source set's directory: `src/test`,
// `src/testFixtures`, an Android variant's `src/testDebug` or
// `src/androidTest`, a Kotlin Multiplatform target's `src/commonTest`, and a
// JVM test suite's `src/integrationTest`. Gradle compiles them only for tests.
func testSourceSet() *regexp.Regexp {
	return regexp.MustCompile(`(?:^|/)src/(?:test|[a-z]\w*Test)(?:[A-Z]\w*)?/`)
}

// TestFiles returns the files in a Gradle test source set.
func (JVM) TestFiles(_ context.Context, _ *language.Context, files []string) (map[string]bool, error) {
	pattern := testSourceSet()
	found := map[string]bool{}

	for _, file := range files {
		if pattern.MatchString(file) {
			found[file] = true
		}
	}

	return found, nil
}

var (
	_ language.Planner            = JVM{}
	_ language.VariableProvider   = JVM{}
	_ language.Guarder            = JVM{}
	_ language.PostApplier        = JVM{}
	_ language.TestFileClassifier = JVM{}
	_ language.SubcommandProvider = JVM{}
)
