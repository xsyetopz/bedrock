import {
	ahead,
	behind,
	identifiers,
	isSpace,
	isWord,
	type Line,
	parseLines,
	prefixWhile,
	scalars,
} from "./lexer.ts";
import { type Container, isSuperCall, rewriteLifecycle } from "./lifecycle.ts";

/** Words a test name cannot become once its `test` prefix is gone. */
const KEYWORDS = new Set(
	`class enum extension func import init let var struct protocol static default case if else
	for in is as do guard return switch where while try throw throws true false nil self super
	repeat break continue defer`.split(/\s+/u),
);

const isIndent = (c: string): boolean => c === " " || c === "\t";
const indentOf = (line: string): string => prefixWhile(line, isIndent);
const words = (text: string, separators: string): string[] =>
	text.split(new RegExp(`[${separators}]+`, "u")).filter((word) => word !== "");

/** The rewritten declaration and class name of `class X: XCTestCase`. */
export const rewriteClass = (
	line: string,
): { text: string; name: string } | undefined => {
	const brace = line.indexOf("{");
	const headEnd = brace === -1 ? line.length : brace;
	const head = line.slice(0, headEnd).trim();
	const keyword = head.indexOf("class ");
	if (keyword === -1 || (keyword > 0 && head[keyword - 1] !== " ")) {
		return undefined;
	}
	const afterKeyword = head.slice(keyword + "class ".length);
	const name = prefixWhile(afterKeyword, isWord);
	const after = afterKeyword.slice(name.length).trim();
	const bases = after
		.slice(1)
		.split(",")
		.map((base) => base.trim())
		.filter((base) => base !== "");
	const found = bases.findIndex(
		(base) => base === "XCTestCase" || base === "XCTest.XCTestCase",
	);
	if (!after.startsWith(":") || found === -1) {
		return undefined;
	}
	bases.splice(found, 1);
	const modifiers = head.slice(0, keyword);
	const isFinal = words(modifiers, " ").some(
		(word) => word === "final" || word === "open",
	);
	const inheritance = bases.length === 0 ? "" : `: ${bases.join(", ")}`;
	const tail = line.slice(headEnd);
	let gap = " ";
	if (tail === "") {
		gap = line.endsWith("\r") ? "\r" : "";
	}
	const text = `${indentOf(line)}${modifiers}${isFinal ? "" : "final "}class ${name}${inheritance}${gap}${tail}`;
	return { text, name };
};

const isExtension = (line: string, names: ReadonlySet<string>): boolean => {
	const [keyword, target] = words(line.split("{")[0] ?? "", " \t");
	return (
		keyword === "extension" &&
		target !== undefined &&
		names.has(prefixWhile(target, isWord))
	);
};

const SKIPPED = new Set([
	"override",
	"private",
	"fileprivate",
	"static",
	"class",
]);

/** The name of a `func testX()` declaration that XCTest would run. */
export const testName = (line: string): string | undefined => {
	const found = line.indexOf("func test");
	if (
		found === -1 ||
		words(line.slice(0, found), " ").some((word) => SKIPPED.has(word))
	) {
		return undefined;
	}
	const afterFunc = line.slice(found + "func ".length);
	const name = prefixWhile(afterFunc, isWord);
	return afterFunc.slice(name.length).startsWith("()") ? name : undefined;
};

/** The name without its `test` prefix, or undefined when that is not a valid name. */
const deprefixed = (name: string): string | undefined => {
	const tail = name.slice(4).replace(/^_+/u, "");
	const first = tail[0];
	if (first === undefined || !/^\p{L}$/u.test(first)) {
		return undefined;
	}
	const short = first.toLowerCase() + tail.slice(1);
	return KEYWORDS.has(short) ? undefined : short;
};

/** Names a test could collide with: declared members and called functions. */
const usedNames = (source: string): Set<string> => {
	const b = scalars(source);
	const tokens = identifiers(b);
	const declarers = new Set([
		"func",
		"var",
		"let",
		"case",
		"class",
		"struct",
		"enum",
		"typealias",
	]);
	const used = new Set<string>();
	for (const [k, token] of tokens.entries()) {
		const previous = tokens[k - 1];
		const declared =
			previous !== undefined &&
			declarers.has(previous.name) &&
			b.slice(previous.end, token.start).every((c) => isSpace(c));
		const called =
			ahead(b, token.end).startsWith("(") && behind(b, token.start) !== ".";
		if (declared || called) {
			used.add(token.name);
		}
	}
	return used;
};

/** How each test is named after conversion: without `test` unless that collides. */
const renames = (
	source: string,
	lines: readonly Line[],
): Map<string, string> => {
	const used = usedNames(source);
	const tests = new Set(
		lines.flatMap((line) => {
			const name = line.code ? testName(line.text) : undefined;
			return name === undefined ? [] : [name];
		}),
	);
	const shorts = [...tests].map((name) => deprefixed(name));
	return new Map(
		[...tests].map((name) => {
			const short = deprefixed(name);
			const unique =
				short !== undefined &&
				shorts.filter((other) => other === short).length === 1 &&
				!used.has(short);
			return [name, unique ? short : name];
		}),
	);
};

const testLine = (
	text: string,
	name: string,
	renamed: string,
	cr: string,
): string[] => [
	`${indentOf(text)}@Test${cr}`,
	text.replace(`func ${name}(`, `func ${renamed}(`),
];

const classNamesOf = (lines: readonly Line[]): Set<string> =>
	new Set(
		lines.flatMap((line) => {
			const name = line.code ? rewriteClass(line.text)?.name : undefined;
			return name === undefined ? [] : [name];
		}),
	);

/** Drops containers the line has left and marks the current one as entered. */
const enter = (stack: Container[], line: Line): Container | undefined => {
	while (
		(stack.at(-1)?.seenBody ?? false) &&
		line.depth < (stack.at(-1)?.bodyDepth ?? 0)
	) {
		stack.pop();
	}
	const top = stack.at(-1);
	if (top !== undefined && line.depth >= top.bodyDepth) {
		top.seenBody = true;
	}
	return top;
};

/** Rewrites a member line of a test container. */
const rewriteMember = (
	text: string,
	top: Container,
	plan: ReadonlyMap<string, string>,
	cr: string,
): string[] => {
	const name = testName(text);
	const renamed = name === undefined ? undefined : plan.get(name);
	if (name !== undefined && renamed !== undefined) {
		return testLine(text, name, renamed, cr);
	}
	return [rewriteLifecycle(text, top) ?? text];
};

/** Rewrites test classes into plain classes with `@Test` functions and `init`/`deinit`. */
export const restructure = (source: string): string => {
	const lines = parseLines(source);
	const cr = source.includes("\r") ? "\r" : "";
	const classNames = classNamesOf(lines);
	const plan = renames(source, lines);
	const stack: Container[] = [];
	const out: string[] = [];
	for (const line of lines) {
		const { text } = line;
		if (!line.code || (stack.length > 0 && isSuperCall(text))) {
			if (!line.code) {
				out.push(text);
			}
			continue;
		}
		const top = enter(stack, line);
		const converted = rewriteClass(text)?.text;
		if (converted !== undefined || isExtension(text, classNames)) {
			out.push(converted ?? text);
			stack.push({
				bodyDepth: line.depth + 1,
				seenBody: line.opens > 0,
				hasInit: false,
				hasDeinit: false,
			});
		} else if (top !== undefined && line.depth === top.bodyDepth) {
			out.push(...rewriteMember(text, top, plan, cr));
		} else {
			out.push(text);
		}
	}
	return out.join("\n");
};
