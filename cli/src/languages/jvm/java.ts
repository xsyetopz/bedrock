import { readdir } from "node:fs/promises";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";

/** A JDK that can start Gradle: `home` is what to export as JAVA_HOME, absent when `java` on PATH works. */
export interface Jdk {
	readonly home?: string;
}

/** Homebrew's JDK formulae, newest supported first, under each Homebrew prefix. */
const BREW_FORMULAE = ["openjdk@25", "openjdk@21", "openjdk@17", "openjdk"];
const BREW_PREFIXES = ["/opt/homebrew", "/usr/local"];

const runs = async (
	command: readonly string[]
): Promise<string | undefined> => {
	try {
		const child = Bun.spawn([...command], { stdout: "pipe", stderr: "ignore" });
		const [out, code] = await Promise.all([
			new Response(child.stdout).text(),
			child.exited
		]);
		return code === 0 ? out.trim() : undefined;
	} catch {
		return undefined;
	}
};

/** Directories under `parent` whose names start with `prefix`, highest name first. */
const listed = async (parent: string, prefix: string): Promise<string[]> => {
	const names = await readdir(parent).catch(() => []);
	return names
		.filter((name) => name.startsWith(prefix))
		.toSorted((left, right) =>
			right.localeCompare(left, "en", { numeric: true })
		)
		.map((name) => join(parent, name));
};

const candidates = async (): Promise<string[]> => {
	const brew = BREW_PREFIXES.flatMap((prefix) =>
		BREW_FORMULAE.map((name) => join(prefix, "opt", name))
	);
	const more = await Promise.all([
		...BREW_PREFIXES.map((prefix) => listed(join(prefix, "opt"), "openjdk@")),
		listed("/usr/lib/jvm", "")
	]);
	return [...brew, ...more.flat()].flatMap((path) => [
		join(path, "libexec/openjdk.jdk/Contents/Home"),
		path
	]);
};

/**
 * Finds a JDK the way the `gradle` recipe in `.just/jvm.just` does: a valid
 * JAVA_HOME, a `java` that runs (macOS's `/usr/bin/java` stub fails), macOS's
 * `java_home`, then Homebrew and `/usr/lib/jvm`. Returns undefined when none runs.
 */
export const findJdk = async (
	env: Readonly<Record<string, string | undefined>> = process.env
): Promise<Jdk | undefined> => {
	const configured = env["JAVA_HOME"];
	if (
		configured !== undefined &&
		(await exists(join(configured, "bin/java")))
	) {
		return { home: configured };
	}
	if ((await runs(["java", "-version"])) !== undefined) {
		return {};
	}
	const reported = await runs(["/usr/libexec/java_home"]);
	if (reported !== undefined && reported !== "") {
		return { home: reported };
	}
	const paths = await candidates();
	const present = await Promise.all(
		paths.map((path) => exists(join(path, "bin/java")))
	);
	const home = paths.find((_, index) => present[index] === true);
	return home === undefined ? undefined : { home };
};

/** What to do when no JDK is found. */
export const JDK_MISSING =
	"Gradle needs a JDK 17 or newer to start, and none runs here. Install one, such as brew install openjdk@25 or your system's openjdk package; bedrock finds Homebrew's and /usr/lib/jvm's JDKs itself, or set JAVA_HOME.";
