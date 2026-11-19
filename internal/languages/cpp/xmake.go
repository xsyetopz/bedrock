package cpp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/language"
)

// targetsScript prints the project's targets as JSON, with the ones
// `xmake test` runs, from the same `get_tests` xmake's test action and
// project plugin call. A test with `files` runs a clone of its target with
// those files added, and a binary target that declares tests is test code even
// when only its clones run, while a library that declares them stays product
// code. On-off options named for tests, such as `enable_tests`, are turned on
// first, since they often hide the test targets. Loading targets installs no
// packages.
const targetsScript = `import("core.base.json")
import("core.project.config")
import("core.project.project")
import("actions.test.main", {rootdir = os.programdir(), alias = "test_action"})

function main()
    config.load()
    for name, opt in pairs(project.options()) do
        local default = opt:get("default")
        local flag = opt:get("values") == nil and (default == nil or type(default) == "boolean")
        if flag and name:lower():find("test", 1, true) then
            config.set(name, true)
        end
    end
    local tested = {}
    for _, info in pairs(test_action.get_tests()) do
        tested[info.target:name()] = true
    end
    for _, target in ipairs(project.ordertargets()) do
        if target:kind() == "binary" and #table.wrap(target:get("tests")) > 0 then
            tested[target:name()] = true
        end
    end
    local targets = {}
    for _, target in ipairs(project.ordertargets()) do
        local sources = {}
        for _, file in ipairs(target:sourcefiles()) do
            table.insert(sources, path.absolute(file))
        end
        table.insert(targets, {
            id = target:name(),
            sources = sources,
            deps = table.wrap(target:get("deps")),
            test = tested[target:name()] == true,
            installed = target:is_default() and not tested[target:name()]
        })
    end
    print(json.encode(targets))
end
`

// xmakeProjects returns the xmake projects: each top `xmake.lua`'s directory.
func xmakeProjects(ctx context.Context, c *language.Context) []string {
	return projectsOf(ctx, c, "xmake.lua")
}

// xmakeEntries returns the compile_commands.json entries of each xmake
// project.
func xmakeEntries(ctx context.Context, c *language.Context) ([]map[string]any, error) {
	return eachProject(c, xmakeProjects(ctx, c), func(project string) ([]map[string]any, error) {
		output, err := os.MkdirTemp("", "bedrock-xmake-")
		if err != nil {
			return nil, fmt.Errorf("temporary directory: %w", err)
		}

		defer func() { _ = os.RemoveAll(output) }()

		result := run(ctx, c, c.Root, "xmake", "project", "-q", "-y", "-P", filepath.Join(c.Root, project),
			"-k", "compile_commands", output)
		if result.code != 0 {
			return nil, toolFailed("xmake project of %s failed: %s", displayName(project), firstLine(result.stderr))
		}

		return readEntries(filepath.Join(output, "compile_commands.json"))
	})
}

// xmakeTarget is a target as the script prints it.
type xmakeTarget struct {
	ID        string `json:"id"`
	Sources   strs   `json:"sources"`
	Deps      strs   `json:"deps"`
	Test      bool   `json:"test"`
	Installed bool   `json:"installed"`
}

// xmakeTargets returns the targets of the xmake project, or none when xmake
// cannot load it.
func xmakeTargets(ctx context.Context, c *language.Context, project string) ([]target, error) {
	dir, err := os.MkdirTemp("", "bedrock-xmake-")
	if err != nil {
		return nil, fmt.Errorf("temporary directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	script := filepath.Join(dir, "targets.lua")
	if err = os.WriteFile(script, []byte(targetsScript), 0o600); err != nil {
		return nil, fmt.Errorf("write `%s`: %w", script, err)
	}

	result := run(ctx, c, c.Root, "xmake", "lua", "-P", filepath.Join(c.Root, project), script)
	if result.code != 0 {
		return nil, nil
	}

	// xmake prints warnings, such as deprecations, around the JSON line.
	var line string

	for candidate := range strings.SplitSeq(result.stdout, "\n") {
		if strings.HasPrefix(candidate, "[") {
			line = candidate

			break
		}
	}

	var raw []xmakeTarget
	if line == "" {
		return nil, nil
	}

	if err = json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse the targets of xmake project `%s`: %w", displayName(project), err)
	}

	inRepo := repoPaths(c.Root)
	targets := make([]target, 0, len(raw))

	for _, item := range raw {
		out := target{id: item.ID, sources: nil, deps: item.Deps, test: item.Test, installed: item.Installed}

		for _, source := range item.Sources {
			if path, ok := inRepo(source); ok && !inBuildDirs(path) {
				out.sources = append(out.sources, path)
			}
		}

		targets = append(targets, out)
	}

	return targets, nil
}
