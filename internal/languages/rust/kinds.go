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
func kinds() []string {
	return []string{
		libKind, binKind, "no-std", ffiKind, wasmName, bunFFIKind,
		"proc-macro", "cli", "service", "staticlib",
	}
}

// The kind and platform names the kind list, the target maps, and the crate
// templates share.
const (
	wasmName        = "wasm" // both the `wasm` kind and the `wasm` platform
	libKind         = "lib"
	binKind         = "bin"
	ffiKind         = "ffi"
	platformLinux   = "linux"
	platformMacOS   = "macos"
	platformWASI    = "wasi"
	platformWindows = "windows"
)

// wasmTarget is the Rust target a `wasm` crate builds for.
const wasmTarget = "wasm32-unknown-unknown"

// triples maps each platform to the rustup targets that build it. tvOS,
// watchOS, and visionOS are tier 3: rustup ships no standard library for
// them, so they need nightly `-Zbuild-std` and stay out.
func triples() map[string][]string {
	return map[string][]string{
		"android":       {"aarch64-linux-android", "x86_64-linux-android"},
		"ios":           {"aarch64-apple-ios", "aarch64-apple-ios-sim"},
		platformLinux:   {"x86_64-unknown-linux-gnu"},
		platformMacOS:   {"aarch64-apple-darwin", "x86_64-apple-darwin"},
		platformWASI:    {"wasm32-wasip2"},
		wasmName:        {wasmTarget},
		platformWindows: {"x86_64-pc-windows-msvc"},
	}
}

// kindPlatforms returns the platforms each kind builds for. A library, static
// library, or C ABI crate goes anywhere Rust does; a program goes where it can
// open files or a console; a `service` needs sockets, so no WASI. A kind left
// out (`proc-macro`, which runs in the compiler, and `wasm`, which is already
// one target) builds for the host.
func kindPlatforms() map[string][]string {
	everywhere := []string{"android", "ios", platformLinux, platformMacOS, platformWASI, wasmName, platformWindows}
	programs := []string{platformLinux, platformMacOS, platformWASI, platformWindows}

	return map[string][]string{
		libKind:     everywhere,
		"staticlib": everywhere,
		ffiKind:     everywhere,
		"no-std":    everywhere,
		binKind:     programs,
		"cli":       programs,
		"service":   {platformLinux, platformMacOS, platformWindows},
	}
}

// targetsOf returns the rustup targets the wanted kinds and the `platform`
// option call for: each listed platform, plus the `wasm` kind's own target. A
// platform no wanted kind supports is an error.
func targetsOf(options config.Options, wanted []string) ([]string, error) {
	listed, err := config.SplitList(options["platform"])
	if err != nil {
		return nil, fmt.Errorf("platform: %w", err)
	}

	var targets []string

	if slices.Contains(wanted, wasmName) {
		targets = append(targets, wasmTarget)
	}

	supported := kindPlatforms()

	for _, platform := range listed {
		known, ok := triples()[platform]
		if !ok {
			return nil, fmt.Errorf("platform %q: rust builds for %s", platform, strings.Join(platformNames(), ", "))
		}

		if !slices.ContainsFunc(wanted, func(kind string) bool { return slices.Contains(supported[kind], platform) }) {
			return nil, fmt.Errorf(
				"platform %q: none of the kinds %s builds for it", platform, strings.Join(wanted, ", "))
		}

		targets = append(targets, known...)
	}

	return targets, nil
}

// platformNames returns the platforms Rust maps, sorted.
func platformNames() []string {
	names := make([]string, 0, len(triples()))
	for name := range triples() {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// kindsOf returns the kinds the `kind` option names, or a library when it
// names none.
func kindsOf(options config.Options) ([]string, error) {
	listed, err := config.SplitList(options["kind"])
	if err != nil {
		return nil, fmt.Errorf("kind: %w", err)
	}

	if len(listed) == 0 {
		return []string{libKind}, nil
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
		source = ffiKind
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
