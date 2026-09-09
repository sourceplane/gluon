# Spec: orun-baseline-tracking (BT — the binary's leg)

**The bootstrap is work, and the binary is how the flows and the agent reach
the task plane.** The cross-repo programme
[`cirrus/specs/epics/saas-baseline-tracking`](https://github.com/sourceplane/cirrus/tree/main/specs/epics/saas-baseline-tracking)
makes a baseline bootstrap author itself into the workspace's task plane —
one epic ("Infra baselining"), one milestone per phase, one task per landing,
every landing PR on an `orun/<KEY>-<phase>` branch so the platform's
observation drain folds each task to `done` from evidence alone. Three repos
carry it: **orun-cloud** owns the plane and the MCP contract (BT0, shipped);
**cirrus** owns the flows and the agent brief (BT1–BT6); **this repo** owns
the two hands the other two hold — the CLI the flows call in a headless
container, and the MCP the sandbox agent speaks. Both are behind.

## Status

| Field | Value |
|-------|-------|
| Status | **Draft (not started)** — BT-O1–BT-O4 below; the paired orun-cloud leg (BT0) shipped as [orun-cloud#1374](https://github.com/sourceplane/orun-cloud/pull/1374) |
| Cluster | **BT** — this repo's milestones are **BT-O1–BT-O4**; cirrus carries BT1–BT6, orun-cloud carries BT0 |
| Owner(s) | `cmd/orun/tasks.go` (`orun task`), `cmd/orun/pr.go` + `internal/provenance` (the pen), `internal/remotestate/tasks.go` (+ a new `epics.go`), `internal/platformmcp` (the native tool plane) + `specs/orun-cloud/vendored/` (the manifest vendor), `website/docs/cli/` |
| Target branch | `main` |
| Builds on | `orun-mcp` UM0–UM6 (the unified `orun mcp serve`, the vendored-manifest parity pattern) · v2.54.0's provenance pen (`orun pr open`, `internal/provenance`, byte-twinned with orun-cloud's `packages/db/src/provenance`) · `orun task create/attach/list/show/check` (the task plane's CLI half, TK) · orun-cloud `orun-tasks` E1–E5 / TV2 / W1–W3 (epics, milestones, membership, briefs, assignment) |
| Decisions locked | (1) **The CLI is the flows' path, not curl.** The baseline flows already require this binary (≥ v2.52.6) and run it for every other platform call; the task plane is reached the same way, and the flows bump their floor to the release that carries BT-O1/BT-O2 rather than hand-rolling REST. (2) **Epics and milestones nest under `orun task`.** v2.54.0 retired the `orun epic` top-level verb with the work plane and said "no replacement"; the successor plane's containers are `orun task epic …` / `orun task milestone …`, so the retirement stays true. (3) **The manifest is re-vendored, not forked.** `internal/platformmcp` serves what orun-cloud's `tool-manifest.json` says, whole: the plane is at 33 tools and this binary advertises 27, so BT-O3 implements the six missing task-plane tools natively and re-pins parity — no subset mechanism, no local additions. (4) **The pen grows flags, not a second pen.** `land-pr.sh` calls `orun pr open` for the branch, the trailer and the manifest; the flows keep owning merge and convergence. |

## Thesis

Two gaps, both in this binary, sit between the plane orun-cloud shipped and
the flows cirrus is about to write.

**The CLI can create a task but cannot say where it belongs.**
`orun task create` (`cmd/orun/tasks.go`) takes `--adopt`, `--derive`,
`--prefix`, `--title` and attaches `tasks/<KEY>.TaskContract.yaml` if one
exists for the *issued* key — which, for a minted key, cannot exist yet. The
wire type behind it (`remotestate.TaskCreateRequest`) carries four fields; the
plane's `CreateTaskRequest` carries nine, and the five it lacks — `brief`,
`epic`, `milestone`, `assignee`, and a contract in the same breath — are
exactly the ones a bootstrap needs. There is no way to create an epic or a
milestone from the CLI at all (`internal/remotestate` has `PushEpicDoc` and
`ListEpicDocs`, nothing that creates a container).

**The MCP the sandbox agent speaks has no task tools.** The bootstrap door
provisions a sandbox whose driver config always mounts `orun mcp serve`
(`internal/agent/mcp.go`), which composes the pen (`pr_open`) and the
platform plane. The platform plane embeds
`specs/orun-cloud/vendored/mcp-tool-manifest.json` — **27 tools**, the
post-teardown roster. orun-cloud's plane went 27 → 31 with `orun-tasks` M1/M2
(`task_list`, `task_get`, `policy_preview`, `task_create`) and 31 → 33 with
BT0 (`epic_create`, `milestone_create`). `parity_test.go` pins the roster to
the vendored file, so this is not drift: it is a vendor that was never
refreshed. An agent in a bootstrap sandbox today cannot list a task, let
alone lay out an epic.

The pen is the one piece already right. `orun pr open --task KEY --title T`
checks out `orun/<KEY>-<slug(title)>`, pushes, renders the
`<!-- orun:manifest … -->` block into the body and opens the PR with the
ambient token — `internal/provenance/pen.go`. What cirrus's `land-pr.sh`
needs from it is two flags (a fixed branch slug, and the epic for the
manifest), and to be told the PR number back, which `--json` already does.

## The shape

```
flows (cirrus, headless container or sandbox)          agent (sandbox, claude-code harness)
  orun task epic create   --slug infra-baselining  ┐      orun mcp serve ──► epic_create
  orun task milestone create --epic … --after …    ├──►   remotestate ◄──── milestone_create
  orun task create --epic … --milestone … --brief …│      (same wire)       task_create (epic, milestone, contract)
                   --contract flows/…/contract.yaml ┘                        task_get  (the rollup the agent reports)
  orun pr open --task BASE-3 --branch-slug 03-infrastructure --epic infra-baselining --json
```

- **`orun task create`** gains `--epic`, `--milestone`, `--brief`,
  `--assignee` (default unset; `me` allowed) and `--contract <file>`: an
  explicit contract document attached under the same idempotency attempt,
  independent of the key-named file convention (which stays for
  repo-authored contracts). Output names where the task landed.
- **`orun task epic create|show`** and **`orun task milestone create`**:
  the container writes (`POST …/tasks/epics`, `…/epics/{ref}/milestones`)
  and the rollup read (`GET …/tasks/epics/{ref}/rollup`). A taken slug is
  adopted, not suffixed (the 409 carries the holder), and said so.
- **`internal/platformmcp`** re-vendors the 33-tool manifest and implements
  the six task-plane tools natively over `remotestate` — the same
  argument-validation-then-wire pattern as `writes.go`, with `task_get`'s
  ref widening (`tsk_` | key | `epc_` | `EP-n` | `mls_` | slug) ported from
  the TS handler. Parity, checksum and doctor all re-pin.
- **`orun pr open`** gains `--branch-slug` (use this slug instead of
  slugifying the title), `--epic` (the manifest's `epic`), and
  `--body-file`. Nothing else in the pen changes; the trailer and manifest
  are rendered as today.

## Read order

1. This README.
2. [`implementation-plan.md`](./implementation-plan.md) — BT-O1–BT-O4 with
   files and "done when".
3. [`risks-and-open-questions.md`](./risks-and-open-questions.md).
4. The umbrella:
   [`cirrus/specs/epics/saas-baseline-tracking/README.md`](https://github.com/sourceplane/cirrus/tree/main/specs/epics/saas-baseline-tracking)
   (the shape of the tracked bootstrap, decisions, the cross-repo table).
5. The plane: orun-cloud `specs/epics/orun-tasks/` and
   `packages/mcp/src/tools/tasks.ts` (what BT0 shipped; the semantics
   BT-O3 mirrors).

## Milestones at a glance

| # | Ships | Unblocks (cirrus) |
|---|---|---|
| BT-O1 | `orun task create --epic --milestone --brief --assignee --contract`; `remotestate.TaskCreateRequest` widened; `AttachTaskContract` reused | BT1 (`track.sh ensure-task`) |
| BT-O2 | `orun task epic create|show`, `orun task milestone create`; `remotestate/epics.go` | BT1 (`ensure-epic`, `ensure-milestone`, `rollup`), BT3 |
| BT-O3 | Manifest re-vendored at 33; six task-plane tools native in `internal/platformmcp`; parity/CHECKSUM/doctor re-pinned; release note | BT5 (the agent's Step 1b over MCP) |
| BT-O4 | `orun pr open --branch-slug --epic --body-file` | BT2 (`land-pr.sh` on the task's branch) |
