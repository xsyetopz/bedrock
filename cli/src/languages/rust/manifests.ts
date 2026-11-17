import {
	findSection,
	hasNoEntries,
	makeSection,
	parseSections,
	removeSections,
	renderSections,
	type Section,
	tidy,
	upsertSection,
	withKey,
	withoutKeys,
} from "./toml.ts";

/** A parsed TOML table. */
export type Table = Record<string, unknown>;

export const isTable = (value: unknown): value is Table =>
	typeof value === "object" && value !== null && !Array.isArray(value);

/** The parsed TOML, or undefined when the text is not valid TOML. */
export const parseToml = (text: string): Table | undefined => {
	try {
		const parsed: unknown = Bun.TOML.parse(text);
		return isTable(parsed) ? parsed : undefined;
	} catch {
		return undefined;
	}
};

export const at = (value: unknown, ...path: readonly string[]): unknown => {
	let node = value;
	for (const key of path) {
		node = isTable(node) ? node[key] : undefined;
	}
	return node;
};

/** What bedrock manages in a root manifest, read from `parts/workspace.toml`. */
export interface Managed {
	/** The `[workspace.lints.*]` tables, with the comments above them. */
	readonly lints: readonly Section[];
	/** `workspace.lints` as parsed values, to tell whether a manifest matches. */
	readonly lintValues: unknown;
	/** The `[profile.release]` settings. */
	readonly release: Readonly<Record<string, unknown>>;
}

/** Reads the managed lints and release profile from the template. */
export const readManaged = (workspaceToml: string): Managed => {
	const parsed = parseToml(workspaceToml);
	const release = at(parsed, "profile", "release");
	return {
		lints: parseSections(workspaceToml).filter(
			(section) => section.name?.startsWith("workspace.lints") === true,
		),
		lintValues: at(parsed, "workspace", "lints"),
		release: isTable(release) ? release : {},
	};
};

const isLintsName = (name: string, prefix: string): boolean =>
	name === prefix || name.startsWith(`${prefix}.`);

/** Dev-dependencies whose job the justfile and `.githooks` took over. */
const REPLACED_DEV_DEPENDENCIES = ["cargo-husky"];

const dropReplacedDevDependencies = (
	sections: readonly Section[],
): Section[] => {
	const kept = removeSections(sections, (name) =>
		REPLACED_DEV_DEPENDENCIES.some((dep) => name === `dev-dependencies.${dep}`),
	).sections;
	return kept.flatMap((section) => {
		if (section.name !== "dev-dependencies") {
			return [section];
		}
		const cleaned = withoutKeys(section, (key) =>
			REPLACED_DEV_DEPENDENCIES.includes(key),
		);
		const emptied =
			cleaned.units.length < section.units.length && hasNoEntries(cleaned);
		return emptied ? [] : [cleaned];
	});
};

/** Points the package at the workspace lints, which replace its own. */
const useWorkspaceLints = (sections: readonly Section[]): Section[] => {
	const { sections: rest, at: position } = removeSections(sections, (name) =>
		isLintsName(name, "lints"),
	);
	const lints = makeSection("lints", ["workspace = true"]);
	const without = dropReplacedDevDependencies(rest);
	return without.toSpliced(position ?? without.length, 0, lints);
};

const packageSatisfied = (doc: Table): boolean =>
	at(doc, "package", "edition") !== undefined &&
	Bun.deepEquals(doc["lints"], { workspace: true }) &&
	REPLACED_DEV_DEPENDENCIES.every(
		(dep) => at(doc, "dev-dependencies", dep) === undefined,
	);

/**
 * Names the edition of a package that declares none. Cargo reads that as
 * 2015, so `edition = "2015"` keeps the meaning and lets the edition
 * migration take the package to 2024. An `edition.workspace` counts as one.
 */
const withDeclaredEdition = (
	doc: Table,
	sections: readonly Section[],
): Section[] => {
	const found = findSection(sections, "package");
	return found === undefined || at(doc, "package", "edition") !== undefined
		? [...sections]
		: upsertSection(sections, withKey(found, "edition", '"2015"'));
};

/** The member manifest with a declared edition and the workspace lints in place of its own. */
export const memberManifest = (text: string): string => {
	const doc = parseToml(text);
	if (doc === undefined) {
		throw new Error("not valid TOML");
	}
	if (packageSatisfied(doc)) {
		return text;
	}
	const sections = useWorkspaceLints(parseSections(text));
	return renderSections(tidy(withDeclaredEdition(doc, sections)));
};

const rootSatisfied = (doc: Table, managed: Managed): boolean =>
	(doc["package"] === undefined || packageSatisfied(doc)) &&
	at(doc, "workspace", "resolver") === "3" &&
	Bun.deepEquals(at(doc, "workspace", "lints"), managed.lintValues) &&
	Object.entries(managed.release).every(([key, value]) =>
		Bun.deepEquals(at(doc, "profile", "release", key), value),
	);

const withResolver = (sections: readonly Section[]): Section[] => {
	const workspace = findSection(sections, "workspace");
	if (workspace === undefined) {
		return [...sections, makeSection("workspace", ['resolver = "3"'])];
	}
	const cleaned = withoutKeys(workspace, (key) => isLintsName(key, "lints"));
	return upsertSection(sections, withKey(cleaned, "resolver", '"3"'));
};

const withManagedLints = (
	sections: readonly Section[],
	managed: Managed,
): Section[] => {
	const { sections: rest, at: position } = removeSections(sections, (name) =>
		isLintsName(name, "workspace.lints"),
	);
	const profile = rest.findIndex(
		(section) => section.name?.startsWith("profile") === true,
	);
	const place = position ?? (profile === -1 ? rest.length : profile);
	return rest.toSpliced(place, 0, ...managed.lints);
};

const withReleaseProfile = (
	sections: readonly Section[],
	managed: Managed,
): Section[] => {
	const found = findSection(sections, "profile.release");
	const base = found ?? makeSection("profile.release", []);
	let release = base;
	for (const [key, value] of Object.entries(managed.release)) {
		release = withKey(release, key, JSON.stringify(value));
	}
	return upsertSection(sections, release);
};

/**
 * The root manifest with the managed workspace lints, resolver, and release
 * profile. A single-package root gains a `[workspace]` table and uses them.
 */
export const rootManifest = (text: string, managed: Managed): string => {
	const doc = parseToml(text);
	if (doc === undefined) {
		throw new Error("not valid TOML");
	}
	if (rootSatisfied(doc, managed)) {
		return text;
	}
	let sections = parseSections(text);
	sections =
		doc["package"] === undefined
			? sections
			: withDeclaredEdition(doc, useWorkspaceLints(sections));
	sections = withManagedLints(withResolver(sections), managed);
	return renderSections(tidy(withReleaseProfile(sections, managed)));
};

/** The editions a manifest declares, at the package and workspace level. */
export const editionsOf = (text: string): string[] => {
	const doc = parseToml(text);
	return [
		at(doc, "package", "edition"),
		at(doc, "workspace", "package", "edition"),
	].filter((edition) => typeof edition === "string");
};

/** rust-toolchain.toml with the pinned channel, keeping the targets the project added. */
export const toolchainFile = (
	template: string,
	current: string | undefined,
	extra: readonly string[],
): string => {
	const listed = at(parseToml(current ?? ""), "toolchain", "targets");
	const kept = Array.isArray(listed)
		? listed.filter((target) => typeof target === "string")
		: [];
	const targets = [...new Set([...kept, ...extra])].toSorted();
	const base = template.endsWith("\n") ? template : `${template}\n`;
	return targets.length === 0
		? base
		: `${base}targets = [${targets.map((target) => JSON.stringify(target)).join(", ")}]\n`;
};
