import type { Context } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { withoutXtaskMember } from "./legacy-edits.ts";
import {
	editionsOf,
	memberManifest,
	readManaged,
	rootManifest,
} from "./manifests.ts";
import type { Manifest } from "./project.ts";

/** Steps and notes for part of a plan. */
export interface Part {
	readonly steps: readonly Step[];
	readonly notes: readonly string[];
	/** Paths the steps write that stay the project's own. */
	readonly projectFiles: readonly string[];
}

/** A part with nothing in it. */
export const EMPTY: Part = { steps: [], notes: [], projectFiles: [] };

/** `edit(text)` as a write step, or a note when the TOML does not parse. */
const edited = (
	path: string,
	text: string,
	edit: (text: string) => string,
): Part => {
	try {
		const content = edit(text);
		return {
			steps: [{ kind: "write", path, content }],
			notes: editionNotes(path, content),
			projectFiles: [path],
		};
	} catch (error) {
		const reason = error instanceof Error ? error.message : String(error);
		return { ...EMPTY, notes: [`${path}: ${reason}; left as it is.`] };
	}
};

/** One note when the manifest declares an edition older than 2024, which bedrock does not migrate. */
const editionNotes = (path: string, text: string): string[] => {
	const old = editionsOf(text).filter((edition) => edition !== "2024");
	return old.length === 0
		? []
		: [
				`${path}: edition ${old.join(", ")}; run cargo fix --edition, then raise it to 2024 (bedrock does not migrate editions).`,
			];
};

/**
 * The manifests with the workspace lints, resolver, and release profile. At
 * init, a repository without a `Cargo.toml` starts from the template's.
 */
export const manifestPart = async (
	context: Context,
	found: readonly Manifest[],
	dropXtask: boolean,
): Promise<Part> => {
	const parts = context.templates("parts");
	const managed = readManaged((await parts.read("workspace.toml")) ?? "");
	const root = found.find((manifest) => manifest.dir === ".");
	const text =
		root?.text ?? (context.init ? await parts.read("Cargo.toml") : undefined);
	if (text === undefined) {
		return {
			...EMPTY,
			notes: ["No Cargo.toml at the root; bedrock init starts a project."],
		};
	}
	const own = edited("Cargo.toml", text, (current) =>
		rootManifest(dropXtask ? withoutXtaskMember(current) : current, managed),
	);
	const members = found
		.filter((manifest) => manifest.hasPackage && manifest.dir !== ".")
		.map((manifest) =>
			edited(manifest.path, manifest.text, (current) =>
				memberManifest(current),
			),
		);
	return merge([own, ...members]);
};

/** The parts as one. */
export const merge = (parts: readonly Part[]): Part => ({
	steps: parts.flatMap((part) => part.steps),
	notes: parts.flatMap((part) => part.notes),
	projectFiles: parts.flatMap((part) => part.projectFiles),
});
