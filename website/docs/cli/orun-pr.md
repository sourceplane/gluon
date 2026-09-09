---
title: orun pr
---

`orun pr` is the **provenance pen**: it opens a task-carrying PR with its
lineage written in — the branch on the grammar (`orun/<task-key>-<slug>`),
the `Task: <KEY>` trailer, and a machine-readable manifest block in the
body — and preflights those rules locally before the PR exists.

```bash
orun pr open  --task KEY [--title …] [--base main] [--draft] \
              [--branch-slug SLUG] [--epic REF] [--body-file FILE|-] [--json]
orun pr check [task-key] [--base main]
orun pr githooks install     # the commit-msg hook that stamps the trailer
```

## `open`

A PR opens **for** a task. The pen checks out `orun/<KEY>-<slug>` from the
current HEAD when the current branch is not already on the grammar for
that key, pushes it, renders the manifest into the body, and opens the PR
with the ambient GitHub credential (`GITHUB_TOKEN`, `GH_TOKEN`, or `gh
auth`). Without a credential it still prepares everything — branch pushed,
body rendered — and prints the compare URL plus the body to paste.

- `--branch-slug` uses the slug half **verbatim** instead of slugifying
  the title, so a flow that names its landings (`03-infrastructure`) keeps
  the branch it documents. It must already be in the grammar's alphabet
  (`[a-z0-9-]`); anything else is refused before git is touched.
- `--epic` writes the epic (`epc_…` or its slug) into the manifest, beside
  the task, the session and the skill revisions it ran under.
- `--body-file` supplies the prose half of the body (`-` reads stdin); the
  manifest block is appended after it.
- `--json` returns `branch`, `pushed`, `opened`, `url`, `number` (the PR
  number, for a caller that merges next) or `compareUrl`, and `body`.

```bash
orun pr open --task BASE-3 --branch-slug 03-infrastructure --epic infra-baselining \
  --title "phase(03-infrastructure): d1, kv, db-migrate" --body-file - --json <<'EOF2'
Automated phase landing.
EOF2
```

The body's manifest then reads
`<!-- orun:manifest {"version":1,"task":"BASE-3","epic":"infra-baselining"} -->`,
and the pushes, PR and merge on `orun/BASE-3-03-infrastructure` bind to
`BASE-3` on the platform.

## `check`

The same rules, locally, before the PR exists: the branch parses to a task
key, the commits carry the trailer, the manifest (when present) is
well-formed and names the same task. Exit 1 on errors; `--json` for the
findings.
