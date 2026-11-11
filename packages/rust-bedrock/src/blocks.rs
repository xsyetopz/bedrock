//! Marked blocks in files that several bedrocks write to, such as AGENTS.md. Each
//! bedrock replaces only its own blocks and keeps the rest of the file.

use crate::files::Block;

#[derive(Clone, Copy, Debug)]
pub(crate) enum Syntax {
    Markdown,
    Hash,
}

impl Syntax {
    fn markers(self, name: &str) -> (String, String) {
        match self {
            Self::Markdown => (format!("<!-- {name} -->"), format!("<!-- /{name} -->")),
            Self::Hash => (format!("# >>> {name}"), format!("# <<< {name}")),
        }
    }
}

/// Every bedrock's block, in the order a file lists them, so a file comes out the
/// same whichever bedrock adopts first.
const ORDER: [&str; 6] = [
    "bedrock",
    "typescript-bedrock",
    "rust-bedrock",
    "swift-bedrock",
    "swift-bedrock-cxx",
    "jvm-bedrock",
];

/// Where a line starting with `marker` begins in `text`.
fn line_start(text: &str, marker: &str) -> Option<usize> {
    text.match_indices(marker)
        .map(|(index, _)| index)
        .find(|&index| index == 0 || text[..index].ends_with('\n'))
}

/// Returns `current` with each block replaced or, when missing, added in bedrock
/// order. A missing file starts as `header`.
pub(crate) fn upsert(
    current: Option<&str>,
    header: &str,
    syntax: Syntax,
    blocks: &[Block],
) -> String {
    let mut text = current.unwrap_or(header).to_owned();
    for block in blocks {
        let (open, close) = syntax.markers(block.name);
        let marked = format!("{open}\n{}{close}\n", block.content);
        let span = text.find(&open).and_then(|start| {
            text[start..]
                .find(&close)
                .map(|end| (start, start + end + close.len()))
        });
        if let Some((start, end)) = span {
            let end = if text[end..].starts_with('\n') {
                end + 1
            } else {
                end
            };
            text.replace_range(start..end, &marked);
        } else if let Some(next) = ORDER
            .iter()
            .skip_while(|&&name| name != block.name)
            .skip(1)
            .filter_map(|name| line_start(&text, &syntax.markers(name).0))
            .min()
        {
            text.insert_str(next, &format!("{marked}\n"));
        } else {
            if !text.is_empty() && !text.ends_with("\n\n") {
                text.push_str(if text.ends_with('\n') { "\n" } else { "\n\n" });
            }
            text.push_str(&marked);
        }
    }
    text
}

#[cfg(test)]
mod tests;
