// Package cpp is the C and C++ brick: clang-format, clang-tidy, and
// compile_commands.json from each build system the repository uses, alone or
// beside Swift. The swift brick writes the C++ blocks of `Package.swift` and
// the starter files, since both change the same files as its own.
package cpp

import (
	"context"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// id is the language's ID and its first alias.
const id = "cpp"

// oldConfigs were renamed to the dotted names the tools read.
func oldConfigs() []string { return []string{"_clang-format", "_clang-tidy"} }

// Cpp is the C and C++ language.
type Cpp struct{}

// New returns the C and C++ language.
func New() Cpp { return Cpp{} }

// Spec declares the language's templates, markers, and sources.
func (Cpp) Spec() language.Spec {
	markers := []string{".just/cpp.just", ".clang-format", ".clang-tidy"}
	markers = append(markers, oldConfigs()...)
	markers = append(markers,
		"CMakeLists.txt", "meson.build", "xmake.lua",
		// Sources at the top two levels, such as micropython's py/*.c, mark a
		// Make-built repository; deeper ones may be vendored, such as node_modules.
		"*.{c,cc,cpp,cxx}",
		"*/*.{c,cc,cpp,cxx}",
		// SwiftPM builds C, C++, and Objective-C targets from the same tree.
		"Sources/**/*.{c,cc,cpp,cxx,m,mm}",
	)

	return language.Spec{
		ID: id,
		// The block name predates C++ without Swift; adopted repositories carry it.
		Block:       "swift-bedrock-cxx",
		Aliases:     []string{id, "cxx", "c++", "c"},
		Markers:     markers,
		Tools:       nil,
		Flags:       nil,
		Sources:     []string{"**/*.{c,cc,cpp,cxx,h,hh,hpp,hxx,m,mm}", ".clang-format", ".clang-tidy"},
		Kinds:       nil,
		Executables: nil,
		// clang-tidy checks function size and complexity but not file length.
		Lizard: []string{"cpp", "objectivec"},
	}
}

// Plan removes the underscore-named configs the dotted ones replaced.
func (Cpp) Plan(_ context.Context, c *language.Context) (language.Contribution, error) {
	var steps []step.Step
	for _, path := range fsutil.Existing(c.Root, oldConfigs()) {
		steps = append(steps, step.Remove{Path: path})
	}

	return language.Contribution{Steps: steps, Notes: nil, ProjectFiles: nil}, nil
}
