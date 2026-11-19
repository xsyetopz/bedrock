import { at, parseToml } from "./manifests.ts";
import {
	findSection,
	hasNoEntries,
	keyOf,
	parseSections,
	renderSections,
	tidy,
	upsertSection,
	withUnits
} from "./toml.ts";

const isAlias = (unit: string): boolean => {
	const key = keyOf(unit);
	return (
		(key === "bedrock" || key === "alias.bedrock") && unit.includes("xtask")
	);
};

/** The `bedrock` alias that ran the xtask crate, which older releases wrote. */
export const withoutBedrockAlias = (text: string): string => {
	const sections = parseSections(text);
	if (!sections.some((section) => section.units.some(isAlias))) {
		return text;
	}
	const stripped = sections.map((section) =>
		withUnits(
			section,
			section.units.filter((unit) => !isAlias(unit))
		)
	);
	// An alias table that held only the bedrock alias goes with it.
	const kept = stripped.filter(
		(section) => section.name !== "alias" || !hasNoEntries(section)
	);
	return renderSections(tidy(kept));
};

const BEDROCK_GIT = /xsyetopz\/bedrock(?:\.git)?\/?$/u;

/** `deny.toml` without the `allow-git` entry for the bedrock repository, which the old xtask crate needed. */
export const withoutBedrockGit = (text: string): string => {
	const sections = parseSections(text);
	const sources = findSection(sections, "sources");
	const unit = sources?.units.find((entry) => keyOf(entry) === "allow-git");
	if (sources === undefined || unit === undefined) {
		return text;
	}
	const listed = at(parseToml(unit), "allow-git");
	const urls = Array.isArray(listed) ? listed : [];
	const rest = urls.filter(
		(url) => !(typeof url === "string" && BEDROCK_GIT.test(url))
	);
	if (rest.length === urls.length) {
		return text;
	}
	const index = sources.units.indexOf(unit);
	const before = sources.units.slice(0, index);
	while (/^\s*#.*(?:xtask|bedrock)/u.test(before.at(-1) ?? "")) {
		before.pop();
	}
	const replacement =
		rest.length === 0 ? [] : [`allow-git = ${JSON.stringify(rest)}\n`];
	const edited = {
		...sources,
		units: [...before, ...replacement, ...sources.units.slice(index + 1)]
	};
	return renderSections(tidy(upsertSection(sections, edited)));
};

const MEMBER_LINE = /^\s*["']xtask["']\s*,?\s*(?:#.*)?\n?$/u;

/** The `members` entry without `"xtask"`, whether the list is inline or one member per line. */
const withoutMember = (unit: string): string => {
	const lines = unit.split(/(?<=\n)/u);
	const kept = lines.filter((line) => !MEMBER_LINE.test(line));
	if (kept.length < lines.length) {
		return kept.join("");
	}
	return unit
		.replace(/,\s*["']xtask["']/u, "")
		.replace(/["']xtask["']\s*,\s*/u, "")
		.replace(/["']xtask["']/u, "");
};

/** Drops the `"xtask"` workspace member, which older releases added. */
export const withoutXtaskMember = (text: string): string => {
	const sections = parseSections(text);
	const workspace = findSection(sections, "workspace");
	const unit = workspace?.units.find((entry) => keyOf(entry) === "members");
	if (workspace === undefined || unit === undefined) {
		return text;
	}
	const fixed = withoutMember(unit);
	const units = workspace.units.map((entry) =>
		entry === unit ? fixed : entry
	);
	return renderSections(upsertSection(sections, { ...workspace, units }));
};
