// Package dotnet is the .NET brick: C# and F# on .NET 10, alone or beside
// other languages. CSharpier and Fantomas lay the code out, the SDK analyzers
// with the `.editorconfig` code style and FSharpLint lint it, a build of each
// solution or project type-checks it, and `dotnet test` runs the tests.
package dotnet

import "github.com/xsyetopz/bedrock/internal/language"

const (
	// id names the language and its SDK command.
	id = "dotnet"
	// csharpier and fantomas are the formatter tools and their commands.
	csharpier = "csharpier"
	fantomas  = "fantomas"
	// formatName and testName are subcommand names the recipes call.
	formatName = "format"
	testName   = "test"
)

// Dotnet is the .NET language.
type Dotnet struct{}

// New returns the .NET language.
func New() Dotnet { return Dotnet{} }

// Spec declares .NET's templates, markers, and tools.
func (Dotnet) Spec() language.Spec {
	return language.Spec{
		ID:      id,
		Block:   "dotnet-bedrock",
		Aliases: []string{".net", "csharp", "c#", "cs", "fsharp", "f#"},
		Markers: []string{".just/dotnet.just", "global.json", "*.{sln,slnx}", "**/*.{csproj,fsproj}"},
		Tools: []language.Tool{{
			Name:    id,
			Install: "the .NET 10 SDK from https://dotnet.microsoft.com/download, such as `brew install dotnet@10`",
		}},
		Sources: []string{
			"**/*.{cs,csx,fs,fsi,fsx}",
			"**/*.{csproj,fsproj,props,targets}",
			"*.{sln,slnx}",
			"global.json",
			".config/dotnet-tools.json",
			"dotnet-tools.json",
			".editorconfig",
		},
		Kinds: nil,
		// The SDK analyzers and FSharpLint, as configured, bound no file,
		// function, complexity, or parameter size, so lizard measures both
		// languages.
		Lizard: []string{"csharp", "fsharp"},
	}
}
