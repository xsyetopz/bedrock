package jvm

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// conventionDir holds the precompiled script plugins a build applies as
// `bedrock.*`; the Kotlin helpers in its `bedrock/` package serve them all.
const conventionDir = "gradle/bedrock/src/main/kotlin/"

// pluginID matches a quoted `bedrock.*` plugin id, as `id("bedrock.kotlin")`
// and `apply(plugin = "bedrock.kmp-android")` name it.
func pluginID() *regexp.Regexp { return regexp.MustCompile(`["'](bedrock\.[a-z][a-z-]*)["']`) }

// pluginFile matches a root `gradle/bedrock*` file a plugin reads, such as
// the ProGuard rules of `bedrock.application`.
func pluginFile() *regexp.Regexp { return regexp.MustCompile(`["'](gradle/bedrock[^"'/]*)["']`) }

// conventionPlugin is one managed precompiled script plugin.
type conventionPlugin struct {
	path string
	text string
}

// conventionPlugins returns the managed plugins by id: the file name without
// `.gradle.kts`, and without the `.settings` that marks a settings plugin.
func conventionPlugins(dir template.Dir, paths []string) (map[string]conventionPlugin, error) {
	plugins := map[string]conventionPlugin{}

	for _, target := range paths {
		name, ok := strings.CutPrefix(target, conventionDir)
		if !ok || strings.Contains(name, "/") || !strings.HasSuffix(name, ".gradle.kts") {
			continue
		}

		text, _, readErr := dir.Read(target, template.Variables{})
		if readErr != nil {
			return nil, readErr
		}

		id := strings.TrimSuffix(strings.TrimSuffix(name, ".gradle.kts"), ".settings")
		plugins[id] = conventionPlugin{path: target, text: text}
	}

	return plugins, nil
}

// buildSource reports whether a file can apply a convention plugin: a Gradle
// script, or a Kotlin or Java source of a `buildSrc` or `build-logic` build.
func buildSource(file string) bool {
	if strings.HasSuffix(file, ".gradle.kts") || strings.HasSuffix(file, ".gradle") {
		return true
	}

	top, _, _ := strings.Cut(file, "/")

	return (top == "buildSrc" || top == "build-logic") &&
		(strings.HasSuffix(file, ".kt") || strings.HasSuffix(file, ".java"))
}

// buildSources returns the text of every file under root that can apply a
// convention plugin, the managed `gradle/bedrock` build left out, with the
// planned writes in place of what is on disk.
func buildSources(root string, planned []step.Step) (map[string]string, error) {
	texts := map[string]string{}

	// Reading through the root keeps a symlink swapped in mid-walk from
	// pointing a read outside the repository.
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}

	defer func() { _ = scoped.Close() }()

	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(root, name)
		rel = filepath.ToSlash(rel)

		if entry.IsDir() {
			if rel != "." && (ignoredDirectory(entry.Name()) || rel == "gradle/bedrock") {
				return filepath.SkipDir
			}

			return nil
		}

		if !buildSource(rel) {
			return nil
		}

		data, readErr := scoped.ReadFile(rel)
		if readErr != nil {
			return readErr
		}

		texts[rel] = string(data)

		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, each := range planned {
		if written, ok := each.(step.Write); ok && buildSource(written.Path) &&
			!strings.HasPrefix(written.Path, "gradle/bedrock/") {
			texts[written.Path] = string(written.Content)
		}
	}

	return texts, nil
}

// unusedConventions returns the managed plugins no build source applies,
// directly or through another plugin, and the `gradle/bedrock*` files only
// those plugins read. A plugin a build later applies comes back with the
// next update, and the drift check reports it missing until then.
func unusedConventions(c *language.Context, planned []step.Step) ([]string, error) {
	dir := c.Dir("jvm", "files")

	paths, err := dir.Paths()
	if err != nil {
		return nil, err
	}

	plugins, err := conventionPlugins(dir, paths)
	if err != nil {
		return nil, err
	}

	sources, err := buildSources(c.Root, planned)
	if err != nil {
		return nil, err
	}

	return unusedFiles(plugins, appliedPlugins(plugins, sources), paths), nil
}

// appliedPlugins returns the ids of the plugins the sources apply, and of
// those the applied plugins apply in turn.
func appliedPlugins(plugins map[string]conventionPlugin, sources map[string]string) map[string]bool {
	ids := pluginID()
	used := map[string]bool{}

	var queue []string

	mark := func(text string) {
		for _, match := range ids.FindAllStringSubmatch(text, -1) {
			if _, known := plugins[match[1]]; known && !used[match[1]] {
				used[match[1]] = true
				queue = append(queue, match[1])
			}
		}
	}

	for _, file := range slices.Sorted(maps.Keys(sources)) {
		mark(sources[file])
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		mark(plugins[id].text)
	}

	return used
}

// unusedFiles returns the paths of the plugins not used, then of the managed
// `gradle/bedrock*` files no used plugin reads.
func unusedFiles(plugins map[string]conventionPlugin, used map[string]bool, paths []string) []string {
	files := pluginFile()
	read := map[string]bool{}
	readByUsed := map[string]bool{}

	var unused []string

	for _, id := range slices.Sorted(maps.Keys(plugins)) {
		plugin := plugins[id]
		if !used[id] {
			unused = append(unused, plugin.path)
		}

		for _, match := range files.FindAllStringSubmatch(plugin.text, -1) {
			read[match[1]] = true
			readByUsed[match[1]] = readByUsed[match[1]] || used[id]
		}
	}

	for _, file := range slices.Sorted(maps.Keys(read)) {
		if !readByUsed[file] && slices.Contains(paths, file) {
			unused = append(unused, file)
		}
	}

	return unused
}
