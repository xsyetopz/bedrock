import { lstat } from "node:fs/promises";
import { join } from "node:path";

import { type Block, type Syntax, upsert } from "./blocks.ts";
import type { Step } from "./steps.ts";

const isRegularFile = async (path: string): Promise<boolean> => {
	try {
		const meta = await lstat(path);
		return meta.isFile();
	} catch {
		return false;
	}
};

const readText = async (path: string): Promise<string | undefined> => {
	const file = Bun.file(path);
	return (await file.exists()) ? file.text() : undefined;
};

/** The block every bedrock writes, then the one bun-bedrock adds. */
const blocksOf = async (filesRoot: string, name: string): Promise<Block[]> => [
	{
		name: "bedrock",
		content: await Bun.file(join(filesRoot, "base/blocks", name)).text(),
	},
	{
		name: "bun-bedrock",
		content: await Bun.file(join(filesRoot, "blocks", name)).text(),
	},
];

const blockFile = async (
	filesRoot: string,
	path: string,
	block: string,
	header: string,
	syntax: Syntax,
	current: string | undefined,
): Promise<Step> => ({
	kind: "write",
	path,
	content: upsert(current, header, syntax, await blocksOf(filesRoot, block)),
});

/** AGENTS.md, its links, and the files every bedrock writes a block into. */
export const sharedSteps = async (
	filesRoot: string,
	target: string,
): Promise<Step[]> => {
	const read = (path: string): Promise<string | undefined> =>
		readText(join(target, path));
	const seed = (await isRegularFile(join(target, "CLAUDE.md")))
		? await read("CLAUDE.md")
		: undefined;
	const dependabot = await read(".github/dependabot.yml");
	const hash = async (
		path: string,
		block: string,
		header = "",
	): Promise<Step> =>
		blockFile(filesRoot, path, block, header, "hash", await read(path));
	return [
		await blockFile(
			filesRoot,
			"AGENTS.md",
			"AGENTS.md",
			"# Agents\n",
			"markdown",
			(await read("AGENTS.md")) ?? seed,
		),
		...["CLAUDE.md", "GEMINI.md"].map((path): Step => ({
			kind: "link",
			path,
			target: "AGENTS.md",
		})),
		await hash(".gitignore", "gitignore"),
		await hash(".gitattributes", "gitattributes"),
		await hash(".editorconfig", "editorconfig", "root = true\n"),
		// A dependabot.yml from before bedrock is replaced, not merged.
		await blockFile(
			filesRoot,
			".github/dependabot.yml",
			"dependabot.yml",
			"version: 2\nupdates:\n",
			"hash",
			dependabot?.includes("# >>> bedrock") === true ? dependabot : undefined,
		),
	];
};
