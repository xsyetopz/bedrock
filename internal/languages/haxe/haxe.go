// Package haxe is the Haxe brick: haxe-formatter, haxe-checkstyle with a
// config that bounds file and function size, a type check of each hxml
// build, and the test hxml files that run their tests. It works alone or
// beside other languages.
package haxe

import (
	"github.com/xsyetopz/bedrock/internal/language"
)

// program is the language id, its alias, and the compiler's executable.
const program = "haxe"

// Haxe is the Haxe language.
type Haxe struct{}

// New returns the Haxe language.
func New() Haxe { return Haxe{} }

// Spec declares Haxe's templates, markers, and tools.
//
// `checkstyle.json` bounds file length, method length, complexity, and
// parameters (`FileLength`, `MethodLength`, `CyclomaticComplexity`,
// `ParameterNumber`), and lizard reads no Haxe, so no lizard bound applies.
func (Haxe) Spec() language.Spec {
	return language.Spec{
		ID:      program,
		Block:   "haxe-bedrock",
		Aliases: []string{program, "hx"},
		Markers: []string{".just/haxe.just", "haxelib.json", "**/*.hxml", "**/*.hx"},
		Tools: []language.Tool{{
			Name:    program,
			Install: "https://haxe.org/download, such as `brew install haxe`, which brings `haxelib`",
		}},
		Sources: []string{"**/*.hx", "**/*.hxml", "haxelib.json", "checkstyle.json", "hxformat.json"},
	}
}

// Subcommands are the commands the recipes in `.just/haxe.just` call.
func (Haxe) Subcommands() []language.Subcommand {
	return []language.Subcommand{
		{
			Name:    "format",
			Args:    "[--check]",
			Summary: "Format the Haxe sources, or with `--check` fail if one is not.",
			Run:     format,
		},
		{Name: "lint", Args: "", Summary: "Run checkstyle over the Haxe sources.", Run: lint},
		{
			Name:    "typecheck",
			Args:    "",
			Summary: "Type-check each hxml file that names its target.",
			Run:     typecheck,
		},
		{Name: "test", Args: "", Summary: "Run each test hxml file that runs its tests.", Run: test},
	}
}
