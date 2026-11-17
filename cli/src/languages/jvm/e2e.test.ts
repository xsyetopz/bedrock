import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { guardJvm } from "./guard.ts";
import { adoptJvm, commitAll, gradleIn } from "./harness.ts";
import { findJdk } from "./java.ts";
import { KINDS } from "./kinds.ts";

const jdk = await findJdk();
const TASKS = ["spotlessCheck", "bedrockCompile", "bedrockLint", "bedrockTest"];
// Native images need GraalVM, Android and Compose need the Android SDK, and
// Kotlin/Native and multiplatform download toolchains: those kinds stop at the guard.
const BUILT = [
	"java-library",
	"kotlin-library",
	"java-application",
	"kotlin-application",
];

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
	},
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
	600_000,
);
