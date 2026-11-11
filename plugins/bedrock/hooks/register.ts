import type { BashCall, Engine, Hook, On } from "./engine.ts";
import {
	ALIASES,
	CXX_ALIASES,
	type Language,
	LANGUAGES,
	MARKERS,
	NEEDS,
	RUNNERS,
	TOOLS,
} from "./languages.ts";

const REPOSITORY = "https://github.com/xsyetopz/bedrock";
// A package manager in command position: at a line start or after ; & | ( $( or `, past env assignments and sudo, exec, or command.
const FOREIGN =
	/(?:^|[;&|(`]|\$\()\s*(?:\w+=\S*\s+)*(?:(?:sudo|exec|command)\s+)?(npm|npx|pnpm|pnpx|yarn)(?=\s|$)/mu;

const USAGE = [
	"Usage: /adopt [<language>...] [<target>] [--cxx] [--yes]",
	"Languages: typescript (ts, js, bun), rust, swift, cpp (c, c++, objc: Swift with C++20), jvm (java, kotlin, kt, gradle), go (golang).",
	"Without a language, it adopts each one the target has. Without --yes, it prints the plan and changes nothing.",
].join("\n");

export interface Request {
	readonly languages: readonly Language[];
	readonly target: string;
	readonly cxx: boolean;
	readonly apply: boolean;
}

/** Reads `/adopt` arguments, or returns why they don't parse. */
export const parseArgs = (input: string): Request | string => {
	const wanted = new Set<Language>();
	const targets: string[] = [];
	const flags = new Set<string>();
	for (const arg of input.split(/\s+/u).filter((word) => word !== "")) {
		const name = arg.toLowerCase();
		const language = ALIASES.get(name);
		if (arg.startsWith("--")) {
			flags.add(arg);
		} else if (language !== undefined) {
			wanted.add(language);
		} else if (CXX_ALIASES.has(name)) {
			wanted.add("swift");
			flags.add("--cxx");
		} else {
			targets.push(arg);
		}
	}
	const unknown = [...flags].filter(
		(flag) => !["--cxx", "--yes"].includes(flag),
	);
	if (unknown.length > 0) {
		return `Unknown option ${unknown.join(", ")}.\n${USAGE}`;
	}
	if (targets.length > 1) {
		return `Name one target, not ${targets.join(", ")}. A language name that isn't listed reads as a target.\n${USAGE}`;
	}
	return {
		languages: LANGUAGES.filter((language) => wanted.has(language)),
		target: targets[0] ?? ".",
		cxx: flags.has("--cxx"),
		apply: flags.has("--yes"),
	};
};

const absolute = (cwd: string, path: string): string =>
	path.startsWith("/") ? path : `${cwd}/${path}`;

/** The command that runs one bedrock's adopt from the checkout at `source`. */
export const adoptCommand = (
	source: string,
	language: Language,
	request: Request,
): string[] => [
	...RUNNERS[language](source),
	"adopt",
	request.target,
	...(language === "swift" && request.cxx ? ["--cxx"] : []),
	request.apply ? "--yes" : "--dry-run",
];

/**
 * The bedrock repository: the checkout the plugin sits in when it has one,
 * else a clone beside the plugin.
 */
const findSource = async ($: Engine): Promise<string> => {
	const checkout = $.plugin.root.replace(/\/plugins\/[^/]+\/?$/u, "");
	if (
		await $.fs.exists(`${checkout}/packages/typescript-bedrock/package.json`)
	) {
		return checkout;
	}
	const clone = `${$.plugin.root}/.bedrock`;
	if (!(await $.fs.exists(`${clone}/packages`))) {
		const { exitCode, stderr } = await $.process.run(
			["git", "clone", "--quiet", "--depth", "1", REPOSITORY, clone],
			{ timeoutMs: 120_000 },
		);
		if (exitCode !== 0) {
			throw new Error(`git clone ${REPOSITORY} failed: ${stderr.trim()}`);
		}
	}
	return clone;
};

const detect = async (
	$: Engine,
	target: string,
): Promise<readonly Language[]> => {
	const found = await Promise.all(
		LANGUAGES.map(async (language) => {
			const exists = await Promise.all(
				MARKERS[language].map((marker) => $.fs.exists(`${target}/${marker}`)),
			);
			return exists.includes(true);
		}),
	);
	return LANGUAGES.filter((_, index) => found[index] === true);
};

const missingTools = async (
	$: Engine,
	languages: readonly Language[],
): Promise<string[]> => {
	const tools = [
		"git",
		"just",
		...languages.map((language) => NEEDS[language]),
	];
	const found = await Promise.all(
		tools.map(async (tool) => {
			const { exitCode } = await $.process.run(
				["sh", "-c", 'command -v "$1"', "sh", tool],
				{ timeoutMs: 10_000 },
			);
			return exitCode === 0;
		}),
	);
	return tools.filter((_, index) => found[index] === false).toSorted();
};

/** Runs each adopt after the one before, since they edit the same shared files, and stops at a failure. */
const adoptEach = async (
	$: Engine,
	languages: readonly Language[],
	command: (language: Language, request: Request) => string[],
	request: Request,
): Promise<{ sections: readonly string[]; failed: boolean }> => {
	const [language, ...rest] = languages;
	if (language === undefined) {
		return { sections: [], failed: false };
	}
	const { exitCode, stdout, stderr } = await $.process.run(
		command(language, request),
		{ timeoutMs: 600_000 },
	);
	const status = exitCode === 0 ? "" : ` (exited ${exitCode})`;
	const section = `${language}-bedrock${status}:\n${`${stdout}${stderr}`.trim()}`;
	if (exitCode !== 0) {
		return { sections: [section], failed: true };
	}
	const later = await adoptEach($, rest, command, request);
	return { ...later, sections: [section, ...later.sections] };
};

export const runAdopt = async (
	$: Engine,
	input: string,
): Promise<{ text: string; context: string[] }> => {
	const request = parseArgs(input);
	if (typeof request === "string") {
		return { text: request, context: [] };
	}
	const target = absolute(await $.session.cwd(), request.target);
	const languages =
		request.languages.length > 0 ? request.languages : await detect($, target);
	if (languages.length === 0) {
		return {
			text: `${request.target} has no package.json, Cargo.toml, Package.swift, go.mod, or JVM build file; name the languages.\n${USAGE}`,
			context: [],
		};
	}
	const missing = await missingTools($, languages);
	if (missing.length > 0) {
		const text = [
			"Install these, then run /adopt again:",
			...missing.map((tool) => `- ${tool}: ${TOOLS[tool] ?? ""}`),
		].join("\n");
		return { text, context: [`/adopt ${input}:\n${text}`] };
	}
	const source = await findSource($);
	const { sections, failed } = await adoptEach(
		$,
		languages,
		adoptCommand.bind(undefined, source),
		{ ...request, target },
	);
	const named = languages.join(" ");
	const text =
		!failed && !request.apply
			? `${sections.join("\n\n")}\n\nTo apply this plan, run /adopt ${named} ${request.target} --yes`
			: sections.join("\n\n");
	return { text, context: [`/adopt ${input}:\n${text}`] };
};

export const foreignPackageManager = (command: string): string | undefined =>
	FOREIGN.exec(command)?.[1];

export const announce: Hook<unknown> = async ($, e, next) => {
	await $.command.register({
		name: "adopt",
		description:
			"Set a repo up with the bedrocks for its languages. Prints the plan; --yes applies it.",
	});
	return next(e);
};

export const answerAdopt: Hook<{ readonly args: string }> = ($, e) =>
	runAdopt($, e.args);

/** In a repo with `bun.lock`, Bun is the package manager. */
export const blockForeign: Hook<BashCall> = async ($, e, next) => {
	const manager = foreignPackageManager(e.command);
	if (
		manager === undefined ||
		!(await $.fs.exists(`${await $.session.cwd()}/bun.lock`))
	) {
		return next(e);
	}
	return {
		deny: `${$.plugin.name}: this repo uses Bun. Run bun in place of ${manager}, or bunx for a one-off package.`,
	};
};

export const register = (on: On): void => {
	on("session.start", announce);
	on("command.run", { command: "adopt" }, answerAdopt);
	on("tool.call", { tool: "Bash" }, blockForeign);
};
