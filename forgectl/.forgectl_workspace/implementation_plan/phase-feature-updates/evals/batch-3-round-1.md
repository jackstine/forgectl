# Evaluation Report

**Round:** 1
**Batch:** 3/4
**Layer:** L2 Behavior keyed on eval_mode

VERDICT: PASS

## Items Evaluated

### [evalmode.advance-gate] Gate --eval-report on eval_mode in advance transitions

**Files reviewed:** forgectl/state/advance.go

#### Test Results

- [PASS] In report mode, advancing from EVALUATE with --verdict but no --eval-report errors with exit code 1.
  - All four eval-entry handlers (specifying EVALUATE/CROSS_REFERENCE_EVAL/RECONCILE_EVAL, planning EVALUATE, implementing EVALUATE) now gate on `EvalModeFor(...) == "report" && in.EvalReport == ""` and return `--eval-report is required in <STATE> state`. Verified by `TestEvaluateReportModeRequiresEvalReport`, `TestReconcileEvalRequiresEvalReport`.
- [PASS] In report mode, --eval-report pointing to a non-existent file errors naming the path.
  - `checkEvalReportExists` is still invoked for any supplied path (planning made conditional on non-empty, cross-ref/reconcile retained). Error is `eval report %q does not exist`. Verified by `TestEvaluateReportModeRejectsMissingFile`, `TestSpecifyingEvalReportMustExist`.
- [PASS] In direct/conversational mode, --eval-report is accepted-but-ignored and the advance proceeds.
  - The state layer no longer prints any ignore warning (the old `os.Stderr` warns were removed); non-report modes do not require the flag and proceed. The single user-facing warning now lives only in the cmd layer. Verified by `TestEvaluateConversationalModeIgnoresEvalReport`.

#### Notes
The always-required `--verdict` check is preserved unchanged. Planning EVALUATE was correctly migrated from unconditional `--eval-report` requirement to report-mode-gated, with `checkEvalReportExists` now guarded by `if in.EvalReport != ""`.

### [evalmode.cmd-warnings] Align cmd-layer advance warnings to spec wording

**Files reviewed:** forgectl/cmd/advance.go

#### Test Results

- [PASS] advance --message with enable_commits=false at COMMIT prints `--message is ignored, commits are not enabled` and proceeds.
  - String changed to `warning: --message is ignored, commits are not enabled`; no instruction to enable commits (matches batch-implementation invariant 13 and spec-lifecycle edge cases). Verified by `TestAdvanceWarnMessageIgnoredWhenCommitsDisabled`.
- [PASS] advance --eval-report in CROSS_REFERENCE_EVAL in a non-report mode prints the spec-worded ignore warning.
  - `StateCrossReferenceEval` was added to `evalStates`; warning is `warning: --eval-report is ignored, --eval-report is only used in report mode`, resolved via `EvalModeFor`. Verified by `TestAdvanceWarnEvalReportIgnoredInCrossRefNonReport`.
- [PASS] The --eval-report ignore warning is printed exactly once (not duplicated by both cmd and state layers).
  - State layer emissions removed; `TestAdvanceEvalReportWarningPrintedExactlyOnce` counts occurrences across both cmd buffer and captured stderr and asserts exactly 1.

#### Notes
Warning text retains the existing `warning: ` prefix convention; the spec-mandated wording is the substring following it, matching the spec tables character-for-character.

### [evalmode.spec-eval-output] Add specifying EVALUATE eval output (PrintSpecEvalOutput)

**Files reviewed:** forgectl/state/output.go, forgectl/cmd/eval.go

#### Test Results

- [PASS] `forgectl eval` in specifying EVALUATE prints `=== SPEC EVALUATION ROUND` with evaluators.SpecEval contents and the batch spec list.
  - `PrintSpecEvalOutput` prints the round header, `Domain:`/`Batch:` lines, `--- EVALUATOR INSTRUCTIONS ---` embedding `evaluators.SpecEval`, and `--- SPECS TO EVALUATE ---` listing filename (basename), Topic, and full File path. Routing case added in cmd/eval.go before the planning/implementing case. Verified by `TestEvalOutputSpecEvaluateContainsEvaluatorPrompt`.
- [PASS] `forgectl eval` outside an eval state still errors naming the current state (no regression to the default branch).
  - Guard returns an error naming `s.State`; verified by `TestEvalOutputSpecEvaluateRejectsWrongState` and `TestEvalOutputOutsideValidStatesReturnsError`.

#### Notes
Per-mode REPORT OUTPUT / PREVIOUS EVALUATIONS sections are correctly deferred to evalmode.eval-output-modes (documented in the function doc comment); their absence is not penalized.

### [evalmode.action-evaluate] Per-mode action/status text for all eval-entry states

**Files reviewed:** forgectl/state/output.go

#### Test Results

- [PASS] Planning EVALUATE report mode ends with `--eval-report <path>`; conversational ends with `--verdict PASS|FAIL` and no report path.
  - Verified by `TestEvalEntryActionReportMode` / `TestEvalEntryActionConversationalMode`. Planning uses `Sub-agent runs: forgectl eval` (matches plan-production.md L191), distinct from specifying/implementing `The sub-agent should run: forgectl eval`.
- [PASS] Implementing EVALUATE direct mode renders `evaluate and correct` plus the staged-files note.
  - `evaluate and correct the batch.` + `Batch files have been staged. Sub-agent makes corrections directly.`; no `--eval-report`. Verified by `TestEvalEntryActionDirectMode`.
- [PASS] CROSS_REFERENCE_EVAL and RECONCILE_EVAL render the matching per-mode action text.
  - Both states migrated from single-form to `writeEvalEntryAction`. Cross-ref direct uses `evaluate and correct cross-references.`; reconcile uses `evaluate cross-domain reconciliation.`. Verified by `TestEvalEntryActionEdgeCases` (cross-ref direct, reconcile conversational, plus legacy enable_eval_output→report back-compat).
- [PASS] status output for an eval-entry state matches the same per-mode variant as advance.
  - `PrintStatus` delegates the current-state Action block to `PrintAdvanceOutput`, so status and advance share the identical per-mode rendering by construction.

#### Notes
Implementing report tail correctly orders flags as `--eval-report <path> --verdict PASS|FAIL` (batch-implementation.md L208), while specifying/planning/reconcile/cross-ref use `--verdict PASS|FAIL --eval-report <path>`. Verified against all four spec files.

### [evalmode.action-refine] Per-mode REFINE and IMPLEMENT re-entry action text

**Files reviewed:** forgectl/state/output.go

#### Test Results

- [PASS] Planning REFINE in direct mode renders `Review unstaged changes from the evaluator (git diff).`
  - Verified by `TestRefineActionDirectMode`.
- [PASS] Specifying REFINE in conversational mode renders `Make corrections based off communication with the evaluator.`
  - Verified by `TestRefineActionConversationalMode`; conversational also emits the `Implement any corrections as needed.` follow-up and retains Format/Process/Scoping + `advance to continue evaluation.` per spec-lifecycle.md L253-261.
- [PASS] Implementing IMPLEMENT after-eval re-entry in report mode renders `Study the eval file`.
  - Verified by `TestRefineActionReportMode`.

#### Notes
The shared `writeRefineBody` helper preserves the `Apply "fresh" eyes and a tightened lens ...` lines on all three arms. Specifying REFINE keeps Format/Process/Scoping in report+conversational but drops them in direct (and switches the tail to `advance to continue.`), matching spec-lifecycle.md L237 vs L217/L260. Planning REFINE correctly prints the `Minimum evaluation rounds not met.` preamble on PASS-below-min and omits it on FAIL (plan-production.md L283 vs L234), with proper continuation indentation. Implementing re-entry 3-way branch matches batch-implementation.md L134/161/188.

## Summary

All five items are fully implemented and spec-compliant. The full suite (`go build` + `go test ./...`) passes; `go vet` is clean. Per-mode action/refine wording matches the spec example blocks character-for-character across specifying, planning, and implementing, including the implementing-specific report-tail flag ordering and the per-state suffix differences. The `--eval-report` ignore warning is emitted exactly once (cmd layer only), and `EvalModeFor` correctly resolves explicit `eval_mode`, legacy `enable_eval_output` (→ report), and the conversational default. Deferred sub-items (eval-output-modes REPORT OUTPUT/PREVIOUS EVALUATIONS per mode) are appropriately out of scope and documented as such. Pre-existing header lines were intentionally left unchanged per the batch scope and were not penalized.
