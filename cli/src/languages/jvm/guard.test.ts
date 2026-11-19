import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { guardJvm } from "./guard.ts";

const BUILD = 'plugins { id("bedrock.kotlin") }\n';

test("a clean project has no findings", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle.kts": "",
		"lib/build.gradle.kts": BUILD,
		"lib/src/Main.kt": '// the reason\n@Suppress("x") class A\n',
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found).toEqual([]);
});

test("reports build files of other tools", async () => {
	// Arrange
	const root = await tempRepo({
		"pom.xml": "",
		mvnw: "",
		".mvn/wrapper/x.properties": "",
		"app/build.gradle": "",
		"build.sbt": "",
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found.map((finding) => finding.split(":")[0])).toEqual([
		".mvn/wrapper/x.properties",
		"app/build.gradle",
		"build.sbt",
		"mvnw",
		"pom.xml",
	]);
});

test("reports a build script without a bedrock plugin", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle.kts": "",
		"lib/build.gradle.kts": 'plugins { id("java") }\n',
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found).toHaveLength(1);
	expect(found[0]?.startsWith("lib/build.gradle.kts: ")).toBe(true);
});

test("reports proguard rules that undo shrinking", async () => {
	// Arrange
	const root = await tempRepo({
		"app/proguard-rules.pro":
			"-keep class Foo\n-dontshrink\n-keep class ** { *; }\n",
		"gradle/bedrock.pro": "-dontobfuscate\n",
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found.map((finding) => finding.slice(0, 26))).toEqual([
		"app/proguard-rules.pro:2: ",
		"app/proguard-rules.pro:3: ",
	]);
});

test("reports a suppression without a reason", async () => {
	// Arrange
	const root = await tempRepo({
		"src/A.java": '@SuppressWarnings("x") // why\nclass A {}\n',
		"src/B.java": '@SuppressWarnings("x")\nclass B {}\n',
		"src/C.kt": '@file:Suppress("x")\n',
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found.map((finding) => finding.split(": ")[0])).toEqual([
		"src/B.java:1",
		"src/C.kt:1",
	]);
});

test("reports JUnit 4 tests except in Android modules", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle.kts": "",
		"lib/build.gradle.kts": BUILD,
		"lib/src/test/A.java": "import org.junit.Test;\n",
		"app/build.gradle.kts": 'plugins { id("bedrock.android") }\n',
		"app/src/test/B.kt": "import org.junit.Test\n",
		"shared/src/android/C.kt": "import org.junit.Assert\n",
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found.map((finding) => finding.split(": ")[0])).toEqual([
		"lib/src/test/A.java:1",
	]);
});

test("skips openspec and the managed gradle/bedrock build", async () => {
	// Arrange
	const root = await tempRepo({
		"openspec/pom.xml": "",
		"gradle/bedrock/build.gradle.kts": "plugins {}\n",
		"gradle/bedrock.pro": "-dontshrink\n",
	});

	// Act
	const found = await guardJvm(root);

	// Assert
	expect(found).toEqual([]);
});
