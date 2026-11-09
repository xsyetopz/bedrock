import { afterAll, describe, expect, test } from "bun:test";
import { cp, mkdir, mkdtemp, readlink, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { $ } from "bun";

import { planAdoption } from "./adopt.ts";
import { asRecord, exists } from "./json.ts";
import { applySteps } from "./steps.ts";

const roots: string[] = [];
const tempDir = async (): Promise<string> => {
	const dir = await mkdtemp(join(tmpdir(), "adopt-test-"));
	roots.push(dir);
	return dir;
};
const put = async (
	root: string,
	files: Record<string, string>,
): Promise<void> => {
	await Promise.all(
		Object.entries(files).map(async ([path, content]) => {
			await mkdir(dirname(join(root, path)), { recursive: true });
			await Bun.write(join(root, path), content);
		}),
	);
};
const json = (value: unknown): string => JSON.stringify(value);

const packageFiles: Record<string, string> = {
	"package.json": json({
		name: "typescript-bedrock",
		version: "0.1.0",
		peerDependencies: { oxlint: "1.0.0" },
		engines: { bun: ">=1.4.4" },
	}),
	"tsconfig.base.json": json({
		compilerOptions: { strict: true, types: ["bun"] },
	}),
};

const packageRoot = await tempDir();
await put(packageRoot, packageFiles);
await cp(join(import.meta.dir, "../../files"), join(packageRoot, "files"), {
	recursive: true,
});
const readJson = async (path: string): Promise<Record<string, unknown>> =>
	asRecord(await Bun.file(path).json());

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("planAdoption and applySteps", () => {
	test("refuses to adopt the package itself", async () => {
		const [result] = await Promise.allSettled([
			planAdoption(packageRoot, packageRoot, {}),
		]);
		expect(result).toEqual({
			status: "rejected",
			reason: new Error(
				"The target is typescript-bedrock itself. Pass the path of the repo to adopt.",
			),
		});
	});

	test("a second plan is empty", async () => {
		const target = await tempDir();
		const plan = await planAdoption(packageRoot, target, {});
		await applySteps(target, plan.steps);
		const rerun = await planAdoption(packageRoot, target, {});
		expect(rerun.steps).toEqual([]);
	});

	test.each([
		"typescript-bedrock/files/base",
		"rust-bedrock/files/base",
		"swift-bedrock/Sources/SwiftBedrock/files/base",
		"jvm-bedrock/files/base",
	])("packages/%s is base/ unchanged", async (copy) => {
		const root = join(import.meta.dir, "../../../..");
		const diff = await $`diff -r ${root}/base ${root}/packages/${copy}`
			.nothrow()
			.quiet();
		expect(diff.stdout.toString()).toBe("");
	});

	test("pins typescript-bedrock in the catalog, or keeps a --package spec", async () => {
		const target = await tempDir();
		const plan = await planAdoption(packageRoot, target, {});
		await applySteps(target, plan.steps);
		const pinned = await readJson(join(target, "package.json"));
		expect(asRecord(pinned["devDependencies"])["typescript-bedrock"]).toBe(
			"catalog:",
		);
		expect(asRecord(asRecord(pinned["workspaces"])["catalog"])).toEqual({
			"typescript-bedrock": "0.1.0",
			oxlint: "1.0.0",
		});

		const local = await tempDir();
		const localPlan = await planAdoption(packageRoot, local, {
			packageSpec: "file:../typescript-bedrock",
		});
		await applySteps(local, localPlan.steps);
		const rerun = await planAdoption(packageRoot, local, {});
		expect(rerun.steps).toEqual([]);
		const manifest = await readJson(join(local, "package.json"));
		expect(asRecord(manifest["devDependencies"])["typescript-bedrock"]).toBe(
			"file:../typescript-bedrock",
		);
		expect(manifest["packageManager"]).toBe("bun@1.4.4");
	});

	test("migrates a Prettier, ESLint, and pnpm repo, keeping backups", async () => {
		const target = await tempDir();
		await put(target, {
			"package.json": json({
				name: "app",
				prettier: { semi: false },
				devDependencies: { eslint: "9" },
			}),
			".prettierrc": "{}",
			"eslint.config.js": "",
			"pnpm-workspace.yaml": "packages:\n  - apps/*\n",
			"pnpm-lock.yaml": "",
			"tsconfig.json": json({ compilerOptions: { esModuleInterop: true } }),
			".gitattributes": "mine\n",
			Makefile: "all:",
			"CLAUDE.md": "- own rule\n",
			".github/dependabot.yml": "old",
		});
		const plan = await planAdoption(packageRoot, target, {});
		expect(plan.lockfiles).toEqual(["pnpm-lock.yaml"]);
		expect(plan.notes.join("\n")).toContain('Removed the "prettier" field');
		await applySteps(target, plan.steps);

		const attributes = await Bun.file(join(target, ".gitattributes")).text();
		expect(attributes).toStartWith("mine\n\n# >>> bedrock\n");
		expect(attributes).toContain("# >>> typescript-bedrock\nbun.lock");
		expect(await Bun.file(join(target, "AGENTS.md")).text()).toStartWith(
			"- own rule\n",
		);
		expect(await readlink(join(target, "CLAUDE.md"))).toBe("AGENTS.md");
		expect(await Bun.file(join(target, "CLAUDE.md.bak")).text()).toBe(
			"- own rule\n",
		);
		expect(await exists(join(target, "Makefile.bak"))).toBe(true);
		expect(
			await Bun.file(join(target, ".github/dependabot.yml.bak")).text(),
		).toBe("old");
		expect(await exists(join(target, "eslint.config.js"))).toBe(false);
		expect(await exists(join(target, "eslint.config.js.bak"))).toBe(true);
		expect(await exists(join(target, "pnpm-workspace.yaml.bak"))).toBe(true);
		const bunfig = await Bun.file(join(target, "bunfig.toml")).text();
		expect(bunfig).toContain("useTabs = true");
		expect(await exists(join(target, ".prettierrc.bak"))).toBe(true);
		const manifest = asRecord(
			await Bun.file(join(target, "package.json")).json(),
		);
		expect(asRecord(manifest["workspaces"])["packages"]).toEqual(["apps/*"]);
		expect(manifest["prettier"]).toBeUndefined();
		const tsconfig = asRecord(
			await Bun.file(join(target, "tsconfig.json")).json(),
		);
		expect(tsconfig["extends"]).toBe("typescript-bedrock/tsconfig.base.json");
		const oxlint = await readJson(join(target, ".oxlintrc.json"));
		expect(oxlint["extends"]).toEqual([
			"./node_modules/typescript-bedrock/oxlintrc.json",
		]);
		const hook = await stat(join(target, ".githooks/pre-commit"));
		expect(hook.mode & 0o111).not.toBe(0);

		await applySteps(target, plan.steps);
		expect(await Bun.file(join(target, "package.json.bak")).exists()).toBe(
			true,
		);
	});

	test("starts an empty directory from the template and replaces another formatter", async () => {
		const target = await tempDir();
		await put(target, {
			".prettierrc": "{}",
			"package.json": json({ prettier: {} }),
		});
		const plan = await planAdoption(packageRoot, target, {});
		await applySteps(target, plan.steps);
		expect(await exists(join(target, ".prettierrc"))).toBe(false);
		expect(await Bun.file(join(target, "bunfig.toml")).text()).toContain(
			"[fmt]",
		);
		expect(
			asRecord(await Bun.file(join(target, "package.json")).json())["prettier"],
		).toBeUndefined();

		const empty = await tempDir();
		const fresh = await planAdoption(packageRoot, empty, {});
		expect(fresh.steps.some((step) => step.kind === "remove")).toBe(false);
	});

	test("keeps the settings a repo adds to bunfig.toml", async () => {
		const target = await tempDir();
		await put(target, {
			"bunfig.toml":
				'[test]\n# Cached packages carry their own tests.\npathIgnorePatterns = ["captures/**"]\n\n[fmt]\nignorePatterns = [\n\t# Built and committed.\n\t"cli/bin/",\n]\n',
		});
		const plan = await planAdoption(packageRoot, target, {});
		await applySteps(target, plan.steps);
		const bunfig = await Bun.file(join(target, "bunfig.toml")).text();
		expect(bunfig).toContain(
			'# Cached packages carry their own tests.\npathIgnorePatterns = ["captures/**"]\n',
		);
		expect(bunfig).toContain('\t# Built and committed.\n\t"cli/bin/",\n]');
		expect(bunfig).toContain("useTabs = true");
		const rerun = await planAdoption(packageRoot, target, {});
		expect(rerun.steps).toEqual([]);
	});

	test("leaves a tsconfig alone when only its layout differs", async () => {
		const target = await tempDir();
		await put(target, {
			"tsconfig.json":
				'{\n\t"$schema": "https://json.schemastore.org/tsconfig.json",\n\t// mine\n\t"extends": "typescript-bedrock/tsconfig.base.json", "compilerOptions": {},\n\t"exclude": ["**/.build", "**/build", "**/coverage", "**/dist", "**/target"]\n}\n',
		});
		const plan = await planAdoption(packageRoot, target, {});
		expect(plan.steps.map((step) => step.path)).not.toContain("tsconfig.json");
	});
});
