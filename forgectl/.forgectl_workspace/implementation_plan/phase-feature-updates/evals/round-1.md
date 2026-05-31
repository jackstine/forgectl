# Evaluation Report — Round 1

## Verdict: FAIL

## Summary
- Dimensions assessed: 11/11
- The plan is structurally sound, layered correctly, and the dependency DAG is valid. The
  config foundation, validation, back-compat resolution helper, advance gating, cmd-layer
  warning alignment, the `PrintSpecEvalOutput` routing gap, and the per-mode REPORT
  OUTPUT / PREVIOUS EVALUATIONS work are all well decomposed and accurate against the code.
- Two genuine coverage gaps cause the FAIL: (1) per-mode EVALUATE *advance/status action
  text* for CROSS_REFERENCE_EVAL and RECONCILE_EVAL is not covered by any item (only the
  specifying/planning/implementing EVALUATE state is scoped); (2) the cmd-layer warning item
  does not account for the fact that the current cmd `evalStates` map omits
  CROSS_REFERENCE_EVAL, so the spec-worded `--eval-report` warning will not fire there in
  any layer.
- No false positives found: every item targets genuinely-missing work. Nothing in the plan
  rebuilds already-implemented functionality (self_review, state constants, plan.json
  validation, add-queue-item/set-roots, user_review branching are all correctly left out).

## Dimension Results

### 1. Behavior — FAIL
The transition tables themselves (eval-mode-independent) are already implemented and correctly
out of scope. The mode-dependent behavior is mostly covered:
- EVALUATE → REFINE/ACCEPT gating on `--eval-report` per mode: covered by `evalmode.advance-gate`
  for EVALUATE, CROSS_REFERENCE_EVAL, RECONCILE_EVAL. Good.
- REFINE / IMPLEMENT re-entry action text per mode: covered by `evalmode.action-refine`.
- EVALUATE entry action text per mode (specifying/planning/implementing): covered by
  `evalmode.action-evaluate`.

Gap (blocker): The **advance action text for CROSS_REFERENCE_EVAL and RECONCILE_EVAL** also
varies by `eval_mode` per the specs:
- spec-lifecycle.md "Entering CROSS_REFERENCE_EVAL" has three rendered variants
  (report / direct / conversational) at lines 299–340 — the direct arm adds
  "evaluate and correct cross-references" + "Spec files have been staged. Sub-agent makes
  corrections directly." and drops `--eval-report` from the trailing advance line; report keeps it.
- spec-reconciliation.md "Entering RECONCILE_EVAL" has three rendered variants at lines 65–97
  with the same report/direct/conversational distinction (direct: "evaluate and correct the
  reconciliation" + staged-files note; report ends with `--eval-report <path>`).

The current code renders both states single-form (output.go ~193-209 for CROSS_REFERENCE_EVAL;
the RECONCILE_EVAL renderer likewise). `evalmode.action-evaluate` step 1 explicitly scopes only
"specifying ~98-117, planning ~490-503, implementing ~840-870" — i.e. the EVALUATE state — and
no item touches the CROSS_REFERENCE_EVAL or RECONCILE_EVAL *action* lines. These spec-defined
output variants therefore have no plan coverage.

### 2. Error Handling — PASS
The `--eval-report` required-in-report-mode error and the non-existent-file error are covered by
`evalmode.advance-gate` for all three eval-bearing states, with tests. The always-required
`--verdict` check is correctly preserved unchanged.

### 3. Rejection — PASS
Every Rejection-table row tied to eval_mode is covered:
- "without `--eval-report` when report" → error: covered (advance-gate, all three states).
- "`--eval-report` pointing to non-existent file" → covered (checkEvalReportExists retained).
- "`--eval-report` when not report" → spec-worded warning: covered for the state-layer path.
The non-eval_mode rejection rows (add-queue-item/set-roots, verdict-outside-eval) are already
implemented and correctly out of scope.

### 4. Interface — FAIL
`eval` command interface: `PrintSpecEvalOutput` + the specifying-EVALUATE routing case is
correctly added (`evalmode.spec-eval-output`), closing the real gap where `forgectl eval` in
specifying EVALUATE falls through to the default error. The function signature and embedding of
`evaluators.SpecEval` are accurate.

`advance` output interface: the per-mode EVALUATE/REFINE/IMPLEMENT action and status lines are
covered for the three primary phases, but — as in Behavior — the CROSS_REFERENCE_EVAL and
RECONCILE_EVAL advance-action interface variants defined in the specs are not produced by any
item. (See Behavior blocker.)

Minor: `evalmode.spec-eval-output` step 2 shows the routing call as
`state.PrintSpecEvalOutput(...)` and `evalmode.eval-output-modes` lists `PrintSpecEvalOutput`
alongside `PrintReconcileEvalOutput`/`PrintCrossRefEvalOutput`. Note the existing signatures are
`PrintReconcileEvalOutput(w, s)` and `PrintCrossRefEvalOutput(w, s)` (no projectRoot), while
`PrintEvalOutput(w, s, dir)` takes a dir. The plan's prose is consistent with this, but the
implementer should be careful not to assume a uniform 3-arg signature across all four functions.
Nit, not a deficiency.

### 5. Configuration — PASS
`eval_mode` parameter is fully addressed:
- Type/JSON tag on `EvalConfig` (types.go) — `evalmode.config`.
- TOML pointer field + `mergeEvalConfig` pointer-guarded merge — `evalmode.config` (matches the
  existing `EnableEvalOutput *bool` pattern at config.go:26,300-301). Correct.
- Default `report` seeded in `DefaultForgeConfig` — `evalmode.config` step 4. Note: the item's
  `files` array correctly includes `state/types.go` (DefaultForgeConfig lives there, not in
  config.go as the prose loosely implies). No deficiency.
- Validation against {report,direct,conversational} mirroring the `commit_strategy` pattern —
  `evalmode.validate`, with rejection + edge-case (empty) tests. Correct.
- Three-valued enumeration and the back-compat default are documented. EnableEvalOutput is
  correctly retained (not removed) for already-locked state files.

### 6. Observability / Logging — PASS
The only logging-class requirements in scope are the two warnings. `--eval-report` ignore
warning (spec wording) and `--message` ignore warning (spec wording `--message is ignored,
commits are not enabled`) are both covered by `evalmode.advance-gate` and `evalmode.cmd-warnings`,
including the "warning does not instruct how to enable commits" invariant (cmd-warnings step 3).
The "printed exactly once" de-dup concern is explicitly tested.

### 7. Integration Points — PASS
`EvalModeFor` is correctly established as the single resolution point (L1) that the L2/L3 items
depend on. `evalmode.eval-output-modes` correctly depends on both `evalmode.resolve` and
`evalmode.spec-eval-output` (it must compose the new spec-eval function). The
`evaluators.SpecEval` embed (currently unused) is correctly wired through `spec-eval-output`.

### 8. Invariants — PASS
- "report is default for new sessions; conversational preserves legacy
  enable_eval_output:false" — encoded in `EvalModeFor` (resolve item) and tested
  (empty + enable_eval_output:false → conversational; empty + true → report).
- Back-compat: honoring both `ec.EnableEvalOutput || gen.EnableEvalOutput` is correct given the
  current code reads phase-level for REFINE (output.go:516) but general-level for planning
  EVALUATE/status (output.go:499). Folding both into the resolver avoids a regression for
  already-locked sessions. Sound.
- "scaffold does not parse eval files; no path stored in direct/conversational" — preserved
  because the gating only requires/records the report in report mode.

### 9. Edge Cases — PASS
- direct/conversational `--eval-report` accepted-but-ignored → covered (advance-gate edge test).
- omitted `eval_mode` in TOML leaves default (pointer-nil) → covered (config edge test).
- empty `eval_mode` validates as valid → covered (validate edge test).
- conversational omits REPORT OUTPUT and PREVIOUS EVALUATIONS → covered
  (eval-output-modes edge test).
- legacy locked session (enable_eval_output:false) → conversational → covered (resolve edge test).

### 10. Testing Criteria — FAIL
Most spec Testing-Criteria entries touching eval_mode are covered by item tests:
- "Implementing eval command outputs direct correction prompt when eval_mode is direct" and
  "...omits report target when eval_mode is conversational" (batch-implementation.md) → covered
  by `evalmode.eval-output-modes` tests.
- "Planning eval command outputs context" (report) → covered.
- "--message ignored when enable_commits is false" across all three commit specs → covered by
  `evalmode.cmd-warnings`.
- specifying EVALUATE eval regression (no longer errors) → covered by `evalmode.spec-eval-output`.

Gaps:
1. (major) No test asserts the per-mode **CROSS_REFERENCE_EVAL / RECONCILE_EVAL advance action
   text** (the report/direct/conversational variants in spec-lifecycle 299-340 and
   spec-reconciliation 65-97). This follows directly from the Behavior/Interface gap — the work
   isn't planned, so it isn't tested.
2. (minor) `evalmode.eval-output-modes` tests cover reconcile/cross-ref PREVIOUS EVALUATIONS in
   report mode, but there is no test for the **direct-mode "(direct corrections)"** rendering of
   PREVIOUS EVALUATIONS for reconcile/cross-ref specifically (the spec shows this format; the
   planning/spec variant is tested but not reconcile/cross-ref). Spec evidence:
   spec-lifecycle.md line 603 "(direct corrections)".

### 11. Dependencies & Format — PASS
- Item IDs unique; every item appears in exactly one layer; every layer item references a valid
  item ID. Verified by inspection of the `layers` and `items` arrays.
- depends_on form a valid DAG with no cycles: L0 (config) ← L1 (validate, resolve) ←
  L2 (advance-gate, cmd-warnings, spec-eval-output, action-evaluate, action-refine) ←
  L3 (eval-output-modes). `eval-output-modes` depends on resolve + spec-eval-output (both
  earlier layers). Layer ordering respected (no item depends on a later layer).
- Each item has id, name, description, depends_on, steps, files, specs, refs, tests. Tests carry
  category + description. Format-compliant.
- Build-safety: layering will not leave the build broken — L0 adds the field and default
  (compiles), L1 reads it, L2/L3 consume the resolver. EnableEvalOutput retained throughout so
  no half-migrated state-file schema. Sound.

## Deficiency List

| # | Dimension | Spec Section | Missing Coverage |
|---|-----------|-------------|-----------------|
| 1 | Behavior / Interface | spec-lifecycle.md "Entering CROSS_REFERENCE_EVAL" (lines 299–340); spec-reconciliation.md "Entering RECONCILE_EVAL" (lines 65–97) | No item renders the per-`eval_mode` advance/status **action text** for CROSS_REFERENCE_EVAL and RECONCILE_EVAL. `evalmode.action-evaluate` scopes only the EVALUATE state in specifying/planning/implementing. The direct/conversational arms (staged-files note; dropping `--eval-report` from the advance line) for these two states are unplanned. (blocker) |
| 2 | Observability | spec-lifecycle.md Rejection row "`--eval-report` when eval_mode is not report" for CROSS_REFERENCE_EVAL | The cmd-layer `printAdvanceWarnings` `evalStates` map (cmd/advance.go:234-237) currently includes only EVALUATE and RECONCILE_EVAL, NOT CROSS_REFERENCE_EVAL. `evalmode.cmd-warnings` says to swap the boolean for `EvalModeFor` but does not call out adding CROSS_REFERENCE_EVAL to the set; combined with the "emit in exactly one layer" guidance, the spec-worded warning risks not firing in CROSS_REFERENCE_EVAL in any layer. Verify which layer emits it for cross-ref-eval and ensure coverage. (major) |
| 3 | Testing Criteria | spec-lifecycle.md 299–340; spec-reconciliation.md 65–97 | No test asserts per-mode CROSS_REFERENCE_EVAL / RECONCILE_EVAL action text (consequence of deficiency #1). (major) |
| 4 | Testing Criteria | spec-lifecycle.md line 603 ("Round 1: FAIL — (direct corrections)") | No test for the **direct-mode** PREVIOUS EVALUATIONS "(direct corrections)" rendering specifically in `PrintReconcileEvalOutput` / `PrintCrossRefEvalOutput`. (minor) |

## Notes for the planner (non-blocking)
- The plan correctly avoids all already-implemented work. No false-positive build items.
- Back-compat handling via `EvalModeFor(ec, gen)` honoring both the phase-level and general-level
  `EnableEvalOutput` is the right call and will not regress locked sessions.
- To clear the FAIL: extend `evalmode.action-evaluate` (or add a sibling item) to render the
  report/direct/conversational advance+status action text for CROSS_REFERENCE_EVAL and
  RECONCILE_EVAL, add the corresponding tests, and have `evalmode.cmd-warnings` (or advance-gate)
  explicitly cover CROSS_REFERENCE_EVAL in the warning path.
