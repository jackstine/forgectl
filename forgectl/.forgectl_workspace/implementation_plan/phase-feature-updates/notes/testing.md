# Testing patterns

## Where tests live
- `forgectl/state/config_test.go` — config merge/validate/default tests (table-driven).
- `forgectl/state/advance_test.go` (and sibling `*_test.go`) — transition + flag-gating tests on `ForgeState`.
- `forgectl/state/output_test.go` — assert rendered output strings (`bytes.Buffer` capture).
- `forgectl/cmd/commands_test.go` — end-to-end command runs via the Cobra command tree.

## Conventions
- Table-driven: `tests := []struct{ name string; ... }` then `for _, tt := range tests { t.Run(tt.name, ...) }`.
- Build a `ForgeState` in-memory with the desired `Config.<Phase>.Eval.EvalMode` and `State`/`Phase`,
  then call the function under test and assert error / captured output substring.
- Output assertions use `strings.Contains(buf.String(), "...")` against the exact spec strings.

## Coverage to add (maps to spec Testing Criteria)
- **Config**: default seeds `eval_mode: "report"`; TOML override merges; back-compat
  (`EvalModeFor` returns conversational when only `enable_eval_output:false`); validate rejects
  an invalid `eval_mode` value.
- **Gating** (advance): report mode requires `--eval-report` (error when missing); direct/
  conversational ignore it with the spec-worded warning; report file existence still checked.
- **eval output**: specifying EVALUATE emits `=== SPEC EVALUATION ROUND ===` (regression for the
  routing gap); report vs direct vs conversational REPORT OUTPUT section; PREVIOUS EVALUATIONS
  present in report/direct, omitted in conversational, and present for reconcile/cross-ref.
- **Action text**: EVALUATE and REFINE action lines differ per mode across specifying/planning/
  implementing (report "Study the eval file ..."; direct "Review unstaged changes ... (git diff)";
  conversational "Make corrections based off communication with the evaluator.").

Run: `cd forgectl && go test ./...`
