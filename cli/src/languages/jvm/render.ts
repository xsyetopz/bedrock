import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Step } from "../../core/steps.ts";
import {
	aliasOf,
	type Dependency,
	gradlePath,
	type Library,
	type Migration,
	type Module
} from "./model.ts";

/** Libraries the bedrock plugins already add. */
const PROVIDED =
	/^(org\.jetbrains\.kotlin:kotlin-(stdlib|test)\S*|junit:junit|org\.testng:testng|org\.junit(\.\w+)*:\S+)$/u;

const OLD_TESTS = new Set(["junit:junit", "org.testng:testng"]);

const moduleOf = (library: Library): string =>
	`${library.group}:${library.name}`;

/** Catalog entries, keyed by alias, that the migrated build refers to as `libs.…`. */
class Aliases {
	readonly entries = new Map<string, Library>();

	/** The alias for `library`, which reuses an entry with the same coordinates and version. */
	of(library: Library): string {
		for (const [alias, known] of this.entries) {
			if (
				moduleOf(known) === moduleOf(library) &&
				known.version === library.version
			) {
				return alias;
			}
		}
		const base = aliasOf(library.name);
		const name = this.taken(base)
			? aliasOf(`${library.group.split(".").at(-1)}-${library.name}`)
			: base;
		let unique = name;
		for (let count = 2; this.taken(unique); count += 1) {
			unique = `${name}${count}`;
		}
		this.entries.set(unique, library);
		return unique;
	}

	// Gradle cannot make accessors for both foo and foo-bar.
	private taken(alias: string): boolean {
		return [...this.entries.keys()].some(
			(key) =>
				key === alias ||
				key.startsWith(`${alias}-`) ||
				alias.startsWith(`${key}-`)
		);
	}
}

const accessor = (alias: string): string =>
	`libs.${alias.replaceAll("-", ".")}`;

const notationOf = (dependency: Dependency, aliases: Aliases): string => {
	switch (dependency.kind) {
		case "library": {
			return accessor(aliases.of(dependency));
		}
		case "platform": {
			return `platform(${accessor(aliases.of(dependency.library))})`;
		}
		case "module": {
			return `project("${dependency.path}")`;
		}
		case "jars": {
			return `files(${dependency.paths.map((path) => `"${path}"`).join(", ")})`;
		}
		case "notation": {
			return dependency.text;
		}
		default: {
			return "";
		}
	}
};

const dependencyLines = (
	module: Module,
	aliases: Aliases,
	notes: Set<string>
): string[] => {
	const lines: string[] = [];
	for (const dependency of module.dependencies) {
		if (dependency.kind === "library") {
			const coordinates = moduleOf(dependency);
			if (OLD_TESTS.has(coordinates)) {
				notes.add(
					`${gradlePath(module.directory)}: tests used ${coordinates}; bedrock runs JUnit 6, so port them to org.junit.jupiter.api, which just guard checks`
				);
			}
			if (PROVIDED.test(coordinates)) {
				continue;
			}
		}
		lines.push(
			`  ${dependency.configuration}(${notationOf(dependency, aliases)})`
		);
	}
	return [...new Set(lines)];
};

const scriptOf = (module: Module, lines: readonly string[]): string => {
	const plugins = [
		module.kotlin ? "bedrock.kotlin" : "bedrock.java",
		...(module.mainClass === undefined ? [] : ["bedrock.application"])
	];
	const head =
		plugins.length === 1
			? `plugins { id("${plugins[0]}") }\n`
			: `plugins {\n${plugins.map((id) => `  id("${id}")`).join("\n")}\n}\n`;
	const application =
		module.mainClass === undefined
			? ""
			: `\napplication { mainClass = "${module.mainClass}" }\n`;
	const dependencies =
		lines.length === 0 ? "" : `\ndependencies {\n${lines.join("\n")}\n}\n`;
	return head + application + dependencies;
};

const catalogStep = async (root: string, aliases: Aliases): Promise<Step[]> => {
	if (aliases.entries.size === 0) {
		return [];
	}
	const catalog = await readText(join(root, "gradle/libs.versions.toml"));
	const entries = [...aliases.entries]
		.map(
			([name, library]) =>
				`${name} = { module = "${moduleOf(library)}"${
					library.version === undefined
						? ""
						: `, version = "${library.version}"`
				} }`
		)
		.join("\n");
	return [
		{
			kind: "write",
			path: "gradle/libs.versions.toml",
			content:
				catalog === undefined
					? `[libraries]\n${entries}\n`
					: `${catalog.trimEnd()}\n\n# Added by bedrock adopt.\n${entries}\n`
		}
	];
};

/** The steps that write a migration's build files, with each library in gradle/libs.versions.toml. */
export const render = async (
	root: string,
	migration: Migration
): Promise<Step[]> => {
	const aliases = new Aliases();
	const scripts = migration.modules
		.filter((module) => !module.kept)
		.map((module): Step => {
			const lines = dependencyLines(module, aliases, migration.notes);
			return {
				kind: "write",
				path:
					module.directory === ""
						? "build.gradle.kts"
						: `${module.directory}/build.gradle.kts`,
				content: scriptOf(module, lines)
			};
		});
	return [
		...migration.steps,
		...scripts,
		...(await catalogStep(root, aliases))
	];
};

/** The settings line that includes each module below the root, or "". */
export const includes = (migration: Migration): string => {
	const paths = migration.modules
		.filter((module) => module.directory !== "")
		.map((module) => `"${gradlePath(module.directory)}"`);
	return paths.length === 0 ? "" : `include(${paths.join(", ")})\n`;
};
