import { expect, spyOn, test } from "bun:test";
import { join } from "node:path";

import { readConfig } from "../core/config.ts";
import { tempRepo } from "../core/testing.ts";
import { VERSION } from "../version.ts";
import { adopt } from "./adopt.ts";

const SOURCE_CONFIG = '{\n\t"version": "source",\n\t"languages": {}\n}\n';

test("adopt keeps a repo that runs bedrock from source on its source tree", async () => {
	// Arrange
	const root = await tempRepo({ ".bedrock.json": SOURCE_CONFIG });

	// Act
	const code = await adopt("adopt", [root, "typescript", "--yes", "--no-run"]);

	// Assert
	expect(code).toBe(0);
	expect(await readConfig(root)).toEqual({
		version: "source",
		languages: { typescript: {} },
	});
});

test("adopt pins this release in a repo without a source pin", async () => {
	// Arrange
	const root = await tempRepo({ "package.json": "{}\n" });

	// Act
	await adopt("adopt", [root, "typescript", "--yes", "--no-run"]);

	// Assert
	expect(await Bun.file(join(root, ".bedrock.json")).json()).toEqual({
		version: VERSION,
		languages: { typescript: {} },
	});
});

test("--no-run lists each after-run it skips", async () => {
	// Arrange
	const root = await tempRepo({ "package.json": "{}\n" });
	const printed: string[] = [];
	const info = spyOn(console, "info").mockImplementation((line: string) => {
		printed.push(line);
	});

	// Act
	await adopt("adopt", [root, "typescript", "--yes", "--no-run"]);

	// Assert
	info.mockRestore();
	expect(printed).toContain(
		"  skip    typescript: bun install, then bun fmt (--no-run)",
	);
});

test.skipIf(Bun.which("cargo") === null)(
	"an update with nothing to change finishes the edition migration --no-run skipped",
	async () => {
		// Arrange
		const root = await tempRepo({
			"Cargo.toml": '[package]\nname = "old"\nversion = "0.1.0"\n',
			"src/lib.rs": "",
		});
		await adopt("adopt", [root, "rust", "--yes", "--no-run"]);

		// Act
		const code = await adopt("update", [root, "--yes"]);

		// Assert
		expect(code).toBe(0);
		expect(await Bun.file(join(root, "Cargo.toml")).text()).toContain(
			'edition = "2024"',
		);
	},
	240_000,
);
