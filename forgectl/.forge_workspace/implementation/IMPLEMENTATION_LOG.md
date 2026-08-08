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

### 2026-08-08 — L2: ui.inline-terminal-commit + gate.logging
- **Errors:** None
- **All Tests Pass:** Yes (unit and `-tags=integration`)
- **Batch:** 5/5
- **Eval Rounds:** 1
- **Notes:** ui_implementing now commits the batch inline at terminal E2E_VERIFY, closing the last commit-flow gap. The subtle part is item marking: the ui phase can only decide passed/failed after all three loops have run, so that marking used to live solely in COMMIT. It is now `markUIBatchTerminal`, called by whichever path runs — inline when `enable_commits` is true, COMMIT when it is false — so it happens exactly once either way, never twice with different force-accept state in between. `finishUIBatch` takes the already-marked plan rather than reloading, so the completeness check sees this batch's own statuses. `advanceUIFromCommit` lost its `--message` requirement and its `in` parameter along with it. Gate logging landed as `LogReadinessGate` in `state/readiness.go`: INFO with the inspected count and verdict, ERROR with the dirty-domain list when blocked, DEBUG per domain — all through the existing Logger/LogEntry with the level carried in `Detail`, since `LogEntry` has no level field and the spec forbids a new sink. Wired at all three cold-start entries and deliberately not at `preflight`, which can be run any number of times from outside a session; the session-less no-op needs no special case because `NewLogger` already disables itself on an empty session id.

### 2026-08-08 — L3: output.commit-flow
- **Errors:** One pre-existing regression found and fixed (see Notes)
- **All Tests Pass:** Yes (unit and `-tags=integration`)
- **Batch:** 6/6
- **Eval Rounds:** 1
- **Notes:** Output now matches the revised transitions. Both COMMIT renderings lost their `enable_commits: true` arm — the state is unreachable with commits on, so a `--message` instruction there could only mislead; each phase's commits-disabled wording is unchanged (implementing keeps "Commit your changes before continuing.", ui keeps "Advance to continue."). The inline batch commit is reported through a new transient `ForgeState.InlineBatchCommit` (`json:"-"`, so it never survives a save/load and can't claim a stale commit on the next command); `PrintAdvanceOutput` emits it before the state block, since with COMMIT skipped that line is the only acknowledgement the commit happened. An empty `AutoCommit` hash renders as an explicit "nothing left to commit" rather than silence. `writeDirectReentryNote` adds the previously-unreachable EVALUATE-from-EVALUATE `Note:` line for both phases, matching the spec's worked example verbatim and distinguishing a looping PASS ("Minimum rounds not yet met") from a FAIL.
  **Regression fixed:** batch 3 moved the implementing `EvalRound++` from the EVALUATE exit to the IMPLEMENT→EVALUATE entry (matching the ui convention) but left three `+1` offsets behind in `output.go`. The first EVALUATE of a batch was rendering "Round: 2/3" and `implEvalReportPath`/`forgectl eval` were naming `batch-N-round-2.md` — so the path the operator passes to `--eval-report` had drifted from the round actually being evaluated. Offsets removed at `implEvalReportPath`, the EVALUATE `Round:` line, and `printImplEval`; `TestEvalReportPathMatchesRenderedRound` pins the two together across a direct-mode re-entry.
