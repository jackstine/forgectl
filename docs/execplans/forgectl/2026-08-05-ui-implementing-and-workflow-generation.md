# ExecPlan Archive — forgectl: ui_implementing Phase + Workflow Generation / Adversarial Evaluation Gauntlet

## Provenance
- **Domain:** forgectl
- **Workspace archived:** forgectl/.forge_workspace/
- **Archived on:** 2026-08-05
- **Session:** unrecoverable — no session-state file lives inside the workspace itself; the current `.forgectl/state/forgectl-state.json` (session `3efc2999-dce1-498d-b4aa-c26495b4edae`) belongs to a later, unrelated cycle ("Batch Auto-Commit Refinement & Planning Readiness Gate") that had not yet drafted a plan and never touched this workspace.
- **Last workspace activity:** 2026-06-30 21:04 (`implementation/IMPLEMENTATION_LOG.md`, final entry)

## What This Cycle Was Building

This workspace was never cleared after **two consecutive, unrelated planning+implementation cycles** ran back to back in the `forgectl` domain, plus a leftover fragment of a third, earlier cycle. All three are fully finished and already landed in git; nothing here was abandoned mid-flight, but the workspace itself was never closed out.

**Cycle A — `ui_implementing` phase implementation** (earliest, June 2026). Added a whole new forgectl phase: after code `implementing`, a plan whose plan-queue entry carries `kind: "ui"` routes into `ui_implementing`, which runs three sequential, independently-budgeted verification loops per batch — a code eval loop (reused from `implementing`), a QA placement loop driven through the Playwright MCP (`QA_TEST` ⟲ `UI_REFINE`), and an e2e loop that authors and verifies Playwright tests (`E2E_AUTHOR` → `E2E_VERIFY` ⟲ `E2E_REMEDIATE`). This required new phase/state constants, a `UIImplementingConfig` (app launch/URL, and per-loop eval config for code/qa/e2e), new `UIBatchState`/`UIImplementingState` types, a new `kind` field on plan-queue entries, a new `handoff` command for the sub-agent to register QA/e2e artifacts, and full state-machine wiring (`advance.go`) plus output rendering, docs, and diagrams.

**Cycle B — Workflow Generation + Adversarial Evaluation Gauntlet** (immediately after, June 30 2026). Added `forgectl generate-workflow <plan.json>`, which compiles a validated plan and resolved config into a self-contained Claude Code workflow script under `.claude/workflows/`. The generated script runs an "adversarial evaluation gauntlet": for each batch, a primary agent implements once, then a bounded, mutating evaluator loop (min/max rounds, baked from config) directly edits code against each item's `tests` criteria with no verdict word — convergence is decided solely by a git-based change-detector agent, not the evaluator's self-report. This required an embedded `GauntletEval` prompt, batch-computation/topological-sort logic, output-name derivation with collision handling, the full script renderer, and the `generate-workflow` CLI command.

**Fragment — a stale prior-cycle leftover.** The plan's own round-3 evaluator noted that `evals/round-1.md` (a PASS verdict for the *ui-batch-implementation* plan) was sitting in the workspace as an orphaned file with no relation to the workflow-generation plan then being evaluated, and that no `round-2.md` existed at all. That confusion is itself evidence the workspace had already gone uncleared for at least one cycle boundary before Cycle B even started.

## Plan Inventory

### Cycle A — `ui_implementing` phase implementation (from `implementation/IMPLEMENTATION_LOG.md`; `plan.json` for this cycle was already overwritten by Cycle B and is not recoverable from the workspace)

| Layer | Batch | Item(s) | Status | Rounds |
|-------|-------|---------|--------|--------|
| L0 Type & Config Foundations | 1/6 | types.constants, types.kind, evaluators.embed | passed | 1 |
| L0 Type & Config Foundations | 2/6 | types.config | passed | 1 |
| L0 Type & Config Foundations | 3/6 | config.toml, config.validate, types.uistate | passed | 1 |
| L1 Output & Eval-Context Rendering | 4/6 | output.advance, output.eval | passed | 1 |
| L2 State Machine | 5/6 | advance.core | passed | 1 |
| L2 State Machine | 6/6 | advance.qa | passed | 1 |
| L2 State Machine | 7/6 | advance.e2e | passed | 1 |
| L3 Commands & Phase-Shift Routing | 8/6 | advance.phaseshift, cmd.init, cmd.evalwiring, cmd.handoff | passed (round 1 FAIL → round 2 PASS) | 2 |
| L4 Derived Documentation | 9 | docs.schemas, docs.config, docs.diagrams | passed | 1 |

Note: the log's own batch/total labels are internally inconsistent ("Batch: 8/6" for the 8th of what the log elsewhere implies is 9 batches) — reproduced as written, not corrected.

### Cycle B — Workflow Generation / Adversarial Evaluation Gauntlet (from `implementation_plan/plan.json`, `context.domain: forgectl`, `context.module: forgectl`)

| Layer | Item | Status | Rounds | Files |
|-------|------|--------|--------|-------|
| L0 Evaluator Prompt | evaluators.gauntlet-prompt | passed | 1 | forgectl/evaluators/gauntlet-eval.md, forgectl/evaluators/evaluators.go |
| L1 Core Logic | workflow.batch-compute | passed | 1 | forgectl/cmd/generateworkflow.go |
| L1 Core Logic | workflow.output-name | passed | 1 | forgectl/cmd/generateworkflow.go |
| L2 Script Rendering | workflow.script-render | passed | 1 | forgectl/cmd/generateworkflow.go |
| L3 Command Wiring | cmd.generate-workflow | passed | 1 | forgectl/cmd/generateworkflow.go |

All 5 items terminal `passed`, 0 items pending or failed.

## What Landed in the Codebase

Both cycles are fully represented in `git log --oneline -- forgectl/`, in this order (oldest first):

**Cycle A (`ui_implementing`):**
- `81e54de` — Plan: forgectl ui_implementing phase implementation
- `86cc105` — L0 batch 1: phase/state constants, plan-queue kind, evaluator embeds
- `733892c` — L0 batch 2: UIImplementingConfig struct and defaults
- `a59a266` — L0 batch 3: TOML decode/merge, config validation, state types
- `bd93b17` — L1 batch 4: advance/status and QA/e2e eval-context rendering
- `521ad6b` — L2 batch 5: core state machine (spine + code loop)
- `6e01ab8` — L2 batch 6: QA loop transitions (QA_TEST / UI_REFINE)
- `655035a` — L2 batch 7: e2e loop transitions (E2E_AUTHOR/E2E_VERIFY/E2E_REMEDIATE)
- `bcded81` — L3 batch 8: commands & phase-shift routing
- `9b4eb29` — L4 batch 9: derived documentation (schemas / config / diagrams)

**Cycle B (workflow generation / gauntlet):**
- `c55f08a` — plan for workflows
- `3471596` — L0 batch 1: embed gauntlet adversarial evaluator prompt
- `8387be3` — L1 script rendering batch: batch computation and output-name derivation (script L1)
- `a0c3d87` — L3 command wiring: forgectl generate-workflow
- `130747a` — L2 script rendering: emit the self-contained gauntlet workflow
- `e5a7b2a` — docs: close out workflow-generation implementation log
- `16fe8b8` — feat: add set-commit-hashes command
- `a4a3bac` — docs: document set-commit-hashes across specs, schemas, and diagrams
- `8777350` — feat: surface spec Read: git show command in eval output
- `4971609` — test: add CLI-level integration test for eval Read: git show command
- `e12d5c3` — docs: add round-3 eval report for workflow-generation plan

No discrepancy found: every item marked `passed` in both cycles has corresponding commits, and the implementation log's own "Session complete" entries and the round-1/round-2 eval PASS verdicts corroborate full completion. One item the round-1 batch-8 eval flagged as **FAIL** (`cmd.evalwiring` — the `ui_implementing eval` rejection message named only the state, not the phase) was fixed and re-evaluated **PASS** in round 2, per `batch-8-round-2.md`; the fix is reflected in the shipped `cmd/eval.go`.

One follow-up flagged in the implementation log as explicitly deferred, not done: the `generate-workflow` CLI-diagram edits (`docs/diagrams/04-cli-commands.txt`, `cli-commands.html`) were authored but deliberately left **uncommitted**, because they were being concurrently edited by an in-flight `set-commit-hashes` change and staging them wholesale risked sweeping in another actor's work. Worth checking whether those diagram edits ever landed separately — they are not visible in the `forgectl/` commit list above under an obviously matching message.

## Evaluation History

- **Cycle A plan-level eval** (`evals/round-1.md`, round 1): PASS, 11/11 dimensions, 120/120 spec requirements covered, 0 deficiencies. Evaluated the `ui-batch-implementation.md` / `session-init.md` / `phase-transitions.md` plan.
- **Cycle A batch evals** (`evals/batch-5..7-round-1.md`): all PASS round 1, no deficiencies.
- **Cycle A batch 8 eval, round 1** (`evals/batch-8-round-1.md`): **FAIL** — `cmd.evalwiring`'s `ui_implementing` eval-rejection message named only the current state, not the phase, contrary to the spec's rejection-row wording and the acceptance criterion. The accompanying test under-specified the criterion and didn't catch it.
- **Cycle A batch 8 eval, round 2** (`evals/batch-8-round-2.md`): PASS — the message was fixed to name both phase and state, and the test was strengthened to assert both.
- **Cycle A batch 9 eval** (`evals/batch-9-round-1.md`): PASS, docs/schemas/diagrams all verified against the implemented Go types.
- **Cycle B plan-level eval** (`evals/round-3.md`, round 3 — rounds 1 and 2 are not present in the workspace and could not be recovered): **FAIL**, 5/11 dimensions, ~76/~85 spec requirements covered, 8 distinct deficiencies across Configuration, Observability, Invariants, Edge Cases, Testing Criteria, and Dependencies & Format. Recurring theme: the plan tested the **primary** agent's baked model/type and the `enable_commits=false` negative case thoroughly, but several **evaluator-side** and **positive (`enable_commits=true`)** counterparts, DEBUG-level logging, and the spec-mandated deterministic-stand-in execution tests for loop invariants (primary-runs-once, min/max round bounds, force-accept-at-ceiling) were originally unwritten. It also flagged a path-convention inconsistency: `items[].files` entries omitted the `forgectl/` domain prefix that `items[].specs` entries correctly included.
- **Cycle B batch evals** (`evals/batch-1..4-round-1.md`): all PASS round 1. Notably, `batch-3-round-1.md` (workflow.script-render) shows the round-3 plan-eval deficiencies were in fact addressed by implementation time — the batch eval explicitly confirms tests exist for the evaluator's distinct `eval.model`/`eval.type`, the `enable_commits=true` positive case, DEBUG logging, and executed-stand-in tests for force-accept-at-ceiling and the min-rounds floor. The plan-eval FAIL and the implementation's actual PASS are not contradictory — the gap the round-3 report found in the plan document was closed during drafting/implementation, not left unresolved.

## Abandoned Unfinished

None. Every item in both cycles reached terminal `passed`. The one item that failed an eval round (`cmd.evalwiring`, Cycle A batch 8) was corrected and passed on the very next round. The one plan-level FAIL (Cycle B round 3) predates implementation and was addressed by the time batches were built and evaluated.

The only genuinely loose end is procedural, not a code gap: the two CLI-diagram files noted above as deliberately left uncommitted at the end of Cycle B, to avoid clobbering concurrent work. Whether those edits were ever re-applied is unverified from this workspace alone — check `docs/diagrams/04-cli-commands.txt` / `cli-commands.html` against the current `cmd/generateworkflow.go` before assuming they're in sync.

## Notes and Reasoning Worth Keeping

- **Decision (Cycle A):** `ui_implementing` gets its own state structs (`UIBatchState`, `UIImplementingState`, etc.) rather than extending the shipped `implementing` phase's `BatchState` — explicitly to avoid JSON schema drift/regression risk on the already-shipped phase. Confirmed with the operator at the time (`notes/state-types.md`).
- **Round-counter convention (Cycle A):** all three UI loops (`eval_round`/`qa_round`/`e2e_round`) increment **on entry** to their evaluator state, and the evaluator records against the already-incremented value — deliberately different from the `implementing` phase's code-eval loop, which increments *inside* the evaluator. Both conventions are internally consistent within their own phase; this is called out explicitly in `evals/batch-6-round-1.md` and `batch-7-round-1.md` notes so a future reader doesn't "fix" the UI phase to match `implementing`'s convention.
- **Gauntlet convergence authority (Cycle B):** the change-detector agent's git-based observation is the *sole* signal for convergence — the evaluator's own return value is only ever used to log an ERROR on `null`, never consulted for changed/clean status. This is structurally enforced (`changed_N` is derived exclusively from `classifyDetector`), not just a convention, per `notes/workflow-rendering.md` and confirmed in `evals/batch-3-round-1.md`.
- **`eval.count` is intentionally ignored by the workflow renderer** (Cycle B) — the generated script always spawns exactly one evaluator `agent()` call per round regardless of the configured count; this was a deliberate scope decision, tested explicitly (`TestRender_CountIgnored`).
- **Supporting change discovered mid-implementation (Cycle B):** the plan called for the primary's model/type to be baked distinctly from the evaluator's, but `state/types.go` had no field for it — `implementing.implement` (an `AgentConfig`) was added to `state/types.go`, `state/config.go`, and both `default-config.toml` copies as a necessary supporting change, documented in `docs/configurations.md`.
- **`PLAN_FORMAT.md` is missing from the repository** — the round-3 evaluator (Cycle B) needed it as the plan-format authority per the evaluator instructions and `evaluators/plan-eval.md`, but found it had been deleted in an earlier commit and never replaced; it fell back to `docs/schemas/plan-json.md` instead. Worth restoring or repointing the evaluator instructions if this hasn't been fixed since.
- **The workspace path-numbering confusion documented at the top of this archive** (round-1.md flagged as an unrelated stale leftover by the round-3 evaluator, no round-2.md ever existing) is itself the clearest in-workspace evidence that this exact close-out was overdue well before this session.

## Resuming This Work

Both cycles this archive covers are complete — there is nothing to resume for `ui_implementing` or workflow-generation/the gauntlet themselves. For a future cycle touching this domain:

- Verify whether `docs/diagrams/04-cli-commands.txt` and `cli-commands.html` reflect `generate-workflow` — they were edited but deliberately left uncommitted at the end of Cycle B pending reconciliation with a concurrent `set-commit-hashes` change.
- Verify `forgectl/evaluators/plan-eval.md`'s reference to `PLAN_FORMAT.md` still points at a nonexistent file; if so, either restore the file or repoint the reference to `docs/schemas/plan-json.md`, which is what evaluators have been falling back to.
- If evaluating plans in this domain again, do not rely on any `evals/round-*.md` or `evals/batch-*.md` file surviving between cycles — this close-out removes all of them. Future plan-level evaluators should not need to reason about "stale" files from a prior cycle the way `round-3.md`'s evaluator had to.
