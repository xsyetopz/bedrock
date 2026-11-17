import { expect, test } from "bun:test";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import {
	childFile,
	declarations,
	inlineTests,
	type OutOfLine,
} from "./modules.ts";

const summary = (source: string): [string, boolean][] =>
	declarations(source).map((found) => [found.name, found.testOnly]);

test("finds test modules behind attributes and visibility", () => {
	// Arrange
	const source =
		"#[cfg(test)]\n#[allow(dead_code)]\npub(crate) mod tests;\nmod util;\n";

	// Act
	const found = declarations(source);

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
	const results = cases.map(([source]) => declarations(source)[0]?.testOnly);

	// Assert
	expect(results).toEqual(cases.map(([, expected]) => expected));
});

test("inline modules, comments, strings, and raw strings declare nothing", () => {
	// Arrange
	const source = [
		"#[cfg(test)] mod tests { }",
		"// #[cfg(test)] mod x;",
		"/* #[cfg(test)] mod z; */",
		'const S: &str = "#[cfg(test)] mod y;";',
		'const R: &str = r#"#[cfg(test)] mod w;"#;',
	].join("\n");

	// Act
	const found = declarations(source);

	// Assert
	expect(found).toEqual([]);
});

test("reads the path attribute", () => {
	// Arrange / Act
	const found = declarations('#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;');

	// Assert
	expect(found[0]?.path).toBe("checks.rs");
});

test("finds the child file for both module layouts", async () => {
	// Arrange
	const root = await tempRepo({
		"src/parser/tests.rs": "",
		"src/tests.rs": "",
	});
	const declaration: OutOfLine = {
		name: "tests",
		written: "tests",
		line: 1,
		path: undefined,
		pathAttribute: undefined,
		testOnly: true,
		span: [0, 0],
	};

	// Act
	const beside = await childFile(join(root, "src/parser.rs"), declaration);
	const crateRoot = await childFile(join(root, "src/lib.rs"), declaration);

	// Assert
	expect(beside).toBe(join(root, "src/parser/tests.rs"));
	expect(crateRoot).toBe(join(root, "src/tests.rs"));
});

test("moves the tests inline at the end of the file and removes their file", async () => {
	// Arrange
	const body =
		'use super::f;\n\n#[test]\nfn raw() {\n    let _ = r"\n  kept\n";\n    let _ = "}{";\n}\n';
	const root = await tempRepo({
		"src/parser.rs": "fn f() {}\n\n#[cfg(test)]\nmod tests;\n",
		"src/parser/tests.rs": body,
	});

	// Act
	const problems = await inlineTests(join(root, "src/parser.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/parser.rs")).text()).toBe(
		`fn f() {}\n\n#[cfg(test)]\nmod tests {\n${body.trimEnd()}\n}\n`,
	);
	expect(await exists(join(root, "src/parser"))).toBe(false);
});

test("drops the path attribute of an inlined module", async () => {
	// Arrange
	const root = await tempRepo({
		"src/parser.rs": '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n',
		"src/checks.rs": "#[test]\nfn t() {}\n",
	});

	// Act
	await inlineTests(join(root, "src/parser.rs"));

	// Assert
	expect(await Bun.file(join(root, "src/parser.rs")).text()).toBe(
		"#[cfg(test)]\nmod tests {\n#[test]\nfn t() {}\n}\n",
	);
	expect(await exists(join(root, "src/checks.rs"))).toBe(false);
});

test("inlines several test modules without shifting each other", async () => {
	// Arrange
	const root = await tempRepo({
		"src/lib.rs": "#[cfg(test)]\nmod a;\nfn mid() {}\n#[cfg(test)]\nmod b;\n",
		"src/a.rs": "fn a() {}\n",
		"src/b.rs": "fn b() {}\n",
	});

	// Act
	await inlineTests(join(root, "src/lib.rs"));

	// Assert
	expect(await Bun.file(join(root, "src/lib.rs")).text()).toBe(
		"#[cfg(test)]\nmod a {\nfn a() {}\n}\nfn mid() {}\n#[cfg(test)]\nmod b {\nfn b() {}\n}\n",
	);
});

test("refuses a child with its own out-of-line modules", async () => {
	// Arrange
	const root = await tempRepo({
		"src/parser.rs": "#[cfg(test)]\nmod tests;\n",
		"src/parser/tests.rs": "mod fixtures;\n",
	});

	// Act
	const problems = await inlineTests(join(root, "src/parser.rs"));

	// Assert
	expect(problems).toHaveLength(1);
	expect(await exists(join(root, "src/parser/tests.rs"))).toBe(true);
});

test("reports a test module whose file is missing", async () => {
	// Arrange
	const root = await tempRepo({
		"src/parser.rs": "#[cfg(test)]\nmod tests;\n",
	});
	await mkdir(join(root, "src/parser"), { recursive: true });

	// Act
	const problems = await inlineTests(join(root, "src/parser.rs"));

	// Assert
	expect(problems[0]).toContain("cannot find the file for mod tests");
});
