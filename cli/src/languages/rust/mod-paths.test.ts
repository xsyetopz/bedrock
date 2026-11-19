import { expect, test } from "bun:test";

import { prefixPaths } from "./mod-paths.ts";

test("a path attribute on an out-of-line module and an include gain the prefix", () => {
	// Arrange
	const source =
		'#[cfg(test)]\n#[path = "matrix/tests.rs"]\nmod matrix;\ninclude!("data.rs");\nconst D: &str = include_str!("d.txt");\n';

	// Act
	const result = prefixPaths(source, "session/");

	// Assert
	expect(result).toBe(
		'#[cfg(test)]\n#[path = "session/matrix/tests.rs"]\nmod matrix;\ninclude!("session/data.rs");\nconst D: &str = include_str!("session/d.txt");\n',
	);
});

test("includes and path attributes can gain different prefixes", () => {
	// Arrange
	const source =
		'#[path = "a.rs"]\nmod a;\nconst D: &str = include_str!("d.txt");\n';

	// Act
	const result = prefixPaths(source, { include: "../", path: "tests/" });

	// Assert
	expect(result).toBe(
		'#[path = "tests/a.rs"]\nmod a;\nconst D: &str = include_str!("../d.txt");\n',
	);
});

test("a source with no relative paths comes back unchanged", () => {
	// Arrange
	const source = 'mod a;\ninclude!("/abs/x.rs");\n// include!("c.rs");\n';

	// Act
	const result = prefixPaths(source, "x/");

	// Assert
	expect(result).toBe(source);
});

test("a path inside an inline module keeps its text", () => {
	// Arrange
	const source = 'mod inner {\n    #[path = "a.rs"]\n    mod a;\n}\n';

	// Act
	const result = prefixPaths(source, "x/");

	// Assert
	expect(result).toBe(source);
});

test("a path it cannot rewrite safely gives up", () => {
	// Arrange
	const cases = [
		'include!(concat!(env!("OUT_DIR"), "/x.rs"));',
		'#[cfg_attr(test, path = "a.rs")]\nmod a;',
		'#[path = "a.rs"]\nmod a { }',
		'#[path = "a\\\\b.rs"]\nmod a;',
		'include!(r"a.rs");',
	];

	// Act
	const results = cases.map((source) => prefixPaths(source, "x/"));

	// Assert
	expect(results.filter((result) => result !== undefined)).toEqual([]);
	expect(results).toHaveLength(cases.length);
});
