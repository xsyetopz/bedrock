import { join } from "node:path";

import { asRecord, type Json, readJsonc } from "./json.ts";

const NAME = "typescript-bedrock";
/** The same script names in every repo; the commands come from typescript-bedrock. */
const SCRIPTS = {
	format: "bun fmt",
	"format:check": "bun fmt --check",
	guard: "typescript-bedrock guard",
	lint: "bun run --sequential --no-exit-on-error oxlint markdownlint-cli2",
	"lint:fix":
		"bun run --sequential --no-exit-on-error 'oxlint --fix' 'markdownlint-cli2 --fix'",
	test: "bun test --pass-with-no-tests",
	typecheck: "bun check",
	verify: "bun run --sequential format:check typecheck lint guard test",
};

/**
 * Picks where the repo gets typescript-bedrock from: the --package spec, then the
 * repo's own non-catalog spec, then its catalog pin, then this version.
 */
const bedrockSpec = (
	manifest: Json,
	own: Json,
	packageSpec: string | undefined,
): string => {
	const current = asRecord(manifest["devDependencies"])[NAME];
	const pinned = asRecord(asRecord(manifest["workspaces"])["catalog"])[NAME];
	if (packageSpec !== undefined) {
		return packageSpec;
	}
	if (typeof current === "string" && current !== "catalog:") {
		return current;
	}
	return typeof pinned === "string" ? pinned : String(own["version"]);
};

/** The package.json fields every adopted repo shares. */
export const templateManifest = async (
	packageRoot: string,
	manifest: Json,
	packageSpec: string | undefined,
): Promise<Json> => {
	const own = (await readJsonc(join(packageRoot, "package.json"))) ?? {};
	// In this package's own workspace the peers read `catalog:`; `bun publish` replaces it with versions.
	const workspace = await readJsonc(join(packageRoot, "../../package.json"));
	const rootCatalog = asRecord(asRecord(workspace?.["workspaces"])["catalog"]);
	const tools = Object.entries(asRecord(own["peerDependencies"])).map(
		([name, range]): [string, unknown] => [
			name,
			range === "catalog:" ? rootCatalog[name] : range,
		],
	);
	const catalog = Object.fromEntries(tools);
	const spec = bedrockSpec(manifest, own, packageSpec);
	// `workspace:`, `file:`, `link:`, and git specs cannot live in a catalog.
	const fromCatalog = !/[:/]/u.test(spec);
	const bunVersion = await Bun.file(
		join(packageRoot, "files/.bun-version"),
	).text();
	return {
		scripts: SCRIPTS,
		devDependencies: {
			...Object.fromEntries(tools.map(([name]) => [name, "catalog:"])),
			[NAME]: fromCatalog ? "catalog:" : spec,
		},
		workspaces: {
			catalog: { ...catalog, ...(fromCatalog ? { [NAME]: spec } : {}) },
		},
		// A `catalog:` override does not dedupe, so it repeats the version.
		overrides: { "@types/node": catalog["@types/node"] },
		packageManager: `bun@${bunVersion.trim()}`,
		engines: own["engines"],
	};
};
