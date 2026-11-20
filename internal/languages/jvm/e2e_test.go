package jvm_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/plan"
)

// gradleTimeout bounds one real Gradle run, which downloads Gradle, a JDK,
// and every plugin on a cold cache.
const gradleTimeout = 20 * time.Minute

// buildCase is one kind built for the platforms in its init option; empty
// platforms build the kind's defaults.
type buildCase struct {
	kind, platforms string
}

func (c buildCase) name() string {
	if c.platforms == "" {
		return c.kind
	}

	// Commas would land in the temp directory name, and the Kotlin native compiler
	// splits its -Xfragment-sources argument on them.
	return c.kind + "@" + strings.ReplaceAll(c.platforms, ",", "+")
}

func (c buildCase) selected(t *testing.T) plan.Selected {
	t.Helper()

	if c.platforms == "" {
		return jvm(t, c.kind, "")
	}

	return jvmOn(t, c.kind, c.platforms)
}

func kinds() []string {
	return []string{
		"android", "cmp", "java-application", "java-gradle-plugin", "java-library", "java-native",
		"kmp", "kotlin-application", "kotlin-gradle-plugin", "kotlin-library", "kotlin-native",
		"scala-application", "scala-library",
	}
}

// cases lists each kind with its defaults, then the platform choices that
// reach build files the defaults do not.
func cases() []buildCase {
	all := make([]buildCase, 0, len(kinds())+5)
	for _, kind := range kinds() {
		all = append(all, buildCase{kind: kind, platforms: ""})
	}

	return append(all,
		buildCase{kind: "kmp", platforms: "ios,macos,linux,windows"},
		buildCase{kind: "kmp", platforms: "wasm,wasi"},
		buildCase{kind: "kmp", platforms: "android,ios,macos,linux,windows,wasm,wasi"},
		buildCase{kind: "cmp", platforms: "ios,macos,linux,windows,wasm"},
		buildCase{kind: "kotlin-native", platforms: "macos,linux,windows"},
	)
}

// needsAndroid reports whether a case builds against the Android SDK: the
// android kind, and kmp and cmp unless their platforms leave android out.
func needsAndroid(c buildCase) bool {
	switch c.kind {
	case "android":
		return true
	case "cmp", "kmp":
		return c.platforms == "" || strings.Contains(c.platforms, "android")
	default:
		return false
	}
}

func androidSDK() bool {
	return os.Getenv("ANDROID_HOME") != "" || os.Getenv("ANDROID_SDK_ROOT") != ""
}

func TestInitedKindsPassTheGuard(t *testing.T) {
	t.Parallel()

	for _, c := range cases() {
		t.Run(c.name(), func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			bedrocktest.Adopt(t, root, true, c.selected(t))
			bedrocktest.CommitAll(t, root)

			// Act
			found := guard(t, root)

			// Assert
			if len(found) != 0 {
				t.Errorf("guard: %v", found)
			}
		})
	}
}

// javaMajor returns the major version of the `java` on PATH, or 0 when it
// does not report one.
func javaMajor(t *testing.T) int {
	t.Helper()

	// `java -version` writes to stderr, which CombinedOutput includes.
	output, err := exec.CommandContext(t.Context(), "java", "-version").CombinedOutput()
	if err != nil {
		return 0
	}

	match := regexp.MustCompile(`version "(?:1\.)?(\d+)`).FindSubmatch(output)
	if match == nil {
		return 0
	}

	major, _ := strconv.Atoi(string(match[1]))

	return major
}

// gradleJDK is the oldest JDK that starts Gradle 9. The daemon itself runs on
// the JDK `gradle/gradle-daemon-jvm.properties` pins, which Gradle finds or
// downloads whatever JDK starts `gradlew`.
const gradleJDK = 17

// TestInitedKindsBuildWithGradle runs the kinds one after another, never in
// parallel: concurrent builds contend for the shared Gradle cache and daemons.
// Run it with its own budget, such as `-timeout 30m`.
//
//nolint:paralleltest // Gradle builds share one cache and must run in sequence.
func TestInitedKindsBuildWithGradle(t *testing.T) {
	if testing.Short() {
		t.Skip("a Gradle build is slow")
	}

	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("no JDK on PATH")
	}

	if have := javaMajor(t); have < gradleJDK {
		t.Skipf("Gradle needs JDK %d or newer to start, and the `java` on PATH is %d", gradleJDK, have)
	}

	for _, c := range cases() {
		t.Run(c.name(), func(t *testing.T) {
			if needsAndroid(c) && !androidSDK() {
				t.Skip("no Android SDK: set ANDROID_HOME or ANDROID_SDK_ROOT")
			}

			// Arrange
			root := bedrocktest.Repo(t, nil)
			bedrocktest.Adopt(t, root, true, c.selected(t))
			bedrocktest.CommitAll(t, root)

			ctx, cancel := context.WithTimeout(t.Context(), gradleTimeout)
			defer cancel()

			// Act
			command := exec.CommandContext(ctx, filepath.Join(root, "gradlew"), "--console=plain",
				"spotlessCheck", "bedrockCompile", "bedrockLint", "bedrockTest")
			command.Dir = root

			var output bytes.Buffer

			command.Stdout = &output
			command.Stderr = &output

			err := command.Run()

			// Assert
			if err != nil || !strings.Contains(output.String(), "BUILD SUCCESSFUL") {
				t.Errorf("gradle failed: %v\n%s", err, output.String())
			}
		})
	}
}
