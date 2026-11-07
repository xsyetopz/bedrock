//! The files rust-bedrock writes, embedded at build time.

/// A file rust-bedrock owns: adopt writes it and guard reports drift from it.
#[derive(Debug)]
pub(crate) struct Managed {
    pub path: &'static str,
    pub content: &'static str,
    pub executable: bool,
}

macro_rules! managed {
    ($root:literal, $path:literal) => {
        managed!($root, $path, false)
    };
    ($root:literal, $path:literal, $executable:literal) => {
        Managed {
            path: $path,
            content: include_str!(concat!("../files/", $root, "/", $path)),
            executable: $executable,
        }
    };
}

/// Shared by every bedrock; a repository test keeps these equal to `base/`.
pub(crate) const BASE: &[Managed] = &[
    managed!("base", "justfile"),
    managed!("base", ".githooks/commit-msg", true),
    managed!("base", ".githooks/pre-commit", true),
    managed!("base", ".markdownlint.jsonc"),
    managed!("base", ".github/workflows/base.yml"),
];

/// typescript-bedrock writes its own Markdown config, with the custom rules it installs.
pub(crate) const MARKDOWN: Managed = managed!("base", ".markdownlint-cli2.jsonc");

pub(crate) const RUST: &[Managed] = &[
    managed!("rust", ".just/rust.just"),
    managed!("rust", ".github/workflows/rust.yml"),
    managed!("rust", "rustfmt.toml"),
    managed!("rust", "clippy.toml"),
];

/// Written once; the project changes them afterwards.
pub(crate) const STARTERS: &[Managed] = &[managed!("rust", "deny.toml"), XTASK_MAIN];

pub(crate) const XTASK_MAIN: Managed = managed!("rust", "xtask/src/main.rs");

pub(crate) const TOOLCHAIN: &str = include_str!("../files/rust/rust-toolchain.toml");
pub(crate) const WORKSPACE: &str = include_str!("../files/rust/workspace.toml");
pub(crate) const CARGO_CONFIG: &str = include_str!("../files/rust/.cargo/config.toml");

/// A marked block inside a file that other tools also write to.
#[derive(Debug)]
pub(crate) struct Block {
    pub name: &'static str,
    pub content: &'static str,
}

pub(crate) const AGENTS: [Block; 2] = [
    Block {
        name: "bedrock",
        content: include_str!("../files/base/blocks/AGENTS.md"),
    },
    Block {
        name: "rust-bedrock",
        content: include_str!("../files/blocks/AGENTS.md"),
    },
];

pub(crate) const GITIGNORE: [Block; 2] = [
    Block {
        name: "bedrock",
        content: include_str!("../files/base/blocks/gitignore"),
    },
    Block {
        name: "rust-bedrock",
        content: include_str!("../files/blocks/gitignore"),
    },
];

pub(crate) const EDITORCONFIG: [Block; 2] = [
    Block {
        name: "bedrock",
        content: include_str!("../files/base/blocks/editorconfig"),
    },
    Block {
        name: "rust-bedrock",
        content: include_str!("../files/blocks/editorconfig"),
    },
];

pub(crate) const GITATTRIBUTES: [Block; 2] = [
    Block {
        name: "bedrock",
        content: include_str!("../files/base/blocks/gitattributes"),
    },
    Block {
        name: "rust-bedrock",
        content: include_str!("../files/blocks/gitattributes"),
    },
];

pub(crate) const DEPENDABOT: [Block; 2] = [
    Block {
        name: "bedrock",
        content: include_str!("../files/base/blocks/dependabot.yml"),
    },
    Block {
        name: "rust-bedrock",
        content: include_str!("../files/blocks/dependabot.yml"),
    },
];

/// A starter crate: its manifest and source files, with `__name__` and `__crate__`
/// standing for the package name and the crate name.
#[derive(Debug)]
pub(crate) struct Kind {
    pub name: &'static str,
    pub summary: &'static str,
    pub files: &'static [(&'static str, &'static str)],
    /// A target the toolchain must install for it.
    pub target: Option<&'static str>,
}

macro_rules! kind_file {
    ($kind:literal, $path:literal) => {
        (
            $path,
            include_str!(concat!("../files/kinds/", $kind, "/", $path)),
        )
    };
}

pub(crate) const KINDS: &[Kind] = &[
    Kind {
        name: "lib",
        summary: "a library with unit, integration, and doc tests",
        files: &[
            kind_file!("lib", "Cargo.toml"),
            kind_file!("lib", "src/lib.rs"),
            kind_file!("lib", "tests/parse.rs"),
        ],
        target: None,
    },
    Kind {
        name: "bin",
        summary: "a command-line program",
        files: &[
            kind_file!("bin", "Cargo.toml"),
            kind_file!("bin", "src/main.rs"),
        ],
        target: None,
    },
    Kind {
        name: "no-std",
        summary: "a library for targets without std or an allocator",
        files: &[
            kind_file!("no-std", "Cargo.toml"),
            kind_file!("no-std", "src/lib.rs"),
        ],
        target: None,
    },
    Kind {
        name: "ffi",
        summary: "a C ABI library that Bun, C, or Swift loads",
        files: &[
            kind_file!("ffi", "Cargo.toml"),
            kind_file!("ffi", "src/lib.rs"),
        ],
        target: None,
    },
    Kind {
        name: "wasm",
        summary: "a WebAssembly module",
        files: &[
            kind_file!("wasm", "Cargo.toml"),
            kind_file!("wasm", "src/lib.rs"),
        ],
        target: Some("wasm32-unknown-unknown"),
    },
];

/// The root manifest `new` starts from.
pub(crate) const ROOT_MANIFEST: &str = include_str!("../files/new/Cargo.toml");
pub(crate) const README: &str = include_str!("../files/new/README.md");

/// The Bun side of an `ffi` crate in a repository that also uses Bun.
pub(crate) const BUN_FFI: &[(&str, &str)] = &[
    (
        "src/__name__.ts",
        include_str!("../files/bun-ffi/src/__name__.ts"),
    ),
    (
        "src/__name__.test.ts",
        include_str!("../files/bun-ffi/src/__name__.test.ts"),
    ),
];
