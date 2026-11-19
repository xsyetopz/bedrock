// Port of compilerla/conventional-pre-commit v4.5.0 (Apache-2.0), default options:
// https://github.com/compilerla/conventional-pre-commit/blob/v4.5.0/conventional_pre_commit/format.py
// Changed: the subject must be the first line, where upstream accepts a conventional line
// anywhere in the message. Each line holds at most 100 characters, the limit of
// @commitlint/config-conventional, counted as graphemes so accents and emoji count once.

const SCISSORS = "# ------------------------ >8 ------------------------";
const TYPES = "build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test";
const SUBJECT = new RegExp(
	`^(?:${TYPES})(?:\\([a-z0-9_ :,./#-]+\\))?!?: .+$`,
	"u"
);
const EXEMPT = /^(?:amend|fixup|squash)! /u;
const MERGE = /^merge(?:[^a-z0-9_]|$)/u;
const BREAKING = /^BREAKING[ -]CHANGE: /u;
const MAX_LINE = 100;
const GRAPHEMES = new Intl.Segmenter(undefined, { granularity: "grapheme" });

const USAGE = `Usage: bedrock commit-msg <file>

Checks the commit message in <file>, as the commit-msg hook hands it, against
Conventional Commits with no line over ${MAX_LINE} characters.`;

const HELP = `commit-msg: use Conventional Commits, for example "feat(cli): add \`--dry-run\`", with no line over ${MAX_LINE} characters.
Types: ${TYPES.replaceAll("|", ", ")}. https://www.conventionalcommits.org`;

/**
 * The lines git keeps from `text`: those above the scissors line that are not
 * comments. A message of nothing but comments keeps every line, as upstream does.
 */
const keptLines = (text: string): string[] => {
	const raw = text.split("\n");
	if (raw.at(-1) === "") {
		raw.pop();
	}
	const cut = raw.indexOf(SCISSORS);
	const kept = (cut === -1 ? raw : raw.slice(0, cut)).filter(
		(line) => !line.startsWith("#")
	);
	return kept.length > 0 ? kept : raw;
};

/** Whether `text` is a commit message the commit-msg hook accepts. */
export const isConventional = (text: string): boolean => {
	const lines = keptLines(text);
	const [subject = "", second, third = ""] = lines;
	if (EXEMPT.test(subject) || MERGE.test(subject.toLowerCase())) {
		return true;
	}
	if (!SUBJECT.test(subject.toLowerCase())) {
		return false;
	}
	if (lines.some((line) => [...GRAPHEMES.segment(line)].length > MAX_LINE)) {
		return false;
	}
	if (second !== undefined && second !== "") {
		return false;
	}
	// A breaking-change footer must be upper case, as the spec writes it.
	return !(BREAKING.test(third.toUpperCase()) && !BREAKING.test(third));
};

/** `bedrock commit-msg <file>`. */
export const commitMsg = async (args: readonly string[]): Promise<number> => {
	const [path, ...rest] = args;
	if (path === "-h" || path === "--help") {
		console.info(USAGE);
		return 0;
	}
	const file = path === undefined ? undefined : Bun.file(path);
	if (
		file !== undefined &&
		rest.length === 0 &&
		(await file.exists()) &&
		isConventional(await file.text())
	) {
		return 0;
	}
	console.error(HELP);
	return 1;
};
