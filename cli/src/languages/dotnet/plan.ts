import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { Context, LanguagePlan } from "../../core/language.ts";
import {
	GLOBAL_JSON,
	globalJsonStep,
	langVersionNotes,
	retargetSteps,
	TESTING_PLATFORM,
	toolManifestStep,
} from "./manifests.ts";
import { testProjects, type TestRunner } from "./test-projects.ts";

const WORKFLOW = ".github/workflows/dotnet.yml";
const PROPS = "Directory.Build.props";

/**
 * A note when adopt replaces a .NET workflow bedrock did not write, since its
 * steps, such as packing or publishing, are the project's and a person moves them.
 */
const workflowNote = async (root: string): Promise<string[]> => {
	const text = await readText(join(root, WORKFLOW));
	if (text === undefined || text.startsWith("# Managed by bedrock.")) {
		return [];
	}
	return [
		`${WORKFLOW}: replaced with bedrock's; git has yours. Move the steps it still needs into .github/actions/project-setup/action.yml and the recipes it ran into project.just.`,
	];
};

/**
 * A note for each other workflow that sets up .NET or runs `dotnet`, such as a
 * hand-written `ci.yml`: bedrock's .NET workflow now runs those checks, on the
 * SDK `global.json` pins, so the old steps duplicate them on another SDK.
 */
const duplicateWorkflowNotes = async (root: string): Promise<string[]> => {
	const paths = await listFiles(
		root,
		new Glob(".github/workflows/*.{yml,yaml}"),
	);
	const others = paths.filter((path) => path !== WORKFLOW);
	const texts = await Promise.all(
		others.map((path) => readText(join(root, path))),
	);
	return others.flatMap((path, index) => {
		const text = texts[index] ?? "";
		const runsDotnet =
			text.includes("actions/setup-dotnet") || /\bdotnet\s+\w/u.test(text);
		return runsDotnet && !text.startsWith("# Managed by bedrock.")
			? [
					`${path}: sets up .NET or runs dotnet, which ${WORKFLOW} now does on the SDK global.json pins; remove the steps it duplicates.`,
				]
			: [];
	});
};

/**
 * The settings bedrock's starter `Directory.Build.props` holds, each with the
 * text that shows a project's own file sets it. The lint recipe and the
 * editorconfig rules lean on them: IDE0005 needs the documentation file.
 */
const PROPS_SETTINGS: Readonly<Record<string, RegExp>> = {
	"<Nullable>enable</Nullable>": /<Nullable>\s*enable\s*<\/Nullable>/u,
	"<AnalysisLevel>latest-recommended</AnalysisLevel>":
		/<AnalysisLevel>\s*latest-(?:recommended|all)\s*<\/AnalysisLevel>/u,
	"<TreatWarningsAsErrors>true</TreatWarningsAsErrors>":
		/<TreatWarningsAsErrors>\s*true\s*<\/TreatWarningsAsErrors>/u,
	"<GenerateDocumentationFile>true</GenerateDocumentationFile>":
		/<GenerateDocumentationFile>\s*true\s*<\/GenerateDocumentationFile>/u,
};

/**
 * A note naming the settings a project's own `Directory.Build.props` lacks,
 * since the starter is written only where none exists, and the file is the
 * project's to change.
 */
export const propsNote = (text: string | undefined): string[] => {
	if (text === undefined) {
		return [];
	}
	const missing = Object.entries(PROPS_SETTINGS)
		.filter(([, pattern]) => !pattern.test(text))
		.map(([setting]) => setting);
	return missing.length === 0
		? []
		: [
				`${PROPS}: kept as yours; bedrock's starter also sets ${missing.join(", ")}, which the lint build and the .editorconfig rules rely on.`,
			];
};

/**
 * A note when the test projects need both runners, since `dotnet test` on the
 * .NET 10 SDK runs one per repo: a Microsoft.Testing.Platform project fails in
 * VSTest mode, and a VSTest project fails in the other. Moving the VSTest
 * projects over, such as xUnit v2 to `xunit.v3` or NUnit to
 * `EnableNUnitRunner`, is the project's call.
 */
export const runnerNote = (
	tests: readonly { path: string; runner: TestRunner }[],
	platform: boolean,
): string[] => {
	const vstest = tests.filter(({ runner }) => runner === "vstest");
	const mtp = tests.filter(({ runner }) => runner === "mtp");
	const names = (list: typeof tests): string =>
		list.map(({ path }) => path).join(", ");
	if (vstest.length > 0 && mtp.length > 0) {
		return [
			`${GLOBAL_JSON}: ${names(mtp)} run on Microsoft.Testing.Platform and ${names(vstest)} on VSTest; dotnet test runs one, so move the VSTest projects over and set test.runner to ${TESTING_PLATFORM}.`,
		];
	}
	if (vstest.length > 0 && platform) {
		return [
			`${GLOBAL_JSON}: test.runner is ${TESTING_PLATFORM}, but ${names(vstest)} run on VSTest; move them over or drop test.runner.`,
		];
	}
	return [];
};

/**
 * The .NET part of an adopt: `global.json` on the .NET 10 SDK, and on
 * Microsoft.Testing.Platform when every test project needs it, the pinned
 * formatter and linter tools, and each project moved to `net10.0`, with notes
 * for what a person decides.
 */
export const planDotnet = async (context: Context): Promise<LanguagePlan> => {
	const { root } = context;
	const [tests, current] = await Promise.all([
		testProjects(root),
		readText(join(root, GLOBAL_JSON)),
	]);
	const runners = new Set(tests.map(({ runner }) => runner));
	const onlyPlatform = runners.has("mtp") && !runners.has("vstest");
	const platform = current?.includes(TESTING_PLATFORM) ?? false;
	const [globalJson, manifest, retargets, langNotes, workflow, others, props] =
		await Promise.all([
			globalJsonStep(root, onlyPlatform),
			toolManifestStep(root),
			retargetSteps(root),
			langVersionNotes(root),
			workflowNote(root),
			duplicateWorkflowNotes(root),
			readText(join(root, PROPS)),
		]);
	const steps = [...globalJson, ...manifest, ...retargets];
	return {
		steps,
		notes: [
			...workflow,
			...others,
			...langNotes,
			...propsNote(props),
			...runnerNote(tests, platform),
		],
		projectFiles: [
			GLOBAL_JSON,
			...steps.flatMap((step) => (step.kind === "write" ? [step.path] : [])),
		],
	};
};
