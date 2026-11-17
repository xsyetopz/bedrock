import { expect, test } from "bun:test";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import { buildPlan } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { tempRepo } from "../../core/testing.ts";
import { guardJvm } from "./guard.ts";
import { adoptJvm, textOf } from "./harness.ts";
import { jvm } from "./index.ts";

const OLD_SETTINGS = `// >>> jvm-bedrock
pluginManagement { includeBuild("gradle/bedrock") }
plugins { id("bedrock.settings") }
// <<< jvm-bedrock

rootProject.name = "old"
include(":lib")
`;

test("a repo adopted by an older release loses its manifest and Guard.kt", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle.kts": OLD_SETTINGS,
		"lib/build.gradle.kts": 'plugins { id("bedrock.java") }\n',
		"gradle/bedrock/managed.sha256": "abc  gradlew\n",
		"gradle/bedrock/src/main/kotlin/bedrock/core/Guard.kt":
			"package bedrock.core\n",
	});

	// Act
	await adoptJvm(root);

	// Assert
	expect(await exists(join(root, "gradle/bedrock/managed.sha256"))).toBe(false);
	expect(
		await exists(
			join(root, "gradle/bedrock/src/main/kotlin/bedrock/core/Guard.kt"),
		),
	).toBe(false);
	expect(await exists(join(root, "gradle/bedrock/build.gradle.kts"))).toBe(
		true,
	);
	expect(await textOf(root, "settings.gradle.kts")).toContain(
		'rootProject.name = "old"',
	);
});

test("a Maven project becomes a Gradle build with the plugin block first", async () => {
	// Arrange
	const root = await tempRepo({
		"pom.xml":
			"<project><groupId>org.acme</groupId><artifactId>x</artifactId><version>1</version>" +
			"<dependencies><dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId><version>33.0.0-jre</version></dependency></dependencies></project>",
	});

	// Act
	const plan = await adoptJvm(root);

	// Assert
	expect(await exists(join(root, "pom.xml"))).toBe(false);
	const settings = await textOf(root, "settings.gradle.kts");
	expect(settings.startsWith("// >>> jvm-bedrock")).toBe(true);
	expect(await textOf(root, "gradle.properties")).toContain("group=org.acme");
	expect(await textOf(root, "build.gradle.kts")).toContain("libs.guava");
	expect(plan.projectFiles.has("build.gradle.kts")).toBe(true);
});

test("a repo without a group gets a note", async () => {
	// Arrange
	const root = await tempRepo({ "settings.gradle.kts": "" });

	// Act
	const plan = await adoptJvm(root);

	// Assert
	expect(plan.notes.join("\n")).toContain("gradle.properties: set group=");
});

test("Scala sources are a note, and the rest still adopts", async () => {
	// Arrange
	const root = await tempRepo({ "src/main/scala/A.scala": "object A\n" });

	// Act
	const plan = await adoptJvm(root);

	// Assert
	expect(plan.notes.join("\n")).toContain("not Scala or Groovy");
	expect(await exists(join(root, "gradlew"))).toBe(true);
});

test("a Mill build is a note", async () => {
	// Arrange
	const root = await tempRepo({ "build.mill": "" });

	// Act
	const plan = await adoptJvm(root);

	// Assert
	expect(plan.notes.join("\n")).toContain("cannot read Mill builds");
});

test("init with a kind writes its module, includes it, and passes the guard", async () => {
	// Arrange
	const root = await tempRepo();

	// Act
	await adoptJvm(root, "java-library,kotlin-native", true);

	// Assert
	expect(await textOf(root, "settings.gradle.kts")).toContain(
		'include(":lib", ":app")',
	);
	expect(await exists(join(root, "lib/build.gradle.kts"))).toBe(true);
	expect(await exists(join(root, "app/build.gradle.kts"))).toBe(true);
	expect(await guardJvm(root)).toEqual([]);
});

test("a second adopt changes nothing in the project's own files", async () => {
	// Arrange
	const root = await tempRepo();
	await adoptJvm(root, "kotlin-library", true);
	await Bun.write(join(root, "lib/src/main/kotlin/Own.kt"), "class Own\n");

	// Act
	const again = await adoptJvm(root, "kotlin-library", true);

	// Assert
	expect(again.steps).toEqual([]);
});

test("the group option replaces the group gradle.properties held, with a note", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle.kts": "",
		"gradle.properties": "group=org.old\n",
	});

	// Act
	const plan = await buildPlan(
		root,
		[{ language: jvm, options: { kind: "", group: "org.chosen" } }],
		{ init: false },
	);
	await applySteps(root, plan.steps);

	// Assert
	const properties = await textOf(root, "gradle.properties");
	expect(properties).toContain("group=org.chosen\n");
	expect(properties).not.toContain("org.old");
	expect(plan.notes.join("\n")).toContain("group org.old becomes org.chosen");
});
