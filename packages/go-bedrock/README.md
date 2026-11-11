# Go Bedrock

Shared tooling for strict Go modules. The module follows the [golang-standards project layout](https://github.com/golang-standards/project-layout): commands under `cmd/<name>/`, private code under `internal/`.

## Commands

| Command | Does |
| --- | --- |
| `go-bedrock new <dir> <module>` | Start a module in `<dir>` with `go.mod`, `cmd/<dir>/main.go`, `.golangci.yml`, and `.just/go.just`. |
| `go-bedrock adopt [<dir>] [--dry-run \| --yes]` | Set up an existing module. It prints the plan by default and writes it with `--yes`. |

Run it from this checkout with `go run -C packages/go-bedrock ./cmd/go-bedrock <command>`.

## What It Sets Up

- `.golangci.yml`, the strict golangci-lint v2 config from [`xsyetopz/golangci-lint-config-next`](https://github.com/xsyetopz/golangci-lint-config-next), with `goimports` grouping your module path. It is yours once written: `adopt` keeps it.
- `.just/go.just`, which `adopt` keeps current. It formats with `golangci-lint fmt`, lints with `golangci-lint run`, builds and compiles every test, runs `go test`, and fails when `go mod tidy` would change `go.mod` or `go.sum`.

## Resource Bounds

A lint or build run on a shared machine stays within these limits.

| Bound | Where | Default | What it caps |
| --- | --- | --- | --- |
| `run.concurrency` | `.golangci.yml` | 4 | The threads golangci-lint runs at once. |
| `GOMEMLIMIT` | `.just/go.just` | 2GiB | The Go runtime's soft memory limit, which golangci-lint obeys. Set `BEDROCK_GO_MEMLIMIT` to change it. |
| `GOFLAGS=-p=N` | `.just/go.just` | 4 | The build and test programs that run in parallel. Set `BEDROCK_GO_JOBS` to change it. |

## Requirements

Go 1.27 or newer and golangci-lint 2.15 or newer. `just go::setup` installs golangci-lint with Homebrew when it is missing.
