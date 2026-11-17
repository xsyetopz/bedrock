import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { gradle, isGradle } from "./gradle.ts";
import { removed, rendered } from "./migrate-helpers.ts";

const GROOVY = `plugins { id 'java'; id 'application' }
group = 'org.acme'
application { mainClass = 'org.acme.Main' }
dependencies {
  implementation 'com.google.guava:guava:33.0.0-jre'
  testImplementation 'junit:junit:4.13'
  implementation project(':core')
}
`;

test("detects a Gradle build and skips one that applies bedrock only", async () => {
	// Arrange
	const plain = await tempRepo({ "build.gradle": GROOVY });
	const none = await tempRepo({ "README.md": "" });

	// Act
	const results = await Promise.all([isGradle(plain), isGradle(none)]);

	// Assert
	expect(results).toEqual([true, false]);
});

test("converts Groovy scripts and drops the root that holds no code", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle": "rootProject.name = 'x'\ninclude ':app', ':core'\n",
		"build.gradle":
			"group = 'org.acme'\nallprojects { repositories { mavenCentral() } }\n",
		"app/build.gradle": GROOVY,
		"core/build.gradle": "plugins { id 'java-library' }\n",
	});

	// Act
	const migration = await gradle(root);
	const files = await rendered(root, migration);

	// Assert
	expect(migration.group).toBe("org.acme");
	expect(removed(migration)).toEqual([
		"app/build.gradle",
		"build.gradle",
		"core/build.gradle",
		"settings.gradle",
	]);
	expect(files["app/build.gradle.kts"]).toContain("libs.guava");
	expect(files["app/build.gradle.kts"]).toContain('project(":core")');
	expect(files["build.gradle.kts"]).toBeUndefined();
	expect([...migration.notes].join("\n")).toContain(
		"allprojects or subprojects",
	);
});

test("leaves a script that applies bedrock as it is", async () => {
	// Arrange
	const root = await tempRepo({
		"settings.gradle.kts": 'include(":lib")\n',
		"lib/build.gradle.kts": 'plugins { id("bedrock.java") }\n',
	});

	// Act
	const migration = await gradle(root);
	const files = await rendered(root, migration);

	// Assert
	expect(migration.modules.map((module) => module.kept)).toEqual([true]);
	expect(files["lib/build.gradle.kts"]).toBeUndefined();
});

test("sends an Android build to a hand migration", async () => {
	// Arrange
	const root = await tempRepo({
		"build.gradle": "plugins { id 'com.android.application' }\n",
	});

	// Act
	const migration = await gradle(root);

	// Assert
	expect(migration.modules).toEqual([]);
	expect([...migration.notes].join("\n")).toContain("apply bedrock.android");
});
