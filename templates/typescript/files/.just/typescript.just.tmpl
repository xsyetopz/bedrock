# Managed by bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# Type-aware oxlint starts a tsgolint that loads the whole TypeScript program.
# tsgolint is a Go program, so GOMEMLIMIT (a soft heap limit) and GOMAXPROCS bound it,
# and `--threads` bounds oxlint. Raise them for a large repo, such as
# `BEDROCK_LINT_MEMORY=8GiB just lint`.
lint_memory := env("BEDROCK_LINT_MEMORY", "4GiB")
lint_threads := env("BEDROCK_LINT_THREADS", "4")

# Install the dependencies. CI must not change bun.lock.
setup:
    bun install {{ if env("CI", "") == "" { "" } else { "--frozen-lockfile" } }}

# Format the code.
format:
    bun fmt

# Fail if the code is not formatted.
format-check:
    bun fmt --check

# Type-check without emitting anything.
typecheck:
    bun check

# Run oxlint on the repo, or on the given paths. One run per repo at a time; the base justfile lints Markdown.
lint *paths: (oxlint paths)

# Apply oxlint's fixes, under the same lock and bounds as lint.
lint-fix *paths: (oxlint "--fix" paths)

# Run the tests.
test:
    bun test --pass-with-no-tests

# Report suppressions, other package managers, undocumented package API, TypeScript drift, and managed-file drift.
guard:
    {{ just_executable() }} bedrock guard typescript

# Run every Bun check CI runs.
verify: format-check typecheck lint guard test

# The Bun checks the pre-commit hook runs.
pre-commit: format-staged typecheck lint guard

[private]
format-staged:
    bun fmt --staged

# A second run in the same repo waits for the first instead of starting its own tsgolint.
# The lock is a directory because `mkdir` is atomic on every platform, and a dead holder's lock is taken over.
[private]
oxlint *args:
    #!/usr/bin/env bash
    set -euo pipefail
    lock="${TMPDIR:-/tmp}/bedrock-lint-$(pwd -P | cksum | cut -d ' ' -f 1)"
    waited=
    until mkdir "$lock" 2>/dev/null; do
        holder="$(cat "$lock/pid" 2>/dev/null || true)"
        if [[ -n "$holder" ]] && ! kill -0 "$holder" 2>/dev/null; then
            rm -rf "$lock"
            continue
        fi
        if [[ -z "$waited" ]]; then
            echo "Another lint run holds this repo's lock (pid ${holder:-unknown}); waiting for it." >&2
            waited=1
        fi
        sleep 1
    done
    trap 'rm -rf "$lock"' EXIT
    echo $$ >"$lock/pid"
    export GOMEMLIMIT="{{ lint_memory }}" GOMAXPROCS="{{ lint_threads }}"
    bun run oxlint --threads={{ lint_threads }} {{ args }}
