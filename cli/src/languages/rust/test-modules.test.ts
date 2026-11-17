import { expect, test } from "bun:test";
import { chmod, mkdir } from "node:fs/promises";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import { inlineTests, moveOut, pathTests, testsFile } from "./test-modules.ts";

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

test("moves the body out and leaves the declaration", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs":
			"fn f() {}\n\n#[cfg(test)]\nmod tests {\n    use super::f;\n\n    #[test]\n    fn t() {\n        f();\n    }\n}\n",
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"fn f() {}\n\n#[cfg(test)]\nmod tests;\n",
	);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"    use super::f;\n\n    #[test]\n    fn t() {\n        f();\n    }\n",
	);
});

test("moves the body of a crate root beside it", async () => {
	// Arrange
	const root = await tempRepo({
		"src/lib.rs": "#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n",
	});

	// Act
	await moveOut(join(root, "src/lib.rs"));

	// Assert
	expect(await exists(join(root, "src/tests.rs"))).toBe(true);
});

test("refuses what it cannot move and changes nothing", async () => {
	// Arrange
	const source =
		"#[cfg(test)]\nmod checks { }\nmod inner {\n    #[cfg(test)]\n    mod tests { }\n}\n";
	const root = await tempRepo({ "src/foo.rs": source });

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toHaveLength(2);
	expect(problems[0]).toContain(":2:");
	expect(problems[0]).toContain("checks");
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(source);
	expect(await exists(join(root, "src/foo"))).toBe(false);
});

test("does not overwrite a tests.rs that exists", async () => {
	// Arrange
	const root = await tempRepo({
		"src/bar.rs": "#[cfg(test)]\nmod tests { }\n",
		"src/bar/tests.rs": "// mine\n",
	});

	// Act
	const problems = await moveOut(join(root, "src/bar.rs"));

	// Assert
	expect(problems).toHaveLength(1);
	expect(await Bun.file(join(root, "src/bar/tests.rs")).text()).toBe(
		"// mine\n",
	);
});

test("drops the path attribute and moves the file to the default place", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs": '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n',
		"src/checks.rs": "fn t() {}\n",
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"#[cfg(test)]\nmod tests;\n",
	);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"fn t() {}\n",
	);
	expect(await exists(join(root, "src/checks.rs"))).toBe(false);
});

test("only drops the path attribute when the file already sits in the default place", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs": '#[cfg(test)]\n#[path = "foo/tests.rs"]\nmod tests;\n',
		"src/foo/tests.rs": "fn t() {}\n",
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"#[cfg(test)]\nmod tests;\n",
	);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"fn t() {}\n",
	);
});

test("leaves a path attribute whose file names others by relative path", async () => {
	// Arrange
	const source = '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n';
	const root = await tempRepo({
		"src/foo.rs": source,
		"src/checks.rs": 'const D: &str = include_str!("data.txt");\n',
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toHaveLength(1);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(source);
	expect(await exists(join(root, "src/checks.rs"))).toBe(true);
});

test("leaves a path attribute when tests.rs is taken", async () => {
	// Arrange
	const source = '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n';
	const root = await tempRepo({
		"src/foo.rs": source,
		"src/checks.rs": "fn t() {}\n",
		"src/foo/tests.rs": "// mine\n",
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toHaveLength(1);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(source);
});

test("reports a file it cannot read instead of throwing", async () => {
	// Arrange
	const root = await tempRepo();

	// Act
	const problems = await moveOut(join(root, "src/missing.rs"));

	// Assert
	expect(problems).toHaveLength(1);
	expect(problems[0]).toContain("cannot read");
});

test("reports a write that fails and leaves the parent as it was", async () => {
	// Arrange
	const source = "#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n";
	const root = await tempRepo({ "src/foo.rs": source });
	await mkdir(join(root, "src/foo"));
	await chmod(join(root, "src/foo"), 0o500);

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));
	await chmod(join(root, "src/foo"), 0o700);

	// Assert
	expect(problems).toHaveLength(1);
	expect(problems[0]).toContain("cannot write");
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(source);
});
