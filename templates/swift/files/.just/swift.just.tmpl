# Managed by bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# SwiftPM compiles each manifest and plugin inside its own sandbox. Where an outer sandbox
# (a coding agent, a git hook) forbids nesting, sandbox-exec fails with sandbox_apply, so
# the outer one is the only sandbox left.
spm := "--disable-sandbox"
# The processes SwiftPM, Xcode, and swift-format run at once; see BEDROCK_JOBS in the justfile.
jobs := env("BEDROCK_SWIFT_JOBS", env("BEDROCK_JOBS", "4"))
derived := ".build/xcode"

# SwiftPM 6.4 has no switch for the compiler's cache, so these flags turn it on, in a
# cache every checkout on the Mac shares. SwiftPM names the package after its folder,
# so worktrees reuse each other's results when their folders share a name. With C++
# interop, the 6.4 compiler warns about its own module map once caching is on, and
# warnings fail the build, so a repository with C++ builds uncached, in Xcode too.
cxx := path_exists(justfile_directory() / ".just" / "cpp.just")
cas := env("BEDROCK_SWIFT_CACHE", cache_directory() / "swift-bedrock" / "swiftpm")
cache_gib := env("BEDROCK_SWIFT_CACHE_GIB", "20")
cached := if os() != "macos" { "" } else if env("CI", "") != "" { "" } else if cxx == "true" { "" } else { "-Xswiftc -cache-compile-job -Xswiftc -cas-path -Xswiftc " + quote(cas) + " -Xswiftc -scanner-prefix-map-sdk -Xswiftc /^sdk -Xswiftc -scanner-prefix-map-toolchain -Xswiftc /^toolchain -Xswiftc -scanner-prefix-map-paths -Xswiftc " + quote(parent_directory(justfile_directory())) + " -Xswiftc /^src" }

# Xcode leaves Bedrock.xcconfig off Swift package targets, so the command line repeats its
# cache settings for them.
xcode_cache := if cxx == "true" { "COMPILATION_CACHE_ENABLE_CACHING=NO" } else { "COMPILATION_CACHE_ENABLE_CACHING=YES COMPILATION_CACHE_CAS_PATH=" + quote(cache_directory() / "swift-bedrock" / "xcode") + " SWIFT_ENABLE_PREFIX_MAPPING=YES SWIFT_ENABLE_PROJECT_PREFIX_MAPPING=YES CLANG_ENABLE_PREFIX_MAPPING=YES CLANG_ENABLE_PROJECT_PREFIX_MAPPING=YES" }

# Install the Swift toolchain from .swift-version and SwiftLint, then resolve dependencies.
setup:
    if command -v swiftly >/dev/null; then swiftly install; fi
    if ! command -v swiftlint >/dev/null; then brew install swiftlint; fi
    if [[ -f Package.swift ]]; then swift package {{ spm }} resolve; fi

# Format the Swift code. swift-format's --parallel takes every core, so xargs runs
# `jobs` of them over batches of files instead.
format: hold
    just swift::files | xargs -0 -r -P {{ jobs }} -n 64 swift format format --in-place

# Fail if the Swift code is not formatted.
format-check: hold
    just swift::files | xargs -0 -r -P {{ jobs }} -n 64 swift format lint --strict

# Build the package, its tests, and each shared Xcode scheme, without running them.
typecheck: hold
    just swift::trim-cache
    if [[ -f Package.swift ]]; then swift build {{ spm }} --jobs {{ jobs }} --build-tests {{ cached }}; fi
    just swift::xcode build-for-testing

# Run SwiftLint on the listed files. Without paths it would walk the tree itself, into
# worktrees and generated code; with an empty list it would too, hence xargs -r.
lint: hold
    just swift::files | xargs -0 -r swiftlint lint --strict --quiet --force-exclude

# Convert XCTest unit tests (bedrock swift convert-tests) and apply SwiftLint's and swift-format's fixes.
lint-fix: hold
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard swift --fix
    just swift::files | xargs -0 -r swiftlint lint --fix --quiet --force-exclude
    just swift::format

# Run the Swift Testing tests, then each shared Xcode scheme's tests. A package
# without a test target passes, as it does for Bun and Rust. grep reads the whole dump,
# because under pipefail the early exit of grep -q fails the pipe and skips the tests.
test: hold
    just swift::trim-cache
    if [[ -f Package.swift ]] && swift package {{ spm }} dump-package | grep '"type" : "test"' >/dev/null; then swift test {{ spm }} --jobs {{ jobs }} --parallel --num-workers {{ jobs }} {{ cached }}; fi
    just swift::xcode test

# Report XCTest outside UI tests, settings the manifest loses, foreign tools, and drift.
guard:
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard swift

# Run every Swift check CI runs.
verify: format-check typecheck lint guard test

# The Swift checks the pre-commit hook runs.
pre-commit: format-check lint guard

# The Swift checks the pre-push hook runs: verify without the tests, which CI runs.
pre-push: format-check typecheck lint guard

# Delete the git-ignored .build folder of each other worktree with no build running in it.
clean-worktrees:
    #!/usr/bin/env bash
    set -euo pipefail
    here=$(pwd -P)
    busy=$(lsof -a -d cwd -Fn 2>/dev/null | sed -n 's/^n//p'; ps -axo command=)
    git worktree list --porcelain | sed -n 's/^worktree //p' | while IFS= read -r tree; do
        build="$tree/.build"
        if [[ "$tree" == "$here" || ! -d "$build" ]] || ! git -C "$tree" check-ignore -q .build; then
            continue
        fi
        if grep -qF -- "$tree" <<<"$busy"; then
            echo "Kept $build: a process is running in $tree"
        else
            echo "Deleting $build ($(du -sh "$build" | cut -f1))"
            rm -rf -- "$build"
        fi
    done

[private]
trim-cache:
    #!/usr/bin/env bash
    set -euo pipefail
    cas={{ quote(cas) }}
    # The 6.4 compiler sets no size limit, so a cache past it starts over. A build still
    # running keeps the files it has open.
    if [[ -n {{ quote(cached) }} && -d "$cas" ]] && (( $(du -sk "$cas" | cut -f1) > {{ cache_gib }} * 1024 * 1024 )); then
        # Another checkout may be trimming it too.
        old="$cas.old.$$"
        if mv "$cas" "$old" 2>/dev/null; then
            rm -rf -- "$old" &
        fi
    fi

[private]
files:
    {{ quote(replace(just_executable(), '\', '/')) }} files '*.swift'

[private]
xcode action:
    #!/usr/bin/env bash
    set -euo pipefail
    shopt -s nullglob
    # Each parallel test runner is a full app process, so `jobs` bounds them too.
    workers=()
    if [[ "{{ action }}" == test ]]; then workers=(-maximum-parallel-testing-workers {{ jobs }}); fi
    for project in *.xcodeproj; do
        for scheme in "$project"/xcshareddata/xcschemes/*.xcscheme; do
            xcodebuild {{ action }} -quiet -project "$project" -scheme "$(basename "$scheme" .xcscheme)" \
                -destination "platform=macOS" -derivedDataPath {{ derived }} -jobs {{ jobs }} ${workers[@]+"${workers[@]}"} {{ xcode_cache }}
        done
    done

# Takes one of the machine's check slots for this run; see `hold-slot` in the justfile.
[private]
hold:
    @{{ quote(replace(just_executable(), '\', '/')) }} hold-slot "$PPID"
