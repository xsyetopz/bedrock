package typescript

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// sourceGlob matches every JavaScript and TypeScript source the checks read.
const sourceGlob = "**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts,vue,svelte,astro}"

// suppression is written as alternations so this file does not match itself.
func suppression() *regexp.Regexp {
	return regexp.MustCompile(`(?:eslint|oxlint)-disable|biome-(?:ignore)|@ts-(?:ignore|expect-error|nocheck)`)
}

// findSuppressions lists the lines of paths that switch a linter or the
// type checker off.
func findSuppressions(root string, paths []string) ([]language.Finding, error) {
	var findings []language.Finding

	for _, path := range paths {
		text, _, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}

		for row, line := range strings.Split(text, "\n") {
			if suppression().MatchString(line) {
				findings = append(findings, fmt.Sprintf("%s:%d: suppression comment", path, row+1))
			}
		}
	}

	return findings, nil
}

func otherPackageManager() *regexp.Regexp {
	return regexp.MustCompile(`\b(?:npm|npx|pnpm|pnpx|yarn)\b`)
}

// foreignFiles are files only another package manager writes. `.npmrc` is
// absent on purpose: Bun reads it for registry auth.
func foreignFiles() []string {
	return []string{
		".pnpmfile.cjs", ".yarnrc", ".yarnrc.yml", nameBunLockb, "npm-shrinkwrap.json", "package-lock.json",
		"pnpm-lock.yaml", "pnpm-workspace.yaml", "yarn.lock",
	}
}

func findForeignFiles(root string) []language.Finding {
	var findings []language.Finding
	for _, path := range fsutil.Existing(root, foreignFiles()) {
		findings = append(findings, path+": not used in a Bun-only repo")
	}

	return findings
}

// findForeignScripts lists scripts in the root package.json that call npm,
// pnpm, or yarn.
func findForeignScripts(manifest *object) []language.Finding {
	scripts := asObject(asObject(manifest).get("scripts"))

	var findings []language.Finding

	for _, name := range scripts.keys {
		if command, ok := scripts.get(name).(string); ok && otherPackageManager().MatchString(command) {
			findings = append(findings, "`package.json`: script "+quote(name)+" calls another package manager")
		}
	}

	return findings
}

// findUnsortedCatalogs checks catalogs, which `bun fmt` does not sort the way
// it sorts dependencies.
func findUnsortedCatalogs(manifest *object) []language.Finding {
	workspaces := asObject(asObject(manifest).get("workspaces"))
	catalogs := []struct {
		name  string
		value any
	}{{"catalog", workspaces.get("catalog")}}

	named := asObject(workspaces.get("catalogs"))
	for _, name := range named.keys {
		catalogs = append(catalogs, struct {
			name  string
			value any
		}{"catalogs." + name, named.get(name)})
	}

	var findings []language.Finding

	for _, catalog := range catalogs {
		keys := asObject(catalog.value).keys
		if !slices.IsSorted(keys) {
			findings = append(findings, "`package.json`: sort `workspaces."+catalog.name+"` by package name")
		}
	}

	return findings
}

// findTypescriptDrift reports an installed `typescript` that differs from
// the one `bun check` runs.
func findTypescriptDrift(ctx context.Context, c *language.Context) ([]language.Finding, error) {
	installed, err := readJSONC(filepath.Join(c.Root, "node_modules", "typescript", namePackageJSON))
	if err != nil || installed == nil {
		return nil, err
	}

	// The compiled bedrock binary carries its own Bun; `bun check` is the repo's.
	result, err := c.Runner.Run(ctx, execx.Command{
		Dir: c.Root, Name: nameBun, Args: []string{"-e", "console.log(process.versions.typescript)"},
	})
	if err != nil || !result.OK() {
		return nil, nil //nolint:nilerr // A missing or broken Bun cannot be compared, and the tool check names it.
	}

	builtIn := strings.TrimSpace(string(result.Stdout))
	version, _ := installed.get("version").(string)

	if version == builtIn {
		return nil, nil
	}

	return []language.Finding{
		"typescript: " + version + " is installed but `bun check` runs " + builtIn + "; set the catalog to " + builtIn,
	}, nil
}

// Guard reports what the linters cannot see: suppression comments, other
// package managers, undocumented package API, TypeScript drift, and unsorted
// catalogs. Managed files that differ from the templates are core's drift
// check.
func (Typescript) Guard(ctx context.Context, c *language.Context, _ bool) ([]language.Finding, error) {
	ignore, err := readOxlintIgnores(c.Root)
	if err != nil {
		return nil, err
	}

	manifest, err := readJSONC(filepath.Join(c.Root, namePackageJSON))
	if err != nil {
		return nil, err
	}

	paths, err := listSources(ctx, c.Runner, c.Root, sourceGlob, ignore)
	if err != nil {
		return nil, err
	}

	findings, err := findSuppressions(c.Root, paths)
	if err != nil {
		return nil, err
	}

	api, err := findUndocumentedAPI(ctx, c, paths, ignore)
	if err != nil {
		return nil, err
	}

	drift, err := findTypescriptDrift(ctx, c)
	if err != nil {
		return nil, err
	}

	findings = slices.Concat(findings, api, findForeignFiles(c.Root), findForeignScripts(manifest), drift,
		findUnsortedCatalogs(manifest))

	return findings, nil
}
