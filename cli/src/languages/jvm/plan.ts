import { basename, join } from "node:path";

import { markers, upsert } from "../../core/blocks.ts";
import { existing, readText } from "../../core/fs.ts";
import type { Context, LanguagePlan } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { kindSteps, packageName } from "./kinds.ts";
import { migrationFor } from "./migration.ts";
import type { Migration } from "./model.ts";
import { includes, render } from "./render.ts";

/**
 * What older bedrock releases wrote: files under gradle/bedrock, which core's
 * drift check replaced, and the wrapper jar as base64, which bedrock now
 * writes as the jar itself.
 */
const LEGACY = [
	"gradle/bedrock/managed.sha256",
	"gradle/bedrock/src/main/kotlin/bedrock/core/Guard.kt",
	"gradle/wrapper/gradle-wrapper.jar.b64",
];

const ROOT_NAME = /rootProject\.name\s*=\s*['"]([^'"]+)['"]/u;
const BLOCK = "jvm-bedrock";

/** The group option, or the package a new project's code lives in. */
export const groupOf = (context: Context): string | undefined => {
	const group = context.options["group"];
	return typeof group === "string" ? group : undefined;
};

/** The block's text first in the file: plugins and pluginManagement must precede the rest. */
const blockFirst = (
	current: string | undefined,
	header: string,
	content: string,
): string => {
	const [open] = markers("slash", BLOCK);
	if (current?.includes(open) === true) {
		return upsert(current, header, "slash", [{ name: BLOCK, content }]);
	}
	const marked = upsert(undefined, "", "slash", [{ name: BLOCK, content }]);
	return `${marked}\n${current ?? header}`;
};

const includeLine = (modules: readonly string[]): string =>
	modules.length === 0
		? ""
		: `include(${modules.map((module) => `":${module}"`).join(", ")})\n`;

const settingsStep = async (
	context: Context,
	migration: Migration | undefined,
	modules: readonly string[],
): Promise<Step> => {
	const { root } = context;
	const [kts, groovy, block] = await Promise.all([
		readText(join(root, "settings.gradle.kts")),
		readText(join(root, "settings.gradle")),
		context.templates("blocks").read("settings.gradle.kts", {}),
	]);
	const name = ROOT_NAME.exec(kts ?? groovy ?? "")?.[1] ?? basename(root);
	// A Gradle build's own settings would apply plugins before bedrock's, so they start over.
	const current = migration?.tool === "Gradle" ? undefined : kts;
	let settings = blockFirst(
		current,
		`rootProject.name = "${name}"\n`,
		block ?? "",
	);
	const lines =
		migration === undefined ? includeLine(modules) : includes(migration);
	if (lines !== "" && !settings.includes(lines)) {
		settings += lines;
	}
	return { kind: "write", path: "settings.gradle.kts", content: settings };
};

const propertiesStep = async (
	context: Context,
	migration: Migration | undefined,
	fallback: string | undefined,
): Promise<{ step: Step; notes: string[] }> => {
	const [current, block] = await Promise.all([
		readText(join(context.root, "gradle.properties")),
		context.templates("blocks").read("gradle.properties", {}),
	]);
	let text = upsert(current, "", "hash", [
		{ name: BLOCK, content: block ?? "" },
	]);
	const notes: string[] = [];
	const option = groupOf(context);
	const declared = /^group\s*=\s*(.*?)\s*$/mu.exec(text);
	if (declared === null) {
		const chosen = option ?? migration?.group ?? fallback;
		if (chosen === undefined) {
			notes.push(
				"gradle.properties: set group= to the package the code lives in; NullAway checks it and Android derives namespaces from it",
			);
		} else {
			text += `group=${chosen}\n`;
		}
	} else if (option !== undefined && declared[1] !== option) {
		// The group option, kept in .editorconfig as bedrock_jvm_group, is what the user chose last.
		text = text.replace(declared[0], `group=${option}`);
		notes.push(
			`gradle.properties: group ${declared[1] ?? ""} becomes ${option}, bedrock_jvm_group in .editorconfig; NullAway and the Android namespaces follow it.`,
		);
	}
	return {
		step: { kind: "write", path: "gradle.properties", content: text },
		notes,
	};
};

/** Plans the jvm part of an adopt: build tool migration, settings, properties, kinds, and legacy removals. */
export const planJvm = async (context: Context): Promise<LanguagePlan> => {
	const { root } = context;
	const migration = await migrationFor(root);
	const name = basename(root);
	const fallback = `com.example.${packageName(name)}`;
	const group = groupOf(context) ?? fallback;
	const variables = {
		name,
		package: group,
		path: group.replaceAll(".", "/"),
	};
	const kinds = context.init
		? await kindSteps(context, variables)
		: { steps: [], modules: [] };
	const kindGroup = kinds.steps.length > 0 ? fallback : undefined;
	const [settings, properties, legacy, migrated] = await Promise.all([
		settingsStep(context, migration, kinds.modules),
		propertiesStep(context, migration, kindGroup),
		existing(root, LEGACY),
		migration === undefined ? [] : render(root, migration),
	]);
	const steps: Step[] = [
		settings,
		properties.step,
		...migrated,
		...kinds.steps,
		...legacy.map((path): Step => ({ kind: "remove", path })),
	];
	return {
		steps,
		notes: [...properties.notes, ...(migration?.notes ?? [])],
		projectFiles: [
			"settings.gradle.kts",
			"gradle.properties",
			...[...migrated, ...kinds.steps]
				.filter((step) => step.kind === "write")
				.map((step) => step.path),
		],
	};
};
