# Contributing

Thanks for contributing to `datasim`.

This project is intended to be a public-facing CLI with a clear command model and a codebase that stays easy to read as new workload types are added. The conventions below are meant to keep that standard high.

## Project Principles

- Keep the CLI user-facing and task-oriented.
- Prefer concrete workload packages over generic abstractions.
- Keep command wiring separate from implementation logic.
- Treat public docs and help text as product surface area.
- Preserve manifest compatibility carefully.

## Current Command Model

The current top-level nouns are:

- `block-device`
- `fileset`

That split is intentional.

- `block-device` owns formatting, mounting, and teardown of disposable filesystems.
- `fileset` owns generation, rotation, inspection, and cleanup of synthetic file-tree datasets.

If a future feature is fundamentally a new workload type, prefer adding a new top-level command family rather than extending `fileset` beyond its natural scope.

## Repository Layout

- `cmd/datasim/` contains the binary entrypoint.
- `internal/cli/` contains Cobra commands only.
- `internal/fileset/` contains fileset-specific planning, profiles, and lifecycle logic.
- `internal/filesystem/` contains block-device lifecycle helpers.
- `internal/manifest/` contains the `.cirrusdata-datasim` manifest contract.
- `internal/storage/` contains storage-capacity helpers.
- `docs/` contains engineering and design documentation.

## Coding Conventions

- Write Go code in a straightforward, maintainable style.
- Prefer explicit code over clever abstractions.
- Keep functions and types named for the domain they serve.
- Every Go function should have a Go doc comment.
- Keep comments factual and concise.
- Use ASCII unless a file already requires something else.

## CLI Conventions

- Keep help text short, clear, and operator-focused.
- Prefer noun-verb command structure when it reflects the actual domain.
- Add flags only where they naturally belong.
- Avoid introducing cross-workload flags or abstractions unless at least two real workloads need them.

## Release Conventions

- Use Conventional Commits for commit messages.
- Version releases with SemVer tags such as `v0.2.0`.
- GitHub Actions publishes releases only from matching tags.
- Keep release artifact names compatible with `datasim update`.
- `datasim update` follows the latest stable GitHub release only and verifies `checksums.txt` before replacing the binary.

## Documentation Conventions

- `README.md` should stay user-facing.
- `docs/design.md` is the right place for architecture and implementation detail.
- Update help text and docs whenever command behavior changes.

## Testing

Before submitting a change, run:

```bash
go test ./...
```

If you change command behavior, also exercise the relevant CLI flow locally with `go run ./cmd/datasim`.

## Formatting

Run:

```bash
gofmt -w cmd/datasim/main.go internal/cli/*.go internal/*/*.go pkg/*/*.go
```

Or format the changed Go files directly if you prefer.

## Manifest Changes

Be careful when changing `.cirrusdata-datasim`.

- Preserve existing meaning unless there is a clear migration plan.
- Prefer additive changes over disruptive schema changes.
- Keep status and history fields readable and stable.

## Adding a Fileset Profile

If you are adding a new `fileset` profile:

1. Add the profile definition under `internal/fileset/`.
2. Register it in the fileset catalog.
3. Keep the profile focused on file-tree shape, names, extensions, and size tendencies.
4. Update user docs if the new profile is intended to be public.

## Pull Request Notes

Good pull requests for this repo usually include:

- a short explanation of the user-visible change
- tests or verification notes
- doc/help text updates when needed
- explicit mention of any manifest changes
- note whether the change affects release packaging, self-update, or SemVer behavior
