# Notes — Output Rendering (`state/output.go`)

`output.go` is ~2900 lines. Entry point `PrintAdvanceOutput` (`output.go:41`) dispatches by phase, with `StatePhaseShift` intercepted uniformly at `output.go:58` before the phase switch.

Phase printers, each a `switch s.State`:

| Function | Location |
| --- | --- |
| `printSpecifyingOutput` | `output.go:278` |
| `printGeneratePlanningQueueOutput` | `output.go:627` |
| `printPlanningOutput` | `output.go:653` |
| `printImplementingOutput` | `output.go:838` |
| `printUIImplementingOutput` | `output.go:1225` |
| `printPhaseShiftOutput` | `output.go:1745` |
| `printReverseEngineeringOutput` | `output.go:1811` |

## Shared writers

- `writeEvalEntryAction` (`output.go:85`) — renders the evaluator-entry action per `eval_mode`.
- `writeRefineBody` (`output.go:124`) — refinement instructions per mode.
- `writeImplementReviewReminders` (`output.go:148`) — the spec/refs review reminders in IMPLEMENT actions. Spec invariant: the **spec review reminder is unconditional on every round**; the **refs reminder is gated on refs being present**.
- `writeEvalTrailingSections` (`output.go:172`) — `--- PREVIOUS EVALUATIONS ---` and `--- REPORT OUTPUT ---`.

Deterministic report paths are centralized as single-source-of-truth helpers: `implEvalReportPath` (`output.go:211`), `planEvalReportPath` (`217`), `specEvalReportPath` (`223`), `crossRefEvalReportPath` (`230`), `reconcileEvalReportPath` (`248`), `qaStepListPath` (`1188`), `qaReportPath` (`1192`), `e2eReportPath` (`1196`).

## What this change touches

### IMPLEMENT action text

Implementing IMPLEMENT is rendered inside `printImplementingOutput`; ui IMPLEMENT inside `printUIImplementingOutput`. Any wording that presents `--message` as required on the first round must become optional-with-appending, and must no longer be first-round-conditional now that every round commits.

### COMMIT state rendering

Implementing COMMIT: `output.go:1102-1133`. UI COMMIT: `output.go:1590-1623`. Both currently branch on `enable_commits` — "Advance with `--message`" when true, "Commit your changes before continuing." when false.

Because COMMIT is now only reachable when `enable_commits: false`, the `true` arm of each becomes dead. The `false` arm is the only surviving rendering.

### Terminal EVALUATE / E2E_VERIFY → ORIENT or DONE

When `enable_commits: true` the terminal transition now lands directly on ORIENT (or DONE) and the *inline commit already happened*. The ORIENT rendering should not imply the operator still owes a commit, and — per the spec's output examples — the terminal transition is where the operator learns the batch was committed. This is the one place new output is genuinely added rather than edited.

### EVALUATE re-entry under direct mode

`batch-implementation.md:228-244` gives the worked example for **Entering EVALUATE (implementing phase, subsequent round, `eval_mode: "direct"`)**. It carries the incremented `Round:` and a `Note:` line:

```
Note:     FAIL recorded for round 1. Corrections were made directly to batch files.
```

Neither `printImplementingOutput`'s nor `printUIImplementingOutput`'s `case StateEvaluate` emits any `Note:` line today. That is not an oversight — the rendering is currently **unreachable**, because nothing ever re-enters EVALUATE from EVALUATE. The direct-mode transition change makes it reachable for the first time, so the rendering has to be written alongside it.

The `Note:` text reports the *prior* round's verdict, so it must be absent on a batch's first EVALUATE entry and present on every direct-mode re-entry, in both the implementing and the ui code-eval loops.

### `preflight` has no state output

`preflight` prints its own two output forms directly (see `readiness-gate.md`); it is not part of the `PrintAdvanceOutput` dispatch and needs no changes here.

## Testing the output

`state/output_test.go` exercises these printers directly. `setItemPassesOnDisk` (`output_test.go:1871`) is the helper for seeding a plan file with per-item pass state — useful for driving COMMIT/ORIENT renderings.

Integration golden files live under `forgectl/integration/` with an `updateGolden` flag (`harness_test.go:42`); output wording changes will require regenerating affected goldens.
