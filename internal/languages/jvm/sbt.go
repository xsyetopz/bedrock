package jvm

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// sbtPatterns are the forms of build.sbt adopt reads. An sbt build is Scala
// code, so only the common forms are understood.
type sbtPatterns struct {
	library      *regexp.Regexp
	project      *regexp.Regexp
	dependsOn    *regexp.Regexp
	mainClass    *regexp.Regexp
	organization *regexp.Regexp
	javacRelease *regexp.Regexp
}

func sbtForms() sbtPatterns {
	return sbtPatterns{
		library: regexp.MustCompile(
			`"([^"]+)"\s*(%%?)\s*"([^"]+)"\s*%\s*"([^"]+)"(?:\s*%\s*"?(\w+)"?)?`),
		project: regexp.MustCompile(
			`lazy\s+val\s+(\w+)\s*=\s*\(?\s*project(?:\s+in\s*|\.in\s*)\(?\s*file\s*\(\s*"([^"]*)"\s*\)`),
		dependsOn:    regexp.MustCompile(`dependsOn\s*\(([^)]*)\)`),
		mainClass:    regexp.MustCompile(`mainClass\s*:=\s*Some\s*\(\s*"([^"]+)"\s*\)`),
		organization: regexp.MustCompile(`organization\s*:=\s*"([^"]+)"`),
		javacRelease: regexp.MustCompile(`javacOptions\s*\+\+?=\s*Seq\([^)]*"(?:--release|-target)"\s*,\s*"([^"]+)"`),
	}
}

// sbtConfigurations maps an sbt scope to a Gradle configuration.
func sbtConfigurations() map[string]string {
	return map[string]string{
		nameTest:    confTestImplementation,
		"provided":  confCompileOnly,
		nameRuntime: confRuntimeOnly,
	}
}

// section is the text one sbt project owns: its directory and its part of
// build.sbt.
type section struct {
	directory string
	text      string
}

// sbtParts is build.sbt split into the settings every project shares and the
// text each project owns.
type sbtParts struct {
	names    map[string]string
	common   string
	sections []section
	single   bool
}

func (p sbtPatterns) split(text string) sbtParts {
	projects := p.project.FindAllStringSubmatchIndex(text, -1)
	parts := sbtParts{names: map[string]string{}, common: text, sections: nil, single: false}

	if len(projects) == 0 {
		parts.sections = []section{{directory: "", text: text}}
		parts.single = true

		return parts
	}

	parts.common = text[:projects[0][0]]

	for _, match := range projects {
		parts.names[text[match[2]:match[3]]] = strings.Trim(text[match[4]:match[5]], "/.")
	}

	for index, match := range projects {
		end := len(text)
		if index+1 < len(projects) {
			end = projects[index+1][0]
		}

		parts.sections = append(parts.sections, section{
			directory: parts.names[text[match[2]:match[3]]],
			text:      text[match[0]:end],
		})
	}

	return parts
}

func (p sbtPatterns) libraries(source, api string, m *migration) []dependency {
	scopes := sbtConfigurations()

	var found []dependency

	for _, match := range p.library.FindAllStringSubmatch(source, -1) {
		if match[2] == "%%" {
			m.notes.add("`build.sbt`: `" + match[1] + " %% " + match[3] +
				"` is a Scala library; bedrock builds Java and Kotlin, so find a Java one")

			continue
		}

		configuration := api
		if mapped, ok := scopes[strings.ToLower(match[5])]; ok {
			configuration = mapped
		}

		found = append(found, dependency{
			kind: libraryKind, configuration: configuration,
			lib:  library{group: match[1], name: match[3], version: match[4]},
			path: "", paths: nil, text: "",
		})
	}

	return found
}

func (p sbtPatterns) modules(text, api string, names map[string]string) []dependency {
	match := p.dependsOn.FindStringSubmatch(text)
	if match == nil {
		return nil
	}

	var found []dependency

	for reference := range strings.SplitSeq(match[1], ",") {
		fields := strings.Fields(reference)
		if len(fields) == 0 {
			continue
		}

		if target, ok := names[fields[0]]; ok {
			found = append(found, dependency{
				kind: moduleKind, configuration: api, path: gradlePath(target),
				lib: library{}, paths: nil, text: "",
			})
		}
	}

	return found
}

func (p sbtPatterns) moduleFor(part section, kotlin bool, parts sbtParts, m *migration) *module {
	made := newModule(part.directory)
	made.kotlin = kotlin

	source := parts.common + part.text
	if parts.single {
		source = part.text
	}

	if match := p.mainClass.FindStringSubmatch(part.text); match != nil {
		made.mainClass = match[1]
	} else if shared := p.mainClass.FindStringSubmatch(parts.common); shared != nil {
		made.mainClass = shared[1]
	}

	api := confAPI
	if made.mainClass != "" {
		api = confImplementation
	}

	made.dependencies = append(made.dependencies, p.libraries(source, api, m)...)
	made.dependencies = append(made.dependencies, p.modules(part.text, api, parts.names)...)

	if match := p.javacRelease.FindStringSubmatch(source); match != nil {
		javaLevelNote("build.sbt", match[1], m.notes)
	}

	return made
}

// sbt converts build.sbt: settings before the first project apply to every
// project, and each `lazy val x = project.in(file(...))` owns the text up to
// the next one.
func sbt(root string) (*migration, error) {
	converted := newMigration("sbt")
	p := sbtForms()

	text, _, err := fsutil.ReadText(filepath.Join(root, "build.sbt"))
	if err != nil {
		return nil, err
	}

	if match := p.organization.FindStringSubmatch(text); match != nil {
		converted.group = match[1]
	}

	parts := p.split(text)

	for _, part := range parts.sections {
		kotlin := fsutil.Exists(filepath.Join(root, filepath.FromSlash(part.directory), "src", "main", "kotlin"))
		converted.modules = append(converted.modules, p.moduleFor(part, kotlin, parts, converted))
	}

	converted.remove("build.sbt")

	if fsutil.Exists(filepath.Join(root, "project")) {
		converted.remove("project")
	}

	converted.notes.add("`build.sbt`: sbt builds are code, so compare each new `build.gradle.kts` with it; " +
		"`project/` held sbt's plugins and build code")

	return converted, nil
}
