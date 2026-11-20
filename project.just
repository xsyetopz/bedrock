# Build the release binaries into dist/, with the templates embedded, and write
# SHA256SUMS, which `just setup` checks each download against. The asset names
# match bedrock_asset in templates/base/files/justfile.tmpl.
[group("release")]
dist:
    #!/usr/bin/env bash
    set -euo pipefail
    rm -rf dist
    mkdir dist
    # A static, path-free binary: it runs inside other people's repositories on
    # machines with no Go toolchain. Windows runs a program only by its .exe name.
    for target in darwin-arm64 darwin-x64 linux-arm64 linux-x64 windows-arm64 windows-x64; do
        os="${target%-*}"
        arch="${target#*-}"
        [[ "$arch" == x64 ]] && arch=amd64
        suffix=""
        [[ "$os" == windows ]] && suffix=.exe
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
            go build -trimpath -ldflags="-s -w" -o "dist/bedrock-$target$suffix" ./cmd/bedrock
    done
    cd dist
    shasum -a 256 bedrock-* > SHA256SUMS
    cat SHA256SUMS
