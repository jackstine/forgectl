# Evaluation Report

**Round:** 1
**Batch:** 3
**Layer:** L0 Type & Config Foundations

VERDICT: PASS

## Items Evaluated

### [config.toml] Decode and merge [ui_implementing] TOML

**Files reviewed:** state/config.go, state/config_test.go

#### Test Results

- [PASS] A [ui_implementing] TOML block with app/eval/qa/e2e values overrides the corresponding ForgeConfig fields after load.
  - `tomlUIImplementingConfig` (config.go:110-117) mirrors the spec table with nested `app`/`eval`/`qa`/`e2e` tables. `tomlUIE2EConfig` (config.go:103-107) embeds `tomlEvalConfig` so the e2e loop's round fields decode at the same `[ui_implementing.e2e]` level as `test_command`/`test_dir`. The `UIImplementing` field is wired into `tomlForgeConfig` (config.go:167). The merge block (config.go:289-313) overrides batch/commit_strategy/app via non-zero/non-empty guards, merges all three loops through `mergeEvalConfig`, and the e2e merge correctly targets the embedded config via `&raw.UIImplementing.E2E.tomlEvalConfig` (config.go:307). `TestLoadConfigUIImplementingToml` exercises all of these (batch=2, strategy="all", app overrides, eval sonnet/2/5, qa max=4 eval_mode=direct, e2e min/max + test_command/test_dir) and passes.
- [PASS] When [ui_implementing] is absent from config, all UIImplementing fields fall back to DefaultForgeConfig values.
  - `DefaultForgeConfig` seeds the `UIImplementing` block (types.go:289-304) with Batch:1, CommitStrategy:"scoped", App.ReadyTimeoutSeconds:30, and each loop via `defaultUILoopEvalConfig()` (min 1 / max 3 / opus / eval / count 1 / mode report). Merge guards skip zero/empty TOML values, preserving defaults. `TestLoadConfigUIImplementingDefaults` (config supplies only `[implementing]`) confirms batch, strategy, app timeout, and per-loop defaults survive. Passes.

#### Notes
Unset-vs-zero semantics are preserved: scalar overrides use `> 0` / `!= ""` guards and `mergeEvalConfig` only writes non-zero/non-empty fields, so an absent table leaves the seeded default intact. The embedded-EvalConfig field promotion is correct on both the TOML side (anonymous `tomlEvalConfig`) and the Go config side (anonymous `EvalConfig` in `UIE2EConfig`), and the test reads `ui.E2E.MinRounds` (a promoted field) directly, confirming promotion works.

### [config.validate] Validate ui_implementing config constraints

**Files reviewed:** state/config.go, state/config_test.go

#### Test Results

- [PASS] ValidateConfig rejects ui_implementing.batch < 1.
  - config.go:536-538 emits `"ui_implementing.batch must be >= 1"`. `TestValidateConfigUIImplementingBatchBelowOne` passes.
- [PASS] ValidateConfig rejects min_rounds > max_rounds for each of eval, qa, and e2e loops, naming the loop.
  - The `uiLoops` map (config.go:516-520) collects all three loops (e2e via the embedded `E2E.EvalConfig`), and config.go:555-559 emits `"ui_implementing.<loop>.min_rounds cannot exceed max_rounds"` per loop. `TestValidateConfigUIImplementingMinExceedsMax` is table-driven over eval/qa/e2e and asserts the loop-named violation; all three subtests pass.
- [PASS] ValidateConfig rejects an invalid ui_implementing commit_strategy and an invalid loop eval_mode.
  - Bad commit_strategy is caught at config.go:495-497; per-loop bad eval_mode at config.go:521-525 (named `ui_implementing.<loop>.eval_mode`). `TestValidateConfigUIImplementingBadStrategyAndEvalMode` checks both and passes.

#### Notes
Required-key deferral confirmed: ValidateConfig (config.go:475-579) contains no checks against `app.launch_command`, `app.url`, `e2e.test_command`, or `e2e.test_dir`. A grep over config.go shows those identifiers appear only in the TOML structs and the merge block, never in the validator. Since `ValidateConfig` runs for every phase, deferring those required-key checks to the init/phase-shift boundary is correct and matches the notes and spec.

### [types.uistate] Add UIImplementingState and UIBatchState types

**Files reviewed:** state/types.go, state/state.go, state/types_config_test.go

#### Test Results

- [PASS] NewUIImplementingState returns a zero-valued UIImplementingState (nil current_batch, batch_number 0) ready for ORIENT.
  - `NewUIImplementingState` (state.go:205-208) returns `&UIImplementingState{LayerHistory: []UILayerHistory{}}` — nil CurrentBatch, nil CurrentLayer, BatchNumber 0, empty-but-non-nil LayerHistory. `TestNewUIImplementingState` asserts exactly these and passes.
- [PASS] A ForgeState carrying a populated UIImplementing with all three round counters and histories round-trips through Save/Load unchanged.
  - `UIBatchState` (types.go:568-588) defines the three round counters, three eval histories, handed_off_artifacts, and three force-accept flags; `UIBatchHistory`/`UILayerHistory`/`UIImplementingState` (types.go:592-619) mirror the implementing-phase shapes with per-loop round/eval fields; `ForgeState.UIImplementing` is wired in at types.go:686. `TestForgeStateUIImplementingSaveLoadRoundTrip` populates every field (all three counters, all three eval histories, force-accept, layer history, plan file/domain) and asserts `reflect.DeepEqual` after Save/Load; passes.

#### Notes
Types match notes/state-types.md exactly. `EvalRecord` and `LayerRef` are reused as-is. The decision to give the phase its own state structs (rather than extending BatchState) is honored, so the shipped implementing-phase schema is untouched — no serialization-drift regression risk.

## Summary

All three items are fully implemented and every acceptance criterion is satisfied. `go build ./...` succeeds and `go test ./state/` passes (the seven batch-3 tests verified individually). Merge preserves unset-vs-zero semantics, embedded EvalConfig field promotion works on both TOML and Go sides, the e2e merge reaches the embedded `tomlEvalConfig`, and the four required app/e2e keys are correctly NOT required in the always-run ValidateConfig.
