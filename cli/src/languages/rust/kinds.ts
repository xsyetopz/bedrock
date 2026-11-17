import { basename } from "node:path";

import type { LanguageOptions } from "../../core/config.ts";
import type { Context } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import type { Variables } from "../../core/templates.ts";

/** What `bedrock init` can start; each has crate templates under `templates/rust/crates`. */
export const KINDS = ["lib", "bin", "no-std", "ffi", "wasm", "bun-ffi"];

/** The Rust target a `wasm` crate builds for. */
export const WASM_TARGET = "wasm32-unknown-unknown";

/** The kinds `rust:<kind>[,<kind>]` named, or a library when it named none. */
export const kindsOf = (options: LanguageOptions): string[] => {
	const listed =
		typeof options["kind"] === "string"
			? options["kind"].split(",").filter((kind) => kind !== "")
			: [];
	return listed.length === 0 ? ["lib"] : [...new Set(listed)];
};

/**
 * The package name for the project: its directory name, in lower case, with characters Cargo
 * rejects replaced by hyphens and a prefix when it would start with a digit.
 */
export const packageName = (root: string): string => {
	// Clippy rejects crate names that are not snake case, so capitals go.
	const cleaned = basename(root)
		.toLowerCase()
		.replaceAll(/[^\w-]+/gu, "-")
		.replaceAll(/^-+|-+$/gu, "");
	if (cleaned === "") {
		return "app";
	}
	return /^\d/u.test(cleaned) ? `app-${cleaned}` : cleaned;
};

/** The placeholder values for one crate. */
export const crateVariables = (pkg: string): Variables => ({
	package: pkg,
	crate: pkg.replaceAll("-", "_"),
});

/** One crate's package name: the project's alone, else `<name>-<kind>`. */
const packageOf = (name: string, kind: string, count: number): string =>
	count === 1 ? name : `${name}-${kind}`;

const templateSteps = async (
	context: Context,
	dir: string,
	target: (path: string) => string,
	variables: Variables,
): Promise<Step[]> => {
	const templates = context.templates(dir);
	const paths = await templates.paths();
	return Promise.all(
		paths.map(async (path): Promise<Step> => ({
			kind: "write",
			path: target(path),
			content: (await templates.read(path, variables)) ?? "",
		})),
	);
};

/** The steps for one crate. A `bun-ffi` crate is an `ffi` crate plus its Bun side at `src/<package>.ts`. */
const crateSteps = async (
	context: Context,
	kind: string,
	pkg: string,
): Promise<Step[]> => {
	const variables = crateVariables(pkg);
	const rust = await templateSteps(
		context,
		`crates/${kind === "bun-ffi" ? "ffi" : kind}`,
		(path) => `crates/${pkg}/${path}`,
		variables,
	);
	return kind === "bun-ffi"
		? [
				...rust,
				...(await templateSteps(
					context,
					"crates/bun-ffi",
					(path) => path.replace("__name__", pkg),
					variables,
				)),
			]
		: rust;
};

/** The crates `bedrock init` creates, and a note where a kind needs another language. */
export const kindSteps = async (
	context: Context,
	kinds: readonly string[],
): Promise<{ steps: Step[]; notes: string[] }> => {
	const name = packageName(context.root);
	const steps = await Promise.all(
		kinds.map((kind) =>
			crateSteps(context, kind, packageOf(name, kind, kinds.length)),
		),
	);
	const needsBun = kinds.includes("bun-ffi");
	return {
		steps: steps.flat(),
		notes:
			needsBun && !context.languages.includes("typescript")
				? [
						"bun-ffi writes its Bun side to src/, which needs Bun: adopt typescript too (bedrock adopt typescript) to lint and test it.",
					]
				: [],
	};
};
