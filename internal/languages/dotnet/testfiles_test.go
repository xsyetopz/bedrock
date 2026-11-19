package dotnet_test

import (
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
)

func TestTestFilesAreThoseUnderATestProjectDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj": `<Project Sdk="Microsoft.NET.Sdk" />` + "\n",
		"src/App/Program.cs": "",
		"tests/A/A.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup>` +
			`<PackageReference Include="xunit.v3" /></ItemGroup></Project>` + "\n",
		"tests/A/ATests.cs": "",
	})
	classifier, ok := bedrocktest.Language(t, "dotnet").(language.TestFileClassifier)

	if !ok {
		t.Fatal("dotnet classifies no test files")
	}

	files := []string{"src/App/Program.cs", "tests/A/ATests.cs"}

	// Act
	found, err := classifier.TestFiles(t.Context(), bedrocktest.Context(t, root, selectDotnet(t)), files)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if !found["tests/A/ATests.cs"] || found["src/App/Program.cs"] || len(found) != 1 {
		t.Errorf("test files = %v", found)
	}
}

func TestTestFilesTreatsEveryFileUnderARootTestProjectAsATest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"App.Tests.csproj": `<Project Sdk="MSTest.Sdk/4.5.3" />` + "\n",
		"Tests.cs":         "",
	})
	classifier, ok := bedrocktest.Language(t, "dotnet").(language.TestFileClassifier)

	if !ok {
		t.Fatal("dotnet classifies no test files")
	}

	// Act
	found, err := classifier.TestFiles(t.Context(), bedrocktest.Context(t, root, selectDotnet(t)), []string{"Tests.cs"})
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if !found["Tests.cs"] {
		t.Errorf("test files = %v", found)
	}
}
