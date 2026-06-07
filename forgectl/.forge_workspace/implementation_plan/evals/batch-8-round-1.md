# Evaluation Report

**Round:** 1
**Batch:** 8
**Layer:** L3 Commands & Phase-Shift Routing

VERDICT: FAIL

## Items Evaluated

### [advance.phaseshift] Route phase shift by plan kind

**Files reviewed:** state/advance.go (advancePhaseShift ~1229-1419; routeImplementationDomainBoundary 1192-1225; ValidateUIConfigKeys 1171-1186), state/advance_test.go (3440-3589)

#### Test Results

- [PASS] planning→implementation PHASE_SHIFT with active plan kind:ui and non-empty UI config sets phase ui_implementing / state ORIENT and mutates plan.json items; kind:code or absent sets phase implementing.
  - advance.go:1334-1349 routes on `s.Planning.CurrentPlan.Kind == "ui"`: validates UI keys, builds `NewUIImplementingState`, sets `PhaseUIImplementing`; else builds `NewImplementingState`, sets `PhaseImplementing`. Items are annotated `passes:"pending"`/`rounds:0` and plan.json rewritten (1318-1331) before routing. `TestAdvancePhaseShiftRoutesByKind` covers ui/code/absent and asserts phase, ORIENT, and `plan.Items[0].Passes == "pending"`.
- [PASS] planning→implementation PHASE_SHIFT with kind:ui and an empty required UI config key prints the missing key and remains at PHASE_SHIFT.
  - advance.go:1335-1338 returns a `*ValidationError` from `ValidateUIConfigKeys` before any phase mutation, so state stays PHASE_SHIFT. `TestAdvancePhaseShiftUIMissingConfigKey` clears `e2e.test_dir`, asserts the error names `e2e.test_dir`, and that state/phase remain PHASE_SHIFT/planning.
- [PASS] In all-planning-first mode the implementation→implementation domain boundary routes the next plan by its kind (code→implementing, ui→ui_implementing).
  - advance.go:1361-1389 pops the next queue entry and delegates to `routeImplementationDomainBoundary`, which validates UI keys for ui and sets the destination phase/state. Both the implementing→implementing (1361) and ui_implementing→implementing (1376) boundaries are handled. `TestAdvancePhaseShiftDomainBoundaryRoutesByKind` covers code and ui next-plan kinds and asserts destination phase and `CurrentPlanFile`.

#### Notes
The implementation-domain-boundary block does not re-read/mutate plan.json (passes/rounds) for the next domain's plan, which step 4's prose mentions. This matches the pre-batch-8 behavior of the implementing→implementing boundary (verified against commit a98487a) and is not asserted by any acceptance criterion (the boundary criterion only checks phase routing). Not a deficiency.

### [cmd.init] init --phase ui_implementing

**Files reviewed:** cmd/init.go (27,38-41,174-229,287-288), cmd/commands_test.go (220-363)

#### Test Results

- [PASS] init --phase ui_implementing with valid config and plan.json creates state with phase ui_implementing, state ORIENT, and plan.json items carrying passes/rounds.
  - init.go:38 adds `ui_implementing` to validPhases; case at 174 validates UI keys, validates plan.json, resets `passes:"pending"`/`rounds:0`, rewrites the file, and builds state via `state.NewUIImplementingState` (222). `phaseRoundConfig` has a `PhaseUIImplementing` case (287). `TestInitUIImplementing` asserts phase/ORIENT/started_at_phase and item passes/rounds.
- [PASS] init --phase ui_implementing with any one of the four required UI config keys empty exits 1 naming the missing key.
  - init.go:177-195 collects each empty key (launch_command, url, test_command, test_dir), prints `missing required UI config key: <k>` per key, and returns an error. `TestInitUIImplementingRejectsMissingConfig` exercises all four keys individually and asserts the key is named and no state file is written.
- [PASS] init --phase planning rejects a plan-queue entry whose kind is an invalid value, naming the entry and value.
  - ValidatePlanQueue (validate.go:120-125) rejects any kind other than code/ui with `plans[%d]: invalid kind %q`. `TestInitRejectsInvalidPlanQueueKind` uses kind "frontend" and asserts the value is named.

#### Notes
None.

### [cmd.evalwiring] Wire advance/eval for the new evaluator states

**Files reviewed:** cmd/advance.go (185-202 isTerminalState, 215-236 validateAdvanceFlags, 239-289 printAdvanceWarnings), cmd/eval.go (32-55), cmd/commands_test.go (411-464)

#### Test Results

- [PASS] forgectl eval in ui_implementing QA_TEST emits the QA context and in E2E_VERIFY emits the e2e context; --verdict is accepted in both states.
  - eval.go:47-50 routes QA_TEST→`PrintUIQAEvalOutput` and E2E_VERIFY→`PrintUIE2EEvalOutput`; EVALUATE→`PrintEvalOutput` (45). `validateAdvanceFlags` adds StateQATest/StateE2EVerify to validStates (228-229). `TestEvalUIImplementingQATest`/`E2EVerify` and `TestAdvanceVerdictValidInUILoops` confirm.
- [FAIL] forgectl eval in ui_implementing UI_REFINE is rejected naming the current state and phase.
  - The rejection at eval.go:52 names only the state: `eval is only valid in EVALUATE, QA_TEST, or E2E_VERIFY (current: %s)` interpolating `s.State` only. The phase (`ui_implementing`) is NOT named. The spec rejection row (ui-batch-implementation.md:449) and this acceptance criterion both require naming the current state **and phase**. The accompanying test `TestEvalUIImplementingRejectsNonEvalState` only asserts the valid-state list is present and does not assert the phase, so it passes despite the missing phase. By contrast `handoff` (handoff.go:45) correctly names both phase and state — eval should match.
- [PASS] ui_implementing DONE is treated as a terminal state so the session is archived on advance.
  - isTerminalState (advance.go:191-193) returns true for `PhaseUIImplementing && StateDone`.

#### Notes
The printAdvanceWarnings PhaseUIImplementing branch (260-270) correctly selects the per-loop EvalConfig (QA for QA_TEST, E2E for E2E_VERIFY, code Eval otherwise) for the `--eval-report` warning, matching the step requirement.

### [cmd.handoff] Add the handoff command

**Files reviewed:** cmd/handoff.go (full), state/types.go:585 (HandedOffArtifacts), state/output.go:1573-1583 (writeUIReviewLine), cmd/commands_test.go (470-559)

#### Test Results

- [PASS] handoff in QA_TEST with existing files registers them (latest call replaces prior) and a subsequent status shows a Review: line listing them with no verdict recorded.
  - handoff.go:61 sets `CurrentBatch.HandedOffArtifacts = args` (replace semantics), Saves, prints the registered files; no transition. `writeUIReviewLine` renders the `Review:` block. `TestHandoffRegistersArtifactsForReview` asserts both artifacts recorded, state unchanged (QA_TEST), and status surfaces a `Review:` line listing both.
- [PASS] handoff outside an evaluator state (e.g. UI_REFINE) is rejected naming the current state and phase.
  - handoff.go:44-46 gates to PhaseUIImplementing + {EVALUATE, QA_TEST, E2E_VERIFY}, error names both phase and state via `(current: %s %s)`. `TestHandoffRejectedOutsideEvaluatorState` asserts the error contains both `UI_REFINE` and `ui_implementing` and that nothing is registered.
- [PASS] handoff naming a non-existent file, and handoff with no file arguments, are each rejected and register nothing.
  - handoff.go:54-58 `os.Stat`s every arg before registering any; a missing file returns `file not found: <path>` and registers nothing. `Args: cobra.MinimumNArgs(1)` rejects no-args. `TestHandoffRejectsNonExistentFile` and `TestHandoffRejectsNoArgs` confirm.

#### Notes
Modeled cleanly on the adddomain pattern; replace-not-append semantics match the spec ("latest hand-off wins").

## Build & Tests

- `go build ./...` — succeeds.
- `go test ./...` (with `-count=1`):
  - `?   	forgectl	[no test files]`
  - `ok  	forgectl/cmd	0.406s`
  - `ok  	forgectl/evaluators	0.463s`
  - `ok  	forgectl/state	1.214s`

All tests pass. Note: the failing acceptance criterion below is satisfied by the test as written but not by the implementation — the test under-specifies the criterion (it does not assert the phase is named), so the gap is not caught by the suite.

## Deficiencies

- In cmd/eval.go:52, the ui_implementing eval rejection message names only the current state, not the phase. Change it to name both — e.g. `eval is only valid in ui_implementing EVALUATE, QA_TEST, or E2E_VERIFY (current: %s %s)` interpolating `s.Phase, s.State` — to satisfy the spec rejection row (ui-batch-implementation.md:449) and the acceptance criterion ("naming the current state and phase"). Match the wording already used in handoff.go:45.
- In cmd/commands_test.go, strengthen `TestEvalUIImplementingRejectsNonEvalState` to also assert the error names the current state (`UI_REFINE`) and phase (`ui_implementing`), so the criterion ("naming the current state and phase") is actually enforced by the suite.

## Summary

Three of the four items (advance.phaseshift, cmd.init, cmd.handoff) fully satisfy every acceptance criterion, with correct kind-routing, UI-config validation, terminal-state handling, and a well-gated handoff command; the build and full test suite pass. The one gap is in cmd.evalwiring: the ui_implementing `eval` rejection message names only the current state, not the phase, which the spec and the acceptance criterion both require (the handoff command gets this right). It is a one-line fix plus a test assertion tightening.
