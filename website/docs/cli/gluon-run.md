---
title: gluon run
---

`gluon run` executes the jobs and steps from a compiled plan. **Execution is the default** — add `--dry-run` to preview without running.

## Usage

```bash
gluon run
```

With no flags, `run` resolves the latest plan from `.gluon/plans/latest.json` and executes it.

## Common examples

Preview the execution order without running:

```bash
gluon run --dry-run
```

Execute with an explicit plan reference:

```bash
gluon run --plan my-plan
```

Execute a specific plan file:

```bash
gluon run --plan /tmp/plan.json
```

Execute inside Docker:

```bash
gluon run --runner docker
```

Force GitHub Actions compatibility mode:

```bash
gluon run --gha
```

Show full step logs instead of the compact summary view:

```bash
gluon run --verbose
```

Run only one job:

```bash
gluon run --job web-app@staging.deploy
```

Retry a failed job (clears its saved state first):

```bash
gluon run --job web-app@staging.deploy --retry
```

Override the working directory for every job:

```bash
gluon run --workdir ./examples
```

Filter to one environment or component at runtime:

```bash
gluon run --env staging
gluon run --component api-edge-worker
```

Override plan concurrency:

```bash
gluon run --concurrency 4
```

Pin an execution ID for CI/parallel-safe tracking:

```bash
gluon run --exec-id ci-run-${GITHUB_RUN_ID}
```

## Flags

| Flag | Meaning |
| --- | --- |
| `--plan`, `-p` | Plan reference: file path, name, checksum prefix, or `latest` (default: `latest`) |
| `--dry-run` | Preview what would run without executing |
| `--verbose` | Expand full step logs instead of the compact summary view |
| `--workdir` | Override the working directory for all jobs |
| `--job` | Run only one job ID (matches plan job ID or prefix) |
| `--retry` | Clear existing state for `--job` before running |
| `--runner` | Execution backend: `local`, `docker`, or `github-actions` |
| `--gha` | Shortcut for `--runner github-actions` |
| `--exec-id` | Execution ID for resume or CI tracing (auto-generated if not set) |
| `--concurrency` | Override plan concurrency (0 uses the plan's value) |
| `--component` | Filter jobs to a specific component (repeatable) |
| `--env`, `-e` | Filter jobs to a specific environment |
| `--json` | Output execution summary in JSON format |

:::note Deprecated flag
`--job-id` is a deprecated alias for `--job`. Use `--job` in new scripts.
:::

## Backend resolution

`run` chooses its backend in this order:

1. `--gha`
2. `--runner`
3. `GLUON_RUNNER`
4. `GITHUB_ACTIONS=true`
5. Auto-detection when the plan contains a `use:` step
6. Default to `local`

## State and execution records

Each run creates an execution record under `.gluon/executions/{exec-id}/`:

- `state.json` — per-job and per-step completion status
- `metadata.json` — timing, user, trigger, and job counts
- `logs/{job}/{step}.log` — raw step output

That structure lets `run`:

- skip already-completed jobs on resume
- retry a single failed job with `--job` and `--retry`
- record immutable per-execution logs accessible via `gluon logs`

If a `.gluon-state.json` file exists from a pre-v0.10 installation, `gluon run` automatically migrates it on the first execution.

## Plan resolution

`--plan` accepts:

| Value | Resolves to |
| --- | --- |
| _(omitted)_ | `.gluon/plans/latest.json` |
| `latest` | `.gluon/plans/latest.json` |
| `my-plan` | `.gluon/plans/my-plan.json` |
| `a1b2c3` | Any plan whose checksum starts with `a1b2c3` |
| `./plan.json` | Explicit file path |

When no plan exists yet, `run` automatically generates one from `intent.yaml` in the current directory before executing.
