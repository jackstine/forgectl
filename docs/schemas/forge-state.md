# forgectl-state.json Schema (State File)

> Persistent state file created and managed by forgectl.
> Written atomically (tmpfile → backup → rename) for crash recovery.
> Located at: `.forgectl/state/forgectl-state.json` (relative to project root).

---

## Root: ForgeState

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | **yes** | UUID v4, generated at init, never changes for the session lifetime |
| `phase` | string | **yes** | Current phase: `"specifying"`, `"generate_planning_queue"`, `"planning"`, `"implementing"`, `"ui_implementing"`, or `"reverse_engineering"` |
| `state` | string | **yes** | Current state within the phase (see State Values below) |
| `started_at_phase` | string | **yes** | Phase selected at `forgectl init` time |
| `config` | ConfigObject | **yes** | Configuration nested from .forgectl/config (TOML) |
| `phase_shift` | PhaseShiftInfo | no | Present only during PHASE_SHIFT state |
| `specifying` | SpecifyingState | no | Non-null when phase = `"specifying"` |
| `generate_planning_queue` | GeneratePlanningQueueState | no | Non-null when phase = `"generate_planning_queue"` |
| `planning` | PlanningState | no | Non-null when phase = `"planning"`, `"implementing"`, or `"ui_implementing"` (holds the `current_plan.file` reference) |
| `implementing` | ImplementingState | no | Non-null when phase = `"implementing"` |
| `ui_implementing` | UIImplementingState | no | Non-null when phase = `"ui_implementing"` |

Two things deliberately absent from this schema:

- **The inline batch-commit notice.** `ForgeState.InlineBatchCommit` exists in Go but is tagged `json:"-"` and never serialized. It describes the single `advance` that just ran, so the output can report the commit before rendering the state it landed on; persisting it would make the next command's output claim a commit that already happened.
- **Planning readiness.** The gate is stateless — its verdict is a pure function of the incoming plan queue's domains and the current filesystem. It records no marker, no "already planned" flag, and no verdict history, so nothing about it appears in the state file.

---

## State Values by Phase

### Specifying
`ORIENT` → `SELECT` → `DRAFT` → `EVALUATE` ⇄ `REFINE` → `ACCEPT` → `DONE` → `RECONCILE` → `RECONCILE_EVAL` → `RECONCILE_REVIEW` → `COMPLETE` → `PHASE_SHIFT`

### Generate Planning Queue
`ORIENT` → `REFINE` → `PHASE_SHIFT`

### Planning
`ORIENT` → `STUDY_SPECS` → `STUDY_CODE` → `STUDY_PACKAGES` → `REVIEW` → `DRAFT` → `VALIDATE` → `SELF_REVIEW`* → `EVALUATE` ⇄ `REFINE` → `ACCEPT` → `DONE` → `PHASE_SHIFT`

*SELF_REVIEW only entered when `planning.self_review: true`.

### Implementing
`ORIENT` → `IMPLEMENT` → `EVALUATE` ⇄ `IMPLEMENT` → [`COMMIT`] → `ORIENT` | `DONE`

`COMMIT` appears only when `general.enable_commits` is `false`, and performs no git operation. When commits are enabled, the terminal `EVALUATE` advance commits the batch inline and lands directly on `ORIENT` | `DONE`.

Under `implementing.eval.eval_mode: "direct"` the ⇄ back-edge targets `EVALUATE`, not `IMPLEMENT`: the evaluator corrects the files itself, so `IMPLEMENT` runs once per batch and every non-terminal verdict re-enters `EVALUATE` (carrying the round increment).

### UI Implementing
Each batch passes through three sequential verification loops — code-eval, QA, then e2e — before the batch-terminal commit:

`ORIENT` → `IMPLEMENT` → `EVALUATE` ⇄ `IMPLEMENT` → `QA_TEST` ⇄ `UI_REFINE` → `E2E_AUTHOR` → `E2E_VERIFY` ⇄ `E2E_REMEDIATE` → [`COMMIT`] → `ORIENT` | `DONE`

`COMMIT` again appears only when `general.enable_commits` is `false`; with commits enabled the terminal `E2E_VERIFY` advance commits the batch inline. The code-eval loop's `direct`-mode back-edge targets `EVALUATE` exactly as in the implementing phase; the QA and e2e loops keep `UI_REFINE` and `E2E_REMEDIATE` in every `eval_mode`.

- `EVALUATE` ⇄ `IMPLEMENT`: the code-eval loop (shared with the implementing phase), bounded by `ui_implementing.eval.{min,max}_rounds`.
- `QA_TEST` ⇄ `UI_REFINE`: the QA placement loop, bounded by `ui_implementing.qa.{min,max}_rounds`. A PASS (rounds ≥ min) advances to `E2E_AUTHOR`; a FAIL routes to `UI_REFINE` and back.
- `E2E_AUTHOR` → `E2E_VERIFY` ⇄ `E2E_REMEDIATE`: the e2e loop, bounded by `ui_implementing.e2e.{min,max}_rounds`. A zero-scenario step list short-circuits the loop at its minimum rounds.

The three evaluator states (`EVALUATE`, `QA_TEST`, `E2E_VERIFY`) accept `--verdict` and are the only states valid for the `eval` and `handoff` commands.

---

## ConfigObject

| Field | Type | Description |
|-------|------|-------------|
| `domains` | DomainConfig[] | Optional. Configured domains. Empty array if none configured. |
| `specifying` | SpecifyingPhaseConfig | Specifying-phase config |
| `planning` | PlanningPhaseConfig | Planning-phase config |
| `implementing` | ImplementingPhaseConfig | Implementing-phase config |
| `ui_implementing` | UIImplementingPhaseConfig | UI-implementing-phase config |
| `paths` | PathsConfig | File path configuration |
| `general` | GeneralConfig | Global config (enable_commits, user_guided) |
| `logs` | LogsConfig | Logging configuration |

### DomainConfig

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Domain name |
| `path` | string | Domain directory path relative to project root |

### LogsConfig

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable logging |
| `retention_days` | int | Number of days to retain log files |
| `max_files` | int | Maximum number of log files to keep |

### SpecifyingPhaseConfig

| Field | Type | Description |
|-------|------|-------------|
| `batch` | int | Specs per specifying cycle (domain-grouped) |
| `commit_strategy` | string | Git staging strategy: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `all-specs`) |
| `eval` | AgentEvalConfig | Evaluation settings |
| `cross_reference` | CrossReferenceConfig | Cross-reference settings |
| `reconciliation` | ReconciliationConfig | Reconciliation settings |

### PlanningPhaseConfig

| Field | Type | Description |
|-------|------|-------------|
| `batch` | int | Plans per planning cycle (>1 not yet supported, reserved for future use) |
| `commit_strategy` | string | Git staging strategy: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `strict`) |
| `self_review` | bool | Whether SELF_REVIEW state is entered between validation and EVALUATE (default: `false`) |
| `plan_all_before_implementing` | bool | When `false` (default): interleaved plan-implement per domain. When `true`: all planning then all implementing. |
| `study_specs` | AgentConfig | Agent config for spec study (sharded across sub-agents) |
| `study_code` | AgentConfig | Agent config for codebase exploration |
| `eval` | AgentEvalConfig | Evaluation settings |
| `refine` | AgentConfig | Agent config for plan refinement |

### ImplementingPhaseConfig

| Field | Type | Description |
|-------|------|-------------|
| `batch` | int | Plan items per implementing batch |
| `commit_strategy` | string | Git staging strategy: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `scoped`) |
| `eval` | AgentEvalConfig | Evaluation settings |

### UIImplementingPhaseConfig

Configures the `ui_implementing` phase. The code-eval (`eval`), QA (`qa`), and e2e (`e2e`) loops each carry an independent round budget and eval mode.

| Field | Type | Description |
|-------|------|-------------|
| `batch` | int | Plan items per UI-implementing batch |
| `commit_strategy` | string | Git staging strategy: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `scoped`) |
| `app` | UIAppConfig | How the QA loop reaches the running application |
| `eval` | AgentEvalConfig | Code-eval loop settings (the `EVALUATE` ⇄ `IMPLEMENT` loop) |
| `qa` | AgentEvalConfig | QA loop settings (the `QA_TEST` ⇄ `UI_REFINE` loop) |
| `e2e` | UIE2EConfig | e2e verification loop settings (the `E2E_VERIFY` ⇄ `E2E_REMEDIATE` loop) |

#### UIAppConfig

`launch_command` and `url` are **required** and validated non-empty at the phase boundary (`init --phase ui_implementing` or the phase shift into `ui_implementing`); they are not defaulted.

| Field | Type | Description |
|-------|------|-------------|
| `launch_command` | string | Shell command that starts the application (e.g. `npm run dev`) |
| `url` | string | URL the QA sub-agent drives through the Playwright MCP (e.g. `http://localhost:5173`) |
| `ready_timeout_seconds` | int | How long to wait for the app to become reachable before QA begins |

#### UIE2EConfig

Extends AgentEvalConfig (the embedded `min_rounds`/`max_rounds`/`model`/`type`/`count`/`eval_mode`) with the authored suite's run command and directory. `test_command` and `test_dir` are **required** and validated non-empty at the phase boundary; they are not defaulted.

| Field | Type | Description |
|-------|------|-------------|
| `min_rounds` / `max_rounds` / `model` / `type` / `count` / `eval_mode` | — | As in AgentEvalConfig, scoped to the e2e loop |
| `test_command` | string | Command that runs the authored Playwright test suite (e.g. `npm run e2e`) |
| `test_dir` | string | Directory the authored test files live in (e.g. `e2e/`) |

### AgentConfig

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Model name (e.g., `"opus"`, `"haiku"`) |
| `type` | string | Agent role (e.g., `"eval"`, `"explore"`, `"refine"`) |
| `count` | int | Number of agent instances to spawn |

### AgentEvalConfig

Extends AgentConfig with additional evaluation fields:

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Model name |
| `type` | string | Agent role |
| `count` | int | Number of agent instances to spawn |
| `min_rounds` | int | Minimum evaluation rounds (>= 1) |
| `max_rounds` | int | Maximum evaluation rounds (>= min_rounds) |
| `eval_mode` | string | How eval findings are recorded and applied: `"report"` (write a report file; default for new sessions), `"direct"` (sub-agent edits files in place), or `"conversational"` (verbal verdict, no file). An empty value resolves via back-compat (see below). |
| `enable_eval_output` | bool | **Legacy / back-compat.** Superseded by `eval_mode`. When `eval_mode` is empty, `enable_eval_output: true` (here or in `[general]`) resolves to `"report"`, otherwise `"conversational"`. An explicit `eval_mode` always wins (default: `false`). |

**eval_mode resolution (`EvalModeFor`):** an explicit `eval_mode` wins; if empty, `enable_eval_output` (per-eval or `[general]`) → `"report"` when true, else `"conversational"`. Valid values are `"report"`, `"direct"`, `"conversational"`; an empty string is accepted (resolved by back-compat). Reconciliation and cross-reference evaluation inherit the mode from `specifying.eval`.

### CrossReferenceConfig

| Field | Type | Description |
|-------|------|-------------|
| `min_rounds` | int | Min rounds for cross-reference evaluation |
| `max_rounds` | int | Max rounds for cross-reference evaluation |
| `model` | string | Model name |
| `type` | string | Agent role |
| `count` | int | Number of agent instances to spawn |
| `user_review` | bool | Whether user review is required |
| `eval` | AgentConfig | Agent config for cross-reference evaluation |

### ReconciliationConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `min_rounds` | int | 0 | Min rounds for spec reconciliation |
| `max_rounds` | int | 3 | Max rounds for spec reconciliation |
| `model` | string | — | Model name |
| `type` | string | — | Agent role |
| `count` | int | — | Number of agent instances to spawn |
| `user_review` | bool | false | Whether user review is required |

### PathsConfig

| Field | Type | Description |
|-------|------|-------------|
| `state_dir` | string | State file directory (default: `.forgectl/state`) |
| `workspace_dir` | string | Domain artifact directory name (default: `.forge_workspace`) |

### GeneralConfig

| Field | Type | Description |
|-------|------|-------------|
| `enable_commits` | bool | If true, the scaffold stages and commits automatically. `--message` is required only at specifying's COMPLETE and planning's ACCEPT; the implementing / ui_implementing per-item and batch commits synthesize their own message and treat `--message` as optional extra context. If false, no git operation occurs anywhere and `--message` is ignored with a warning. |
| `user_guided` | bool | Runtime user_guided override |

---

## PhaseShiftInfo

| Field | Type | Description |
|-------|------|-------------|
| `from` | string | Source phase |
| `to` | string | Target phase |

---

## SpecifyingState

| Field | Type | Description |
|-------|------|-------------|
| `current_specs` | ActiveSpec[] | Spec batch being drafted/evaluated. Null between specs. |
| `domains` | object | Per-domain metadata. Each key is a domain name. |
| `queue` | SpecQueueEntry[] | Remaining specs to process. |
| `completed` | CompletedSpec[] | Specs that have been accepted. |
| `reconcile` | ReconcileState | Reconciliation state. Populated when state = DONE. |

### Domains Object

Each key in `domains` is a domain name, value is:

| Field | Type | Description |
|-------|------|-------------|
| `code_search_roots` | string[] | Root directories for code search, set via `set-roots` command |

### ActiveSpec

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Unique ID, increments from 1 |
| `name` | string | Spec name |
| `domain` | string | Domain |
| `topic` | string | Topic of concern |
| `file` | string | Path to spec file |
| `planning_sources` | string[] | Planning document paths |
| `depends_on` | string[] | Names of dependent specs |
| `round` | int | Current eval round (starts at 1) |
| `evals` | EvalRecord[] | Evaluation history |

### CompletedSpec

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | ID from when the spec was active |
| `name` | string | Spec name |
| `domain` | string | Domain |
| `file` | string | Path to spec file |
| `rounds_taken` | int | Total eval rounds before acceptance |
| `commit_hashes` | string[] | Git commit hashes registered via auto-commit at COMPLETE when `enable_commits: true`, or overwritten via `set-commit-hashes` (optional, array only) |
| `evals` | EvalRecord[] | Evaluation history (optional) |

### ReconcileState

| Field | Type | Description |
|-------|------|-------------|
| `round` | int | Reconciliation round counter (starts at 0) |
| `evals` | EvalRecord[] | Reconciliation evaluation history |

---

## GeneratePlanningQueueState

| Field | Type | Description |
|-------|------|-------------|
| `plan_queue_file` | string | Path to the auto-generated plan queue file (`<state_dir>/plan-queue.json`) |

---

## PlanningState

| Field | Type | Description |
|-------|------|-------------|
| `current_plan` | ActivePlan | Plan being worked on. Null after acceptance. |
| `round` | int | Current eval round (starts at 1) |
| `evals` | EvalRecord[] | Evaluation history |
| `queue` | PlanQueueEntry[] | Remaining plans to process |
| `completed` | object[] | Completed plans |

### ActivePlan

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Unique ID, increments from 1 |
| `name` | string | Plan name |
| `domain` | string | Domain |
| `file` | string | Path to plan.json |
| `specs` | string[] | Spec file paths |
| `spec_commits` | string[] | Git commit hashes from spec phase |
| `code_search_roots` | string[] | Directories for code exploration |
| `kind` | string | Implementation target carried from the plan-queue entry: `"ui"` routes the phase shift to ui_implementing; `"code"`/absent to implementing (optional) |

### PlanQueueEntry

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Unique ID, increments from 1 |
| `name` | string | Plan name |
| `domain` | string | Domain |
| `file` | string | Path to plan.json |
| `specs` | string[] | Spec file paths |
| `spec_commits` | string[] | Git commit hashes from spec phase |
| `code_search_roots` | string[] | Directories for code exploration |
| `kind` | string | `"code"` (default) or `"ui"` — routes the implementation phase shift (optional) |

---

## ImplementingState

| Field | Type | Description |
|-------|------|-------------|
| `plan_queue` | PlanRef[] | Plans remaining to implement. Populated when `plan_all_before_implementing: true`. Each entry has `domain`, `name`, `file`. |
| `completed_plans` | PlanRef[] | Plans that have been fully implemented. |
| `current_plan` | PlanRef | The plan currently being implemented (`domain`, `name`, `file`). |
| `current_layer` | LayerRef | Current layer being worked on |
| `batch_number` | int | Incremental batch counter across all layers |
| `current_batch` | BatchState | Current batch of items |
| `layer_history` | LayerHistory[] | Completed layers with batch histories |

### PlanRef

| Field | Type | Description |
|-------|------|-------------|
| `domain` | string | Domain name |
| `name` | string | Plan display name |
| `file` | string | Path to plan.json |

### LayerRef

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Layer ID from plan.json |
| `name` | string | Layer name from plan.json |

### BatchState

| Field | Type | Description |
|-------|------|-------------|
| `items` | string[] | Item IDs in this batch |
| `current_item_index` | int | Index of current item (0-based) |
| `eval_round` | int | Evaluation round counter for this batch |
| `evals` | EvalRecord[] | Batch evaluation history |

### LayerHistory

| Field | Type | Description |
|-------|------|-------------|
| `layer_id` | string | ID of completed layer |
| `batches` | BatchHistory[] | Batches processed in this layer |

### BatchHistory

| Field | Type | Description |
|-------|------|-------------|
| `batch_number` | int | Batch number at completion |
| `items` | string[] | Item IDs |
| `eval_rounds` | int | Total eval rounds for this batch |
| `evals` | EvalRecord[] | Evaluation history |

---

## UIImplementingState

Mirrors ImplementingState but with UI-specific batch/history types so each batch can track three independent verification loops. Non-null when `phase = "ui_implementing"`.

| Field | Type | Description |
|-------|------|-------------|
| `current_layer` | LayerRef | Current layer being worked on |
| `batch_number` | int | Incremental batch counter across all layers |
| `current_batch` | UIBatchState | Current batch of items (tracks all three loops) |
| `layer_history` | UILayerHistory[] | Completed layers with batch histories |
| `current_plan_file` | string | Path to the active plan.json (optional) |
| `current_plan_domain` | string | Domain of the active plan (optional) |
| `plan_queue` | PlanQueueEntry[] | Plans remaining to implement when `plan_all_before_implementing: true` (optional) |

### UIBatchState

Tracks the current UI batch across the code-eval, QA, and e2e loops. The three loops never share a round namespace; each counter is reset to 0 at ORIENT and incremented per the transition table.

| Field | Type | Description |
|-------|------|-------------|
| `items` | string[] | Item IDs in this batch |
| `current_item_index` | int | Index of current item (0-based) |
| `eval_round` | int | Round counter for the code-eval loop |
| `qa_round` | int | Round counter for the QA loop |
| `e2e_round` | int | Round counter for the e2e loop |
| `evals` | EvalRecord[] | Code-eval loop history (optional) |
| `qa_evals` | EvalRecord[] | QA loop history (optional) |
| `e2e_evals` | EvalRecord[] | e2e loop history (optional) |
| `handed_off_artifacts` | string[] | Files the current evaluator round's sub-agent handed off for review via `handoff`; latest hand-off replaces prior (optional) |
| `code_force_accepted` | bool | Set when the code-eval loop exhausts `eval.max_rounds` on a FAIL (optional) |
| `qa_force_accepted` | bool | Set when the QA loop exhausts `qa.max_rounds` on a FAIL (optional) |
| `e2e_force_accepted` | bool | Set when the e2e loop exhausts `e2e.max_rounds` on a FAIL (optional) |

At the batch-terminal boundary — the `COMMIT` state when `enable_commits: false`, or the inline auto-commit on the terminal `E2E_VERIFY` advance when `enable_commits: true` — items are marked failed iff any of the three force-accept flags is set; otherwise they pass.

### UILayerHistory

| Field | Type | Description |
|-------|------|-------------|
| `layer_id` | string | ID of completed layer |
| `batches` | UIBatchHistory[] | Batches processed in this layer |

### UIBatchHistory

Records the three per-loop round totals and histories, feeding the DONE summary and `status --verbose`.

| Field | Type | Description |
|-------|------|-------------|
| `batch_number` | int | Batch number at completion |
| `items` | string[] | Item IDs |
| `eval_rounds` | int | Total code-eval rounds for this batch |
| `qa_rounds` | int | Total QA rounds for this batch |
| `e2e_rounds` | int | Total e2e rounds for this batch |
| `evals` | EvalRecord[] | Code-eval history (optional) |
| `qa_evals` | EvalRecord[] | QA history (optional) |
| `e2e_evals` | EvalRecord[] | e2e history (optional) |

---

## Shared: EvalRecord

| Field | Type | Description |
|-------|------|-------------|
| `round` | int | Round number |
| `verdict` | string | `"PASS"` or `"FAIL"` |
| `eval_report` | string | Path to evaluation report file (optional) |

---

## Key Invariants

1. Only one phase state object is active based on `phase` value.
2. `planning` remains non-null during implementing and ui_implementing (holds `current_plan.file` reference).
3. `config` mirrors `.forgectl/config` (TOML) structure at state init time; persisted in state for audit trail.
4. `commit_hashes` (array) on completed specs contain hashes registered via auto-commit at COMPLETE when `enable_commits: true`, or set directly via the `set-commit-hashes` command.
5. `.workspace` renamed to `.forge_workspace` throughout.
6. Empty arrays and null objects are omitted from JSON (`omitempty`).
7. File is serialized with 2-space indentation.
8. Atomic write: tmpfile → backup (`.bak`) → rename. Recovery reads `.bak` if primary is corrupt.
9. Phase sections that haven't been reached yet are `null` in the state file. The `generate_planning_queue` section is `null` when skipped via `--from` at specifying PHASE_SHIFT or when starting at `--phase planning`.

---

## Source

- Type definitions: `forgectl/state/types.go`
- Persistence: `forgectl/state/state.go`
- Transitions: `forgectl/state/advance.go`
- Config loading: `forgectl/state/config.go`
- Location: `.forgectl/state/forgectl-state.json`
