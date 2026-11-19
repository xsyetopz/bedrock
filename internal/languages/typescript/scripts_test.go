package typescript_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

// manifestOf adopts files and returns the resulting package.json and the
// plan's notes.
func manifestOf(t *testing.T, manifest string) (map[string]any, []string) {
	t.Helper()

	root := bedrocktest.Repo(t, map[string]string{packageJSON: manifest})
	result := bedrocktest.Adopt(t, root, false, selected(t))

	var parsed map[string]any
	if err := json.Unmarshal([]byte(bedrocktest.Read(t, root, packageJSON)), &parsed); err != nil {
		t.Fatal(err)
	}

	return parsed, result.Notes
}

func TestAdoptRewritesEachScriptOntoBun(t *testing.T) {
	t.Parallel()

	// Arrange
	cases := [][2]string{
		{"npm run build", "bun run build"},
		{"npm test", "bun run test"},
		{"npm ci", "bun install --frozen-lockfile"},
		{"yarn install --immutable", "bun install --frozen-lockfile"},
		{"pnpm install", "bun install"},
		{"pnpm build", "bun run build"},
		{"yarn add -D zod", "bun add -D zod"},
		{"npm publish --access public", "bun publish --access public"},
		{"npx --yes cowsay hi", "bunx cowsay hi"},
		{"pnpm dlx prettier --check .", "bun fmt --check"},
		{"prettier --write .", "bun fmt"},
		{"prettier --write src/**/*.ts", "bun fmt src/**/*.ts"},
		{"oxfmt --check", "bun fmt --check"},
		{"eslint . --ext .ts", "oxlint"},
		{"eslint --fix .", "oxlint --fix"},
		{"biome check --write .", "oxlint --fix"},
		{"biome format .", "bun fmt --check"},
		{"biome format --write .", "bun fmt"},
		{"tsc --noEmit", "bun check"},
		{"tsc --noEmit -p tsconfig.app.json", "bun check -p tsconfig.app.json"},
		{"tsc -p tsconfig.build.json", "tsc -p tsconfig.build.json"},
		{"tsx watch src/main.ts", "bun --watch src/main.ts"},
		{"ts-node src/main.ts", "bun src/main.ts"},
		{"node dist/index.js", "bun dist/index.js"},
		{"jest --coverage", "bun test --coverage"},
		{"vitest run", "bun test"},
		{"vitest --watch", "bun test --watch"},
		{"cross-env NODE_ENV=production tsx src/main.ts", "NODE_ENV=production bun src/main.ts"},
		{"rimraf dist", "rm -rf dist"},
		{"typescript-bedrock guard", "just typescript::guard"},
		{"typescript-bedrock adopt .", "just bedrock adopt ."},
		{"npm whoami", "npm whoami"},
		{"vite build", "vite build"},
		{"rimraf dist && tsc --noEmit || echo failed; npm run x", "rm -rf dist && bun check || echo failed; bun run x"},
		{"FOO=1", "FOO=1"},
	}

	scripts := map[string]string{}
	for i, pair := range cases {
		scripts[fmt.Sprintf("s%02d", i)] = pair[0]
	}

	encoded, err := json.Marshal(map[string]any{"name": "x", "scripts": scripts})
	if err != nil {
		t.Fatal(err)
	}

	// Act
	manifest, _ := manifestOf(t, string(encoded))

	// Assert
	got, _ := manifest["scripts"].(map[string]any)
	for i, pair := range cases {
		if name := fmt.Sprintf("s%02d", i); got[name] != pair[1] {
			t.Errorf("%q became %q, want %q", pair[0], got[name], pair[1])
		}
	}
}

func TestAdoptDropsPackagesBunReplacesAndNamesThem(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"name":"x","devDependencies":{"eslint":"^9","jest":"^29","rimraf":"^5","zod":"^3"}}`

	// Act
	manifest, notes := manifestOf(t, input)

	// Assert
	dev, _ := manifest["devDependencies"].(map[string]any)
	for _, gone := range []string{"eslint", "jest", "rimraf"} {
		if _, ok := dev[gone]; ok {
			t.Errorf("%s stayed in devDependencies", gone)
		}
	}

	if !slices.Contains(notes, "Removed packages Bun replaces: eslint, jest, rimraf.") {
		t.Errorf("notes: %q", notes)
	}
}

func TestAdoptDropsNpmPnpmAndYarnEngines(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"name":"x","engines":{"node":">=20","npm":">=10","yarn":"1.x"}}`

	// Act
	manifest, _ := manifestOf(t, input)

	// Assert
	engines, _ := manifest["engines"].(map[string]any)
	if _, ok := engines["npm"]; ok {
		t.Errorf("engines: %v", engines)
	}

	if engines["node"] != ">=20" || engines["bun"] == nil {
		t.Errorf("engines: %v", engines)
	}
}

func TestAdoptRemovesFieldsOfReplacedToolsAndNotesThem(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"name":"x","prettier":{"semi":false},"lint-staged":{"*":"x"}}`

	// Act
	manifest, notes := manifestOf(t, input)

	// Assert
	for _, key := range []string{"prettier", "lint-staged"} {
		if _, ok := manifest[key]; ok {
			t.Errorf("%s stayed", key)
		}
	}

	if !slices.Contains(notes, "Removed the `\"prettier\"` field from `package.json`.") {
		t.Errorf("notes: %q", notes)
	}
}

func TestAdoptMergesPnpmWorkspaceYamlIntoWorkspaces(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		packageJSON:           `{"name":"x"}`,
		"pnpm-workspace.yaml": "packages:\n  - packages/*\ncatalog:\n  zod: 3.0.0\nonlyBuiltDependencies:\n  - esbuild\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	var manifest struct {
		Workspaces struct {
			Packages []string          `json:"packages"`
			Catalog  map[string]string `json:"catalog"`
		} `json:"workspaces"`
		Trusted []string `json:"trustedDependencies"`
	}

	if err := json.Unmarshal([]byte(bedrocktest.Read(t, root, packageJSON)), &manifest); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(manifest.Workspaces.Packages, []string{"packages/*"}) ||
		manifest.Workspaces.Catalog["zod"] != "3.0.0" || !slices.Equal(manifest.Trusted, []string{"esbuild"}) {
		t.Errorf("manifest: %+v", manifest)
	}

	if strings.Contains(bedrocktest.Read(t, root, packageJSON), "pnpm") {
		t.Error("package.json names pnpm")
	}
}

func TestAdoptNotesAJestScriptAndMovesItToBunTest(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"name":"x","scripts":{"test":"jest"}}`

	// Act
	manifest, notes := manifestOf(t, input)

	// Assert
	scripts, _ := manifest["scripts"].(map[string]any)
	if scripts["test"] != "bun test" {
		t.Errorf("scripts: %v", scripts)
	}

	if !slices.ContainsFunc(notes, func(note string) bool { return strings.Contains(note, "ran Jest or Vitest") }) {
		t.Errorf("notes: %q", notes)
	}
}
