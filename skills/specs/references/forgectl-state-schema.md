# forgectl State File Schema

> Defines the JSON structure of `forgectl-state.json` — the persistent state file that drives the spec generation workflow.
> See `forgectl-state-example.json` for a concrete example.

---

## File Location

- **File name:** `forgectl-state.json`
- **Created by:** `forgectl init`
- **Backup:** `forgectl-state.json.bak` (previous state, atomic write with crash recovery)

---

## Top-Level: ForgeState

| Field | Type | Description |
|-------|------|-------------|
| `phase` | `string` | Active phase: `"specifying"`, `"planning"`, or `"implementing"` |
| `state` | `string` | Current state within the phase (see State Names below) |
| `batch_size` | `int` | Max items per batch |
| `min_rounds` | `int` | Minimum evaluation rounds per spec (default 1) |
| `max_rounds` | `int` | Maximum evaluation rounds per spec |
| `user_guided` | `bool` | Whether the session pauses for user input at key states |
| `started_at_phase` | `string` | The phase selected at `init` time |
| `phase_shift` | `object \| null` | Present only during a phase transition |
| `specifying` | `object \| null` | Specifying phase state (populated when `phase` = `"specifying"`) |
| `planning` | `object \| null` | Planning phase state |
| `implementing` | `object \| null` | Implementing phase state |

---

## State Names

### Specifying Phase

```
ORIENT → SELECT → DRAFT → EVALUATE ⇄ REFINE → ACCEPT → (next spec or DONE)
DONE → RECONCILE → RECONCILE_EVAL → RECONCILE_REVIEW → COMPLETE
COMPLETE → PHASE_SHIFT
```

| State | Description |
|-------|-------------|
| `ORIENT` | Read plans and existing specs. Build mental model. |
| `SELECT` | Pull next spec from queue. If guided, discuss with user. |
| `DRAFT` | Write the spec file. |
| `EVALUATE` | Spawn evaluator sub-agent. Record verdict and eval report. |
| `REFINE` | Fix deficiencies found during evaluation. |
| `ACCEPT` | Spec finalized. Next spec → ORIENT. Empty queue → DONE. |
| `DONE` | All individual specs complete. Advance to begin reconciliation. |
| `RECONCILE` | Fix cross-references across all specs. Stage files. |
| `RECONCILE_EVAL` | Sub-agent evaluates cross-spec consistency. |
| `RECONCILE_REVIEW` | Human reviews reconciliation eval. Accept or grant another pass. |
| `COMPLETE` | Session fully done. |
| `PHASE_SHIFT` | Transitioning to the next phase (specifying → planning). |

### Planning Phase

```
ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT → VALIDATE → EVALUATE ⇄ REFINE → ACCEPT → PHASE_SHIFT
```

### Implementing Phase

```
ORIENT → IMPLEMENT → EVALUATE ⇄ IMPLEMENT → COMMIT → ORIENT | DONE
```

---

## Specifying Phase State: `specifying`

| Field | Type | Description |
|-------|------|-------------|
| `current_spec` | `ActiveSpec \| null` | Spec currently being drafted/evaluated. Null between specs. |
| `queue` | `SpecQueueEntry[]` | Remaining specs to process. |
| `completed` | `CompletedSpec[]` | Specs that have been accepted. |
| `reconcile` | `ReconcileState \| null` | Reconciliation state. Populated after all specs reach DONE. |

---

### ActiveSpec

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | Unique ID, increments from 1. |
| `name` | `string` | Human-readable spec name. |
| `domain` | `string` | Domain this spec belongs to. |
| `topic` | `string` | One-sentence topic of concern. |
| `file` | `string` | Path where the spec file is written. |
| `planning_sources` | `string[]` | Paths to planning documents this spec is derived from. |
| `depends_on` | `string[]` | Names of other specs this one depends on. |
| `round` | `int` | Current evaluation round. Increments on REFINE. |
| `evals` | `EvalRecord[]` | History of all evaluation rounds. Optional, omitted when empty. |

---

### SpecQueueEntry

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Human-readable spec name. |
| `domain` | `string` | Domain this spec belongs to. |
| `topic` | `string` | One-sentence topic of concern. |
| `file` | `string` | Target path for the spec file. |
| `planning_sources` | `string[]` | Paths to reference material. Required, can be empty. |
| `depends_on` | `string[]` | Names of other specs that must be written first. Required, can be empty. |

---

### CompletedSpec

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | ID from when the spec was active. |
| `name` | `string` | Spec name. |
| `domain` | `string` | Domain. |
| `file` | `string` | Path to the spec file. |
| `rounds_taken` | `int` | Total rounds before acceptance. |
| `commit_hash` | `string` | Single commit hash. Optional. |
| `commit_hashes` | `string[]` | Multiple commit hashes. Optional. |
| `evals` | `EvalRecord[]` | Complete evaluation history. Optional. |

---

### EvalRecord

| Field | Type | Description |
|-------|------|-------------|
| `round` | `int` | Round number. |
| `verdict` | `string` | `"PASS"` or `"FAIL"`. |
| `eval_report` | `string` | Path to evaluation report file. Optional. |

---

### ReconcileState

| Field | Type | Description |
|-------|------|-------------|
| `round` | `int` | Reconciliation round counter. |
| `evals` | `EvalRecord[]` | Reconciliation evaluation history. Optional. |

---

## PhaseShiftInfo

| Field | Type | Description |
|-------|------|-------------|
| `from` | `string` | Source phase. |
| `to` | `string` | Target phase. |

---

## Input File: Spec Queue

The `--from` file for `forgectl init --phase specifying`. This is the same schema defined in `spec-queue-from-plan.md`.

### Validation Rules

- `specs` must be a non-empty array.
- All 6 fields are required on every entry.
- No additional fields are allowed.
- Every value in `depends_on` must match a `name` in another entry.

---

## CLI Commands That Mutate State

| Command | What it does |
|---------|-------------|
| `forgectl init --from <file> --batch-size <n> --max-rounds <n>` | Creates the state file from a spec queue. |
| `forgectl advance` | Transitions to the next state. Flags vary by current state. |
| `forgectl advance --verdict PASS --eval-report <path> --message "msg"` | Accept an evaluation (EVALUATE state). |
| `forgectl advance --verdict FAIL --eval-report <path>` | Fail an evaluation → REFINE. |
| `forgectl advance --file <path>` | Override spec file path (DRAFT state only). |
| `forgectl add-commit --id <n> --hash <hash>` | Register a commit to a completed spec. |
| `forgectl reconcile-commit --hash <hash>` | Auto-register a commit to all specs it touched. |
| `forgectl status` | Read-only. Print current state and session overview. |
