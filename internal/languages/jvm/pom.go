package jvm

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// pom is a parsed pom.xml: its own elements, and the properties and managed
// versions it inherits.
type pom struct {
	parent     *pom
	element    *element
	properties map[string]string
	managed    map[string]string
	directory  string
	groupID    string
	artifactID string
	boms       []library
}

func propertyReference() *regexp.Regexp { return regexp.MustCompile(`\$\{([^}]+)\}`) }

// file returns the pom's path relative to the root.
func (p *pom) file() string { return pomPath(p.directory) }

func pomPath(directory string) string {
	if directory == "" {
		return filePomXML
	}

	return directory + "/pom.xml"
}

// resolve fills ${property} references the pom defines and leaves the rest.
func (p *pom) resolve(value string) string {
	pattern := propertyReference()

	return pattern.ReplaceAllStringFunc(value, func(whole string) string {
		if found, ok := p.properties[pattern.FindStringSubmatch(whole)[1]]; ok {
			return found
		}

		return whole
	})
}

// library returns the coordinates of a dependency element, with the version
// the pom or its parents manage when the element names none.
func (p *pom) library(dep *element) library {
	group := p.resolve(dep.textOf("groupId"))
	name := p.resolve(dep.textOf("artifactId"))

	return library{
		group:   group,
		name:    name,
		version: cmp.Or(p.resolve(dep.textOf("version")), p.managed[group+":"+name]),
	}
}

// dependenciesOf returns the dependency elements below e's dependencies.
func dependenciesOf(e *element) []*element {
	return e.child("dependencies").childrenNamed("dependency")
}

type coordinates struct{ groupID, artifactID, version string }

func inherited(e *element, parent *pom) coordinates {
	parentElement := e.child("parent")

	version := e.textOf("version")
	if version == "" && parent != nil {
		version = parent.properties["project.version"]
	}

	groupID := e.textOf("groupId")
	if groupID == "" && parent != nil {
		groupID = parent.groupID
	}

	return coordinates{
		groupID:    cmp.Or(groupID, parentElement.textOf("groupId")),
		version:    cmp.Or(version, parentElement.textOf("version")),
		artifactID: e.textOf("artifactId"),
	}
}

func propertiesOf(e *element, parent *pom, own coordinates) map[string]string {
	properties := map[string]string{}
	if parent != nil {
		maps.Copy(properties, parent.properties)
	}

	for _, property := range e.child("properties").childrenNamed("") {
		properties[property.name] = strings.TrimSpace(property.text)
	}

	properties["project.groupId"] = own.groupID
	properties["project.artifactId"] = own.artifactID
	properties["project.version"] = own.version

	return properties
}

func newPom(directory string, e *element, parent *pom) *pom {
	own := inherited(e, parent)
	made := &pom{
		directory:  directory,
		element:    e,
		parent:     parent,
		properties: propertiesOf(e, parent, own),
		managed:    map[string]string{},
		groupID:    own.groupID,
		artifactID: own.artifactID,
		boms:       nil,
	}

	if parent != nil {
		maps.Copy(made.managed, parent.managed)
		made.boms = append(made.boms, parent.boms...)
	}

	management := e.child("dependencyManagement")
	for _, dep := range dependenciesOf(management) {
		version := made.resolve(dep.textOf("version"))
		if version != "" && dep.textOf("scope") != "import" {
			made.managed[made.resolve(dep.textOf("groupId"))+":"+made.resolve(dep.textOf("artifactId"))] = version
		}
	}

	for _, dep := range dependenciesOf(management) {
		if dep.textOf("scope") == "import" {
			made.boms = append(made.boms, made.library(dep))
		}
	}

	return made
}

// readPoms reads the pom at directory and, recursively, its modules, removing
// each pom.xml. A pom already read is not read twice, so modules that name
// each other cannot loop.
func readPoms(root, directory string, parent *pom, m *migration, seen map[string]bool) ([]*pom, error) {
	file := pomPath(directory)

	text, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("`%s`: listed as a Maven module but missing", file)
	}

	parsed, err := parseXML(text)
	if err != nil {
		return nil, fmt.Errorf("`%s`: %w", file, err)
	}

	seen[directory] = true
	current := newPom(directory, parsed, parent)
	m.remove(file)

	poms := []*pom{current}

	for _, module := range parsed.child("modules").childrenNamed("") {
		nested := path.Clean(path.Join(directory, strings.TrimSpace(module.text)))
		nested = strings.TrimSuffix(strings.TrimSuffix(nested, filePomXML), "/")

		if nested == "." {
			nested = ""
		}

		if seen[nested] {
			continue
		}

		more, readErr := readPoms(root, nested, current, m, seen)
		if readErr != nil {
			return nil, readErr
		}

		poms = append(poms, more...)
	}

	return poms, nil
}
