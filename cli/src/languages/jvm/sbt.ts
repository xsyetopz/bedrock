import { join } from "node:path";

import { exists, readText } from "../../core/fs.ts";
import {
	type Dependency,
	gradlePath,
	javaLevelNote,
	type Migration,
	type Module,
	newMigration,
	newModule
} from "./model.ts";

const LIBRARY =
	/"([^"]+)"\s*(%%?)\s*"([^"]+)"\s*%\s*"([^"]+)"(?:\s*%\s*"?(\w+)"?)?/gu;
const PROJECT =
	/lazy\s+val\s+(\w+)\s*=\s*\(?\s*project(?:\s+in\s*|\.in\s*)\(?\s*file\s*\(\s*"([^"]*)"\s*\)/gu;
const DEPENDS_ON = /dependsOn\s*\(([^)]*)\)/u;
const MAIN_CLASS = /mainClass\s*:=\s*Some\s*\(\s*"([^"]+)"\s*\)/u;
const ORGANIZATION = /organization\s*:=\s*"([^"]+)"/u;
const JAVAC_RELEASE =
	/javacOptions\s*\+\+?=\s*Seq\([^)]*"(?:--release|-target)"\s*,\s*"([^"]+)"/u;

const CONFIGURATIONS: Readonly<Record<string, string>> = {
	test: "testImplementation",
	provided: "compileOnly",
	runtime: "runtimeOnly"
};

/** The text each project owns: its directory and its own section of build.sbt. */
const sectionsOf = (
	text: string
): {
	common: string;
	sections: [string, string][];
	names: Map<string, string>;
} => {
	const projects = [...text.matchAll(PROJECT)];
	const common = text.slice(0, projects[0]?.index ?? text.length);
	const names = new Map(
		projects.map((match) => [
			match[1] ?? "",
			(match[2] ?? "").replaceAll(/^[/.]+|[/.]+$/gu, "")
		])
	);
	const sections: [string, string][] =
		projects.length === 0
			? [["", text]]
			: projects.map((match, index): [string, string] => [
					names.get(match[1] ?? "") ?? "",
					text.slice(match.index, projects[index + 1]?.index ?? text.length)
				]);
	return { common, sections, names };
};

interface Parts {
	readonly common: string;
	readonly single: boolean;
	readonly names: ReadonlyMap<string, string>;
}

const librariesOf = (
	source: string,
	api: string,
	migration: Migration
): Dependency[] =>
	[...source.matchAll(LIBRARY)].flatMap((match): Dependency[] => {
		const [, group = "", cross, name = "", version, scope = ""] = match;
		if (cross === "%%") {
			migration.notes.add(
				`build.sbt: ${group} %% ${name} is a Scala library; bedrock builds Java and Kotlin, so find a Java one`
			);
			return [];
		}
		const configuration = CONFIGURATIONS[scope.toLowerCase()] ?? api;
		return [{ kind: "library", configuration, group, name, version }];
	});

const modulesOf = (
	section: string,
	api: string,
	names: ReadonlyMap<string, string>
): Dependency[] =>
	(DEPENDS_ON.exec(section)?.[1] ?? "").split(",").flatMap((reference) => {
		const target = names.get(reference.trim().split(" ")[0] ?? "");
		return target === undefined
			? []
			: [{ kind: "module", configuration: api, path: gradlePath(target) }];
	});

const moduleOf = (
	directory: string,
	section: string,
	kotlin: boolean,
	parts: Parts,
	migration: Migration
): Module => {
	const module = newModule(directory);
	const source = parts.single ? section : parts.common + section;
	module.kotlin = kotlin;
	module.mainClass = (MAIN_CLASS.exec(section) ??
		MAIN_CLASS.exec(parts.common))?.[1];
	const api = module.mainClass === undefined ? "api" : "implementation";
	module.dependencies.push(
		...librariesOf(source, api, migration),
		...modulesOf(section, api, parts.names)
	);
	javaLevelNote("build.sbt", JAVAC_RELEASE.exec(source)?.[1], migration.notes);
	return module;
};

/**
 * Converts build.sbt. An sbt build is Scala code, so this reads the common
 * forms: settings before the first project apply to every project, and each
 * `lazy val x = project.in(file(…))` owns the text up to the next one.
 */
export const sbt = async (root: string): Promise<Migration> => {
	const migration = newMigration("sbt");
	const text = (await readText(join(root, "build.sbt"))) ?? "";
	migration.group = ORGANIZATION.exec(text)?.[1];
	const { common, sections, names } = sectionsOf(text);
	const parts = {
		common,
		names,
		single: sections.length === 1 && names.size === 0
	};
	const kotlin = await Promise.all(
		sections.map(([directory]) =>
			exists(join(root, directory, "src/main/kotlin"))
		)
	);
	for (const [index, [directory, section]] of sections.entries()) {
		migration.modules.push(
			moduleOf(directory, section, kotlin[index] === true, parts, migration)
		);
	}
	migration.steps.push({ kind: "remove", path: "build.sbt" });
	if (await exists(join(root, "project"))) {
		migration.steps.push({ kind: "remove", path: "project" });
	}
	migration.notes.add(
		"build.sbt: sbt builds are code, so compare each new build.gradle.kts with it; project/ held sbt's plugins and build code"
	);
	return migration;
};
