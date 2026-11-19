import { join, normalize } from "node:path";

import { readText } from "../../core/fs.ts";
import { inOrder } from "../../core/order.ts";
import type { Library, Migration } from "./model.ts";
import {
	childOf,
	childrenOf,
	parseXml,
	textOf,
	type XmlElement
} from "./xml.ts";

/** A parsed pom.xml: its own elements, and the properties and managed versions it inherits. */
export interface Pom {
	readonly directory: string;
	readonly element: XmlElement;
	readonly parent: Pom | undefined;
	readonly properties: ReadonlyMap<string, string>;
	readonly managed: ReadonlyMap<string, string>;
	readonly boms: readonly Library[];
	readonly resolve: (value: string | undefined) => string | undefined;
	readonly library: (configuration: string, dependency: XmlElement) => Library;
	readonly groupId: string;
	readonly artifactId: string;
}

export const dependenciesOf = (
	element: XmlElement | undefined
): XmlElement[] => [
	...childrenOf(childOf(element, "dependencies"), "dependency")
];

const inherited = (element: XmlElement, parent: Pom | undefined) => {
	const parentElement = childOf(element, "parent");
	const groupId =
		textOf(element, "groupId") ??
		parent?.groupId ??
		textOf(parentElement, "groupId") ??
		"";
	const version =
		textOf(element, "version") ??
		(parent === undefined
			? undefined
			: parent.properties.get("project.version")) ??
		textOf(parentElement, "version") ??
		"";
	return { groupId, version, artifactId: textOf(element, "artifactId") ?? "" };
};

const propertiesOf = (
	element: XmlElement,
	parent: Pom | undefined,
	own: { groupId: string; artifactId: string; version: string }
): Map<string, string> => {
	const properties = new Map(parent?.properties);
	for (const property of childrenOf(childOf(element, "properties"))) {
		properties.set(property.name, property.text.trim());
	}
	properties.set("project.groupId", own.groupId);
	properties.set("project.artifactId", own.artifactId);
	properties.set("project.version", own.version);
	return properties;
};

const managedVersions = (
	parent: Pom | undefined,
	management: XmlElement | undefined,
	resolve: (value: string | undefined) => string | undefined
): Map<string, string> => {
	const managed = new Map(parent?.managed);
	for (const dependency of dependenciesOf(management)) {
		const version = resolve(textOf(dependency, "version"));
		if (version !== undefined && textOf(dependency, "scope") !== "import") {
			managed.set(
				`${resolve(textOf(dependency, "groupId"))}:${resolve(textOf(dependency, "artifactId"))}`,
				version
			);
		}
	}
	return managed;
};

const makePom = (
	directory: string,
	element: XmlElement,
	parent: Pom | undefined
): Pom => {
	const own = inherited(element, parent);
	const management = childOf(element, "dependencyManagement");
	const properties = propertiesOf(element, parent, own);
	const resolve = (value: string | undefined): string | undefined =>
		value?.replaceAll(
			/\$\{([^}]+)\}/gu,
			(whole, key: string) => properties.get(key) ?? whole
		);
	const managed = managedVersions(parent, management, resolve);
	const library = (configuration: string, dependency: XmlElement): Library => {
		const group = resolve(textOf(dependency, "groupId")) ?? "";
		const name = resolve(textOf(dependency, "artifactId")) ?? "";
		return {
			kind: "library",
			configuration,
			group,
			name,
			version:
				resolve(textOf(dependency, "version")) ??
				managed.get(`${group}:${name}`)
		};
	};
	const boms = [
		...(parent?.boms ?? []),
		...dependenciesOf(management)
			.filter((dependency) => textOf(dependency, "scope") === "import")
			.map((dependency) => library("implementation", dependency))
	];
	return {
		directory,
		element,
		parent,
		properties,
		managed,
		boms,
		resolve,
		library,
		groupId: own.groupId,
		artifactId: own.artifactId
	};
};

/** Reads the pom at `directory` and, recursively, its modules, removing each pom.xml. */
export const readPoms = async (
	root: string,
	directory: string,
	parent: Pom | undefined,
	migration: Migration
): Promise<Pom[]> => {
	const file = directory === "" ? "pom.xml" : `${directory}/pom.xml`;
	const text = await readText(join(root, file));
	if (text === undefined) {
		throw new Error(`${file}: listed as a Maven module but missing`);
	}
	const pom = makePom(directory, parseXml(text), parent);
	migration.steps.push({ kind: "remove", path: file });
	const nested: Pom[] = [];
	const paths = childrenOf(childOf(pom.element, "modules")).map((module) =>
		normalize(
			directory === ""
				? module.text.trim()
				: `${directory}/${module.text.trim()}`
		).replace(/\/?pom\.xml$/u, "")
	);
	await inOrder(paths, async (path) => {
		nested.push(...(await readPoms(root, path, pom, migration)));
	});
	return [pom, ...nested];
};
