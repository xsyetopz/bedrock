import { join } from "node:path";

import { existing, exists } from "../../core/fs.ts";
import { ant } from "./ant.ts";
import { gradle, isGradle } from "./gradle.ts";
import { maven } from "./maven.ts";
import { foreignSources, type Migration, newMigration } from "./model.ts";
import { sbt } from "./sbt.ts";

/** Build tools adopt finds but cannot read. */
const UNREAD: Readonly<Record<string, string>> = {
	"build.sc": "Mill",
	"build.mill": "Mill",
	"MODULE.bazel": "Bazel",
	WORKSPACE: "Bazel",
};

const present = (root: string, name: string): Promise<boolean> =>
	exists(join(root, name));

const migrationOf = async (root: string): Promise<Migration | undefined> => {
	const [pom, build, antBuild, ivy] = await Promise.all(
		["pom.xml", "build.sbt", "build.xml", "ivy.xml"].map((name) =>
			present(root, name),
		),
	);
	if (pom === true) {
		return maven(root);
	}
	if (build === true) {
		return sbt(root);
	}
	if (antBuild === true || ivy === true) {
		return ant(root);
	}
	return (await isGradle(root)) ? gradle(root) : undefined;
};

/**
 * The migration for the build tool in `root`, or undefined when it has none.
 * Scala and Groovy sources and builds adopt cannot read become notes, since
 * the rest of the repo still adopts.
 */
export const migrationFor = async (
	root: string,
): Promise<Migration | undefined> => {
	const [migration, foreign, unread] = await Promise.all([
		migrationOf(root),
		foreignSources(root),
		existing(root, Object.keys(UNREAD)),
	]);
	const notes = [
		...(foreign.length > 0
			? [
					`${foreign.join(", ")}: bedrock compiles Java and Kotlin, not Scala or Groovy; port those sources first`,
				]
			: []),
		...unread.map(
			(file) =>
				`${file}: adopt cannot read ${UNREAD[file] ?? "this tool"} builds; write each module's build.gradle.kts by hand`,
		),
	];
	if (notes.length === 0) {
		return migration;
	}
	const noted = migration ?? newMigration(UNREAD[unread[0] ?? ""] ?? "Scala");
	for (const note of notes) {
		noted.notes.add(note);
	}
	return noted;
};
