package cpp_test

import (
	"testing"

	"github.com/xsyetopz/bedrock/internal/languages/cpp"
)

func TestIdentifierMakesACppNameFromADirectory(t *testing.T) {
	t.Parallel()

	for dir, want := range map[string]string{
		"my-app":    "my_app",
		"My App.v2": "my_app_v2",
		"2d":        "p2d",
		"snake_ok":  "snake_ok",
		"日本":        "p",
	} {
		if got := cpp.Identifier(dir); got != want {
			t.Errorf("Identifier(%q) = %q, want %q", dir, got, want)
		}
	}
}
