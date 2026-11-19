import { dirname, join } from "node:path";

import { readText } from "../../core/fs.ts";
import { run } from "../swift/shell.ts";
import { asRecord } from "../typescript/json.ts";
import { dotnetJobs } from "./msbuild.ts";
import { projectFiles } from "./projects.ts";

/**
 * The runner `dotnet test` must use for a test project: `mtp` for a
 * Microsoft.Testing.Platform application, which the .NET 10 SDK refuses to
 * run in VSTest mode even when it also carries a VSTest adapter, and
 * `vstest` for the rest.
 */
export type TestRunner = "mtp" | "vstest";

/**
 * What decides a project's runner: whether it builds on `MSTest.Sdk`, the
 * NuGet packages it references, and which of {@link PROPERTIES} it sets to
 * true, the last two lower-cased.
 */
interface Facts {
	readonly mstestSdk: boolean;
	readonly packages: ReadonlySet<string>;
	readonly flags: ReadonlySet<string>;
}

/**
 * Packages that make a project a Microsoft.Testing.Platform application:
 * xUnit v3 on its MTP v1 or v2 runner, and TUnit, which runs on nothing else.
 * `xunit.v3.mtp-off` and `TUnit.Core`, a library of attributes, do not.
 */
const MTP_PACKAGES = [
	"xunit.v3",
	"xunit.v3.mtp-v1",
	"xunit.v3.mtp-v2",
	"tunit",
	"tunit.engine"
];

/**
 * Packages that make a project a VSTest project: the test SDK, which sets
 * `IsTestProject`, the MSTest metapackage, which brings it, and the VSTest
 * adapters of xUnit, NUnit, MSTest, and Expecto.
 */
const VSTEST_PACKAGES = [
	"microsoft.net.test.sdk",
	"mstest",
	"mstest.testadapter",
	"nunit3testadapter",
	"xunit.runner.visualstudio",
	"yolodev.expecto.testsdk"
];

/**
 * Runner switches, each with the packages whose build files act on it: the
 * switch alone, as a `Directory.Build.props` sets it for every project,
 * leaves a project without the adapter no test application.
 */
const RUNNER_SWITCHES: Readonly<Record<string, readonly string[]>> = {
	enablemstestrunner: ["mstest", "mstest.testadapter"],
	enablenunitrunner: ["nunit3testadapter"]
};

/**
 * Every property {@link Facts} records: the runner switches, the property
 * the test frameworks set for Microsoft.Testing.Platform, `IsTestProject`,
 * and `UseVSTest` of `MSTest.Sdk`. `UseMicrosoftTestingPlatformRunner` set
 * to false is no way out: xUnit v3 stays such an application, and VSTest
 * mode still refuses it.
 */
const PROPERTIES = [
	"EnableMSTestRunner",
	"EnableNUnitRunner",
	"IsTestingPlatformApplication",
	"IsTestProject",
	"UseVSTest"
];

/** The runner {@link Facts} call for, or undefined for a project with no tests. */
const runnerOf = ({
	mstestSdk,
	packages,
	flags
}: Facts): TestRunner | undefined => {
	if (mstestSdk) {
		return flags.has("usevstest") ? "vstest" : "mtp";
	}
	const references = (names: readonly string[]): boolean =>
		names.some((name) => packages.has(name));
	const switched = Object.entries(RUNNER_SWITCHES).some(
		([flag, adapters]) => flags.has(flag) && references(adapters)
	);
	if (
		references(MTP_PACKAGES) ||
		switched ||
		flags.has("istestingplatformapplication")
	) {
		return "mtp";
	}
	return references(VSTEST_PACKAGES) || flags.has("istestproject")
		? "vstest"
		: undefined;
};

/** The {@link Facts} a project file's own text states. */
const textFacts = (text: string): Facts => ({
	mstestSdk: /\bSdk="MSTest\.Sdk|<Sdk\b[^>]*\bName="MSTest\.Sdk"/iu.test(text),
	packages: new Set(
		[
			...text.matchAll(
				/<(?:Global)?PackageReference\b[^>]*?\bInclude="([^"]+)"/giu
			)
		].map((match) => (match[1] ?? "").toLowerCase())
	),
	flags: new Set(
		PROPERTIES.filter((name) =>
			new RegExp(`<${name}>\\s*true\\s*</${name}>`, "iu").test(text)
		).map((name) => name.toLowerCase())
	)
});

/**
 * The runner of a project file's text, or undefined for a project with no
 * tests. `MSTest.Sdk` runs on Microsoft.Testing.Platform unless `UseVSTest`
 * is set.
 */
export const testRunner = (text: string): TestRunner | undefined =>
	runnerOf(textFacts(text));

/** Whether a project file's text makes it a test project, on either runner. */
export const isTestProject = (text: string): boolean =>
	testRunner(text) !== undefined;

/**
 * The files MSBuild imports around a project that can add a test package or
 * set a runner property for it: the `Directory.Build.props` and
 * `Directory.Build.targets` it finds walking up, and the
 * `Directory.Packages.props` whose `GlobalPackageReference` reaches every
 * project.
 */
const CENTRAL_FILES = [
	"Directory.Build.props",
	"Directory.Build.targets",
	"Directory.Packages.props"
];

/** Text that shows a central file may change a project's runner. */
const CENTRAL_TRIGGER = new RegExp(
	[
		`Include="(?:${[...MTP_PACKAGES, ...VSTEST_PACKAGES].map((name) => name.replaceAll(".", String.raw`\.`)).join("|")})"`,
		`<(?:${PROPERTIES.join("|")})>`
	].join("|"),
	"iu"
);

/** The text of the central files in the directories from `path`'s up to the root. */
const centralText = async (root: string, path: string): Promise<string> => {
	const dirs: string[] = [];
	for (let dir = dirname(path); ; dir = dirname(dir)) {
		dirs.push(dir);
		if (dir === ".") {
			break;
		}
	}
	const texts = await Promise.all(
		dirs.flatMap((dir) =>
			CENTRAL_FILES.map((name) => readText(join(root, dir, name)))
		)
	);
	return texts.join("\n");
};

/**
 * The {@link Facts} MSBuild evaluates for a project, with the central files'
 * conditions applied, such as a test package added only to projects named
 * `*.Tests`. Evaluation needs no restore and writes nothing. Undefined when
 * `dotnet` is missing or fails on the project.
 */
const evaluatedFacts = async (
	root: string,
	path: string,
	text: string
): Promise<Facts | undefined> => {
	const { code, stdout } = await run(
		[
			"dotnet",
			"msbuild",
			path,
			"-getItem:PackageReference",
			...[...PROPERTIES, "UsingMSTestSdk"].map(
				(name) => `-getProperty:${name}`
			),
			"-nodeReuse:false"
		],
		root
	);
	if (code !== 0) {
		return undefined;
	}
	let parsed: unknown;
	try {
		parsed = JSON.parse(stdout);
	} catch {
		return undefined;
	}
	const json = asRecord(parsed);
	const properties = asRecord(json["Properties"]);
	const items = asRecord(json["Items"])["PackageReference"];
	const isTrue = (name: string): boolean =>
		String(properties[name]).toLowerCase() === "true";
	return {
		mstestSdk: isTrue("UsingMSTestSdk") || textFacts(text).mstestSdk,
		packages: new Set(
			(Array.isArray(items) ? items : []).map((item) =>
				String(asRecord(item)["Identity"]).toLowerCase()
			)
		),
		flags: new Set(
			PROPERTIES.filter((name) => isTrue(name)).map((name) =>
				name.toLowerCase()
			)
		)
	};
};

/**
 * The runner of one project: from its own text, or, when a central file
 * around it names a test package or runner property, from MSBuild's
 * evaluation, falling back to the text without `dotnet`.
 */
const projectRunner = async (
	root: string,
	path: string
): Promise<TestRunner | undefined> => {
	const [text, central] = await Promise.all([
		readText(join(root, path)),
		centralText(root, path)
	]);
	const own = text ?? "";
	const evaluated = CENTRAL_TRIGGER.test(central)
		? await evaluatedFacts(root, path, own)
		: undefined;
	return runnerOf(evaluated ?? textFacts(own));
};

/** `map` over `items`, at most `size` at a time, in order. */
const inBatches = async <T, R>(
	items: readonly T[],
	size: number,
	map: (item: T) => Promise<R>
): Promise<R[]> => {
	if (items.length === 0) {
		return [];
	}
	const head = await Promise.all(items.slice(0, size).map((item) => map(item)));
	return [...head, ...(await inBatches(items.slice(size), size, map))];
};

/** Each test project under `root` with the runner it needs, sorted by path. */
export const testProjects = async (
	root: string
): Promise<{ path: string; runner: TestRunner }[]> => {
	const found = await projectFiles(root);
	const projects = found.toSorted();
	const runners = await inBatches(projects, dotnetJobs(), (path) =>
		projectRunner(root, path)
	);
	return projects.flatMap((path, index) => {
		const runner = runners[index];
		return runner === undefined ? [] : [{ path, runner }];
	});
};

/** The directories, relative to `root`, that hold a test project. */
export const testProjectDirs = async (root: string): Promise<string[]> => {
	const tests = await testProjects(root);
	return tests.map(({ path }) => dirname(path));
};

/**
 * The files of `files` that a test project compiles: those under its
 * directory, as the SDK's default `Compile` glob takes them. A file outside
 * every test project stays product code.
 */
export const dotnetTestFiles = async (
	root: string,
	files: readonly string[]
): Promise<Set<string>> => {
	const dirs = await testProjectDirs(root);
	return new Set(
		files.filter((file) =>
			dirs.some((dir) => dir === "." || file.startsWith(`${dir}/`))
		)
	);
};
