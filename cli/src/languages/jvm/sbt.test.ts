import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { removed, rendered } from "./migrate-helpers.ts";
import { sbt } from "./sbt.ts";

const SBT = `organization := "org.acme"
libraryDependencies += "com.google.guava" % "guava" % "33.0.0-jre"
lazy val core = (project in file("core"))
  .settings(libraryDependencies += "org.slf4j" % "slf4j-api" % "2.0.0" % Provided)
lazy val app = (project in file("app"))
  .dependsOn(core)
  .settings(Compile / mainClass := Some("org.acme.Main"), libraryDependencies += "org.scala-lang" %% "scala3-library" % "3.3.0")
`;

test("maps each project to a module and its dependsOn to a project dependency", async () => {
	// Arrange
	const root = await tempRepo({
		"build.sbt": SBT,
		"project/build.properties": ""
	});

	// Act
	const migration = await sbt(root);
	const files = await rendered(root, migration);

	// Assert
	expect(migration.group).toBe("org.acme");
	expect(migration.modules.map((module) => module.directory)).toEqual([
		"core",
		"app"
	]);
	expect(files["core/build.gradle.kts"]).toContain("libs.guava");
	expect(files["app/build.gradle.kts"]).toContain('project(":core")');
	expect(removed(migration)).toEqual(["build.sbt", "project"]);
});

test("notes Scala libraries it cannot carry over", async () => {
	// Arrange
	const root = await tempRepo({ "build.sbt": SBT });

	// Act
	const migration = await sbt(root);

	// Assert
	expect(migration.notes.size).toBeGreaterThan(0);
});

test("a single-project build becomes the root module", async () => {
	// Arrange
	const root = await tempRepo({
		"build.sbt": 'libraryDependencies += "junit" % "junit" % "4.13" % Test\n'
	});

	// Act
	const migration = await sbt(root);

	// Assert
	expect(migration.modules.map((module) => module.directory)).toEqual([""]);
});
