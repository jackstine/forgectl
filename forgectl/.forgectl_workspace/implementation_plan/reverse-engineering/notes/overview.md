# Reverse Engineering Phase — Implementation Overview

## What this plan delivers

A new top-level lifecycle phase, `reverse_engineering`, added to the forgectl Go
CLI. It derives specifications from existing code via the state machine:

```
ORIENT
  → (SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE) per domain
  → (EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER) per queued item
  → (RECONCILE → RECONCILE_EVAL → [COLLEAGUE_REVIEW] → RECONCILE_ADVANCE) per domain
  → DONE
```

Source of truth: `forgectl/specs/reverse-engineering.md`. Supporting specs describe
how this phase threads through four already-implemented subsystems:
`session-init`, `state-persistence`, `activity-logging`, `validate-command`.

## Scope boundary — READ THIS FIRST (for the evaluator)

forgectl **already implements** the `specifying`, `planning`, and `implementing`
phases end-to-end. The four supporting specs (session-init, state-persistence,
activity-logging, validate-command) are **already satisfied for those three
phases**. This plan covers ONLY the *delta* required to add the
`reverse_engineering` phase. Concretely, that means:

- **session-init**: add `reverse_engineering` as a 4th initializable phase + its
  init-input schema. The init machinery (project-root discovery, config load,
  config lock, session_id, log pruning) already exists and is reused unchanged.
- **state-persistence**: add a `reverse_engineering` state section to the state
  file struct. Atomic writes, backup, startup recovery, archiving already exist
  and are reused unchanged.
- **activity-logging**: ensure `advance` in the new phase emits log entries with
  domain/state detail. The JSONL logger, naming, pruning, best-effort guarantee
  already exist and are reused unchanged.
- **validate-command**: the new RE validators are written as shared functions
  (reused by init + the QUEUE advance), exactly as the existing 3 validators are
  shared between `init`, phase shifts, and `validate`. Per the decision recorded
  for this plan, the `validate` *command surface* is NOT extended with a new
  `reverse-engineering-queue` type — the RE queue's top-level key `"specs"`
  collides with spec-queue auto-detection and validate-command.md's concrete
  interface defines only `spec-queue`, `plan-queue`, `plan`. The "same validation
  logic" integration point is satisfied by the shared validator functions.

Do not flag the existing 3-phase behavior as missing; it is implemented and out
of scope. Evaluate the items below against the `reverse_engineering` additions.

## Codebase facts (confirmed by reading the source)

- Module `forgectl`, Go 1.21.6. Deps: cobra 1.10.2, BurntSushi/toml 1.6.0,
  google/uuid 1.6.0.
- `cmd/` — Cobra commands; each file self-registers via `init()` →
  `rootCmd.AddCommand`. Existing: init.go, advance.go, status.go, eval.go,
  validate.go, addqueueitem.go, setroots.go, root.go (no version.go; version is
  on root). **No** `adddomain.go`. (File names have no separators.)
- `state/` — **types.go** (phase consts, state consts, ALL config structs,
  `DefaultForgeConfig`, input/queue/plan schemas, every phase state struct,
  `ForgeState`, `AdvanceInput`), **config.go** (TOML mirror structs +
  `mergeTomlConfig` + `FindProjectRoot` + `GenerateSessionID` + `ValidateConfig`),
  **advance.go** (`Advance()` dispatch + `advanceSpecifying/Planning/Implementing`
  + `detectCycle` lives in validate.go), **validate.go** (`ValidateSpecQueue`,
  `ValidatePlanQueue`, `ValidatePlanJSON`, schema strings), **output.go**
  (`PrintAdvanceOutput`, per-phase `print*Output`, `PrintStatus`,
  `PrintReconcileEvalOutput`, `PrintEvalOutput`, `PrintCrossRefEvalOutput`),
  **state.go** (`Load/Save/Exists/Archive`, `New*State` constructors),
  **logger.go** (`NewLogger`, `LogEntry`, `Write`, `PruneLogs`, `LogNow`),
  **git.go** (`AutoCommit`, staging strategies). There is **no** phase.go,
  states.go, transitions.go, log.go, or eval.go in state/.
- `evaluators/` — **evaluators.go** (`//go:embed` one constant per `.md`),
  spec-eval.md, plan-eval.md, impl-eval.md, cross-reference-eval.md, and
  **reconcile-eval.md (a ~427-byte STUB)** that must be completed with the
  7-dimension checklist.
- Grep for `reverse`, `SURVEY`, `concept`, `colleague`, `add-domain` across all
  three packages returns **zero** matches (outside the spec): this phase is
  entirely greenfield.

## Pattern to imitate throughout: the `planning` phase

Every new piece has a direct analogue in the existing `planning` phase — use it
as the template:

| New work | Mirror this existing element |
|----------|------------------------------|
| `PhaseReverseEngineering` const | `PhasePlanning` (`state/types.go:6-11`) |
| RE state constants | the `StateName` const block (`state/types.go:16-40`) |
| `ReverseEngineeringState` struct | `PlanningState` (`state/types.go:392`) |
| `ReverseEngineeringConfig` struct + defaults | `PlanningConfig` (`state/types.go:98`), `DefaultForgeConfig` (`:154`) |
| TOML decode + merge for RE config | `tomlPlanningConfig` + `mergeTomlConfig` (`state/config.go`) |
| `NewReverseEngineeringState()` | `NewPlanningState()` (`state/state.go`) |
| `advanceReverseEngineering()` | `advancePlanning()` (`state/advance.go`) |
| `ValidateReverseEngineering*` | `ValidatePlanQueue`/`ValidatePlanJSON` (`state/validate.go`) |
| `printReverseEngineeringOutput()` | `printPlanningOutput()` (`state/output.go:381`) |
| RE reconcile-eval rendering | `PrintReconcileEvalOutput()` (`state/output.go`) |

## Path conventions for THIS plan.json

- `refs[].path` is resolved by `filepath.Join(planDir, path)` and `os.Stat`'d
  (`state/validate.go:148-151`). planDir =
  `forgectl/.forgectl_workspace/implementation_plan/reverse-engineering/`.
  So note refs use `notes/<name>.md` and the files MUST exist before VALIDATE.
- `items[].files` / `items[].spec` resolve against the **project root**
  (`forgectl/state/...`, `forgectl/specs/...`) and item files are NOT existence
  checked (they may not exist yet — they are outputs).
