# Implementation Log — Batch Auto-Commit Refinement & Planning Readiness Gate

Domain: `forgectl`. Plan: `forgectl/.forge_workspace/implementation_plan/plan.json`.

---

## Entries

### 2026-08-08 — L0 Foundations: git.message-synthesis + state.workspace-inspection
- **Errors:** None
- **All Tests Pass:** Yes
- **Batch:** 1/8
- **Eval Rounds:** 1
- **Notes:** Added `ItemCommitMessage` / `BatchCommitMessage` / `AppendSuppliedMessage` to `state/git.go` (AutoCommit's signature unchanged — synthesis happens in callers) and the new `state/readiness.go` workspace inspection (`DomainPath`, `DomainWorkspacePath`, `InspectDomainWorkspace`). Both are pure helpers with no call sites yet; L1 wires them in.

### 2026-08-08 — L1: impl.per-item-commit + impl.inline-terminal-commit
- **Errors:** None
- **All Tests Pass:** Yes (one pre-existing integration failure remains, see Notes)
- **Batch:** 2/5
- **Eval Rounds:** 1
- **Notes:** IMPLEMENT now commits on every round with a synthesized message (`--message` optional, appended); the terminal EVALUATE commits the batch inline via `terminateImplBatch` and goes straight to ORIENT/DONE; COMMIT is now a pure no-op reachable only with `enable_commits: false`. Three stale tests that encoded the removed `--message` requirement were replaced (one of which had been passing only because its fixture lacked a git repo). `TestSkillContractSubcommands/preflight` still fails — the `preflight` command is item `cmd.preflight` in L2, not yet implemented.

### 2026-08-08 — L1: impl.direct-mode-reentry + state.readiness-gate
- **Errors:** None
- **All Tests Pass:** Yes (`preflight` integration gap still open, see Notes)
- **Batch:** 3/5
- **Eval Rounds:** 1
- **Notes:** `reenterImplEvaluationLoop` routes non-terminal verdicts by eval mode — `direct` re-enters EVALUATE and carries the round increment (the missing increment is what would make the loop spin forever), `report`/`conversational` keep the IMPLEMENT re-entry. `state/readiness.go` gained `ReadinessVerdict`, `QueueDomains`, `EvaluateReadiness`, and `Render()` producing the spec's exact READY/BLOCKED text. `TestRefineActionDirectMode` lost its implementing-phase assertion — that IMPLEMENT re-entry no longer exists under direct mode.

### 2026-08-08 — L2: ui.per-item-commit + ui.direct-mode-reentry + cmd.preflight + gate.enforce-init + gate.enforce-phase-shift
- **Errors:** None
- **All Tests Pass:** Yes (unit and `-tags=integration`; the `TestSkillContractSubcommands/preflight` gap noted in batches 2–3 is closed by `cmd.preflight`)
- **Batch:** 4/5
- **Eval Rounds:** 1
- **Notes:** ui_implementing reached parity with implementing on both commit points and on direct-mode re-entry. `forgectl preflight [--from]` landed as the non-mutating readiness query — it deliberately avoids `state.Scaffold` (which would create `.forgectl/` and a default config) and falls back to cwd + defaults instead, so the read-only invariant holds even in an uninitialized project. The gate is now enforced at all three cold-start planning entries: `init --phase planning` (prints the verdict, returns before the state file is written) and the two phase shifts (`generate_planning_queue→planning` and the `specifying→generate_planning_queue --from` skip), which return a new `state.ReadinessError` carrying the verdict — its `Error()` is `Render()` verbatim, so one message reaches the operator from every entry point. The load-bearing negative is that the intra-session domain boundaries (`planning→planning`, `implementing/ui_implementing→planning`) stay ungated; gating them would false-block every multi-domain continuation, since the scaffold's own output fills each finished domain's workspace. Four tests pin that exemption, including a foreign mid-cycle write into a not-yet-planned domain.
