import { expect, test } from "bun:test";
import { join } from "node:path";

import { tempRepo } from "../../core/testing.ts";
import { childFile, type Module, moduleDirectory, modules } from "./modules.ts";

const summary = (source: string): [string, boolean][] =>
	modules(source).map((found) => [found.name, found.testOnly]);

test("finds test modules behind attributes and visibility", () => {
	// Arrange
	const source =
		"#[cfg(test)]\n#[allow(dead_code)]\npub(crate) mod tests;\nmod util;\n";

	// Act
	const found = modules(source);

	// Assert
	expect(summary(source)).toEqual([
		["tests", true],
		["util", false],
	]);
	expect(found[0]?.line).toBe(3);
});

test("reads compound cfg predicates", () => {
	// Arrange
	const cases: [string, boolean][] = [
		["#[cfg(all(test, unix))] mod a;", true],
		["#[cfg(not(test))] mod b;", false],
		["#[cfg(any(test, debug_assertions))] mod c;", false],
		["#[cfg(any(test, all(test, unix)))] mod d;", true],
		['#[cfg(test = "x")] mod e;', false],
		['#[cfg(feature = "test")] mod f;', false],
		["#[cfg( test )] unsafe mod g;", true],
	];

	// Act
	const results = cases.map(([source]) => modules(source)[0]?.testOnly);

	// Assert
	expect(results).toEqual(cases.map(([, expected]) => expected));
});

test("comments, strings, and raw strings declare nothing", () => {
	// Arrange
	const source = [
		"// #[cfg(test)] mod x;",
		"/* #[cfg(test)] mod z; */",
		'const S: &str = "#[cfg(test)] mod y;";',
		'const R: &str = r#"#[cfg(test)] mod w;"#;',
	].join("\n");

	// Act
	const found = modules(source);

	// Assert
	expect(found).toEqual([]);
});

test("reads the path attribute", () => {
	// Arrange / Act
	const found = modules('#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;');

	// Assert
	expect(found[0]?.path).toBe("checks.rs");
});

test("an inline module has its body range and the depth it sits at", () => {
	// Arrange
	const source = "#[cfg(test)]\nmod tests {\n    mod inner { }\n}\nmod last;\n";

	// Act
	const [outer, inner, last] = modules(source);

	// Assert
	expect(outer?.body).toEqual([source.indexOf("{"), source.indexOf("}\nmod")]);
	expect([outer?.depth, inner?.depth, last?.depth]).toEqual([0, 1, 0]);
	expect(last?.body).toBeUndefined();
	expect(outer?.start).toBe(source.indexOf("mod tests"));
});

const declaration: Module = {
	name: "tests",
	line: 1,
	path: undefined,
	pathAttribute: undefined,
	testOnly: true,
	start: 0,
	body: undefined,
	depth: 0,
};

test("finds the child file for both module layouts", async () => {
	// Arrange
	const root = await tempRepo({
		"src/parser/tests.rs": "",
		"src/tests.rs": "",
	});

	// Act
	const beside = await childFile(join(root, "src/parser.rs"), declaration);
	const crateRoot = await childFile(join(root, "src/lib.rs"), declaration);

	// Assert
	expect(beside).toBe(join(root, "src/parser/tests.rs"));
	expect(crateRoot).toBe(join(root, "src/tests.rs"));
});

test("a file in a target directory of a package owns its directory", async () => {
	// Arrange
	const root = await tempRepo({
		"Cargo.toml": '[package]\nname = "a"\n',
		"src/bin/x.rs": "",
		"tests/it.rs": "",
		"src/tests/y.rs": "",
	});

	// Act
	const directories = await Promise.all(
		["src/bin/x.rs", "tests/it.rs", "src/tests/y.rs"].map((path) =>
			moduleDirectory(join(root, path)),
		),
	);

	// Assert
	expect(directories).toEqual([
		join(root, "src/bin"),
		join(root, "tests"),
		join(root, "src/tests/y"),
	]);
});

test("a path attribute names a file beside the declaring file", async () => {
	// Arrange
	const root = await tempRepo({ "src/checks.rs": "" });

	// Act
	const found = await childFile(join(root, "src/lib.rs"), {
		...declaration,
		path: "checks.rs",
	});

	// Assert
	expect(found).toBe(join(root, "src/checks.rs"));
});
