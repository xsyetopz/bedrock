package typescript_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func TestInitPlansEachKindsFilesScriptsAndPackages(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind, platform string
		files          []string
		manifest       []string
		tsconfig       string
	}{
		{
			"server", "",
			[]string{"src/app.ts", "src/index.ts", "src/app.test.ts", "scripts/build.ts"},
			[]string{`"build": "bun scripts/build.ts src/index.ts"`, `"start": "bun src/index.ts"`},
			"",
		},
		{
			"server", "linux,macos,windows", nil,
			[]string{`"build": "bun scripts/build.ts src/index.ts linux macos windows"`},
			"",
		},
		{
			"cli", "linux,windows",
			[]string{"src/cli.ts", "scripts/build.ts"},
			[]string{`"build": "bun scripts/build.ts src/cli.ts linux windows"`},
			"",
		},
		{
			"react", "",
			[]string{"src/index.ts", "src/index.html", "src/frontend.tsx", "src/app.tsx", "src/app.test.tsx"},
			[]string{`"react": "catalog:"`, `"@types/react-dom": "catalog:"`, `"react-dom": "19.3.1"`},
			`"jsx": "react-jsx"`,
		},
	} {
		t.Run(tc.kind+"@"+tc.platform, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			selected := bedrocktest.Select(t, "typescript", map[string]string{"kind": tc.kind, "platform": tc.platform})

			// Act
			result := bedrocktest.Plan(t, root, true, selected)

			// Assert
			paths := bedrocktest.Paths(result.Steps)
			for _, file := range tc.files {
				if !slices.Contains(paths, file) {
					t.Errorf("no %s in the plan: %q", file, paths)
				}
			}

			manifest := bedrocktest.Written(result.Steps, packageJSON)
			for _, want := range tc.manifest {
				if !strings.Contains(manifest, want) {
					t.Errorf("package.json misses %s:\n%s", want, manifest)
				}
			}

			if got := bedrocktest.Written(result.Steps, "tsconfig.json"); !strings.Contains(got, tc.tsconfig) {
				t.Errorf("tsconfig.json misses %s:\n%s", tc.tsconfig, got)
			}
		})
	}
}

func TestSecondAdoptOfAnInitializedKindPlansNothingMore(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"library", "server", "react"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			selected := bedrocktest.Select(t, "typescript", map[string]string{"kind": kind, "platform": ""})
			bedrocktest.Adopt(t, root, true, selected)
			bedrocktest.CommitAll(t, root)

			// Act
			again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "typescript", nil))

			// Assert
			// `.editorconfig` holds the bedrock version line, which the core owns and the
			// test's selection does not carry; every file the language writes must stay put.
			paths := slices.DeleteFunc(bedrocktest.Paths(again.Steps), func(path string) bool {
				return path == ".editorconfig"
			})
			if len(paths) != 0 {
				t.Errorf("a second adopt plans %q", paths)
			}
		})
	}
}
