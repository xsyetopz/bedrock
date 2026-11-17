/** What a token is; punctuation carries its character as the token text. */
export type TokenKind = "ident" | "literal" | "punct";

/** One Rust token: its place in the source and its text. */
export interface Token {
	readonly kind: TokenKind;
	readonly text: string;
	/** Offset of the first character. */
	readonly start: number;
	/** Offset after the last character. */
	readonly end: number;
	/** 1-based line of the first character. */
	readonly line: number;
}

const isIdent = (char: string): boolean =>
	/[\p{L}\p{N}_]/u.test(char) || (char.codePointAt(0) ?? 0) >= 0x80;

const isSpace = (char: string): boolean => /\s/u.test(char);

/** A cursor that counts lines as it moves. */
class Scanner {
	readonly source: string;
	at = 0;
	line = 1;

	constructor(source: string) {
		this.source = source;
	}

	get done(): boolean {
		return this.at >= this.source.length;
	}

	peek(offset = 0): string {
		return this.source[this.at + offset] ?? "";
	}

	bump(count = 1): void {
		for (let step = 0; step < count && !this.done; step += 1) {
			this.line += this.peek() === "\n" ? 1 : 0;
			this.at += 1;
		}
	}

	skipWhile(keep: (char: string) => boolean): void {
		while (!this.done && keep(this.peek())) {
			this.bump();
		}
	}

	/** Skips a block comment, which may nest, from its opening slash. */
	blockComment(): void {
		let depth = 0;
		while (!this.done) {
			const pair = this.peek() + this.peek(1);
			if (pair === "/*") {
				depth += 1;
				this.bump();
			} else if (pair === "*/") {
				depth -= 1;
				this.bump();
				if (depth === 0) {
					this.bump();
					return;
				}
			}
			this.bump();
		}
	}

	/** Skips a quoted literal from its opening quote, honoring backslash escapes. */
	quoted(quote: string): void {
		this.bump();
		while (!this.done) {
			const char = this.peek();
			if (char === "\\") {
				this.bump();
			} else if (char === quote) {
				this.bump();
				return;
			}
			this.bump();
		}
	}

	/** Skips `#*"…"#*` from the first `#` or `"`, after the `r` prefix. */
	rawString(): void {
		let hashes = 0;
		while (this.peek() === "#") {
			hashes += 1;
			this.bump();
		}
		this.bump();
		const closing = `"${"#".repeat(hashes)}`;
		while (!this.done) {
			if (this.source.startsWith(closing, this.at)) {
				this.bump(closing.length);
				return;
			}
			this.bump();
		}
	}

	/** The length of a raw string prefix (`r`, `br`, `cr`) at the cursor, or 0. */
	rawPrefix(): number {
		const first = this.peek();
		let prefix = 0;
		if (first === "r") {
			prefix = 1;
		} else if ((first === "b" || first === "c") && this.peek(1) === "r") {
			prefix = 2;
		}
		if (prefix === 0) {
			return 0;
		}
		let offset = prefix;
		while (this.peek(offset) === "#") {
			offset += 1;
		}
		return this.peek(offset) === '"' ? prefix : 0;
	}

	/** Skips a char literal or a lifetime from its `'`. */
	quote(): TokenKind {
		if (this.peek(1) === "\\") {
			this.quoted("'");
			return "literal";
		}
		const width = (this.source.codePointAt(this.at + 1) ?? 0) > 65_535 ? 2 : 1;
		if (this.peek(1 + width) === "'") {
			this.bump(width + 2);
			return "literal";
		}
		this.bump();
		this.skipWhile(isIdent);
		return "ident";
	}

	/** Skips one token that is not a comment or whitespace; returns its kind. */
	token(): TokenKind {
		const char = this.peek();
		const next = this.peek(1);
		const prefix = this.rawPrefix();
		if (prefix > 0) {
			this.bump(prefix);
			this.rawString();
			return "literal";
		}
		if ((char === "b" || char === "c") && next === '"') {
			this.bump();
			this.quoted('"');
			return "literal";
		}
		if (char === "b" && next === "'") {
			this.bump();
			this.quoted("'");
			return "literal";
		}
		if (char === '"') {
			this.quoted('"');
			return "literal";
		}
		return this.simpleToken(char, next);
	}

	private simpleToken(char: string, next: string): TokenKind {
		if (char === "'") {
			return this.quote();
		}
		if (char === "r" && next === "#") {
			this.bump(2);
			this.skipWhile(isIdent);
			return "ident";
		}
		if (/\d/u.test(char)) {
			this.skipWhile(isIdent);
			return "literal";
		}
		if (isIdent(char)) {
			this.skipWhile(isIdent);
			return "ident";
		}
		this.bump();
		return "punct";
	}
}

/**
 * Splits Rust source into tokens, skipping whitespace and comments and keeping
 * strings, raw strings, and char literals whole, so text inside them never
 * looks like code.
 */
export const tokens = (source: string): Token[] => {
	const scanner = new Scanner(source);
	const found: Token[] = [];
	while (!scanner.done) {
		const char = scanner.peek();
		const pair = char + scanner.peek(1);
		if (isSpace(char)) {
			scanner.bump();
		} else if (pair === "//") {
			scanner.skipWhile((each) => each !== "\n");
		} else if (pair === "/*") {
			scanner.blockComment();
		} else {
			const { at: start, line } = scanner;
			const kind = scanner.token();
			found.push({
				kind,
				text: source.slice(start, scanner.at),
				start,
				end: scanner.at,
				line,
			});
		}
	}
	return found;
};
