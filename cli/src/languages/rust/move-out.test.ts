import { expect, test } from "bun:test";
import { chmod, mkdir } from "node:fs/promises";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import { moveOut } from "./move-out.ts";

test("moves the body out and leaves the declaration", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs":
			"fn f() {}\n\n#[cfg(test)]\nmod tests {\n    use super::f;\n\n    #[test]\n    fn t() {\n        f();\n    }\n}\n"
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"fn f() {}\n\n#[cfg(test)]\nmod tests;\n"
	);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"    use super::f;\n\n    #[test]\n    fn t() {\n        f();\n    }\n"
	);
});

test("keeps the relative paths of a moved body pointing at the same files", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs":
			'#[cfg(test)]\nmod tests {\n#[path = "a.rs"]\nmod a;\nconst D: &str = include_str!("d.txt");\n}\n'
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		'#[path = "tests/a.rs"]\nmod a;\nconst D: &str = include_str!("../d.txt");\n'
	);
});

test("moves the body of a crate root beside it", async () => {
	// Arrange
	const root = await tempRepo({
		"src/lib.rs": "#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n"
	});

	// Act
	await moveOut(join(root, "src/lib.rs"));

	// Assert
	expect(await exists(join(root, "src/tests.rs"))).toBe(true);
});

test("moves a test module with another name to a file of that name", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs": "#[cfg(test)]\npub(crate) mod r#support {\npub fn f() {}\n}\n"
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"#[cfg(test)]\npub(crate) mod r#support;\n"
	);
	expect(await Bun.file(join(root, "src/foo/support.rs")).text()).toBe(
		"pub fn f() {}\n"
	);
});

test("moves a test module with the test modules nested in it", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs":
			"#[cfg(test)]\nmod tests {\n    #[cfg(test)]\n    mod deep { }\n}\n"
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"    #[cfg(test)]\n    mod deep { }\n"
	);
});

test("refuses what it cannot move and changes nothing", async () => {
	// Arrange
	const source =
		"mod inner {\n    #[cfg(test)]\n    mod checks { }\n    #[cfg(test)]\n    mod tests { }\n}\n";
	const root = await tempRepo({ "src/foo.rs": source });

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toHaveLength(2);
	expect(problems[0]).toContain(":3:");
	expect(problems[0]).toContain("checks");
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(source);
	expect(await exists(join(root, "src/foo"))).toBe(false);
});

test("does not overwrite a tests.rs that exists", async () => {
	// Arrange
	const root = await tempRepo({
		"src/bar.rs": "#[cfg(test)]\nmod tests { }\n",
		"src/bar/tests.rs": "// mine\n"
	});

	// Act
	const problems = await moveOut(join(root, "src/bar.rs"));

	// Assert
	expect(problems).toHaveLength(1);
	expect(await Bun.file(join(root, "src/bar/tests.rs")).text()).toBe(
		"// mine\n"
	);
});

test("drops the path attribute and moves the file to the default place", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs": '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n',
		"src/checks.rs": "fn t() {}\n"
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"#[cfg(test)]\nmod tests;\n"
	);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"fn t() {}\n"
	);
	expect(await exists(join(root, "src/checks.rs"))).toBe(false);
});

test("only drops the path attribute when the file already sits in the default place", async () => {
	// Arrange
	const root = await tempRepo({
		"src/foo.rs": '#[cfg(test)]\n#[path = "foo/tests.rs"]\nmod tests;\n',
		"src/foo/tests.rs": "fn t() {}\n"
	});

	// Act
	const problems = await moveOut(join(root, "src/foo.rs"));

	// Assert
	expect(problems).toEqual([]);
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(
		"#[cfg(test)]\nmod tests;\n"
	);
	expect(await Bun.file(join(root, "src/foo/tests.rs")).text()).toBe(
		"fn t() {}\n"
	);
});

test("leaves a path attribute whose file names others by relative path", async () => {
	// Arrange
	const source = '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n';
	const root = await tempRepo({
		"src/foo.rs": source,
		"src/checks.rs": 'const D: &str = include_str!("data.txt");\n'
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
		"src/foo/tests.rs": "// mine\n"
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
	expect(await Bun.file(join(root, "src/foo.rs")).text()).toBe(source);
});
