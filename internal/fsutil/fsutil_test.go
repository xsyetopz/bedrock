package fsutil_test

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

func TestListFilesLeavesOutIgnoredGeneratedNestedAndLinkedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".gitignore":                  ".claude/worktrees/\nbuild/\n",
		".gitattributes":              "gen/** linguist-generated\nmock.go linguist-generated=true\n",
		"main.go":                     "package main\n",
		"build/out.go":                "package main\n",
		"gen/api.go":                  "package gen\n",
		"mock.go":                     "package main\n",
		".claude/worktrees/a/copy.go": "package main\n",
	})
	bedrocktest.Write(t, root, "untracked.go", "package main\n")
	nested := bedrocktest.Repo(t, map[string]string{"inner.go": "package inner\n"})
	if err := os.Rename(nested, filepath.Join(root, "nested")); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink("main.go", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}

	runner := execx.System{Stdout: io.Discard, Stderr: io.Discard}

	// Act
	files, err := fsutil.ListFiles(t.Context(), runner, root, func(path string) bool {
		return filepath.Ext(path) == ".go"
	})

	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"main.go", "untracked.go"}; !slices.Equal(files, want) {
		t.Errorf("files = %v, want %v", files, want)
	}
}
