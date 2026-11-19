import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { Context, Finding } from "../../core/language.ts";
import { TOOL_MANIFESTS } from "./manifests.ts";

/**
 * Configs that would override what `.editorconfig` gives the formatters: a
 * `.csharpierrc` makes CSharpier skip `.editorconfig` altogether, and Fantomas
 * dropped `.fantomasrc` for `.editorconfig` in 4.0.
 */
const FOREIGN_CONFIGS = new Set([
	".csharpierrc",
	".csharpierrc.json",
	".csharpierrc.yaml",
	".csharpierrc.yml",
	".fantomasrc",
]);

/**
 * A C# suppression that gives no reason: a `#pragma warning disable` without a
 * trailing comment, or a `SuppressMessage` attribute without `Justification`.
 */
const csharpFinding = (line: string): string | undefined => {
	if (/^\s*#pragma\s+warning\s+disable\b(?!.*\/\/\s*\S)/u.test(line)) {
		return "#pragma warning disable without a reason; add `// <reason>` after the ids";
	}
	if (/\bSuppressMessage\s*\(/u.test(line) && !line.includes("Justification")) {
		return 'SuppressMessage without Justification; give the reason in Justification = "..."';
	}
	return undefined;
};

/** An F# `#nowarn` that gives no reason in a trailing comment. */
const fsharpFinding = (line: string): string | undefined =>
	/^\s*#nowarn\b(?!.*\/\/\s*\S)/u.test(line)
		? "#nowarn without a reason; add `// <reason>` after the ids"
		: undefined;

/** The findings of one source file, `path:line: message`, one per unexplained suppression. */
export const suppressionFindings = (path: string, text: string): Finding[] => {
	const check =
		path.endsWith(".cs") || path.endsWith(".csx")
			? csharpFinding
			: fsharpFinding;
	return text.split("\n").flatMap((line, index) => {
		const message = check(line);
		return message === undefined
			? []
			: [`${path}:${String(index + 1)}: ${message}`];
	});
};

/**
 * Reports what the SDK analyzers and FSharpLint cannot: a suppression that
 * gives no reason, a formatter config that would shadow `.editorconfig`, and a
 * second tool manifest, which `dotnet tool restore` would read beside the first.
 */
export const guardDotnet = async (context: Context): Promise<Finding[]> => {
	const { root } = context;
	const [sources, all] = await Promise.all([
		listFiles(root, new Glob("**/*.{cs,csx,fs,fsi,fsx}")),
		listFiles(root, new Glob("**")),
	]);
	const texts = await Promise.all(
		sources.map((path) => readText(join(root, path))),
	);
	const suppressions = sources.flatMap((path, index) =>
		suppressionFindings(path, texts[index] ?? ""),
	);
	const configs = all
		.filter((path) => FOREIGN_CONFIGS.has(path.split("/").at(-1) ?? path))
		.map(
			(path) =>
				`${path}: the formatters read their layout from .editorconfig; move the settings there and delete this file`,
		);
	const manifests = TOOL_MANIFESTS.filter((path) => all.includes(path));
	const doubled =
		manifests.length > 1
			? [
					`${manifests.join(", ")}: two tool manifests; keep the tools in ${manifests[0] ?? ""} and delete the other`,
				]
			: [];
	return [...suppressions, ...configs, ...doubled];
};
