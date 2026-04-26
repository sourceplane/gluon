---
title: gluon describe
---

`gluon describe` shows detailed information about a specific run, plan, job, or component.

## Usage

```bash
gluon describe <resource> [name]
```

Supported resources: `run`, `plan`, `job`, `component`.

## Common examples

Describe the latest execution:

```bash
gluon describe run
gluon describe run latest
```

Describe a specific execution:

```bash
gluon describe run my-plan-20240601-a1b2c3
```

Describe the latest plan:

```bash
gluon describe plan
```

Describe a plan by name or checksum prefix:

```bash
gluon describe plan release-candidate
gluon describe plan a1b2c3
```

Describe a job from the latest plan:

```bash
gluon describe job api-edge-worker@production.deploy
```

Describe a component:

```bash
gluon describe component web-app
```

## Slash notation

`describe` also accepts slash notation directly on the parent command:

```bash
gluon describe run/latest
gluon describe plan/release-candidate
gluon describe job/api-edge-worker@production.deploy
gluon describe component/web-app
```

## Output

### `describe run`

Shows full execution metadata including plan reference, status, timing, trigger, and a per-job breakdown with status, duration, and any errors.

### `describe plan`

Shows plan ID, generated timestamp, checksum, concurrency settings, composition sources, and a full job list with dependency edges.

### `describe job`

Shows component, environment, composition, working directory, timeout, retries, dependencies, and step details (run commands or `use:` references). If an execution record exists, also shows the job's runtime state.

### `describe component`

Equivalent to `gluon component <name> --long`. Shows the merged view with all inputs, labels, overrides, and per-environment instances.

## Related commands

- `gluon status` — compact live view of an execution
- `gluon logs` — raw step output
- `gluon get` — listing views for plans, jobs, components, environments
