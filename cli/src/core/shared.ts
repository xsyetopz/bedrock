import { join } from "node:path";

import { type Block, markers, type Syntax, upsert } from "./blocks.ts";
import { isRegularFile, readText } from "./fs.ts";
import type { Language } from "./language.ts";
import type { Step } from "./steps.ts";
import { isExecutable, templateDir, type Variables } from "./templates.ts";

/** A shared file every part of bedrock writes a block into, keyed by its block template name. */
interface SharedFile {
	readonly path: string;
	readonly syntax: Syntax;
	/** The text a missing file starts as. */
	readonly header: string;
	/** Whether a copy without bedrock's own block is replaced rather than merged. */
	readonly replaceForeign?: boolean;
}

/** The shared files. A language's `blocks/` holds a template for each one it adds to. */
export const SHARED_FILES: Readonly<Record<string, SharedFile>> = {
	"AGENTS.md": { path: "AGENTS.md", syntax: "markdown", header: "# Agents\n" },
	gitignore: { path: ".gitignore", syntax: "hash", header: "" },
	gitattributes: { path: ".gitattributes", syntax: "hash", header: "" },
	editorconfig: {
		path: ".editorconfig",
		syntax: "hash",
		header: "root = true\n",
	},
	// Two update lists for one ecosystem make dependabot fail, so a foreign copy is replaced.
	"dependabot.yml": {
		path: ".github/dependabot.yml",
		syntax: "hash",
		header: "version: 2\nupdates:\n",
		replaceForeign: true,
	},
};

/** Agent instruction files that point at AGENTS.md. */
const AGENT_LINKS = ["CLAUDE.md", "GEMINI.md"];

/** The blocks for one shared file: base first, then each language in block order. */
const blocksFor = async (
	name: string,
	languages: readonly Language[],
	variables: Variables,
): Promise<Block[]> => {
	const parts = [
		{ dir: "base", block: "bedrock" },
		...languages.map((language) => ({
			dir: language.id,
			block: language.block,
		})),
	];
	const contents = await Promise.all(
		parts.map((part) =>
			templateDir(`${part.dir}/blocks`).read(name, variables),
		),
	);
	return parts.flatMap((part, index) => {
		const content = contents[index];
		return content === undefined ? [] : [{ name: part.block, content }];
	});
};

/** AGENTS.md and its links, and every shared file with its blocks. */
export const sharedSteps = async (
	root: string,
	languages: readonly Language[],
	variables: Variables,
): Promise<Step[]> => {
	const files = await Promise.all(
		Object.entries(SHARED_FILES).map(async ([name, file]): Promise<Step> => {
			let current = await readText(join(root, file.path));
			const [open] = markers(file.syntax, "bedrock");
			if (file.replaceForeign === true && current?.includes(open) !== true) {
				current = undefined;
			}
			// A repo that kept its instructions in CLAUDE.md moves them to AGENTS.md.
			if (
				file.path === "AGENTS.md" &&
				current === undefined &&
				(await isRegularFile(join(root, "CLAUDE.md")))
			) {
				current = await readText(join(root, "CLAUDE.md"));
			}
			const blocks = await blocksFor(name, languages, variables);
			return {
				kind: "write",
				path: file.path,
				content: upsert(current, file.header, file.syntax, blocks),
			};
		}),
	);
	return [
		...files,
		...AGENT_LINKS.map((path): Step => ({
			kind: "link",
			path,
			target: "AGENTS.md",
		})),
	];
};

/** The templates under `<dir>`, written over what the repo has. */
export const copySteps = async (
	dir: string,
	variables: Variables,
): Promise<Step[]> => {
	const templates = templateDir(dir);
	const paths = await templates.paths();
	return Promise.all(
		paths.map(async (path): Promise<Step> => ({
			kind: "write",
			path,
			content: (await templates.read(path, variables)) ?? "",
			executable: isExecutable(path),
		})),
	);
};

/** The templates under `<dir>` that the repo does not have yet. */
export const starterSteps = async (
	root: string,
	dir: string,
	variables: Variables,
): Promise<Step[]> => {
	const steps = await copySteps(dir, variables);
	const present = await Promise.all(
		steps.map((step) => Bun.file(join(root, step.path)).exists()),
	);
	return steps.filter((_, index) => present[index] !== true);
};
