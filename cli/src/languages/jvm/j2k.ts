import { join } from "node:path";

import type { Context, Subcommand } from "../../core/language.ts";
import { findJdk, JDK_MISSING } from "./java.ts";

/** Runs one Gradle task line in the repository with the JDK found, returning its exit code. */
const gradlew = (
	context: Context,
	home: string | undefined,
	args: readonly string[],
): Promise<number> => {
	const env =
		home === undefined ? process.env : { ...process.env, JAVA_HOME: home };
	const child = Bun.spawn(
		[join(context.root, "gradlew"), "--console=plain", ...args],
		{ cwd: context.root, env, stdout: "inherit", stderr: "inherit" },
	);
	return child.exited;
};

/**
 * `bedrock jvm j2k <path>...`: converts Java files, or directories of them,
 * to Kotlin with IntelliJ IDEA's J2K, then formats the result. The first run
 * downloads IntelliJ IDEA, about 1.5 GB; BEDROCK_IDEA points at an installed one.
 */
export const j2k: Subcommand = {
	args: "<path>...",
	summary: "Convert Java files to Kotlin with IntelliJ IDEA's J2K.",
	run: async (context, args) => {
		if (args.length === 0) {
			console.error("jvm j2k takes the Java files or directories to convert");
			return 2;
		}
		const jdk = await findJdk();
		if (jdk === undefined) {
			console.error(JDK_MISSING);
			return 1;
		}
		const converted = await gradlew(context, jdk.home, [
			"j2k",
			...args.map((path) => `--file=${path}`),
		]);
		return converted === 0
			? gradlew(context, jdk.home, ["spotlessApply"])
			: converted;
	},
};
