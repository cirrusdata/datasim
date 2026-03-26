# datasim

datasim is a workload simulation tool to facilitate storage systems testing and demonstration on Windows and Linux. 

The first simulation included is `fileset`, which generates and rotates synthetic file trees. Additional simulation scenarios are planned which may include other databases, file/object-oriented and block-oriented workload simulations.

## Installing

Download the latest prebuilt binary from the [releases page](https://github.com/cirrusdata/datasim/releases). Each release includes copy-paste install commands for Linux (amd64 and arm64) and Windows (amd64 and arm64).


## fileset

The `fileset` simulation generates a directory tree with realistic file names, extensions, and directory structure on a mounted filesystem. datasim tracks everything it created in a manifest file and lets you apply ongoing churn to simulate active usage. When the test is done, one command removes exactly what was created and nothing else.

> [!NOTE]
> File content is currently randomly generated. This simulation produces realistic-looking structure, not readable documents.

### Quick Start

```bash
# Initialize a 10 GiB corporate-profile dataset
datasim fileset init --fs /mnt/test --profile corporate --size 10GiB

# See what was created
datasim fileset status /mnt/test

# Apply one round of file churn (creates, deletes, and modifies files)
datasim fileset rotate --fs /mnt/test

# Remove everything datasim created
datasim fileset destroy /mnt/test
```


### init

`datasim fileset init` populates a directory with a synthetic file tree and writes a `.cirrusdata-datasim` manifest file at the root. The manifest tracks every file datasim created so that later commands know exactly what is there without scanning the directory.

```bash
datasim fileset init --fs /mnt/test --profile corporate --size 10GiB
```

`--fs` is the directory to populate. `--profile` picks the dataset profile; it defaults to `corporate` if you leave it out. `--size` sets the target dataset size, with suffixes like `MiB`, `GiB`, or `TiB`. If you omit `--size`, datasim targets 80% of the filesystem's available capacity.

To get the same dataset every time — useful when you need a repeatable test scenario — pass a fixed seed:

```bash
datasim fileset init --fs /mnt/test --profile nasa --size 50GiB --seed 42
```

The `--strategy` flag controls how files are distributed across the tree. `balanced` (the default) applies the profile's characteristic distribution. `random` produces higher-variance output with irregular file counts and sizes, which can stress certain edge cases in migration or sync tools.

#### Dataset Profile

A profile controls the shape of the generated dataset: how directories are named, how deep the tree goes, what file extensions appear, and how large files tend to be.

`corporate` produces a generic office file tree with documents, spreadsheets, PDFs, and images organized into project-style subdirectories. Use this when you want something that looks like a typical shared drive or file server.

`school` mimics academic file collections — course materials, readings, and student work organized by subject or term. It produces a wider mix of document and media types at smaller average file sizes.

`nasa` generates data shaped like scientific or research archives: larger files, more uniform naming, and directory structures that resemble instrument or mission data. Use this when you need a dataset that is less document-heavy and more data-file-heavy.


### status

`datasim fileset status` reads the manifest and prints the current state of the dataset: file count, total bytes, profile, seed, last operation, number of rotations applied, and a per-category file breakdown.

```bash
datasim fileset status /mnt/test
```

Add `--json` to print the raw manifest as JSON, which is useful for scripting or piping into other tools:

```bash
datasim fileset status /mnt/test --json
```

### rotate

`datasim fileset rotate` applies a round of changes to an existing dataset. It creates new files, deletes some existing ones, and modifies others. The defaults are 5% create, 5% delete, and 10% modify — applied to the current file count.

```bash
datasim fileset rotate --fs /mnt/test
```

Override any of the rates when you want a different churn pattern. For a heavy-write scenario with minimal deletes:

```bash
datasim fileset rotate --fs /mnt/test --create-pct 20 --delete-pct 1 --modify-pct 5
```

For a dataset under heavy deletion pressure:

```bash
datasim fileset rotate --fs /mnt/test --create-pct 2 --delete-pct 30 --modify-pct 5
```

Each rotation is recorded in the manifest with its timestamp, seed, and operation counts. You can see the history with `datasim fileset status /mnt/test`.

### rotate loop

`datasim fileset rotate loop` runs rotation on a repeating schedule. Use this when you want the dataset to keep churning throughout a longer test run without calling rotate manually.

```bash
datasim fileset rotate loop --fs /mnt/test --interval 5m
```

By default it runs until you stop it with Ctrl-C. Set `--iterations` to limit the number of rounds:

```bash
datasim fileset rotate loop --fs /mnt/test --interval 2m --iterations 30
```

All the same percentage flags from `rotate` apply here:

```bash
datasim fileset rotate loop --fs /mnt/test --interval 1m --create-pct 10 --delete-pct 10 --modify-pct 20
```

### destroy

`datasim fileset destroy` removes everything the fileset created — the generated files, directories, and the manifest itself. It uses the manifest to determine what to remove, so it does not touch anything it did not create.

```bash
datasim fileset destroy /mnt/test
```


## Updating

If you installed datasim from a release binary, you can update it in place:

```bash
datasim update
```

This fetches the latest stable release, verifies its checksum, and replaces the running binary.

## Agent Skill

If you are using an AI coding agent, install the datasim operator skill:

```bash
npx skills add cirrusdata/datasim --skill datasim-operator
```

## Getting help

Every command and subcommand has a `--help` flag:

```bash
datasim --help
datasim fileset --help
datasim fileset init --help
datasim fileset rotate --help
```

For engineering notes and architecture details, see [docs/design.md](docs/design.md).

---

## Appendix: block-device utility

`datasim block-device` is a convenience utility for formatting and mounting a raw disk or partition before running a simulation, and tearing it down afterward. It is not part of the simulation itself — if you already have a mounted filesystem, you do not need it.

### Linux

```bash
# Format /dev/sdc1 and mount it at /mnt/test
datasim block-device format /dev/sdc1 /mnt/test

# Run the simulation
datasim fileset init --fs /mnt/test --profile corporate --size 50GiB
# ...
datasim fileset destroy /mnt/test

# Unmount and clean up
datasim block-device destroy /mnt/test
```

The default filesystem type on Linux is `xfs`. Use `--fstype ext4` if your workload requires ext4:

```bash
datasim block-device format /dev/sdc1 /mnt/test --fstype ext4
```

If the device or mount point is already in use and you want to wipe and reuse it without unmounting first, add `--force`. Without it, the command refuses to touch a device or mount point that is already mounted.

### Windows

Use either a drive letter or a directory mount point. The default filesystem type is `ntfs`.

```powershell
datasim block-device format \\.\PHYSICALDRIVE4 X:\
datasim fileset init --fs X:\ --profile corporate --size 10GiB
datasim fileset destroy X:\
datasim block-device destroy X:\
```

---

## License

This project is licensed under the [Apache License 2.0](LICENSE).
