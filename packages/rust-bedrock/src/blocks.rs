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
const ORDER: [&str; 5] = [
    "bedrock",
    "bun-bedrock",
    "rust-bedrock",
    "swift-bedrock",
    "swift-bedrock-cxx",
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
mod tests {
    use super::{Syntax, upsert};
    use crate::files::Block;

    const BLOCKS: [Block; 2] = [
        Block {
            name: "bedrock",
            content: "- base\n",
        },
        Block {
            name: "rust-bedrock",
            content: "- rust\n",
        },
    ];

    #[test]
    fn starts_from_the_header() {
        assert_eq!(
            upsert(None, "# Agents\n", Syntax::Markdown, &BLOCKS),
            "# Agents\n\n<!-- bedrock -->\n- base\n<!-- /bedrock -->\n\n<!-- rust-bedrock -->\n- rust\n<!-- /rust-bedrock -->\n"
        );
    }

    #[test]
    fn puts_a_new_block_before_the_later_bedrocks() {
        let current = "# >>> bedrock\nbase\n# <<< bedrock\n\n# >>> swift-bedrock\n.build/\n# <<< swift-bedrock\n";
        let updated = upsert(Some(current), "", Syntax::Hash, &BLOCKS[1..]);
        assert_eq!(
            updated,
            "# >>> bedrock\nbase\n# <<< bedrock\n\n# >>> rust-bedrock\n- rust\n# <<< rust-bedrock\n\n# >>> swift-bedrock\n.build/\n# <<< swift-bedrock\n"
        );
        assert_eq!(
            upsert(Some(&updated), "", Syntax::Hash, &BLOCKS[1..]),
            updated
        );
    }

    #[test]
    fn replaces_its_blocks_and_keeps_the_rest() {
        let current = "own\n# >>> bun-bedrock\nnode_modules/\n# <<< bun-bedrock\n# >>> rust-bedrock\nold\n# <<< rust-bedrock\ntail\n";
        let updated = upsert(Some(current), "", Syntax::Hash, &BLOCKS[1..]);
        assert_eq!(
            updated,
            "own\n# >>> bun-bedrock\nnode_modules/\n# <<< bun-bedrock\n# >>> rust-bedrock\n- rust\n# <<< rust-bedrock\ntail\n"
        );
        assert_eq!(
            upsert(Some(&updated), "", Syntax::Hash, &BLOCKS[1..]),
            updated
        );
    }
}
