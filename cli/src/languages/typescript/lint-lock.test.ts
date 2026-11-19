import { describe, expect, test } from "bun:test";
import { chmod, exists, mkdir } from "node:fs/promises";
import { join } from "node:path";

import { templateDir } from "../../core/templates.ts";
import { tempRepo } from "../../core/testing.ts";

/** Stands in for `bun run oxlint`: logs its start and end around a pause, then exits with FAKE_EXIT. */
const FAKE_BUN = `#!/usr/bin/env bash
echo "start $RUN" >>"$LOG"
sleep 1
echo "end $RUN" >>"$LOG"
exit "\${FAKE_EXIT:-0}"
`;

interface Repo {
	readonly root: string;
	readonly lock: string;
	readonly log: string;
}

/** A repo with the managed typescript.just and a fake bun on PATH. */
const lintRepo = async (): Promise<Repo> => {
	const recipes = await templateDir("typescript/files").read(
		".just/typescript.just",
	);
	const root = await tempRepo({
		".just/typescript.just": recipes ?? "",
		"bin/bun": FAKE_BUN,
	});
	await chmod(join(root, "bin/bun"), 0o755);
	return { root, lock: join(root, "lint.lock"), log: join(root, "lint.log") };
};

/** Starts `just lint` in `repo` as run `run`. */
const lint = (repo: Repo, run: string, fail = false) =>
	Bun.spawn(["just", "-f", join(repo.root, ".just/typescript.just"), "lint"], {
		env: {
			...process.env,
			PATH: `${join(repo.root, "bin")}:${process.env["PATH"] ?? ""}`,
			BEDROCK_LINT_LOCK: repo.lock,
			LOG: repo.log,
			RUN: run,
			FAKE_EXIT: fail ? "1" : "0",
		},
		stderr: "pipe",
		stdout: "ignore",
	});

/** Waits until the file at `path` exists. */
const appears = async (path: string): Promise<void> => {
	if (await exists(path)) {
		return;
	}
	await Bun.sleep(20);
	await appears(path);
};

const logLines = async (repo: Repo): Promise<string[]> => {
	const text = await Bun.file(repo.log).text();
	return text.trim().split("\n");
};

describe.skipIf(Bun.which("just") === null)("the lint lock", () => {
	test("a second run waits for the first to finish", async () => {
		// Arrange
		const repo = await lintRepo();
		const first = lint(repo, "a");
		await appears(repo.log);

		// Act
		const second = lint(repo, "b");
		const codes = await Promise.all([first.exited, second.exited]);

		// Assert
		expect(codes).toEqual([0, 0]);
		expect(await logLines(repo)).toEqual([
			"start a",
			"end a",
			"start b",
			"end b",
		]);
	}, 15_000);

	test("a lock whose holder died is taken over", async () => {
		// Arrange
		const repo = await lintRepo();
		const dead = Bun.spawn(["true"]);
		await dead.exited;
		await mkdir(repo.lock);
		await Bun.write(join(repo.lock, "pid"), `${dead.pid}\n`);

		// Act
		const code = await lint(repo, "a").exited;

		// Assert
		expect(code).toBe(0);
		expect(await logLines(repo)).toEqual(["start a", "end a"]);
	}, 15_000);

	test("a failed run releases the lock", async () => {
		// Arrange
		const repo = await lintRepo();

		// Act
		const code = await lint(repo, "a", true).exited;

		// Assert
		expect(code).not.toBe(0);
		expect(await exists(repo.lock)).toBe(false);
	}, 15_000);
});
