# Notes — testing conventions

## Patterns (existing)

- **cmd/ tests** (`cmd/*_test.go`): table-driven. Build a temp project dir with
  `t.TempDir()` containing `.forgectl/config` + a state fixture; invoke the
  command (via the `executeCommand(rootCmd, args...)` helper or by calling
  `RunE`); assert stdout and the returned error / exit behavior. See
  `cmd/init_test.go`, `cmd/validate_test.go`.
- **state/ tests** (`state/*_test.go`): unit tests on `Advance`, the validators,
  and output rendering. Table-driven `(name, input, want)` rows; golden-style
  string assertions for action output; `t.TempDir()` for on-disk path checks;
  fixtures inline as structs / JSON strings. See `state/transitions_test.go`,
  `state/validate_test.go`, `state/output_test.go`.
- **Convention**: every new validator / transition / output / command gets a
  sibling `_test.go` with rows covering the happy path plus each rejection,
  error, and edge-case branch.

## Mapping spec Testing Criteria → test rows

reverse-engineering.md ships ~40 Testing Criteria; session-init / state-persistence
/ activity-logging / validate-command add the supporting ones. They map almost
1:1 to table rows. Key clusters:

- **Init / phase**: init validates domains (duplicate rejected), ORIENT displays
  domain order, init at `--phase reverse_engineering` creates RE section.
- **QUEUE advance**: rejects `--file`; validates schema; rejects unrecognized
  domains (suggests add-domain); validates code_search_roots exist; subsequent
  advance detects unchanged file; re-validates changed file; last-domain →
  EXECUTE; domain loop advances index.
- **add-domain**: adds during QUEUE; rejects duplicate; blocked outside QUEUE.
- **Execution loop**: empty queue rejected; specs dir pre-created; per-item
  action reflects item + config; EXECUTE→POST; POST loops / →RECONCILE; failed
  item skipped without error; CWD portability.
- **Action output**: SURVEY/GAP_ANALYSIS sub-agent counts from config;
  GAP_ANALYSIS topic-of-concern rules present; EXECUTE item block; POST STOP
  message verbatim; RECONCILE lists spec files; RECONCILE_EVAL references
  `forgectl eval`.
- **Reconcile loop**: FAIL loops back (round++); PASS<min loops back; max-rounds
  with colleague_review on/off; PASS with colleague_review on/off; COLLEAGUE_
  REVIEW→RECONCILE_ADVANCE; RECONCILE_ADVANCE→next domain / DONE.
- **eval gating**: `forgectl eval` outputs the reconcile evaluator prompt during
  RECONCILE_EVAL; blocked outside it.
- **Logging**: RE advance appends an entry with domain/state detail (best-effort,
  non-fatal).

Use these as the `tests` arrays in plan items. **The plan.json validator accepts
only three test categories**: `functional`, `rejection`, `edge_case`
(`state/validate.go:201`). Map spec dimensions onto them: behavior / interface /
config / observability / invariants / integration → `functional`; rejection &
error-handling rows → `rejection`; edge cases → `edge_case`.
