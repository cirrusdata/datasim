# Repository Guidelines

## Project Structure & Module Organization

`datasim` uses [`cmd/datasim/`](cmd/datasim) for the binary entrypoint and [`internal/cli/`](internal/cli) for Cobra command wiring. Keep CLI files thin: parse flags, validate input, and delegate. Core logic lives under [`internal/`](internal):

- `internal/fileset/`: fileset profiles, planning, init/rotate/destroy logic
- `internal/filesystem/`: block-device format/mount/teardown helpers
- `internal/manifest/`: `.cirrusdata-datasim` schema and persistence
- `internal/config/`, `internal/app/`, `internal/storage/`: bootstrap and shared runtime support

Reusable helpers belong in [`pkg/`](pkg) only when they are truly generic.

## Build, Test, and Development Commands

- `go build -o datasim ./cmd/datasim`: build the CLI locally
- `go test ./...`: run the full test suite
- `go run ./cmd/datasim --help`: inspect the top-level CLI
- `go run ./cmd/datasim fileset init --help`: inspect a workload command
- `go run ./cmd/datasim update --help`: inspect the self-update command
- `goreleaser release --snapshot --clean`: validate release packaging locally
- `gofmt -w cmd/datasim/main.go internal/cli/*.go internal/*/*.go pkg/*/*.go`: format Go sources before submitting

Use `go run ./cmd/datasim fileset ...` and `go run ./cmd/datasim block-device ...` to manually verify behavior after changing commands.

## Coding Style & Naming Conventions

Follow standard Go style and keep code explicit over clever. Use tabs as produced by `gofmt`. Every Go function must have a Go doc comment. Name CLI files by noun/verb pattern, for example `internal/cli/fileset_init.go` or `internal/cli/block_device_format.go`. Keep workload-specific logic inside the relevant package instead of introducing broad abstractions early.

## Testing Guidelines

Use Go’s built-in `testing` package. Place tests next to the code they cover, using `_test.go` suffix and descriptive names such as `TestInitAndRotate`. Add or update tests whenever manifest behavior, rotation behavior, or command-facing options change.

## Commit & Pull Request Guidelines

Use Conventional Commits and tag releases with SemVer versions such as `v0.2.0`. Pull requests should include a brief summary, test or verification notes, and any manifest, CLI help text, or release-pipeline changes. Update [`README.md`](README.md) for user-facing changes and [`docs/design.md`](docs/design.md) for architectural changes.
