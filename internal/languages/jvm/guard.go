package jvm

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// foreignTools are build files of other tools, which the Gradle Kotlin DSL
// build replaces.
func foreignTools() map[string]string {
	return map[string]string{
		filePomXML:         toolMaven,
		"mvnw":             toolMaven,
		"mvnw.cmd":         toolMaven,
		"build.xml":        "Ant",
		"ivy.xml":          "Ivy",
		"build.sbt":        "sbt",
		fileBuildGradle:    "Gradle's Groovy DSL",
		fileSettingsGradle: "Gradle's Groovy DSL",
	}
}

// guardPatterns are the line shapes the guard rejects.
type guardPatterns struct {
	weakening   *regexp.Regexp
	suppression *regexp.Regexp
	oldTests    *regexp.Regexp
	proguard    *regexp.Regexp
	readable    *regexp.Regexp
}

func guardForms() guardPatterns {
	return guardPatterns{
		// Rules that turn off what bedrock.pro is for, or keep the whole program.
		weakening: regexp.MustCompile(
			`^\s*(-dontshrink|-dontoptimize|-dontobfuscate|-ignorewarnings|-keep\S*\s+class\s+\*\*\s*\{?)`),
		suppression: regexp.MustCompile(`@(file:)?Suppress(Warnings)?\(`),
		oldTests: regexp.MustCompile(
			`^import\s+(org\.junit\.(Test|Assert|Before|After|Rule|runner)|junit\.framework|org\.testng)\b`),
		proguard: regexp.MustCompile(`^gradle/bedrock[^/]*\.pro$`),
		readable: regexp.MustCompile(`\.(kt|java|pro|kts)$`),
	}
}

func pathFindings(file string) []language.Finding {
	name := path.Base(file)
	tool, foreign := foreignTools()[name]

	var found []language.Finding

	if foreign {
		found = append(found, file+": "+tool+
			" build file; the Gradle Kotlin DSL build replaces it, so `bedrock adopt` migrates it")
	}

	if strings.HasPrefix(file, ".mvn/") {
		found = append(found, file+": Maven wrapper; `./gradlew` replaces it")
	}

	if strings.HasSuffix(file, ".gradle") && !foreign {
		found = append(found, file+": Groovy build script; write it as `"+file+".kts`")
	}

	return found
}

func (g guardPatterns) proguardFindings(file, text string) []language.Finding {
	var found []language.Finding

	for index, line := range strings.Split(text, "\n") {
		if g.weakening.MatchString(line) {
			found = append(found, file+":"+strconv.Itoa(index+1)+": `"+strings.TrimSpace(line)+
				"` undoes the shrinking and optimizing; keep only the classes reflection needs")
		}
	}

	return found
}

func (g guardPatterns) sourceFindings(file, text string, junit6 bool) []language.Finding {
	lines := strings.Split(text, "\n")

	var found []language.Finding

	for index, line := range lines {
		where := file + ":" + strconv.Itoa(index+1) + ": "
		reasoned := strings.Contains(line, "//") ||
			(index > 0 && strings.HasPrefix(strings.TrimSpace(lines[index-1]), "//"))

		if g.suppression.MatchString(line) && !reasoned {
			found = append(found, where+"say why in a `//` comment on the suppression or the line above it")
		}

		if junit6 && g.oldTests.MatchString(line) {
			found = append(found, where+"JUnit 4 or TestNG; write the test with JUnit 6, `org.junit.jupiter.api`")
		}
	}

	return found
}

// androidModules returns the directories, with a trailing slash, of modules
// that apply `bedrock.android`, which tests on JUnit 4.
func androidModules(scripts map[string]string) []string {
	var found []string

	for file, text := range scripts {
		if strings.Contains(text, `id("bedrock.android")`) {
			found = append(found, strings.TrimSuffix(file, fileBuildGradleKts))
		}
	}

	return found
}

// scriptFindings reports a build script that applies no bedrock plugin. An
// included build, such as `gradle/bedrock`, has a settings file and cannot
// apply them.
func scriptFindings(file, text string, hasSettings bool) []language.Finding {
	if hasSettings || strings.Contains(text, `id("bedrock.`) {
		return nil
	}

	return []language.Finding{file +
		": applies no `bedrock.*` plugin, so it skips the toolchain, Java 8 bytecode, and lints"}
}

// isOwn reports whether a path is the project's, not the managed
// `gradle/bedrock` build's.
func (g guardPatterns) isOwn(file string) bool {
	return !strings.HasPrefix(file, "gradle/bedrock/") && !g.proguard.MatchString(file)
}

// Guard reports what the compilers and linters cannot see: foreign build
// tools, build scripts without a bedrock plugin, proguard rules that disable
// shrinking, suppressions without a reason, and JUnit 4 tests. Drift of the
// managed files is core's check.
func (JVM) Guard(ctx context.Context, c *language.Context, _ bool) ([]language.Finding, error) {
	forms := guardForms()

	files, err := fsutil.ListFiles(ctx, c.Runner, c.Root, forms.isOwn)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	texts := map[string]string{}
	scripts := map[string]string{}

	for _, file := range files {
		if !forms.readable.MatchString(file) {
			continue
		}

		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
		if readErr != nil {
			return nil, readErr
		}

		texts[file] = text

		if path.Base(file) == fileBuildGradleKts {
			scripts[file] = text
		}
	}

	android := androidModules(scripts)

	var findings []language.Finding

	for _, file := range files {
		findings = append(findings, pathFindings(file)...)
		findings = append(findings, forms.contentFindings(file, texts[file], files, android)...)
	}

	return findings, nil
}

func (g guardPatterns) contentFindings(file, text string, files, android []string) []language.Finding {
	switch {
	case path.Base(file) == fileBuildGradleKts:
		sibling := strings.TrimSuffix(file, fileBuildGradleKts) + fileSettingsGradleKts

		return scriptFindings(file, text, slices.Contains(files, sibling))
	case strings.HasSuffix(file, ".pro"):
		return g.proguardFindings(file, text)
	case strings.HasSuffix(file, ".java"), strings.HasSuffix(file, ".kt"):
		junit4 := strings.Contains(file, "/src/android") ||
			slices.ContainsFunc(android, func(prefix string) bool { return strings.HasPrefix(file, prefix) })

		return g.sourceFindings(file, text, !junit4)
	default:
		return nil
	}
}
