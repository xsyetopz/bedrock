import { expect, test } from "bun:test";
import { join } from "node:path";

import { tempRepo } from "../../core/testing.ts";
import { inlineTests, pathTests, testsFile } from "./test-modules.ts";

const found = (source: string): [string, number][] =>
	inlineTests(source).map((module) => [module.name, module.line]);

test("finds each inline test module with its line", () => {
	// Arrange
	const source =
		"fn f() {}\n\n#[cfg(test)]\n#[allow(dead_code)]\nmod tests {\n    fn t() {}\n}\n#[cfg(test)]\npub(crate) mod extra { }\n";

	// Act
	const result = found(source);

	// Assert
	expect(result).toEqual([
		["tests", 5],
		["extra", 9],
	]);
});

test("leaves out a test module nested in another, which moves with it", () => {
	// Arrange
	const source =
		"#[cfg(test)]\nmod tests {\n    #[cfg(test)]\n    mod deep { }\n}\n";

	// Act
	const result = found(source);

	// Assert
	expect(result).toEqual([["tests", 2]]);
});

test("allows the out-of-line form and ignores other modules", () => {
	// Arrange
	const source =
		'#[cfg(test)]\nmod tests;\nmod util { }\n#[cfg(not(test))]\nmod live { }\n// #[cfg(test)] mod x { }\nconst S: &str = "#[cfg(test)] mod y { }";\n';

	// Act
	const result = found(source);

	// Assert
	expect(result).toEqual([]);
});

test("finds a path attribute on a test module and nothing else", () => {
	// Arrange
	const source =
		'#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n#[path = "live.rs"]\nmod live;\n#[cfg(test)]\nmod plain;\n';

	// Act
	const result = pathTests(source).map((module) => [module.name, module.line]);

	// Assert
	expect(result).toEqual([["tests", 3]]);
});

test("puts the file beside the roots and in a directory for other modules", async () => {
	// Arrange
	const root = await tempRepo();

	// Act
	const foo = await testsFile(join(root, "src/foo.rs"));
	const roots = await Promise.all(
		["lib.rs", "main.rs", "mod.rs"].map((name) =>
			testsFile(join(root, "src", name)),
		),
	);

	// Assert
	expect(foo).toBe(join(root, "src/foo/tests.rs"));
	expect(roots).toEqual(roots.map(() => join(root, "src/tests.rs")));
	expect(roots).toHaveLength(3);
});
