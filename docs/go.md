# Go

How bedrock sets up Go, and why. The pins sit in `.just/go.just` (golangci-lint 2.15.0) and `go.mod` (Go 1.27).

## Toolchain

Go modules build and test the code with `go build` and `go test`. golangci-lint v2 formats it (`golangci-lint fmt`, with `goimports` and `golines`) and lints it, with the strict config from [`xsyetopz/golangci-lint-config-next`](https://github.com/xsyetopz/golangci-lint-config-next). `just go::setup` installs the pinned golangci-lint with `go install`, so the binary lands in `GOBIN` or `$GOPATH/bin`, which has to be on `PATH`, and downloads the modules.

## Files

| File | Owner | Content |
| --- | --- | --- |
| `.just/go.just` | bedrock | The recipes, which `bedrock update` rewrites |
| `.github/workflows/go.yml` | bedrock | CI: Go from `go.mod`, the pinned golangci-lint, and `just go::verify` |
| `.golangci.yml` | the project | The strict config, with `goimports` grouping your module path. Adopt writes it once and then keeps your copy |
| `go.mod` | the project | Written by adopt only when the repo has none |

`bedrock init <dir> go:command` starts a module with `cmd/<dir>/main.go` and an `internal/greeting` package. `go:library` starts a root package with a test. Both write tests that pass `just verify`. A new `go.mod` names `example.com/<dir>` unless `.bedrock.json` sets `"module"` under `go`, and adopt notes the placeholder so you replace it, and the `goimports` prefix in `.golangci.yml`, with the real path.

## Recipes

`just go::verify` runs `format-check`, `typecheck` (build every package and compile every test), `lint`, `guard`, and `test`. `guard` fails when `go mod tidy` would change `go.mod` or `go.sum`, then runs `bedrock guard go`, which reports the files of replaced dependency managers (`Gopkg.toml`, `glide.yaml`, `vendor/vendor.json`, `Godeps/Godeps.json`) and a `.golangci.yaml`, `.toml`, or `.json` beside `.golangci.yml`. Adopt runs `go mod tidy` after it applies.

## Resource Bounds

A lint or build run on a shared machine stays within these limits.

| Bound | Where | Default | What it caps |
| --- | --- | --- | --- |
| `run.concurrency` | `.golangci.yml` | 4 | The threads golangci-lint runs at once |
| `GOMEMLIMIT` | `.just/go.just` | 2GiB | The Go runtime's soft memory limit, which golangci-lint obeys. `BEDROCK_GO_MEMLIMIT` changes it |
| `GOFLAGS=-p=N` | `.just/go.just` | 4 | The build and test programs that run in parallel. `BEDROCK_GO_JOBS` changes it |

## Silencing a Finding

Write `//nolint:<linter> // <reason>`. nolintlint rejects a directive without a linter name or a reason.
