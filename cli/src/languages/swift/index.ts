import { basename } from "node:path";

import type { Context, Language } from "../../core/language.ts";
import { guardSwift } from "./guard.ts";
import { KINDS } from "./kit.ts";
import { typeName } from "./manifest.ts";
import { planSwift } from "./plan.ts";
import { run } from "./shell.ts";
import { convertTests } from "./subcommand.ts";

/** Resolves the package's dependencies when swift is installed; a failure never fails the adopt. */
const afterApply = async (context: Context): Promise<void> => {
	if (
		Bun.which("swift") === null ||
		!(await Bun.file(`${context.root}/Package.swift`).exists())
	) {
		return;
	}
	// The recipes run SwiftPM again and report what is wrong.
	await run(["swift", "package", "--disable-sandbox", "resolve"], context.root);
};

/** Swift: strict Swift 6 manifests, swift-format, SwiftLint, Swift Testing, Xcode wiring, and the XCTest converter. */
export const swift: Language = {
	id: "swift",
	block: "swift-bedrock",
	aliases: ["swift"],
	markers: [
		"Package.swift",
		".swift-version",
		".swift-format",
		".swiftlint.yml"
	],
	tools: {
		swift: "swiftly (https://www.swift.org/install/), or Xcode",
		swiftlint: "brew install swiftlint"
	},
	sources: [
		"**/*.swift",
		"Package.swift",
		".swift-format",
		".swift-version",
		".swiftlint.yml",
		"**/.swiftlint.yml",
		"**/*.xcodeproj/project.pbxproj",
		"Config/*.xcconfig"
	],
	kinds: KINDS,
	variables: (context) => {
		const name = typeName(basename(context.root));
		return Promise.resolve({ typeName: name, lowerName: name.toLowerCase() });
	},
	plan: planSwift,
	afterApply,
	runs: "swift package resolve",
	guard: guardSwift,
	subcommands: { "convert-tests": convertTests }
};
