# Notes: ui_implementing State Machine (advance.go)

Backs the L2 state-machine items and the L3 phase-shift item. Files relative to `forgectl/` domain root. Canonical behavior: `specs/ui-batch-implementation.md` §State Machine / Transition Table / Round Tracking.

## Dispatch — `state/advance.go`

- Top-level `Advance` switch at `advance.go:24-38`: add `case PhaseUIImplementing: return advanceUIImplementing(s, in, dir)`.
- Model `advanceUIImplementing` on `advanceImplementing` (`advance.go:582-631`) — a `switch s.State`.

## Reusable helpers (all in advance.go, reuse unchanged)

`loadPlan`/`savePlan` (`1064-1106`), `selectBatch` (`1108-1127`), `itemUnblocked`/`findItem`/`setItemPasses`/`incrementItemRounds`/`allLayerItemsTerminal`/`allLayersComplete` (`1129-1189`), `archiveBatch` pattern (`1191-1219`), `checkEvalReportExists` (`1221-1225`). These operate via `CurrentPlanFile`; point them at `s.UIImplementing`.

## Three loops, three independent budgets

Counters live on `UIBatchState`: `EvalRound`/`QARound`/`E2ERound`; histories `Evals`/`QAEvals`/`E2EEvals`. Reset all three to 0 when a batch is selected at ORIENT. The verdict→round pattern from `advanceImplFromEvaluate` (`advance.go:729-799`) is replicated three times, each reading its own loop's `min_rounds`/`max_rounds` from `s.Config.UIImplementing.{Eval,QA,E2E}`.

### Core spine (item: advance-core) — ORIENT / IMPLEMENT / code EVALUATE / COMMIT / DONE
- ORIENT: `selectBatch`; create `UIBatchState{Items, CurrentItemIndex:0, EvalRound:0, QARound:0, E2ERound:0}`; → IMPLEMENT. Layer/queue handling mirrors implementing ORIENT.
- IMPLEMENT: identical to implementing (`advanceImplFromImplement`, `688-727`): mark item `done`; last item → EVALUATE and increment item `rounds`.
- code EVALUATE: PASS & `EvalRound>=eval.min` → **QA_TEST** (increment `QARound` to 1); PASS<min or FAIL<max → IMPLEMENT; FAIL & `>=eval.max` → set `CodeForceAccepted`, → QA_TEST. (Transition table rows EVALUATE→*.)
- COMMIT: terminal for the batch. Mark items `passed` iff no loop force-accepted; else `failed` (invariant 11). `archiveBatch` into `UILayerHistory`. → ORIENT (more) or DONE.
- DONE: interleaved (`plan_all_before_implementing:false`, planning queue non-empty) → PHASE_SHIFT (ui_implementing→planning); all-first → PHASE_SHIFT domain boundary; else session complete. Mirror `advanceImplementing` DONE handling; DONE summary adds qa/e2e round totals (ui-batch-implementation.md DONE output).

### QA loop (item: advance-qa-loop) — QA_TEST ⟲ UI_REFINE
- QA_TEST: requires `--verdict` (+`--eval-report` in qa report mode). Increment `QARound`, append `QAEvals`. PASS & `>=qa.min` → **E2E_AUTHOR** (require step-list file present, else reject naming the path — invariant 6). PASS<min or FAIL<max → UI_REFINE. FAIL & `>=qa.max` → set `QAForceAccepted`, → E2E_AUTHOR (still require step list).
- UI_REFINE: always → QA_TEST, increment `QARound`.
- **Step-list boundary check**: on BOTH QA exit paths (PASS@min and force-accept@max), the file `<domain>/.forgectl_workspace/ui_plan/qa/batch-N-steps.json` must exist; if absent, reject (exit 1), state stays QA_TEST. The scaffold reads it only to confirm presence and count `scenarios` (invariant 14); a zero-scenario list is valid (WARN) and flows to a vacuous e2e pass.

### e2e loop (item: advance-e2e-loop) — E2E_AUTHOR → E2E_VERIFY ⟲ E2E_REMEDIATE
- E2E_AUTHOR: bridging state, no verdict; always → E2E_VERIFY, increment `E2ERound` to 1. (Zero-scenario step list still advances.)
- E2E_VERIFY: requires `--verdict` (+`--eval-report` in e2e report mode). Increment `E2ERound`, append `E2EEvals`. PASS & `>=e2e.min` → COMMIT (mark terminal). PASS<min or FAIL<max → E2E_REMEDIATE. FAIL & `>=e2e.max` → set `E2EForceAccepted`, → COMMIT.
- E2E_REMEDIATE: always → E2E_VERIFY, increment `E2ERound`.

## Phase-shift routing (item: phase-shift-routing) — `advancePhaseShift` (`advance.go:803-973`)

- planning→implementation (existing block ~`872-911`): read the active plan's `Kind`. `ui` → set `s.Phase = PhaseUIImplementing`, validate the four required UI keys (empty → print errors, stay PHASE_SHIFT), init `s.UIImplementing` via `NewUIImplementingState()`, add `passes:"pending"`/`rounds:0` to items (same loop as implementing), write plan.json. `code`/absent → existing implementing path.
- implementing/ui_implementing → next implementation phase domain boundary (all-first, ~`921`): pull next plan, route by its `Kind` the same way.
- `kind` is carried unchanged from plan-queue → planning → implementation plan queue (phase-transitions.md invariant 11).

## Tests (`state/advance_test.go`)

Add `newUIImplementingState` helper + `advanceUIImplTo{QATest,E2EVerify,Commit}` helpers (mirror `newImplementingState`/`advanceImplTo*`). Cover the full transition table from ui-batch-implementation.md §Testing Criteria: code-eval PASS/force→QA; QA FAIL→UI_REFINE; UI_REFINE→QA (round++); QA PASS→E2E_AUTHOR; QA PASS step-list-absent rejected; E2E_AUTHOR→E2E_VERIFY; E2E FAIL→E2E_REMEDIATE; E2E_REMEDIATE→E2E_VERIFY (round++); E2E PASS→COMMIT passed; E2E force→COMMIT failed; force-in-any-loop→failed at COMMIT; empty step list vacuous pass; independent round budgets; QA force-accept step-list-absent rejected; three-gates-in-order; DONE per plan_all_before_implementing.
