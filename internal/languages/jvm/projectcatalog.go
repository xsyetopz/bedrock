package jvm

import (
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// catalogLibrary is one `[libraries]` entry in a form Gradle reads: a
// `group:name:version` string, or a table with `module`, or `group` and
// `name`, and a `version` string or `version.ref`.
type catalogLibrary struct {
	Version  any
	notation string
	Module   string
	Group    string
	Name     string
}

// UnmarshalTOML reads either form of the entry.
func (c *catalogLibrary) UnmarshalTOML(value any) error {
	switch typed := value.(type) {
	case string:
		c.notation = typed
	case map[string]any:
		c.Module, _ = typed["module"].(string)
		c.Group, _ = typed["group"].(string)
		c.Name, _ = typed["name"].(string)
		c.Version = typed["version"]
	}

	return nil
}

// library returns the entry as a library, resolving `version.ref` through
// versions. A rich version, such as `{ strictly = "…" }`, leaves the version
// empty, so a migrated library with a version does not reuse the entry.
func (c *catalogLibrary) library(versions map[string]any) library {
	group, rest, _ := strings.Cut(c.notation, ":")
	if name, version, found := strings.Cut(rest, ":"); found {
		return library{group: group, name: name, version: version}
	}

	group, name := c.Group, c.Name

	if before, after, found := strings.Cut(c.Module, ":"); found {
		group, name = before, after
	}

	version, _ := c.Version.(string)
	if ref, isTable := c.Version.(map[string]any); isTable {
		key, _ := ref["ref"].(string)
		version, _ = versions[key].(string)
	}

	return library{group: group, name: name, version: version}
}

// projectAliases returns aliases seeded with the libraries the project's
// catalog declares, so a migrated library reuses its entry and a new alias
// avoids its names. A catalog that does not parse seeds none.
func projectAliases(catalog string) *aliases {
	var parsed struct {
		Versions  map[string]any             `toml:"versions"`
		Libraries map[string]*catalogLibrary `toml:"libraries"`
	}

	known := &aliases{entries: nil, existing: 0}
	if _, err := toml.Decode(catalog, &parsed); err != nil {
		return known
	}

	names := make([]string, 0, len(parsed.Libraries))
	for name := range parsed.Libraries {
		names = append(names, name)
	}

	slices.Sort(names)

	// Gradle reads `_` and `.` in an alias as `-`.
	normalize := strings.NewReplacer("_", "-", ".", "-")

	for _, name := range names {
		known.entries = append(known.entries, alias{
			name: normalize.Replace(name),
			lib:  parsed.Libraries[name].library(parsed.Versions),
		})
	}

	known.existing = len(known.entries)

	return known
}
