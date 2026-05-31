# Evaluation Report — Round 2

## Verdict: PASS

## Summary
- Dimensions passed: 11/11
- All three round-1 findings are addressed by real, code-accurate plan changes.
- The plan continues to scope ONLY the `eval_mode` gap (report|direct|conversational) and
  correctly leaves already-implemented work (self_review/SELF_REVIEW, state machine, the 12
  plan.json validation checks, add-queue-item, set-roots, user_review branching, the
  reverse_engineering RECONCILE_EVAL handler) out of scope. No false-positive rebuild items.
- DAG valid, layers respected, all required fields present, back-compat sound.

## Round-1 Fix Verification

### Fix #1 (was blocker) — per-mode action text for CROSS_REFERENCE_EVAL + RECONCILE_EVAL — CONFIRMED
`evalmode.action-evaluate` was broadened. Its name is now "Per-mode action and status text for
all eval-entry states" and step 4 explicitly scopes "ENTERING CROSS_REFERENCE_EVAL
(spec-lifecycle.md 299-340) and ENTERING RECONCILE_EVAL (spec-reconciliation.md 65-97)",
noting "These two states render single-form today (output.go ~193-209 and the reconcile-eval
renderer) and must be migrated too." Step 5 extends the per-mode branch to `status` output for
all of these states.
- Verified against code: `state/output.go` CROSS_REFERENCE_EVAL renderer (lines 193-209) and
  RECONCILE_EVAL renderer (lines 261-275) are indeed single-form today — both unconditionally
  emit `--verdict PASS|FAIL --eval-report <path>` with no direct/conversational arms. The fix
  targets genuinely-missing work.
- Verified against specs: spec-lifecycle.md lines 299-340 define all three CROSS_REFERENCE_EVAL
  variants; spec-reconciliation.md lines 65-97 define all three RECONCILE_EVAL variants. The
  spec-reconciliation.md `## Interface` / Outputs section referenced by fix #1 exists (line 32+,
  with the three RECONCILE_EVAL blocks under it).
- Test #3 of `evalmode.action-evaluate` now asserts the matching per-mode action text for both
  states (report includes `--eval-report <path>`; direct includes staged-files/`evaluate and
  correct` note; conversational has neither). Adequate.

### Fix #2 (was major) — CROSS_REFERENCE_EVAL added to cmd evalStates map — CONFIRMED
`evalmode.cmd-warnings` step 2 now reads: "Add CROSS_REFERENCE_EVAL to the warning's
`evalStates` map (advance.go:234-237 currently lists only EVALUATE and RECONCILE_EVAL), so the
`--eval-report is ignored ...` warning fires for cross-reference eval too."
- Verified against code: `cmd/advance.go` `printAdvanceWarnings` (lines 234-237) does in fact
  list only `StateEvaluate` and `StateReconcileEval`, omitting `StateCrossReferenceEval`. The
  step is necessary and correct.
- A functional test ("advance --eval-report in CROSS_REFERENCE_EVAL in a non-report mode prints
  the spec-worded ignore warning") was added, plus the de-dup edge test ("printed exactly once").
  Combined with the spec-lifecycle.md line-51 Rejection/interface row keying `--eval-report` on
  `eval_mode: "report"`, the warning path is now covered for cross-ref eval in some layer.

### Fix #3 (was minor) — direct-mode "(direct corrections)" PREVIOUS EVALUATIONS test — CONFIRMED
`evalmode.eval-output-modes` test #5 now reads: "Reconcile and cross-reference eval output in
direct mode render prior rounds as `(direct corrections)` in PREVIOUS EVALUATIONS on round 2+."
- Verified against code: `PrintReconcileEvalOutput` (output.go 1745-1787) and
  `PrintCrossRefEvalOutput` (1791-1836) currently emit only REPORT OUTPUT (gated on the legacy
  `EnableEvalOutput`) and have NO PREVIOUS EVALUATIONS section — so step 5 of the item
  ("Add the PREVIOUS EVALUATIONS section ... where it is currently absent") is real work, and the
  new test gives it direct-mode coverage.
- Verified against spec: the `(direct corrections)` format is specified in spec-lifecycle.md
  (lines 499-500 and 603).

## Dimension Results

### 1. Behavior — PASS
Mode-dependent behavior fully covered. EVALUATE/CROSS_REFERENCE_EVAL/RECONCILE_EVAL
`--eval-report` gating per mode → `evalmode.advance-gate` (all three advance.go handlers at
88-110, 185-198, 243-256 confirmed to currently gate on `EnableEvalOutput`). REFINE/IMPLEMENT
re-entry action text → `evalmode.action-refine` (planning REFINE output.go:516, implementing
REFINE :822 confirmed 2-way today). EVALUATE entry action text (all three phases) plus the
now-included CROSS_REFERENCE_EVAL and RECONCILE_EVAL entry action text → `evalmode.action-evaluate`.
The round-1 blocker is resolved.

### 2. Error Handling — PASS
`--eval-report` required-in-report-mode error and non-existent-file error covered by
`evalmode.advance-gate` for all three eval states; always-required `--verdict` check preserved
unchanged (advance.go:90-91 etc.).

### 3. Rejection — PASS
Every eval_mode Rejection row covered: missing report in report mode → error; non-existent file →
error (checkEvalReportExists retained); `--eval-report` when not report → spec-worded warning,
now covering CROSS_REFERENCE_EVAL via fix #2. Non-eval_mode rejection rows are out of scope and
already implemented.

### 4. Interface — PASS
`eval` command: `PrintSpecEvalOutput` + specifying-EVALUATE routing case closes the real
fall-through-to-default gap (`evalmode.spec-eval-output`). Signatures accurate: the plan respects
that `PrintReconcileEvalOutput(w, s)` / `PrintCrossRefEvalOutput(w, s)` take no projectRoot while
`PrintEvalOutput(w, s, dir)` and the new `PrintSpecEvalOutput(w, s, projectRoot)` differ — verified
against output.go. `advance` output: per-mode action/status lines now produced for all eval-entry
states including CROSS_REFERENCE_EVAL/RECONCILE_EVAL.

### 5. Configuration — PASS
`eval_mode` fully addressed: typed JSON field on EvalConfig, TOML pointer field + pointer-guarded
`mergeEvalConfig`, `DefaultForgeConfig` seeding `report` (files array correctly includes
state/types.go where DefaultForgeConfig lives), and `ValidateConfig` rejection against
{report,direct,conversational} mirroring commit_strategy. EnableEvalOutput retained for
back-compat.

### 6. Observability / Logging — PASS
Both warnings covered with exact spec wording: `--eval-report is ignored, --eval-report is only
used in report mode` and `--message is ignored, commits are not enabled`. The
"warning does not instruct how to enable commits" invariant is in `evalmode.cmd-warnings` step 4.
De-dup ("exactly once") explicitly tested. Fix #2 ensures cross-ref-eval emits the warning.

### 7. Integration Points — PASS
`EvalModeFor` (L1) is the single resolution point that all L2/L3 items depend on.
`evalmode.eval-output-modes` correctly depends on `evalmode.resolve` + `evalmode.spec-eval-output`.
`evaluators.SpecEval` embed wired through `spec-eval-output`.

### 8. Invariants — PASS
"report default for new sessions; conversational preserves legacy enable_eval_output:false"
encoded in `EvalModeFor` and tested. Back-compat folds both phase-level and general-level
`EnableEvalOutput` into the resolver (verified the code mixes the two — planning EVALUATE/status
reads general at output.go:499/866; REFINE reads phase at :516/:822 — so the fold is correct and
avoids regression). Scaffold does not parse eval files; no path stored in direct/conversational.

### 9. Edge Cases — PASS
direct/conversational `--eval-report` ignored; omitted eval_mode keeps default (pointer-nil);
empty eval_mode validates as valid; conversational omits REPORT OUTPUT + PREVIOUS EVALUATIONS;
legacy locked session → conversational. All covered with tests.

### 10. Testing Criteria — PASS
Round-1 testing gaps closed: `evalmode.action-evaluate` test #3 covers CROSS_REFERENCE_EVAL /
RECONCILE_EVAL per-mode action text; `evalmode.eval-output-modes` test #5 covers direct-mode
"(direct corrections)" PREVIOUS EVALUATIONS for reconcile/cross-ref. Implementing direct/
conversational eval output, planning report context, `--message` ignored across commit specs, and
the specifying-EVALUATE regression all remain covered.

### 11. Dependencies & Format — PASS
Item IDs unique; each item in exactly one layer; layer items reference valid IDs. DAG valid:
L0 (config) ← L1 (validate, resolve) ← L2 (advance-gate, cmd-warnings, spec-eval-output,
action-evaluate, action-refine) ← L3 (eval-output-modes, depending on resolve + spec-eval-output).
No item depends on a later layer. All items carry id/name/description/depends_on/steps/files/
specs/refs/tests; tests carry category + description. Build-safe layering (L0 compiles standalone;
EnableEvalOutput retained throughout). `forgectl/state` and `forgectl/cmd` confirmed to build
clean (`go build ./...` → OK).

## Deficiency List
None.

## Notes (non-blocking)
- The plan does not touch the reverse_engineering RECONCILE_EVAL handler (advance.go ~1296) or its
  output (PrintReverseEngineeringEvalOutput, output.go ~1839) — correct, as reverse_engineering is
  not among the five in-scope specs and its eval surface is a separate phase.
- `evalmode.action-evaluate` describes the CROSS_REFERENCE_EVAL renderer location as
  "output.go ~193-209"; the actual single-form block is 193-209 for the entry and the compact
  status form at ~261-275/1275+ for reconcile. The item's step 5 ("apply to compact `status`
  output for all of these states") covers the status renderers regardless of the exact line
  numbers cited. Non-blocking.

VERDICT: PASS — All three round-1 findings (one blocker, two major/minor) are addressed by
plan changes that target genuinely-missing work verified against the live code and the five
in-scope specs. All 11 dimensions pass, the DAG and layering are valid, required fields are
present, back-compat via `EvalModeFor` is sound, and no false-positive items rebuild existing
functionality.
