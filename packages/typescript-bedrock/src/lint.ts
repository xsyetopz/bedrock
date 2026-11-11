import { mkdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

/** Flags that print information and lint nothing, so they need no lock. */
const QUERY_FLAGS = new Set([
	"-h",
	"--help",
	"-V",
	"--version",
	"--rules",
	"--print-config",
	"--init",
	"--lsp",
]);

const POLL_MS = 500;
/** A path every user and sandbox on the machine sees, so all runs share one lock. */
const LOCK_ROOT = process.platform === "win32" ? tmpdir() : "/tmp";

/** The directory that holds the machine-wide type-aware lint lock. */
export const LOCK_PATH = join(LOCK_ROOT, "typescript-bedrock-lint.lock");

/** Options of {@link withLock}. */
export interface LockOptions {
	/** Receives the line that says why the run waits; prints to stderr by default. */
	onWait?: (message: string) => void;
	/** Milliseconds between two tries; 500 by default. */
	pollMs?: number;
}

/**
 * Whether oxlint runs type-aware rules for these arguments. The shared config
 * turns `typeAware` on, so every run lints with tsgolint except a query.
 */
export const isTypeAware = (args: readonly string[]): boolean =>
	!args.some((arg) => QUERY_FLAGS.has(arg));

const ownerOf = async (path: string): Promise<number | undefined> => {
	const text = await Bun.file(join(path, "pid"))
		.text()
		.catch(() => "");
	const pid = Number(text);
	return Number.isInteger(pid) && pid > 0 ? pid : undefined;
};

const isAlive = (pid: number): boolean => {
	try {
		process.kill(pid, 0);
		return true;
	} catch (error) {
		// EPERM means the process exists and belongs to another user.
		return error instanceof Error && "code" in error && error.code === "EPERM";
	}
};

const isExists = (error: unknown): boolean =>
	error instanceof Error && "code" in error && error.code === "EEXIST";

/** Takes the lock, or returns the pid that holds it. */
const tryAcquire = async (path: string): Promise<number | undefined> => {
	try {
		await mkdir(path);
	} catch (error) {
		if (!isExists(error)) {
			throw error;
		}
		const owner = await ownerOf(path);
		if (owner !== undefined && !isAlive(owner)) {
			await rm(path, { recursive: true, force: true });
			return tryAcquire(path);
		}
		return owner ?? 0;
	}
	await Bun.write(join(path, "pid"), String(process.pid));
	return undefined;
};

/**
 * Runs `task` while holding the lock at `path`. A second caller waits, prints
 * that it waits, and runs once the first finishes; a lock whose owner is gone
 * is broken.
 */
export const withLock = async <T>(
	path: string,
	task: () => Promise<T>,
	{ onWait = console.error, pollMs = POLL_MS }: LockOptions = {},
): Promise<T> => {
	const acquire = async (announced: boolean): Promise<void> => {
		const holder = await tryAcquire(path);
		if (holder === undefined) {
			return;
		}
		if (!announced) {
			onWait(
				`typescript-bedrock: waiting for the type-aware lint that process ${holder} runs; one runs at a time on this machine.`,
			);
		}
		await Bun.sleep(pollMs);
		return acquire(true);
	};
	await acquire(false);
	try {
		return await task();
	} finally {
		await rm(path, { recursive: true, force: true });
	}
};

const oxlintBin = (): string =>
	join(
		dirname(fileURLToPath(import.meta.resolve("oxlint/package.json"))),
		"bin/oxlint",
	);

const runOxlint = (args: readonly string[]): Promise<number> => {
	const child = Bun.spawn(["bun", oxlintBin(), ...args], {
		stdin: "inherit",
		stdout: "inherit",
		stderr: "inherit",
	});
	const stop = () => {
		child.kill();
	};
	process.once("SIGINT", stop);
	process.once("SIGTERM", stop);
	return child.exited;
};

/**
 * Runs oxlint with `args`. A type-aware run takes the machine-wide lock first,
 * because each one loads a whole TypeScript program into memory.
 */
export const lint = (args: readonly string[]): Promise<number> =>
	isTypeAware(args)
		? withLock(LOCK_PATH, () => runOxlint(args))
		: runOxlint(args);
