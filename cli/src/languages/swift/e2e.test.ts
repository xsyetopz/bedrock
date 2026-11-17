import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { guardSwift } from "./guard.ts";
import { adoptSwift, commitAll, contextIn } from "./harness.ts";
import { KINDS } from "./kit.ts";
import { run } from "./shell.ts";

const hasSwift = Bun.which("swift") !== null;
// The app kind is an Xcode project, which SwiftPM cannot build.
const BUILT = KINDS.filter((kind) => kind !== "app");

test.each([...KINDS])(
	"init %s writes a repo that passes the guard files check",
	async (kind) => {
		// Arrange
		const root = await tempRepo();
		await adoptSwift(root, { kind, init: true });
		await commitAll(root);

		// Act
		const found = await guardSwift(contextIn(root, kind), false);

		// Assert
		expect(
			found.filter((finding) => !finding.startsWith("Package.swift:")),
		).toEqual([]);
	},
	60_000,
);

test.skipIf(!hasSwift).each(BUILT)(
	"init %s builds, tests, and passes the guard with SwiftPM",
	async (kind) => {
		// Arrange
		const root = await tempRepo();
		await adoptSwift(root, { kind, init: true });
		await commitAll(root);

		// Act
		const build = await run(["swift", "build", "--disable-sandbox"], root);
		const tests = await run(["swift", "test", "--disable-sandbox"], root);
		const found = await guardSwift(contextIn(root, kind), false);

		// Assert
		expect(build.stderr + build.stdout).toContain("Build complete");
		expect(build.code).toBe(0);
		expect(tests.code).toBe(0);
		expect(found).toEqual([]);
	},
	600_000,
);

test.skipIf(!hasSwift)(
	"init library with cpp builds C++ interop",
	async () => {
		// Arrange
		const root = await tempRepo();
		await adoptSwift(root, { kind: "library", init: true, cxx: true });
		await commitAll(root);

		// Act
		const build = await run(["swift", "build", "--disable-sandbox"], root);
		const tests = await run(["swift", "test", "--disable-sandbox"], root);

		// Assert
		expect(build.code).toBe(0);
		expect(tests.code).toBe(0);
	},
	600_000,
);
