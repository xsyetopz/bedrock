import { expect, test } from "bun:test";
import { mkdir } from "node:fs/promises";
import { join } from "node:path";

import { tempRepo } from "../../core/testing.ts";
import { findJdk } from "./java.ts";

test("a JAVA_HOME with bin/java is used as it is", async () => {
	// Arrange
	const home = await tempRepo();
	await mkdir(join(home, "bin"));
	await Bun.write(join(home, "bin/java"), "");

	// Act
	const jdk = await findJdk({ JAVA_HOME: home });

	// Assert
	expect(jdk).toEqual({ home });
});
