package jvm

import (
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/step"
)

// dependencyKind says which Gradle notation a dependency renders to.
type dependencyKind int

const (
	// libraryKind is Maven coordinates, rendered as a catalog accessor.
	libraryKind dependencyKind = iota
	// platformKind is a BOM or parent, rendered as platform(accessor).
	platformKind
	// moduleKind is another Gradle project in the build.
	moduleKind
	// jarsKind is local jars, rendered as files(...).
	jarsKind
	// notationKind is Gradle notation kept as written.
	notationKind
)

// library is Maven coordinates; the version is empty when a BOM or parent
// supplies it.
type library struct {
	group   string
	name    string
	version string
}

// coordinates returns group:name, which a catalog entry's module names.
func (l library) coordinates() string { return l.group + ":" + l.name }

// dependency is a dependency in Gradle terms.
type dependency struct {
	configuration string
	// text is notation kept as written, for notationKind.
	text string
	// path is the Gradle path of a moduleKind dependency, such as :core.
	path string
	// paths are the jars of a jarsKind dependency.
	paths []string
	lib   library
	kind  dependencyKind
}

// module is one Gradle project the old build described, at directory
// relative to the root ("" for it).
type module struct {
	directory    string
	mainClass    string
	dependencies []dependency
	kotlin       bool
	// kept marks a build script that applies bedrock already, so render
	// leaves it as it is.
	kept bool
}

// noteSet keeps notes in the order they were added, each once.
type noteSet struct {
	seen map[string]bool
	list []string
}

func newNoteSet() *noteSet { return &noteSet{seen: map[string]bool{}, list: nil} }

func (n *noteSet) add(note string) {
	if n.seen[note] {
		return
	}

	n.seen[note] = true
	n.list = append(n.list, note)
}

// migration is what a build tool's files turn into, and what adopt could not
// carry over.
type migration struct {
	notes   *noteSet
	tool    string
	group   string
	modules []*module
	steps   []step.Step
}

func newMigration(tool string) *migration {
	return &migration{tool: tool, notes: newNoteSet(), group: "", modules: nil, steps: nil}
}

func newModule(directory string) *module {
	return &module{directory: directory, mainClass: "", dependencies: nil, kotlin: false, kept: false}
}

func (m *migration) remove(target string) {
	m.steps = append(m.steps, step.Remove{Path: target})
}

// gradlePath returns the Gradle path, such as :core, for a module below the
// root.
func gradlePath(directory string) string {
	return ":" + strings.ReplaceAll(directory, "/", ":")
}

// maxBytecodeLevel is the Java level bedrock compiles main code to.
const maxBytecodeLevel = 8

// javaLevelNote adds a note when a build compiles for a Java level above the
// Java 8 bytecode bedrock targets.
func javaLevelNote(file, level string, notes *noteSet) {
	level = strings.TrimPrefix(strings.TrimSpace(level), "1.")

	number, err := strconv.ParseFloat(level, 64)
	if err != nil || math.IsNaN(number) {
		return
	}

	if whole := int(math.Trunc(number)); whole > maxBytecodeLevel {
		notes.add("`" + file + "`: compiled for Java " + strconv.Itoa(whole) +
			"; bedrock compiles main code to Java 8, so the compilers reject newer APIs and syntax")
	}
}

// ignoredDirectory is a directory of build output or dependencies, not
// sources.
func ignoredDirectory(name string) bool {
	return name == "build" || name == "target" || name == "node_modules" || strings.HasPrefix(name, ".")
}

// foreignSourceSet reports whether a directory named name inside directory is
// a Scala or Groovy source set no bedrock plugin compiles. A module that
// applies `bedrock.scala` compiles its Scala.
func foreignSourceSet(root, name, directory string, parts []string) bool {
	if (name != nameScala && name != "groovy") || len(parts) < 2 || parts[len(parts)-2] != "src" {
		return false
	}

	if name == nameScala {
		module := path.Dir(path.Dir(directory))
		buildFile := filepath.Join(root, filepath.FromSlash(path.Join(module, fileBuildGradleKts)))

		script, _, err := fsutil.ReadText(buildFile)
		if err == nil && strings.Contains(script, `"bedrock.scala"`) {
			return false
		}
	}

	return true
}

// foreignSources returns the Scala and Groovy source directories below
// directory, which the bedrock plugins do not compile.
func foreignSources(root, directory string) []string {
	entries, err := os.ReadDir(path.Join(root, directory))
	if err != nil {
		return nil
	}

	var found []string

	for _, entry := range entries {
		if !entry.IsDir() || ignoredDirectory(entry.Name()) {
			continue
		}

		child := path.Join(directory, entry.Name())

		if foreignSourceSet(root, entry.Name(), directory, strings.Split(directory, "/")) {
			found = append(found, child)
		}

		found = append(found, foreignSources(root, child)...)
	}

	return found
}

func nonAlphanumeric() *regexp.Regexp { return regexp.MustCompile(`[^a-z0-9]+`) }

// aliasOf returns a catalog alias Gradle can make accessors for: lowercase
// segments joined by -, each starting with a letter, so log4j-1.2-api
// becomes log4j12-api.
func aliasOf(name string) string {
	var segments []string

	for _, part := range nonAlphanumeric().Split(strings.ToLower(name), -1) {
		if part == "" {
			continue
		}

		last := len(segments) - 1
		if last >= 0 && part[0] >= '0' && part[0] <= '9' {
			segments[last] += part
		} else {
			segments = append(segments, part)
		}
	}

	joined := strings.Join(segments, "-")
	if joined == "" {
		joined = "library"
	}

	reserved := len(segments) > 0 && slices.Contains([]string{"bundles", "versions", "plugins"}, segments[0])
	if joined[0] < 'a' || joined[0] > 'z' || reserved {
		return "lib-" + joined
	}

	return joined
}
