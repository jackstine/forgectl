# Notes: State Types, Config Structs, Plan `kind`

Backs the L0 type/config items. All file:line references are to the codebase as of spec commit `c18fcf7`. Paths are relative to the `forgectl/` domain root.

## Decision: separate UIImplementingState (not extending BatchState)

The `ui_implementing` phase gets its **own** state structs, leaving the shipped `implementing` phase's JSON schema untouched (no serialization drift / regression risk). Confirmed with the operator.

## Phase + State constants — `state/types.go`

- `PhaseName` consts at `types.go:3-12`. Add: `PhaseUIImplementing PhaseName = "ui_implementing"`. Convention: `Phase<PascalCase>` = `snake_case`.
- `StateName` consts at `types.go:16-52`. Add 5: `StateQATest = "QA_TEST"`, `StateUIRefine = "UI_REFINE"`, `StateE2EAuthor = "E2E_AUTHOR"`, `StateE2EVerify = "E2E_VERIFY"`, `StateE2ERemediate = "E2E_REMEDIATE"`. Convention: `State<PascalCase>` = `SCREAMING_SNAKE_CASE`.
- Existing implementing-phase states reused as-is: `StateOrient`, `StateImplement`, `StateEvaluate`, `StateCommit`, `StateDone`, `StatePhaseShift`.

## Config structs — `state/types.go`

Existing reference: `ImplementingConfig` at `types.go:122-126` (`Batch`, `CommitStrategy`, `Eval EvalConfig`); `EvalConfig` at `types.go:66-72` (embeds `AgentConfig` to promote `model`/`type`/`count` to the same JSON level; has `MinRounds`, `MaxRounds`, `EnableEvalOutput`, `EvalMode`).

Add `UIImplementingConfig` and wire into `ForgeConfig` (`types.go:172-183`) as field `UIImplementing` (`json:"ui_implementing"`):

```
type UIAppConfig struct {
    LaunchCommand       string `json:"launch_command"`
    URL                 string `json:"url"`
    ReadyTimeoutSeconds int    `json:"ready_timeout_seconds"`
}
type UIE2EConfig struct {
    EvalConfig             // min/max rounds, model, type, count, eval_mode
    TestCommand string `json:"test_command"`
    TestDir     string `json:"test_dir"`
}
type UIImplementingConfig struct {
    Batch          int          `json:"batch"`
    CommitStrategy string       `json:"commit_strategy"`
    App            UIAppConfig  `json:"app"`
    Eval           EvalConfig   `json:"eval"`   // code-eval loop
    QA             EvalConfig   `json:"qa"`     // QA loop
    E2E            UIE2EConfig  `json:"e2e"`    // e2e loop + test_command/test_dir
}
```

Reuse `EvalConfig` for `Eval`/`QA` and embed it in `UIE2EConfig` so each loop carries independent `min_rounds`/`max_rounds`/`model`/`eval_mode` (spec Configuration table, `ui-batch-implementation.md` §Configuration).

## Defaults — `DefaultForgeConfig()` (`types.go:186-255`)

Add a `UIImplementing:` block mirroring `ImplementingConfig` defaults (`types.go:220-233`): `Batch:1`, `CommitStrategy:"scoped"`; `App.ReadyTimeoutSeconds:30`; each of `Eval`/`QA`/`E2E`: `MinRounds:1, MaxRounds:3, Model:"opus", Type:"eval", Count:1, EvalMode:"report"`. `App.LaunchCommand`/`App.URL`/`E2E.TestCommand`/`E2E.TestDir` default to empty (they are required and validated at init, not defaulted).

## Plan `kind` field — `state/types.go` + `state/validate.go`

- `PlanQueueEntry` (`types.go:275-287`): add `Kind string \`json:"kind,omitempty"\``. `"code"` (default/absent) or `"ui"`.
- `PlanJSON` (the `context`/`refs`/`layers`/`items` struct, ~`types.go:311+`): NOT required to carry `kind` — routing reads `kind` from the active plan-queue entry carried in state, not from plan.json. (Keep plan.json schema unchanged; `ValidatePlanJSON` is `kind`-agnostic.)
- `ValidatePlanQueue` (`validate.go:68-120`): add `"kind"` to `allowedFields` (so it is not rejected as an unexpected field), and reject any value other than `"code"`/`"ui"`/absent, naming the offending entry and value (session-init.md rejection table + "Init rejects plan queue entry with invalid kind").

## UI state structs — `state/types.go` + constructor in `state/state.go`

Mirror `ImplementingState` (`types.go:477-485`) and `BatchState`/`BatchHistory`/`LayerHistory` (`types.go:448-485`):

```
type UIBatchState struct {
    Items              []string     `json:"items"`
    CurrentItemIndex   int          `json:"current_item_index"`
    EvalRound          int          `json:"eval_round"`
    QARound            int          `json:"qa_round"`
    E2ERound           int          `json:"e2e_round"`
    Evals              []EvalRecord `json:"evals,omitempty"`      // code loop
    QAEvals            []EvalRecord `json:"qa_evals,omitempty"`
    E2EEvals           []EvalRecord `json:"e2e_evals,omitempty"`
    HandedOffArtifacts []string     `json:"handed_off_artifacts,omitempty"`
    CodeForceAccepted  bool         `json:"code_force_accepted,omitempty"`
    QAForceAccepted    bool         `json:"qa_force_accepted,omitempty"`
    E2EForceAccepted   bool         `json:"e2e_force_accepted,omitempty"`
}
type UIBatchHistory struct { /* BatchNumber + Items + per-loop *Rounds + per-loop *Evals */ }
type UILayerHistory struct { LayerID string; Batches []UIBatchHistory }
type UIImplementingState struct {
    CurrentLayer *LayerRef; BatchNumber int; CurrentBatch *UIBatchState
    LayerHistory []UILayerHistory
    CurrentPlanFile string; CurrentPlanDomain string
    PlanQueue []PlanQueueEntry
}
```

- `ForgeState` (`types.go:542` area): add `UIImplementing *UIImplementingState \`json:"ui_implementing,omitempty"\``.
- `NewUIImplementingState()` constructor next to `NewImplementingState()` (`state.go:197-201`).
- `EvalRecord` (`types.go:358-362`) reused as-is for all three eval histories.

## Tests (mirror existing patterns)

- `types_test.go` (value-equality table test, see `TestReverseEngineeringStateConstants`): assert all new phase/state string values.
- `types_config_test.go`: defaults assertions + JSON round-trip for `UIImplementingConfig` and `UIImplementingState`.
- `validate_test.go`: `kind` accepted (`code`/`ui`/absent), rejected for other values.
