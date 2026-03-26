# datasim

`datasim` is a cross-platform CLI for creating synthetic workloads you can run against storage, migration, sync, and integrity-validation systems.

It helps you stand up realistic test data, evolve that data over time, inspect the current state, and clean everything up when the run is over.

Today, the first built-in workload is `fileset`, which generates and rotates realistic file trees. The CLI is intentionally organized around workload families so `datasim` can grow beyond filesets over time with additional simulations such as database-style or block-oriented workloads.

## Why datasim

Use `datasim` when you want to:

- create realistic synthetic data without hand-building test fixtures
- size a dataset by capacity target instead of manually picking file counts
- simulate churn by creating, deleting, and modifying data over time
- prepare disposable test filesystems for workload runs
- inspect manifest-backed state instead of guessing from the filesystem
- repeat the same test scenario with a known seed

## Install

Download the latest release for your OS and architecture from [GitHub Releases](https://github.com/cirrusdata/datasim/releases).

Extract the archive and place the `datasim` binary somewhere on your `PATH`.

Verify the install:

```bash
datasim version
```

If you installed from a release build, you can update in place later with:

```bash
datasim update
```

## OS support

Official release binaries are currently published for:

- Linux (`amd64`, `arm64`)
- Windows (`amd64`, `arm64`)

Platform-specific notes:

- `datasim fileset` is the primary workload and is intended for normal CLI use on the supported release platforms
- `datasim block-device` is supported on Linux and Windows only
- on Linux, block-device formatting supports `xfs` by default and `ext4` as an option
- on Windows, block-device formatting uses `ntfs` and supports either drive letters or directory mount points
- macOS and other operating systems do not currently have official release artifacts, and `datasim block-device` is not supported there

## Commands / Simulated Workloads
`datasim` uses top-level command families for each workload or support area:

- `datasim block-device`: utilities to prepare and tear down disposable filesystems for test runs
- `datasim fileset`: manage synthetic file-tree datasets

Right now, `fileset` is the primary workload. More workload families can be added over time without forcing everything into file-tree semantics.

## Fileset

The current `fileset` workload is designed to look and behave more like a real dataset than a flat pile of random files.

With `datasim fileset`, you can:

- initialize a dataset in an existing mounted path
- choose a built-in profile such as `corporate`, `school`, or `nasa`
- generate content toward a target size
- rotate the dataset over time
- inspect manifest-backed status
- destroy the generated dataset cleanly

### Quick start

If you already have a mounted filesystem, initialize a synthetic fileset in it:

```bash
datasim fileset init --fs /mnt/datasim-source --profile corporate --size 512MiB
```

Rotate the dataset to simulate ongoing change:

```bash
datasim fileset rotate --fs /mnt/datasim-source
```

Inspect the current workload state:

```bash
datasim fileset status /mnt/datasim-source
```

Remove the generated dataset when you are done:

```bash
datasim fileset destroy /mnt/datasim-source
```

### Common workflows

Prepare a disposable filesystem and then populate it:

```bash
datasim block-device format /dev/sdc1 /mnt/datasim-source
datasim fileset init --fs /mnt/datasim-source --profile corporate --size 10GiB
```

On Windows, mount to a drive letter:

```powershell
datasim block-device format \\.\PHYSICALDRIVE4 X:\
datasim fileset init --fs X:\ --profile corporate --size 10GiB
```

Recreate an already-mounted test filesystem in place:

```bash
datasim block-device format /dev/sdc1 /mnt/datasim-source --force
```

Run repeated churn every 5 minutes:

```bash
datasim fileset rotate loop --fs /mnt/datasim-source --interval 5m
```

Clean up the workload and then remove the mounted test filesystem:

```bash
datasim fileset destroy /mnt/datasim-source
datasim block-device destroy /mnt/datasim-source
```

Install the open-source datasim agent skill:

```bash
npx skills add cirrusdata/datasim --skill datasim-operator
```

### Profiles

The built-in fileset profiles are:

- `corporate`
- `school`
- `nasa`

`corporate` is the default.

Choose a profile with `--profile`:

```bash
datasim fileset init --fs /mnt/datasim-source --profile school --size 5GiB
```

### Size-driven initialization

`datasim fileset init` is driven by size:

- set `--size` to choose an explicit initialization target
- omit `--size` to default to 80% of the target filesystem capacity
- set `--seed` when you want a reproducible dataset shape

### Rotation behavior

By default, `datasim fileset rotate` applies:

- `5%` create
- `5%` delete
- `10%` modify

Override those values when you want a different churn pattern:

```bash
datasim fileset rotate \
  --fs /mnt/datasim-source \
  --create-pct 10 \
  --delete-pct 2 \
  --modify-pct 20
```

### Manifest

Each initialized fileset writes a `.cirrusdata-datasim` JSON manifest at the dataset root.

That manifest records the workload type, selected profile, initialization settings, tracked files, current status, and rotation history.

Use `datasim fileset status <path>` to inspect the recorded state without opening the manifest manually.

## Block-device helpers

`datasim block-device format` formats and mounts a block device for test use.

By default, `datasim block-device format` refuses to reuse a mount point or block device that is already mounted. Use `--force` when you want the same command to unmount and recreate the filesystem in place.

`datasim block-device destroy` unmounts and tears down a block-device mount created for datasim.

On Linux, the default filesystem is `xfs`. On Windows, the default is `ntfs`, and the mount target may be either a drive letter such as `X:\` or a directory mount point such as `C:\datasim-mount`.

## Help

Inspect the CLI directly:

```bash
datasim --help
datasim fileset --help
datasim fileset init --help
datasim block-device --help
```

For engineering and architecture notes, see [docs/design.md](docs/design.md).
