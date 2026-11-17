import { expect, test } from "bun:test";
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
