package editorconfig_test

import (
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/editorconfig"
)

const sample = `root = true

# >>> jvm-bedrock
[*.{java,kt}]
Indent_Size = 4
bedrock_file_code_lines_warning = 603
# <<< jvm-bedrock

; generated code
[gen/**]
bedrock_file_code_lines_warning = unset

[/src/Big.kt]
bedrock_file_code_lines_warning = 900
`

const fileBound = "bedrock_file_code_lines_warning"

func TestParseKeepsSectionsInOrderAndLowercasesKeys(t *testing.T) {
	t.Parallel()

	// Act
	sections := editorconfig.Parse(sample)

	// Assert
	var globs []string
	for _, section := range sections {
		globs = append(globs, section.Glob)
	}

	if want := []string{"*.{java,kt}", "gen/**", "/src/Big.kt"}; !slices.Equal(globs, want) {
		t.Errorf("globs = %v, want %v", globs, want)
	}

	if got := sections[0].Pairs[0]; got != (editorconfig.Pair{Key: "indent_size", Value: "4"}) {
		t.Errorf("first pair = %v, want indent_size = 4", got)
	}
}

func TestPropertiesOfOneFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want string
	}{
		{"app/src/A.kt", "603"},
		{"A.java", "603"},
		{"gen/B.kt", ""},
		{"src/Big.kt", "900"},
		{"sub/src/Big.kt", "603"},
		{"README.md", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			sections := editorconfig.Parse(sample)

			// Act
			properties, err := editorconfig.Properties(sections, tc.path)
			if err != nil {
				t.Fatal(err)
			}

			// Assert
			if got := properties[fileBound]; got != tc.want {
				t.Errorf("%s gets a file bound of %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestTreeNestedFilesOverrideAndRootEndsTheSearch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want string
	}{
		{"src/A.kt", "603"},
		{"legacy/A.kt", "900"},
		{"legacy/gen/A.kt", ""},
		{"vendor/A.kt", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, map[string]string{
				".editorconfig":        "root = true\n\n[*.kt]\n" + fileBound + " = 603\n",
				"legacy/.editorconfig": "[*.kt]\n" + fileBound + " = 900\n\n[gen/**]\n" + fileBound + " = unset\n",
				"vendor/.editorconfig": "root = true\n\n[*]\nindent_size = 2\n",
			})

			// Act
			properties, err := editorconfig.NewTree(root).Properties(tc.path)
			if err != nil {
				t.Fatal(err)
			}

			// Assert
			if got := properties[fileBound]; got != tc.want {
				t.Errorf("%s gets a file bound of %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestTreeFilesListsTheRootAndEachNestedFileAPathRead(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".editorconfig":        "root = true\n",
		"legacy/.editorconfig": "[*]\n",
		"other/.editorconfig":  "[*]\n",
	})
	tree := editorconfig.NewTree(root)

	if _, err := tree.Properties("legacy/A.kt"); err != nil {
		t.Fatal(err)
	}

	// Act
	files, err := tree.Files()
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	var paths []string
	for _, file := range files {
		paths = append(paths, file.Path)
	}

	if want := []string{".editorconfig", "legacy/.editorconfig"}; !slices.Equal(paths, want) {
		t.Errorf("files = %v, want %v", paths, want)
	}
}

func TestDeclaresRootReadsOnlyThePreamble(t *testing.T) {
	t.Parallel()

	// Act and Assert
	if !editorconfig.DeclaresRoot("# top\nRoot = TRUE\n\n[*]\nindent_size = 2\n") {
		t.Error("a root = true before the first section should count")
	}

	if editorconfig.DeclaresRoot("[*]\nroot = true\n") {
		t.Error("a root = true inside a section should not count")
	}
}
