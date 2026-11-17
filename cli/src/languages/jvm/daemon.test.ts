import { expect, test } from "bun:test";

import { templateDir } from "../../core/templates.ts";

const FILES = templateDir("jvm/files");

test("the Gradle daemon runs on the JDK the version catalog pins", async () => {
	// Arrange
	const versions = (await FILES.read("gradle/bedrock.versions.toml")) ?? "";
	const daemon =
		(await FILES.read("gradle/gradle-daemon-jvm.properties")) ?? "";

	// Act
	const pinned = /^jdk = "(\d+)"$/mu.exec(versions)?.[1];
	const asked = /^toolchainVersion=(\d+)$/mu.exec(daemon)?.[1];

	// Assert
	expect(pinned).toBeDefined();
	expect(asked).toBe(pinned);
});
