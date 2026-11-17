import { expect, test } from "bun:test";

import { templateDir } from "../../core/templates.ts";
import {
	editionsOf,
	type Managed,
	memberManifest,
	readManaged,
	rootManifest,
	toolchainFile,
} from "./manifests.ts";

const PARTS = templateDir("rust/parts");
const managed: Managed = readManaged(
	(await PARTS.read("workspace.toml")) ?? "",
);

test("a member with its own lints points at the workspace lints", () => {
	// Arrange
	const manifest = `[package]
name = "a"
edition = "2024"

[lints.rust]
unsafe_code = "forbid"

[dependencies]
serde = "1"
`;

	// Act
	const result = memberManifest(manifest);

	// Assert
	expect(result).toBe(`[package]
name = "a"
edition = "2024"

[lints]
workspace = true

[dependencies]
serde = "1"
`);
});

test("a member that already uses the workspace lints is returned unchanged", () => {
	// Arrange
	const manifest =
		'[package]\nname = "a"\nedition = "2024"\n[lints]\nworkspace   =  true # ok\n';

	// Act
	const result = memberManifest(manifest);

	// Assert
	expect(result).toBe(manifest);
});

test("a member without lints gains them", () => {
	// Arrange
	const manifest = '[package]\nname = "a"\nedition = "2024"\n';

	// Act
	const result = memberManifest(manifest);

	// Assert
	expect(result).toBe(
		'[package]\nname = "a"\nedition = "2024"\n\n[lints]\nworkspace = true\n',
	);
});

test("drops the cargo-husky dev-dependency the hooks replace", () => {
	// Arrange
	const manifest = `[package]
name = "a"
edition = "2024"

[dev-dependencies]
cargo-husky = { version = "1", features = ["user-hooks"] }
`;

	// Act
	const result = memberManifest(manifest);

	// Assert
	expect(result).toBe(
		'[package]\nname = "a"\nedition = "2024"\n\n[lints]\nworkspace = true\n',
	);
});

test("a member manifest that is not TOML is an error", () => {
	// Arrange
	const manifest = "[package\nname = ";

	// Act
	const run = () => memberManifest(manifest);

	// Assert
	expect(run).toThrow("not valid TOML");
});

test("a virtual workspace gains the resolver, the managed lints, and the release profile", () => {
	// Arrange
	const manifest = '[workspace]\nmembers = ["crates/*"]\n';

	// Act
	const result = rootManifest(manifest, managed);
	const parsed = Bun.TOML.parse(result);

	// Assert
	expect(parsed).toMatchObject({
		workspace: { resolver: "3", members: ["crates/*"] },
		profile: { release: { lto: "thin", panic: "abort" } },
	});
	expect(result).toContain("[workspace.lints.clippy]");
});

test("the root manifest is stable: a second pass changes nothing", () => {
	// Arrange
	const once = rootManifest('[workspace]\nmembers = ["a"]\n', managed);

	// Act
	const twice = rootManifest(once, managed);

	// Assert
	expect(twice).toBe(once);
});

test("release profile keys the repo already sets keep their values", () => {
	// Arrange
	const manifest =
		'[workspace]\nmembers = ["a"]\n\n[profile.release]\npanic = "unwind"\nlto = "fat"\n';

	// Act
	const once = rootManifest(manifest, managed);
	const twice = rootManifest(once, managed);

	// Assert
	expect(Bun.TOML.parse(once)).toMatchObject({
		profile: { release: { panic: "unwind", lto: "fat" } },
	});
	expect(twice).toBe(once);
});

test("a workspace with its own lint tables gets the managed ones in their place", () => {
	// Arrange
	const manifest = `[workspace]
members = ["a"]

[workspace.lints.rust]
unused = "warn"

[profile.release]
debug = true
`;

	// Act
	const parsed = Bun.TOML.parse(rootManifest(manifest, managed));

	// Assert
	expect(parsed).toMatchObject({
		workspace: { lints: { rust: { unsafe_code: "deny" } } },
		profile: { release: { debug: true, lto: "thin" } },
	});
});

test("a single-package root becomes a workspace that uses its own lints", () => {
	// Arrange
	const manifest = '[package]\nname = "a"\nversion = "0.1.0"\n';

	// Act
	const parsed = Bun.TOML.parse(rootManifest(manifest, managed));

	// Assert
	expect(parsed).toMatchObject({
		package: { name: "a" },
		lints: { workspace: true },
		workspace: { resolver: "3" },
	});
});

test("the toolchain keeps the targets the project added and adds the new ones once", () => {
	// Arrange
	const template = '[toolchain]\nchannel = "1.99.1"\n';
	const current =
		'[toolchain]\nchannel = "1.80"\ntargets = ["aarch64-apple-ios"]\n';

	// Act
	const result = toolchainFile(template, current, [
		"wasm32-unknown-unknown",
		"aarch64-apple-ios",
	]);

	// Assert
	expect(Bun.TOML.parse(result)).toEqual({
		toolchain: {
			channel: "1.99.1",
			targets: ["aarch64-apple-ios", "wasm32-unknown-unknown"],
		},
	});
});

test("lists the editions a manifest declares", () => {
	// Arrange
	const manifest =
		'[package]\nname = "a"\nedition = "2021"\n[workspace.package]\nedition = "2024"\n';

	// Act
	const editions = editionsOf(manifest);

	// Assert
	expect(editions).toEqual(["2021", "2024"]);
});

test("a package without an edition names the 2015 that Cargo reads", () => {
	// Arrange
	const manifest = '[package]\nname = "a"\n\n[lints]\nworkspace = true\n';

	// Act
	const result = memberManifest(manifest);

	// Assert
	expect(result).toBe(
		'[package]\nname = "a"\nedition = "2015"\n\n[lints]\nworkspace = true\n',
	);
});

test("a package that inherits its edition keeps it", () => {
	// Arrange
	const manifest =
		'[package]\nname = "a"\nedition.workspace = true\n\n[lints]\nworkspace = true\n';

	// Act
	const result = memberManifest(manifest);

	// Assert
	expect(result).toBe(manifest);
});
