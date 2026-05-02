---
title: Distributed execution with remote state
---

Run `orun` jobs across parallel GitHub Actions matrix workers coordinated through [orun-backend](https://github.com/sourceplane/orun-backend).  The backend enforces DAG ordering — each matrix job polls until its dependencies complete, then claims work and executes.

## How it works

1. A **plan job** generates the plan and emits the job list as a `matrix` output.
2. A **matrix fan-out** starts one runner per job ID.
3. Each runner calls `orun run --remote-state --job <id>` — the backend ensures only jobs whose dependencies have completed are claimed.

## Prerequisites

| Item | Purpose |
| --- | --- |
| orun-backend instance | Coordinates remote state and DAG ordering |
| `ORUN_BACKEND_URL` | Variable pointing to the backend URL |
| `ORUN_TOKEN` or OIDC | Auth token (static or GitHub Actions OIDC with audience `orun`) |

## Full workflow

See [`examples/github-actions/remote-state-matrix.yml`](https://github.com/sourceplane/orun/blob/main/examples/github-actions/remote-state-matrix.yml) for the complete GitHub Actions workflow.

### Plan generation step

```yaml
- name: Generate plan
  id: plan
  env:
    ORUN_BACKEND_URL: ${{ vars.ORUN_BACKEND_URL }}
    ORUN_TOKEN: ${{ secrets.ORUN_TOKEN }}
  run: |
    orun plan --output plan.json --format json
    echo "plan-id=$(cat plan.json | jq -r '.metadata.checksum')" >> "$GITHUB_OUTPUT"
    MATRIX=$(cat plan.json | jq -c '[.jobs[].id]')
    echo "matrix=${MATRIX}" >> "$GITHUB_OUTPUT"
```

### Matrix execution step

```yaml
run:
  name: Run ${{ matrix.job }}
  needs: plan
  strategy:
    fail-fast: false
    matrix:
      job: ${{ fromJson(needs.plan.outputs.matrix) }}
  steps:
    - name: Execute job
      env:
        ORUN_BACKEND_URL: ${{ vars.ORUN_BACKEND_URL }}
        ORUN_TOKEN: ${{ secrets.ORUN_TOKEN }}
        ORUN_PLAN_ID: ${{ needs.plan.outputs.plan-id }}
      run: |
        orun run \
          --remote-state \
          --backend-url "$ORUN_BACKEND_URL" \
          --plan plan.json \
          --job "${{ matrix.job }}"
```

## Authentication

Two auth methods, tried in order:

1. **GitHub Actions OIDC** — automatic when `GITHUB_ACTIONS=true` and OIDC endpoint vars are set. Audience: `orun`.
2. **`ORUN_TOKEN`** — static API token stored as a repository secret.

## Intent configuration

Instead of passing `--remote-state` on every command, configure it in `intent.yaml`:

```yaml
execution:
  state:
    mode: remote
    backendUrl: https://orun-backend.example.com
```

With this in place, `orun run`, `orun status`, and `orun logs` automatically use the backend.

## Monitoring

From any machine with access to the backend:

```bash
orun status --remote-state --backend-url https://… --exec-id gh-12345678-1-a1b2c3 --watch
orun logs --remote-state --backend-url https://… --exec-id gh-12345678-1-a1b2c3
```

## Related

- [Remote state flags in `orun run`](../cli/orun-run.md#remote-state-distributed-execution)
- [Remote state configuration](../reference/configuration.md#remote-state-configuration)
- [Environment variables](../reference/environment-variables.md)
