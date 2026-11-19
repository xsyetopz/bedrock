import { expect, test } from "bun:test";

import { jvmTestFiles } from "./test-files.ts";

test.each([
	["app/src/test/kotlin/MainTest.kt", true],
	["lib/src/testFixtures/java/Fixture.java", true],
	["app/src/testDebug/kotlin/A.kt", true],
	["app/src/androidTest/kotlin/A.kt", true],
	["app/src/androidUnitTest/kotlin/A.kt", true],
	["shared/src/commonTest/kotlin/A.kt", true],
	["src/integrationTest/java/A.java", true],
	["app/src/main/kotlin/Main.kt", false],
	["src/testing/java/A.java", false],
	["src/latest/java/A.java", false],
	["test/A.java", false]
])("%s is in a test source set: %p", async (file, expected) => {
	// Act
	const picked = await jvmTestFiles("", [file]);

	// Assert
	expect(picked.has(file)).toBe(expected);
});
