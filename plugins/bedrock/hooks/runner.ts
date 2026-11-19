import type { Engine } from "./engine.ts";

const REPOSITORY = "https://github.com/xsyetopz/bedrock";

// A literal, not an import of plugin.json: Claude Code loads a hooks module only with imports of code files, and refuses the whole module otherwise.
/** The bedrock release this plugin runs; a test keeps it equal to the CLI's and the plugin manifest's version. */
export const VERSION = "0.5.0";

/** Downloads one release asset beside the plugin and checks it against the release's SHA256SUMS, as `just setup` does. */
const INSTALL = String.raw`set -eu
dest="$1"; url="$2"; asset="$3"
mkdir -p "$(dirname "$dest")"
tmp="$(mktemp "$dest.XXXXXX")"
trap 'rm -f "$tmp" "$tmp.sums"' EXIT
curl --fail --silent --show-error --location --retry 3 -o "$tmp" "$url/$asset"
curl --fail --silent --show-error --location --retry 3 -o "$tmp.sums" "$url/SHA256SUMS"
expected="$(awk -v asset="$asset" '$2 == asset { print $1 }' "$tmp.sums")"
actual="$(shasum -a 256 "$tmp" | cut -d ' ' -f 1)"
[ -n "$expected" ] && [ "$expected" = "$actual" ] || { echo "checksum mismatch for $asset" >&2; exit 1; }
chmod +x "$tmp"
mv -f "$tmp" "$dest"`;

// Helpers that take $ are function declarations: Claude Code follows $ past an import only into a function declaration.
async function output($: Engine, argv: readonly string[]): Promise<string> {
	const { exitCode, stdout } = await $.process.run(argv, { timeoutMs: 10_000 });
	return exitCode === 0 ? stdout.trim() : "";
}

/** The release asset name for this machine, such as `bedrock-darwin-arm64`. */
export const assetName = (system: string, machine: string): string => {
	const os = system.toLowerCase();
	const arch = machine === "aarch64" || machine === "arm64" ? "arm64" : "x64";
	return `bedrock-${os}-${arch}`;
};

/**
 * The command that runs bedrock: the source tree when the plugin sits in a
 * bedrock checkout, else `bedrock` on PATH at this plugin's version, else the
 * pinned release, downloaded once beside the plugin.
 */
export async function findBedrock($: Engine): Promise<string[]> {
	const checkout = $.plugin.root.replace(/\/plugins\/[^/]+\/?$/u, "");
	if (await $.fs.exists(`${checkout}/cli/src/main.ts`)) {
		return ["bun", `${checkout}/cli/src/main.ts`];
	}
	if ((await output($, ["bedrock", "version"])) === VERSION) {
		return ["bedrock"];
	}
	const [system, machine] = await Promise.all([
		output($, ["uname", "-s"]),
		output($, ["uname", "-m"]),
	]);
	const asset = assetName(system, machine);
	const binary = `${$.plugin.root}/.bedrock/${VERSION}/${asset}`;
	if (!(await $.fs.exists(binary))) {
		const url = `${REPOSITORY}/releases/download/v${VERSION}`;
		const { exitCode, stderr } = await $.process.run(
			["sh", "-c", INSTALL, "sh", binary, url, asset],
			{ timeoutMs: 300_000 },
		);
		if (exitCode !== 0) {
			throw new Error(
				`Downloading bedrock ${VERSION} (${asset}) failed: ${stderr.trim()}`,
			);
		}
	}
	return [binary];
}
