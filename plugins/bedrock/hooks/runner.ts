import type { Engine } from "./engine.ts";

const REPOSITORY = "https://github.com/xsyetopz/bedrock";

// A literal, not an import of plugin.json: Claude Code loads a hooks module only with imports of code files, and refuses the whole module otherwise.
/** The bedrock release this plugin runs; a test keeps it equal to the CLI's and the plugin manifest's version. */
export const VERSION = "0.7.1";

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
// $.process.run rejects when the program can't start, as `bedrock` off PATH can't; that probe has no output.
async function output($: Engine, argv: readonly string[]): Promise<string> {
	try {
		const { exitCode, stdout } = await $.process.run(argv, {
			timeoutMs: 10_000
		});
		return exitCode === 0 ? stdout.trim() : "";
	} catch {
		return "";
	}
}

// `go run` would report a failed check as exit 1 whatever bedrock returned, and
// it resolves the module from the working folder, which is the user's repo.
/** Builds the checkout's bedrock into its gitignored `bin/` and returns the binary. */
async function buildSource($: Engine, checkout: string): Promise<string> {
	const binary = `${checkout}/bin/bedrock`;
	const { exitCode, stderr } = await $.process.run(
		["go", "-C", checkout, "build", "-o", binary, "./cmd/bedrock"],
		{ timeoutMs: 300_000 }
	);
	if (exitCode !== 0) {
		throw new Error(
			`Building bedrock from ${checkout} failed: ${stderr.trim()}`
		);
	}
	return binary;
}

/** The release asset name for this machine, such as `bedrock-darwin-arm64` or `bedrock-windows-x64.exe`. */
export const assetName = (system: string, machine: string): string => {
	// Git Bash, MSYS2, and Cygwin report Windows as MINGW64_NT-10.0, MSYS_NT-10.0, or CYGWIN_NT-10.0.
	const windows = /^(?:mingw|msys|cygwin)/iu.test(system);
	const os = windows ? "windows" : system.toLowerCase();
	const arch = machine === "aarch64" || machine === "arm64" ? "arm64" : "x64";
	// Windows runs a program only by its .exe name.
	return `bedrock-${os}-${arch}${windows ? ".exe" : ""}`;
};

/**
 * The command that runs bedrock: a build of the source tree when the plugin
 * sits in a bedrock checkout, else `bedrock` on PATH at this plugin's version, else the
 * pinned release, downloaded once beside the plugin.
 */
export async function findBedrock($: Engine): Promise<string[]> {
	const checkout = $.plugin.root.replace(/\/plugins\/[^/]+\/?$/u, "");
	if (await $.fs.exists(`${checkout}/cmd/bedrock/main.go`)) {
		return [await buildSource($, checkout)];
	}
	if ((await output($, ["bedrock", "version"])) === VERSION) {
		return ["bedrock"];
	}
	const [system, machine] = await Promise.all([
		output($, ["uname", "-s"]),
		output($, ["uname", "-m"])
	]);
	const asset = assetName(system, machine);
	const binary = `${$.plugin.root}/.bedrock/${VERSION}/${asset}`;
	if (!(await $.fs.exists(binary))) {
		const url = `${REPOSITORY}/releases/download/v${VERSION}`;
		const { exitCode, stderr } = await $.process.run(
			["sh", "-c", INSTALL, "sh", binary, url, asset],
			{ timeoutMs: 300_000 }
		);
		if (exitCode !== 0) {
			throw new Error(
				`Downloading bedrock ${VERSION} (${asset}) failed: ${stderr.trim()}`
			);
		}
	}
	return [binary];
}
