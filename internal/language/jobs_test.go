package language_test

import (
	"testing"

	"github.com/xsyetopz/bedrock/internal/language"
)

func TestJobsPrefersTheLanguageVariableThenTheSharedOne(t *testing.T) {
	cases := []struct {
		name, own, shared string
		want              int
	}{
		{"unset", "", "", language.DefaultJobs},
		{"shared", "", "6", 6},
		{"own over shared", "2", "6", 2},
		{"invalid own falls back", "many", "6", 6},
		{"zero counts as unset", "0", "", language.DefaultJobs},
		{"negative counts as unset", "", "-3", language.DefaultJobs},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			t.Setenv("BEDROCK_CPP_JOBS", tc.own)
			t.Setenv("BEDROCK_JOBS", tc.shared)

			// Act
			got := language.Jobs("cpp")

			// Assert
			if got != tc.want {
				t.Fatalf("Jobs(cpp) = %d, want %d", got, tc.want)
			}
		})
	}
}
