# Managed by bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# Bounds that keep one run from taking the machine. GOMEMLIMIT is a soft limit on the Go
# runtime, golangci-lint included. `-p` caps the build and test programs that run at once;
# `run.concurrency` in .golangci.yml caps golangci-lint's own threads.
jobs := env("BEDROCK_GO_JOBS", env("BEDROCK_JOBS", "4"))
export GOMEMLIMIT := env("BEDROCK_GO_MEMLIMIT", "2GiB")
export GOFLAGS := env("GOFLAGS", "") + " -p=" + jobs

# Install the pinned golangci-lint with `go install`, then download the modules. The binary lands in
# GOBIN, or in $GOPATH/bin, which has to be on PATH.
setup:
    if [[ "$(golangci-lint version --short 2>/dev/null || true)" != "2.15.0" ]]; then go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0; fi
    @command -v golangci-lint >/dev/null || { echo "golangci-lint is not on PATH: add the directory go env GOBIN names, or GOPATH/bin when that is empty, then run just setup again" >&2; exit 1; }
    go mod download

# Format the Go code.
format: hold
    golangci-lint fmt

# Fail if the Go code is not formatted.
format-check: hold
    golangci-lint fmt --diff

# Build every package and compile every test, without running anything.
typecheck: hold
    go build ./...
    go test -run '^$' ./...

# Run golangci-lint.
lint: hold
    golangci-lint run

# Apply the fixes golangci-lint can make, then format.
lint-fix: hold
    golangci-lint run --fix
    golangci-lint fmt

# Build for another platform into dist/<platform>/: the commands under cmd/ or, when there are
# none, a compile check of every package. The platform is one of linux, macos, windows, freebsd,
# wasi (wasip1), wasm (js, with wasm_exec.js beside the build), or ios (a gomobile xcframework
# of the root package). arch overrides the platform's default GOARCH.
build platform arch="": hold
    #!/usr/bin/env bash
    set -euo pipefail
    arch="{{ arch }}"
    ext=""
    case "{{ platform }}" in
        linux)   goos=linux;   arch="${arch:-amd64}" ;;
        macos)   goos=darwin;  arch="${arch:-arm64}" ;;
        windows) goos=windows; arch="${arch:-amd64}"; ext=.exe ;;
        freebsd) goos=freebsd; arch="${arch:-amd64}" ;;
        wasi)    goos=wasip1;  arch=wasm; ext=.wasm ;;
        wasm)    goos=js;      arch=wasm; ext=.wasm ;;
        ios)
            # gomobile binds the root package and needs the iOS SDK, which comes with Xcode.
            # gobind has to be on PATH, so build the one the module pins into a scratch directory.
            # -iosversion raises gomobile's iOS 13 default to iOS 15, the oldest Xcode 26 and later build for.
            tools="$(mktemp -d)"
            trap 'rm -rf "$tools"' EXIT
            name="$(go list -f '{{{{.Name}}' .)"
            mkdir -p dist
            GOBIN="$tools" go install golang.org/x/mobile/cmd/gobind
            PATH="$tools:$PATH" go tool gomobile bind -target ios -iosversion 15.0 -o "dist/$(tr '[:lower:]' '[:upper:]' <<<"${name:0:1}")${name:1}.xcframework" .
            exit 0 ;;
        *) echo "unknown platform {{ platform }}; use linux, macos, windows, freebsd, wasi, wasm, or ios" >&2; exit 1 ;;
    esac
    export CGO_ENABLED=0 GOOS="$goos" GOARCH="$arch"
    out="dist/{{ platform }}"
    rm -rf "$out"
    mkdir -p "$out"
    mains="$(go list ./cmd/... 2>/dev/null || true)"
    if [[ -z "$mains" ]]; then go build ./...; exit 0; fi
    for pkg in $mains; do
        go build -trimpath -o "$out/${pkg##*/}$ext" "$pkg"
    done
    if [[ "{{ platform }}" == wasm ]]; then
        cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$out/"
        if [[ -f web/index.html ]]; then cp web/index.html "$out/"; fi
    fi

# Run the tests.
test: hold
    go test ./...

# Fail if go.mod or go.sum need tidying, then report what the linters cannot see.
guard:
    go mod tidy -diff
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard go

# Run every Go check CI runs.
verify: format-check typecheck lint guard test

# The Go checks the pre-commit hook runs.
pre-commit: format-check lint guard

# The Go checks the pre-push hook runs: verify without the tests, which CI runs.
pre-push: format-check typecheck lint guard

# Takes one of the machine's check slots for this run; see `hold-slot` in the justfile.
[private]
hold:
    @{{ quote(replace(just_executable(), '\', '/')) }} hold-slot "$PPID"
