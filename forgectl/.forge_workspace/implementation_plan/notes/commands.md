# Notes: CLI Commands (init / advance / eval / handoff)

Backs the L3 command items and L0 evaluator embed. Files relative to `forgectl/` domain root. Cobra wiring: each command file has a package-level `var xCmd` + `func init(){ rootCmd.AddCommand(xCmd) }`; state-aware commands call the shared `resolveSession()` helper (`cmd/root.go:33`).

## init — `cmd/init.go` (item: init-ui)

- `validPhases` map at `init.go:38` is the ONLY phase allowlist: add `"ui_implementing": true`.
- Update the error string (`init.go:40`) and `--phase` flag help (`init.go:27`) to include `ui_implementing`.
- Phase switch (`init.go:81-189`): add `case state.PhaseUIImplementing:` after `PhaseImplementing`. Behavior: (1) validate the four required UI keys non-empty — `cfg.UIImplementing.App.LaunchCommand`, `.App.URL`, `.E2E.TestCommand`, `.E2E.TestDir` — error naming each empty key, exit 1; (2) otherwise identical to the implementing case — `ValidatePlanJSON`, add `passes:"pending"`/`rounds:0` to items, build state via `state.NewUIImplementingState()`.
- `phaseRoundConfig` (`init.go:222-236`): add `case state.PhaseUIImplementing:` returning `cfg.UIImplementing.Batch, cfg.UIImplementing.Eval.MinRounds, cfg.UIImplementing.Eval.MaxRounds`.
- `--phase planning` already validates the plan-queue `kind` field (validation lives in `state.ValidatePlanQueue`; see notes/state-types.md) — covered by the L0 kind-field item, surfaced here at planning init.

## advance — `cmd/advance.go` (item: advance-eval-wiring, shared with eval)

- `validateAdvanceFlags` `validStates` (`advance.go:218-227`): add `state.StateQATest` and `state.StateE2EVerify` so `--verdict` is accepted there; update its error message.
- `printAdvanceWarnings` `evalStates` (`advance.go:234-238`): add the same two states. The per-phase eval-mode lookup (`244-252`) needs a `PhaseUIImplementing` branch that selects the loop by current state (EVALUATE→Eval, QA_TEST→QA, E2E_VERIFY→E2E) so the `--eval-report ignored` warning fires correctly per loop mode.
- `isTerminalState` (`advance.go:185-209`): add `PhaseUIImplementing && StateDone` so completed sessions archive.
- No new flags needed (`--verdict`/`--eval-report`/`--message` already defined).

## eval — `cmd/eval.go` (item: advance-eval-wiring)

`runEval` switch (`eval.go:32-48`): add before `default`:
- `ui_implementing` + `EVALUATE` → `state.PrintEvalOutput(...)` (reuses impl eval).
- `ui_implementing` + `QA_TEST` → `state.PrintUIQAEvalOutput(...)` (new, notes/output.md).
- `ui_implementing` + `E2E_VERIFY` → `state.PrintUIE2EEvalOutput(...)` (new).
- `ui_implementing` (any other state) → error "eval is only valid in EVALUATE, QA_TEST, or E2E_VERIFY (current: %s)" (invariant 13 negative).

## handoff — NEW `cmd/handoff.go` (item: handoff)

Does not exist today. Model on `cmd/adddomain.go` (state-gated mutate→save→print). Spec: ui-batch-implementation.md §`handoff` command + §Hand-off Behavior + Rejection table.
- `Use: "handoff <file> [<file>...]"`, `Args: cobra.MinimumNArgs(1)`.
- Load state; gate: phase must be `ui_implementing` and state in {EVALUATE, QA_TEST, E2E_VERIFY}, else error naming current state+phase (exit 1).
- For each arg: `os.Stat` to verify existence; a missing file → error naming the path, register nothing (exit 1).
- On success: set `s.UIImplementing.CurrentBatch.HandedOffArtifacts = args` (latest hand-off wins, replacing prior), `state.Save`, print the registered files. Carries NO verdict and does not transition (invariant 15). The `Review:` line is rendered by output (notes/output.md), not here.
- Coexists with `--eval-report`: divergence between a handed-off report and a later `--eval-report` path is a WARN at advance time (handled in advance/output, not handoff).

## Tests (`cmd/commands_test.go`, `cmd/validate_test.go`)

Direct-call pattern (`setupProjectDir`, set flag vars, call `runX`, `state.Load`, assert; output via `rootCmd.SetOut`). Add: init ui_implementing success; init rejects each of the 4 missing keys; init rejects plan-queue `kind:"frontend"`; eval in QA_TEST/E2E_VERIFY contains prompt; eval in UI_REFINE rejected; `--verdict` valid in QA_TEST; handoff registers artifacts + `Review:` via status; handoff rejected outside evaluator state; handoff rejects non-existent file; handoff with no args rejected.
