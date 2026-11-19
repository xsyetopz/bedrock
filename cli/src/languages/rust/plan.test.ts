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
		'members = ["crates/*"]'
	);
	expect(await readText(join(root, ".cargo/config.toml"))).toBe(
		"[build]\njobs = 4\n"
	);
	expect(await readText(join(root, "deny.toml"))).toBe(
		'[sources]\nunknown-git = "deny"\n'
	);
});

test("adopting moves the crate onto the workspace lints, the layout, and the gated tests", async () => {
	// Arrange
	const root = await tempRepo(OLD_BEDROCK);

	// Act
	await adoptRust(root);

	// Assert
	expect(await readText(join(root, "crates/a/Cargo.toml"))).toContain(
		"[lints]\nworkspace = true"
	);
	expect(await exists(join(root, "crates/a/src/foo/mod.rs"))).toBe(false);
	expect(await readText(join(root, "crates/a/src/foo.rs"))).toBe("//! Foo.\n");
	expect(await readText(join(root, "crates/a/tests/it.rs"))).toContain(
		"#![cfg(test)]"
	);
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

test("an xtask that does not depend on rust-bedrock stays, with its alias", async () => {
	// Arrange
	const own =
		'[package]\nname = "xtask"\nversion = "0.0.0"\n\n[dependencies]\nserde = "1"\n';
	const root = await tempRepo({ ...OLD_BEDROCK, "xtask/Cargo.toml": own });

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.steps.some((step) => step.path === "xtask")).toBe(false);
});

test("a cargo config that held only the alias is removed", async () => {
	// Arrange
	const root = await tempRepo({
		...OLD_BEDROCK,
		".cargo/config.toml":
			'[alias]\nbedrock = "run --quiet --package xtask --"\n'
	});

	// Act
	await adoptRust(root);

	// Assert
	expect(await exists(join(root, ".cargo/config.toml"))).toBe(false);
});

test("files of replaced tools go", async () => {
	// Arrange
	const root = await tempRepo({
		"Cargo.toml": "[workspace]\nmembers = []\n",
		"rust-toolchain": "stable\n",
		".rustfmt.toml": "max_width = 80\n"
	});

	// Act
	await adoptRust(root);

	// Assert
	expect(await exists(join(root, "rust-toolchain"))).toBe(false);
	expect(await exists(join(root, ".rustfmt.toml"))).toBe(false);
});

test("an older edition gets a note instead of a silent change", async () => {
	// Arrange
	const root = await tempRepo({
		"Cargo.toml": '[workspace]\nmembers = ["a"]\n',
		"a/Cargo.toml":
			'[package]\nname = "a"\nversion = "0.1.0"\nedition = "2021"\n',
		"a/src/lib.rs": ""
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
		"tools/x/Cargo.toml": '[package]\nname = "x"\nversion = "0.1.0"\n'
	});

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.steps.map((step) => step.path)).not.toContain(
		"tools/x/Cargo.toml"
	);
});

test("a Cargo.toml that is not TOML is left as it is", async () => {
	// Arrange
	const root = await tempRepo({ "Cargo.toml": "[workspace\n" });

	// Act
	const plan = await planRust(contextIn(root));

	// Assert
	expect(plan.steps.map((step) => step.path)).not.toContain("Cargo.toml");
});

const MODULE_CRATE = {
	"Cargo.toml": '[workspace]\nmembers = ["crates/*"]\n',
	"crates/a/Cargo.toml":
		'[package]\nname = "a"\nversion = "0.1.0"\n\n[lints]\nworkspace = true\n',
	"crates/a/src/lib.rs": "//! A.\n"
};

test("renaming foo/mod.rs to foo.rs keeps its #[path] and include! targets", async () => {
	// Arrange
	const root = await tempRepo({
		...MODULE_CRATE,
		"crates/a/src/session/mod.rs":
			'#[cfg(test)]\n#[path = "matrix/tests.rs"]\nmod matrix;\ninclude!("data.rs");\n',
		"crates/a/src/session/matrix/tests.rs": "",
		"crates/a/src/session/data.rs": ""
	});

	// Act
	await adoptRust(root);

	// Assert
	expect(await exists(join(root, "crates/a/src/session/mod.rs"))).toBe(false);
	expect(await readText(join(root, "crates/a/src/session.rs"))).toBe(
		'#[cfg(test)]\n#[path = "session/matrix/tests.rs"]\nmod matrix;\ninclude!("session/data.rs");\n'
	);
	const again = await adoptRust(root);
	expect(again.steps.filter((step) => step.path.includes("session"))).toEqual(
		[]
	);
});

test("a foo/mod.rs whose paths cannot be rewritten stays", async () => {
	// Arrange
	const root = await tempRepo({
		...MODULE_CRATE,
		"crates/a/src/odd/mod.rs": 'include!(concat!(env!("OUT_DIR"), "/x.rs"));\n'
	});

	// Act
	await adoptRust(root);

	// Assert
	expect(await exists(join(root, "crates/a/src/odd/mod.rs"))).toBe(true);
	expect(await exists(join(root, "crates/a/src/odd.rs"))).toBe(false);
});
