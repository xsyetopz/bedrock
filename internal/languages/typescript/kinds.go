package typescript

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
)

// compilePlatforms are the platforms `bun build --compile` makes an executable
// for. `scripts/build.ts` maps each to Bun's `bun-<os>-<arch>` targets, and Bun
// cross-compiles every one of them from any host.
func compilePlatforms() []string { return []string{"linux", "macos", "windows"} }

// kindFields are the package.json fields each kind's starter files need: the
// entry point other packages import, the executable, the dependencies a
// framework adds, and the scripts that run and build it. platforms are the
// ones the user chose for the kind.
func kindFields(kind, name string, platforms []string) *object {
	fields := newObject()

	switch kind {
	case kindCLI:
		bin := newObject()
		bin.set(name, "./src/cli.ts")
		fields.set("bin", bin)
		fields.set(nameScripts, buildScripts("src/cli.ts", platforms))
	case "library":
		exports := newObject()
		exports.set(".", "./src/index.ts")
		fields.set("exports", exports)
	case kindServer:
		scripts := buildScripts("src/index.ts", platforms)
		scripts.set("dev", "bun --hot src/index.ts")
		scripts.set("start", "bun src/index.ts")
		fields.set(nameScripts, scripts)
	case kindReact:
		scripts := newObject()
		scripts.set("build", "bun scripts/build.ts")
		scripts.set("dev", "bun --hot src/index.ts")
		scripts.set("start", "NODE_ENV=production bun src/index.ts")
		fields.set(nameScripts, scripts)
		fields.set(nameDependencies, catalogDependencies("react", "react-dom"))
		fields.set(nameDevDeps, catalogDependencies("@types/react", "@types/react-dom"))

		catalog := newObject()
		catalog.set("@types/react", "19.3.0")
		catalog.set("@types/react-dom", "19.3.0")
		catalog.set("react", "19.3.1")
		catalog.set("react-dom", "19.3.1")

		workspaces := newObject()
		workspaces.set("catalog", catalog)
		fields.set("workspaces", workspaces)
	}

	return fields
}

// buildScripts holds the `build` script of a kind that compiles to an
// executable: the chosen platforms, or the host alone when none is chosen.
func buildScripts(entry string, platforms []string) *object {
	scripts := newObject()
	scripts.set("build", strings.Join(append([]string{"bun scripts/build.ts", entry}, platforms...), " "))

	return scripts
}

// catalogDependencies maps each package to its `workspaces.catalog` version.
func catalogDependencies(names ...string) *object {
	deps := newObject()
	for _, name := range names {
		deps.set(name, "catalog:")
	}

	return deps
}

// kindTsconfig are the compiler options a kind needs beyond the base config.
func kindTsconfig(kind string) *object {
	options := newObject()

	if kind == kindReact {
		options.set("jsx", "react-jsx")
		options.set("lib", stringList([]string{"ESNext", "DOM", "DOM.Iterable"}))
	}

	return options
}

// choices are the kinds and platforms `bedrock init` was given.
type choices struct{ kinds, platforms []string }

// initChoices reads the kinds and platforms `bedrock init` was given; both are
// empty for any other command.
func initChoices(c *language.Context) (choices, error) {
	if !c.Init {
		return choices{nil, nil}, nil
	}

	kinds, err := config.SplitList(c.Options["kind"])
	if err != nil {
		return choices{nil, nil}, fmt.Errorf("read the `kind` option: %w", err)
	}

	platforms, err := config.SplitList(c.Options["platform"])
	if err != nil {
		return choices{nil, nil}, fmt.Errorf("read the `platform` option: %w", err)
	}

	return choices{kinds, platforms}, nil
}

// kindsFields are the fields of the kinds `bedrock init` starts, for fields
// a manifest lacks.
func kindsFields(c *language.Context, name string) (*object, error) {
	fields := newObject()

	chosen, err := initChoices(c)
	if err != nil {
		return nil, err
	}

	for _, kind := range chosen.kinds {
		next := kindFields(kind, name, chosen.platforms)
		for _, key := range []string{nameScripts, nameDependencies, nameDevDeps} {
			if next.has(key) {
				next.set(key, asObject(fields.get(key)).overlay(asObject(next.get(key))))
			}
		}

		fields = fields.overlay(next)
	}

	return fields, nil
}

// kindsTsconfig are the compiler options of the kinds `bedrock init` starts.
func kindsTsconfig(c *language.Context) (*object, error) {
	options := newObject()

	chosen, err := initChoices(c)
	if err != nil {
		return nil, err
	}

	for _, kind := range chosen.kinds {
		options = options.overlay(kindTsconfig(kind))
	}

	return options, nil
}

// withOptions sets each option on the template's one-line `"key": value,`
// entries.
func withOptions(text string, options *object) (string, error) {
	for _, key := range options.keys {
		entry := regexp.MustCompile(`(?m)^(\t+"` + regexp.QuoteMeta(key) + `": ).*,$`)
		if !entry.MatchString(text) {
			return "", fmt.Errorf("the tsconfig template has no `%s` option to set", key)
		}

		text = entry.ReplaceAllString(text, "${1}"+strings.ReplaceAll(compact(options.get(key)), ",", ", ")+",")
	}

	return text, nil
}
