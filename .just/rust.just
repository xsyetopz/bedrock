# Managed by rust-bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# Install the toolchain from rust-toolchain.toml, cargo-deny, and kache, then
# route Cargo through kache. `kache init` sets the wrapper for this user, not
# this repo, skips the steps already done, and leaves shell startup files alone.
# Cargo runs the wrapper by name, so setup stops first when kache is off PATH.
setup:
    rustup toolchain install --no-self-update
    if [[ "$(cargo deny --version 2>/dev/null || true)" != "cargo-deny 0.20.2" ]]; then cargo install --locked cargo-deny@0.20.2; fi
    if [[ "$(kache --version 2>/dev/null || true)" != "kache 2.0.0" ]]; then cargo install --locked kache@2.0.0; fi
    @command -v kache >/dev/null || { echo "kache is in ${CARGO_HOME:-$HOME/.cargo}/bin, which is not on PATH: add it, then run just setup again" >&2; exit 1; }
    kache init --yes --no-shell

# Format the Rust code.
format:
    cargo fmt --all

# Fail if the Rust code is not formatted.
format-check:
    cargo fmt --all --check

# Build every target without running anything.
typecheck:
    cargo check --workspace --all-targets --all-features --locked

# Run Clippy, the rustdoc lints, and cargo-deny.
lint:
    cargo clippy --workspace --all-targets --all-features --locked -- -D warnings
    RUSTDOCFLAGS="-D warnings" cargo doc --workspace --all-features --locked --no-deps
    @just rust::deny

# Inline out-of-line test modules and apply Clippy's fixes.
lint-fix:
    cargo bedrock guard --fix
    cargo clippy --fix --allow-dirty --allow-staged --workspace --all-targets --all-features
    cargo fmt --all

# Run the tests.
test:
    cargo test --workspace --all-features --locked

# Report out-of-line tests, members that skip the workspace lints, and drift.
guard:
    cargo bedrock guard

# Run every Rust check CI runs.
verify: format-check typecheck lint guard test

# The Rust checks the pre-commit hook runs.
pre-commit: format-check lint guard

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
