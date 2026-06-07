# Notes: Output & Eval-Context Rendering (output.go)

Backs the L1 rendering items. Files relative to `forgectl/` domain root. Canonical formats: `specs/ui-batch-implementation.md` §Outputs (advance output, eval output, status output).

## Dispatch points needing a `case PhaseUIImplementing:` — `state/output.go`

- `PrintAdvanceOutput` switch (`output.go:29-47`) → call new `printUIImplementingOutput`.
- `CurrentEvalMode` (`output.go:17-26`): return the eval_mode of the loop matching the current state (EVALUATE→`Eval`, QA_TEST→`QA`, E2E_VERIFY→`E2E`) from `s.Config.UIImplementing`.
- `phaseConfig` (`output.go:1639-1652`): return batch + primary (code) eval min/max for the status header.
- `printProgressLine` (`output.go:1656-1696`): "N/M passed, F failed, R remaining" sourced from plan.json item `passes`.
- `PrintEvalOutput` (`output.go:1702-1711`): route ui_implementing EVALUATE to the existing `printImplementingEval` (same `impl-eval.md`, same item list); route QA_TEST/E2E_VERIFY to the new functions below.

## printUIImplementingOutput (item: output-advance)

A `switch s.State` mirroring `printImplementingOutput` (`output.go:685-1067`). States: ORIENT, IMPLEMENT, EVALUATE, QA_TEST, UI_REFINE, E2E_AUTHOR, E2E_VERIFY, E2E_REMEDIATE, COMMIT, DONE. The `Phase:` line reads `ui_implementing`. Reuse renderers: `writeEvalEntryAction` (`output.go:70-87`, 3-mode action block) for EVALUATE/QA_TEST/E2E_VERIFY; `writeRefineBody` (`output.go:98-112`) for UI_REFINE/E2E_REMEDIATE.

Per-state specifics from the spec:
- EVALUATE/QA_TEST/E2E_VERIFY add a `Loop:` line (`code`/`qa`/`e2e`) and `Round: n/max`.
- QA_TEST adds `App:` (launch+url) and `Steps:` (step-list path); action tells the agent to drive Playwright MCP and `forgectl handoff <qa-report> <step-list>`.
- E2E_AUTHOR adds `Steps:` (path + scenario count), `Tests:` (test_dir), `Run:` (test_command); zero-scenario variant has its own short action.
- E2E_VERIFY adds `Run:` (test_command).
- **`Review:` line**: when `s.UIImplementing.CurrentBatch.HandedOffArtifacts` is non-empty, render a `Review:` block listing the handed-off paths before the `Action:` line, in QA_TEST/EVALUATE/E2E_VERIFY (handoff behavior, ui-batch-implementation.md §Hand-off + Outputs).
- COMMIT lists items with terminal status (`passed`/`failed (loop force-accept, n/n rounds)`); action gated on `enable_commits` (`--message` required when true).
- DONE summary reports per-loop round totals: `Rounds: code C, qa Q, e2e E (across B batches)`.
- `user_guided:true` adds the STOP line in ORIENT (invariant 16).
- `status` compact: in QA/e2e loops show the `Batch:` item list (no single active item) + `Loop:`/round; `--verbose` adds per-item per-loop round breakdown from history.

## PrintUIQAEvalOutput + PrintUIE2EEvalOutput (item: output-eval)

New functions in `output.go`, gated to their states. Embed the prompts from the evaluators package (`evaluators.UIQAEval`, `evaluators.UIE2EEval` — see notes/evaluators.md). Exact section layout from ui-batch-implementation.md §`eval` output:

QA (`=== UI QA EVALUATION ROUND n/max ===`): `--- QA EVALUATOR INSTRUCTIONS ---` (embed UIQAEval) · `--- APPLICATION ---` (Launch, URL, Ready timeout, Driver=Playwright MCP) · `--- ITEMS TO QA ---` (per-item description/specs/refs/files/steps) · `--- STEP LIST OUTPUT ---` (all modes) · `--- REPORT OUTPUT ---` (report mode only; direct = "make corrections directly"; conversational omits) · `--- HANDOFF ---` (all modes; report/direct list report+step-list, conversational lists step-list only) · `--- PREVIOUS EVALUATIONS ---` on rounds >1.

E2E (`=== UI E2E VERIFICATION ROUND n/max ===`): `--- E2E EVALUATOR INSTRUCTIONS ---` (embed UIE2EEval) · `--- E2E SUITE ---` (Step list, Test command, Test dir, Runner=Playwright test runner) · `--- REPORT OUTPUT ---` (report mode only) · `--- HANDOFF ---` (report mode only; omitted for direct/conversational) · `--- PREVIOUS EVALUATIONS ---` on rounds >1.

Config sourced from `s.Config.UIImplementing.App.{LaunchCommand,URL,ReadyTimeoutSeconds}` and `.E2E.{TestCommand,TestDir}`; paths under `<domain>/.forgectl_workspace/ui_plan/`.

## Tests (`state/output_test.go`)

Use the `outputOf(s,dir)` + `strings.Contains` pattern. Assert: QA_TEST output contains the QA prompt H1 (`UI QA Evaluation Prompt`), app URL, and step-list path; E2E_VERIFY contains the e2e prompt H1 (`UI E2E Verification Prompt`), test dir, test command; `Review:` line appears when artifacts handed off; per-state advance outputs match the spec lines.
