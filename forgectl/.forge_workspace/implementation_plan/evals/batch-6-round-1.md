# Evaluation Report

**Round:** 1
**Batch:** 6
**Layer:** L2 State Machine

VERDICT: PASS

## Items Evaluated

### [advance.qa] QA loop transitions (QA_TEST / UI_REFINE)

**Files reviewed:**
- `forgectl/state/advance.go` (`advanceUIFromQATest`, `advanceUIFromUIRefine`, the `advanceUIImplementing` switch, `requireVerdict`, and the `advanceUIFromEvaluate` QA-entry increment)
- `forgectl/state/output.go` (`qaStepListPath`, `countQAScenarios`)
- `forgectl/state/advance_test.go` (`TestAdvanceUIQAFailToRefineAndBack`, `TestAdvanceUIQAPassToE2EAuthor`, `TestAdvanceUIQAStepListAbsentRejected`, helpers `writeQAStepList`, `uiAtQATest`)

#### Test Results

- [PASS] **QA_TEST FAIL below qa.max_rounds → UI_REFINE; UI_REFINE → QA_TEST with qa_round incremented.**
  `advanceUIFromQATest`: with `MinRounds:1, MaxRounds:3` and `QARound==1`, a FAIL is below max, so `toE2E` is false; the eval is appended and `s.State = StateUIRefine`. `advanceUIFromUIRefine` does `batch.QARound++` then `s.State = StateQATest`. `TestAdvanceUIQAFailToRefineAndBack` confirms QA_TEST entry has `qa_round==1`, FAIL → UI_REFINE, advance → QA_TEST with `qa_round==2`. Passes.

- [PASS] **QA_TEST PASS at >= qa.min_rounds with step-list present → E2E_AUTHOR.**
  With `QARound==1 >= MinRounds:1` and a present step list, `countQAScenarios` returns `exists==true`, the eval is appended, and `s.State = StateE2EAuthor`. `TestAdvanceUIQAPassToE2EAuthor` writes a 3-scenario step list, advances PASS, asserts E2E_AUTHOR and exactly one recorded QA eval. Passes.

- [PASS] **Advancing out of QA_TEST toward E2E_AUTHOR (PASS@min or FAIL force-accept@max) with the step-list file absent is rejected naming the expected path; state remains QA_TEST.**
  On both exit paths the step-list presence check runs first: `count, exists := countQAScenarios(...)`; if `!exists`, it returns an error that interpolates `qaStepListPath(...)` and returns before appending to `QAEvals` or setting `QAForceAccepted`. `TestAdvanceUIQAStepListAbsentRejected` covers PASS@min (error names the path, stays QA_TEST) and FAIL force-accept@max=1 (rejected, stays QA_TEST, `QAForceAccepted` remains false). Passes.

#### Verification of spec contract and notes

- **--verdict / --eval-report requirement:** `advanceUIFromQATest` calls `requireVerdict(in, EvalModeFor(cfg.QA, ...))`, which rejects missing/invalid `--verdict`, requires `--eval-report` in report mode, and verifies the report file exists. Matches the Rejection table and QA_TEST flag row.
- **QARound increment on ENTRY, not in QA_TEST:** `advanceUIFromEvaluate` increments `batch.QARound` (to 1) when transitioning into QA_TEST; `advanceUIFromUIRefine` increments on re-entry; `advanceUIFromQATest` itself records its eval against `batch.QARound` directly and never increments. Consistent with the transition table (EVALUATE/UI_REFINE carry the increment), the file header comment, and the L1 output that renders `qa_round` directly.
- **Transition arithmetic:** `toE2E = (PASS && QARound>=qa.min) || (FAIL && QARound>=qa.max)`. PASS→E2E_AUTHOR; FAIL@max→E2E_AUTHOR with `QAForceAccepted=true`; otherwise (PASS<min or FAIL<max)→UI_REFINE. Matches the four QA_TEST rows of the transition table.
- **Step-list invariant (invariant 6) on BOTH exits:** the presence check guards the single `toE2E` branch, so it applies to PASS@min and FAIL force-accept@max alike. Reject leaves clean state (no eval appended, no force flag) — ordering confirmed in source and asserted by the test.
- **Zero scenarios valid:** a present file with `0` scenarios returns `(0, true)`; the code logs `WARN: ... has 0 scenarios ...` to stderr and proceeds. Matches invariant 14 / the zero-scenario edge case.
- **UI_REFINE always → QA_TEST with QARound++:** `advanceUIFromUIRefine` confirmed.
- **E2E states unhandled:** E2E_AUTHOR/E2E_VERIFY/E2E_REMEDIATE fall to the switch default (error). Per the batch scope these belong to advance.e2e; their absence is expected and not flagged.

#### Notes
- `go build ./...` exits 0; `go test ./...` passes (`forgectl/cmd`, `forgectl/evaluators`, `forgectl/state` all ok). The three named tests pass individually under `-v`.
- `countQAScenarios` treats an unparseable-but-present file as `(0, true)` — a present-but-malformed step list passes the presence gate and warns as zero scenarios. This is consistent with invariant 14 ("reads only to confirm presence and count scenarios") and is not a defect for this batch.

## Summary

The QA placement loop (QA_TEST ⟲ UI_REFINE) is implemented per spec: verdict/report validation via `requireVerdict`, QARound incremented on loop entry (EVALUATE and UI_REFINE) so QA_TEST records directly, correct PASS/FAIL × min/max routing to E2E_AUTHOR vs UI_REFINE, and the invariant-6 step-list presence check enforced on both exits toward E2E_AUTHOR — rejecting (exit 1, named path) with clean state before any eval/force-flag mutation, while zero scenarios is a valid WARN. All three acceptance tests pass; full build and test suite are green.
