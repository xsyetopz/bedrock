package cpp

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/template"
)

// kinds are the CMake starters `bedrock init cpp:<kind>` writes. CMake is the
// only build system with presets, toolchain files for every platform, and
// generators for Visual Studio and Xcode, so the kinds use it alone; Meson and
// xmake projects are still adopted.
func kinds() []string { return []string{"executable", "header-only", "library"} }

// platforms maps each kind to the targets `@<platform>` may name. A library
// builds for every platform; an executable only for those that run a command
// line program, since iOS, tvOS, watchOS, visionOS, and Mac Catalyst launch
// app bundles, which those platforms' own app starters write.
func platforms() map[string][]string {
	programs := []string{"android", "freebsd", "linux", "macos", "wasi", "wasm", "windows"}
	everywhere := slices.Concat(programs, []string{"ios", "maccatalyst", "tvos", "visionos", "watchos"})
	slices.Sort(everywhere)

	return map[string][]string{
		"executable":  programs,
		"header-only": everywhere,
		"library":     everywhere,
	}
}

// Variables fill the target name, the macro prefix, and the presets a new
// project includes. The target name is the directory's name as a C++
// identifier, since it is also the namespace and the include directory.
func (Cpp) Variables(_ context.Context, c *language.Context) (template.Variables, error) {
	selected, err := config.SplitList(c.Options["platform"])
	if err != nil {
		return nil, fmt.Errorf("platform: %w", err)
	}

	includes := make([]string, 0, len(selected))
	for _, platform := range slices.Sorted(slices.Values(selected)) {
		includes = append(includes, `"cmake/presets/`+platform+`.json"`)
	}

	project := identifier(filepath.Base(c.Root))

	return template.Variables{
		"project":        project,
		"macro":          strings.ToUpper(project),
		"presetIncludes": strings.Join(includes, ",\n    "),
	}, nil
}

// identifier makes a lower-case C++ identifier from a directory name, such as
// my-app to my_app; one that starts with a digit gets a p prefix.
func identifier(name string) string {
	word := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r == '-' || r == '.' || r == ' ':
			return '_'
		default:
			return -1
		}
	}, name)

	if word == "" || word[0] >= '0' && word[0] <= '9' {
		return "p" + word
	}

	return word
}
