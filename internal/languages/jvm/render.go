package jvm

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/step"
)

// catalogPath is the version catalog every bedrock build shares.
const catalogPath = "gradle/libs.versions.toml"

// provided matches the libraries the bedrock plugins already add.
func provided() *regexp.Regexp {
	return regexp.MustCompile(
		`^(org\.jetbrains\.kotlin:kotlin-(stdlib|test)\S*|junit:junit|org\.testng:testng|org\.junit(\.\w+)*:\S+)$`)
}

type alias struct {
	name string
	lib  library
}

// aliases are the catalog entries the migrated build refers to as `libs.…`,
// in the order they were first needed.
type aliases struct {
	entries []alias
}

// of returns the alias for lib, reusing an entry with the same coordinates and
// version.
func (a *aliases) of(lib library) string {
	for _, known := range a.entries {
		if known.lib.coordinates() == lib.coordinates() && known.lib.version == lib.version {
			return known.name
		}
	}

	name := aliasOf(lib.name)
	if a.taken(name) {
		groupParts := strings.Split(lib.group, ".")
		name = aliasOf(groupParts[len(groupParts)-1] + "-" + lib.name)
	}

	unique := name
	for count := 2; a.taken(unique); count++ {
		unique = name + strconv.Itoa(count)
	}

	a.entries = append(a.entries, alias{name: unique, lib: lib})

	return unique
}

// taken reports whether the alias clashes with one in use: Gradle cannot make
// accessors for both foo and foo-bar.
func (a *aliases) taken(name string) bool {
	for _, known := range a.entries {
		key := known.name
		if key == name || strings.HasPrefix(key, name+"-") || strings.HasPrefix(name, key+"-") {
			return true
		}
	}

	return false
}

func accessor(name string) string { return "libs." + strings.ReplaceAll(name, "-", ".") }

func (a *aliases) notation(dep dependency) string {
	switch dep.kind {
	case libraryKind:
		return accessor(a.of(dep.lib))
	case platformKind:
		return "platform(" + accessor(a.of(dep.lib)) + ")"
	case moduleKind:
		return `project("` + dep.path + `")`
	case jarsKind:
		quotedPaths := make([]string, len(dep.paths))
		for index, jar := range dep.paths {
			quotedPaths[index] = `"` + jar + `"`
		}

		return "files(" + strings.Join(quotedPaths, ", ") + ")"
	case notationKind:
		return dep.text
	default:
		return ""
	}
}

func (a *aliases) dependencyLines(made *module, notes *noteSet) []string {
	skip := provided()

	var lines []string

	for _, dep := range made.dependencies {
		if dep.kind == libraryKind {
			coordinates := dep.lib.coordinates()
			if coordinates == "junit:junit" || coordinates == "org.testng:testng" {
				notes.add("`" + gradlePath(made.directory) + "`: tests used `" + coordinates +
					"`; bedrock runs JUnit 6, so port them to `org.junit.jupiter.api`, which `just guard` checks")
			}

			if skip.MatchString(coordinates) {
				continue
			}
		}

		line := "  " + dep.configuration + "(" + a.notation(dep) + ")"
		if !slices.Contains(lines, line) {
			lines = append(lines, line)
		}
	}

	return lines
}

func scriptOf(made *module, lines []string) string {
	plugins := []string{"bedrock.java"}
	if made.kotlin {
		plugins = []string{"bedrock.kotlin"}
	}

	if made.mainClass != "" {
		plugins = append(plugins, "bedrock.application")
	}

	head := `plugins { id("` + plugins[0] + `") }` + "\n"

	if len(plugins) > 1 {
		ids := make([]string, len(plugins))
		for index, id := range plugins {
			ids[index] = `  id("` + id + `")`
		}

		head = "plugins {\n" + strings.Join(ids, "\n") + "\n}\n"
	}

	application := ""
	if made.mainClass != "" {
		application = "\napplication { mainClass = \"" + made.mainClass + "\" }\n"
	}

	dependencies := ""
	if len(lines) > 0 {
		dependencies = "\ndependencies {\n" + strings.Join(lines, "\n") + "\n}\n"
	}

	return head + application + dependencies
}

func (a *aliases) catalogStep(root string) ([]step.Step, error) {
	if len(a.entries) == 0 {
		return nil, nil
	}

	catalog, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(catalogPath)))
	if err != nil {
		return nil, err
	}

	lines := make([]string, len(a.entries))

	for index, entry := range a.entries {
		version := ""
		if entry.lib.version != "" {
			version = `, version = "` + entry.lib.version + `"`
		}

		lines[index] = entry.name + ` = { module = "` + entry.lib.coordinates() + `"` + version + " }"
	}

	entries := strings.Join(lines, "\n")
	content := "[libraries]\n" + entries + "\n"

	if found {
		content = strings.TrimRight(catalog, " \t\r\n") + "\n\n# Added by bedrock adopt.\n" + entries + "\n"
	}

	return []step.Step{step.Write{Path: catalogPath, Content: []byte(content), Executable: false}}, nil
}

// render returns the steps that write a migration's build files, with each
// library in `gradle/libs.versions.toml`.
func render(root string, m *migration) ([]step.Step, error) {
	known := &aliases{entries: nil}
	steps := slices.Clone(m.steps)

	for _, made := range m.modules {
		if made.kept {
			continue
		}

		lines := known.dependencyLines(made, m.notes)
		file := fileBuildGradleKts

		if made.directory != "" {
			file = made.directory + "/build.gradle.kts"
		}

		steps = append(steps, step.Write{Path: file, Content: []byte(scriptOf(made, lines)), Executable: false})
	}

	catalog, err := known.catalogStep(root)
	if err != nil {
		return nil, err
	}

	return append(steps, catalog...), nil
}

// includes returns the settings line that includes each module below the
// root, or "".
func includes(m *migration) string {
	var paths []string

	for _, made := range m.modules {
		if made.directory != "" {
			paths = append(paths, `"`+gradlePath(made.directory)+`"`)
		}
	}

	if len(paths) == 0 {
		return ""
	}

	return "include(" + strings.Join(paths, ", ") + ")\n"
}
