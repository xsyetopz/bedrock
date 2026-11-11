type Tokens = readonly string[];

const ASSIGNMENT = /^[A-Za-z_]\w*=/u;
const SEPARATOR = /(\s*(?:&&|\|\||[;|])\s*)/u;
const FIX_FLAGS = new Set(["--fix", "--write"]);
const PRETTIER_CHECK = new Set(["--check", "-c", "--list-different", "-l"]);
const PRETTIER_DROPPED = new Set([...PRETTIER_CHECK, "--write", "-w", "."]);

const withFix = (args: Tokens): string[] =>
	args.some((arg) => FIX_FLAGS.has(arg)) ? ["--fix"] : [];

const runOnBun = (args: Tokens): string[] =>
	args[0] === "watch" ? ["bun", "--watch", ...args.slice(1)] : ["bun", ...args];

const rewriteCommand = (tokens: Tokens): Tokens => {
	const start = tokens.findIndex((token) => !ASSIGNMENT.test(token));
	const handler = TOOLS.get(tokens[start] ?? "");
	return handler === undefined
		? tokens
		: [...tokens.slice(0, start), ...handler(tokens.slice(start + 1))];
};

const execute = (args: Tokens): Tokens => {
	const rest = args.filter((arg) => arg !== "--yes" && arg !== "-y");
	return TOOLS.has(rest[0] ?? "") ? rewriteCommand(rest) : ["bunx", ...rest];
};

const packageManager = (tool: string, [sub = "", ...args]: Tokens): Tokens => {
	if (["ci", "i", "install"].includes(sub)) {
		const frozen =
			sub === "ci" ||
			args.includes("--frozen-lockfile") ||
			args.includes("--immutable");
		return ["bun", "install", ...(frozen ? ["--frozen-lockfile"] : [])];
	}
	if (["add", "publish", "remove"].includes(sub)) {
		return ["bun", sub, ...args];
	}
	if (["run", "run-script"].includes(sub)) {
		return ["bun", "run", ...args];
	}
	if (["t", "test"].includes(sub)) {
		return ["bun", "run", "test", ...args];
	}
	if (["dlx", "exec"].includes(sub)) {
		return execute(args);
	}
	// `pnpm build` and `yarn build` run scripts; npm has no such shorthand.
	return tool !== "npm" && /^[a-z]/u.test(sub)
		? ["bun", "run", sub, ...args]
		: [tool, sub, ...args];
};

const TOOLS: ReadonlyMap<string, (args: Tokens) => Tokens> = new Map<
	string,
	(args: Tokens) => Tokens
>([
	[
		"biome",
		([sub = "", ...args]) =>
			sub === "format"
				? ["bun", "fmt", ...(args.includes("--write") ? [] : ["--check"])]
				: ["typescript-bedrock", "lint", ...withFix(args)],
	],
	["cross-env", (args) => rewriteCommand(args)],
	["esno", runOnBun],
	["eslint", (args) => ["typescript-bedrock", "lint", ...withFix(args)]],
	["jest", (args) => ["bun", "test", ...args]],
	["node", runOnBun],
	["npm", (args) => packageManager("npm", args)],
	["npx", (args) => execute(args)],
	["oxfmt", (args) => ["bun", "fmt", ...args]],
	["pnpm", (args) => packageManager("pnpm", args)],
	["pnpx", (args) => execute(args)],
	[
		"prettier",
		(args) => [
			"bun",
			"fmt",
			...(args.some((arg) => PRETTIER_CHECK.has(arg)) ? ["--check"] : []),
			...args.filter((arg) => !PRETTIER_DROPPED.has(arg)),
		],
	],
	["rimraf", (args) => ["rm", "-rf", ...args]],
	["ts-node", runOnBun],
	["ts-node-dev", runOnBun],
	[
		"tsc",
		(args) =>
			args.includes("--noEmit")
				? ["bun", "check", ...args.filter((arg) => arg !== "--noEmit")]
				: ["tsc", ...args],
	],
	["tsx", runOnBun],
	[
		"vitest",
		(args) => ["bun", "test", ...(args[0] === "run" ? args.slice(1) : args)],
	],
	["yarn", (args) => packageManager("yarn", args)],
]);

export const rewriteScript = (command: string): string =>
	command
		.split(SEPARATOR)
		.map((part, index) => {
			if (index % 2 === 1) {
				return part;
			}
			const tokens = part.trim().split(/\s+/u);
			const rewritten = rewriteCommand(tokens);
			return rewritten === tokens ? part : rewritten.join(" ");
		})
		.join("");
