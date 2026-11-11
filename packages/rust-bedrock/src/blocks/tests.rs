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
    let current =
        "# >>> bedrock\nbase\n# <<< bedrock\n\n# >>> swift-bedrock\n.build/\n# <<< swift-bedrock\n";
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
    let current = "own\n# >>> typescript-bedrock\nnode_modules/\n# <<< typescript-bedrock\n# >>> rust-bedrock\nold\n# <<< rust-bedrock\ntail\n";
    let updated = upsert(Some(current), "", Syntax::Hash, &BLOCKS[1..]);
    assert_eq!(
        updated,
        "own\n# >>> typescript-bedrock\nnode_modules/\n# <<< typescript-bedrock\n# >>> rust-bedrock\n- rust\n# <<< rust-bedrock\ntail\n"
    );
    assert_eq!(
        upsert(Some(&updated), "", Syntax::Hash, &BLOCKS[1..]),
        updated
    );
}
