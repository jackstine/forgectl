# Evaluation Report

**Round:** 1
**Batch:** 7
**Layer:** L2 State Machine

VERDICT: PASS

## Items Evaluated

### [advance.e2e] E2E loop transitions (E2E_AUTHOR / E2E_VERIFY / E2E_REMEDIATE)

**Files reviewed:**
- `forgectl/state/advance.go` (advanceUIFromE2EAuthor, advanceUIFromE2EVerify, advanceUIFromE2ERemediate, advanceUIFromCommit, the E2E_* switch cases)
- `forgectl/state/advance_test.go` (uiAtE2EAuthor helper; TestAdvanceUIE2EAuthorVerifyRemediate, TestAdvanceUIE2EPassAndForceToCommit, TestAdvanceUIE2EZeroScenarioVacuousAndIndependentBudgets)
- `forgectl/state/types.go` (UIE2EConfig embedding EvalConfig)
- `forgectl/state/output.go` (L1 round-counter rendering for the three loops)

#### Test Results

- [PASS] `[functional]` E2E_AUTHOR advances to E2E_VERIFY (e2e_round 1); E2E_VERIFY FAIL below e2e.max transitions to E2E_REMEDIATE; E2E_REMEDIATE returns to E2E_VERIFY with e2e_round incremented.
  - `advanceUIFromE2EAuthor` (advance.go:1013) carries no verdict, increments `E2ERound` (0→1) on entry, and always sets `StateE2EVerify`. `advanceUIFromE2EVerify` (advance.go:1022) with FAIL and `E2ERound < MaxRounds` falls to the else branch → `StateE2ERemediate`. `advanceUIFromE2ERemediate` (advance.go:1051) increments `E2ERound` (1→2) and returns to `StateE2EVerify`. Matches transition table rows E2E_AUTHOR→E2E_VERIFY, E2E_VERIFY FAIL<max→E2E_REMEDIATE, E2E_REMEDIATE→E2E_VERIFY. Verified by TestAdvanceUIE2EAuthorVerifyRemediate.

- [PASS] `[functional]` E2E_VERIFY PASS at >= e2e.min_rounds with no loop force-accepted transitions to COMMIT with items passed; FAIL at e2e.max transitions to COMMIT with items failed.
  - `toCommit` is `(PASS && E2ERound>=MinRounds) || (FAIL && E2ERound>=MaxRounds)`. On the PASS path `E2EForceAccepted` stays false; `advanceUIFromCommit` (advance.go:1058) computes `forced = Code||QA||E2E ForceAccepted` and marks items `passed`. On the FAIL@max path it sets `batch.E2EForceAccepted = true`, so COMMIT marks items `failed`. Verified by TestAdvanceUIE2EPassAndForceToCommit (asserts both COMMIT state and the resulting plan.json item `passes` values, and the force-accept flag).

- [PASS] `[edge_case]` A zero-scenario step list passes the e2e loop vacuously (E2E_AUTHOR → E2E_VERIFY PASS at min rounds), and the three loops count/bound their rounds independently given different per-loop max_rounds.
  - The zero-scenario QA step list flows through QA→E2E (with the documented WARN), and `advanceUIFromE2EAuthor` advances to E2E_VERIFY regardless of scenario count; an E2E_VERIFY PASS at min reaches COMMIT. Independent budgets confirmed: with `e2e.max=1` the e2e loop force-accepts after one FAIL while `EvalRound` and `QARound` remain 1 each. Verified by TestAdvanceUIE2EZeroScenarioVacuousAndIndependentBudgets; the expected `WARN: ... has 0 scenarios` line is printed during the run.

#### Steps Check

1. E2E_AUTHOR no verdict, always → E2E_VERIFY, E2ERound to 1, zero-scenario advances — implemented (advance.go:1013-1020), test-covered.
2. E2E_VERIFY require --verdict (+--eval-report in report mode via `requireVerdict(in, EvalModeFor(cfg.E2E.EvalConfig, ...))`); records E2EEval against the on-entry-incremented E2ERound; PASS>=min→COMMIT, below-min/FAIL<max→E2E_REMEDIATE, FAIL>=max→E2EForceAccepted+COMMIT — implemented (advance.go:1022-1049).
3. E2E_REMEDIATE always → E2E_VERIFY, E2ERound++ — implemented (advance.go:1051-1056).
4. COMMIT terminal marking observes E2EForceAccepted alongside CodeForceAccepted/QAForceAccepted — implemented (advance.go:1072).
5. Tests for the empty-step-list vacuous pass and independent-round-budget cases — present.

#### Notes
- The switch in `advanceUIImplementing` (advance.go:825-830) routes all three E2E_* states to their handlers; build confirms no missing case.
- `UIE2EConfig` embeds `EvalConfig`, so `cfg.E2E.MinRounds`/`MaxRounds` are the promoted embedded fields and `cfg.E2E.EvalConfig` is passed to `EvalModeFor` for the mode. Round budget reads from the e2e loop's own config, satisfying invariant 8.
- Increment-on-entry convention is uniform across all three loops: code (advanceUIFromImplement:923 increments EvalRound entering EVALUATE), qa (advanceUIFromEvaluate:950 / advanceUIFromUIRefine:1008 increment QARound entering QA_TEST), e2e (advanceUIFromE2EAuthor:1017 / advanceUIFromE2ERemediate:1053 increment E2ERound entering E2E_VERIFY). Each evaluator records its EvalRecord against the already-incremented counter.
- L1 rendering is consistent: EVALUATE/QA_TEST/E2E_VERIFY/E2E_REMEDIATE all print `batch.{Eval,QA,E2E}Round` directly with no `+1` offset (output.go:1265, 1291/1329, 1391, 1414), matching the pre-incremented value. (The implementing phase's EVALUATE uses `+1` because that phase increments inside the evaluator rather than on entry; the UI phase is internally self-consistent.)

#### Full-Chain Sanity Check
With all three loops now implemented, a batch flows: ORIENT (resets EvalRound/QARound/E2ERound to 0, selects batch) → IMPLEMENT(1..n) → EVALUATE (code loop, EvalRound) → QA_TEST ⟲ UI_REFINE (qa loop, QARound; step-list invariant enforced on both exits) → E2E_AUTHOR → E2E_VERIFY ⟲ E2E_REMEDIATE (e2e loop, E2ERound) → COMMIT (items terminal: passed iff no force-accept, else failed) → ORIENT/DONE. The three force-accept flags (CodeForceAccepted, QAForceAccepted, E2EForceAccepted) are each set on their respective FAIL@max exit and all three are read at COMMIT (invariant 11). Three round budgets are independent and reset together at ORIENT.

## Summary

Batch 7 completes the L2 State Machine layer. The e2e bridging state and verification loop are implemented exactly per the transition table and edge cases: E2E_AUTHOR is a verdictless bridge that always advances (including the zero-scenario case), E2E_VERIFY records its verdict against an on-entry-incremented counter and terminates to COMMIT on PASS>=min or FAIL>=max (setting E2EForceAccepted on the latter), and E2E_REMEDIATE loops back. COMMIT marks items failed if any of the three force-accept flags is set. The three required tests plus helper are present and pass; `go build ./...` and `go test ./...` both succeed (the WARN line about 0 scenarios is the expected vacuous-pass path). The round-counter increment-on-entry convention is uniform across all three loops and matches the L1 output rendering.
