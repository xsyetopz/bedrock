import { join } from "node:path";

import { existing, exists } from "../../core/fs.ts";
import {
	type Dependency,
	gradlePath,
	javaLevelNote,
	type Library,
	type Migration,
	type Module,
	newMigration,
	newModule
} from "./model.ts";
import { dependenciesOf, type Pom, readPoms } from "./pom.ts";
import { childOf, descendants, textOf, type XmlElement } from "./xml.ts";

const SCOPES: Readonly<Record<string, string | null>> = {
	provided: "compileOnly",
	runtime: "runtimeOnly",
	test: "testImplementation",
	system: null
};

const dependencyFor = (
	pom: Pom,
	dependency: XmlElement,
	api: string,
	reactor: ReadonlyMap<string, string>,
	migration: Migration,
	file: string
): Dependency => {
	const scope = textOf(dependency, "scope");
	const configuration = scope === undefined ? api : (SCOPES[scope] ?? api);
	const library = pom.library(configuration, dependency);
	const local = reactor.get(`${library.group}:${library.name}`);
	if (scope === "system") {
		const path = pom.resolve(textOf(dependency, "systemPath")) ?? "";
		migration.notes.add(
			`${file}: ${library.group}:${library.name} has system scope; replace files("${path}") with its Maven coordinates`
		);
		return { kind: "jars", configuration: "implementation", paths: [path] };
	}
	if (local !== undefined) {
		return { kind: "module", configuration, path: gradlePath(local) };
	}
	const type = textOf(dependency, "type");
	if (
		textOf(dependency, "classifier") !== undefined ||
		(type !== undefined && type !== "jar")
	) {
		migration.notes.add(
			`${file}: ${library.group}:${library.name} has a classifier or type; add it to the catalog entry by hand`
		);
	}
	return library;
};

const parentPlatform = (pom: Pom, api: string): Dependency[] => {
	let top = pom;
	while (top.parent !== undefined) {
		top = top.parent;
	}
	const parent = childOf(top.element, "parent");
	if (parent === undefined) {
		return [];
	}
	const library: Library = {
		kind: "library",
		configuration: api,
		group: textOf(parent, "groupId") ?? "",
		name: textOf(parent, "artifactId") ?? "",
		version: textOf(parent, "version")
	};
	return [{ kind: "platform", configuration: api, library }];
};

const levelNotes = (pom: Pom, file: string, migration: Migration): void => {
	for (const key of [
		"maven.compiler.release",
		"maven.compiler.target",
		"maven.compiler.source",
		"java.version"
	]) {
		javaLevelNote(file, pom.properties.get(key), migration.notes);
	}
	const release = descendants(pom.element, "release")[0];
	if (release !== undefined) {
		javaLevelNote(file, pom.resolve(release.text.trim()), migration.notes);
	}
	const build = childOf(pom.element, "build");
	for (const tag of ["sourceDirectory", "testSourceDirectory"]) {
		const directory = textOf(build, tag);
		if (directory !== undefined) {
			migration.notes.add(
				`${file}: ${tag} ${directory} is not where Gradle looks; move the sources to src/main or src/test`
			);
		}
	}
};

const mainClassOf = (pom: Pom): string | undefined => {
	const declared = [
		...descendants(pom.element, "mainClass"),
		...descendants(pom.element, "Main-Class")
	]
		.map((element) => pom.resolve(element.text.trim()))
		.find((value) => value !== undefined && value !== "");
	return (
		declared ??
		pom.properties.get("start-class") ??
		pom.properties.get("exec.mainClass")
	);
};

const moduleFor = async (
	root: string,
	pom: Pom,
	reactor: ReadonlyMap<string, string>,
	migration: Migration
): Promise<Module> => {
	const file = pom.directory === "" ? "pom.xml" : `${pom.directory}/pom.xml`;
	const module = newModule(pom.directory);
	module.kotlin =
		(await exists(join(root, pom.directory, "src/main/kotlin"))) ||
		descendants(pom.element, "artifactId").some(
			(element) => element.text.trim() === "kotlin-maven-plugin"
		);
	module.mainClass = mainClassOf(pom);
	const api = module.mainClass === undefined ? "api" : "implementation";
	module.dependencies.push(
		...parentPlatform(pom, api),
		...pom.boms.map((library): Dependency => ({
			kind: "platform",
			configuration: api,
			library
		})),
		...dependenciesOf(pom.element).map((dependency) =>
			dependencyFor(pom, dependency, api, reactor, migration, file)
		)
	);
	levelNotes(pom, file, migration);
	return module;
};

const noteParent = (pom: Pom, migration: Migration): void => {
	const file = pom.directory === "" ? "pom.xml" : `${pom.directory}/pom.xml`;
	const parent = childOf(pom.element, "parent");
	if (pom.parent === undefined && parent !== undefined) {
		migration.notes.add(
			`${file}: the parent ${textOf(parent, "groupId")}:${textOf(parent, "artifactId")}:${textOf(parent, "version")} is outside the repository, so each module imports it as a platform for the versions it manages; move its plugins and settings by hand`
		);
	}
};

/** Converts the Maven build rooted at `root`'s pom.xml, following its modules. */
export const maven = async (root: string): Promise<Migration> => {
	const migration = newMigration("Maven");
	const poms = await readPoms(root, "", undefined, migration);
	const wrappers = await existing(root, ["mvnw", "mvnw.cmd", ".mvn"]);
	for (const path of wrappers) {
		migration.steps.push({ kind: "remove", path });
	}
	const top = poms[0];
	migration.group = top?.groupId === "" ? undefined : top?.groupId;
	const reactor = new Map(
		poms.map((pom) => [`${pom.groupId}:${pom.artifactId}`, pom.directory])
	);
	for (const pom of poms) {
		noteParent(pom, migration);
	}
	const converted = poms.filter(
		(pom) => textOf(pom.element, "packaging") !== "pom"
	);
	const modules = await Promise.all(
		converted.map((pom) => moduleFor(root, pom, reactor, migration))
	);
	migration.modules.push(...modules);
	return migration;
};
