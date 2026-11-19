package dotnet_test

import (
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
)

func guard(t *testing.T, files map[string]string) []string {
	t.Helper()

	root := bedrocktest.Repo(t, files)
	guarder, ok := bedrocktest.Language(t, "dotnet").(language.Guarder)

	if !ok {
		t.Fatal("dotnet has no guard")
	}

	found, err := guarder.Guard(t.Context(), bedrocktest.Context(t, root, selectDotnet(t)), false)
	if err != nil {
		t.Fatal(err)
	}

	return found
}

func TestGuardReportsACSharpPragmaAndSuppressMessageWithoutAReason(t *testing.T) {
	t.Parallel()

	// Arrange
	source := "#pragma warning disable CA1822\n" +
		"#pragma warning disable CA1822 // instance method for the test runner\n" +
		"[SuppressMessage(\"Design\", \"CA1031\")]\n" +
		"[SuppressMessage(\"Design\", \"CA1031\", Justification = \"top-level handler\")]\n"

	// Act
	found := guard(t, map[string]string{"src/A.cs": source})

	// Assert
	want := []string{
		"src/A.cs:1: `#pragma warning disable` without a reason; add `// <reason>` after the ids",
		"src/A.cs:3: `SuppressMessage` without `Justification`; give the reason in `Justification = \"...\"`",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings = %q, want %q", found, want)
	}
}

func TestGuardReportsAnFSharpNowarnWithoutAReason(t *testing.T) {
	t.Parallel()

	// Arrange
	source := "#nowarn \"40\"\n#nowarn \"3370\" // the indexer syntax the API needs\n"

	// Act
	found := guard(t, map[string]string{"src/A.fs": source})

	// Assert
	want := []string{"src/A.fs:1: `#nowarn` without a reason; add `// <reason>` after the ids"}
	if !slices.Equal(found, want) {
		t.Errorf("findings = %q, want %q", found, want)
	}
}

func TestGuardReportsAFormatterConfigAndASecondToolManifest(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		".csharpierrc.json":         "{}\n",
		"dotnet-tools.json":         "{}\n",
		".config/dotnet-tools.json": "{}\n",
	}

	// Act
	found := guard(t, files)

	// Assert
	want := []string{
		".csharpierrc.json: the formatters read their layout from `.editorconfig`; " +
			"move the settings there and delete this file",
		".config/dotnet-tools.json, dotnet-tools.json: two tool manifests; " +
			"keep the tools in `.config/dotnet-tools.json` and delete the other",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings = %q, want %q", found, want)
	}
}

func TestGuardPassesExplainedSuppressions(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"src/A.cs":                  "#pragma warning disable CA1822 // reason\n",
		".config/dotnet-tools.json": "{}\n",
	}

	// Act
	found := guard(t, files)

	// Assert
	if len(found) != 0 {
		t.Errorf("findings: %v", found)
	}
}
