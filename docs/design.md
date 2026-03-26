# Design

## Goals

`datasim` is intended to be a long-lived open source CLI, not a one-off script. The current design priorities are:

- a command surface that reflects real workload types
- clear separation between Cobra wiring and domain logic
- manifest-backed operations so the tool can reason about state over time
- public source code that is straightforward to read and extend

## Command model

The top-level command structure is workload-oriented.

- `datasim block-device ...` manages disposable block devices and mounted filesystems
- `datasim fileset ...` manages synthetic file-tree datasets

This is deliberate. A fileset workload is different from a future block, object, or database workload, so the CLI should model those as separate top-level areas rather than pretending they all fit behind the same generic interface.

Within `fileset`, profiles such as `corporate`, `school`, and `nasa` are just presets selected with `--profile`. They affect naming, directory structure, extension pools, and file-size tendencies, but they remain part of the same workload type.

## Architecture

### `cmd/datasim/` and `internal/cli/`

`cmd/datasim/` contains the binary entrypoint. `internal/cli/` contains Cobra command definitions. Commands parse flags, validate input, and delegate to the concrete services underneath.

### `internal/app/`

Application bootstrap and dependency wiring live here. This layer loads configuration and constructs the fileset, block-device, and self-update services used by the command layer.

### `skills/`

Repository-distributed agent skills live under `skills/` as standard `SKILL.md` assets. They are installed directly from the open-source repository with `npx skills add ...`, so they are versioned with the product but are not coupled to the compiled CLI binary.

### `internal/fileset/`

This is the core fileset workload package. It owns:

- built-in profile definitions
- initialization planning
- rotation planning
- manifest-backed fileset lifecycle operations

The important design choice here is that `fileset` is concrete. It is not a placeholder for every future workload type.

### `internal/filesystem/`

This package handles block-device lifecycle operations such as formatting, mounting, unmounting, and teardown. It is intentionally isolated from fileset generation logic.

### `internal/manifest/`

The manifest package owns the `.cirrusdata-datasim` contract. It stores the current state of a dataset and the history needed for subsequent rotation, inspection, and cleanup.

### `internal/config/`

This package loads configuration from disk and environment variables using Viper. It provides platform-specific defaults for filesystem type and state file location.

### `internal/update/`

This package implements the self-update service. It wraps `go-selfupdate` behind a testable `client` interface so the update workflow can be unit tested without network access. The service detects the latest GitHub release, compares versions via SemVer, and replaces the running binary in place.

### `internal/storage/`

This package provides filesystem capacity helpers used to derive the default fileset initialization size when `--size` is omitted.

### `pkg/bytefmt/`

This is a generic byte-size parsing and formatting utility. It converts human-readable size strings such as `512MiB` or `10GiB` to byte counts and back. It lives in `pkg/` because it has no dependency on datasim internals.

## Manifest model

The manifest is the source of truth for a fileset dataset. It currently stores:

- workload type
- profile name
- seed and strategy
- target generation size
- current dataset status
- tracked file inventory
- rotation history

That lets `fileset rotate`, `fileset status`, and `fileset destroy` operate from recorded state rather than by guessing from the filesystem.

## Fileset design

The current fileset model is intentionally profile-driven.

- `fileset` is the workload
- `--profile` selects a built-in flavor within that workload

That gives us room to add profile-specific flags later without forcing a cross-workload abstraction too early. If a future workload is fundamentally different, it should become a new top-level package and command family instead of being squeezed into fileset semantics.

## Storage design

`block-device format` and `block-device destroy` are kept separate from fileset lifecycle operations. That separation matters because users may want to:

- prepare a disposable test filesystem and then initialize a fileset inside it
- initialize a fileset directly in an already-mounted directory
- destroy a fileset without destroying the underlying block-device mount

## Near-term direction

The current scaffold leaves space for the next meaningful improvements:

- richer fileset metadata fidelity such as ownership mixes and xattrs
- additional fileset profiles
- integrity verification and checksum commands
- future workload families such as block or database
- expanded release packaging and CI automation beyond GitHub releases

## Release model

Releases are tag-driven and SemVer-based.

- GitHub Actions runs CI on pushes and pull requests.
- GitHub Actions runs GoReleaser when a tag such as `v0.2.0` is pushed.
- GoReleaser builds release archives for Linux and Windows on `amd64` and `arm64`.
- Each release publishes a shared `checksums.txt` file.
- `datasim update` uses the GitHub release as the source of truth, ignores prereleases by default, verifies the downloaded asset against `checksums.txt`, and then replaces the current executable.

The main constraint is simple: new capabilities should be added by introducing a concrete workload or package with a clear job, not by broadening generic abstractions before they are earned.
