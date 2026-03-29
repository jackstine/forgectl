# forge.json Schema (State File)

> Persistent state file created and managed by forgectl.
> Written atomically (tmpfile → backup → rename) for crash recovery.
> Located in the directory specified by `--dir` (default: current directory).

---

## Root: ForgeState

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `phase` | string | **yes** | Current phase: `"specifying"`, `"planning"`, or `"implementing"` |
| `state` | string | **yes** | Current state within the phase (see State Values below) |
| `batch_size` | int | **yes** | Max items per batch during implementing |
| `min_rounds` | int | **yes** | Minimum evaluation rounds (>= 1) |
| `max_rounds` | int | **yes** | Maximum evaluation rounds (>= min_rounds) |
| `user_guided` | bool | **yes** | Whether session pauses for user discussion at key states |
| `started_at_phase` | string | **yes** | Phase selected at `forgectl init` time |
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
| `queue` | SpecQueueEntry[] | Remaining specs to process. |
| `completed` | CompletedSpec[] | Specs that have been accepted. |
| `reconcile` | ReconcileState | Reconciliation state. Populated when state = DONE. |

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
| `commit_hash` | string | Single commit hash (optional) |
| `commit_hashes` | string[] | Multiple commit hashes (optional) |
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
| `topic` | string | Topic description |
| `file` | string | Path to plan.json |
| `specs` | string[] | Spec file paths |
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
3. Empty arrays and null objects are omitted from JSON (`omitempty`).
4. File is serialized with 2-space indentation.
5. Atomic write: tmpfile → backup (`.bak`) → rename. Recovery reads `.bak` if primary is corrupt.

---

## Source

- Type definitions: `forgectl/state/types.go`
- Persistence: `forgectl/state/state.go`
- Transitions: `forgectl/state/advance.go`
- Existing docs: `skills/specs/references/forgectl-state-schema.md`
