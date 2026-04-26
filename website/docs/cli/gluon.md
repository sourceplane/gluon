---
title: gluon CLI
---

The root `gluon` command is the entry point for planning, inspection, and execution.

## Command map

| Command | Purpose |
| --- | --- |
| `gluon plan` | Compile intent and compositions into a deterministic execution plan |
| `gluon run` | Execute a compiled plan (executes by default; use `--dry-run` to preview) |
| `gluon status` | Show execution status for the latest or a specific execution |
| `gluon logs` | Stream or filter per-step logs from an execution |
| `gluon get` | List resources: `plans`, `runs`, `jobs`, `components`, `environments` |
| `gluon describe` | Show detailed information for a run, plan, job, or component |
| `gluon gc` | Clean up old executions and orphan plan files |
| `gluon validate` | Validate intent and discovered components against schemas |
| `gluon debug` | Inspect intent processing and planning internals |
| `gluon compositions` | List or inspect available compositions |
| `gluon component` | List components or inspect a merged component view |
| `gluon completion` | Generate shell completion scripts |

## Global flags

| Flag | Meaning |
| --- | --- |
| `--config-dir`, `-c` | Legacy fallback path or glob for folder-shaped compositions |
| `--version` | Print the CLI version |
| `--help` | Show command help |

`--config-dir` can also be set through `GLUON_CONFIG_DIR`, but packaged composition sources declared in the intent are the recommended path.

## Typical flow

```bash
# Plan
gluon plan --intent intent.yaml

# Inspect what was planned
gluon get jobs
gluon status

# Execute
gluon run

# Review logs if needed
gluon logs
```

The plan is stored in `.gluon/plans/` and resolved automatically by `gluon run`. You can also pass an explicit `--plan` reference.

Read the command-specific pages next if you need examples and flag details.
