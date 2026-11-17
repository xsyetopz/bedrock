#!/usr/bin/env bun
/**
 * Builds the release binaries into `dist/`, with the templates embedded, and
 * writes `SHA256SUMS`, which `just setup` checks each download against.
 * Asset names match `bedrock_asset` in templates/base/files/justfile.tmpl.
 */
import { basename } from "node:path";

import { $ } from "bun";

const TARGETS = ["darwin-arm64", "darwin-x64", "linux-arm64", "linux-x64"];
const OUT = "dist";

const build = async (target: string): Promise<string> => {
	const outfile = `${OUT}/bedrock-${target}`;
	// The binary runs inside other people's repos, so their .env and bunfig.toml must not change it.
	await $`bun build cli/src/main.ts --compile --target=bun-${target} --asset=templates --no-compile-autoload-dotenv --no-compile-autoload-bunfig --outfile ${outfile}`.quiet();
	const hash = new Bun.CryptoHasher("sha256")
		.update(await Bun.file(outfile).arrayBuffer())
		.digest("hex");
	return `${hash}  ${basename(outfile)}`;
};

await $`rm -rf ${OUT}`;
const sums = await Promise.all(TARGETS.map((target) => build(target)));
await Bun.write(`${OUT}/SHA256SUMS`, `${sums.join("\n")}\n`);
console.info(sums.join("\n"));
