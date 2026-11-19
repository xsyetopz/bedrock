import { Glob } from "bun";

import { listFiles } from "../../core/fs.ts";
import type { Context, Finding } from "../../core/language.ts";

/** Dependency managers that Go modules replaced, each with its name. */
const FOREIGN: Readonly<Record<string, string>> = {
	"Gopkg.toml": "dep",
	"Gopkg.lock": "dep",
	"glide.yaml": "Glide",
	"glide.lock": "Glide",
	"vendor/vendor.json": "govendor",
	"Godeps/Godeps.json": "Godep"
};

/** Other names golangci-lint reads, which would sit beside `.golangci.yml`. */
const OTHER_CONFIGS = new Set([
	".golangci.yaml",
	".golangci.toml",
	".golangci.json"
]);

/** Reports dependency manager files and a second golangci-lint config; `go mod tidy` and golangci-lint cover the rest. */
export const guardGo = async (context: Context): Promise<Finding[]> => {
	const files = await listFiles(context.root, new Glob("**"));
	return files.flatMap((path): Finding[] => {
		const tool = FOREIGN[path];
		if (tool !== undefined) {
			return [
				`${path}: ${tool} file; Go modules replace it, so go.mod and go.sum hold the dependencies`
			];
		}
		return OTHER_CONFIGS.has(path)
			? [
					`${path}: bedrock writes .golangci.yml, so keep the settings there and delete this file`
				]
			: [];
	});
};
