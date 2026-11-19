package swift_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const oldTools = `let package = Package(name: "BuildTools", dependencies: ` +
	`[.package(url: "https://github.com/xsyetopz/bedrock", revision: "swift-bedrock-v0.1.0")])` + "\n"

func TestAdoptRemovesBuildToolsThatDependOnSwiftBedrock(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"Package.swift": "let package = 1\n", "BuildTools/Package.swift": oldTools}

	// Act
	root, result := planSwift(t, files)
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "swift", nil))

	// Assert
	if !slices.Contains(bedrocktest.Paths(result.Steps), "BuildTools") {
		t.Errorf("plan keeps `BuildTools`: %v", bedrocktest.Paths(result.Steps))
	}
}

func TestAdoptKeepsABuildToolsPackageTheProjectOwns(t *testing.T) {
	t.Parallel()

	// Arrange
	own := `let package = Package(name: "BuildTools", dependencies: ` +
		`[.package(url: "https://example.com/other", from: "1.0.0")])` + "\n"
	files := map[string]string{"Package.swift": "let package = 1\n", "BuildTools/Package.swift": own}

	// Act
	_, result := planSwift(t, files)

	// Assert
	if slices.Contains(bedrocktest.Paths(result.Steps), "BuildTools") {
		t.Error("plan removes the project's own `BuildTools`")
	}
}

func TestAdoptRemovesFilesOfReplacedTools(t *testing.T) {
	t.Parallel()

	for _, file := range []string{".swiftformat", ".swiftlint.yaml", "Mintfile"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			// Act
			_, result := planSwift(t, map[string]string{"Package.swift": "let package = 1\n", file: "x\n"})

			// Assert
			if !slices.Contains(bedrocktest.Paths(result.Steps), file) {
				t.Errorf("plan keeps %s: %v", file, bedrocktest.Paths(result.Steps))
			}
		})
	}
}

func TestAdoptNotesWhereBuildToolsAreStillMentioned(t *testing.T) {
	t.Parallel()

	for file, text := range map[string]string{
		"project.just":              "lint:\n\tswift run --package-path BuildTools swiftlint\n",
		".gitignore":                "BuildTools/.build\n",
		".just/ci.just":             "x:\n\tcd BuildTools\n",
		".github/workflows/ci.yml":  "run: swift run --package-path BuildTools x\n",
		".github/dependabot.yaml":   "# BuildTools\n",
		".just/deep/not-listed.txt": "BuildTools\n",
	} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := map[string]string{
				"Package.swift":            "let package = 1\n",
				"BuildTools/Package.swift": oldTools,
				file:                       text,
			}

			// Act
			_, result := planSwift(t, files)

			// Assert
			want := "`" + file + "`: mentions `BuildTools`"
			noted := slices.ContainsFunc(result.Notes, func(n string) bool { return strings.HasPrefix(n, want) })

			if listed := !strings.Contains(file, "not-listed"); noted != listed {
				t.Errorf("noted = %v, want %v: %v", noted, listed, result.Notes)
			}
		})
	}
}

func TestAdoptSkipsManagedSwiftFilesAndBlocks(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"Package.swift":               "let package = 1\n",
		"BuildTools/Package.swift":    oldTools,
		".just/swift.just":            "BuildTools\n",
		".github/workflows/swift.yml": "BuildTools\n",
		".gitignore":                  "# >>> swift-bedrock\nBuildTools/.build\n# <<< swift-bedrock\n",
	}

	// Act
	_, result := planSwift(t, files)

	// Assert
	for _, note := range result.Notes {
		if strings.Contains(note, "BuildTools") {
			t.Errorf("unexpected note: %s", note)
		}
	}
}
