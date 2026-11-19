import { expect, test } from "bun:test";
import { join } from "node:path";

import { readConfig } from "../core/config.ts";
import { tempRepo } from "../core/testing.ts";
import { VERSION } from "../version.ts";
import { adopt } from "./adopt.ts";

const SOURCE_CONFIG = '{\n\t"version": "source",\n\t"languages": {}\n}\n';

test("adopt moves a source pin from .bedrock.json into .editorconfig", async () => {
	// Arrange
	const root = await tempRepo({ ".bedrock.json": SOURCE_CONFIG });

	// Act
	const code = await adopt("adopt", [root, "typescript", "--yes", "--no-run"]);

	// Assert
	expect(code).toBe(0);
	expect(await readConfig(root)).toEqual({
		version: "source",
		languages: { typescript: {} }
	});
	expect(await Bun.file(join(root, ".bedrock.json")).exists()).toBe(false);
});

test("adopt pins this release in a repo without a source pin", async () => {
	// Arrange
	const root = await tempRepo({ "package.json": "{}\n" });

	// Act
	await adopt("adopt", [root, "typescript", "--yes", "--no-run"]);

	// Assert
	expect(await readConfig(root)).toEqual({
		version: VERSION,
		languages: { typescript: {} }
	});
});

test.skipIf(Bun.which("cargo") === null)(
	"an update with nothing to change finishes the edition migration --no-run skipped",
	async () => {
		// Arrange
		const root = await tempRepo({
			"Cargo.toml": '[package]\nname = "old"\nversion = "0.1.0"\n',
			"src/lib.rs": ""
		});
		await adopt("adopt", [root, "rust", "--yes", "--no-run"]);

		// Act
		const code = await adopt("update", [root, "--yes"]);

		// Assert
		expect(code).toBe(0);
		expect(await Bun.file(join(root, "Cargo.toml")).text()).toContain(
			'edition = "2024"'
		);
	},
	240_000
);
