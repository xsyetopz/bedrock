import { readdir } from "node:fs/promises";
import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Step } from "../../core/steps.ts";
import {
	javaLevelNote,
	type Migration,
	type Module,
	newMigration,
	newModule,
} from "./model.ts";
import { descendants, parseXml, type XmlElement } from "./xml.ts";

/** The files below `directory`, as paths relative to it. */
const filesBelow = async (directory: string): Promise<string[]> => {
	const entries = await readdir(directory, {
		recursive: true,
		withFileTypes: true,
	}).catch(() => []);
	return entries
		.filter((entry) => entry.isFile())
		.map((entry) =>
			join(entry.parentPath, entry.name).slice(directory.length + 1),
		);
};

/** Moves a file as a write and a removal; a file that is not UTF-8 text is counted instead. */
const moveSteps = async (
	root: string,
	from: string,
	to: string,
): Promise<Step[] | undefined> => {
	const bytes = await Bun.file(join(root, from)).bytes();
	try {
		const content = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
		return [
			{ kind: "write", path: to, content },
			{ kind: "remove", path: from },
		];
	} catch {
		return undefined;
	}
};

const sourceDirectories = (
	project: XmlElement,
	resolve: (value: string) => string,
	migration: Migration,
): Set<string> => {
	const sources = new Set<string>();
	for (const javac of descendants(project, "javac")) {
		const srcdir = javac.attributes["srcdir"];
		if (srcdir !== undefined) {
			for (const part of resolve(srcdir).split(/[:,]/u)) {
				sources.add(part);
			}
		}
		for (const src of descendants(javac, "src")) {
			const path = src.attributes["path"];
			if (path !== undefined) {
				sources.add(resolve(path));
			}
		}
		const level = ["release", "target", "source"].find(
			(name) => javac.attributes[name] !== undefined,
		);
		if (level !== undefined) {
			javaLevelNote(
				"build.xml",
				resolve(javac.attributes[level] ?? ""),
				migration.notes,
			);
		}
	}
	return sources;
};

const moveSources = async (
	root: string,
	sources: ReadonlySet<string>,
	migration: Migration,
): Promise<void> => {
	const wanted = [...sources]
		.map((source) => source.trim().replaceAll(/^\/+|\/+$/gu, ""))
		.filter((source) => source !== "");
	await Promise.all(
		wanted.map(async (source) => {
			const set = source.includes("test") ? "test" : "main";
			if (source === `src/${set}/java`) {
				return;
			}
			const files = await filesBelow(join(root, source));
			const moved = await Promise.all(
				files.map(async (relative) => {
					const kind = relative.endsWith(".java") ? "java" : "resources";
					const steps = await moveSteps(
						root,
						`${source}/${relative}`,
						`src/${set}/${kind}/${relative}`,
					);
					if (steps === undefined) {
						migration.notes.add(
							`${source}/${relative}: not text, so adopt left it; move it to src/${set}/${kind}/${relative} by hand`,
						);
					}
					return steps ?? [];
				}),
			);
			migration.steps.push(...moved.flat());
		}),
	);
};

const readBuild = async (
	root: string,
	module: Module,
	migration: Migration,
): Promise<void> => {
	const text = await readText(join(root, "build.xml"));
	if (text === undefined) {
		return;
	}
	const project = parseXml(text);
	const properties = new Map<string, string>();
	for (const property of descendants(project, "property")) {
		const { name, value } = property.attributes;
		if (name !== undefined && value !== undefined) {
			properties.set(name, value);
		}
	}
	const resolve = (value: string): string =>
		value.replaceAll(
			/\$\{([^}]+)\}/gu,
			(whole, key: string) => properties.get(key) ?? whole,
		);
	const main = descendants(project, "attribute").find(
		(element) => element.attributes["name"] === "Main-Class",
	);
	module.mainClass =
		main === undefined ? undefined : resolve(main.attributes["value"] ?? "");
	await moveSources(
		root,
		sourceDirectories(project, resolve, migration),
		migration,
	);
	migration.steps.push({ kind: "remove", path: "build.xml" });
};

const readIvy = async (
	root: string,
	module: Module,
	migration: Migration,
): Promise<void> => {
	const text = await readText(join(root, "ivy.xml"));
	if (text === undefined) {
		return;
	}
	const ivy = parseXml(text);
	const organisation = descendants(ivy, "info")[0]?.attributes["organisation"];
	if (organisation !== undefined && organisation !== "") {
		migration.group = organisation;
	}
	for (const dependency of descendants(ivy, "dependency")) {
		const conf = dependency.attributes["conf"] ?? "";
		const configuration = ivyConfiguration(conf);
		module.dependencies.push({
			kind: "library",
			configuration,
			group: dependency.attributes["org"] ?? "",
			name: dependency.attributes["name"] ?? "",
			version: nonEmpty(dependency.attributes["rev"]),
		});
	}
	migration.steps.push({ kind: "remove", path: "ivy.xml" });
};

const nonEmpty = (value: string | undefined): string | undefined =>
	value === undefined || value === "" ? undefined : value;

const ivyConfiguration = (conf: string): string => {
	if (conf.includes("test")) {
		return "testImplementation";
	}
	if (conf.includes("provided")) {
		return "compileOnly";
	}
	return conf.startsWith("runtime") ? "runtimeOnly" : "implementation";
};

/**
 * Converts build.xml and ivy.xml. Ant names its source folders in javac tasks,
 * so the sources move to Gradle's src/main and src/test; jars in lib/ stay as
 * files() until they get coordinates.
 */
export const ant = async (root: string): Promise<Migration> => {
	const migration = newMigration("Ant");
	const module = newModule("");
	migration.modules.push(module);
	await readBuild(root, module, migration);
	await readIvy(root, module, migration);
	const entries = await readdir(join(root, "lib")).catch(() => []);
	const jars = entries
		.filter((name) => name.endsWith(".jar"))
		.toSorted()
		.map((name) => `lib/${name}`);
	if (jars.length > 0) {
		module.dependencies.push({
			kind: "jars",
			configuration: "implementation",
			paths: jars,
		});
		migration.notes.add(
			`lib/: the build uses ${jars.length} jar(s) as files(); replace each with its Maven coordinates in gradle/libs.versions.toml`,
		);
	}
	return migration;
};
