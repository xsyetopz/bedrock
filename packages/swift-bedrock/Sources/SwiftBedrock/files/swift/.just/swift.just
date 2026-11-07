# Managed by swift-bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

bedrock := "swift run --quiet --package-path BuildTools swift-bedrock"
derived := ".build/xcode"

# Install the Swift toolchain from .swift-version and SwiftLint, then build the tools.
setup:
    if command -v swiftly >/dev/null; then swiftly install; fi
    if ! command -v swiftlint >/dev/null; then brew install swiftlint; fi
    swift package --package-path BuildTools resolve

# Format the Swift code.
format:
    just swift::files | xargs -0 -r swift format format --in-place --parallel

# Fail if the Swift code is not formatted.
format-check:
    just swift::files | xargs -0 -r swift format lint --strict --parallel

# Build the package, its tests, and each shared Xcode scheme, without running them.
typecheck:
    if [[ -f Package.swift ]]; then swift build --build-tests; fi
    just swift::xcode build-for-testing

# Run SwiftLint.
lint:
    swiftlint lint --strict --quiet

# Convert XCTest unit tests and apply SwiftLint's and swift-format's fixes.
lint-fix:
    {{ bedrock }} guard --fix
    swiftlint lint --fix --quiet
    just swift::format

# Run the Swift Testing tests, then each shared Xcode scheme's tests. A package
# without a test target passes, as it does for Bun and Rust.
test:
    if [[ -f Package.swift ]] && swift package dump-package | grep -q '"type" : "test"'; then swift test --parallel; fi
    just swift::xcode test

# Report XCTest outside UI tests, settings the manifest loses, foreign tools, and drift.
guard:
    {{ bedrock }} guard

# Run every Swift check CI runs.
verify: format-check typecheck lint guard test

# The Swift checks the pre-commit hook runs.
pre-commit: format-check lint guard

[private]
files:
    git ls-files -z --cached --others --exclude-standard -- '*.swift'

[private]
xcode action:
    #!/usr/bin/env bash
    set -euo pipefail
    shopt -s nullglob
    for project in *.xcodeproj; do
        for scheme in "$project"/xcshareddata/xcschemes/*.xcscheme; do
            xcodebuild {{ action }} -quiet -project "$project" -scheme "$(basename "$scheme" .xcscheme)" \
                -destination "platform=macOS" -derivedDataPath {{ derived }}
        done
    done
