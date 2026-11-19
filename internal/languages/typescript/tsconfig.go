package typescript

import (
	"path"
	"regexp"
	"slices"
)

const baseFile = "bedrock's `tsconfig.json`"

// removedOptions are the options TypeScript 7 rejects (TS5023, TS5102,
// TS5108) or makes the only behavior.
func removedOptions() []string {
	return []string{
		"allowSyntheticDefaultImports", "alwaysStrict", "charset", "downlevelIteration", "esModuleInterop",
		"importsNotUsedAsValues", "keyofStringsOnly", "noImplicitUseStrict", "noStrictGenericChecks", "out",
		"outFile", "prepend", "preserveValueImports", "suppressExcessPropertyErrors",
		"suppressImplicitAnyIndexErrors",
	}
}

// projectOptions are project facts the base config cannot know.
func projectOptions() []string { return []string{"jsx", "lib", "types"} }

func emitOptions() []string {
	return []string{"composite", "declaration", "emitDeclarationOnly", "outDir"}
}

// legacyBase matches what older releases extended: the file from the
// `typescript-bedrock` package.
func legacyBase() *regexp.Regexp {
	return regexp.MustCompile(`(?:^|/)typescript-bedrock/tsconfig\.base\.json$`)
}

func rebasePaths(paths any, baseURL string) *object {
	entries := asObject(paths)
	rebase := func(target string) string { return "./" + path.Join(baseURL, target) }

	if entries.len() == 0 {
		out := newObject()
		out.set("*", []any{rebase("*")})

		return out
	}

	out := newObject()

	for _, key := range entries.keys {
		var targets []string
		for _, target := range asStrings(entries.get(key)) {
			targets = append(targets, rebase(target))
		}

		out.set(key, stringList(targets))
	}

	return out
}

// migrateOptions drops what TypeScript 7 removed and what the base config
// sets, keeping the project's other options.
func migrateOptions(compilerOptions, policy *object) (*object, []string) {
	var notes []string

	options := newObject()

	for _, key := range compilerOptions.keys {
		value := compilerOptions.get(key)
		_, isText := value.(string)

		switch {
		case slices.Contains(removedOptions(), key):
			notes = append(notes, "`tsconfig.json`: removed `"+quote(key)+
				"`; TypeScript 7 rejects it or always behaves as if it were on.")
		case key == "baseUrl" && isText:
			notes = append(notes, "`tsconfig.json`: replaced `\"baseUrl\"` with `\"paths\"`; TypeScript 7 removed it.")
		case policy.has(key) && !slices.Contains(projectOptions(), key):
			if compact(policy.get(key)) != compact(value) {
				notes = append(notes, "`tsconfig.json`: `"+quote(key)+": "+compact(value)+"` is now `"+
					compact(policy.get(key))+"`, as in "+baseFile+".")
			}
		default:
			options.set(key, value)
		}
	}

	if baseURL, ok := compilerOptions.get("baseUrl").(string); ok {
		options.set("paths", rebasePaths(options.get("paths"), baseURL))
	}

	// A project `types` replaces the base one, so it must name `bun` itself.
	if options.has("types") {
		options.set(
			"types",
			stringList(unique(append(asStrings(options.get("types")), asStrings(policy.get("types"))...))),
		)
	}

	if slices.ContainsFunc(emitOptions(), options.has) {
		options.set("noEmit", false)
		options.set("allowImportingTsExtensions", false)

		notes = append(
			notes,
			"`tsconfig.json`: the project emits with `tsc`, so `\"noEmit\"` is off. Check `\"module\"` and `\"target\"` from "+
				baseFile+".",
		)
	}

	return options, notes
}

// extendsOf lists a tsconfig's `extends`, a string or a list.
func extendsOf(tsconfig *object) []string {
	if one, ok := tsconfig.get(nameExtends).(string); ok {
		return []string{one}
	}

	return asStrings(tsconfig.get(nameExtends))
}

// migrateTsconfig inlines the template's compiler options into a project
// tsconfig: its options win, the project's `jsx`, `lib`, `types` and
// unrelated options stay, and an `extends` of the old `typescript-bedrock`
// base file goes.
func migrateTsconfig(tsconfig, base *object) (*object, []string) {
	policy := asObject(base.get("compilerOptions"))
	migrated, notes := migrateOptions(asObject(tsconfig.get("compilerOptions")), policy)
	options := policy.overlay(migrated)

	// An earlier run left the template's base file here; replace it rather
	// than stack it.
	parents := slices.DeleteFunc(extendsOf(tsconfig), legacyBase().MatchString)
	if len(parents) > 0 {
		notes = append(notes, "`tsconfig.json`: still extends "+joinCode(parents)+
			". If one sets an option TypeScript 7 removed, `bun check` names it.")
	}

	next := newObject()
	next.set("$schema", "https://json.schemastore.org/tsconfig.json")

	for _, key := range tsconfig.keys {
		if key != nameExtends {
			next.set(key, tsconfig.get(key))
		}
	}

	if len(parents) > 0 {
		next.set(nameExtends, stringList(parents))
	}

	next.set("compilerOptions", options)
	next.set(
		"exclude",
		stringList(unique(append(asStrings(tsconfig.get("exclude")), asStrings(base.get("exclude"))...))),
	)

	return next, notes
}
