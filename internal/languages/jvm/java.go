package jvm

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// jdkMissing is what to tell a person when no JDK is found.
const jdkMissing = "Gradle needs a JDK 17 or newer to start, and none runs here. " +
	"Install one, such as `brew install openjdk@25` or your system's openjdk package; " +
	"bedrock finds Homebrew's and `/usr/lib/jvm`'s JDKs itself, or set `JAVA_HOME`."

// jdk is a JDK that can start Gradle. Home is what to export as JAVA_HOME,
// and is empty when the `java` on PATH works.
type jdk struct {
	home string
}

// env returns the environment a Gradle run needs, or nil for the inherited one.
func (j jdk) env() []string {
	if j.home == "" {
		return nil
	}

	return []string{"JAVA_HOME=" + j.home}
}

// brewFormulae are Homebrew's JDK formulae, newest supported first.
func brewFormulae() []string { return []string{"openjdk@25", "openjdk@21", "openjdk@17", "openjdk"} }

func brewPrefixes() []string { return []string{"/opt/homebrew", "/usr/local"} }

// output runs a command and returns its trimmed stdout, false when it fails
// or is missing.
func output(ctx context.Context, runner execx.Runner, name string, args ...string) (string, bool) {
	result, err := runner.Run(ctx, execx.Command{Stdin: nil, Dir: "", Name: name, Args: args, Env: nil, Stream: false})
	if err != nil || result.Code != 0 {
		return "", false
	}

	return strings.TrimSpace(string(result.Stdout)), true
}

// listed returns the directories under parent whose names start with prefix,
// highest name first, with digit runs compared as numbers.
func listed(parent, prefix string) []string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}

	var names []string

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			names = append(names, entry.Name())
		}
	}

	slices.SortFunc(names, func(left, right string) int { return naturalCompare(right, left) })

	paths := make([]string, len(names))
	for index, name := range names {
		paths[index] = filepath.Join(parent, name)
	}

	return paths
}

// naturalCompare orders strings with digit runs as numbers, so openjdk@21
// sorts above openjdk@9.
func naturalCompare(left, right string) int {
	for left != "" && right != "" {
		leftEnd, rightEnd := runEnd(left), runEnd(right)
		leftRun, rightRun := left[:leftEnd], right[:rightEnd]

		if isDigit(leftRun[0]) && isDigit(rightRun[0]) {
			leftNumber := strings.TrimLeft(leftRun, "0")
			rightNumber := strings.TrimLeft(rightRun, "0")

			if len(leftNumber) != len(rightNumber) {
				return len(leftNumber) - len(rightNumber)
			}

			leftRun, rightRun = leftNumber, rightNumber
		}

		if order := strings.Compare(leftRun, rightRun); order != 0 {
			return order
		}

		left, right = left[leftEnd:], right[rightEnd:]
	}

	return len(left) - len(right)
}

func isDigit(char byte) bool { return char >= '0' && char <= '9' }

// runEnd returns where the leading run of digits or non-digits in text ends.
func runEnd(text string) int {
	digits := isDigit(text[0])
	end := 1

	for end < len(text) && isDigit(text[end]) == digits {
		end++
	}

	return end
}

// candidates returns the places a JDK may live, each as a macOS bundle home
// first and the directory itself second.
func candidates() []string {
	var roots []string

	for _, prefix := range brewPrefixes() {
		for _, formula := range brewFormulae() {
			roots = append(roots, filepath.Join(prefix, "opt", formula))
		}
	}

	for _, prefix := range brewPrefixes() {
		roots = append(roots, listed(filepath.Join(prefix, "opt"), "openjdk@")...)
	}

	roots = append(roots, listed("/usr/lib/jvm", "")...)

	found := make([]string, 0, len(roots)*homesPerRoot)
	for _, root := range roots {
		found = append(found, filepath.Join(root, "libexec", "openjdk.jdk", "Contents", "Home"), root)
	}

	return found
}

// findJdk finds a JDK the way the `gradle` recipe in `.just/jvm.just` does: a
// valid JAVA_HOME, a `java` that runs (macOS's `/usr/bin/java` stub fails),
// macOS's `java_home`, then Homebrew and `/usr/lib/jvm`. It returns false when
// none runs.
func findJdk(ctx context.Context, runner execx.Runner, configured string) (jdk, bool) {
	if configured != "" && fsutil.Exists(filepath.Join(configured, "bin", "java")) {
		return jdk{home: configured}, true
	}

	if _, ok := output(ctx, runner, "java", "-version"); ok {
		return jdk{home: ""}, true
	}

	if reported, ok := output(ctx, runner, "/usr/libexec/java_home"); ok && reported != "" {
		return jdk{home: reported}, true
	}

	for _, home := range candidates() {
		if fsutil.Exists(filepath.Join(home, "bin", "java")) {
			return jdk{home: home}, true
		}
	}

	return jdk{home: ""}, false
}
