import { asRecord, asStrings, type Json } from "./json.ts";
import { rewriteScript } from "./rewrite-script.ts";

const REMOVED_PACKAGES =
	/^(?:@biomejs\/.+|@eslint(?:-community)?\/.+|@evilmartians\/lefthook|@typescript-eslint\/.+|cross-env|eslint(?:-.+)?|esno|husky|@jest\/.+|@types\/jest|@vitest\/.+|babel-jest|jest(?:-.+)?|ts-jest|vitest|lefthook|lint-staged|npm-run-all2?|rimraf|simple-git-hooks|ts-node(?:-dev)?|tsx|typescript-eslint)$/u;
const OTHER_ENGINES = new Set(["npm", "pnpm", "yarn"]);
const REMOVED_KEYS = [
	"eslintConfig",
	"husky",
	"jest",
	"lint-staged",
	"pnpm",
	"prettier",
	"simple-git-hooks",
];
// `just setup` points git at .githooks, so a script that installs another hook manager does nothing.
const HOOK_INSTALLERS =
	/^(?:typescript-bedrock hooks|husky|lefthook install|simple-git-hooks)(?:\s|$)/u;
// `bun fmt` and `bun check` run these names instead of the built-in command.
const SHADOWING_SCRIPTS = new Set(["check", "fmt"]);

const migrateScripts = (
	scripts: Json,
	templateScripts: Json,
): { scripts: Json; notes: string[] } => {
	const notes: string[] = [];
	const kept: Json = {};
	for (const [name, value] of Object.entries(scripts)) {
		if (typeof value === "string" && HOOK_INSTALLERS.test(value)) {
			notes.push(
				`Removed script "${name}": ${JSON.stringify(value)} installs git hooks, which \`just setup\` does now.`,
			);
			continue;
		}
		const command = typeof value === "string" ? rewriteScript(value) : value;
		const key = SHADOWING_SCRIPTS.has(name) ? `${name}:project` : name;
		if (key !== name) {
			notes.push(
				`Renamed script "${name}" to "${key}": the name shadows \`bun ${name}\`.`,
			);
		}
		if (command !== value) {
			notes.push(
				`Script "${name}": ${JSON.stringify(value)} → ${JSON.stringify(command)}`,
			);
		}
		if (typeof value === "string" && /\b(?:jest|vitest)\b/u.test(value)) {
			notes.push(
				`Script "${name}" ran Jest or Vitest. Move its setup files and other config into bunfig.toml [test], such as preload.`,
			);
		}
		kept[key] = command;
	}
	const { test, ...tools } = templateScripts;
	for (const [name, command] of Object.entries(tools)) {
		if (kept[name] !== undefined && kept[name] !== command) {
			notes.push(
				`Replaced script "${name}" (${JSON.stringify(kept[name])}) with ${JSON.stringify(command)}. Move any step it still needs, such as a build, into "${name}".`,
			);
		}
	}
	return { scripts: { ...kept, test: kept["test"] ?? test, ...tools }, notes };
};

/** Drops the packages Bun and the bedrock tools replace, except the ones the template ships. */
const removePackages = (
	deps: unknown,
	template: Json,
): { deps: Json; removed: string[] } => {
	const removes = (name: string): boolean =>
		REMOVED_PACKAGES.test(name) && !(name in template);
	const entries = Object.entries(asRecord(deps));
	return {
		deps: Object.fromEntries(entries.filter(([name]) => !removes(name))),
		removed: entries.map(([name]) => name).filter((name) => removes(name)),
	};
};

/** Merges pnpm-workspace.yaml and the template catalog into `workspaces`. */
const migrateWorkspaces = (
	current: unknown,
	pnpm: Json | undefined,
	templateCatalog: Json,
): Json => {
	const base = Array.isArray(current)
		? { packages: current }
		: asRecord(current);
	const workspaces: Json = {
		...base,
		packages: [
			...new Set([
				...asStrings(base["packages"]),
				...asStrings(pnpm?.["packages"]),
			]),
		],
		// `bun fmt` does not sort catalogs; the guard requires sorted keys.
		catalog: Object.fromEntries(
			Object.entries({
				...asRecord(base["catalog"]),
				...asRecord(pnpm?.["catalog"]),
				...templateCatalog,
			}).toSorted(([a], [b]) => (a < b ? -1 : 1)),
		),
	};
	const catalogs = {
		...asRecord(base["catalogs"]),
		...asRecord(pnpm?.["catalogs"]),
	};
	return Object.keys(catalogs).length > 0
		? { ...workspaces, catalogs }
		: workspaces;
};

/** Drops npm/pnpm/yarn pins; the template's engines win a conflict. */
const migrateEngines = (manifest: Json, template: Json): Json => ({
	...Object.fromEntries(
		Object.entries(asRecord(manifest["engines"])).filter(
			([name]) => !OTHER_ENGINES.has(name),
		),
	),
	...asRecord(template["engines"]),
});

/** Resolves a shared transitive version once; the template wins a conflict. */
const migrateOverrides = (manifest: Json, template: Json): Json => {
	const overrides = asRecord(template["overrides"]);
	return Object.keys(overrides).length > 0
		? { overrides: { ...asRecord(manifest["overrides"]), ...overrides } }
		: {};
};

/** Bun runs no install scripts unless the package is trusted. */
const trustedDependencies = (manifest: Json, pnpm?: Json): string[] => [
	...new Set([
		...asStrings(asRecord(manifest["pnpm"])["onlyBuiltDependencies"]),
		...asStrings(pnpm?.["onlyBuiltDependencies"]),
	]),
];

export const migrateManifest = (
	manifest: Json,
	template: Json,
	pnpm?: Json,
): { manifest: Json; notes: string[] } => {
	const notes: string[] = [];
	const shipped = asRecord(template["devDependencies"]);
	const dev = removePackages(manifest["devDependencies"], shipped);
	const prod = removePackages(manifest["dependencies"], shipped);
	const removed = [...dev.removed, ...prod.removed];
	if (removed.length > 0) {
		notes.push(`Removed packages Bun replaces: ${removed.join(", ")}.`);
	}
	const scripts = migrateScripts(
		asRecord(manifest["scripts"]),
		asRecord(template["scripts"]),
	);
	const trusted = trustedDependencies(manifest, pnpm);
	const next: Json = {
		...manifest,
		...(Object.keys(prod.deps).length > 0 ||
		manifest["dependencies"] !== undefined
			? { dependencies: prod.deps }
			: {}),
		devDependencies: {
			...dev.deps,
			...asRecord(template["devDependencies"]),
		},
		scripts: scripts.scripts,
		workspaces: migrateWorkspaces(
			manifest["workspaces"],
			pnpm,
			asRecord(asRecord(template["workspaces"])["catalog"]),
		),
		packageManager: template["packageManager"],
		engines: migrateEngines(manifest, template),
		...migrateOverrides(manifest, template),
		...(trusted.length > 0 ? { trustedDependencies: trusted } : {}),
	};
	for (const key of REMOVED_KEYS.filter((name) => name in next)) {
		notes.push(`Removed the "${key}" field from package.json.`);
	}
	return {
		manifest: Object.fromEntries(
			Object.entries(next).filter(([key]) => !REMOVED_KEYS.includes(key)),
		),
		notes: [...notes, ...scripts.notes],
	};
};
