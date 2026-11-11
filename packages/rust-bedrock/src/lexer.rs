//! A token scanner that knows enough Rust to skip comments, strings, and char
//! literals, so text inside them never looks like code.

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum Kind {
    Ident,
    Literal,
    Punct(u8),
}

#[derive(Clone, Copy, Debug)]
pub(crate) struct Token {
    pub kind: Kind,
    pub start: usize,
    pub end: usize,
    pub line: usize,
}

impl Token {
    pub(crate) fn text<'a>(&self, source: &'a str) -> &'a str {
        &source[self.start..self.end]
    }
}

struct Scanner<'a> {
    bytes: &'a [u8],
    at: usize,
    line: usize,
}

impl Scanner<'_> {
    fn peek(&self, offset: usize) -> u8 {
        self.bytes.get(self.at + offset).copied().unwrap_or(0)
    }

    fn bump(&mut self) {
        if self.peek(0) == b'\n' {
            self.line += 1;
        }
        self.at += 1;
    }

    fn skip_while(&mut self, keep: impl Fn(u8) -> bool) {
        while self.at < self.bytes.len() && keep(self.peek(0)) {
            self.bump();
        }
    }

    fn block_comment(&mut self) {
        let mut depth = 0_usize;
        while self.at < self.bytes.len() {
            match (self.peek(0), self.peek(1)) {
                (b'/', b'*') => {
                    depth += 1;
                    self.bump();
                }
                (b'*', b'/') => {
                    depth -= 1;
                    self.bump();
                    if depth == 0 {
                        self.bump();
                        return;
                    }
                }
                _ => {}
            }
            self.bump();
        }
    }

    /// Consumes a quoted literal from its opening quote, honoring backslash escapes.
    fn quoted(&mut self, quote: u8) {
        self.bump();
        while self.at < self.bytes.len() {
            match self.peek(0) {
                b'\\' => {
                    self.bump();
                }
                byte if byte == quote => {
                    self.bump();
                    return;
                }
                _ => {}
            }
            self.bump();
        }
    }

    /// Consumes `#*"…"#*` from the first `#` or `"`, after the `r` prefix.
    fn raw_string(&mut self) {
        let mut hashes = 0;
        while self.peek(0) == b'#' {
            hashes += 1;
            self.bump();
        }
        self.bump();
        while self.at < self.bytes.len() {
            if self.peek(0) == b'"' && (1..=hashes).all(|offset| self.peek(offset) == b'#') {
                for _ in 0..=hashes {
                    self.bump();
                }
                return;
            }
            self.bump();
        }
    }

    /// Returns the length of a raw string prefix (`r`, `br`, `cr`) at the cursor, if one starts here.
    fn raw_prefix(&self) -> Option<usize> {
        let prefix = match (self.peek(0), self.peek(1)) {
            (b'r', _) => 1,
            (b'b' | b'c', b'r') => 2,
            _ => return None,
        };
        let mut offset = prefix;
        while self.peek(offset) == b'#' {
            offset += 1;
        }
        (self.peek(offset) == b'"').then_some(prefix)
    }

    /// Consumes a char literal or a lifetime from its `'`.
    fn quote(&mut self) -> Kind {
        if self.peek(1) == b'\\' {
            self.quoted(b'\'');
            return Kind::Literal;
        }
        let width = std::str::from_utf8(&self.bytes[self.at + 1..])
            .ok()
            .and_then(|rest| rest.chars().next())
            .map_or(1, char::len_utf8);
        if self.peek(1 + width) == b'\'' {
            for _ in 0..width + 2 {
                self.bump();
            }
            return Kind::Literal;
        }
        self.bump();
        self.skip_while(is_ident);
        Kind::Ident
    }
}

const fn is_ident(byte: u8) -> bool {
    byte.is_ascii_alphanumeric() || byte == b'_' || byte >= 0x80
}

pub(crate) fn tokens(source: &str) -> Vec<Token> {
    let mut scanner = Scanner {
        bytes: source.as_bytes(),
        at: 0,
        line: 1,
    };
    let mut found = Vec::new();
    while scanner.at < scanner.bytes.len() {
        let start = scanner.at;
        let line = scanner.line;
        let kind = match (scanner.peek(0), scanner.peek(1)) {
            (byte, _) if byte.is_ascii_whitespace() => {
                scanner.bump();
                continue;
            }
            (b'/', b'/') => {
                scanner.skip_while(|byte| byte != b'\n');
                continue;
            }
            (b'/', b'*') => {
                scanner.block_comment();
                continue;
            }
            _ if scanner.raw_prefix().is_some() => {
                for _ in 0..scanner.raw_prefix().unwrap_or(0) {
                    scanner.bump();
                }
                scanner.raw_string();
                Kind::Literal
            }
            (b'b' | b'c', b'"') => {
                scanner.bump();
                scanner.quoted(b'"');
                Kind::Literal
            }
            (b'b', b'\'') => {
                scanner.bump();
                scanner.quoted(b'\'');
                Kind::Literal
            }
            (b'"', _) => {
                scanner.quoted(b'"');
                Kind::Literal
            }
            (b'\'', _) => scanner.quote(),
            (b'r', b'#') => {
                scanner.bump();
                scanner.bump();
                scanner.skip_while(is_ident);
                Kind::Ident
            }
            (byte, _) if byte.is_ascii_digit() => {
                scanner.skip_while(is_ident);
                Kind::Literal
            }
            (byte, _) if is_ident(byte) => {
                scanner.skip_while(is_ident);
                Kind::Ident
            }
            (byte, _) => {
                scanner.bump();
                Kind::Punct(byte)
            }
        };
        found.push(Token {
            kind,
            start,
            end: scanner.at,
            line,
        });
    }
    found
}

#[cfg(test)]
mod tests;
