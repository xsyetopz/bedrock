import { basename } from "node:path";

import type { Context, Language } from "../../core/language.ts";
import { guardJvm } from "./guard.ts";
import { j2k } from "./j2k.ts";
import { findJdk } from "./java.ts";
import { KINDS, packageName } from "./kinds.ts";
import { groupOf, planJvm } from "./plan.ts";

/** Runs `./gradlew --version` when a JDK is found, which downloads the Gradle distribution; a failure never fails the adopt. */
const afterApply = async (context: Context): Promise<void> => {
	try {
		const jdk = await findJdk();
		if (
			jdk === undefined ||
			!(await Bun.file(`${context.root}/gradlew`).exists())
		) {
			return;
		}
		const env =
			jdk.home === undefined
				? process.env
				: { ...process.env, JAVA_HOME: jdk.home };
		const child = Bun.spawn(["./gradlew", "--version"], {
			cwd: context.root,
			env,
			stdout: "ignore",
			stderr: "ignore",
		});
		await child.exited;
	} catch {
		// The recipes run Gradle again and report what is missing.
	}
};

/** Java and Kotlin on Gradle: the bedrock.* convention plugins, Maven, sbt, Ant, and Gradle migrations, and J2K. */
export const jvm: Language = {
	id: "jvm",
	block: "jvm-bedrock",
	aliases: ["jvm", "java", "kotlin"],
	markers: [
		"pom.xml",
		"build.gradle",
		"build.gradle.kts",
		"settings.gradle",
		"settings.gradle.kts",
		"build.sbt",
		"build.xml",
		"ivy.xml",
		"gradlew",
	],
	tools: {},
	sources: [
		"**/*.java",
		"**/*.kt",
		"**/*.kts",
		"**/*.pro",
		"gradle/**",
		"gradle.properties",
	],
	kinds: KINDS,
	variables: (context) => {
		const group =
			groupOf(context) ?? `com.example.${packageName(basename(context.root))}`;
		return Promise.resolve({
			package: group,
			path: group.replaceAll(".", "/"),
		});
	},
	plan: planJvm,
	afterApply,
	runs: "./gradlew --version, which fetches Gradle",
	guard: (context) => guardJvm(context.root),
	subcommands: { j2k },
};
