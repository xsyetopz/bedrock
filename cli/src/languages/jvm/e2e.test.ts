import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { guardJvm } from "./guard.ts";
import { adoptJvm, commitAll, gradleIn } from "./harness.ts";
import { findJdk } from "./java.ts";
import { KINDS } from "./kinds.ts";

const jdk = await findJdk();
const TASKS = ["spotlessCheck", "bedrockCompile", "bedrockLint", "bedrockTest"];
// Android, Compose, and multiplatform build an Android target, which needs the SDK.
const NEEDS_ANDROID_SDK = new Set(["android", "cmp", "kmp"]);
const hasAndroidSdk = [
	process.env["ANDROID_HOME"],
	process.env["ANDROID_SDK_ROOT"]
].some((dir) => dir !== undefined && dir !== "");
const BUILT = KINDS.filter(
	(kind) => hasAndroidSdk || !NEEDS_ANDROID_SDK.has(kind)
);
const UNBUILT = KINDS.filter((kind) => !BUILT.includes(kind));

test.each([...KINDS])(
	"init %s writes a repo that passes the guard",
	async (kind) => {
		// Arrange
		const root = await tempRepo();

		// Act
		await adoptJvm(root, kind, true);
		await commitAll(root);

		// Assert
		expect(await guardJvm(root)).toEqual([]);
	}
);

test.skipIf(jdk === undefined).each(BUILT)(
	"init %s builds, lints, and tests with Gradle",
	async (kind) => {
		// Arrange
		const root = await tempRepo();
		await adoptJvm(root, kind, true);
		await commitAll(root);

		// Act
		const result = await gradleIn(root, TASKS, jdk?.home);

		// Assert
		expect(result.output).toContain("BUILD SUCCESSFUL");
		expect(result.code).toBe(0);
	},
	// The first build on a fresh machine downloads Gradle, the JDK 25 daemon, GraalVM, and the
	// Android and Compose artifacts; a cold cmp build took 9 minutes on an emulated x64 runner.
	1_200_000
);

// Listed as skipped, so a run without the Android SDK shows which kinds it left out.
test.skip.each(UNBUILT)("init %s builds (needs ANDROID_HOME)", () => {
	expect(process.env["ANDROID_HOME"]).toBeDefined();
});
