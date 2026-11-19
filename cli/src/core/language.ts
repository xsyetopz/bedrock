import type { LanguageOptions } from "./config.ts";
import type { Step } from "./steps.ts";
import type { TemplateDir, Variables } from "./templates.ts";

/** What a language's planning and checks read. */
export interface Context {
	/** Absolute path of the repository. */
	readonly root: string;
	/** This language's settings from `.editorconfig`, merged with the command line. */
	readonly options: LanguageOptions;
	/** Every adopted language's id, so a language can tell whether, say, Bun is present. */
	readonly languages: readonly string[];
	/** Whether this run is `bedrock init`, which also writes the `kinds/<kind>` starters. */
	readonly init: boolean;
	/** `templates/<id>/<dir>`, such as `files`, `starter`, `blocks`, or `kinds/library`. */
	readonly templates: (dir: string) => TemplateDir;
}

/** What one language adds to an adopt, on top of its `files/`, `starter/`, and `blocks/` templates. */
export interface LanguagePlan {
	/** Steps beyond the template copies: manifest migrations, removed foreign tools, legacy bedrock parts. */
	readonly steps: readonly Step[];
	/** Things adopt could not carry over, which a person reviews. */
	readonly notes: readonly string[];
	/** Paths a step writes that stay the project's own; drift checks skip them. */
	readonly projectFiles: readonly string[];
}

/** One finding from a check: `path:line: message`, or `path: message`. */
export type Finding = string;

/** A language-specific subcommand, run as `bedrock <id> <name> [args]`. */
export interface Subcommand {
	/** The arguments it takes, as the usage line shows them, such as `[<path>...]`. */
	readonly args?: string;
	readonly summary: string;
	readonly run: (context: Context, args: readonly string[]) => Promise<number>;
}

/** Everything bedrock knows about one language. Each lives in `cli/src/languages/<id>`. */
export interface Language {
	/** Directory under `templates/` and `cli/src/languages/`, and the `.just/<id>.just` module name. */
	readonly id: string;
	/** The block name in shared files, from {@link import("./blocks.ts").BLOCK_ORDER}. */
	readonly block: string;
	/** Names accepted on the command line, lower case. */
	readonly aliases: readonly string[];
	/** Paths, or globs such as `Sources/**\/*.cpp`, whose match under the root shows the repo uses the language. */
	readonly markers: readonly string[];
	/** Languages adopted with this one, such as `swift` for `cpp`. */
	readonly requires?: readonly string[];
	/** Executables adopt needs, each with how to install it. */
	readonly tools: Readonly<Record<string, string>>;
	/** Globs of the files this language's checks read; the pre-commit and pre-push hooks run them only when a changed path matches. */
	readonly sources: readonly string[];
	/**
	 * The kinds `bedrock init` can start, each a `templates/<id>/kinds/<kind>`
	 * directory. The chosen ones sit in `options.kind`, comma-separated.
	 */
	readonly kinds: readonly string[];
	/** Placeholder values for this language's templates. */
	readonly variables: (context: Context) => Promise<Variables>;
	/** Plans this language's part of an adopt. */
	readonly plan: (context: Context) => Promise<LanguagePlan>;
	/** Runs after the steps apply, such as `bun install`; skipped with `--no-run`. */
	readonly afterApply?: (context: Context) => Promise<void>;
	/** What {@link afterApply} does, as the plan lists it, such as `bun install, bun fmt`. */
	readonly runs?: string;
	/**
	 * Whether {@link afterApply} has work that a skipped or failed run left,
	 * such as an unfinished edition migration, so a plan with no steps still
	 * runs it.
	 */
	readonly runPending?: (context: Context) => Promise<boolean>;
	/** Reports what the linters cannot see. With `fix`, repairs what it can first. */
	readonly guard: (context: Context, fix: boolean) => Promise<Finding[]>;
	/** Extra commands, such as `swift compile-commands` or `jvm j2k`. */
	readonly subcommands?: Readonly<Record<string, Subcommand>>;
}
