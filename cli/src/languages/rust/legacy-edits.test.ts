import { expect, test } from "bun:test";

import {
	withoutBedrockAlias,
	withoutBedrockGit,
	withoutXtaskMember,
} from "./legacy-edits.ts";

const ALIAS = '[alias]\nbedrock = "run --quiet --package xtask --"\n';

test("drops the bedrock alias and its empty table, and keeps the rest", () => {
	// Arrange
	const config = `${ALIAS}\n[build]\njobs = 4\n`;

	// Act
	const result = withoutBedrockAlias(config);

	// Assert
	expect(result).toBe("[build]\njobs = 4\n");
});

test("keeps other aliases", () => {
	// Arrange
	const config = `[alias]\nbedrock = "run --quiet --package xtask --"\nb = "build"\n`;

	// Act
	const result = withoutBedrockAlias(config);

	// Assert
	expect(result).toBe('[alias]\nb = "build"\n');
});

test("leaves an alias named bedrock that does not run xtask", () => {
	// Arrange
	const config = '[alias]\nbedrock = "build"\n';

	// Act
	const result = withoutBedrockAlias(config);

	// Assert
	expect(result).toBe(config);
});

test("removes the bedrock git source and its comment from deny.toml", () => {
	// Arrange
	const deny = `[sources]
unknown-git = "deny"
# xtask runs bedrock from git.
allow-git = ["https://github.com/xsyetopz/bedrock"]
`;

	// Act
	const result = withoutBedrockGit(deny);

	// Assert
	expect(result).toBe('[sources]\nunknown-git = "deny"\n');
});

test("keeps the other git sources", () => {
	// Arrange
	const deny = `[sources]
allow-git = ["https://github.com/xsyetopz/bedrock.git", "https://github.com/a/b"]
`;

	// Act
	const result = withoutBedrockGit(deny);

	// Assert
	expect(result).toBe('[sources]\nallow-git = ["https://github.com/a/b"]\n');
});

test("drops the xtask member from an inline list", () => {
	// Arrange
	const manifest = '[workspace]\nmembers = ["crates/*", "xtask"]\n';

	// Act
	const result = withoutXtaskMember(manifest);

	// Assert
	expect(result).toBe('[workspace]\nmembers = ["crates/*"]\n');
});

test("drops the xtask member from a multi-line list", () => {
	// Arrange
	const manifest = '[workspace]\nmembers = [\n  "a",\n  "xtask",\n]\n';

	// Act
	const result = withoutXtaskMember(manifest);

	// Assert
	expect(result).toBe('[workspace]\nmembers = [\n  "a",\n]\n');
});
