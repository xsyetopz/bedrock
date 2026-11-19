import { expect, test } from "bun:test";

import { $ } from "bun";

import { configPairs, readConfig } from "./config.ts";
import { tempRepo } from "./testing.ts";

const CONFIG = {
	version: "1.0.0",
	languages: { jvm: { group: "com.example.App" }, swift: { cxx: true } },
};

test("readConfig reads back the pairs configPairs writes", async () => {
	// Arrange
	const root = await tempRepo({
		".editorconfig": `root = true\n\n[*]\n${configPairs(CONFIG)}\n`,
	});

	// Act
	const config = await readConfig(root);

	// Assert
	expect(config).toEqual(CONFIG);
});

test("the justfile's sed finds the version configPairs writes", async () => {
	// Arrange
	const root = await tempRepo({
		".editorconfig": `[*]\n${configPairs(CONFIG)}\n`,
	});

	// Act
	const version =
		await $`sed -n 's/^bedrock_version = //p' .editorconfig | tail -n 1`
			.cwd(root)
			.text();

	// Assert
	expect(version).toBe("1.0.0\n");
});

test("readConfig falls back to the .bedrock.json of an older release", async () => {
	// Arrange
	const root = await tempRepo({
		".editorconfig": "root = true\n",
		".bedrock.json": JSON.stringify({ version: "0.5.0", languages: {} }),
	});

	// Act
	const config = await readConfig(root);

	// Assert
	expect(config).toEqual({ version: "0.5.0", languages: {} });
});

test("readConfig finds nothing in a repo bedrock never adopted", async () => {
	// Arrange
	const root = await tempRepo({ ".editorconfig": "[*]\nindent_size = 2\n" });

	// Act
	const config = await readConfig(root);

	// Assert
	expect(config).toBeUndefined();
});
