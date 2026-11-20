/** A fake `Engine` and the machines the plugin hook tests run against. */
import type { Engine, Next, ProcessResult } from "./engine.ts";

export const CHECKOUT = "/src/bedrock";
export const ROOT = `${CHECKOUT}/plugins/bedrock`;
export const INSTALLED = "/home/me/.claude/plugins/cache/bedrock";
export const MAIN = `${CHECKOUT}/cmd/bedrock/main.go`;
export const BUILT = `${CHECKOUT}/bin/bedrock`;
export const BUILD = [
	"go",
	"-C",
	CHECKOUT,
	"build",
	"-o",
	BUILT,
	"./cmd/bedrock"
];

export const OK: ProcessResult = { exitCode: 0, stdout: "", stderr: "" };

interface Fake {
	readonly root?: string;
	readonly files?: readonly string[];
	readonly calls?: string[][];
	/** Answers one process call; the default succeeds with no output, and a throw rejects the call as a program that can't start does. */
	readonly answer?: (argv: readonly string[]) => ProcessResult;
}

export const engine = ({
	root = ROOT,
	files = [MAIN],
	calls = [],
	answer = () => OK
}: Fake = {}): Engine => ({
	plugin: { name: "bedrock", root },
	command: { register: () => Promise.resolve({}) },
	fs: { exists: (path) => Promise.resolve(files.includes(path)) },
	process: {
		run: (argv) => {
			calls.push([...argv]);
			return new Promise((resolve) => {
				resolve(answer(argv));
			});
		}
	},
	session: { cwd: () => Promise.resolve("/work") }
});

export const next =
	<E>(): Next<E> =>
	() =>
		Promise.resolve("ran");

/** The machine and PATH a plugin installed alone sees. */
export const machine =
	(onPath: string) =>
	(argv: readonly string[]): ProcessResult => {
		const replies: Record<string, string> = {
			"bedrock version": onPath,
			"uname -m": "arm64",
			"uname -s": "Darwin"
		};
		return { ...OK, stdout: replies[argv.join(" ")] ?? "" };
	};

/** A machine with no `bedrock` on PATH: Claude Code rejects a run of a program it can't start. */
export const withoutBedrock = (argv: readonly string[]): ProcessResult => {
	if (argv[0] === "bedrock") {
		throw new Error(
			'$.process.run(bedrock) failed to start: ENOENT: Executable not found in $PATH: "bedrock"'
		);
	}
	return machine("")(argv);
};

/** A machine where the release download fails with `stderr`. */
export const offline =
	(stderr: string) =>
	(argv: readonly string[]): ProcessResult =>
		argv[0] === "sh" ? { exitCode: 1, stdout: "", stderr } : machine("")(argv);
