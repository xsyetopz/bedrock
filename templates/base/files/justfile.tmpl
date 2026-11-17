# Every bedrock repo shares this file. Each language adds `.just/<language>.just`,
# so `just <task>` runs the task for every language the repo has. Put project
# recipes in `project.just`; a recipe there named `project-<task>`, such as
# `project-verify`, runs after the languages' <task>.

set shell := ["bash", "-euo", "pipefail", "-c"]

mod? typescript ".just/typescript.just"
mod? rust ".just/rust.just"
mod? swift ".just/swift.just"
mod? cpp ".just/cpp.just"
mod? jvm ".just/jvm.just"
mod? go ".just/go.just"

import? "project.just"

languages := "typescript rust swift cpp jvm go"

# The bedrock release .bedrock.json pins; "source" runs the CLI from this checkout.
bedrock_version := `sed -n 's/^[[:space:]]*"version": "\(.*\)",$/\1/p' .bedrock.json 2>/dev/null || true`
bedrock_asset := "bedrock-" + replace(os(), "macos", "darwin") + "-" + replace(replace(arch(), "aarch64", "arm64"), "x86_64", "x64")
# One copy per release, shared by every checkout and worktree.
bedrock_bin := cache_directory() / "bedrock" / bedrock_version / bedrock_asset

# List the recipes.
default:
    @just --list --list-submodules

# Install bedrock and the pinned tools, and point git at .githooks.
setup: install-bedrock (each "setup")
    @if [[ -z "${CI:-}" ]]; then git config core.hooksPath .githooks; fi

# Format every file in place.
format: (each "format")

# Fail if a file is not formatted.
format-check: (each "format-check")

# Type-check or build without running anything.
typecheck: (each "typecheck")

# Run every linter.
lint: lint-markdown lint-hooks (each "lint")

# Apply the fixes the linters can make.
lint-fix: lint-markdown-fix (each "lint-fix")

# Run the tests.
test: (each "test")

# Report what the linters cannot see: suppressions, foreign tools, and drift.
guard: (bedrock "guard")

# Run every check CI runs.
verify: (bedrock "guard" "base") lint-markdown lint-hooks (each "verify")

# The checks the pre-commit hook runs, for the languages the commit touches.
pre-commit: (bedrock "guard" "base")
    #!/usr/bin/env bash
    set -euo pipefail
    staged="$(git diff --cached --name-only --diff-filter=ACMR)"
    # Lint the Markdown the commit holds: the staged copies and the staged configs, checked
    # out into a scratch tree, so neither an untracked draft nor an unstaged edit decides
    # the result. The leading ":" makes each a literal path, and --no-globs drops the
    # config's **/*.md.
    markdown=()
    while IFS= read -r path; do markdown+=("$path"); done < <(grep '\.md$' <<<"$staged" || true)
    if (( ${#markdown[@]} > 0 )); then
        scratch="$(mktemp -d)"
        trap 'rm -rf "$scratch"' EXIT
        configs=()
        while IFS= read -r path; do configs+=("$path"); done < <(git ls-files -- ':(glob)**/.markdownlint*')
        git checkout-index --prefix="$scratch/" -- ${configs[@]+"${configs[@]}"} "${markdown[@]}"
        if [[ -d node_modules ]]; then ln -s "$PWD/node_modules" "$scratch/node_modules"; fi
        {{ just_executable() }} --justfile "$PWD/justfile" --working-directory "$scratch" \
            lint-markdown --no-globs "${markdown[@]/#/:}"
    fi
    if grep -q '^\.githooks/' <<<"$staged"; then {{ just_executable() }} lint-hooks; fi
    for language in $({{ just_executable() }} bedrock languages --staged); do
        if [[ -f ".just/$language.just" ]]; then
            {{ just_executable() }} "$language::pre-commit"
        fi
    done

# Run the bedrock CLI this repo pins, such as `just bedrock update`.
[positional-arguments]
bedrock *args: install-bedrock
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ "{{ bedrock_version }}" == source ]]; then
        exec bun cli/src/main.ts "$@"
    fi
    exec "{{ bedrock_bin }}" "$@"

[private]
install-bedrock:
    #!/usr/bin/env bash
    set -euo pipefail
    version="{{ bedrock_version }}"
    if [[ -z "$version" ]]; then
        echo ".bedrock.json has no version. Run bedrock adopt." >&2
        exit 1
    fi
    if [[ "$version" == source || -x "{{ bedrock_bin }}" ]]; then exit 0; fi
    url="https://github.com/xsyetopz/bedrock/releases/download/v$version"
    dir="$(dirname "{{ bedrock_bin }}")"
    mkdir -p "$dir"
    tmp="$(mktemp "$dir/.download.XXXXXX")"
    trap 'rm -f "$tmp" "$tmp.sums"' EXIT
    echo "Downloading bedrock $version ({{ bedrock_asset }})" >&2
    curl --fail --silent --show-error --location --retry 3 -o "$tmp" "$url/{{ bedrock_asset }}"
    curl --fail --silent --show-error --location --retry 3 -o "$tmp.sums" "$url/SHA256SUMS"
    expected="$(awk -v asset="{{ bedrock_asset }}" '$2 == asset { print $1 }' "$tmp.sums")"
    actual="$(shasum -a 256 "$tmp" | cut -d ' ' -f 1)"
    if [[ -z "$expected" || "$expected" != "$actual" ]]; then
        echo "bedrock $version: checksum mismatch for {{ bedrock_asset }}" >&2
        exit 1
    fi
    chmod +x "$tmp"
    # A rename is atomic, so parallel worktrees never run a half-written binary.
    mv -f "$tmp" "{{ bedrock_bin }}"

[private]
each recipe:
    #!/usr/bin/env bash
    set -euo pipefail
    for language in {{ languages }}; do
        if [[ -f ".just/$language.just" ]]; then
            {{ just_executable() }} "$language::{{ recipe }}"
        fi
    done
    # project.just extends a task with a recipe named project-<task>.
    case " $({{ just_executable() }} --summary) " in
        *" project-{{ recipe }} "*) {{ just_executable() }} "project-{{ recipe }}" ;;
    esac

[private]
[positional-arguments]
lint-markdown *args:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -x node_modules/.bin/markdownlint-cli2 ]]; then
        exec bun run markdownlint-cli2 "$@"
    elif command -v markdownlint-cli2 >/dev/null; then
        exec markdownlint-cli2 "$@"
    elif [[ -n "${CI:-}" ]]; then
        echo "markdownlint-cli2 is not installed" >&2
        exit 1
    fi
    echo "Skipped Markdown: markdownlint-cli2 is not installed. CI runs it."

[private]
lint-markdown-fix: (lint-markdown "--fix")

[private]
lint-hooks:
    #!/usr/bin/env bash
    set -euo pipefail
    if command -v shellcheck >/dev/null; then
        exec shellcheck .githooks/*
    elif [[ -n "${CI:-}" ]]; then
        echo "shellcheck is not installed" >&2
        exit 1
    fi
    echo "Skipped hooks: shellcheck is not installed. CI runs it."
