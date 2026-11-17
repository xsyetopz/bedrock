import {
	isIdent,
	isSpace,
	matchClose,
	type Scalars,
	scalars,
	trivia,
} from "./lexer.ts";
import { parseArguments, splitArguments, wrap } from "./operands.ts";

type Assertion =
	| { readonly kind: "truth"; readonly negated: boolean }
	| { readonly kind: "compare"; readonly op: string }
	| { readonly kind: "nilCheck"; readonly op: string }
	| { readonly kind: "closure"; readonly type: string }
	| { readonly kind: "unwrap" }
	| { readonly kind: "fail" };

const truth = (negated: boolean): Assertion => ({ kind: "truth", negated });
const compare = (op: string): Assertion => ({ kind: "compare", op });
const nilCheck = (op: string): Assertion => ({ kind: "nilCheck", op });

const ASSERTIONS: ReadonlyMap<string, Assertion> = new Map([
	["XCTAssert", truth(false)],
	["XCTAssertTrue", truth(false)],
	["XCTAssertFalse", truth(true)],
	["XCTAssertEqual", compare("==")],
	["XCTAssertNotEqual", compare("!=")],
	["XCTAssertGreaterThan", compare(">")],
	["XCTAssertGreaterThanOrEqual", compare(">=")],
	["XCTAssertLessThan", compare("<")],
	["XCTAssertLessThanOrEqual", compare("<=")],
	["XCTAssertIdentical", compare("===")],
	["XCTAssertNotIdentical", compare("!==")],
	["XCTAssertNil", nilCheck("==")],
	["XCTAssertNotNil", nilCheck("!=")],
	["XCTUnwrap", { kind: "unwrap" }],
	["XCTFail", { kind: "fail" }],
	["XCTAssertThrowsError", { kind: "closure", type: "(any Error).self" }],
	["XCTAssertNoThrow", { kind: "closure", type: "Never.self" }],
]);

/** The text a call is replaced by, and the index after the call. */
interface Replacement {
	readonly text: string;
	readonly end: number;
}

const EFFECTS = ["try", "await"] as const;

const leadingEffect = (text: string): string | undefined =>
	EFFECTS.find((effect) => text.startsWith(`${effect} `));

/** Builds one replacement call, tracking the `try` and `await` its operands carried. */
class Builder {
	private readonly effects = new Set<string>();

	private readonly positional: readonly string[];
	private readonly accuracy: string | undefined;

	constructor(positional: readonly string[], accuracy: string | undefined) {
		this.positional = positional;
		this.accuracy = accuracy;
	}

	private operand(index: number, wrapped = true): string {
		let text = this.positional[index] ?? "";
		let word = leadingEffect(text);
		while (word !== undefined) {
			this.effects.add(word);
			text = text.slice(word.length).trim();
			word = leadingEffect(text);
		}
		return wrapped ? wrap(text) : text;
	}

	private message(index: number): string {
		const text = this.positional[index] ?? "";
		const b = scalars(text);
		return trivia(b, 0) === b.length && b[0] !== "/"
			? text
			: `Comment(rawValue: ${text})`;
	}

	private call(
		head: string,
		count: number,
		first: () => string,
		tail = ")",
	): string | undefined {
		if (![count, count + 1].includes(this.positional.length)) {
			return undefined;
		}
		const body = first();
		const lead = EFFECTS.filter((effect) => this.effects.has(effect))
			.map((effect) => `${effect} `)
			.join("");
		const message = this.positional.length > count ? this.message(count) : "";
		return `${head}${lead}${[body, message].filter((part) => part !== "").join(", ")}${tail}`;
	}

	private comparison(op: string): string | undefined {
		if (this.accuracy !== undefined && op !== "==" && op !== "!=") {
			return undefined;
		}
		return this.call("#expect(", 2, () => {
			const left = this.operand(0);
			const right = this.operand(1);
			if (this.accuracy === undefined) {
				return `${left} ${op} ${right}`;
			}
			return `abs(${left} - ${right}) ${op === "==" ? "<=" : ">"} ${wrap(this.accuracy)}`;
		});
	}

	build(kind: Assertion): string | undefined {
		switch (kind.kind) {
			case "truth": {
				return this.call("#expect(", 1, () =>
					kind.negated ? `!${this.operand(0)}` : this.operand(0, false),
				);
			}
			case "nilCheck": {
				return this.call(
					"#expect(",
					1,
					() => `${this.operand(0)} ${kind.op} nil`,
				);
			}
			case "unwrap": {
				return this.call("#require(", 1, () => this.positional[0] ?? "");
			}
			case "fail": {
				return this.call("Issue.record(", 0, () => "");
			}
			case "closure": {
				return this.call(
					"#expect(",
					1,
					() => `throws: ${kind.type}`,
					`) { ${this.positional[0] ?? ""} }`,
				);
			}
			case "compare": {
				return this.comparison(kind.op);
			}
			default: {
				return undefined;
			}
		}
	}
}

const convertCall = (
	kind: Assertion,
	b: Scalars,
	open: number,
): Replacement | undefined => {
	const close = matchClose(b, open);
	const parts =
		close === undefined ? undefined : splitArguments(b.slice(open + 1, close));
	if (close === undefined || parts === undefined) {
		return undefined;
	}
	// A trailing closure is an error handler; those calls stay with a person.
	if (b.slice(close + 1).find((c) => !isSpace(c)) === "{") {
		return undefined;
	}
	const { positional, accuracy } = parseArguments(parts, rewriteAssertions);
	const text = new Builder(positional, accuracy).build(kind);
	return text === undefined ? undefined : { text, end: close + 1 };
};

/** Converts the call whose `(` is the first non-blank scalar from `from`. */
const callAfter = (
	kind: Assertion,
	b: Scalars,
	from: number,
): Replacement | undefined => {
	let open = from;
	while (open < b.length && isSpace(b[open])) {
		open += 1;
	}
	return b[open] === "(" ? convertCall(kind, b, open) : undefined;
};

/** Rewrites each XCTAssert* call, XCTUnwrap, and XCTFail into Swift Testing. */
export const rewriteAssertions = (source: string): string => {
	const b = scalars(source);
	const out: string[] = [];
	let index = 0;
	while (index < b.length) {
		let end = trivia(b, index) ?? index;
		if (end === index && isIdent(b[index])) {
			end = index;
			while (end < b.length && isIdent(b[end])) {
				end += 1;
			}
		}
		const kind = ASSERTIONS.get(b.slice(index, end).join(""));
		const replacement =
			end > index && kind !== undefined
				? callAfter(kind, b, Math.max(end, index + 1))
				: undefined;
		if (replacement === undefined) {
			end = Math.max(end, index + 1);
			out.push(...b.slice(index, end));
			index = end;
		} else {
			out.push(replacement.text);
			index = replacement.end;
		}
	}
	return out.join("");
};
