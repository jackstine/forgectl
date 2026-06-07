# Evaluation Report

**Round:** 1
**Batch:** 5
**Layer:** L2 State Machine

VERDICT: PASS

## Items Evaluated

### [1] advance.core — ui_implementing core state machine (spine + code loop)

**Files reviewed:** state/advance.go (lines 24-39 dispatch, 803-1056 UI handlers, 1445-1452 allLayersComplete), state/advance_test.go (lines 3009-3191), cross-checked state/types.go, state/config.go, state/output.go.

#### Test Results

- [PASS] ORIENT selects the first batch, transitions to IMPLEMENT, and resets eval_round/qa_round/e2e_round to 0.
  - `advanceUIFromOrient` (advance.go:830-877) mirrors the implementing ORIENT: prior-layer completeness check, `selectBatch` with `UIImplementing.Batch`, and on a non-empty batch sets `CurrentLayer`, increments `BatchNumber`, and creates a `UIBatchState{CurrentItemIndex:0, EvalRound:0, QARound:0, E2ERound:0}` then transitions to IMPLEMENT. Empty layers fall through to DONE. `TestAdvanceUIOrientSelectsBatchResetsCounters` asserts state IMPLEMENT, batch size 2, and all three counters == 0. Passes.

- [PASS] Code EVALUATE PASS at >= eval.min transitions to QA_TEST with qa_round 1; FAIL at eval.max force-accepts and still transitions to QA_TEST flagged code-force-accepted.
  - `advanceUIFromImplement` increments `EvalRound` on entry to EVALUATE (advance.go:913), and `advanceUIFromEvaluate` (918-948) records the eval against the pre-incremented `EvalRound`, computes `toQA = (PASS && EvalRound>=min) || (FAIL && EvalRound>=max)`, sets `CodeForceAccepted` only on the FAIL branch, increments `QARound` to 1, and transitions to `StateQATest`. Below-min PASS / below-max FAIL reset `CurrentItemIndex=0` and return to IMPLEMENT. `TestAdvanceUICodeEvaluateToQA` covers both the clean-PASS path (QA_TEST, qa_round 1, not force-accepted) and the three-FAIL force-accept path (QA_TEST, CodeForceAccepted true, qa_round 1). Passes.

- [PASS] After all three loops, COMMIT marks items passed when none force-accepted and DONE transitions to PHASE_SHIFT (ui_implementing -> planning) when interleaved and the planning queue is non-empty.
  - `advanceUIFromCommit` (950-984) gates on `--message` when commits enabled, computes `forced = CodeForceAccepted || QAForceAccepted || E2EForceAccepted`, marks every batch item `passed`/`failed` accordingly, archives via `archiveUIBatch`, then routes to DONE (all layers complete) or ORIENT. `advanceUIFromDone` (986-1000) routes to PHASE_SHIFT ui_implementing->planning when `Planning.Queue` is non-empty, to ui_implementing->implementing on the all-first plan queue, else "session complete." `TestAdvanceUICommitPassedAndDonePhaseShift` confirms passed-marking, batch archived (CurrentBatch nil, one LayerHistory entry), DONE, then PHASE_SHIFT ui_implementing->planning. `TestAdvanceUICommitFailedOnForceAccept` confirms `failed` when `E2EForceAccepted` is set. Passes.

#### Notes

- Build (`go build ./...`) and full test suite (`go test ./...`) both pass; the four named tests (`TestAdvanceUIOrientSelectsBatchResetsCounters`, `TestAdvanceUICodeEvaluateToQA`, `TestAdvanceUICommitPassedAndDonePhaseShift`, `TestAdvanceUICommitFailedOnForceAccept`) pass under `-v`.
- The QA_TEST / UI_REFINE / E2E_AUTHOR / E2E_VERIFY / E2E_REMEDIATE states correctly fall to the default "cannot advance from state %q in ui_implementing phase" error in `advanceUIImplementing` (advance.go:825-826). This is in-scope-correct: those belong to the advance.qa and advance.e2e items in this layer and are intentionally unhandled. The core spine correctly transitions INTO QA_TEST from EVALUATE.
- Round-counter increment-on-entry convention is consistent with the transition table (rows IMPLEMENT->EVALUATE "Increment eval_round" and EVALUATE->QA_TEST "Increment qa_round") and with the committed L1 output rendering: output.go:1265 renders `Round: batch.EvalRound/Eval.MaxRounds` and 1291 renders `batch.QARound/QA.MaxRounds` directly with no `+1` offset, matching the handler incrementing before the evaluator state is entered.
- `archiveUIBatch` (1023-1056) records all three counters (`EvalRounds`/`QARounds`/`E2ERounds`) and all three histories (`Evals`/`QAEvals`/`E2EEvals`) into `UILayerHistory`, then clears `CurrentBatch` — terminal status is decided only at COMMIT, after the loops, per invariant 11 and the Item passes Transitions table.
- `allLayersComplete` now takes only `*PlanJSON` (advance.go:1445); both callers (implementing at 609, UI at 978) pass `plan` alone. No stale callers with the dropped `*ImplementingState` param remain (grep-verified), and the implementing-phase suite still passes — no regression.
- `requireVerdict` (1005-1021) is a shared helper validating `--verdict`/`--eval-report` for the UI evaluator states, used by `advanceUIFromEvaluate` via `EvalModeFor(cfg.Eval, s.Config.General)`. Consistent with the implementing phase's inline checks.
- `--message` gate in `advanceUIFromImplement` correctly keys on `EvalRound == 0 && EnableCommits` (first round only), matching the rejection table.

## Summary

The core ui_implementing spine (ORIENT batch selection with three-counter reset, one-at-a-time IMPLEMENT, the code EVALUATE loop terminating into QA_TEST with force-accept flagging, COMMIT terminal marking with batch archival, and DONE phase-shift routing) is implemented per the spec transition table and the state-machine notes. All three acceptance criteria pass, the build and full test suite are green, and the round-tracking, commit-marking, archival, and allLayersComplete-signature changes are consistent with no regressions to the implementing phase. The deferred QA/e2e states correctly return the default error within this item's scope.
