# orun-baseline-tracking — Implementation Plan

Status: Normative for this repo's BT milestones (BT-O1–BT-O4). The umbrella
plan, with the cirrus flows (BT1–BT6) and orun-cloud's BT0, is
[`cirrus/specs/epics/saas-baseline-tracking/implementation-plan.md`](https://github.com/sourceplane/cirrus/tree/main/specs/epics/saas-baseline-tracking).
Ordered so cirrus's BT1 can start the moment BT-O1+BT-O2 ship in a release;
BT-O3 and BT-O4 are independent of each other and of BT-O1/O2.

## BT-O1 — `orun task create` says where the task belongs

**Scope**

- `internal/remotestate/tasks.go`: `TaskCreateRequest` gains
  `Brief string`, `Epic string`, `Milestone string`, `Assignee string`
  (all `omitempty`) — the plane's `CreateTaskRequest` (orun-cloud
  `packages/contracts/src/tasks.ts`, W3). `PublicTask` gains `Brief`,
  `Assignee`, and the `Epic {ID, Slug, Key}` / `Milestone {ID, Name}`
  members so `show`/`list` can render membership.
- `cmd/orun/tasks.go` `newTaskCreateCommand`: flags `--epic` (epc_… | EP-n |
  slug), `--milestone` (mls_…), `--brief`, `--assignee` (`me` or a subject
  ref), `--contract <file>`. `--contract` reads a `TaskContract.yaml` from
  the given path (the same `taskfile` loader `attach` uses), seals it, and
  attaches it under the create's idempotency key + `:contract` — so a
  bootstrap can keep its contract templates in the baseline and never needs
  a `tasks/<KEY>.…` file in the product. Without `--contract` the existing
  key-named lookup runs as today. Text output adds
  `in milestone <name>` / `under epic <slug>` and, when a contract attached
  with an empty gate list, `merge alone finishes it`.
- `orun task list` gains `--epic` / `--milestone` (the plane's `ListTasksFilter`).

**Done when** `go test ./cmd/orun/... ./internal/remotestate/...` is green
with new cases for the wire shape (a request with all five fields, verbatim
JSON) and the `--contract` path; `website/docs/cli/orun-task.md` (new — the
task group has no page today) documents the flags; the version floor named
in the cirrus flows can be bumped to the release carrying this.

## BT-O2 — `orun task epic` and `orun task milestone`

**Scope**

- `internal/remotestate/epics.go` (new; `epicdocs.go` stays as is):
  `CreateEpic(ctx, org, EpicCreateRequest{Name, Slug, Description,
  TargetDate, Owner})` → `PublicEpic`; `GetEpic(ctx, org, ref)` →
  `{Epic, Milestones, TaskCount}`; `GetEpicRollup(ctx, org, ref)`;
  `CreateMilestone(ctx, org, epicRef, MilestoneCreateRequest{Name,
  TargetDate, ExitCriteria, After *string})`. Routes as pinned by
  orun-cloud `packages/sdk/src/tasks.ts`. A 409 on create is decoded to
  `*APIError` with `Details.existing` preserved.
- `cmd/orun/tasks.go`: a nested `epic` group — `create` (`--name`, `--slug`,
  `--description`, `--target-date`, `--owner`; on 409 prints
  `epic <slug> already exists (<key>) — reusing it` and exits 0 with the
  existing epic in `--json`), `show <ref>` (mirror status beside the
  derived rollup, milestones with their rungs) — and a `milestone` group
  with `create --epic <ref> --name … [--after <mls_…>|--first]
  [--exit-criteria …]…`. The v2.54.0 note ("`orun epic` is gone, no
  replacement") stays true: these are task-plane containers under
  `orun task`, not the work plane's epic verbs.

**Done when** the two groups round-trip against a recorded wire fixture
(`internal/remotestate/testdata/`), `orun task epic create` twice with the
same slug is idempotent by observation (second run: `reusing`), and the CLI
doc page covers both groups.

## BT-O3 — The MCP serves the task plane

**Scope**

- Re-vendor: copy orun-cloud `packages/mcp/tool-manifest.json` (33 tools,
  24 reads + 9 writes) to `specs/orun-cloud/vendored/mcp-tool-manifest.json`,
  update `CHECKSUM`, copy into `internal/platformmcp/` (the embed).
  `parity_test.go` and `manifest.go`'s header comment move from 27 to 33.
- `internal/platformmcp/tasks.go` (new): the six natives, dispatched from
  `provider.go`'s `call` (reads) and `writes.go`'s `callWrite` (writes):
  - `task_list` (`epic`, `milestone`, `assignee` filters) →
    `remotestate.ListTasks` with the filter.
  - `task_get` — the widened ref: `mls_` → milestone + epic + container
    contract; `epc_` / `EP-n` / anything that is not a task key or `tsk_` →
    epic rollup + container contract + the epic's docs (inline under the
    same 16 KiB / 48 KiB budgets as the TS handler, pointer otherwise);
    else task + contract + verdict. The summary strings are the TS ones
    verbatim (the conformance fixture in orun-cloud `tests/mcp` is the
    oracle).
  - `policy_preview` → `secrets_list` at the environment + the same
    env-gate-then-glob intersection (`internal/scope` already has the
    matcher the enforcement pass uses).
  - `task_create` → BT-O1's request, then `AttachTaskContract` when
    `contract` is given (`gatesDefined: true`, hash sealed locally).
  - `epic_create` → BT-O2's `CreateEpic`; 409 with a holder → `existed: true`.
  - `milestone_create` → `CreateMilestone`.
- `orun mcp doctor` lists 33 and names the six as task-plane tools.
- Release note: the sandbox agent's MCP now carries the task plane; the
  bootstrap brief (cirrus BT5) depends on this release.

**Done when** `go test ./internal/platformmcp/...` re-pins parity at 33,
`orun mcp serve` → `tools/list` shows the six, and a sandbox session
(`orun agent serve`) can call `epic_create` under the bootstrapper's
tool policy — which must allow the three writes for the `bootstrapper`
agent type (`internal/agent` policy tables) and deny them for the
implementer type's ask lane as it does every other write.

## BT-O4 — The pen takes the flows' branch

**Scope**

- `cmd/orun/pr.go` `open`: `--branch-slug <slug>` (validated
  `[a-z0-9-]+`; the branch becomes `orun/<KEY>-<slug>` instead of
  `Slugify(title)`), `--epic <slug|epc_…>` (sets `Manifest.Epic`),
  `--body-file <path>` (the prose half of the body; `-` for stdin).
  `internal/provenance.OpenRequest` gains `BranchSlug`; `BranchName` is
  unchanged (the twin in orun-cloud must not move).
- `--json` output already carries `branch`, `url`, `opened`; add `number`
  so `land-pr.sh` can merge without parsing the URL.
- When the PR refuses (no `pull_requests` grant), the branch is pushed and
  the compare URL returned as today — the flows' direct-merge fallback
  keeps the `orun/…` branch, so `branch_seen` still binds.

**Done when** `orun pr open --task BASE-3 --branch-slug 03-infrastructure
--epic infra-baselining --title "phase(03-infrastructure): d1, kv,
db-migrate" --json` (against the pen's test double) pushes
`orun/BASE-3-03-infrastructure`, and the body's manifest reads
`{"version":1,"task":"BASE-3","epic":"infra-baselining"}`; `orun pr check`
passes on that branch.
