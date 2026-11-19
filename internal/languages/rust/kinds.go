package rust

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// kinds are what `bedrock init` can start; each has crate templates under
// `templates/rust/crates`.
func kinds() []string { return []string{"lib", "bin", "no-std", "ffi", "wasm", bunFFIKind} }

// wasmTarget is the Rust target a `wasm` crate builds for.
const wasmTarget = "wasm32-unknown-unknown"

// kindsOf returns the kinds the `kind` option names, or a library when it
// names none.
func kindsOf(options config.Options) ([]string, error) {
	listed, err := config.SplitList(options["kind"])
	if err != nil {
		return nil, fmt.Errorf("kind: %w", err)
	}

	if len(listed) == 0 {
		return []string{"lib"}, nil
	}

	var unique []string

	for _, kind := range listed {
		if !slices.Contains(unique, kind) {
			unique = append(unique, kind)
		}
	}

	return unique, nil
}

// packageName returns the package name for the project: its directory name, in
// lower case, with characters Cargo rejects replaced by hyphens and a prefix
// when it would start with a digit.
func packageName(root string) string {
	// Clippy rejects crate names that are not snake case, so capitals go.
	cleaned := regexp.MustCompile(`[^\w-]+`).ReplaceAllString(strings.ToLower(filepath.Base(root)), "-")
	cleaned = strings.Trim(cleaned, "-")

	switch {
	case cleaned == "":
		return "app"
	case cleaned[0] >= '0' && cleaned[0] <= '9':
		return "app-" + cleaned
	default:
		return cleaned
	}
}

// crateVariables returns the placeholder values for one crate.
func crateVariables(pkg string) template.Variables {
	return template.Variables{"package": pkg, "crate": strings.ReplaceAll(pkg, "-", "_")}
}

// packageOf returns one crate's package name: the project's alone, else
// `<name>-<kind>`.
func packageOf(name, kind string, count int) string {
	if count == 1 {
		return name
	}

	return name + "-" + kind
}

// templateSteps writes each template under dir at the path target gives.
func templateSteps(
	c *language.Context,
	dir string,
	target func(path string) string,
	vars template.Variables,
) ([]step.Step, error) {
	files, err := c.Dir("rust", dir).Files(vars)
	if err != nil {
		return nil, fmt.Errorf("render `%s`: %w", dir, err)
	}

	steps := make([]step.Step, 0, len(files))
	for _, file := range files {
		steps = append(steps, step.Write{Path: target(file.Path), Content: file.Content, Executable: false})
	}

	return steps, nil
}

// crateSteps returns the steps for one crate. A `bun-ffi` crate is an `ffi`
// crate plus its Bun side at `src/<package>.ts`.
func crateSteps(c *language.Context, kind, pkg string) ([]step.Step, error) {
	vars := crateVariables(pkg)

	source := kind
	if kind == bunFFIKind {
		source = "ffi"
	}

	steps, err := templateSteps(
		c,
		"crates/"+source,
		func(path string) string { return "crates/" + pkg + "/" + path },
		vars,
	)
	if err != nil || kind != bunFFIKind {
		return steps, err
	}

	bun, err := templateSteps(c, "crates/bun-ffi", func(path string) string { return path }, vars)

	return append(steps, bun...), err
}

// kindSteps returns the crates `bedrock init` creates, and a note where a kind
// needs another language.
func kindSteps(c *language.Context, wanted []string) ([]step.Step, []string, error) {
	name := packageName(c.Root)

	var steps []step.Step

	for _, kind := range wanted {
		crate, err := crateSteps(c, kind, packageOf(name, kind, len(wanted)))
		if err != nil {
			return nil, nil, err
		}

		steps = append(steps, crate...)
	}

	var notes []string

	if slices.Contains(wanted, bunFFIKind) && !c.Has("typescript") {
		notes = append(notes,
			"`bun-ffi` writes its Bun side to `src/`, which needs Bun: adopt `typescript` too "+
				"(`bedrock adopt typescript`) to lint and test it.")
	}

	return steps, notes, nil
}
