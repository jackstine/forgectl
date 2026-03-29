# forgectl-state.json Schema (State File)

> Persistent state file created and managed by forgectl.
> Written atomically (tmpfile → backup → rename) for crash recovery.
> Located at: `.forgectl/state/forgectl-state.json` (relative to project root).

---

## Root: ForgeState

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | **yes** | UUID v4, generated at init, never changes for the session lifetime |
| `phase` | string | **yes** | Current phase: `"specifying"`, `"planning"`, or `"implementing"` |
| `state` | string | **yes** | Current state within the phase (see State Values below) |
| `started_at_phase` | string | **yes** | Phase selected at `forgectl init` time |
| `config` | ConfigObject | **yes** | Configuration nested from .forgectl/config (TOML) |
| `phase_shift` | PhaseShiftInfo | no | Present only during PHASE_SHIFT state |
| `specifying` | SpecifyingState | no | Non-null when phase = `"specifying"` |
| `planning` | PlanningState | no | Non-null when phase = `"planning"` or `"implementing"` |
| `implementing` | ImplementingState | no | Non-null when phase = `"implementing"` |

---

## State Values by Phase

### Specifying
`ORIENT` → `SELECT` → `DRAFT` → `EVALUATE` ⇄ `REFINE` → `ACCEPT` → `DONE` → `RECONCILE` → `RECONCILE_EVAL` → `RECONCILE_REVIEW` → `COMPLETE` → `PHASE_SHIFT`

### Planning
`ORIENT` → `STUDY_SPECS` → `STUDY_CODE` → `STUDY_PACKAGES` → `REVIEW` → `DRAFT` → `VALIDATE` → `EVALUATE` ⇄ `REFINE` → `ACCEPT` → `PHASE_SHIFT`

### Implementing
`ORIENT` → `IMPLEMENT` → `EVALUATE` ⇄ `IMPLEMENT` → `COMMIT` → `ORIENT` | `DONE`

---

## ConfigObject

| Field | Type | Description |
|-------|------|-------------|
| `specifying` | PhaseConfig | Specifying-phase config (batch, eval.min_rounds, eval.max_rounds, reconciliation.*) |
| `planning` | PhaseConfig | Planning-phase config |
| `implementing` | PhaseConfig | Implementing-phase config |
| `general` | GeneralConfig | Global config (enable_commits, user_guided) |
| `logs` | LogsConfig | Logging configuration |

### LogsConfig

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable logging |
| `retention_days` | int | Number of days to retain log files |
| `max_files` | int | Maximum number of log files to keep |

### PhaseConfig

| Field | Type | Description |
|-------|------|-------------|
| `batch` | int | Batch size for this phase |
| `eval` | EvalConfig | Evaluation settings |
| `reconciliation` | ReconciliationConfig | Reconciliation settings (specifying only) |

### EvalConfig

| Field | Type | Description |
|-------|------|-------------|
| `min_rounds` | int | Minimum evaluation rounds (>= 1) |
| `max_rounds` | int | Maximum evaluation rounds (>= min_rounds) |

### ReconciliationConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `min_rounds` | int | 0 | Min rounds for spec reconciliation |
| `max_rounds` | int | 3 | Max rounds for spec reconciliation |

### GeneralConfig

| Field | Type | Description |
|-------|------|-------------|
| `enable_commits` | bool | If true, --message required at ACCEPT/COMMIT states; auto git commit if enabled. If false, --message optional. |
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
| `current_spec` | ActiveSpec | Spec being drafted/evaluated. Null between specs. |
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
| `commit_hashes` | string[] | Git commit hashes related to this spec (optional, array only) |
| `evals` | EvalRecord[] | Evaluation history (optional) |

### ReconcileState

| Field | Type | Description |
|-------|------|-------------|
| `round` | int | Reconciliation round counter (starts at 0) |
| `evals` | EvalRecord[] | Reconciliation evaluation history |

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

---

## ImplementingState

| Field | Type | Description |
|-------|------|-------------|
| `current_layer` | LayerRef | Current layer being worked on |
| `batch_number` | int | Incremental batch counter across all layers |
| `current_batch` | BatchState | Current batch of items |
| `layer_history` | LayerHistory[] | Completed layers with batch histories |

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

## Shared: EvalRecord

| Field | Type | Description |
|-------|------|-------------|
| `round` | int | Round number |
| `verdict` | string | `"PASS"` or `"FAIL"` |
| `eval_report` | string | Path to evaluation report file (optional) |

---

## Key Invariants

1. Only one phase state object is active based on `phase` value.
2. `planning` remains non-null during implementing (holds `current_plan.file` reference).
3. `config` mirrors `.forgectl/config` (TOML) structure at state init time; persisted in state for audit trail.
4. `commit_hash` (singular) removed; only `commit_hashes` (array) used.
5. `.workspace` renamed to `.forge_workspace` throughout.
6. Empty arrays and null objects are omitted from JSON (`omitempty`).
7. File is serialized with 2-space indentation.
8. Atomic write: tmpfile → backup (`.bak`) → rename. Recovery reads `.bak` if primary is corrupt.

---

## Source

- Type definitions: `forgectl/state/types.go`
- Persistence: `forgectl/state/state.go`
- Transitions: `forgectl/state/advance.go`
- Config loading: TODO (not yet implemented)
- Location: `.forgectl/state/forgectl-state.json`
