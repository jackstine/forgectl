# state package — config, advance gating, output

## Config (state/types.go, state/config.go)

`EvalConfig` (types.go:66-71) is shared by specifying/planning/implementing eval blocks:
```go
type EvalConfig struct {
    MinRounds        int  `json:"min_rounds"`
    MaxRounds        int  `json:"max_rounds"`
    AgentConfig           // embedded model/type/count
    EnableEvalOutput bool `json:"enable_eval_output"`
}
```
Add: `EvalMode string `json:"eval_mode"`` (values: "report" | "direct" | "conversational").

TOML mirror `tomlEvalConfig` (config.go:19-27) uses pointer fields so "unset" is detectable.
`EvalMode` is a string; add `EvalMode *string `toml:"eval_mode"`` and in `mergeEvalConfig`
(config.go:284-302) apply `if src.EvalMode != nil { dst.EvalMode = *src.EvalMode }`.

`DefaultForgeConfig` (find in config.go) — seed `EvalMode: "report"` on each phase's eval block
(matches the existing default-seeding pattern, cf. how reverse_engineering defaults were seeded).

`ValidateConfig` (config.go:401+) — returns a `[]string` of violations. Add, for each of
specifying/planning/implementing: if `eval_mode != ""` and not in {report, direct, conversational},
append `"<phase>.eval.eval_mode: invalid value %q"`. (Mirror the `commit_strategy` validation
pattern at config.go:411-421.)

### Resolution helper (back-compat)

Add a small helper (state package) used by advance + output so the mode is resolved in ONE place:
```go
// EvalModeFor returns the effective eval mode for the given eval block,
// honoring back-compat with the legacy enable_eval_output boolean.
func EvalModeFor(ec EvalConfig, gen GeneralConfig) string {
    if ec.EvalMode != "" { return ec.EvalMode }
    if ec.EnableEvalOutput || gen.EnableEvalOutput { return "report" }
    return "conversational"
}
```
Also a convenience on ForgeState that picks the current phase's eval block (specifying/planning/
implementing) and returns its mode. This keeps locked legacy sessions behaving as today.

## advance gating (state/advance.go)

Current EVALUATE block (advance.go:91-115) requires `--verdict`, then gates `--eval-report`
on a local `enableEvalOutput` bool:
```go
if enableEvalOutput && in.EvalReport == "" {
    return fmt.Errorf("--eval-report is required in EVALUATE state when enable_eval_output is true")
}
if !enableEvalOutput && in.EvalReport != "" {
    fmt.Fprintf(os.Stderr, "warning: --eval-report is ignored, eval output is not enabled\n")
}
```
Same pattern repeats for CROSS_REFERENCE_EVAL (advance.go:187-211) and RECONCILE_EVAL (~245).

Change all three to gate on `EvalModeFor(...) == "report"`:
- report + missing report → error `--eval-report is required in <STATE> state` (keep state-named error).
- mode != report + report provided → warning `--eval-report is ignored, --eval-report is only used in report mode`.
- still call `checkEvalReportExists(in.EvalReport)` when a report path is provided in report mode.

`--verdict` requirement is unchanged (always required in eval states).

## output (state/output.go)

Eval-context output functions (called by `forgectl eval`):
- `PrintEvalOutput` (planning + implementing) — already prints `--- REPORT OUTPUT ---` and
  `--- PREVIOUS EVALUATIONS ---` gated on `EnableEvalOutput` (output.go:1612,1625-1633,1668,1722-1730).
- `PrintReconcileEvalOutput` (output.go:1745+) — REPORT OUTPUT shown; PREVIOUS EVALUATIONS missing.
- `PrintCrossRefEvalOutput` (output.go:1791+) — REPORT OUTPUT shown; PREVIOUS EVALUATIONS missing.
- (NEW) `PrintSpecEvalOutput` — specifying batch eval; must embed `evaluators.SpecEval`,
  print `=== SPEC EVALUATION ROUND n/m ===`, `--- SPECS TO EVALUATE ---` list.

Per-mode behavior for ALL eval-context functions:
- report → `--- REPORT OUTPUT ---` "Write your evaluation report to:\n  <path>" + PREVIOUS EVALUATIONS with report paths.
- direct → `--- REPORT OUTPUT ---` "Make corrections directly to the {spec|batch|plan} files." + PREVIOUS EVALUATIONS as "Round n: VERDICT — (direct corrections)".
- conversational → omit REPORT OUTPUT and PREVIOUS EVALUATIONS entirely.

Action/status text rendering (the `advance`/`status` "Action:" lines):
- EVALUATE action text is currently single-form per phase (output.go ~98-117 specifying,
  ~490-503 planning, ~840-870 implementing). Add 3-way branch on mode (see overview table).
- REFINE action text already branches 2-way on `EnableEvalOutput` (output.go:516 planning,
  :822 implementing). Convert to 3-way on `eval_mode` and add the **direct** arm
  ("Review unstaged changes from the evaluator (git diff)."). Add specifying REFINE branching too.
- Implementing IMPLEMENT re-entry-after-eval text (batch-implementation.md rounds 2+) uses the
  same three arms — branch it on mode as well.

## Testing
`state/*_test.go` are table-driven against `ForgeState`. Add cases that set `EvalMode` and assert
gating (required/ignored), output sections, and action text per mode. See notes/testing.md.
