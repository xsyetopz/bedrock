import { join } from "node:path";

import { exists, isRegularFile, readText } from "../../core/fs.ts";
import { inOrder } from "../../core/order.ts";
import {
	type Dependency,
	javaLevelNote,
	type Migration,
	type Module,
	newMigration,
	newModule,
} from "./model.ts";

const QUOTED = String.raw`['"]([^'"]+)['"]`;
const INCLUDE = /^\s*include\s*\(?(.+?)\)?\s*$/gmu;
const DEPENDENCY = /^\s*(\w+)\s*[ (]\s*(.+?)\s*$/u;
const COORDINATES = new RegExp(String.raw`^\(?${QUOTED}\)?$`, "u");
const MAP = new RegExp(
	String.raw`group\s*[:=]\s*${QUOTED}\s*,\s*name\s*[:=]\s*${QUOTED}(?:\s*,\s*version\s*[:=]\s*${QUOTED})?`,
	"u",
);
const PROJECT = new RegExp(
	String.raw`project\s*\(\s*(?:path\s*[:=]\s*)?${QUOTED}\s*\)`,
	"u",
);
const PLATFORM = new RegExp(
	String.raw`^\(?(?:enforced)?[pP]latform\s*\(\s*${QUOTED}\s*\)\)?$`,
	"u",
);
const CATALOG = /^\(?(libs(?:\.\w+)+)\)?$/u;
const MAIN_CLASS = new RegExp(
	String.raw`mainClass(?:Name)?(?:\s*=\s*|\.set\(\s*|\s+)${QUOTED}`,
	"u",
);
const GROUP = new RegExp(String.raw`^group\s*=?\s*${QUOTED}`, "mu");
const LEVEL =
	/(?:sourceCompatibility|targetCompatibility|jvmTarget)\s*=\s*['"]?(?:JavaVersion\.VERSION_)?([\d_.]+)|(?:jvmToolchain|JavaLanguageVersion\.of)\s*\(\s*(\d+)/u;

/** Gradle's own names for configurations that Gradle 7 removed, and the ones it keeps. */
const CONFIGURATIONS: Readonly<Record<string, string>> = {
	compile: "api",
	testCompile: "testImplementation",
	runtime: "runtimeOnly",
	testRuntime: "testRuntimeOnly",
	...Object.fromEntries(
		[
			"api",
			"implementation",
			"compileOnly",
			"compileOnlyApi",
			"runtimeOnly",
			"annotationProcessor",
			"testImplementation",
			"testCompileOnly",
			"testRuntimeOnly",
			"testAnnotationProcessor",
		].map((name) => [name, name]),
	),
};

/** Whether `root` has a Gradle build that applies no bedrock plugin yet. */
export const isGradle = async (root: string): Promise<boolean> => {
	const groovy = await Promise.all(
		["settings.gradle", "build.gradle"].map((name) =>
			isRegularFile(join(root, name)),
		),
	);
	if (groovy.includes(true)) {
		return true;
	}
	const kotlin = await Promise.all(
		["settings.gradle.kts", "build.gradle.kts"].map((name) =>
			readText(join(root, name)),
		),
	);
	return kotlin.some(
		(text) => text !== undefined && !text.includes("bedrock."),
	);
};

const libraryOf = (
	configuration: string,
	coordinates: string,
): Dependency | undefined => {
	const [group, name, version] = coordinates.split(":");
	return group === undefined || name === undefined
		? undefined
		: { kind: "library", configuration, group, name, version };
};

/** The dependency one line of a `dependencies {}` block means, when it is a form adopt reads. */
const dependencyOf = (
	configuration: string,
	argument: string,
): Dependency | undefined => {
	const coordinates = COORDINATES.exec(argument)?.[1];
	if (coordinates !== undefined) {
		return libraryOf(configuration, coordinates);
	}
	const map = MAP.exec(argument);
	if (map?.[1] !== undefined && map[2] !== undefined) {
		return {
			kind: "library",
			configuration,
			group: map[1],
			name: map[2],
			version: map[3],
		};
	}
	const project = PROJECT.exec(argument)?.[1];
	if (project !== undefined) {
		return {
			kind: "module",
			configuration,
			path: `:${project.replaceAll(/^:+|:+$/gu, "")}`,
		};
	}
	const platform = PLATFORM.exec(argument)?.[1];
	const library =
		platform === undefined ? undefined : libraryOf(configuration, platform);
	if (library?.kind === "library") {
		return { kind: "platform", configuration, library };
	}
	const catalog = CATALOG.exec(argument)?.[1];
	return catalog === undefined
		? undefined
		: { kind: "notation", configuration, text: catalog };
};

const readDependencies = (
	text: string,
	path: string,
	migration: Migration,
): Dependency[] => {
	const block = /^dependencies\s*\{\s*\n(.*?)^\}/msu.exec(text)?.[1] ?? "";
	const found: Dependency[] = [];
	// The bedrock plugins add the Kotlin standard library and test library.
	const lines = block
		.split("\n")
		.map((line) => line.trim())
		.filter(
			(line) =>
				line !== "" && !line.startsWith("//") && !line.includes("kotlin("),
		);
	for (const line of lines) {
		const match = DEPENDENCY.exec(line);
		const configuration =
			match?.[1] === undefined ? undefined : CONFIGURATIONS[match[1]];
		const dependency =
			configuration === undefined || match?.[2] === undefined
				? undefined
				: dependencyOf(configuration, match[2]);
		if (dependency === undefined) {
			migration.notes.add(
				`${path}: kept out "${line}"; add it to the new build by hand`,
			);
		} else {
			found.push(dependency);
		}
	}
	return found;
};

/** The project directories a settings file includes. */
const includedDirectories = (settings: string | undefined): string[] => {
	const lists = [...(settings ?? "").matchAll(INCLUDE)]
		.map((line) => line[1] ?? "")
		.join(" ");
	return [...lists.matchAll(new RegExp(QUOTED, "gu"))].map((quoted) =>
		(quoted[1] ?? "").replaceAll(/^:+|:+$/gu, "").replaceAll(":", "/"),
	);
};

/**
 * Converts a Gradle build in either DSL onto the bedrock plugins. Build
 * scripts are code, so this reads the plugin, mainClass, and dependency lines
 * each module's script has, and names the others in the notes; git holds the
 * old scripts.
 */
export const gradle = async (root: string): Promise<Migration> => {
	const migration = newMigration("Gradle");
	const named = await Promise.all(
		["settings.gradle.kts", "settings.gradle"].map(async (name) => ({
			name,
			text: await readText(join(root, name)),
		})),
	);
	const settings = named.find((entry) => entry.text !== undefined);
	const directories = includedDirectories(settings?.text);
	if (settings?.name === "settings.gradle") {
		migration.steps.push({ kind: "remove", path: "settings.gradle" });
	}
	const hasSources =
		directories.length === 0 || (await exists(join(root, "src")));
	const withRoot = hasSources ? ["", ...directories] : directories;
	await inOrder([...new Set(["", ...directories])], (directory) =>
		readProject(root, directory, withRoot.includes(directory), migration),
	);
	return migration;
};

/** A root script that holds no code only configured the other modules, so it goes. */
const dropRoot = (path: string, text: string, migration: Migration): void => {
	migration.steps.push({ kind: "remove", path });
	if (/\b(allprojects|subprojects)\b/u.test(text)) {
		migration.notes.add(
			`${path}: allprojects or subprojects settings; the bedrock plugins set what each module needs, so move the rest into the modules`,
		);
	}
};

const KOTLIN_PLUGIN =
	/kotlin\(\s*"jvm"\s*\)|org\.jetbrains\.kotlin\.jvm|kotlin-gradle-plugin/u;

const isKotlin = (
	root: string,
	base: string,
	text: string,
): Promise<boolean> =>
	KOTLIN_PLUGIN.test(text)
		? Promise.resolve(true)
		: exists(join(root, `${base}src/main/kotlin`));

/** Whether the script needs converting; a bedrock script stays, and an Android or multiplatform one is left to hand. */
const convertible = (
	text: string,
	directory: string,
	path: string,
	migration: Migration,
): boolean => {
	if (text.includes('id("bedrock.')) {
		migration.modules.push({ ...newModule(directory), kept: true });
		return false;
	}
	if (/com\.android\.|multiplatform/u.test(text)) {
		migration.notes.add(
			`${path}: an Android or multiplatform build; apply bedrock.android, bedrock.kmp, or bedrock.cmp to it by hand`,
		);
		return false;
	}
	return true;
};

const fill = (
	module: Module,
	text: string,
	path: string,
	migration: Migration,
): void => {
	module.mainClass = MAIN_CLASS.exec(text)?.[1];
	const level = LEVEL.exec(text);
	javaLevelNote(
		path,
		(level?.[1] ?? level?.[2])?.replaceAll("_", "."),
		migration.notes,
	);
	module.dependencies.push(...readDependencies(text, path, migration));
};

const readProject = async (
	root: string,
	directory: string,
	hasCode: boolean,
	migration: Migration,
): Promise<void> => {
	const base = directory === "" ? "" : `${directory}/`;
	const [groovyText, kotlinText] = await Promise.all([
		readText(join(root, `${base}build.gradle`)),
		readText(join(root, `${base}build.gradle.kts`)),
	]);
	const text = groovyText ?? kotlinText;
	if (text === undefined) {
		return;
	}
	const isGroovy = groovyText !== undefined;
	const path = `${base}build.gradle${isGroovy ? "" : ".kts"}`;
	migration.group =
		(directory === "" ? GROUP.exec(text)?.[1] : undefined) ?? migration.group;
	if (!hasCode) {
		dropRoot(path, text, migration);
		return;
	}
	if (!convertible(text, directory, path, migration)) {
		return;
	}
	if (isGroovy) {
		migration.steps.push({ kind: "remove", path });
	}
	const module = newModule(directory);
	module.kotlin = await isKotlin(root, base, text);
	fill(module, text, path, migration);
	migration.modules.push(module);
};
