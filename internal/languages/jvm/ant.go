package jvm

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/step"
)

// filesBelow returns the files below directory as slash paths relative to it.
func filesBelow(directory string) []string {
	var found []string

	walkErr := filepath.WalkDir(directory, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // a source folder that is missing or unreadable has no files to move
		}

		relative, relErr := filepath.Rel(directory, file)
		if relErr == nil {
			found = append(found, filepath.ToSlash(relative))
		}

		return nil
	})
	if walkErr != nil {
		return nil
	}

	return found
}

// moveSteps moves a file as a write and a removal. A file that is not UTF-8
// text is left, because the plan stores text.
func moveSteps(root, from, to string) []step.Step {
	bytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from)))
	if err != nil || !utf8.Valid(bytes) {
		return nil
	}

	return []step.Step{step.Write{Path: to, Content: bytes, Executable: false}, step.Remove{Path: from}}
}

// antBuild is what adopt reads from build.xml.
type antBuild struct {
	properties map[string]string
}

func (b antBuild) resolve(value string) string {
	pattern := propertyReference()

	return pattern.ReplaceAllStringFunc(value, func(whole string) string {
		if found, ok := b.properties[pattern.FindStringSubmatch(whole)[1]]; ok {
			return found
		}

		return whole
	})
}

// sourceDirectories returns the folders the javac tasks compile, in the order
// the build names them.
func (b antBuild) sourceDirectories(project *element, m *migration) []string {
	var sources []string

	add := func(source string) {
		if !slices.Contains(sources, source) {
			sources = append(sources, source)
		}
	}

	for _, javac := range project.descendants("javac") {
		if srcdir, ok := javac.attributes["srcdir"]; ok {
			for _, part := range regexp.MustCompile(`[:,]`).Split(b.resolve(srcdir), -1) {
				add(part)
			}
		}

		for _, src := range javac.descendants("src") {
			if value, ok := src.attributes["path"]; ok {
				add(b.resolve(value))
			}
		}

		for _, name := range []string{"release", "target", "source"} {
			if value, ok := javac.attributes[name]; ok {
				javaLevelNote("build.xml", b.resolve(value), m.notes)

				break
			}
		}
	}

	return sources
}

func (b antBuild) moveSources(root string, sources []string, m *migration) {
	for _, source := range sources {
		source = strings.Trim(strings.TrimSpace(source), "/")
		if source == "" {
			continue
		}

		set := "main"
		if strings.Contains(source, nameTest) {
			set = nameTest
		}

		if source == "src/"+set+"/java" {
			continue
		}

		for _, relative := range filesBelow(filepath.Join(root, filepath.FromSlash(source))) {
			kind := "resources"
			if strings.HasSuffix(relative, ".java") {
				kind = nameJava
			}

			target := path.Join("src", set, kind, relative)

			moved := moveSteps(root, source+"/"+relative, target)
			if moved == nil {
				m.notes.add("`" + source + "/" + relative + "`: not text, so adopt left it; move it to `" +
					target + "` by hand")
			}

			m.steps = append(m.steps, moved...)
		}
	}
}

func readBuild(root string, made *module, m *migration) error {
	text, found, err := fsutil.ReadText(filepath.Join(root, "build.xml"))
	if err != nil || !found {
		return err
	}

	project, err := parseXML(text)
	if err != nil {
		return fmt.Errorf("`build.xml`: %w", err)
	}

	build := antBuild{properties: map[string]string{}}

	for _, property := range project.descendants("property") {
		name, hasName := property.attributes["name"]
		value, hasValue := property.attributes["value"]

		if hasName && hasValue {
			build.properties[name] = value
		}
	}

	for _, attribute := range project.descendants("attribute") {
		if attribute.attributes["name"] == "Main-Class" {
			made.mainClass = build.resolve(attribute.attributes["value"])

			break
		}
	}

	build.moveSources(root, build.sourceDirectories(project, m), m)
	m.remove("build.xml")

	return nil
}

// ivyConfiguration maps an Ivy conf to a Gradle configuration.
func ivyConfiguration(conf string) string {
	switch {
	case strings.Contains(conf, nameTest):
		return confTestImplementation
	case strings.Contains(conf, "provided"):
		return confCompileOnly
	case strings.HasPrefix(conf, nameRuntime):
		return confRuntimeOnly
	default:
		return confImplementation
	}
}

func readIvy(root string, made *module, m *migration) error {
	text, found, err := fsutil.ReadText(filepath.Join(root, "ivy.xml"))
	if err != nil || !found {
		return err
	}

	ivy, err := parseXML(text)
	if err != nil {
		return fmt.Errorf("`ivy.xml`: %w", err)
	}

	if info := ivy.descendants("info"); len(info) > 0 && info[0].attributes["organisation"] != "" {
		m.group = info[0].attributes["organisation"]
	}

	for _, dep := range ivy.descendants("dependency") {
		made.dependencies = append(made.dependencies, dependency{
			kind: libraryKind, configuration: ivyConfiguration(dep.attributes["conf"]),
			lib:  library{group: dep.attributes["org"], name: dep.attributes["name"], version: dep.attributes["rev"]},
			path: "", paths: nil, text: "",
		})
	}

	m.remove("ivy.xml")

	return nil
}

// ant converts build.xml and ivy.xml. Ant names its source folders in javac
// tasks, so the sources move to Gradle's `src/main` and `src/test`; jars in
// `lib/` stay as `files()` until they get coordinates.
func ant(root string) (*migration, error) {
	converted := newMigration("Ant")
	made := newModule("")
	converted.modules = append(converted.modules, made)

	if err := readBuild(root, made, converted); err != nil {
		return nil, err
	}

	if err := readIvy(root, made, converted); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(filepath.Join(root, "lib"))
	if err != nil {
		entries = nil
	}

	var jars []string

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".jar") {
			jars = append(jars, "lib/"+entry.Name())
		}
	}

	slices.Sort(jars)

	if len(jars) > 0 {
		made.dependencies = append(made.dependencies, dependency{
			kind: jarsKind, configuration: confImplementation, paths: jars,
			lib: library{}, path: "", text: "",
		})
		converted.notes.add("`lib/`: the build uses " + strconv.Itoa(len(jars)) +
			" jar(s) as `files()`; replace each with its Maven coordinates in `gradle/libs.versions.toml`")
	}

	return converted, nil
}
