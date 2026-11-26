package swift

import (
	"context"
	"path"
	"strings"

	"github.com/xsyetopz/bedrock/internal/blocks"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// manifestBlocks returns the `Package.swift` blocks: the strict settings,
// and the C++ interop settings when cpp is adopted.
func manifestBlocks(c *language.Context, cxx bool) ([]blocks.Block, error) {
	strict, _, err := c.Dir(languageID, "manifest").Read("block.swift", nil)
	if err != nil {
		return nil, err
	}

	managed := []blocks.Block{{Name: swiftBlock, Content: strict}}
	if !cxx {
		return managed, nil
	}

	interop, _, err := c.Dir("cpp", "manifest").Read("block.swift", nil)
	if err != nil {
		return nil, err
	}

	return append(managed, blocks.Block{Name: cxxBlock, Content: interop}), nil
}

// xcconfig switches the C++ interop on and caching off in
// `Config/Bedrock.xcconfig` for C++ repositories.
func xcconfig(text string, cxx bool) string {
	if !cxx {
		return text
	}

	text = strings.Replace(text, "BEDROCK_CXX_FLAGS =\n", "BEDROCK_CXX_FLAGS = $(BEDROCK_CXX_INTEROP)\n", 1)

	return strings.Replace(text, "COMPILATION_CACHE_ENABLE_CACHING = YES\n",
		"COMPILATION_CACHE_ENABLE_CACHING = NO\n", 1)
}

// optionalSteps returns the files that apply only beside an Xcode project or
// a `Tests` folder.
func optionalSteps(c *language.Context, cxx, hasXcode, hasTests bool) ([]step.Step, error) {
	optional := c.Dir(languageID, "optional")

	wanted := []struct {
		path string
		on   bool
	}{
		{"Config/Bedrock.xcconfig", hasXcode},
		{"Tests/.swiftlint.yml", hasTests},
	}

	var steps []step.Step

	for _, file := range wanted {
		if !file.on {
			continue
		}

		text, found, err := optional.Read(file.path, nil)
		if err != nil {
			return nil, err
		}

		if found {
			content := xcconfig(text, cxx && strings.HasSuffix(file.path, ".xcconfig"))
			steps = append(steps, step.Write{Path: file.path, Content: []byte(content), Executable: false})
		}
	}

	return steps, nil
}

// isCxxSource reports whether the path is a C or C++ source or header.
func isCxxSource(file string) bool {
	switch path.Ext(file) {
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx", ".mm":
		return true
	default:
		return false
	}
}

// hasCFamilyTarget reports whether a SwiftPM target folder, `Sources/<name>`
// or `Tests/<name>`, holds C or C++ files and no Swift ones. SwiftPM builds no
// mixed-language target, so C++ files beside Swift ones are resources, such as
// a native project a library ships. Only a C-family target gives Swift a C++
// module to import, and C++ interop in a library breaks each client that lacks
// it.
func hasCFamilyTarget(ctx context.Context, c *language.Context) (bool, error) {
	files, err := fsutil.ListFiles(ctx, c.Runner, c.Root, func(file string) bool {
		return strings.HasPrefix(file, "Sources/") || strings.HasPrefix(file, "Tests/")
	})
	if err != nil {
		return false, err
	}

	cFamily := map[string]bool{}
	swiftTargets := map[string]bool{}

	for _, file := range files {
		// A file directly in `Sources` or `Tests` belongs to no target.
		folder, rest, _ := strings.Cut(file, "/")
		name, _, nested := strings.Cut(rest, "/")
		if !nested {
			continue
		}

		target := folder + "/" + name
		switch {
		case path.Ext(file) == ".swift":
			swiftTargets[target] = true
		case isCxxSource(file):
			cFamily[target] = true
		}
	}

	for target := range cFamily {
		if !swiftTargets[target] {
			return true, nil
		}
	}

	return false, nil
}

// cxxNote suggests adopting with `--cxx` when C or C++ files exist but cpp
// is not adopted.
func cxxNote(ctx context.Context, c *language.Context, cxx bool) ([]string, error) {
	if cxx {
		return nil, nil
	}

	files, err := fsutil.ListFiles(ctx, c.Runner, c.Root, isCxxSource)
	if err != nil || len(files) == 0 {
		return nil, err
	}

	return []string{"C or C++ files found; adopt with `--cxx` to add the C++ setup " +
		"(`clang-format`, `clang-tidy`, SwiftPM interop)."}, nil
}
