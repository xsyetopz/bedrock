package rust

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// errNotTOML marks a manifest that does not parse, which bedrock leaves alone.
var errNotTOML = errors.New("not valid TOML")

// table is a parsed TOML table.
type table = map[string]any

func isTable(value any) bool {
	_, ok := value.(table)

	return ok
}

// parseToml returns the parsed TOML, or false when the text is not valid TOML.
func parseToml(text string) (table, bool) {
	doc := table{}
	if _, err := toml.Decode(text, &doc); err != nil {
		return nil, false
	}

	return doc, true
}

// at returns the value under a path of table keys, or nil.
func at(value any, path ...string) any {
	node := value

	for _, key := range path {
		current, ok := node.(table)
		if !ok {
			return nil
		}

		node = current[key]
	}

	return node
}

// jsonText returns value as `JSON.stringify` writes it: compact and with no
// HTML escapes.
func jsonText(value any) string {
	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		return `""`
	}

	return strings.TrimSuffix(out.String(), "\n")
}

// releaseSetting is one managed `[profile.release]` key.
type releaseSetting struct {
	value any
	key   string
}

// managed is what bedrock manages in a root manifest, read from
// `parts/workspace.toml`.
type managed struct {
	// lintValues is `workspace.lints` as parsed values, to tell whether a
	// manifest matches.
	lintValues any
	// lints are the `[workspace.lints.*]` tables, with the comments above them.
	lints []section
	// release are the `[profile.release]` settings in the template's order.
	release []releaseSetting
}

// readManaged reads the managed lints and release profile from the template.
func readManaged(workspaceToml string) managed {
	parsed, _ := parseToml(workspaceToml)
	sections := parseSections(workspaceToml)

	var result managed

	result.lintValues = at(parsed, "workspace", "lints")

	for _, entry := range sections {
		if entry.named() && strings.HasPrefix(entry.name, "workspace.lints") {
			result.lints = append(result.lints, entry)
		}
	}

	if release, ok := findSection(sections, "profile.release"); ok {
		for _, unit := range release.units {
			if key, isKey := keyOf(unit); isKey {
				result.release = append(
					result.release,
					releaseSetting{key: key, value: at(parsed, "profile", "release", key)},
				)
			}
		}
	}

	return result
}

func isLintsName(name, prefix string) bool {
	return name == prefix || strings.HasPrefix(name, prefix+".")
}

// replacedDevDependencies are the dev-dependencies whose job the `justfile`
// and `lefthook.yml` took over.
func replacedDevDependencies() []string { return []string{"cargo-husky"} }

func dropReplacedDevDependencies(sections []section) []section {
	kept, _ := removeSections(sections, func(name string) bool {
		return slices.ContainsFunc(replacedDevDependencies(), func(dep string) bool {
			return name == "dev-dependencies."+dep
		})
	})

	result := make([]section, 0, len(kept))

	for _, entry := range kept {
		if !entry.named() || entry.name != "dev-dependencies" {
			result = append(result, entry)

			continue
		}

		cleaned := withoutKeys(entry, func(key string) bool { return slices.Contains(replacedDevDependencies(), key) })
		if len(cleaned.units) < len(entry.units) && hasNoEntries(cleaned) {
			continue
		}

		result = append(result, cleaned)
	}

	return result
}

// useWorkspaceLints points the package at the workspace lints, which replace
// its own.
func useWorkspaceLints(sections []section) []section {
	rest, position := removeSections(sections, func(name string) bool { return isLintsName(name, "lints") })
	without := dropReplacedDevDependencies(rest)

	if position < 0 {
		position = len(without)
	}

	return slices.Insert(without, min(position, len(without)), makeSection("lints", []string{"workspace = true"}))
}

func packageSatisfied(doc table) bool {
	if at(doc, "package", "edition") == nil || !reflect.DeepEqual(doc["lints"], table{"workspace": true}) {
		return false
	}

	return !slices.ContainsFunc(replacedDevDependencies(), func(dep string) bool {
		return at(doc, "dev-dependencies", dep) != nil
	})
}

// withDeclaredEdition names the edition of a package that declares none. Cargo
// reads that as 2015, so `edition = "2015"` keeps the meaning and lets the
// edition migration take the package to 2024. An `edition.workspace` counts as
// one.
func withDeclaredEdition(doc table, sections []section) []section {
	found, ok := findSection(sections, "package")
	if !ok || at(doc, "package", "edition") != nil {
		return sections
	}

	return upsertSection(sections, withKey(found, "edition", `"2015"`))
}

// memberManifest returns the member manifest with a declared edition and the
// workspace lints in place of its own.
func memberManifest(text string) (string, error) {
	doc, ok := parseToml(text)
	if !ok {
		return "", errNotTOML
	}

	if packageSatisfied(doc) {
		return text, nil
	}

	sections := useWorkspaceLints(parseSections(text))

	return renderSections(tidy(withDeclaredEdition(doc, sections))), nil
}

func rootSatisfied(doc table, wanted managed) bool {
	if doc["package"] != nil && !packageSatisfied(doc) {
		return false
	}

	if at(doc, "workspace", "resolver") != "3" || !reflect.DeepEqual(at(doc, "workspace", "lints"), wanted.lintValues) {
		return false
	}

	return !slices.ContainsFunc(wanted.release, func(setting releaseSetting) bool {
		return at(doc, "profile", "release", setting.key) == nil
	})
}

func withResolver(sections []section) []section {
	workspace, ok := findSection(sections, "workspace")
	if !ok {
		return append(slices.Clone(sections), makeSection("workspace", []string{`resolver = "3"`}))
	}

	cleaned := withoutKeys(workspace, func(key string) bool { return isLintsName(key, "lints") })

	return upsertSection(sections, withKey(cleaned, "resolver", `"3"`))
}

func withManagedLints(sections []section, wanted managed) []section {
	rest, position := removeSections(sections, func(name string) bool { return isLintsName(name, "workspace.lints") })

	if position < 0 {
		position = slices.IndexFunc(rest, func(entry section) bool {
			return entry.named() && strings.HasPrefix(entry.name, "profile")
		})
	}

	if position < 0 {
		position = len(rest)
	}

	return slices.Insert(rest, min(position, len(rest)), wanted.lints...)
}

// withReleaseProfile adds the managed release keys the repository has not set;
// its own values stay.
func withReleaseProfile(doc table, sections []section, wanted managed) []section {
	release, found := findSection(sections, "profile.release")
	if !found {
		release = makeSection("profile.release", nil)
	}

	added := false

	for _, setting := range wanted.release {
		if at(doc, "profile", "release", setting.key) == nil {
			release = withKey(release, setting.key, jsonText(setting.value))
			added = true
		}
	}

	if !found && !added {
		return sections
	}

	return upsertSection(sections, release)
}

// rootManifest returns the root manifest with the managed workspace lints,
// resolver, and release profile. A single-package root gains a `[workspace]`
// table and uses them.
func rootManifest(text string, wanted managed) (string, error) {
	doc, ok := parseToml(text)
	if !ok {
		return "", errNotTOML
	}

	if rootSatisfied(doc, wanted) {
		return text, nil
	}

	sections := parseSections(text)
	if doc["package"] != nil {
		sections = withDeclaredEdition(doc, useWorkspaceLints(sections))
	}

	sections = withManagedLints(withResolver(sections), wanted)

	return renderSections(tidy(withReleaseProfile(doc, sections, wanted))), nil
}

// editionsOf returns the editions a manifest declares, at the package and
// workspace level.
func editionsOf(text string) []string {
	doc, _ := parseToml(text)

	var editions []string

	for _, value := range []any{at(doc, "package", "edition"), at(doc, "workspace", "package", "edition")} {
		if edition, ok := value.(string); ok {
			editions = append(editions, edition)
		}
	}

	return editions
}

// toolchainFile returns `rust-toolchain.toml` with the pinned channel,
// keeping the targets the project added.
func toolchainFile(template, current string, extra []string) string {
	var targets []string

	doc, _ := parseToml(current)
	if listed, ok := at(doc, "toolchain", "targets").([]any); ok {
		for _, target := range listed {
			if name, isString := target.(string); isString {
				targets = append(targets, name)
			}
		}
	}

	targets = append(targets, extra...)
	slices.Sort(targets)
	targets = slices.Compact(targets)

	base := template
	if !strings.HasSuffix(base, "\n") {
		base += "\n"
	}

	if len(targets) == 0 {
		return base
	}

	quoted := make([]string, 0, len(targets))
	for _, target := range targets {
		quoted = append(quoted, jsonText(target))
	}

	return base + "targets = [" + strings.Join(quoted, ", ") + "]\n"
}
