package typescript

import (
	"cmp"
	"slices"
)

// keyOrder is the order `bun fmt` gives package.json keys; other keys follow
// alphabetically.
func keyOrder() []string {
	return []string{
		"name", "version", "private", "description", "keywords", "homepage", "bugs", "license", "author",
		"repository", "bin", "workspaces", "files", "type", "sideEffects", "main", "module", "types",
		"exports", "publishConfig", nameScripts, nameDependencies, nameDevDeps, "peerDependencies",
		"optionalDependencies", "overrides", "engines", "packageManager", "trustedDependencies",
	}
}

// sortedMaps are the maps of names that `bun fmt` sorts alphabetically.
func sortedMaps() []string {
	return []string{
		nameScripts, nameDependencies, nameDevDeps, "peerDependencies", "optionalDependencies", "overrides",
	}
}

func rank(order []string, key string) int {
	if index := slices.Index(order, key); index >= 0 {
		return index
	}

	return len(order)
}

// sortManifest returns the manifest with its keys in the order `bun fmt`
// writes, so a fresh adopt passes `bun fmt --check`.
func sortManifest(manifest *object) *object {
	order := keyOrder()
	keys := slices.Clone(manifest.keys)

	// Code-unit order, the one the formatter uses; a locale order would
	// reorder `@scope` names.
	slices.SortStableFunc(keys, func(a, b string) int {
		return cmp.Or(cmp.Compare(rank(order, a), rank(order, b)), cmp.Compare(a, b))
	})

	sorted := newObject()

	for _, key := range keys {
		value := manifest.get(key)
		if inner, ok := value.(*object); ok && slices.Contains(sortedMaps(), key) {
			value = sortedByName(inner)
		}

		sorted.set(key, value)
	}

	return sorted
}
