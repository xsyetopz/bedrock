package editorconfig_test

import (
	"testing"

	"github.com/xsyetopz/bedrock/internal/editorconfig"
)

func TestGlobMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		glob string
		path string
		want bool
	}{
		{"*.kt", "A.kt", true},
		{"*.kt", "app/src/A.kt", true},
		{"*.kt", "A.kts", false},
		{"src/*.kt", "src/A.kt", true},
		{"src/*.kt", "src/sub/A.kt", false},
		{"src/*.kt", "app/src/A.kt", false},
		{"/src/*.kt", "src/A.kt", true},
		{"src/**/*.kt", "src/A.kt", true},
		{"src/**/*.kt", "src/a/b/A.kt", true},
		{"gen/**", "gen/a/B.kt", true},
		{"?.kt", "A.kt", true},
		{"?.kt", "AB.kt", false},
		{"[ab].kt", "b.kt", true},
		{"[!ab].kt", "b.kt", false},
		{"[!ab].kt", "c.kt", true},
		{"*.{java,kt,kts}", "A.kts", true},
		{"*.{java,kt}", "A.scala", false},
		{"{src,test}/*.{java,kt}", "test/A.java", true},
		{"*.{kt}", "A.{kt}", true},
		{"*.{kt}", "A.kt", false},
		{"Day{1..12}.kt", "Day7.kt", true},
		{"Day{1..12}.kt", "Day12.kt", true},
		{"Day{1..12}.kt", "Day13.kt", false},
		{"Day{1..12}.kt", "Day0.kt", false},
		{"v{-3..3}.txt", "v-2.txt", true},
		{"v{3..-3}.txt", "v4.txt", false},
		{`a\*.kt`, "a*.kt", true},
		{`a\*.kt`, "ab.kt", false},
		{"a.kt", "aXkt", false},
		{"[a.kt", "[a.kt", true},
	}

	for _, tc := range cases {
		t.Run(tc.glob+" "+tc.path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			glob, err := editorconfig.Compile(tc.glob)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			got := glob.Match(tc.path)

			// Assert
			if got != tc.want {
				t.Errorf("%s matches %s: got %v, want %v", tc.glob, tc.path, got, tc.want)
			}
		})
	}
}

func TestCompileRejectsAnInvertedCharacterRange(t *testing.T) {
	t.Parallel()

	// Act
	_, err := editorconfig.Compile("[z-a].kt")

	// Assert
	if err == nil {
		t.Error("want an error for the range z-a, got none")
	}
}
