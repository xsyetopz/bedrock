import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { maven } from "./maven.ts";
import { removed, rendered } from "./migrate-helpers.ts";

const PARENT = `<project><modelVersion>4.0.0</modelVersion>
<groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version>
<packaging>pom</packaging><modules><module>core</module><module>app</module></modules>
<properties><guava.version>33.0.0-jre</guava.version></properties>
<dependencyManagement><dependencies><dependency><groupId>com.google.guava</groupId>
<artifactId>guava</artifactId><version>\${guava.version}</version></dependency></dependencies></dependencyManagement>
</project>`;

const CORE = `<project><parent><groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version></parent>
<artifactId>core</artifactId>
<dependencies><dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId></dependency>
<dependency><groupId>org.slf4j</groupId><artifactId>slf4j-api</artifactId><version>2.0.0</version><scope>provided</scope></dependency>
</dependencies></project>`;

const APP = `<project><parent><groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version></parent>
<artifactId>app</artifactId>
<properties><maven.compiler.release>17</maven.compiler.release><start-class>org.acme.Main</start-class></properties>
<dependencies><dependency><groupId>org.acme</groupId><artifactId>core</artifactId><version>1</version></dependency></dependencies>
</project>`;

const fixture = () =>
	tempRepo({
		"pom.xml": PARENT,
		"core/pom.xml": CORE,
		"app/pom.xml": APP,
		mvnw: "",
		".mvn/wrapper/maven-wrapper.properties": "",
	});

test("maps reactor modules to Gradle projects with managed versions", async () => {
	// Arrange
	const root = await fixture();

	// Act
	const migration = await maven(root);
	const files = await rendered(root, migration);

	// Assert
	expect(migration.group).toBe("org.acme");
	expect(files["core/build.gradle.kts"]).toContain("api(libs.guava)");
	expect(files["core/build.gradle.kts"]).toContain(
		"compileOnly(libs.slf4j.api)",
	);
	expect(files["app/build.gradle.kts"]).toContain(
		'implementation(project(":core"))',
	);
	expect(files["gradle/libs.versions.toml"]).toContain(
		'guava = { module = "com.google.guava:guava", version = "33.0.0-jre" }',
	);
});

test("removes every pom and the Maven wrapper", async () => {
	// Arrange
	const root = await fixture();

	// Act
	const migration = await maven(root);

	// Assert
	expect(removed(migration)).toEqual([
		".mvn",
		"app/pom.xml",
		"core/pom.xml",
		"mvnw",
		"pom.xml",
	]);
});

test("notes a Java level above 8", async () => {
	// Arrange
	const root = await fixture();

	// Act
	const migration = await maven(root);

	// Assert
	expect(migration.notes.size).toBeGreaterThan(0);
	expect(migration.modules.map((module) => module.mainClass)).toEqual([
		undefined,
		"org.acme.Main",
	]);
});

test("a missing module pom is an error", async () => {
	// Arrange
	const root = await tempRepo({ "pom.xml": PARENT });

	// Act
	const message = await maven(root).then(() => "", String);

	// Assert
	expect(message).toContain("core/pom.xml");
});
