import { readdir } from "node:fs/promises";
import { join } from "node:path";

import type { Step } from "../../core/steps.ts";

/** A library as Maven coordinates; the version is absent when a BOM or parent supplies it. */
export interface Library {
	readonly kind: "library";
	readonly configuration: string;
	readonly group: string;
	readonly name: string;
	readonly version?: string | undefined;
}

/** A dependency in Gradle terms: a library, another module, a platform, local jars, or kept notation. */
export type Dependency =
	| Library
	| {
			readonly kind: "platform";
			readonly configuration: string;
			readonly library: Library;
	  }
	| {
			readonly kind: "module";
			readonly configuration: string;
			readonly path: string;
	  }
	| {
			readonly kind: "jars";
			readonly configuration: string;
			readonly paths: readonly string[];
	  }
	| {
			readonly kind: "notation";
			readonly configuration: string;
			readonly text: string;
	  };

/** One Gradle project the old build described, at `directory` relative to the root ("" for it). */
export interface Module {
	readonly directory: string;
	kotlin: boolean;
	mainClass?: string | undefined;
	/** Whether its build script applies bedrock already, so render leaves it as it is. */
	kept: boolean;
	readonly dependencies: Dependency[];
}

/** What a build tool's files turn into, and what adopt could not carry over. */
export interface Migration {
	readonly tool: string;
	group?: string | undefined;
	readonly modules: Module[];
	readonly steps: Step[];
	readonly notes: Set<string>;
}

/** An empty module at `directory`. */
export const newModule = (directory: string): Module => ({
	directory,
	kotlin: false,
	kept: false,
	dependencies: []
});

/** An empty migration for `tool`. */
export const newMigration = (tool: string): Migration => ({
	tool,
	modules: [],
	steps: [],
	notes: new Set()
});

/** The Gradle path, such as `:core`, for a module below the root. */
export const gradlePath = (directory: string): string =>
	`:${directory.replaceAll("/", ":")}`;

/** Adds a note when a build compiles for a Java level above the Java 8 bytecode bedrock targets. */
export const javaLevelNote = (
	file: string,
	level: string | undefined,
	notes: Set<string>
): void => {
	const number = Math.trunc(Number(level?.replace(/^1\./u, "") ?? ""));
	if (number > 8) {
		notes.add(
			`${file}: compiled for Java ${number}; bedrock compiles main code to Java 8, so the compilers reject newer APIs and syntax`
		);
	}
};

const IGNORED_DIRECTORIES = new Set(["build", "target", "node_modules"]);

const foreignIn = async (
	root: string,
	directory: string
): Promise<string[]> => {
	const entries = await readdir(join(root, directory), {
		withFileTypes: true
	}).catch(() => []);
	const found = await Promise.all(
		entries
			.filter(
				(entry) =>
					entry.isDirectory() &&
					!IGNORED_DIRECTORIES.has(entry.name) &&
					!entry.name.startsWith(".")
			)
			.map(async (entry) => {
				const path =
					directory === "" ? entry.name : `${directory}/${entry.name}`;
				const own =
					(entry.name === "scala" || entry.name === "groovy") &&
					directory.split("/").at(-2) === "src"
						? [path]
						: [];
				const nested = await foreignIn(root, path);
				nested.unshift(...own);
				return nested;
			})
	);
	return found.flat();
};

/** Scala and Groovy source directories, which the bedrock plugins do not compile. */
export const foreignSources = (root: string): Promise<string[]> =>
	foreignIn(root, "");

/**
 * A catalog alias Gradle can make accessors for: lowercase segments joined by
 * `-`, each starting with a letter, so log4j-1.2-api becomes log4j12-api.
 */
export const aliasOf = (name: string): string => {
	const segments: string[] = [];
	for (const part of name.toLowerCase().split(/[^a-z0-9]+/u)) {
		if (part === "") {
			continue;
		}
		const last = segments.at(-1);
		if (last !== undefined && /^\d/u.test(part)) {
			segments[segments.length - 1] = last + part;
		} else {
			segments.push(part);
		}
	}
	const joined = segments.join("-") || "library";
	const reserved = ["bundles", "versions", "plugins"];
	return !/^[a-z]/u.test(joined) || reserved.includes(segments[0] ?? "")
		? `lib-${joined}`
		: joined;
};
