---
name: datasim
description: Use the datasim CLI locally or on remote Linux and Windows hosts to create, rotate, inspect, and clean up synthetic storage workloads, especially fileset datasets and disposable block-device-backed test filesystems.
---

## When To Use

Use this skill when the user wants to operate `datasim` itself: initialize a workload, rotate it over time, inspect recorded state, clean it up, or prepare a disposable filesystem for the simulation.

Do not explain how to install this skill unless the user explicitly asks. Focus on how to use `datasim`.

Assume the skill may be running on a different machine from the one where `datasim` is installed. The skill should help the user locate and run `datasim` on the actual workload host.

## Instructions

First determine where `datasim` needs to run:

1. Local host:
   Run `datasim` directly.
2. Remote Linux host:
   Prefer `ssh <host> '<command>'` for one-off checks and operations.
3. Remote Windows host:
   Prefer PowerShell remoting or SSH if the host supports it. Be explicit about Windows paths, drive letters, and `datasim.exe`.

When the binary location is unclear, help the user find it before giving workload commands:

- Linux or macOS:
  - `command -v datasim`
  - `which datasim`
  - `datasim version`
- Windows:
  - `Get-Command datasim`
  - `where.exe datasim`
  - `datasim version`

If `datasim` is not on `PATH`, ask for or help discover the full executable path and use that explicit path in later commands.

Start by identifying which of these workflows the user needs:

1. Existing filesystem already mounted:
   Use `datasim fileset init`, `status`, `rotate`, `rotate loop`, and `destroy`.
2. Raw block device or disposable disk:
   Use `datasim block-device format` first, then operate `fileset` inside the mounted path, and finish with `datasim block-device destroy` when teardown is wanted.
3. Build and release support:
   Use `datasim version` to inspect build metadata and use `datasim update` only for released builds with a SemVer version.

Prefer these command patterns:

- `datasim fileset init --fs <mount-point> --profile <profile> [--size <size>] [--seed <seed>]`
- `datasim fileset status <mount-point>`
- `datasim fileset rotate --fs <mount-point> [--create-pct N --delete-pct N --modify-pct N]`
- `datasim fileset rotate loop --fs <mount-point> --interval <duration>`
- `datasim fileset destroy <mount-point>`
- `datasim block-device format <device> <mount-point>`
- `datasim block-device destroy <mount-point>`

Use `--seed` when the user wants a repeatable dataset. Omit it when the user wants a fresh randomized run.

Before destructive actions, prefer checking `datasim fileset status <mount-point>` so the current workload state and manifest-backed root are visible.

When helping with remote execution, prefer concrete host-side commands such as:

- Linux over SSH:
  - `ssh user@host 'datasim version'`
  - `ssh user@host 'datasim fileset status /mnt/test'`
- Windows over SSH:
  - `ssh user@host 'datasim.exe version'`
  - `ssh user@host 'datasim.exe fileset status X:\'`

When using an explicit binary path remotely, preserve that path in all follow-up commands instead of switching back to bare `datasim`.

Use `datasim --help` and subcommand `--help` output to confirm current flags and examples before giving detailed command guidance.

## Notes

- The primary built-in workload today is `fileset`.
- Each initialized fileset records state in `.cirrusdata-datasim`.
- `block-device` is a convenience utility around disposable test filesystems; it is not required when the user already has a mounted target path.
