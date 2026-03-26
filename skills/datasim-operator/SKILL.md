---
name: datasim-operator
description: Operate datasim workloads and support commands to prepare synthetic data for migration, sync, and integrity validation workflows.
---

## Install

Install the open-source datasim skill with the skills.sh CLI:

```bash
npx skills add cirrusdata/datasim --skill datasim-operator
```

To target Codex explicitly:

```bash
npx skills add cirrusdata/datasim --skill datasim-operator --agent codex
```

## When To Use

Use this skill when you need to prepare, rotate, inspect, or clean up datasim workloads for migration, sync, and integrity validation workflows.

Today, the primary built-in workload is `fileset`, but datasim is organized to grow into additional workload families over time.

## Instructions

- Use `datasim --help` to inspect the available workload and support command families.
- Use `datasim fileset init --fs <mount-point> [--profile <profile>]` to create a synthetic file-tree workload and write its manifest.
- Use `datasim fileset rotate --fs <mount-point>` or `datasim fileset rotate loop --fs <mount-point>` to simulate workload churn over time.
- Use `datasim fileset status <mount-point>` before destructive operations to inspect the recorded workload state.
- Use `datasim fileset destroy <mount-point>` to remove a fileset workload while leaving the underlying filesystem available.
- Use `datasim block-device format <device> <mount-point>` and `datasim block-device destroy <mount-point>` only when preparing or tearing down disposable test filesystems.
- Use `datasim version` to inspect build metadata and `datasim update` only for released builds.

## Notes

- Each initialized fileset records state in `.cirrusdata-datasim`.
