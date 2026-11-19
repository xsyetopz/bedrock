// Package rust is the Rust brick: Cargo workspaces on strict Clippy and rustdoc
// lints, a pinned toolchain, test modules in their own files, and migration of
// what older rust-bedrock releases left behind.
package rust

import (
	"context"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

// Names the language uses in several places.
const (
	cargoTool     = "cargo"
	toolchainPath = "rust-toolchain.toml"
	bunFFIKind    = "bun-ffi"
)

// Rust is the Rust language.
type Rust struct{}

var (
	_ language.Planner            = Rust{}
	_ language.VariableProvider   = Rust{}
	_ language.Guarder            = Rust{}
	_ language.PostApplier        = Rust{}
	_ language.PendingRunner      = Rust{}
	_ language.TestFileClassifier = Rust{}
)

// New returns the Rust language.
func New() Rust { return Rust{} }

// Spec declares Rust's templates, markers, and tools.
func (Rust) Spec() language.Spec {
	return language.Spec{
		ID:      "rust",
		Block:   "rust-bedrock",
		Markers: []string{"Cargo.toml"},
		Tools: []language.Tool{
			{Name: cargoTool, Install: "rustup, from https://rustup.rs, such as `brew install rustup`"},
		},
		Sources: []string{
			"**/*.rs", "**/Cargo.toml", "Cargo.lock", toolchainPath,
			"rustfmt.toml", "clippy.toml", "deny.toml", ".cargo/**",
		},
		Kinds: kinds(),
		// Clippy has function lints (too_many_lines, too_many_arguments,
		// cognitive_complexity) but no file-length lint.
		Lizard: []string{"rust"},
	}
}

// Variables fill the package and crate names the templates use.
func (Rust) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	return crateVariables(packageName(c.Root)), nil
}

// Plan migrates manifests, the toolchain, test files, and modules, and starts
// the crates at init.
func (Rust) Plan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	return planRust(ctx, c)
}

// Guard reports unit tests that sit beside the code they test, lint
// suppressions without a reason, and files that differ from what adopt writes.
func (Rust) Guard(ctx context.Context, c *language.Context, fix bool) ([]language.Finding, error) {
	return guardRust(ctx, c, fix)
}

// AfterApply migrates each edition older than 2024, then fetches the
// dependencies.
func (Rust) AfterApply(ctx context.Context, c *language.Context) error {
	if !c.Runner.LookPath(cargoTool) {
		return nil
	}

	if err := migrateEditions(ctx, c.Runner, c.Root); err != nil {
		return err
	}

	// A failed fetch, such as offline, leaves the lock for the next cargo run.
	_, err := c.Runner.Run(ctx, execx.Command{Dir: c.Root, Name: cargoTool, Args: []string{"fetch"}, Stream: true})

	return err
}

// Runs names what AfterApply does.
func (Rust) Runs() string {
	return "cargo fix --edition for each edition older than 2024, then cargo fetch"
}

// RunPending reports a migration a skipped or failed run left.
func (Rust) RunPending(ctx context.Context, c *language.Context) (bool, error) {
	if !c.Runner.LookPath(cargoTool) {
		return false, nil
	}

	return editionPending(ctx, c.Runner, c.Root)
}

// TestFiles returns the files rustc compiles only under test.
func (Rust) TestFiles(ctx context.Context, c *language.Context, files []string) (map[string]bool, error) {
	return testFiles(ctx, c, files)
}
