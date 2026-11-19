import type { EditorconfigTree } from "../editorconfig/index.ts";
import type { Finding, Language } from "../language.ts";
import { type Bound, boundsOf, crossed, type Metric } from "./limits.ts";
import {
	findLizard,
	LIZARD_VERSION,
	measure,
	type Measures
} from "./measure.ts";

export { invalidBounds } from "./limits.ts";
export { LIZARD_VERSION } from "./measure.ts";

/** What one bedrock-lizard run reports: errors fail the guard, warnings only print. */
export interface LizardReport {
	readonly errors: readonly Finding[];
	readonly warnings: readonly Finding[];
}

interface Hit {
	readonly at: string;
	readonly value: number;
	readonly bound: Bound;
	readonly what: string;
}

/** Sorts each crossed bound into errors or warnings, worded `path:line: <what> is <value>, over <limit>`. */
const report = (hits: readonly Hit[]): LizardReport => {
	const errors: Finding[] = [];
	const warnings: Finding[] = [];
	for (const { at, value, bound, what } of hits) {
		const over = crossed(value, bound);
		if (over !== undefined) {
			(over.level === "error" ? errors : warnings).push(
				`${at}: ${what} is ${String(value)}, over ${String(over.limit)}`
			);
		}
	}
	return { errors, warnings };
};

const WHAT: Readonly<Record<Metric, string>> = {
	file_code_lines: "file code lines",
	test_file_code_lines: "test file code lines",
	function_code_lines: "code lines",
	function_complexity: "cyclomatic complexity",
	function_parameters: "parameters"
};

/**
 * The bounds `measures` cross, each path bounded by the `.editorconfig`
 * properties `propertiesOf` gives it. A file `isTest` picks takes the test
 * file bound in place of the file one.
 */
export const judge = (
	measures: Measures,
	propertiesOf: (path: string) => ReadonlyMap<string, string>,
	isTest: (path: string) => boolean = () => false
): LizardReport => {
	const cache = new Map<string, Partial<Record<Metric, Bound>>>();
	const boundsAt = (path: string): Partial<Record<Metric, Bound>> => {
		let bounds = cache.get(path);
		if (bounds === undefined) {
			bounds = boundsOf(propertiesOf(path));
			cache.set(path, bounds);
		}
		return bounds;
	};
	const hit = (
		at: string,
		path: string,
		metric: Metric,
		value: number,
		name = ""
	): Hit => ({
		at,
		value,
		bound: boundsAt(path)[metric] ?? {},
		what: name === "" ? WHAT[metric] : `${name} ${WHAT[metric]}`
	});
	return report([
		...measures.files.map((file) =>
			hit(
				file.path,
				file.path,
				isTest(file.path) ? "test_file_code_lines" : "file_code_lines",
				file.nloc
			)
		),
		...measures.functions.flatMap((fn) => {
			const at = `${fn.path}:${String(fn.line)}`;
			return [
				hit(at, fn.path, "function_code_lines", fn.nloc, fn.name),
				hit(at, fn.path, "function_complexity", fn.ccn, fn.name),
				hit(at, fn.path, "function_parameters", fn.params, fn.name)
			];
		})
	]);
};

/**
 * Runs bedrock-lizard over `root` for the lizard `languages`, under the bounds
 * the `.editorconfig` files in `tree` set. `testFiles` picks the measured
 * files that take the test file bound.
 */
export const checkSize = async (
	root: string,
	languages: readonly string[],
	tree: EditorconfigTree,
	testFiles?: Language["testFiles"]
): Promise<LizardReport> => {
	const lizard = await findLizard();
	if (lizard === undefined) {
		return {
			errors: [
				`lizard ${LIZARD_VERSION} cannot run: install uv or pipx (just setup does), so bedrock can bound ${languages.join(", ")} sizes`
			],
			warnings: []
		};
	}
	const measures = await measure(lizard, root, languages);
	const paths = [
		...new Set(
			[...measures.files, ...measures.functions].map((entry) => entry.path)
		)
	];
	const properties = new Map(
		await Promise.all(
			paths.map(async (path) => [path, await tree.properties(path)] as const)
		)
	);
	const measured = measures.files.map(({ path }) => path);
	const tests =
		testFiles === undefined
			? new Set<string>()
			: await testFiles(root, measured);
	return judge(
		measures,
		(path) => properties.get(path) ?? new Map(),
		(path) => tests.has(path)
	);
};
