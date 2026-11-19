import { expect, test } from "bun:test";
import { join } from "node:path";

import { exists, readText } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import { OLD_BEDROCK } from "./fixtures.ts";
import { guardRust } from "./guard.ts";
import { adoptRust, contextIn } from "./harness.ts";

/** An adopted workspace with one member crate and the given extra files. */
const adopted = async (
	files: Readonly<Record<string, string>>,
): Promise<string> => {
	const root = await tempRepo({
		"Cargo.toml": '[workspace]\nmembers = ["crates/*"]\n',
		"crates/a/Cargo.toml":
			'[package]\nname = "a"\nversion = "0.1.0"\n\n[lints]\nworkspace = true\n',
		"crates/a/src/lib.rs": "//! A.\n",
	});
	await adoptRust(root);
	await Promise.all(
		Object.entries(files).map(([path, content]) =>
			Bun.write(join(root, path), content),
		),
	);
	return root;
};

const locations = (findings: string[]): string[] =>
	findings.map((finding) => finding.split(": ")[0] ?? finding);

test("a freshly initialized project has nothing to report", async () => {
	// Arrange
	const root = await tempRepo();
	await adoptRust(root, true, { kind: "lib,bin,no-std,ffi,wasm" });

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings).toEqual([]);
});

test("reports a test module whose body sits in the file", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs": "#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(locations(findings)).toEqual(["crates/a/src/lib.rs:2"]);
});

test("reports a test module that sets its file with #[path]", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs": '#[cfg(test)]\n#[path = "checks.rs"]\nmod tests;\n',
		"crates/a/src/checks.rs": "#[test]\nfn t() {}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(locations(findings)).toEqual(["crates/a/src/lib.rs:3"]);
});

test("a tests.rs beside its module is what the guard wants", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs": "//! A.\n#[cfg(test)]\nmod tests;\n",
		"crates/a/src/tests.rs": "#[test]\nfn t() {}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings).toEqual([]);
});

test("--fix moves the test module to tests.rs and leaves nothing to report", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs":
			"//! A.\n#[cfg(test)]\nmod tests {\n#[test]\nfn t() {}\n}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), true);

	// Assert
	expect(findings).toEqual([]);
	expect(await readText(join(root, "crates/a/src/lib.rs"))).toBe(
		"//! A.\n#[cfg(test)]\nmod tests;\n",
	);
	expect(await readText(join(root, "crates/a/src/tests.rs"))).toBe(
		"#[test]\nfn t() {}\n",
	);
});

test("--fix reports a module it cannot move once and keeps fixing the other files", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/a.rs": "#[cfg(test)]\nmod checks { }\n",
		"crates/a/src/b.rs": "#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), true);

	// Assert
	expect(findings).toHaveLength(1);
	expect(locations(findings)).toEqual(["crates/a/src/a.rs:2"]);
	expect(await exists(join(root, "crates/a/src/b/tests.rs"))).toBe(true);
});

test("does not take a test module in a comment or string for a declaration", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs":
			'//! A.\n// #[cfg(test)] mod tests;\nconst S: &str = "#[cfg(test)] mod tests;";\n',
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings).toEqual([]);
});

test("reports #[allow] and #[expect] without a reason", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs":
			"//! A.\n#[allow(dead_code)]\nfn a() {}\n#[expect(dead_code)]\nfn b() {}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings.map((finding) => finding.slice(0, 22))).toEqual([
		"crates/a/src/lib.rs:2:",
		"crates/a/src/lib.rs:4:",
	]);
});

test("reports foo/mod.rs, which belongs beside its directory as foo.rs", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/foo/mod.rs": "//! Foo.\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(locations(findings)).toEqual(["crates/a/src/foo/mod.rs"]);
});

test("reports an integration test without #![cfg(test)], and --fix adds it", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/tests/it.rs": "//! Tests.\n#[test]\nfn it() {}\n",
	});

	// Act
	const before = await guardRust(contextIn(root), false);
	const after = await guardRust(contextIn(root), true);

	// Assert
	expect(locations(before)).toEqual(["crates/a/tests/it.rs"]);
	expect(after).toEqual([]);
	expect(await readText(join(root, "crates/a/tests/it.rs"))).toContain(
		"#![cfg(test)]",
	);
});

test("reports a member that skips the workspace lints", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/Cargo.toml": '[package]\nname = "a"\nversion = "0.1.0"\n',
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(locations(findings)).toEqual(["crates/a/Cargo.toml"]);
});

test("reports a leftover xtask only through the core drift check, not twice", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings.join("\n")).not.toContain("xtask: differs");
});
