/** The bounds of one `{num1..num2}` in a glob, in capture-group order. */
type Range = readonly [number, number];

type Token = readonly [regex: string, next: number];

const RANGE = /^([+-]?\d+)\.\.([+-]?\d+)$/u;

const escape = (char: string): string =>
	/[\\^$.*+?()[\]{}|/]/u.test(char) ? `\\${char}` : char;

/** The index of the `}` that closes the `{` at `open`, or -1 when none does. */
const closing = (glob: string, open: number): number => {
	let depth = 0;
	let index = open;
	while (index < glob.length) {
		const char = glob[index];
		if (char === "\\") {
			index += 1;
		} else if (char === "{") {
			depth += 1;
		} else if (char === "}") {
			depth -= 1;
			if (depth === 0) {
				return index;
			}
		}
		index += 1;
	}
	return -1;
};

/** `body` split at the commas outside any nested braces. */
const options = (body: string): string[] => {
	const parts = [""];
	let depth = 0;
	let index = 0;
	while (index < body.length) {
		const char = body[index] ?? "";
		const pair = char === "\\" ? body.slice(index, index + 2) : char;
		depth += (char === "{" ? 1 : 0) - (char === "}" ? 1 : 0);
		if (char === "," && depth === 0) {
			parts.push("");
		} else {
			parts[parts.length - 1] += pair;
		}
		index += pair.length;
	}
	return parts;
};

/** `[name]` or `[!name]`; a `[` with no `]`, or one around a `/`, is literal. */
const bracket = (glob: string, open: number): Token => {
	const end = glob.indexOf("]", open + 1);
	const body = glob.slice(open + 1, end);
	if (end === -1 || body === "" || body === "!" || body.includes("/")) {
		return [String.raw`\[`, open + 1];
	}
	const negated = body.startsWith("!");
	const chars = (negated ? body.slice(1) : body).replaceAll(
		/[\\[\]^]/gu,
		(char) => `\\${char}`,
	);
	return [`[${negated ? "^" : ""}${chars}]`, end + 1];
};

/** `{num1..num2}`, `{s1,s2}`, or a literal `{s}`, which has no comma. */
const brace = (glob: string, open: number, ranges: Range[]): Token => {
	const end = closing(glob, open);
	if (end === -1) {
		return [String.raw`\{`, open + 1];
	}
	const body = glob.slice(open + 1, end);
	const range = RANGE.exec(body);
	if (range !== null) {
		ranges.push([Number(range[1]), Number(range[2])]);
		return [String.raw`([+-]?\d+)`, end + 1];
	}
	const parts = options(body);
	if (parts.length < 2) {
		return [`\\{${translate(body, ranges)}\\}`, end + 1];
	}
	const alternatives = parts.map((part) => translate(part, ranges));
	return [`(?:${alternatives.join("|")})`, end + 1];
};

const token = (glob: string, index: number, ranges: Range[]): Token => {
	const char = glob[index] ?? "";
	switch (char) {
		case "\\": {
			return [escape(glob[index + 1] ?? "\\"), index + 2];
		}
		case "*": {
			if (glob[index + 1] !== "*") {
				return ["[^/]*", index + 1];
			}
			return glob[index + 2] === "/"
				? ["(?:.*/)?", index + 3]
				: [".*", index + 2];
		}
		case "?": {
			return ["[^/]", index + 1];
		}
		case "[": {
			return bracket(glob, index);
		}
		case "{": {
			return brace(glob, index, ranges);
		}
		default: {
			return [escape(char), index + 1];
		}
	}
};

/** The regex source for `glob`, recording each `{num1..num2}` in `ranges`. */
const translate = (glob: string, ranges: Range[]): string => {
	let source = "";
	let index = 0;
	while (index < glob.length) {
		const [regex, next] = token(glob, index, ranges);
		source += regex;
		index = next;
	}
	return source;
};

/**
 * Whether a path, relative to the `.editorconfig` directory, matches the
 * section glob as the EditorConfig spec reads it: `*`, `**`, `?`, `[name]`,
 * `[!name]`, `{s1,s2}`, and `{num1..num2}`, which Bun.Glob lacks. A glob
 * with no `/` matches at any depth, and one with a `/` from that directory.
 */
export const globMatcher = (glob: string): ((path: string) => boolean) => {
	const ranges: Range[] = [];
	const anchored = glob.includes("/") ? glob.replace(/^\//u, "") : `**/${glob}`;
	const regex = new RegExp(`^${translate(anchored, ranges)}$`, "u");
	return (path) => {
		const match = regex.exec(path);
		return (
			match !== null &&
			ranges.every(([from, to], index) => {
				const captured = match[index + 1];
				const value = Number(captured);
				return (
					captured === undefined ||
					(value >= Math.min(from, to) && value <= Math.max(from, to))
				);
			})
		);
	};
};
