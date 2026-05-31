# cmd package — eval routing + advance warnings

## cmd/eval.go — routing (eval.go:32-45)

Current `switch`:
```go
case s.Phase == state.PhaseSpecifying && s.State == state.StateReconcileEval:
    return state.PrintReconcileEvalOutput(...)
case s.Phase == state.PhaseSpecifying && s.State == state.StateCrossReferenceEval:
    return state.PrintCrossRefEvalOutput(...)
case s.Phase == state.PhasePlanning || s.Phase == state.PhaseImplementing:
    return state.PrintEvalOutput(...)
case s.Phase == state.PhaseReverseEngineering && s.State == state.StateReconcileEval:
    return state.PrintReverseEngineeringEvalOutput(...)
case s.Phase == state.PhaseReverseEngineering:
    return fmt.Errorf("...only available during RECONCILE_EVAL.")
default:
    return fmt.Errorf("eval is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
```
**Gap:** `PhaseSpecifying && StateEvaluate` (the per-batch spec eval) has no case and hits the
default error. Add:
```go
case s.Phase == state.PhaseSpecifying && s.State == state.StateEvaluate:
    return state.PrintSpecEvalOutput(cmd.OutOrStdout(), s, projectRoot)
```
(Place it before the planning/implementing case. `PrintSpecEvalOutput` is the new function added
in the state-package output item.)

## cmd/advance.go — printAdvanceWarnings (advance.go:232-267)

Two warnings to align to spec wording and `eval_mode`:

1. **--eval-report** (advance.go:239-253): currently switches on per-phase `EnableEvalOutput`
   and prints `warning: ignoring --eval-report: eval output is not enabled`. Change to resolve the
   current phase's `eval_mode` (via the state-package `EvalModeFor` helper). When a report is
   provided in a non-report mode, print exactly:
   `--eval-report is ignored, --eval-report is only used in report mode`.
   (Note: this `cmd`-layer warning overlaps the `state/advance.go` warning. Keep them consistent —
   prefer emitting the spec-worded warning from the state layer's transition path which the specs
   describe as the canonical behavior, and ensure the cmd-layer pre-check does not double-print.)

2. **--message** (advance.go:255-266): currently `warning: ignoring --message: commits are not enabled`.
   Spec wording (plan-production.md, batch-implementation.md, spec-reconciliation.md):
   `--message is ignored, commits are not enabled`. Update the string; keep the state gate
   (COMPLETE/ACCEPT/IMPLEMENT/COMMIT) and the `!EnableCommits` condition.

`validateAdvanceFlags` (advance.go:211-230) — no change needed; `--verdict` gating stays.

## Testing
`cmd/commands_test.go` drives commands end-to-end. Add cases:
- `forgectl eval` in specifying EVALUATE prints the SPEC EVALUATION block (no longer errors).
- `advance --eval-report x` in a `direct`/`conversational` session prints the spec-worded warning.
- `advance --message x` with commits disabled prints `--message is ignored, commits are not enabled`.
