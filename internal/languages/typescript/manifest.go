package typescript

import (
	"regexp"
	"slices"
)

// legacyPackage is the npm package older bedrock releases made every repo
// depend on.
const legacyPackage = "typescript-bedrock"

// removedPackages are the packages Bun and the bedrock tools replace.
func removedPackages() *regexp.Regexp {
	return regexp.MustCompile(`^(?:@biomejs/.+|@eslint(?:-community)?/.+|@evilmartians/lefthook|` +
		`@typescript-eslint/.+|cross-env|eslint(?:-.+)?|esno|husky|@jest/.+|@types/jest|@vitest/.+|` +
		`babel-jest|jest(?:-.+)?|ts-jest|vitest|lefthook|lint-staged|npm-run-all2?|rimraf|` +
		`simple-git-hooks|ts-node(?:-dev)?|tsx|typescript-eslint)$`)
}

func otherEngines() []string { return []string{nameNpm, namePnpm, "yarn"} }

// removedKeys are package.json fields of tools Bun and bedrock replace.
func removedKeys() []string {
	return []string{"eslintConfig", "husky", nameJest, "lint-staged", namePnpm, "prettier", "simple-git-hooks"}
}

// isRemoved reports whether the package goes: the legacy package, or one a
// replaced tool owns that the template does not ship.
func isRemoved(name string, shipped *object) bool {
	return name == legacyPackage || (removedPackages().MatchString(name) && !shipped.has(name))
}

// withoutRemoved drops the removed packages from a dependency map and names
// what it dropped.
func withoutRemoved(deps any, shipped *object) (*object, []string) {
	all := asObject(deps)
	removed := slices.DeleteFunc(slices.Clone(all.keys), func(name string) bool { return !isRemoved(name, shipped) })

	return all.without(func(name string) bool { return isRemoved(name, shipped) }), removed
}

// migrateWorkspaces merges pnpm-workspace.yaml and the template catalog into
// `workspaces`.
func migrateWorkspaces(current any, pnpm *object, templateCatalog *object) *object {
	base := asObject(current)
	if list, ok := current.([]any); ok {
		base = newObject()
		base.set("packages", list)
	}

	workspaces := base.clone()
	workspaces.set(
		"packages",
		stringList(unique(append(asStrings(base.get("packages")), asStrings(pnpm.get("packages"))...))),
	)

	// `bun fmt` does not sort catalogs; the guard requires sorted keys.
	own := asObject(base.get("catalog")).overlay(asObject(pnpm.get("catalog")))
	merged := own.overlay(withNewerPins(own, templateCatalog))
	merged = merged.without(func(name string) bool { return name == legacyPackage })
	workspaces.set("catalog", sortedByName(merged))

	catalogs := asObject(base.get("catalogs")).overlay(asObject(pnpm.get("catalogs")))
	if catalogs.len() > 0 {
		workspaces.set("catalogs", catalogs)
	}

	return workspaces
}

// sortedByName orders the object's keys in code-unit order, the one the
// formatter uses.
func sortedByName(o *object) *object {
	keys := slices.Sorted(slices.Values(o.keys))
	sorted := newObject()

	for _, key := range keys {
		sorted.set(key, o.get(key))
	}

	return sorted
}

// migrateEngines drops npm, pnpm, and yarn pins; the template's engines win
// a conflict.
func migrateEngines(manifest, template *object) *object {
	kept := asObject(manifest.get("engines")).without(func(name string) bool {
		return slices.Contains(otherEngines(), name)
	})

	return kept.overlay(asObject(template.get("engines")))
}

// trustedDependencies lists the packages whose install scripts Bun may run,
// from the manifest and pnpm, minus the removed packages.
func trustedDependencies(manifest, shipped, pnpm *object) []string {
	all := append(asStrings(manifest.get("trustedDependencies")),
		asStrings(asObject(manifest.get(namePnpm)).get("onlyBuiltDependencies"))...)
	all = append(all, asStrings(pnpm.get("onlyBuiltDependencies"))...)

	return slices.DeleteFunc(unique(all), func(name string) bool { return isRemoved(name, shipped) })
}

// migrateManifest moves a package.json onto the template: Bun and bedrock's
// tools in, the packages they replace and the old `typescript-bedrock`
// dependency out.
func migrateManifest(manifest, template, pnpm *object) (*object, []string) {
	shipped := asObject(template.get(nameDevDeps))
	dev, devRemoved := withoutRemoved(manifest.get(nameDevDeps), shipped)
	prod, prodRemoved := withoutRemoved(manifest.get(nameDependencies), shipped)

	var notes []string

	if removed := slices.Concat(devRemoved, prodRemoved); len(removed) > 0 {
		notes = append(notes, "Removed packages Bun replaces: "+joinCode(removed)+".")
	}

	scripts, scriptNotes := migrateScripts(asObject(manifest.get("scripts")), asObject(template.get("scripts")))
	trusted := trustedDependencies(manifest, shipped, pnpm)

	next := manifest.without(func(key string) bool { return key == "trustedDependencies" })
	if prod.len() > 0 || manifest.has(nameDependencies) {
		next.set(nameDependencies, prod)
	}

	next.set(nameDevDeps, dev.overlay(shipped))
	next.set("scripts", scripts)
	next.set("workspaces", migrateWorkspaces(
		manifest.get("workspaces"), pnpm, asObject(asObject(template.get("workspaces")).get("catalog"))))

	if template.has("packageManager") {
		next.set("packageManager", template.get("packageManager"))
	}

	next.set("engines", migrateEngines(manifest, template))

	if overrides := asObject(template.get("overrides")); overrides.len() > 0 {
		own := asObject(manifest.get("overrides"))
		next.set("overrides", own.overlay(withNewerPins(own, overrides)))
	}

	if len(trusted) > 0 {
		next.set("trustedDependencies", stringList(trusted))
	}

	for _, key := range removedKeys() {
		if next.has(key) {
			notes = append(notes, "Removed the `"+quote(key)+"` field from `package.json`.")
		}
	}

	notes = append(notes, scriptNotes...)

	return next.without(func(key string) bool { return slices.Contains(removedKeys(), key) }), notes
}
