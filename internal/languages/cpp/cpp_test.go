package cpp_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func TestAdoptWritesTheCppFilesAndWorkflowWithoutSwifts(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"CMakeLists.txt": "project(app C)\n"})

	// Act
	result := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "cpp", nil))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{".clang-format", ".clang-tidy", ".just/cpp.just", ".github/workflows/cpp.yml"} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses `%s`: %v", want, paths)
		}
	}

	for _, unwanted := range []string{"Package.swift", ".github/workflows/swift.yml"} {
		if slices.Contains(paths, unwanted) {
			t.Errorf("plan writes `%s`: %v", unwanted, paths)
		}
	}
}

func TestAdoptRemovesTheOldUnderscoreConfigs(t *testing.T) {
	t.Parallel()

	for _, old := range []string{"_clang-format", "_clang-tidy"} {
		t.Run(old, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, map[string]string{"CMakeLists.txt": "project(app C)\n", old: "x\n"})

			// Act
			result := bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "cpp", nil))

			// Assert
			if !slices.Contains(bedrocktest.Paths(result.Steps), old) {
				t.Errorf("plan does not remove `%s`: %v", old, bedrocktest.Paths(result.Steps))
			}

			if _, err := os.Stat(filepath.Join(root, old)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("`%s` is still there: %v", old, err)
			}
		})
	}
}

func TestGuardRecipeRunsBedrockGuardCpp(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"CMakeLists.txt": "project(app C)\n"})

	// Act
	result := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "cpp", nil))

	// Assert
	recipe := bedrocktest.Written(result.Steps, ".just/cpp.just")
	if !strings.Contains(recipe, "{{ just_executable() }} bedrock guard cpp") {
		t.Errorf("`.just/cpp.just` does not run the guard:\n%s", recipe)
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"CMakeLists.txt": "project(app C)\n", "_clang-format": "x\n"})
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "cpp", nil))

	// Act
	again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "cpp", nil))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}
