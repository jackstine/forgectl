# forgectl State File Schema

> Defines the JSON structure of `forgectl-state.json` --- the persistent state file that drives the spec generation workflow.
> See `forgectl-state-example.json` for a concrete example.

---

## File Location

- **File name:** `forgectl-state.json`
- **Location:** `<project_root>/<config.paths.state_dir>/forgectl-state.json` (default: `.forgectl/state/forgectl-state.json`)
- **Created by:** `forgectl init`
- **Backup:** `forgectl-state.json.bak` (previous state, atomic write with crash recovery)

---

## Configuration

All configuration comes from `.forgectl/config` (TOML) and is locked into the state file's `config` object at init time. The config is not re-read from the TOML file after init --- the state file's `config` is the single source of truth for the session.

The only mutable config value is `config.general.user_guided`, which can be toggled via `--guided` / `--no-guided` on any `advance` call.

See the nested `config` structure in the ForgeState table below.

---

## Top-Level: ForgeState

| Field | Type | Description |
|-------|------|-------------|
| `phase` | `string` | Active phase: `"specifying"`, `"generate_planning_queue"`, `"planning"`, or `"implementing"` |
| `state` | `string` | Current state within the phase (see State Names below) |
| `config` | `ForgeConfig` | Full project configuration locked at init. See Config section. |
| `session_id` | `string` | UUID v4 generated at init, stable for session lifetime |
| `started_at_phase` | `string` | The phase selected at `init` time |
| `phase_shift` | `object \| null` | Present only during a phase transition |
| `specifying` | `object \| null` | Specifying phase state (populated when `phase` = `"specifying"`) |
| `generate_planning_queue` | `object \| null` | Generate planning queue phase state. Null when skipped or not yet reached. |
| `planning` | `object \| null` | Planning phase state |
| `implementing` | `object \| null` | Implementing phase state |

---

## Config: ForgeConfig

| Field | Type | Description |
|-------|------|-------------|
| `general.enable_commits` | `bool` | Whether scaffold auto-commits at commit points (default: `false`) |
| `general.enable_eval_output` | `bool` | Whether eval sub-agents write report files (default: `false`) |
| `general.user_guided` | `bool` | Whether guided pauses are active. Mutable via `--guided`/`--no-guided`. |
| `domains` | `DomainConfig[]` | Optional. Configured domains with `name` and `path`. |
| `specifying.batch` | `int` | Specs per specifying cycle, domain-grouped (default: 1) |
| `specifying.commit_strategy` | `string` | Git staging strategy: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `all-specs`) |
| `specifying.eval.*` | `EvalConfig` | Eval round limits and agent config for specifying |
| `specifying.cross_reference.*` | `CrossRefConfig` | Cross-reference round limits, agent config, and user_review flag |
| `specifying.reconciliation.*` | `ReconciliationConfig` | Reconciliation round limits and agent config |
| `planning.batch` | `int` | Plans per planning cycle (default: 1) |
| `planning.commit_strategy` | `string` | Git staging strategy (default: `strict`) |
| `planning.self_review` | `bool` | Whether SELF_REVIEW state is entered (default: `false`) |
| `planning.plan_all_before_implementing` | `bool` | When `true`: all planning then all implementing (default: `false`) |
| `planning.study_code.*` | `AgentConfig` | Agent config for codebase exploration |
| `planning.eval.*` | `EvalConfig` | Eval round limits and agent config for planning |
| `planning.refine.*` | `AgentConfig` | Agent config for plan refinement |
| `implementing.batch` | `int` | Plan items per implementing batch (default: 1) |
| `implementing.commit_strategy` | `string` | Git staging strategy (default: `scoped`) |
| `implementing.eval.*` | `EvalConfig` | Eval round limits and agent config for implementing |
| `paths.state_dir` | `string` | State file directory (default: `.forgectl/state`) |
| `paths.workspace_dir` | `string` | Domain artifact directory name (default: `.forge_workspace`) |
| `logs.enabled` | `bool` | Whether activity logging is active (default: `true`) |
| `logs.retention_days` | `int` | Log file age limit for pruning (default: `90`) |
| `logs.max_files` | `int` | Maximum log file count for pruning (default: `50`) |

---

## State Names

### Specifying Phase

```
ORIENT -> SELECT -> DRAFT -> EVALUATE <-> REFINE -> ACCEPT -> (next batch or domain cross-reference)
ACCEPT (domain complete) -> CROSS_REFERENCE -> CROSS_REFERENCE_EVAL <-> CROSS_REFERENCE -> CROSS_REFERENCE_REVIEW -> (next domain or DONE)
DONE -> RECONCILE -> RECONCILE_EVAL <-> RECONCILE -> RECONCILE_REVIEW -> COMPLETE
COMPLETE -> PHASE_SHIFT
```

| State | Description |
|-------|-------------|
| `ORIENT` | Read plans and existing specs. Build mental model. |
| `SELECT` | Pull next batch from queue (domain-grouped). If guided, discuss with user. |
| `DRAFT` | Write the spec files for the batch. |
| `EVALUATE` | Spawn evaluator sub-agent. Record verdict and eval report. |
| `REFINE` | Fix deficiencies found during evaluation. |
| `ACCEPT` | Batch finalized. Next batch -> ORIENT. Domain complete -> CROSS_REFERENCE. |
| `CROSS_REFERENCE` | Cross-reference all specs within the completed domain. |
| `CROSS_REFERENCE_EVAL` | Sub-agent evaluates intra-domain cross-reference consistency. |
| `CROSS_REFERENCE_REVIEW` | Review cross-reference eval. Add specs or set code search roots. |
| `DONE` | All individual specs and domain cross-references complete. Advance to begin reconciliation. |
| `RECONCILE` | Fix cross-references across all specs and domains. Stage files. |
| `RECONCILE_EVAL` | Sub-agent evaluates cross-domain consistency. |
| `RECONCILE_REVIEW` | Human reviews reconciliation eval. Accept or grant another pass. |
| `COMPLETE` | Specifying phase fully done. Auto-commits when `enable_commits` is true. |
| `PHASE_SHIFT` | Transitioning to the next phase (specifying -> generate_planning_queue). |

### Generate Planning Queue Phase

```
ORIENT -> REFINE -> PHASE_SHIFT
```

| State | Description |
|-------|-------------|
| `ORIENT` | Auto-generate plan queue from completed specs. Write to `<state_dir>/plan-queue.json`. |
| `REFINE` | Architect reviews, reorders, edits the plan queue file. Validates on advance. |
| `PHASE_SHIFT` | Transitioning to planning. `--from` override available. |

### Planning Phase

```
ORIENT -> STUDY_SPECS -> STUDY_CODE -> STUDY_PACKAGES -> REVIEW -> DRAFT -> VALIDATE -> SELF_REVIEW* -> EVALUATE <-> REFINE -> ACCEPT -> (next or DONE)
DONE -> PHASE_SHIFT

* SELF_REVIEW only entered when planning.self_review is true.
```

| State | Description |
|-------|-------------|
| `ORIENT` | Begin studying the plan. |
| `STUDY_SPECS` | Study spec files and git diffs. |
| `STUDY_CODE` | Explore codebase with sub-agents. |
| `STUDY_PACKAGES` | Study technical stack. |
| `REVIEW` | Review findings. Guided pause. |
| `DRAFT` | Write the plan. |
| `VALIDATE` | Validate plan.json structure. |
| `SELF_REVIEW` | Self-review checkpoint (only when `planning.self_review` is true). |
| `EVALUATE` | Spawn evaluator sub-agent. Record verdict and eval report. |
| `REFINE` | Fix deficiencies found during evaluation. |
| `ACCEPT` | Plan accepted. Auto-commits when `enable_commits` is true. |
| `DONE` | All plans complete. |
| `PHASE_SHIFT` | Transitioning to implementing. |

### Implementing Phase

```
ORIENT -> IMPLEMENT -> EVALUATE <-> IMPLEMENT -> COMMIT -> ORIENT | DONE
```

| State | Description |
|-------|-------------|
| `ORIENT` | Selects batch of unblocked items. Guided pause. |
| `IMPLEMENT` | Implement the current item. |
| `EVALUATE` | Spawn evaluator sub-agent for the batch. |
| `COMMIT` | Commit implemented items. |
| `DONE` | All layers complete. Terminal state. |

---

## Specifying Phase State: `specifying`

| Field | Type | Description |
|-------|------|-------------|
| `current_specs` | `ActiveSpec[] \| null` | The spec batch being worked on. Array of specs, not a single object. Null between batches. |
| `current_domain` | `string` | The domain currently being processed. |
| `batch_number` | `int` | Current batch number within the domain. |
| `domains` | `object` | Per-domain metadata (e.g., `code_search_roots` from set-roots). |
| `cross_reference` | `object` | Per-domain cross-reference round and eval history. |
| `queue` | `SpecQueueEntry[]` | Remaining specs to process. |
| `completed` | `CompletedSpec[]` | Specs that have been accepted. |
| `reconcile` | `ReconcileState \| null` | Reconciliation state. Populated after all specs and cross-references reach DONE. |

---

### ActiveSpec

| Field | Type | Description |
|-------|------|-------------|
| `id` | `int` | Unique ID, increments from 1. |
| `name` | `string` | Human-readable spec name. |
| `domain` | `string` | Domain this spec belongs to. |
| `topic` | `string` | One-sentence topic of concern. |
| `file` | `string` | Path where the spec file is written. |
| `domain_path` | `string` | Filesystem path to the domain directory. Optional. |
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
| `domain_path` | `string` | Filesystem path to the domain directory. Optional. |
| `batch_number` | `int` | Which batch this spec was part of. Optional. |
| `rounds_taken` | `int` | Total rounds before acceptance. |
| `commit_hashes` | `string[]` | Git commit hashes registered via auto-commit at COMPLETE when `enable_commits: true`. Optional. |
| `evals` | `EvalRecord[]` | Complete evaluation history. Optional. |

---

### EvalRecord

| Field | Type | Description |
|-------|------|-------------|
| `round` | `int` | Round number. |
| `verdict` | `string` | `"PASS"` or `"FAIL"`. |
| `eval_report` | `string` | Path to evaluation report file. Optional (omitted when `enable_eval_output: false`). |

---

### ReconcileState

| Field | Type | Description |
|-------|------|-------------|
| `round` | `int` | Reconciliation round counter. |
| `evals` | `EvalRecord[]` | Reconciliation evaluation history. Optional. |

---

### CrossReferenceState

| Field | Type | Description |
|-------|------|-------------|
| `domain` | `string` | Domain being cross-referenced. |
| `round` | `int` | Cross-reference round counter. |
| `evals` | `EvalRecord[]` | Cross-reference evaluation history. Optional. |

---

### GeneratePlanningQueueState

| Field | Type | Description |
|-------|------|-------------|
| `plan_queue_file` | `string` | Path to the auto-generated plan-queue.json. |

---

## PhaseShiftInfo

| Field | Type | Description |
|-------|------|-------------|
| `from` | `string` | Source phase. |
| `to` | `string` | Target phase. |

---

## Input File: Spec Queue

The `--from` file for `forgectl init --phase specifying`.

### Validation Rules

- `specs` must be a non-empty array.
- All 6 fields are required on every entry.
- No additional fields are allowed.
- Every value in `depends_on` must match a `name` in another entry.

---

## CLI Commands That Mutate State

| Command | What it does |
|---------|-------------|
| `forgectl init --from <file>` | Creates the state file from an input file and `.forgectl/config`. Optional `--phase` (default: specifying). |
| `forgectl advance` | Transitions to the next state. Flags vary by current state. |
| `forgectl advance --verdict PASS\|FAIL --eval-report <path>` | Record an evaluation verdict (EVALUATE states). |
| `forgectl advance --verdict FAIL --eval-report <path>` | Fail an evaluation -> REFINE. |
| `forgectl advance --file <path>` | Override spec file path (DRAFT state only). |
| `forgectl advance --message "msg"` | Commit message (COMPLETE and ACCEPT commit points when `enable_commits: true`). |
| `forgectl advance --guided` / `--no-guided` | Toggle guided mode (accepted on any advance). |
| `forgectl advance --from <path>` | Plan queue input file (PHASE_SHIFT from specifying or generate_planning_queue). |
| `forgectl add-queue-item --name <name> --topic <topic> --file <file>` | Append a spec to the queue (DRAFT, CROSS_REFERENCE_REVIEW, DONE, RECONCILE_REVIEW). |
| `forgectl set-roots <path> [<path>...]` | Set code search roots for a domain (CROSS_REFERENCE_REVIEW, DONE). |
| `forgectl status` | Read-only. Print current state, action guidance, and progress summary. |
