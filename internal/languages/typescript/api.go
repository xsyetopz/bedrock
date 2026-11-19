package typescript

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// apiCache sits under `node_modules` so the config's bare plugin specifier
// resolves from the repo.
const apiCache = "node_modules/.cache"

func reExport() *regexp.Regexp {
	return regexp.MustCompile(
		`export\s+(?:type\s+)?(?:\*(?:\s+as\s+[\w$]+)?|\{[^}]*\})\s+from\s+["'](\.{1,2}/[^"']+)["']`)
}

func specifierEndings() []string {
	return []string{"", ".ts", ".tsx", ".mts", ".js", ".mjs", "/index.ts", "/index.js"}
}

func undocumented() *regexp.Regexp {
	return regexp.MustCompile(`^(.+?):(\d+):\d+: .* \[Error/jsdoc-js\(require-jsdoc\)\]$`)
}

// leaves are the string leaves of a manifest value.
func leaves(value any) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}

	var out []string

	inner := asObject(value)
	for _, key := range inner.keys {
		out = append(out, leaves(inner.get(key))...)
	}

	return out
}

// entryPoints are the files other packages import: the string leaves of
// `exports`, `main`, and `module`.
func entryPoints(manifest *object) []string {
	var out []string

	for _, key := range manifest.keys {
		if key == "exports" || key == "main" || key == "module" {
			out = append(out, leaves(manifest.get(key))...)
		}
	}

	return out
}

// reExported lists the files text re-exports from, among sources.
func reExported(file, text string, sources map[string]bool) []string {
	var out []string

	for _, match := range reExport().FindAllStringSubmatch(text, -1) {
		base := path.Join(path.Dir(file), match[1])

		for _, ending := range specifierEndings() {
			if sources[base+ending] {
				out = append(out, base+ending)

				break
			}
		}
	}

	return out
}

// listAPIFiles lists the source files each package exposes: its entry
// points and, through relative re-exports, the files they re-export from.
func listAPIFiles(ctx context.Context, c *language.Context, sourcePaths, ignore []string) ([]string, error) {
	sources := map[string]bool{}
	for _, source := range sourcePaths {
		sources[source] = true
	}

	manifests, err := listSources(ctx, c.Runner, c.Root, "**/package.json", ignore)
	if err != nil {
		return nil, err
	}

	var pending []string

	for _, manifestPath := range manifests {
		manifest, readErr := readJSONC(filepath.Join(c.Root, filepath.FromSlash(manifestPath)))
		if readErr != nil {
			return nil, readErr
		}

		for _, entry := range entryPoints(asObject(manifest)) {
			pending = append(pending, path.Join(path.Dir(manifestPath), entry))
		}
	}

	found := map[string]bool{}

	for len(pending) > 0 {
		var next []string

		for _, file := range unique(pending) {
			if !sources[file] || found[file] {
				continue
			}

			found[file] = true

			text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
			if readErr != nil {
				return nil, readErr
			}

			next = append(next, reExported(file, text, sources)...)
		}

		pending = next
	}

	files := make([]string, 0, len(found))
	for file := range found {
		files = append(files, file)
	}

	slices.Sort(files)

	return files, nil
}

// findUndocumentedAPI reports exported declarations in the package API that
// lack a JSDoc comment.
func findUndocumentedAPI(
	ctx context.Context, c *language.Context, sourcePaths, ignore []string,
) ([]language.Finding, error) {
	files, err := listAPIFiles(ctx, c, sourcePaths, ignore)
	if err != nil || len(files) == 0 {
		return nil, err
	}

	config, _, err := c.Dir("typescript", "merge").Read("oxlintrc.api.json", nil)
	if err != nil {
		return nil, fmt.Errorf("read the API lint template: %w", err)
	}

	cache := filepath.Join(c.Root, filepath.FromSlash(apiCache))
	if err = os.MkdirAll(cache, 0o750); err != nil {
		return nil, fmt.Errorf("create `%s`: %w", apiCache, err)
	}

	dir, err := os.MkdirTemp(cache, "bedrock-api-")
	if err != nil {
		return nil, fmt.Errorf("create a cache directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	file := filepath.Join(dir, "oxlintrc.api.json")
	if err = os.WriteFile(file, []byte(config), 0o600); err != nil {
		return nil, fmt.Errorf("write `%s`: %w", file, err)
	}

	args := slices.Concat([]string{nameRun, nameOxlint, "--disable-nested-config", "-c", file, "-f", "unix"}, files)

	result, err := c.Runner.Run(ctx, execx.Command{Dir: c.Root, Name: nameBun, Args: args})
	if err != nil {
		return nil, fmt.Errorf("run `bun run oxlint`: %w", err)
	}

	var problems []language.Finding

	for line := range strings.SplitSeq(string(result.Stdout), "\n") {
		if match := undocumented().FindStringSubmatch(line); match != nil {
			problems = append(problems, match[1]+":"+match[2]+": add a JSDoc comment; the package exports this")
		}
	}

	if !result.OK() && len(problems) == 0 {
		return []language.Finding{
			"`oxlint` failed on the package API: " + strings.TrimSpace(string(result.Stderr)),
		}, nil
	}

	slices.Sort(problems)

	return problems, nil
}
