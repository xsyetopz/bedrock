import { parseArgs } from "node:util";

import { readConfig } from "../core/config.ts";
import { editorconfigTree } from "../core/editorconfig/index.ts";
import type { Finding } from "../core/language.ts";
import { checkSize, invalidBounds } from "../core/lizard/index.ts";
import { contextFor, type Selected } from "../core/plan.ts";
import { guardBase } from "./guard-base.ts";
import { select } from "./select.ts";

const USAGE = `Usage: bedrock guard [base|<language>...] [--fix]

Reports what the linters cannot see: suppressions without a reason, files of
replaced tools, managed files that differ from the templates, and each
language's own rules. Without a scope, it checks base and every language
.editorconfig lists. --fix repairs what it can first.`;

/**
 * bedrock-lizard over each selected language with a size gap, under the
 * bounds in `.editorconfig`; prints its warnings and returns its errors.
 */
const guardSizes = async (
	root: string,
	selected: readonly Selected[]
): Promise<Finding[]> => {
	const tree = editorconfigTree(root);
	const reports = await Promise.all(
		selected.flatMap(({ language }) =>
			language.lizard === undefined
				? []
				: [checkSize(root, language.lizard, tree, language.testFiles)]
		)
	);
	// A malformed bound leaves the reports untrustworthy, so it alone is reported.
	const files = await tree.files();
	const invalid = files.flatMap((file) =>
		invalidBounds(
			file.sections.flatMap((section) => section.pairs),
			file.path
		)
	);
	if (invalid.length > 0) {
		return invalid;
	}
	for (const warning of reports.flatMap((report) => report.warnings)) {
		console.warn(`warning: ${warning}`);
	}
	return reports.flatMap((report) => report.errors);
};

/** The scopes to check: those named, or base and every adopted language. */
const scopesOf = (
	positionals: readonly string[],
	selected: readonly Selected[]
): Set<string> => {
	const ids = selected.map((entry) => entry.language.id);
	const scopes = new Set(
		positionals.length === 0 ? ["base", ...ids] : positionals
	);
	const unknown = [...scopes].filter(
		(scope) => scope !== "base" && !ids.includes(scope)
	);
	if (unknown.length > 0) {
		throw new Error(
			`Not adopted here: ${unknown.join(", ")}. Scopes: base, ${ids.join(", ")}.`
		);
	}
	return scopes;
};

/** `bedrock guard`. */
export const guard = async (args: readonly string[]): Promise<number> => {
	const { values, positionals } = parseArgs({
		args: [...args],
		allowPositionals: true,
		options: {
			fix: { type: "boolean", default: false },
			help: { type: "boolean", short: "h", default: false }
		}
	});
	if (values.help) {
		console.info(USAGE);
		return 0;
	}
	const root = process.cwd();
	const config = await readConfig(root);
	if (config === undefined) {
		throw new Error(
			`${root} records no \`bedrock_version\` in \`.editorconfig\`. Run \`bedrock adopt\` first.`
		);
	}
	const selected = select(config, []);
	const scopes = scopesOf(positionals, selected);
	const inScope = selected.filter((entry) => scopes.has(entry.language.id));
	const guarded = Promise.all([
		scopes.has("base") ? guardBase(root, config, selected) : [],
		...inScope.map(({ language, options }) =>
			language.guard(
				contextFor(root, selected, language, options, false),
				values.fix
			)
		)
	]);
	// --fix moves files, such as Rust tests out of the file they test, so the
	// sizes are measured once it is done.
	const sized = async (): Promise<Finding[]> => {
		if (values.fix) {
			await guarded;
		}
		return guardSizes(root, inScope);
	};
	const [found, sizes] = await Promise.all([guarded, sized()]);
	const findings = [...found, sizes];
	for (const finding of findings.flat()) {
		console.error(finding);
	}
	return findings.flat().length === 0 ? 0 : 1;
};
