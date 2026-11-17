import { expect, test } from "bun:test";
import { join } from "node:path";

import { exists, readText } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import { OLD_BEDROCK } from "./fixtures.ts";
import { adoptRust, contextIn } from "./harness.ts";
import { planRust } from "./plan.ts";

test("adopting an old rust-bedrock repo removes the xtask crate, its member entry, the alias, and the git allowance", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);

	// Act
	await adoptRust(root);

	// Assert
	expect(await exists(join(root, "xtask"))).toBe(false);
	expect(await readText(join(root, "Cargo.toml"))).toContain(
		'members = ["crates/*"]',
	);
	expect(await readText(join(root, ".cargo/config.toml"))).toBe(
		"[build]\njobs = 4\n",
	);
	expect(await readText(join(root, "deny.toml"))).toBe(
		'[sources]\nunknown-git = "deny"\n',
	);
});

test("adopting moves the crate onto the workspace lints, the layout, and the gated tests", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);

	// Act
	await adoptRust(root);

	// Assert
	expect(await readText(join(root, "crates/a/Cargo.toml"))).toContain(
		"[lints]\nworkspace = true",
	);
	expect(await exists(join(root, "crates/a/src/foo/mod.rs"))).toBe(false);
	expect(await readText(join(root, "crates/a/src/foo.rs"))).toBe("//! Foo.\n");
	expect(await readText(join(root, "crates/a/tests/it.rs"))).toContain(
		"#![cfg(test)]",
	);
});

test("the module no longer calls cargo bedrock, and guard goes through bedrock", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);

	// Act
	await adoptRust(root);
	const recipes = await readText(join(root, ".just/rust.just"));

	// Assert
	expect(recipes).not.toContain("cargo bedrock");
	expect(recipes).toContain("bedrock guard rust");
	expect(recipes).toContain('set working-directory := ".."');
});

test("a second plan has nothing left to do", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);
	await adoptRust(root);

	// Act
	const again = await adoptRust(root);

	// Assert
	expect(again.steps).toEqual([]);
});

test("an xtask that does not depend on rust-bedrock stays, with its alias, and a note says so", async () => {
	// Arrange
	const own =
		'[package]\nname = "xtask"\nversion = "0.0.0"\n\n[dependencies]\nserde = "1"\n';
	const root = await tempRepo({ ...OLD_BEDROCK, "xtask/Cargo.toml": own });

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.steps.some((step) => step.path === "xtask")).toBe(false);
	expect(plan.notes.join("\n")).toContain(
		"xtask does not depend on rust-bedrock",
	);
});

test("a cargo config that held only the alias is removed", async () => {
	// Arrange
	const root = await tempRepo({
		...OLD_BEDROCK,
		".cargo/config.toml":
			'[alias]\nbedrock = "run --quiet --package xtask --"\n',
	});

	// Act
	await adoptRust(root);

	// Assert
	expect(await exists(join(root, ".cargo/config.toml"))).toBe(false);
});

test("files of replaced tools go, with a note", async () => {
	// Arrange
	const root = await tempRepo({
		"Cargo.toml": "[workspace]\nmembers = []\n",
		"rust-toolchain": "stable\n",
		".rustfmt.toml": "max_width = 80\n",
	});

	// Act
	const plan = await adoptRust(root);

	// Assert
	expect(await exists(join(root, "rust-toolchain"))).toBe(false);
	expect(await exists(join(root, ".rustfmt.toml"))).toBe(false);
	expect(plan.notes.join("\n")).toContain(
		"Removed .rustfmt.toml, rust-toolchain",
	);
});

test("an older edition gets a note instead of a silent change", async () => {
	// Arrange
	const root = await tempRepo({
		"Cargo.toml": '[workspace]\nmembers = ["a"]\n',
		"a/Cargo.toml":
			'[package]\nname = "a"\nversion = "0.1.0"\nedition = "2021"\n',
		"a/src/lib.rs": "",
	});

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.notes.join("\n")).toContain("a/Cargo.toml: edition 2021");
});

test("a project nested in its own workspace is left alone", async () => {
	// Arrange
	const root = await tempRepo({
		"Cargo.toml": "[workspace]\nmembers = []\n",
		"tools/Cargo.toml": '[workspace]\nmembers = ["x"]\n',
		"tools/x/Cargo.toml": '[package]\nname = "x"\nversion = "0.1.0"\n',
	});

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.steps.map((step) => step.path)).not.toContain(
		"tools/x/Cargo.toml",
	);
});

test("a Cargo.toml that is not TOML is left as it is, with a note", async () => {
	// Arrange
	const root = await tempRepo({ "Cargo.toml": "[workspace\n" });

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.steps.map((step) => step.path)).not.toContain("Cargo.toml");
	expect(plan.notes.join("\n")).toContain("Cargo.toml: not valid TOML");
});
