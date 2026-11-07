//! Parses `key=value` lines.

use std::collections::BTreeMap;
use std::error::Error;
use std::fmt;

/// A line that is not `key=value`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParseError {
    /// The 1-based line number.
    pub line: usize,
}

impl fmt::Display for ParseError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(formatter, "line {}: expected key=value", self.line)
    }
}

impl Error for ParseError {}

/// Parses `key=value` lines into a map, skipping blank lines and `#` comments.
///
/// # Errors
///
/// Returns [`ParseError`] for a line without `=` or with an empty key.
///
/// # Examples
///
/// ```
/// let pairs = __crate__::parse("a = 1\n# note\nb=2")?;
/// assert_eq!(pairs["a"], "1");
/// # Ok::<(), __crate__::ParseError>(())
/// ```
pub fn parse(input: &str) -> Result<BTreeMap<String, String>, ParseError> {
    let mut pairs = BTreeMap::new();
    for (index, raw) in input.lines().enumerate() {
        let text = raw.trim();
        if text.is_empty() || text.starts_with('#') {
            continue;
        }
        match text.split_once('=') {
            Some((key, value)) if !key.trim().is_empty() => {
                pairs.insert(key.trim().to_owned(), value.trim().to_owned());
            }
            _ => return Err(ParseError { line: index + 1 }),
        }
    }
    Ok(pairs)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_pairs_and_skips_comments() {
        let pairs = parse("a=1\n\n# c\nb = 2\n").unwrap();
        assert_eq!(pairs.len(), 2);
        assert_eq!(pairs["b"], "2");
    }

    #[test]
    fn reports_the_bad_line() {
        assert_eq!(parse("a=1\n=2"), Err(ParseError { line: 2 }));
        assert_eq!(parse("x").unwrap_err().to_string(), "line 1: expected key=value");
    }
}
