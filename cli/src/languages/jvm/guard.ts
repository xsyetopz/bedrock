import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { Finding } from "../../core/language.ts";

/** Build files of other tools, which the Gradle Kotlin DSL build replaces. */
const FOREIGN: Readonly<Record<string, string>> = {
	"pom.xml": "Maven",
	mvnw: "Maven",
	"mvnw.cmd": "Maven",
	"build.xml": "Ant",
	"ivy.xml": "Ivy",
	"build.sbt": "sbt",
	"build.gradle": "Gradle's Groovy DSL",
	"settings.gradle": "Gradle's Groovy DSL"
};

/** Rules that turn off what bedrock.pro is for, or keep the whole program. */
const WEAKENING =
	/^\s*(-dontshrink|-dontoptimize|-dontobfuscate|-ignorewarnings|-keep\S*\s+class\s+\*\*\s*\{?)/u;
const SUPPRESSION = /@(file:)?Suppress(Warnings)?\(/u;
const OLD_TESTS =
	/^import\s+(org\.junit\.(Test|Assert|Before|After|Rule|runner)|junit\.framework|org\.testng)\b/u;

const nameOf = (path: string): string => path.split("/").at(-1) ?? path;

const pathFindings = (path: string): Finding[] => {
	const name = nameOf(path);
	const tool = FOREIGN[name];
	const found: Finding[] = [];
	if (tool !== undefined) {
		found.push(
			`${path}: ${tool} build file; the Gradle Kotlin DSL build replaces it, so bedrock adopt migrates it`
		);
	}
	if (path.startsWith(".mvn/")) {
		found.push(`${path}: Maven wrapper; ./gradlew replaces it`);
	}
	if (path.endsWith(".gradle") && tool === undefined) {
		found.push(`${path}: Groovy build script; write it as ${path}.kts`);
	}
	return found;
};

const proguardFindings = (path: string, text: string): Finding[] =>
	text
		.split("\n")
		.flatMap((line, index) =>
			WEAKENING.test(line)
				? [
						`${path}:${index + 1}: ${line.trim()} undoes the shrinking and optimizing; keep only the classes reflection needs`
					]
				: []
		);

const sourceFindings = (
	path: string,
	text: string,
	junit6: boolean
): Finding[] => {
	const lines = text.split("\n");
	return lines.flatMap((line, index) => {
		const found: Finding[] = [];
		const reasoned =
			line.includes("//") || lines[index - 1]?.trim().startsWith("//") === true;
		if (SUPPRESSION.test(line) && !reasoned) {
			found.push(
				`${path}:${index + 1}: say why in a // comment on the suppression or the line above it`
			);
		}
		if (junit6 && OLD_TESTS.test(line)) {
			found.push(
				`${path}:${index + 1}: JUnit 4 or TestNG; write the test with JUnit 6, org.junit.jupiter.api`
			);
		}
		return found;
	});
};

/** The directories (with trailing slash) of modules that apply bedrock.android, which tests on JUnit 4. */
const androidModules = (scripts: ReadonlyMap<string, string>): string[] =>
	[...scripts]
		.filter(([, text]) => text.includes('id("bedrock.android")'))
		.map(([path]) => path.slice(0, -"build.gradle.kts".length));

const scriptFindings = (
	path: string,
	text: string,
	hasSettings: boolean
): Finding[] =>
	// An included build, such as gradle/bedrock, has a settings file and cannot apply them.
	hasSettings || text.includes('id("bedrock.')
		? []
		: [
				`${path}: applies no bedrock.* plugin, so it skips the toolchain, Java 8 bytecode, and lints`
			];

const isOwn = (path: string): boolean =>
	!path.startsWith("openspec/") &&
	!path.startsWith("gradle/bedrock/") &&
	!/^gradle\/bedrock[^/]*\.pro$/u.test(path);

/**
 * Reports what the compilers and linters cannot see: foreign build tools,
 * build scripts without a bedrock plugin, proguard rules that disable
 * shrinking, suppressions without a reason, and JUnit 4 tests. Drift of the
 * managed files is core's check.
 */
export const guardJvm = async (root: string): Promise<Finding[]> => {
	const listed = await listFiles(root, new Glob("**/*"));
	const files = listed.filter((path) => isOwn(path));
	const texts = new Map<string, string>();
	const relevant = files.filter((path) => /\.(kt|java|pro|kts)$/u.test(path));
	await Promise.all(
		relevant.map(async (path) => {
			texts.set(path, (await readText(join(root, path))) ?? "");
		})
	);
	const scripts = new Map(
		[...texts].filter(([path]) => nameOf(path) === "build.gradle.kts")
	);
	const android = androidModules(scripts);
	const fileSet = new Set(files);
	return files.flatMap((path) => {
		const text = texts.get(path) ?? "";
		const found = pathFindings(path);
		if (nameOf(path) === "build.gradle.kts") {
			const sibling = path.replace(
				/build\.gradle\.kts$/u,
				"settings.gradle.kts"
			);
			found.push(...scriptFindings(path, text, fileSet.has(sibling)));
		} else if (path.endsWith(".pro")) {
			found.push(...proguardFindings(path, text));
		} else if (/\.(java|kt)$/u.test(path)) {
			const junit4 =
				path.includes("/src/android") ||
				android.some((prefix) => path.startsWith(prefix));
			found.push(...sourceFindings(path, text, !junit4));
		}
		return found;
	});
};
