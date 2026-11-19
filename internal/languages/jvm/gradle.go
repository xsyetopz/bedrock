package jvm

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// quoted matches a single- or double-quoted string and captures its text.
const quoted = `['"]([^'"]+)['"]`

// gradlePatterns are the forms adopt reads from a build script, which is
// code: each is one line shape that Gradle builds commonly use.
type gradlePatterns struct {
	include      *regexp.Regexp
	dependency   *regexp.Regexp
	coordinates  *regexp.Regexp
	mapNotation  *regexp.Regexp
	project      *regexp.Regexp
	platform     *regexp.Regexp
	catalog      *regexp.Regexp
	mainClass    *regexp.Regexp
	group        *regexp.Regexp
	level        *regexp.Regexp
	kotlinPlugin *regexp.Regexp
	block        *regexp.Regexp
	quotedText   *regexp.Regexp
}

func patterns() gradlePatterns {
	return gradlePatterns{
		include:     regexp.MustCompile(`(?m)^\s*include\s*\(?(.+?)\)?\s*$`),
		dependency:  regexp.MustCompile(`^\s*(\w+)\s*[ (]\s*(.+?)\s*$`),
		coordinates: regexp.MustCompile(`^\(?` + quoted + `\)?$`),
		mapNotation: regexp.MustCompile(`group\s*[:=]\s*` + quoted + `\s*,\s*name\s*[:=]\s*` + quoted +
			`(?:\s*,\s*version\s*[:=]\s*` + quoted + `)?`),
		project:   regexp.MustCompile(`project\s*\(\s*(?:path\s*[:=]\s*)?` + quoted + `\s*\)`),
		platform:  regexp.MustCompile(`^\(?(?:enforced)?[pP]latform\s*\(\s*` + quoted + `\s*\)\)?$`),
		catalog:   regexp.MustCompile(`^\(?(libs(?:\.\w+)+)\)?$`),
		mainClass: regexp.MustCompile(`mainClass(?:Name)?(?:\s*=\s*|\.set\(\s*|\s+)` + quoted),
		group:     regexp.MustCompile(`(?m)^group\s*=?\s*` + quoted),
		level: regexp.MustCompile(`(?:sourceCompatibility|targetCompatibility|jvmTarget)\s*=\s*['"]?` +
			`(?:JavaVersion\.VERSION_)?([\d_.]+)|(?:jvmToolchain|JavaLanguageVersion\.of)\s*\(\s*(\d+)`),
		kotlinPlugin: regexp.MustCompile(`kotlin\(\s*"jvm"\s*\)|org\.jetbrains\.kotlin\.jvm|kotlin-gradle-plugin`),
		block:        regexp.MustCompile(`(?ms)^dependencies\s*\{\s*\n(.*?)^\}`),
		quotedText:   regexp.MustCompile(quoted),
	}
}

// configurations maps Gradle's configuration names, including those Gradle 7
// removed, to the ones bedrock writes.
func configurations() map[string]string {
	return map[string]string{
		"compile":                 confAPI,
		"testCompile":             confTestImplementation,
		nameRuntime:               confRuntimeOnly,
		"testRuntime":             confTestRuntimeOnly,
		confAPI:                   confAPI,
		confImplementation:        confImplementation,
		confCompileOnly:           confCompileOnly,
		"compileOnlyApi":          "compileOnlyApi",
		confRuntimeOnly:           confRuntimeOnly,
		"annotationProcessor":     "annotationProcessor",
		confTestImplementation:    confTestImplementation,
		"testCompileOnly":         "testCompileOnly",
		confTestRuntimeOnly:       confTestRuntimeOnly,
		"testAnnotationProcessor": "testAnnotationProcessor",
	}
}

// isGradle reports whether root has a Gradle build that applies no bedrock
// plugin yet.
func isGradle(root string) (bool, error) {
	for _, name := range []string{fileSettingsGradle, fileBuildGradle} {
		if fsutil.IsRegularFile(filepath.Join(root, name)) {
			return true, nil
		}
	}

	for _, name := range []string{fileSettingsGradleKts, fileBuildGradleKts} {
		text, found, err := fsutil.ReadText(filepath.Join(root, name))
		if err != nil {
			return false, err
		}

		if found && !strings.Contains(text, "bedrock.") {
			return true, nil
		}
	}

	return false, nil
}

func libraryOf(configuration, coordinates string) (dependency, bool) {
	parts := strings.Split(coordinates, ":")
	if len(parts) <= 1 {
		return dependency{}, false
	}

	lib := library{group: parts[0], name: parts[1], version: ""}
	if len(parts) > versionIndex {
		lib.version = parts[versionIndex]
	}

	return dependency{kind: libraryKind, configuration: configuration, lib: lib, path: "", paths: nil, text: ""}, true
}

// dependencyOf returns the dependency one line of a dependencies block
// means, when it is a form adopt reads.
func (p gradlePatterns) dependencyOf(configuration, argument string) (dependency, bool) {
	if match := p.coordinates.FindStringSubmatch(argument); match != nil {
		return libraryOf(configuration, match[1])
	}

	if match := p.mapNotation.FindStringSubmatch(argument); match != nil {
		return dependency{
			kind: libraryKind, configuration: configuration, path: "", paths: nil, text: "",
			lib: library{group: match[1], name: match[2], version: match[3]},
		}, true
	}

	if match := p.project.FindStringSubmatch(argument); match != nil {
		return dependency{
			kind: moduleKind, configuration: configuration, path: ":" + strings.Trim(match[1], ":"),
			lib: library{}, paths: nil, text: "",
		}, true
	}

	if match := p.platform.FindStringSubmatch(argument); match != nil {
		if lib, ok := libraryOf(configuration, match[1]); ok {
			lib.kind = platformKind

			return lib, true
		}
	}

	if match := p.catalog.FindStringSubmatch(argument); match != nil {
		return dependency{
			kind: notationKind, configuration: configuration, text: match[1],
			lib: library{}, path: "", paths: nil,
		}, true
	}

	return dependency{}, false
}

func (p gradlePatterns) readDependencies(text, file string, m *migration) []dependency {
	block := p.block.FindStringSubmatch(text)
	if block == nil {
		return nil
	}

	known := configurations()

	var found []dependency

	for line := range strings.SplitSeq(block[1], "\n") {
		line = strings.TrimSpace(line)
		// The bedrock plugins add the Kotlin standard library and test library.
		if line == "" || strings.HasPrefix(line, "//") || strings.Contains(line, "kotlin(") {
			continue
		}

		var (
			dep   dependency
			valid bool
		)

		if match := p.dependency.FindStringSubmatch(line); match != nil && known[match[1]] != "" {
			dep, valid = p.dependencyOf(known[match[1]], match[2])
		}

		if valid {
			found = append(found, dep)
		} else {
			m.notes.add("`" + file + "`: kept out `" + line + "`; add it to the new build by hand")
		}
	}

	return found
}

// includedDirectories returns the project directories a settings file
// includes.
func (p gradlePatterns) includedDirectories(settings string) []string {
	var lists []string
	for _, line := range p.include.FindAllStringSubmatch(settings, -1) {
		lists = append(lists, line[1])
	}

	var directories []string
	for _, match := range p.quotedText.FindAllStringSubmatch(strings.Join(lists, " "), -1) {
		directories = append(directories, strings.ReplaceAll(strings.Trim(match[1], ":"), ":", "/"))
	}

	return directories
}

// settingsFile is a settings script: its name is "" when the build has none.
type settingsFile struct {
	name string
	text string
}

// readSettings returns the settings script, preferring the Kotlin DSL.
func readSettings(root string) (settingsFile, error) {
	for _, candidate := range []string{fileSettingsGradleKts, fileSettingsGradle} {
		body, found, readErr := fsutil.ReadText(filepath.Join(root, candidate))
		if readErr != nil {
			return settingsFile{}, readErr
		}

		if found {
			return settingsFile{name: candidate, text: body}, nil
		}
	}

	return settingsFile{}, nil
}

// gradle converts a Gradle build in either DSL onto the bedrock plugins.
// Build scripts are code, so this reads the plugin, mainClass, and dependency
// lines each module's script has, and names the others in the notes; git
// holds the old scripts.
func gradle(root string) (*migration, error) {
	converted := newMigration("Gradle")
	p := patterns()

	settings, err := readSettings(root)
	if err != nil {
		return nil, err
	}

	directories := p.includedDirectories(settings.text)
	if settings.name == fileSettingsGradle {
		converted.remove(fileSettingsGradle)
	}

	withRoot := directories
	if len(directories) == 0 || fsutil.Exists(filepath.Join(root, "src")) {
		withRoot = append([]string{""}, directories...)
	}

	seen := map[string]bool{}

	for _, directory := range append([]string{""}, directories...) {
		if seen[directory] {
			continue
		}

		seen[directory] = true

		err = p.readProject(root, directory, slices.Contains(withRoot, directory), converted)
		if err != nil {
			return nil, err
		}
	}

	return converted, nil
}

// dropRoot removes a root script that holds no code: it only configured the
// other modules.
func dropRoot(file, text string, m *migration) {
	m.remove(file)

	if regexp.MustCompile(`\b(allprojects|subprojects)\b`).MatchString(text) {
		m.notes.add("`" + file + "`: `allprojects` or `subprojects` settings; " +
			"the bedrock plugins set what each module needs, so move the rest into the modules")
	}
}

// convertible reports whether the script needs converting; a bedrock script
// stays, and an Android or multiplatform one is left to hand.
func convertible(text, directory, file string, m *migration) bool {
	if strings.Contains(text, `id("bedrock.`) {
		kept := newModule(directory)
		kept.kept = true
		m.modules = append(m.modules, kept)

		return false
	}

	if regexp.MustCompile(`com\.android\.|multiplatform`).MatchString(text) {
		m.notes.add("`" + file + "`: an Android or multiplatform build; " +
			"apply `bedrock.android`, `bedrock.kmp`, or `bedrock.cmp` to it by hand")

		return false
	}

	return true
}

func (p gradlePatterns) fill(made *module, text, file string, m *migration) {
	if match := p.mainClass.FindStringSubmatch(text); match != nil {
		made.mainClass = match[1]
	}

	if level := p.level.FindStringSubmatch(text); level != nil {
		javaLevelNote(file, strings.ReplaceAll(level[1]+level[2], "_", "."), m.notes)
	}

	made.dependencies = append(made.dependencies, p.readDependencies(text, file, m)...)
}

func (p gradlePatterns) readProject(root, directory string, hasCode bool, m *migration) error {
	base := ""
	if directory != "" {
		base = directory + "/"
	}

	groovy, groovyFound, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(base+fileBuildGradle)))
	if err != nil {
		return err
	}

	kotlin, kotlinFound, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(base+fileBuildGradleKts)))
	if err != nil {
		return err
	}

	if !groovyFound && !kotlinFound {
		return nil
	}

	text, file := kotlin, base+fileBuildGradleKts
	if groovyFound {
		text, file = groovy, base+fileBuildGradle
	}

	if match := p.group.FindStringSubmatch(text); directory == "" && match != nil {
		m.group = match[1]
	}

	if !hasCode {
		dropRoot(file, text, m)

		return nil
	}

	if !convertible(text, directory, file, m) {
		return nil
	}

	if groovyFound {
		m.remove(file)
	}

	made := newModule(directory)
	made.kotlin = p.kotlinPlugin.MatchString(text) ||
		fsutil.Exists(filepath.Join(root, filepath.FromSlash(base), "src", "main", "kotlin"))
	p.fill(made, text, file, m)
	m.modules = append(m.modules, made)

	return nil
}
