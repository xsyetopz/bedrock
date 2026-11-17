import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { Context, Finding } from "../../core/language.ts";
import { asArray, asRecord, asString, asStrings } from "./json.ts";
import { firstLine, run } from "./shell.ts";
import { importsXCTest, isUITest, suppressions } from "./suppressions.ts";
import { overrides, xcodeProjects } from "./xcode.ts";
import { convertXCTest } from "./xctest/convert.ts";

/** What every target's settings must contain, as `setting` or `setting:value`. */
export const STRICT = [
	"enableUpcomingFeature:ExistentialAny",
	"enableUpcomingFeature:InternalImportsByDefault",
	"enableUpcomingFeature:MemberImportVisibility",
	"enableUpcomingFeature:InferIsolatedConformances",
	"enableUpcomingFeature:NonisolatedNonsendingByDefault",
	"enableUpcomingFeature:ImmutableWeakCaptures",
	"strictMemorySafety",
	"treatAllWarnings:error",
];

/** Dependency files of other tools, which Package.swift replaces. */
const FOREIGN: Readonly<Record<string, string>> = {
	Cartfile:
		"Carthage: move each framework to a SwiftPM dependency in Package.swift",
	Podfile: "CocoaPods: move each pod to a SwiftPM dependency in Package.swift",
	".swiftformat": "SwiftFormat: swift-format and .swift-format replaced it",
};

/** The `setting` or `setting:value` names one target's swift settings carry. */
const settingNames = (
	settings: readonly Record<string, unknown>[],
): Set<string> =>
	new Set(
		settings.flatMap((kind) =>
			Object.entries(kind).map(([key, value]) => {
				const argument = asRecord(value)["_0"];
				return typeof argument === "string" ? `${key}:${argument}` : key;
			}),
		),
	);

const targetFindings = (target: Record<string, unknown>): Finding[] => {
	const name = asString(target["name"]) || "?";
	if (["system", "binary", "plugin"].includes(asString(target["type"]))) {
		return [];
	}
	const settings = asArray(target["settings"]).map((setting) =>
		asRecord(asRecord(setting)["kind"]),
	);
	const found = settingNames(settings);
	const missing = STRICT.filter((setting) => !found.has(setting));
	const problems: Finding[] = [];
	if (missing.length > 0) {
		problems.push(
			`Package.swift: target ${name} lacks ${missing.join(", ")}; keep the swift-bedrock block last`,
		);
	}
	if (settings.some((kind) => "unsafeFlags" in kind)) {
		problems.push(
			`Package.swift: target ${name} uses unsafeFlags, which keeps others from depending on the package`,
		);
	}
	return problems;
};

/** Problems with the package that `swift package dump-package` describes. */
export const manifestFindings = (pkg: Record<string, unknown>): Finding[] => {
	const tools = asString(asRecord(pkg["toolsVersion"])["_version"]);
	const modes = asStrings(pkg["swiftLanguageVersions"]);
	const problems: Finding[] = [];
	if (!tools.startsWith("6.4")) {
		problems.push(`Package.swift: swift-tools-version is ${tools}; use 6.4`);
	}
	if (modes.length !== 1 || modes[0] !== "6") {
		problems.push(
			`Package.swift: the language modes are [${modes.join(", ")}]; use [.v6] alone`,
		);
	}
	return [
		...problems,
		...asArray(pkg["targets"]).flatMap((target) =>
			targetFindings(asRecord(target)),
		),
	];
};

const dumpFindings = async (root: string): Promise<Finding[]> => {
	// SwiftPM would nest its sandbox inside an agent's or hook's, and that fails.
	const dumped = await run(
		["swift", "package", "--disable-sandbox", "dump-package"],
		root,
	);
	if (dumped.code === 127) {
		return [
			"Package.swift: swift is missing, so the manifest was not checked; install it with swiftly or Xcode",
		];
	}
	if (dumped.code !== 0) {
		return [
			`Package.swift: swift package dump-package failed: ${firstLine(dumped.stderr)}`,
		];
	}
	try {
		return manifestFindings(asRecord(JSON.parse(dumped.stdout)));
	} catch {
		return ["Package.swift: swift package dump-package printed no package"];
	}
};

const fileFindings = async (
	root: string,
	path: string,
	fix: boolean,
): Promise<Finding[]> => {
	let text = await readText(join(root, path));
	if (text === undefined) {
		return [];
	}
	const problems: Finding[] = [];
	if (!isUITest(path) && importsXCTest(text)) {
		const converted = fix ? convertXCTest(text) : undefined;
		if (converted !== undefined) {
			await Bun.write(join(root, path), converted.text);
			text = converted.text;
			problems.push(...converted.notes.map((note) => `${path}: ${note}`));
		}
		if (importsXCTest(text)) {
			problems.push(
				`${path}: imports XCTest outside a *UITests target; use Swift Testing (just lint-fix converts it)`,
			);
		}
	}
	return [...problems, ...suppressions(text).map((note) => `${path}:${note}`)];
};

const projectFinding = async (
	root: string,
	project: string,
): Promise<Finding[]> => {
	const path = `${project}/project.pbxproj`;
	const text = await readText(join(root, path));
	if (text === undefined) {
		return [];
	}
	const problems: Finding[] = [];
	if (!text.includes("Bedrock.xcconfig")) {
		problems.push(
			`${path}: set Config/Bedrock.xcconfig as the base configuration of the project's Debug and Release`,
		);
	}
	for (const note of new Set(overrides(text).toSorted())) {
		problems.push(`${path}: ${note}; remove it`);
	}
	return problems;
};

const projectFindings = async (root: string): Promise<Finding[]> => {
	const projects = await xcodeProjects(root);
	const found = await Promise.all(
		projects.map((project) => projectFinding(root, project)),
	);
	return found.flat();
};

const foreignFindings = async (root: string): Promise<Finding[]> => {
	const present = await Promise.all(
		Object.keys(FOREIGN).map((path) => Bun.file(join(root, path)).exists()),
	);
	return Object.entries(FOREIGN).flatMap(([path, problem], index) =>
		present[index] === true ? [`${path}: ${problem}`] : [],
	);
};

/**
 * Reports what SwiftLint cannot see: the settings the manifest ends up with,
 * XCTest outside UI tests, file-wide suppressions, foreign tools, and Xcode
 * projects that skip Config/Bedrock.xcconfig. With `fix`, converts XCTest first.
 */
export const guardSwift = async (
	context: Context,
	fix: boolean,
): Promise<Finding[]> => {
	const { root } = context;
	const [manifest, sources] = await Promise.all([
		Bun.file(join(root, "Package.swift")).exists(),
		listFiles(root, new Glob("**/*.swift")),
	]);
	const [dump, files, foreign, projects] = await Promise.all([
		manifest ? dumpFindings(root) : [],
		Promise.all(sources.map((path) => fileFindings(root, path, fix))),
		foreignFindings(root),
		projectFindings(root),
	]);
	return [...dump, ...files.flat(), ...foreign, ...projects];
};
