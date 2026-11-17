import { join } from "node:path";

import type { Block } from "../../core/blocks.ts";
import type { Context } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { templateDir } from "../../core/templates.ts";
import {
	adoptedManifest,
	isTypeName,
	newManifest,
	typeName,
} from "./manifest.ts";

/** The kinds `bedrock init` starts; each one includes the library. */
export const KINDS = ["library", "executable", "app"] as const;

/** The kinds chosen on the command line, library alone by default. */
export const kindsOf = (context: Context): Set<string> => {
	const chosen = context.options["kind"];
	const named = typeof chosen === "string" ? chosen.split(",") : [];
	return new Set(named.filter((kind) => kind !== ""));
};

/** The type name of the package a repository directory starts, or an error that says why not. */
export const packageType = (directory: string): string => {
	const name = typeName(directory);
	if (!isTypeName(name)) {
		throw new Error(
			`${directory} cannot name a Swift package: its type name "${name}" must start with a letter and use only ASCII letters and digits. Rename the directory.`,
		);
	}
	return name;
};

const fill = (text: string, name: string): string =>
	text.replaceAll("__Name__", name).replaceAll("__name__", name.toLowerCase());

const treeSteps = async (
	templates: ReturnType<typeof templateDir>,
	root: string,
	name: string,
): Promise<Step[]> => {
	const variables = { typeName: name, lowerName: name.toLowerCase() };
	const paths = await templates.paths();
	const steps = await Promise.all(
		paths.map(async (path): Promise<Step> => ({
			kind: "write",
			path: fill(path, name),
			content: (await templates.read(path, variables)) ?? "",
		})),
	);
	const present = await Promise.all(
		steps.map((step) => Bun.file(join(root, step.path)).exists()),
	);
	return steps.filter((_, index) => present[index] !== true);
};

/**
 * The starter files of the chosen kinds, and a Package.swift when the repository
 * has none. Files the repository already has stay as they are.
 */
export const kitSteps = async (
	context: Context,
	cxx: boolean,
	blocks: readonly Block[],
	hasManifest: boolean,
	directory: string,
): Promise<Step[]> => {
	const kinds = kindsOf(context);
	const name = packageType(directory);
	const trees = [
		context.templates("kit/library"),
		...KINDS.filter((kind) => kind !== "library" && kinds.has(kind)).map(
			(kind) => context.templates(`kit/${kind}`),
		),
		...(cxx ? [templateDir("cpp/kit/cxx")] : []),
	];
	const groups = await Promise.all(
		trees.map((tree) => treeSteps(tree, context.root, name)),
	);
	const readme = await treeSteps(
		context.templates("kit/new"),
		context.root,
		name,
	);
	const manifest: Step[] = hasManifest
		? []
		: [
				{
					kind: "write",
					path: "Package.swift",
					content: adoptedManifest(newManifest(name, kinds, cxx), blocks),
				},
			];
	return [...groups.flat(), ...readme, ...manifest];
};
