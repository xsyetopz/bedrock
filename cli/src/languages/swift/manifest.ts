import { type Block, upsert } from "../../core/blocks.ts";

/** The tools version every adopted manifest declares. */
export const TOOLS_VERSION = "// swift-tools-version: 6.4";

/** Replaces the first line when it is a tools-version comment, else puts one first. */
export const withToolsVersion = (text: string): string => {
	const lines = text.split("\n");
	const first = (lines[0] ?? "").replaceAll(/\s/gu, "").toLowerCase();
	if (first.startsWith("//swift-tools-version")) {
		lines[0] = TOOLS_VERSION;
	} else {
		lines.unshift(TOOLS_VERSION);
	}
	return lines.join("\n");
};

const OLD_MACOS =
	/\.macOS\(\s*(?:\.v10_\d+|\.v11|"(?:10\.\d+|11)(?:\.\d+)*")\s*\)/gu;

/** Raises a macOS target below 12, the oldest the templates build for, to `.v12`. */
export const withMinimumMacOS = (text: string): string =>
	text.replaceAll(OLD_MACOS, ".macOS(.v12)");

/** The manifest with the tools version, the macOS floor, and the strict blocks. */
export const adoptedManifest = (
	text: string,
	blocks: readonly Block[],
): string =>
	upsert(withMinimumMacOS(withToolsVersion(text)), "", "slash", blocks);

/** `raw`, such as a directory name, as a Swift type name: `my-tool` becomes `MyTool`. */
export const typeName = (raw: string): string =>
	raw
		.split(/[^\p{L}\p{N}]+/u)
		.filter((word) => word !== "")
		.map((word) => word.slice(0, 1).toUpperCase() + word.slice(1))
		.join("");

/** Whether `name` starts with a letter and holds only ASCII letters and digits. */
export const isTypeName = (name: string): boolean =>
	/^\p{L}/u.test(name) && /^[A-Za-z0-9]+$/u.test(name);

// swift-format drops the trailing comma of a one-element collection, so that case stays inline.
const collection = (items: readonly string[]): string =>
	items.length === 1
		? `[${items[0]}]`
		: `[\n${items.map((item) => `    ${item},\n`).join("")}  ]`;

/** The Package.swift for a new package, before the managed blocks. */
export const newManifest = (
	name: string,
	kinds: ReadonlySet<string>,
	cxx: boolean,
): string => {
	const products = [`.library(name: "${name}", targets: ["${name}"])`];
	const targets = [
		cxx
			? `.target(name: "${name}", dependencies: ["${name}Cxx"])`
			: `.target(name: "${name}")`,
	];
	if (cxx) {
		targets.push(`.target(name: "${name}Cxx")`);
	}
	if (kinds.has("executable")) {
		products.push(
			`.executable(name: "${name.toLowerCase()}", targets: ["${name}CLI"])`,
		);
		targets.push(
			`.executableTarget(name: "${name}CLI", dependencies: ["${name}"])`,
		);
	}
	targets.push(`.testTarget(name: "${name}Tests", dependencies: ["${name}"])`);
	return `// swift-tools-version: 6.4
import PackageDescription

let package = Package(
  name: "${name}",
  platforms: [.macOS(.v12)],
  products: ${collection(products)},
  targets: ${collection(targets)}
)
`;
};
