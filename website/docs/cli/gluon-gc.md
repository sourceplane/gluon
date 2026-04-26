---
title: gluon gc
---

`gluon gc` removes old execution records and orphan plan files based on a retention policy.

## Usage

```bash
gluon gc
```

By default, `gc` keeps the most recent 10 executions and removes anything older than 30 days. Plans that are no longer referenced by any execution and are older than 30 days are also removed.

## Common examples

Preview what would be removed without deleting anything:

```bash
gluon gc --dry-run
```

Keep only the last 5 executions:

```bash
gluon gc --keep 5
```

Remove executions older than 7 days:

```bash
gluon gc --max-age 7
```

Remove all execution records:

```bash
gluon gc --all
```

## Flags

| Flag | Meaning |
| --- | --- |
| `--dry-run` | Print what would be deleted without removing anything |
| `--keep` | Number of most recent executions to retain (default: `10`) |
| `--max-age` | Maximum age in days for retained executions (default: `30`) |
| `--all` | Remove all executions regardless of count or age |

## What gets removed

- Execution directories under `.gluon/executions/` that exceed the retention policy
- Orphan plan files under `.gluon/plans/` that are older than 30 days and not referenced by any retained execution

The `latest` symlink and named plans are not removed by `gc`.
