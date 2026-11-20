package swift

import "strings"

// Versions of the packages the kinds depend on: the newest stable release of
// each at the time of writing, which the toolchain resolves on the first build.
const (
	argumentParserVersion = "1.8.2"
	swiftSyntaxVersion    = "604.0.0"
)

// manifestParts are the lists a starter `Package.swift` is made of.
type manifestParts struct {
	imports                                []string
	products, dependencies, targets, tests []node
}

func targetOf(kind, name string, args ...argument) node {
	return call(kind, append([]argument{arg("name", quoted(name))}, args...)...)
}

func dependsOn(items ...node) argument { return arg("dependencies", array(items...)) }

func product(name, packageName string) node {
	return call(".product", arg("name", quoted(name)), arg("package", quoted(packageName)))
}

// addExecutable adds the `<Name>CLI` executable over the library; the
// executable and tool kinds differ only in what the target depends on.
func (m *manifestParts) addExecutable(name string, extra ...node) {
	m.products = append(m.products, call(".executable",
		arg("name", quoted(strings.ToLower(name))), arg("targets", stringArray(name+"CLI"))))
	m.targets = append(m.targets,
		targetOf(".executableTarget", name+"CLI", dependsOn(append([]node{quoted(name)}, extra...)...)))
}

// addMacro adds the macro implementation, the library's declaration of it, a
// client that uses it, and the tests of the expansion, laid out as
// `swift package init --type macro` does.
func (m *manifestParts) addMacro(name string) {
	m.imports = []string{"CompilerPluginSupport", "PackageDescription"}
	m.products = append(m.products, call(".executable",
		arg("name", quoted(name+"Client")), arg("targets", stringArray(name+"Client"))))
	m.dependencies = append(m.dependencies, call(".package",
		arg("url", quoted("https://github.com/swiftlang/swift-syntax.git")),
		arg("from", quoted(swiftSyntaxVersion))))
	m.targets = append(m.targets,
		targetOf(".macro", name+"Macros", dependsOn(
			product("SwiftSyntaxMacros", "swift-syntax"),
			product("SwiftCompilerPlugin", "swift-syntax"))),
		targetOf(".executableTarget", name+"Client", dependsOn(quoted(name))))
	m.tests = append(m.tests, targetOf(".testTarget", name+"MacrosTests", dependsOn(
		quoted(name+"Macros"),
		product("SwiftSyntaxMacroExpansion", "swift-syntax"),
		product("SwiftSyntaxMacrosGenericTestSupport", "swift-syntax"))))
}

// addPlugin adds a package plugin and the product that vends it.
func (m *manifestParts) addPlugin(plugin string, capability node) {
	m.products = append(m.products, call(".plugin",
		arg("name", quoted(plugin)), arg("targets", stringArray(plugin))))
	m.targets = append(m.targets, targetOf(".plugin", plugin, arg("capability", capability)))
}

// render writes the manifest the way swift-format lays it out. Arrays of two
// or more items always take a line each.
func (m *manifestParts) render(name string) string {
	var out strings.Builder

	out.WriteString(toolsVersion + "\n")

	for _, module := range m.imports {
		out.WriteString("import " + module + "\n")
	}

	out.WriteString("\nlet package = Package(\n" +
		`  name: "` + name + "\",\n" +
		"  platforms: [.macOS(.v13)],\n")

	fields := []struct {
		label string
		items []node
	}{{"products", m.products}, {"dependencies", m.dependencies}, {"targets", m.targets}}

	var written []string

	for _, field := range fields {
		if len(field.items) == 0 {
			continue
		}

		prefix := "  " + field.label + ": "
		list := array(field.items...)

		text := list.render(indentStep, len(prefix), 1)
		if len(field.items) > 1 && !strings.Contains(text, "\n") {
			text = "[\n    " + strings.Join(splitFlat(field.items), ",\n    ") + "\n  ]"
		}

		written = append(written, prefix+text)
	}

	out.WriteString(strings.Join(written, ",\n") + "\n)\n")

	return out.String()
}

// splitFlat renders each item on its own line at the items' indent.
func splitFlat(items []node) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.render(indentStep+indentStep, indentStep+indentStep, 1))
	}

	return lines
}

// newManifest returns the `Package.swift` for a new package, before the
// managed blocks. Every kind keeps the package cross-platform: the macOS floor
// is the one `platforms` entry, and no kind names another Apple platform.
func newManifest(name string, kinds map[string]bool, cxx bool) string {
	library := targetOf(".target", name)
	parts := manifestParts{
		imports: []string{"PackageDescription"},
		products: []node{call(".library",
			arg("name", quoted(name)), arg("targets", stringArray(name)))},
	}

	var extra []node

	if cxx {
		library = targetOf(".target", name, dependsOn(quoted(name+"Cxx")))
		extra = append(extra, targetOf(".target", name+"Cxx"))
	}

	if kinds["macro"] {
		library = targetOf(".target", name, dependsOn(append(
			dependencyNames(library), quoted(name+"Macros"))...))
	}

	parts.targets = append([]node{library}, extra...)

	if kinds["executable"] {
		parts.addExecutable(name)
	}

	if kinds["tool"] {
		parts.addExecutable(name, product("ArgumentParser", "swift-argument-parser"))
		parts.dependencies = append(parts.dependencies, call(".package",
			arg("url", quoted("https://github.com/apple/swift-argument-parser.git")),
			arg("from", quoted(argumentParserVersion))))
	}

	if kinds["macro"] {
		parts.addMacro(name)
	}

	if kinds["build-tool-plugin"] {
		parts.addPlugin(name+"BuildToolPlugin", leaf(".buildTool()"))
	}

	if kinds["command-plugin"] {
		parts.addPlugin(name+"CommandPlugin", call(".command", arg("intent", call(".custom",
			arg("verb", quoted(strings.ToLower(name))), arg("description", quoted("Prints a greeting"))))))
	}

	parts.targets = append(parts.targets, targetOf(".testTarget", name+"Tests", dependsOn(quoted(name))))
	parts.targets = append(parts.targets, parts.tests...)

	return parts.render(name)
}

// dependencyNames returns the dependencies argument of a target built by
// targetOf, or nothing.
func dependencyNames(target node) []node {
	for _, a := range target.args {
		if a.label == "dependencies" {
			return a.value.items
		}
	}

	return nil
}
