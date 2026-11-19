import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { ant } from "./ant.ts";
import { removed, rendered } from "./migrate-helpers.ts";

const IVY = `<ivy-module version="2.0"><info organisation="org.acme" module="x"/>
<dependencies>
<dependency org="com.google.guava" name="guava" rev="33.0.0-jre" conf="compile->default"/>
<dependency org="junit" name="junit" rev="4.13" conf="test->default"/>
</dependencies></ivy-module>`;

const BUILD = `<project name="x" default="jar"><javac srcdir="src" destdir="build" source="11" target="11"/></project>`;

test("reads ivy dependencies and the organisation", async () => {
	// Arrange
	const root = await tempRepo({ "build.xml": BUILD, "ivy.xml": IVY });

	// Act
	const migration = await ant(root);
	const files = await rendered(root, migration);

	// Assert
	expect(migration.group).toBe("org.acme");
	expect(files["build.gradle.kts"]).toContain("libs.guava");
	expect(removed(migration)).toEqual(["build.xml", "ivy.xml"]);
});

test("notes a Java level above 8", async () => {
	// Arrange
	const root = await tempRepo({ "build.xml": BUILD });

	// Act
	const migration = await ant(root);

	// Assert
	expect(migration.notes.size).toBeGreaterThan(0);
});
