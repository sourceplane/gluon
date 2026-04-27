---
title: Execution runtime
---

After planning, `gluon` switches from compiler behavior to runtime behavior. The runtime reads the immutable plan, orders jobs, persists state, and delegates each step to an executor backend.

## Runtime responsibilities

- verify the plan checksum against saved state
- compute topological execution order
- print dry-run or live execution summaries
- persist step and job state when execution is enabled
- delegate each step to the selected executor

## Executor backends

### Local executor

Runs `run:` steps through `sh -c` on the host. It is the simplest backend and the best default for local development.

### Docker executor

Ensures the image is available, mounts the workspace at `/workspace`, and executes inside a container. It uses `job.runsOn` as the image source.

### GitHub Actions executor

Uses the internal GitHub Actions engine to support `use:` steps, workflow command files, post-step handling, and GitHub Actions environment semantics.

## Phase boundaries

Execution stays linear but explicit:

1. `pre`
2. `main`
3. `post`

Within each phase, `order` and declaration order determine the exact step sequence.

## Failure behavior

- `failFast` is read from the plan execution block
- step-level `retry` values are honored
- `onFailure: continue` lets later steps run after a non-fatal failure
- job state is persisted only when execution is enabled

That keeps dry-run side-effect free while still letting execute mode resume safely.

## Concurrent execution

The runtime executes ready jobs in parallel up to the configured concurrency
limit (`--concurrency` or `execution.concurrency` in the plan). Jobs become
ready as soon as their `dependsOn` predecessors finish.

### Per-job workspace isolation

Running multiple jobs against the same source tree at the same time creates
collisions on shared mutable state — for example, two jobs running
`pnpm install` in the same monorepo will trample each other's `node_modules`,
and two jobs that build the same site will race on the same output directory.

To eliminate these collisions, gluon stages an isolated copy of the source
workspace for each concurrent job and re-points `WorkspaceDir`, `WorkDir`, and
`GITHUB_WORKSPACE` at the staged copy. Each job then operates on its own
private tree with its own `node_modules`, build outputs, and lockfiles.

Isolation is enabled in three modes (`--isolation` flag or
`execution.isolation` in the plan):

| Mode | Behaviour |
| --- | --- |
| `auto` _(default)_ | Stage when `concurrency > 1`; share the source workspace otherwise |
| `workspace` | Always stage, even with `concurrency = 1` |
| `none` | Never stage — jobs share the source workspace (legacy behaviour) |

Staged workspaces live under
`<workspace>/.gluon/runs/<execID>/<jobID>/work/` and are removed when the job
succeeds. Failed jobs keep their staged workspace so you can `cd` in and
reproduce the failure. The `--keep-workspaces` flag retains every staged
workspace, even on success.

### Per-job HOME, RUNNER_TEMP, and GITHUB_ENV files

In addition to the workspace, the GitHub Actions engine gives every job its
own HOME directory, `RUNNER_TEMP`, and `GITHUB_ENV` / `GITHUB_OUTPUT` /
`GITHUB_PATH` / `GITHUB_STATE` / `GITHUB_STEP_SUMMARY` files. This is what
prevents popular setup actions (`pnpm/action-setup`, `actions/setup-node`,
`actions/setup-go`, …) from clobbering each other's caches when invoked in
parallel.

### Caching strategy

Three caches are shared across concurrent jobs by design — none of them is a
collision risk because each is content-addressed and writes are coordinated:

| Cache | Location | Coordination |
| --- | --- | --- |
| Action download cache | `~/.cache/gluon/actions/<repo>/<sha>/` | Lock file (`<sha>.lock`) and atomic rename of a `.ready` marker |
| Tool cache (`RUNNER_TOOL_CACHE`) | `~/.cache/gluon/tool-cache/` | Atomic per-version completion markers from `actions/tool-cache` |
| Docker images | host Docker daemon | Per-image mutex inside the engine |

The action download cache is keyed by the resolved commit SHA, so
`pnpm/action-setup@v4` is fetched once per SHA and shared by every concurrent
job. The on-disk lock prevents two jobs from extracting the same tarball
simultaneously.

### Filesystem performance

Workspace staging uses the fastest mechanism the host filesystem supports, in
order:

1. **APFS `clonefile(2)`** on macOS — true copy-on-write at the inode level.
   Staging a 100k-file monorepo completes in milliseconds and uses zero extra
   bytes until a job actually writes to a file.
2. **`ioctl(FICLONE)`** reflinks on Linux btrfs / xfs (with `reflink=1`) /
   bcachefs.
3. **Hard links** for read-only source files when COW is unavailable
   (e.g. ext4). Files inside `.git/` are never hardlinked because git
   rewrites object files in place.
4. **Plain copy** as the universal fallback.

Generated, volatile, and VCS-internal directories are excluded by default so
jobs rebuild them fresh in their own staged tree:
`node_modules`, `.pnpm-store`, `.yarn/cache`, `.next`, `.turbo`, `.cache`,
`.parcel-cache`, `.docusaurus`, `.vite`, `.svelte-kit`, `.astro`, `.nuxt`,
`.output`, `dist`, `build`, `out`, `target`, `coverage`, `.terraform`,
`.venv`, `venv`, `__pycache__`, and `.gluon` itself.