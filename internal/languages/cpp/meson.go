package cpp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// mesonProjects returns the Meson projects: each top `meson.build`'s
// directory.
func mesonProjects(ctx context.Context, c *language.Context) []string {
	return projectsOf(ctx, c, "meson.build")
}

// configureMeson sets project up in its bedrock build directory, or
// reconfigures the one there, which writes compile_commands.json and the
// introspection data.
func configureMeson(ctx context.Context, c *language.Context, project string) (string, error) {
	build := buildDir(c.Root, "meson", project)
	args := []string{"setup"}

	if fsutil.Exists(filepath.Join(build, "meson-private")) {
		args = append(args, "--reconfigure")
	}

	args = append(args, build, filepath.Join(c.Root, project))

	result := run(ctx, c, c.Root, "meson", args...)
	if result.code != 0 {
		return "", toolFailed(
			"meson setup of %s failed: %s",
			displayName(project),
			firstLine(result.stdout+result.stderr),
		)
	}

	return build, nil
}

// mesonEntries returns the compile_commands.json entries of each Meson
// project, configured afresh.
func mesonEntries(ctx context.Context, c *language.Context) ([]map[string]any, error) {
	return eachProject(c, mesonProjects(ctx, c), func(project string) ([]map[string]any, error) {
		build, err := configureMeson(ctx, c, project)
		if err != nil {
			return nil, err
		}

		return readEntries(filepath.Join(build, "compile_commands.json"))
	})
}

// mesonTarget is a target as `meson introspect --targets` reports it.
type mesonTarget struct {
	ID            string `json:"id"`
	Filename      strs   `json:"filename"`
	Depends       strs   `json:"depends"`
	TargetSources []struct {
		Linker     json.RawMessage `json:"linker"`
		Parameters strs            `json:"parameters"`
		Sources    strs            `json:"sources"`
	} `json:"target_sources"`
	Installed bool `json:"installed"`
}

// mesonTest is a test as `meson introspect --tests` reports it.
type mesonTest struct {
	Cmd     strs `json:"cmd"`
	Depends strs `json:"depends"`
}

// introspect decodes one `meson introspect` list, empty when the call fails.
func introspect[T any](ctx context.Context, c *language.Context, build, flag string) []T {
	result := run(ctx, c, build, "meson", "introspect", build, flag)
	if result.code != 0 {
		return nil
	}

	var items []T
	if json.Unmarshal([]byte(result.stdout), &items) != nil {
		return nil
	}

	return items
}

// mesonBuild is what an introspected target is read against.
type mesonBuild struct {
	inRepo func(string) (string, bool)
	// owners maps each built file, as fileOf names it, to the target that builds it.
	owners map[string]string
	// tested are the ids of the targets the tests run.
	tested map[string]bool
	dir    string
}

// fileOf returns a path Meson printed, relative to the root when inside it.
func (b mesonBuild) fileOf(path string) string {
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(b.dir, absolute)
	}

	if rel, ok := b.inRepo(absolute); ok {
		return rel
	}

	return path
}

// testedTargets returns the targets the tests run: the one whose file a
// test's command is, or, when a test runs a script, the uninstalled targets
// it depends on.
func (b mesonBuild) testedTargets(tests []mesonTest, targets []mesonTarget) map[string]bool {
	installed := map[string]bool{}

	for _, t := range targets {
		if t.Installed {
			installed[t.ID] = true
		}
	}

	tested := map[string]bool{}

	for _, test := range tests {
		first := ""
		if len(test.Cmd) > 0 {
			first = test.Cmd[0]
		}

		if runs, ok := b.owners[b.fileOf(first)]; ok {
			tested[runs] = true

			continue
		}

		for _, id := range test.Depends {
			if !installed[id] {
				tested[id] = true
			}
		}
	}

	return tested
}

// target converts an introspected target. Meson's `depends` leaves out the
// libraries a target links, so those come from the linker arguments, which
// name each by its file in the build directory.
func (b mesonBuild) target(raw mesonTarget) target {
	out := target{
		id:        raw.ID,
		sources:   nil,
		deps:      append([]string(nil), raw.Depends...),
		test:      b.tested[raw.ID],
		installed: raw.Installed,
	}

	for _, group := range raw.TargetSources {
		if len(group.Linker) > 0 {
			for _, argument := range group.Parameters {
				owner, ok := b.owners[b.fileOf(argument)]
				if ok && owner != raw.ID && !slices.Contains(out.deps, owner) {
					out.deps = append(out.deps, owner)
				}
			}
		}

		for _, source := range group.Sources {
			if path, ok := b.inRepo(source); ok && !inBuildDirs(path) {
				out.sources = append(out.sources, path)
			}
		}
	}

	return out
}

// mesonTargets returns the targets of the configured build of project, from
// `meson introspect`. A target whose file a test runs is a test target,
// installed or not, since installed tests are tests.
func mesonTargets(ctx context.Context, c *language.Context, project string) []target {
	dir := buildDir(c.Root, "meson", project)
	if !fsutil.Exists(filepath.Join(dir, "meson-private")) {
		return nil
	}

	targets := introspect[mesonTarget](ctx, c, dir, "--targets")
	tests := introspect[mesonTest](ctx, c, dir, "--tests")
	build := mesonBuild{inRepo: repoPaths(c.Root), owners: map[string]string{}, tested: nil, dir: dir}

	for _, t := range targets {
		for _, file := range t.Filename {
			build.owners[build.fileOf(file)] = t.ID
		}
	}

	build.tested = build.testedTargets(tests, targets)

	out := make([]target, 0, len(targets))
	for _, t := range targets {
		out = append(out, build.target(t))
	}

	return out
}
