/** A bound on one metric: crossing `error` fails the guard, crossing `warning` only reports. */
export interface Bound {
	readonly warning?: number;
	readonly error?: number;
}

/**
 * The metrics bedrock-lizard bounds, each read from `.editorconfig` as
 * `bedrock_<metric>_warning` and `bedrock_<metric>_error`. The language
 * blocks set the defaults: functions over 56 code lines went with failed LLM
 * changes (Chong et al., EASE 2026); 15 is the cyclomatic complexity lizard
 * and the TypeScript config use; 7 parameters is Clippy's
 * `too_many_arguments`. File length only warns at 603: no study shows a
 * file-length optimum, and splitting files has measured costs (Doc2Harness,
 * 2026). A test file, which a language that tells its tests apart by their
 * code names, takes `test_file_code_lines` in place of `file_code_lines`.
 */
export const METRICS = [
	"file_code_lines",
	"test_file_code_lines",
	"function_code_lines",
	"function_complexity",
	"function_parameters"
] as const;

/** One of {@link METRICS}. */
export type Metric = (typeof METRICS)[number];

const LEVELS = ["warning", "error"] as const;

const KEYS: ReadonlyMap<string, readonly [Metric, "warning" | "error"]> =
	new Map(
		METRICS.flatMap((metric) =>
			LEVELS.map(
				(level) => [`bedrock_${metric}_${level}`, [metric, level]] as const
			)
		)
	);

/** The bounds the `bedrock_*` properties of one file set. */
export const boundsOf = (
	properties: ReadonlyMap<string, string>
): Partial<Record<Metric, Bound>> => {
	const bounds: Partial<Record<Metric, Bound>> = {};
	for (const [key, value] of properties) {
		const entry = KEYS.get(key);
		const limit = Number(value);
		if (entry !== undefined && Number.isInteger(limit) && limit > 0) {
			const [metric, level] = entry;
			bounds[metric] = { ...bounds[metric], [level]: limit };
		}
	}
	return bounds;
};

/**
 * What is wrong with the size-bound pairs of an `.editorconfig`: a bound
 * that is not a positive integer or `unset`, or a `bedrock_*_warning` or
 * `bedrock_*_error` key that names no metric; each worded from `file`.
 */
export const invalidBounds = (
	pairs: readonly (readonly [string, string])[],
	file = ".editorconfig"
): string[] =>
	pairs.flatMap(([key, value]) => {
		if (KEYS.has(key)) {
			return /^(?:[1-9]\d*|unset)$/iu.test(value)
				? []
				: [
						`${file}: ${key} = ${value}; a bound is a positive integer or unset`
					];
		}
		return /^bedrock_.+_(?:warning|error)$/u.test(key)
			? [
					`${file}: ${key} names no bound; bedrock bounds ${METRICS.join(", ")}, each with _warning or _error`
				]
			: [];
	});

/** Which bound `value` crosses, if any; an error outranks a warning. */
export const crossed = (
	value: number,
	bound: Bound
):
	| { readonly level: "error" | "warning"; readonly limit: number }
	| undefined => {
	if (bound.error !== undefined && value > bound.error) {
		return { level: "error", limit: bound.error };
	}
	if (bound.warning !== undefined && value > bound.warning) {
		return { level: "warning", limit: bound.warning };
	}
	return undefined;
};
