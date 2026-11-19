import { readdir } from "node:fs/promises";

/** The object id of the Bedrock.xcconfig file reference wired into a project. */
export const XCCONFIG_REFERENCE = "B3D20C0000000000000000A1";

/** The `.xcodeproj` folders at the repository root, sorted. */
export const xcodeProjects = async (root: string): Promise<string[]> => {
	const entries = await readdir(root).catch(() => []);
	return entries.filter((entry) => entry.endsWith(".xcodeproj")).toSorted();
};

const idAt = (text: string, from: number): string =>
	/^[\p{L}\p{N}]*/u.exec(text.slice(from))?.[0] ?? "";

const valueOf = (
	key: string,
	text: string,
	after: number
): string | undefined => {
	const start = text.indexOf(`${key} = `, after);
	return start === -1 ? undefined : idAt(text, start + key.length + 3);
};

const objectAt = (id: string, text: string): number | undefined => {
	const start = text.indexOf(`\t\t${id} `);
	return start === -1 ? undefined : start + id.length + 3;
};

const childrenOf = (
	key: string,
	text: string,
	after: number
): string[] | undefined => {
	const start = text.indexOf(`${key} = (\n`, after);
	const end = start === -1 ? -1 : text.indexOf(");", start);
	if (end === -1) {
		return undefined;
	}
	return text
		.slice(start + key.length + 5, end)
		.split("\n")
		.map((line) => idAt(line.trim(), 0))
		.filter((id) => id !== "");
};

const insertAt = (text: string, index: number, insert: string): string =>
	text.slice(0, index) + insert + text.slice(index);

const ISA = "isa = XCBuildConfiguration;\n";

/** Gives one build configuration the xcconfig as its base, or undefined when it cannot. */
const wireConfiguration = (text: string, id: string): string | undefined => {
	const start = objectAt(id, text);
	const isa = start === undefined ? -1 : text.indexOf(ISA, start);
	const settings =
		isa === -1 ? -1 : text.indexOf("buildSettings", isa + ISA.length);
	if (
		settings === -1 ||
		text
			.slice(isa + ISA.length, settings)
			.includes("baseConfigurationReference")
	) {
		return undefined;
	}
	return insertAt(
		text,
		isa + ISA.length,
		`\t\t\tbaseConfigurationReference = ${XCCONFIG_REFERENCE} /* Bedrock.xcconfig */;\n`
	);
};

/** Adds the xcconfig file reference to the main group and to the file section. */
const addReference = (text: string, mainGroup: string): string | undefined => {
	const group = objectAt(mainGroup, text);
	const children =
		group === undefined ? -1 : text.indexOf("children = (\n", group);
	const files = text.indexOf("/* End PBXFileReference section */");
	if (children === -1 || files === -1) {
		return undefined;
	}
	const withGroup = insertAt(
		text,
		children + "children = (\n".length,
		`\t\t\t\t${XCCONFIG_REFERENCE} /* Bedrock.xcconfig */,\n`
	);
	return insertAt(
		withGroup,
		withGroup.indexOf("/* End PBXFileReference section */"),
		`\t\t${XCCONFIG_REFERENCE} /* Bedrock.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; name = Bedrock.xcconfig; path = Config/Bedrock.xcconfig; sourceTree = "<group>"; };\n`
	);
};

/**
 * `pbxproj` with Bedrock.xcconfig as the base configuration of each project-level
 * build configuration, or undefined when it already is or another file is.
 */
export const wired = (pbxproj: string): string | undefined => {
	const project = pbxproj.indexOf("isa = PBXProject;");
	if (project === -1 || pbxproj.includes("Bedrock.xcconfig")) {
		return undefined;
	}
	const after = project + "isa = PBXProject;".length;
	const list = valueOf("buildConfigurationList", pbxproj, after);
	const mainGroup = valueOf("mainGroup", pbxproj, after);
	const listObject = list === undefined ? undefined : objectAt(list, pbxproj);
	const configurations =
		listObject === undefined
			? undefined
			: childrenOf("buildConfigurations", pbxproj, listObject);
	if (mainGroup === undefined || configurations === undefined) {
		return undefined;
	}
	let text: string | undefined = pbxproj;
	for (const id of configurations) {
		text = text === undefined ? undefined : wireConfiguration(text, id);
	}
	return text === undefined ? undefined : addReference(text, mainGroup);
};

/** Build settings in `pbxproj` that override what Bedrock.xcconfig sets. */
export const overrides = (pbxproj: string): string[] =>
	pbxproj.split("\n").flatMap((line) => {
		const setting = line.trim();
		if (
			setting.startsWith("SWIFT_VERSION = ") &&
			!setting.startsWith("SWIFT_VERSION = 6")
		) {
			return [`${setting} keeps the target out of Swift 6 mode`];
		}
		if (
			setting.startsWith("OTHER_SWIFT_FLAGS = ") &&
			!setting.includes("$(inherited)")
		) {
			return [
				"OTHER_SWIFT_FLAGS without $(inherited) drops the upcoming features"
			];
		}
		return [];
	});
