---
title: arx run
---

`arx run` consumes a compiled plan and either previews or executes it.

## Usage

```bash
arx run --plan plan.json
```

Without `--execute`, `run` stays in dry-run mode.

## Common examples

Execute the full plan locally:

```bash
arx run --plan plan.json --execute --runner local
```

Execute inside Docker:

```bash
arx run --plan plan.json --execute --runner docker
```

Force GitHub Actions compatibility mode:

```bash
arx run --plan plan.json --execute --gha
```

Show full step logs instead of the compact summary view:

```bash
arx run --plan plan.json --execute --verbose
```

Retry one failed job after clearing its saved state:

```bash
arx run --plan plan.json --execute --job-id web-app@staging.deploy --retry
```

Override the working directory for every job:

```bash
arx run --plan plan.json --execute --workdir ./examples
```

## Flags

| Flag | Meaning |
| --- | --- |
| `--plan`, `-p` | Path to the compiled plan file |
| `--execute`, `-x` | Actually execute commands instead of dry-running |
| `--verbose` | Expand full step logs instead of the compact summary view |
| `--workdir` | Override the working directory for all jobs |
| `--job-id` | Run only one job ID |
| `--retry` | Clear saved state for the selected `--job-id` before running |
| `--runner` | Choose `local`, `docker`, or `github-actions` |
| `--gha` | Shortcut for `--runner github-actions` |

## Backend resolution

`run` chooses its backend in this order:

1. `--gha`
2. `--runner`
3. `ARX_RUNNER`
4. `GITHUB_ACTIONS=true`
5. Auto-detection when the plan contains a `use:` step
6. Default to `local`

## State files

Executed plans record progress in the configured state file, usually `.arx-state.json`. That allows resumable execution and job-level retries while protecting against plan checksum mismatches.

By default, successful runs render a compact per-step view with summaries and results while collapsing noisy backend logs. Use `--verbose` when you need the raw step logs inline.