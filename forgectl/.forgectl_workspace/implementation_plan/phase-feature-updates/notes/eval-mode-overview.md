# Phase Feature Updates — `eval_mode` Overview

## The feature being added

The five in-scope specs (plan-production, batch-implementation, spec-lifecycle,
spec-reconciliation, phase-transitions) were expanded (commit `025608d`, +505 lines)
to describe a **three-valued `eval_mode`** config that controls how the evaluation
sub-agent communicates and how `advance` gates the `--eval-report` flag.

`eval_mode` lives on each phase's eval block:
`specifying.eval.eval_mode`, `planning.eval.eval_mode`, `implementing.eval.eval_mode`
(and is referenced for cross-reference / reconciliation via `specifying.eval`-style blocks).

| Mode | `--eval-report` | Sub-agent behavior | `eval` output | `advance` action text |
|------|-----------------|--------------------|---------------|------------------------|
| `report` (default) | **required**; file must exist | writes a markdown report file | includes `--- REPORT OUTPUT ---` "Write your evaluation report to: <path>" + `--- PREVIOUS EVALUATIONS ---` with report paths | EVALUATE: "advance with --verdict PASS\|FAIL --eval-report <path>"; REFINE: "Study the eval file ..." |
| `direct` | ignored (warning) | makes corrections directly to the staged files | `--- REPORT OUTPUT ---` "Make corrections directly to the {spec/batch/plan} files." + PREVIOUS EVALUATIONS as "(direct corrections)" | EVALUATE: "evaluate and correct ... files have been staged ... advance with --verdict PASS\|FAIL"; REFINE: "Review unstaged changes from the evaluator (git diff)." |
| `conversational` | ignored (warning) | communicates verdict verbally | REPORT OUTPUT and PREVIOUS EVALUATIONS **omitted** | EVALUATE: "advance with --verdict PASS\|FAIL"; REFINE: "Make corrections based off communication with the evaluator." |

Warning text required by spec when `--eval-report` is passed in a non-report mode:
`--eval-report is ignored, --eval-report is only used in report mode`.

## What is ALREADY implemented (do NOT re-build)

- `planning.self_review` config + `SELF_REVIEW` state + DRAFT/VALIDATE→SELF_REVIEW→EVALUATE
  transitions + its action text. (state/types.go:113,40; advance.go:535,566-584; output.go:469-488)
- All planning + specifying + implementing state constants and their transitions.
- The 12 plan.json structural validation checks (state/validate.go `ValidatePlanJSON`).
- `add-queue-item` and `set-roots` commands (flags, state-gating, domain resolution, validation).
- `CROSS_REFERENCE_REVIEW` / `RECONCILE_REVIEW` `user_review` branching (output.go:227-237, 290-299).
- The `--message is ignored ...` warning EXISTS but with non-spec wording (see below).
- Planning REFINE + implementing REFINE already branch **2-way** on `EnableEvalOutput`
  (report vs conversational) at output.go:516 and :822 — needs to become 3-way on `eval_mode`.

## What is NEW / MISSING (this plan's scope)

1. **`eval_mode` config does not exist.** Everything gates on the boolean
   `EvalConfig.EnableEvalOutput` (state/types.go:70) and `GeneralConfig.EnableEvalOutput`
   (types.go:165). No `EvalMode` field, no enum, no validation.
2. **`--eval-report` gating** keys off `enableEvalOutput` (state/advance.go:96-101, 194-198,
   and the RECONCILE_EVAL block ~245) instead of `eval_mode == "report"`. Warning wording
   differs from spec.
3. **`direct` mode is entirely unexpressed.** No "Review unstaged changes from the evaluator
   (git diff)." action text; no "Make corrections directly to the {spec/batch/plan} files."
   strings in Go (only in spec docs).
4. **`forgectl eval` has NO case for specifying `EVALUATE`** (cmd/eval.go:32-45 routing).
   It falls to the default error. `evaluators.SpecEval` is embedded but unused. Per
   spec-lifecycle.md the specifying batch eval must output a `SPEC EVALUATION ROUND` block.
5. **EVALUATE action text** never varies by mode (single text in all phases).
6. **`--- PREVIOUS EVALUATIONS ---`** missing from `PrintReconcileEvalOutput` and
   `PrintCrossRefEvalOutput` (and the new spec-eval output) — present only for planning/implementing.

## Back-compat note

Existing sessions have `enable_eval_output:false` locked into their state and NO `eval_mode`.
A resolution helper should treat an empty `eval_mode` as: `report` when `enable_eval_output`
is true, else `conversational` — so locked sessions keep their current (no-report) behavior.
New sessions default `eval_mode` to `report` via `DefaultForgeConfig`.

## File map (where work lands)

- `forgectl/state/types.go` — add `EvalMode` to `EvalConfig`.
- `forgectl/state/config.go` — `tomlEvalConfig` field, `mergeEvalConfig`, `DefaultForgeConfig` default, `ValidateConfig`.
- `forgectl/state/advance.go` — `--eval-report` required/ignored gating per `eval_mode`.
- `forgectl/cmd/advance.go` — `printAdvanceWarnings` (eval-report + message wording).
- `forgectl/cmd/eval.go` — add specifying EVALUATE routing case.
- `forgectl/state/output.go` — eval-context output functions + EVALUATE/REFINE/IMPLEMENT action & status text, per mode.
