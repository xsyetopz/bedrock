# Managed by go-bedrock. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# Bounds that keep one run from taking the machine. GOMEMLIMIT is a soft limit on the Go
# runtime, golangci-lint included. `-p` caps the build and test programs that run at once;
# `run.concurrency` in .golangci.yml caps golangci-lint's own threads.
jobs := env("BEDROCK_GO_JOBS", "4")
export GOMEMLIMIT := env("BEDROCK_GO_MEMLIMIT", "2GiB")
export GOFLAGS := env("GOFLAGS", "") + " -p=" + jobs

# Install golangci-lint when it is missing, then download the modules.
setup:
    if ! command -v golangci-lint >/dev/null; then brew install golangci-lint; fi
    go mod download

# Format the Go code.
format:
    golangci-lint fmt

# Fail if the Go code is not formatted.
format-check:
    golangci-lint fmt --diff

# Build every package and compile every test, without running anything.
typecheck:
    go build ./...
    go test -run '^$' ./...

# Run golangci-lint.
lint:
    golangci-lint run

# Apply the fixes golangci-lint can make, then format.
lint-fix:
    golangci-lint run --fix
    golangci-lint fmt

# Run the tests.
test:
    go test ./...

# Fail if go.mod or go.sum need tidying.
guard:
    go mod tidy -diff

# Run every Go check CI runs.
verify: format-check typecheck lint guard test

# The Go checks the pre-commit hook runs.
pre-commit: format-check lint guard
