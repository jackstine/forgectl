# Notes — `evaluators/` package + eval rendering

## Embedding (existing)

`evaluators/evaluators.go` embeds each `.md` as a string constant via
`//go:embed` (one `var` per file, e.g. `var ReconcileEval string`). The directory
already contains `reconcile-eval.md` as a **~427-byte stub** — it is already
embedded; the work is to COMPLETE its content, not to add embedding wiring.

## reconcile-eval.md — complete the 7-dimension checklist

reverse-engineering.md RECONCILE_EVAL requires the evaluator to instruct the
sub-agents to:
1. Read each spec file listed for the domain in full.
2. Read any spec referenced in `depends_on` that is not in the list.
3. Evaluate against the **7 dimensions**:
   1. completeness
   2. depends_on validity (every reference points to an existing spec)
   3. integration points symmetry (A ↔ B)
   4. depends_on ↔ integration points correspondence
   5. naming consistency across references
   6. no circular dependencies in the depends_on graph
   7. topic of concern (single sentence, no "and", describes an activity)
4. Write a report with PASS/FAIL per dimension and an overall verdict.

Follow the structure/voice of plan-eval.md and cross-reference-eval.md. The eval
report path convention is `{domain}/specs/.eval/reconciliation-r{round}.md`.

NOTE: the specifying phase already has a reconciliation flow that uses
`reconcile-eval.md` + `PrintReconcileEvalOutput`. Completing this file benefits
both; ensure the content is phrased to cover the per-domain reverse-engineering
reconciliation (spec list + depends_on for one domain).

## Wiring `forgectl eval` for the RE phase RECONCILE_EVAL

Rendering lives in `state/output.go` (there is no `state/eval.go`). Either extend
`PrintReconcileEvalOutput` to handle the RE phase, or add a sibling
`PrintReverseEngineeringEvalOutput(w, s)`. It must populate the prompt with:
- the specs created/updated for the current domain (filter `Queue` by
  `Domains[DomainIndex-1]`), each with its `depends_on`,
- the current `ReconcileRound`,
- the report path `{domain}/specs/.eval/reconciliation-r{round}.md`.

`cmd/eval.go` (runEval) currently switches on phase/state and errors otherwise.
Add a case: `s.Phase == PhaseReverseEngineering && s.State == StateReconcileEval`
→ call the RE eval renderer. Keep `eval` blocked in all other RE states with the
spec's message "forgectl eval is only available during RECONCILE_EVAL." `eval`
stays read-only (no log entry).