# Managed by bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# The crates Cargo compiles and the tests libtest runs at once; see BEDROCK_JOBS in the justfile.
jobs := env("BEDROCK_RUST_JOBS", env("BEDROCK_JOBS", "4"))
export CARGO_BUILD_JOBS := env("CARGO_BUILD_JOBS", jobs)
export RUST_TEST_THREADS := env("RUST_TEST_THREADS", jobs)

# Install the toolchain from rust-toolchain.toml, cargo-deny, and kache, then
# route Cargo through kache. `kache init` sets the wrapper for this user, not
# this repo, skips the steps already done, and leaves shell startup files alone.
# Cargo runs the wrapper by name, so setup stops first when kache is off PATH.
# uv runs the pinned lizard that bedrock guard bounds file length with.
setup:
    if ! command -v uvx >/dev/null && ! command -v pipx >/dev/null; then brew install uv; fi
    rustup toolchain install --no-self-update
    if [[ "$(cargo deny --version 2>/dev/null || true)" != "cargo-deny 0.20.2" ]]; then cargo install --locked cargo-deny@0.20.2; fi
    if [[ "$(kache --version 2>/dev/null || true)" != "kache 2.0.0" ]]; then cargo install --locked kache@2.0.0; fi
    @command -v kache >/dev/null || { echo "kache is in ${CARGO_HOME:-$HOME/.cargo}/bin, which is not on PATH: add it, then run just setup again" >&2; exit 1; }
    kache init --yes --no-shell

# Format the Rust code.
format: hold
    cargo fmt --all

# Fail if the Rust code is not formatted.
format-check: hold
    cargo fmt --all --check

# Build every target without running anything.
typecheck: hold
    cargo check --workspace --all-targets --all-features --locked

# Build the workspace for each target rust-toolchain.toml lists (`bedrock init
# rust:lib@ios,android` writes them); rustup installs them with the toolchain.
# Android links with the NDK's clang, which must be on PATH.
build-targets: hold
    #!/usr/bin/env bash
    set -euo pipefail
    targets=$(sed -n 's/^targets *= *\[\(.*\)\]/\1/p' rust-toolchain.toml | tr -d '" ' | tr ',' ' ')
    for target in $targets; do
        cargo build --workspace --locked --target "$target"
    done

# Run Clippy, the rustdoc lints, and cargo-deny.
lint: hold
    cargo clippy --workspace --all-targets --all-features --locked -- -D warnings
    RUSTDOCFLAGS="-D warnings" cargo doc --workspace --all-features --locked --no-deps
    @{{ quote(replace(just_executable(), '\', '/')) }} rust::deny

# Move inline test modules to tests.rs and apply Clippy's fixes.
lint-fix: hold
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard rust --fix
    cargo clippy --fix --allow-dirty --allow-staged --workspace --all-targets --all-features
    cargo fmt --all

# Run the tests.
test: hold
    cargo test --workspace --all-features --locked

# Report inline tests, suppressions, members that skip the workspace lints, and drift.
guard:
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard rust

# Run every Rust check CI runs.
verify: format-check typecheck lint guard test

# The Rust checks the pre-commit hook runs.
pre-commit: format-check lint guard

# The Rust checks the pre-push hook runs: verify without the tests, which CI runs.
pre-push: format-check typecheck lint guard

[private]
deny:
    #!/usr/bin/env bash
    set -euo pipefail
    if command -v cargo-deny >/dev/null; then
        exec cargo deny check
    elif [[ -n "${CI:-}" ]]; then
        echo "cargo-deny is not installed" >&2
        exit 1
    fi
    echo "Skipped cargo-deny: it is not installed. Run just setup; CI runs it."

# Takes one of the machine's check slots for this run; see `hold-slot` in the justfile.
[private]
hold:
    @{{ quote(replace(just_executable(), '\', '/')) }} hold-slot "$PPID"
