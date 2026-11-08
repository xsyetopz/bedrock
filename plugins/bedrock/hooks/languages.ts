/** What /adopt knows about each bedrock's language. */

export type Language = "typescript" | "rust" | "swift" | "jvm";

/** The bedrocks in the order their blocks sit in shared files. */
export const LANGUAGES: readonly Language[] = [
	"typescript",
	"rust",
	"swift",
	"jvm",
];
export const ALIASES = new Map<string, Language>([
	["bun", "typescript"],
	["javascript", "typescript"],
	["js", "typescript"],
	["node", "typescript"],
	["ts", "typescript"],
	["typescript", "typescript"],
	["rs", "rust"],
	["rust", "rust"],
	["swift", "swift"],
	["gradle", "jvm"],
	["java", "jvm"],
	["jvm", "jvm"],
	["kotlin", "jvm"],
	["kt", "jvm"],
]);
/** C, C++, and Objective-C come with swift-bedrock's C++ part. */
export const CXX_ALIASES = new Set([
	"c",
	"c++",
	"cpp",
	"cxx",
	"objc",
	"objective-c",
]);
/** The files that show a repo already uses a language. */
export const MARKERS: Readonly<Record<Language, readonly string[]>> = {
	typescript: ["package.json"],
	rust: ["Cargo.toml"],
	swift: ["Package.swift"],
	jvm: [
		"settings.gradle.kts",
		"build.gradle.kts",
		"settings.gradle",
		"build.gradle",
		"pom.xml",
		"build.sbt",
		"build.xml",
	],
};
/** What each adopt runs, and where to get it. The recipes adopt writes need `just` too. */
export const TOOLS: Readonly<Record<string, string>> = {
	bun: "https://bun.sh, or brew install oven-sh/bun/bun",
	cargo: "rustup from https://rustup.rs",
	git: "the system's package manager, such as brew install git",
	java: "a JDK 17 or newer, such as brew install openjdk",
	just: "brew install just, or cargo install --locked just",
	swift: "Xcode, or a toolchain from https://www.swift.org/install",
};
export const NEEDS: Readonly<Record<Language, string>> = {
	typescript: "bun",
	rust: "cargo",
	swift: "swift",
	jvm: "java",
};

/** What starts each bedrock's CLI from the checkout at `source`. */
export const RUNNERS: Readonly<Record<Language, (source: string) => string[]>> =
	{
		typescript: (source) => [
			"bun",
			`${source}/packages/typescript-bedrock/src/cli.ts`,
		],
		rust: (source) => [
			"cargo",
			"run",
			"--quiet",
			"--manifest-path",
			`${source}/packages/rust-bedrock/Cargo.toml`,
			"--",
		],
		swift: (source) => [
			"swift",
			"run",
			"--quiet",
			"--package-path",
			source,
			"swift-bedrock",
		],
		// Gradle builds the CLI from the checkout, then the launcher it installs runs adopt.
		jvm: (source) => [
			"sh",
			"-c",
			'"$0/gradlew" --quiet --project-dir "$0" :jvm-bedrock:installDist && exec "$0/packages/jvm-bedrock/build/install/jvm-bedrock/bin/jvm-bedrock" "$@"',
			source,
		],
	};
