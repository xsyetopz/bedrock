// Package language defines what a language plugs into bedrock. A language is
// a brick: a Spec that names its templates, markers, and tools, plus any of
// the optional capabilities below. The core composes whichever bricks a
// repository selects, so a repository mixes languages freely and a new
// language touches only its own package, its templates, and the catalog.
//
// The smallest language is a Spec and a templates/<id> directory. Each
// capability is a separate interface the core detects with a type
// assertion, so a language implements only what it needs:
//
//	Planner            steps beyond the template copies, such as manifest migrations
//	VariableProvider   values for the language's own template placeholders
//	Guarder            findings the linters cannot see
//	PostApplier        a command after the steps apply, such as bun install
//	TestFileClassifier which files the toolchain treats as test-only
//	SubcommandProvider extra commands, such as bedrock jvm j2k
package language

import (
	"context"
	"io"
	"io/fs"
	"slices"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// Spec is the data every language declares.
type Spec struct {
	// Tools are executables adopt needs, each with how to install it.
	Tools []Tool
	// Flags are the adopt options this language adds, such as --group.
	Flags []Flag
	// ID names templates/<id> and .just/<id>.just, and usually the package
	// under internal/languages; go's is golang, since go is not a package name
	// a reader can tell from the language.
	ID string
	// Block is the block name in shared files, such as rust-bedrock. Blocks
	// keep the names releases before the Go CLI wrote.
	Block string
	// Aliases are other lower-case names accepted on the command line.
	Aliases []string
	// Markers are paths, or globs such as Sources/**/*.cpp, whose match
	// under the root shows the repository uses the language.
	Markers []string
	// Sources are globs of the files this language's checks read; the git
	// hooks run its checks only when a changed path matches one.
	Sources []string
	// Kinds are the starters bedrock init can write, each a
	// templates/<id>/kinds/<kind> directory.
	Kinds []string
	// Platforms maps a kind to the targets `bedrock init <lang>:<kind>@<platform>`
	// may name for it, drawn from Platforms(). A kind left out builds for the
	// host alone. Each platform's extra templates, if any, sit under
	// templates/<id>/platforms/<kind>/<platform>.
	Platforms map[string][]string
	// Executables are globs of written paths that must run as programs,
	// such as **/gradlew.
	Executables []string
	// Lizard names, such as rust or kotlin, of the sources whose size the
	// guard bounds because this language's linters leave a metric out.
	Lizard []string
}

// Tool is an executable a language needs on PATH.
type Tool struct {
	Name string
	// Install says how to get it, such as brew install go.
	Install string
}

// Flag is an adopt option a language contributes. A set flag selects the
// language: a boolean flag adds it, and a valued flag stores its value as
// the option of the same name.
type Flag struct {
	// Validate checks a value before it is stored; nil accepts any.
	Validate func(value string) error
	// Name is the long flag without dashes, such as group.
	Name string
	// Value names the argument in the help, such as <pkg>; empty for a
	// boolean flag.
	Value string
	Usage string
	// Requires makes the flag an error unless the run names the language,
	// for a setting that has no meaning alone.
	Requires bool
}

// Language is one brick. Spec is the only required method.
type Language interface {
	Spec() Spec
}

// Context is what a language plans and checks with.
type Context struct {
	// Runner starts the toolchain's programs.
	Runner execx.Runner
	// Templates is the embedded template tree.
	Templates fs.FS
	// Stdout and Stderr receive what a command prints for a person.
	Stdout io.Writer
	Stderr io.Writer
	// Options are this language's settings from .editorconfig merged with
	// the command line.
	Options config.Options
	// Root is the repository's absolute path.
	Root string
	// Languages are the ids of every language this run covers, so one
	// language can tell whether another, such as typescript, is present.
	Languages []string
	// Init is true for bedrock init, which also writes kinds/<kind>.
	Init bool
}

// Dir returns templates/<id>/<dir> for the language with that id.
func (c *Context) Dir(id, dir string) template.Dir {
	return template.NewDir(c.Templates, id+"/"+dir)
}

// Has reports whether this run covers the language id.
func (c *Context) Has(id string) bool {
	return slices.Contains(c.Languages, id)
}

// Contribution is what a Planner adds to an adopt beyond the template copies.
type Contribution struct {
	// Steps are changes such as migrations or removed foreign tools.
	Steps []step.Step
	// Notes are things adopt could not carry over, for a person to review.
	Notes []string
	// ProjectFiles are written paths that stay the project's own; the
	// drift check skips them.
	ProjectFiles []string
	// Unused are paths under templates/<id>/files the repository has no use
	// for, such as Gradle convention plugins no build script applies. The
	// plan skips writing them and removes a copy an earlier run left, so
	// the repository holds only the managed files its build reads.
	Unused []string
}

// Finding is one guard result: "path:line: message" or "path: message".
type Finding = string

// Planner adds steps to an adopt.
type Planner interface {
	Plan(ctx context.Context, c *Context) (Contribution, error)
}

// VariableProvider fills the language's template placeholders.
type VariableProvider interface {
	Variables(ctx context.Context, c *Context) (template.Variables, error)
}

// Guarder reports what the linters cannot see. With fix, it repairs what it
// can first.
type Guarder interface {
	Guard(ctx context.Context, c *Context, fix bool) ([]Finding, error)
}

// PostApplier runs a command after the steps apply; --no-run skips it.
type PostApplier interface {
	AfterApply(ctx context.Context, c *Context) error
	// Runs says what AfterApply does, as the plan lists it.
	Runs() string
}

// PendingRunner reports AfterApply work a skipped or failed run left, so a
// plan with no steps still runs it.
type PendingRunner interface {
	RunPending(ctx context.Context, c *Context) (bool, error)
}

// TestFileClassifier returns which of files, relative to root, the
// toolchain treats as test-only, so the size guard bounds them by the
// test-file limits.
type TestFileClassifier interface {
	TestFiles(ctx context.Context, c *Context, files []string) (map[string]bool, error)
}

// Subcommand is a language command, run as bedrock <id> <name> [args].
type Subcommand struct {
	Run func(ctx context.Context, c *Context, args []string) (int, error)
	// Name is the word after the language id.
	Name string
	// Args shows the arguments in the usage line, such as [<path>...].
	Args    string
	Summary string
}

// SubcommandProvider adds commands under the language's id.
type SubcommandProvider interface {
	Subcommands() []Subcommand
}
