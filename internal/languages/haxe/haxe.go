// Package haxe is the Haxe brick: haxe-formatter, haxe-checkstyle with a
// config that bounds file and function size, a type check of each hxml
// build, and the test hxml files that run their tests. It works alone or
// beside other languages.
package haxe

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

// program is the language id, its alias, and the compiler's executable.
const program = "haxe"

// Haxe is the Haxe language.
type Haxe struct{}

// New returns the Haxe language.
func New() Haxe { return Haxe{} }

// Spec declares Haxe's templates, markers, and tools.
//
// A Haxe compile target, such as js, jvm, or hl, is an output runtime, not a
// platform, so the `library` kind ships one test hxml per target and takes no
// platform. Only hxcpp turns Haxe into a native binary for the OS it builds
// on, so `command` builds for linux, macos, and windows.
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
		Kinds:   []string{"command", "library"},
		Platforms: map[string][]string{
			"command": {"linux", "macos", "windows"},
		},
	}
}

// Variables fill the Haxe package name: the repository name in lower case
// letters and digits, which the starter code and the native binary use.
func (Haxe) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	return template.Variables{"package": packageOf(c.Root)}, nil
}

// packageOf returns a Haxe package name for the directory at root: lower case
// letters and digits, led by a letter, since a Haxe package starts lower case.
func packageOf(root string) string {
	word := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return -1
		}
	}, filepath.Base(root))

	if word == "" || word[0] < 'a' {
		return "p" + word
	}

	return word
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
