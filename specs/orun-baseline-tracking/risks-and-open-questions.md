# orun-baseline-tracking — Risks & Open Questions

Live register for this repo's leg. The umbrella register is
[`cirrus/specs/epics/saas-baseline-tracking/risks-and-open-questions.md`](https://github.com/sourceplane/cirrus/tree/main/specs/epics/saas-baseline-tracking).

## ⛔ Human-input gates (do NOT auto-pick)

| Item | Blocking decision | Unblock signal |
|------|-------------------|----------------|
| **Release + floor** | cirrus's flows pin `orun ≥ v2.52.6`; BT1/BT2 there need the release carrying BT-O1, BT-O2 and BT-O4, and the bootstrap sandbox (`ORUN_INSTALL_URL` → `install.sh`, always latest) picks it up on the next boot. Somebody names the release and cirrus bumps its floor in `BOOTSTRAP.md`, `flows/AGENT-PROMPT.md` and `flows/agent/BASELINE-TASK.md` in the same week. | A tagged release whose notes name `orun task epic`; the cirrus floor bump merged. |
| **Bootstrapper tool policy** | BT-O3 puts three writes on the sandbox's MCP. The `bootstrapper` agent type rides the allow lane for shell and edits; whether `epic_create` / `milestone_create` / `task_create` join that lane (they are policy-gated and audited server-side, and the door lends admin) or stay in an ask lane that nobody is present to answer is a policy call, not a code one. | The decision recorded in `internal/agent`'s policy table with a test naming the three. |

## Open design questions

| Item | Question | Current lean |
|------|----------|--------------|
| Command shape | `orun task epic create` vs a resurrected `orun epic`. | Nested. v2.54.0 said the top-level verb is gone with no replacement; keeping that sentence true costs one word per invocation. |
| Manifest vendor cadence | The plane's roster moved twice (27 → 31 → 33) without a re-vendor here. Should CI in this repo diff the vendored file against orun-cloud `main` and fail on drift, the way OC0 does for the state contract? | Yes, as part of BT-O3: the OC0 CI-diff job gains the manifest path. A stale vendor is exactly what left the sandbox MCP without task tools. |
| `task_get` docs inline | The TS `task_get` inlines an epic's spec docs under a byte budget. Porting that keeps parity of *behaviour*, which the conformance fixtures check; skipping it keeps BT-O3 small. | Port it — the agent's "epic status and its specs in one call" is the read the bootstrap brief leans on. |
| `--contract` location | A contract file in the baseline (cirrus `flows/phases/NN/task-contract.yaml`) vs the plane's repo-authored `tasks/<KEY>.TaskContract.yaml` in the product. | Baseline-owned via `--contract`; the product's own work can adopt the key-named convention later without the bootstrap having planted files it does not own. |

## Standing risks

- **Twin grammar.** `internal/provenance.BranchName` / `TaskKeyOfBranch`
  are byte-twinned with orun-cloud `packages/db/src/provenance` and
  conformance-pinned by `fixtures/`. BT-O4 adds a flag that *chooses* the
  slug; it must not touch the grammar.
- **Idempotency of `--contract`.** A retried `task create` replays the
  create under its key; the attach must ride `<key>:contract` so the retry
  re-attaches an identical body (dedup by hash, TK-J) instead of erroring.
- **Roster budget in orun-cloud.** 33 is pinned there by tests; a future
  plane change re-opens this repo's vendor. The CI diff above is the guard.
