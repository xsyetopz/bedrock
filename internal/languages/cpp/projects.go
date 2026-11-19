package cpp

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// buildRoot is where bedrock configures each build, relative to the root;
// the cpp gitignore block lists it.
const buildRoot = ".cache/bedrock"

// topProjects returns the projects whose build file no other tracked one of
// the same name encloses; "" is the root.
func topProjects(lists []string) []string {
	projects := make([]string, 0, len(lists))

	for _, list := range lists {
		dir := path.Dir(list)
		if dir == "." {
			dir = ""
		}

		projects = append(projects, dir)
	}

	var top []string

	for _, project := range projects {
		enclosed := false

		for _, other := range projects {
			if other != project && (other == "" || strings.HasPrefix(project, other+"/")) {
				enclosed = true
			}
		}

		if !enclosed {
			top = append(top, project)
		}
	}

	return top
}

// projectsOf returns the top projects of the build file name, such as
// `CMakeLists.txt`: the directories, relative to root and "" for the root,
// whose file git tracks and no enclosing project holds, since the enclosing
// one includes it.
func projectsOf(ctx context.Context, c *language.Context, name string) []string {
	result, err := c.Runner.Run(ctx, execx.Command{
		Dir:  c.Root,
		Name: "git",
		Args: []string{"ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", name, "*/" + name},
		// A hook's git variables would list the hook's repository, not root.
		Unset: execx.HookGitVars(),
	})
	// A failed git listing means no projects, as outside a work tree.
	if err != nil {
		return nil
	}

	var lists []string

	for entry := range strings.SplitSeq(string(result.Stdout), "\x00") {
		// A build directory holds fetched dependencies' build files, such as
		// CMake's _deps, until the gitignore block lists it.
		if entry != "" && !inBuildDirs(entry) {
			lists = append(lists, entry)
		}
	}

	return topProjects(lists)
}

// eachProject returns the items each gives for every project, run in
// parallel. A subproject that fails, such as a starter that needs an SDK
// fetched first, is skipped with a warning, while the root project's failure
// stands, since it is the repository's own build.
func eachProject[T any](c *language.Context, projects []string, each func(project string) ([]T, error)) ([]T, error) {
	lists := make([][]T, len(projects))
	errs := make([]error, len(projects))

	var group sync.WaitGroup

	for i, project := range projects {
		group.Go(func() { lists[i], errs[i] = each(project) })
	}

	group.Wait()

	var items []T

	for i, project := range projects {
		if errs[i] == nil {
			items = append(items, lists[i]...)

			continue
		}

		if project == "" {
			return nil, errs[i]
		}

		_, _ = fmt.Fprintf(c.Stderr, "skipped %s: %v\n", project, errs[i])
	}

	return items, nil
}

// inBuildDirs reports whether target, relative to the root, is in a build
// directory bedrock configured, as generated sources are.
func inBuildDirs(target string) bool { return strings.HasPrefix(target, buildRoot+"/") }

// buildDir is where bedrock configures project, under `.cache/bedrock`.
// `bedrock cpp compile-commands` configures it, and the test recipe reads it.
func buildDir(root, system, project string) string {
	// "+" names the root, so no subproject's name can collide with it.
	name := "+"
	if project != "" {
		name = strings.ReplaceAll(project, "/", "+")
	}

	return filepath.Join(root, filepath.FromSlash(buildRoot), system, name)
}

// displayName names project in a message; "." is the root.
func displayName(project string) string {
	if project == "" {
		return "."
	}

	return project
}

// repoPaths returns a function from a path a build tool printed to its path
// relative to root, or false outside it, such as a generated source in the
// build directory. Tools print root as given or resolved, as /private/tmp for
// /tmp.
func repoPaths(root string) func(target string) (string, bool) {
	base := root
	if abs, err := filepath.Abs(root); err == nil {
		base = abs
	}

	bases := []string{base}
	if resolved, err := filepath.EvalSymlinks(base); err == nil && resolved != base {
		bases = append(bases, resolved)
	}

	return func(target string) (string, bool) {
		for _, from := range bases {
			absolute := target
			if !filepath.IsAbs(absolute) {
				absolute = filepath.Join(from, absolute)
			}

			rel, err := filepath.Rel(from, absolute)
			if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue
			}

			return filepath.ToSlash(rel), true
		}

		return "", false
	}
}
