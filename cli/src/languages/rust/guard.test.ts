import { expect, test } from "bun:test";
import { join } from "node:path";

import { exists, readText } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import { OLD_BEDROCK } from "./fixtures.ts";
import { guardRust, isTestFileInSrc } from "./guard.ts";
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

test("a freshly initialized project has nothing to report", async () => {
	// Arrange
	const root = await tempRepo();
	await adoptRust(root, true, { kind: "lib,bin,no-std,ffi,wasm" });

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings).toEqual([]);
});

test("reports a test module whose body is in another file", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs": "#[cfg(test)]\nmod tests;\n",
		"crates/a/src/tests.rs": "#[test]\nfn t() {}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings).toContain(
		"crates/a/src/lib.rs:2: mod tests keeps its tests in another file; move them here as `mod tests { … }` (just lint-fix does it)",
	);
	expect(findings.join("\n")).toContain(
		"crates/a/src/tests.rs: unit tests belong inline",
	);
});

test("--fix moves the test module inline, deletes its file, and leaves nothing to report", async () => {
	// Arrange
	const root = await adopted({
		"crates/a/src/lib.rs": "//! A.\n#[cfg(test)]\nmod tests;\n",
		"crates/a/src/tests.rs": "#[test]\nfn t() {}\n",
	});

	// Act
	const findings = await guardRust(contextIn(root), true);

	// Assert
	expect(findings).toEqual([]);
	expect(await readText(join(root, "crates/a/src/lib.rs"))).toBe(
		"//! A.\n#[cfg(test)]\nmod tests {\n#[test]\nfn t() {}\n}\n",
	);
	expect(await exists(join(root, "crates/a/src/tests.rs"))).toBe(false);
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
	expect(findings).toEqual([
		"crates/a/src/foo/mod.rs: name it crates/a/src/foo.rs, beside its directory (bedrock update does it)",
	]);
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
	expect(before).toEqual([
		"crates/a/tests/it.rs: differs from what bedrock writes; run bedrock update",
	]);
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
	expect(findings).toEqual([
		"crates/a/Cargo.toml: differs from what bedrock writes; run bedrock update",
	]);
});

test("reports a leftover xtask only through the core drift check, not twice", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);

	// Act
	const findings = await guardRust(contextIn(root), false);

	// Assert
	expect(findings.join("\n")).not.toContain("xtask: differs");
});

test("knows a tests directory or tests.rs inside src", () => {
	// Arrange
	const paths = [
		"src/tests.rs",
		"crates/a/src/x/tests/y.rs",
		"tests/it.rs",
		"src/lib.rs",
	];

	// Act
	const results = paths.map((path) => isTestFileInSrc(path));

	// Assert
	expect(results).toEqual([true, true, false, false]);
});
