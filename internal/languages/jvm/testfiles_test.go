package jvm_test

import (
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
)

func TestTestFilesClassifiesGradleTestSourceSets(t *testing.T) {
	t.Parallel()

	// Arrange
	tests := []string{
		"app/src/test/kotlin/MainTest.kt",
		"lib/src/testFixtures/java/Fixture.java",
		"app/src/testDebug/kotlin/A.kt",
		"app/src/androidTest/kotlin/A.kt",
		"app/src/androidUnitTest/kotlin/A.kt",
		"shared/src/commonTest/kotlin/A.kt",
		"src/integrationTest/java/A.java",
	}
	others := []string{
		"app/src/main/kotlin/Main.kt",
		"src/testing/java/A.java",
		"src/latest/java/A.java",
		"test/A.java",
	}
	selected := jvm(t, "", "")

	classifier, ok := selected.Language.(language.TestFileClassifier)
	if !ok {
		t.Fatal("jvm classifies no test files")
	}

	// Act
	found, err := classifier.TestFiles(t.Context(), bedrocktest.Context(t, t.TempDir(), selected),
		append(append([]string{}, tests...), others...))
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	for _, file := range tests {
		if !found[file] {
			t.Errorf("%s is a test file", file)
		}
	}

	for _, file := range others {
		if found[file] {
			t.Errorf("%s is no test file", file)
		}
	}
}
