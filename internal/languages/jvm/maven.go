package jvm

import (
	"cmp"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// converter turns the poms of one Maven reactor into modules.
type converter struct {
	reactor   map[string]string
	migration *migration
	root      string
}

// configurationFor maps a Maven scope to a Gradle configuration; api is what
// a compile scope becomes.
func configurationFor(scope, api string) string {
	switch scope {
	case "provided":
		return confCompileOnly
	case nameRuntime:
		return confRuntimeOnly
	case nameTest:
		return confTestImplementation
	default:
		return api
	}
}

func (c *converter) dependencyFor(p *pom, dep *element, api string) dependency {
	scope := dep.textOf("scope")
	configuration := configurationFor(scope, api)
	lib := p.library(dep)

	if scope == "system" {
		jar := p.resolve(dep.textOf("systemPath"))
		c.migration.notes.add("`" + p.file() + "`: `" + lib.coordinates() +
			"` has system scope; replace `files(\"" + jar + "\")` with its Maven coordinates")

		return dependency{
			kind: jarsKind, configuration: confImplementation, paths: []string{jar},
			lib: library{}, path: "", text: "",
		}
	}

	if local, ok := c.reactor[lib.coordinates()]; ok {
		return dependency{
			kind: moduleKind, configuration: configuration, path: gradlePath(local),
			lib: library{}, paths: nil, text: "",
		}
	}

	if kind := dep.textOf("type"); dep.textOf("classifier") != "" || (kind != "" && kind != "jar") {
		c.migration.notes.add("`" + p.file() + "`: `" + lib.coordinates() +
			"` has a classifier or type; add it to the catalog entry by hand")
	}

	return dependency{kind: libraryKind, configuration: configuration, lib: lib, path: "", paths: nil, text: ""}
}

// parentPlatform imports the outermost pom's parent, which lies outside the
// repository, as a platform.
func parentPlatform(p *pom, api string) []dependency {
	top := p
	for top.parent != nil {
		top = top.parent
	}

	parent := top.element.child("parent")
	if parent == nil {
		return nil
	}

	lib := library{
		group:   parent.textOf("groupId"),
		name:    parent.textOf("artifactId"),
		version: parent.textOf("version"),
	}

	return []dependency{{kind: platformKind, configuration: api, lib: lib, path: "", paths: nil, text: ""}}
}

func (c *converter) levelNotes(p *pom) {
	for _, key := range []string{
		"maven.compiler.release", "maven.compiler.target", "maven.compiler.source", "java.version",
	} {
		javaLevelNote(p.file(), p.properties[key], c.migration.notes)
	}

	if releases := p.element.descendants("release"); len(releases) > 0 {
		javaLevelNote(p.file(), p.resolve(releases[0].text), c.migration.notes)
	}

	build := p.element.child("build")
	for _, tag := range []string{"sourceDirectory", "testSourceDirectory"} {
		if directory := build.textOf(tag); directory != "" {
			c.migration.notes.add("`" + p.file() + "`: `" + tag + "` `" + directory +
				"` is not where Gradle looks; move the sources to `src/main` or `src/test`")
		}
	}
}

func mainClassOf(p *pom) string {
	for _, tag := range []string{"mainClass", "Main-Class"} {
		for _, found := range p.element.descendants(tag) {
			if value := p.resolve(found.text); value != "" {
				return value
			}
		}
	}

	return cmp.Or(p.properties["start-class"], p.properties["exec.mainClass"])
}

func (c *converter) moduleFor(p *pom) *module {
	made := newModule(p.directory)
	made.kotlin = fsutil.Exists(filepath.Join(c.root, filepath.FromSlash(p.directory), "src", "main", "kotlin"))

	for _, artifact := range p.element.descendants("artifactId") {
		if artifact.text == "kotlin-maven-plugin" {
			made.kotlin = true
		}
	}

	made.mainClass = mainClassOf(p)

	api := confAPI
	if made.mainClass != "" {
		api = confImplementation
	}

	made.dependencies = append(made.dependencies, parentPlatform(p, api)...)

	for _, bom := range p.boms {
		made.dependencies = append(made.dependencies, dependency{
			kind: platformKind, configuration: api, lib: bom, path: "", paths: nil, text: "",
		})
	}

	for _, dep := range dependenciesOf(p.element) {
		made.dependencies = append(made.dependencies, c.dependencyFor(p, dep, api))
	}

	c.levelNotes(p)

	return made
}

func (c *converter) noteParent(p *pom) {
	parent := p.element.child("parent")
	if p.parent == nil && parent != nil {
		c.migration.notes.add("`" + p.file() + "`: the parent `" + parent.textOf("groupId") + ":" +
			parent.textOf("artifactId") + ":" + parent.textOf("version") +
			"` is outside the repository, so each module imports it as a platform for the versions it manages; " +
			"move its plugins and settings by hand")
	}
}

// maven converts the Maven build rooted at root's pom.xml, following its
// modules.
func maven(root string) (*migration, error) {
	converted := newMigration(toolMaven)

	poms, err := readPoms(root, "", nil, converted, map[string]bool{})
	if err != nil {
		return nil, err
	}

	for _, wrapper := range fsutil.Existing(root, []string{"mvnw", "mvnw.cmd", ".mvn"}) {
		converted.remove(wrapper)
	}

	converted.group = poms[0].groupID

	reactor := make(map[string]string, len(poms))
	for _, p := range poms {
		reactor[p.groupID+":"+p.artifactID] = p.directory
	}

	c := &converter{root: root, migration: converted, reactor: reactor}

	for _, p := range poms {
		c.noteParent(p)
	}

	for _, p := range poms {
		if p.element.textOf("packaging") != "pom" {
			converted.modules = append(converted.modules, c.moduleFor(p))
		}
	}

	return converted, nil
}
