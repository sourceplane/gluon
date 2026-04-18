---
title: ciz CLI
---

The root `ciz` command is the entry point for planning, inspection, and execution.

## Command map

| Command | Purpose |
| --- | --- |
| `ciz plan` | Compile intent and compositions into a deterministic execution plan |
| `ciz run` | Dry-run or execute a compiled plan |
| `ciz validate` | Validate intent and discovered components against schemas |
| `ciz debug` | Inspect intent processing and planning internals |
| `ciz compositions` | List or inspect available compositions |
| `ciz component` | List components or inspect a merged component view |
| `ciz completion` | Generate shell completion scripts |

## Global flags

| Flag | Meaning |
| --- | --- |
| `--config-dir`, `-c` | Path or glob used to load composition assets |
| `--version` | Print the CLI version |
| `--help` | Show command help |

`--config-dir` can also be set through `CIZ_CONFIG_DIR`. The deprecated `LITECI_CONFIG_DIR` alias is still accepted.

## Typical flow

```bash
ciz validate --intent intent.yaml --config-dir assets/config/compositions
ciz plan --intent intent.yaml --config-dir assets/config/compositions --output plan.json
ciz run --plan plan.json
```

Read the command-specific pages next if you need examples and flag details.