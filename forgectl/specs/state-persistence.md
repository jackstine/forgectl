# State Persistence

## Topic of Concern
> The scaffold persists session state through atomic file writes with crash recovery on startup.

## Context

Every forgectl command reads and writes a single JSON state file (`forgectl-state.json`). The state file lives in the directory specified by `config.paths.state_dir` (default: `.forgectl/state/`). The file must survive process crashes, power loss, and interrupted writes without data loss. A backup file and write-ahead pattern provide crash safety. On startup, the scaffold detects and recovers from interrupted writes before any command logic runs.

The scaffold discovers the project root by walking up the directory hierarchy from the current working directory until it finds a `.forgectl/` directory. This is identical to how git discovers `.git/`.

Completed sessions are archived to a git-tracked subdirectory within the state directory for audit purposes.

All scaffold output is to stdout. The scaffold writes state changes to `forgectl-state.json` and (during implementing phase) updates `passes` and `rounds` fields in plan.json.

## Depends On
- None. State persistence is a foundational concern with no upstream dependencies.

## Integration Points

| Spec | Relationship |
|------|-------------|
| session-init | Creates the initial state file; persistence layer provides the write mechanism |
| spec-lifecycle | Reads and mutates specifying phase state through the persistence layer |
| spec-reconciliation | Reads and mutates reconciliation state through the persistence layer |
| plan-production | Reads and mutates planning phase state and plan.json through the persistence layer |
| batch-implementation | Reads and mutates implementing phase state and plan.json through the persistence layer |
| activity-logging | `session_id` in state file root is used to name the session log file |
| validate-command | Standalone validation command; does not use the persistence layer (no session required) |
| reverse-engineering | Reads and mutates reverse_engineering phase state through the persistence layer |

---

## Interface

### Inputs

The persistence layer is internal — no CLI inputs. All commands trigger reads and writes through it.

The `status` command reads and displays the state file:

| Command | Flags | Description |
|---------|-------|-------------|
| `status` | `--verbose` / `-v` (optional) | Print current state with action guidance and progress summary. With `--verbose`, appends full session overview (queue, completed, prior phases, item-by-item detail). |

### Outputs

#### File Layout

```
<project_root>/
├── .forgectl/
│   ├── config                                  ← TOML project configuration (user-created)
│   └── state/                                  ← default state_dir
│       ├── forgectl-state.json                 ← active state (gitignored)
│       ├── forgectl-state.json.bak             ← previous state (gitignored)
│       ├── forgectl-state.json.tmp             ← write-in-progress (transient, gitignored)
│       └── sessions/                           ← archived completed sessions (git tracked)
├── <domain>/
│   ├── .forge_workspace/                       ← domain artifacts (plans, notes)
│   │   └── implementation_plan/
│   └── specs/
└── ...
```

#### Project Root Discovery

The scaffold resolves the project root by walking up from the current directory:

1. Check current directory for `.forgectl/`
2. Check parent directory
3. Continue until `.forgectl/` is found or filesystem root is reached
4. If not found: error — "No .forgectl directory found."

All relative paths in the state file and config resolve from the project root.

#### `status` Output

The `status` command prints the current state, action guidance, and a one-line progress summary. The session header includes session file, phase, state, and configuration. The current state's action guidance matches the `advance` output for that state.

With `--verbose` (`-v`), `status` additionally prints the full session overview: queue contents, completed items with eval history, and prior phase summaries. Each phase that has been reached contributes a section in verbose mode.

**Flags:**

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Show full session overview (queue, completed, prior phases, item-by-item detail) |

**Mid-specifying:**

```
Session: .forgectl/state/forgectl-state.json
Phase:   specifying
State:   REFINE
Config:  batch=3, rounds=1-3, guided=true

Batch:   Repository Loading, Snapshot Diffing, Portal Rendering (optimizer)
Round:   1/3

Action:  Read the eval report and address any findings in the spec files
         using the spec skill.
         Eval report: optimizer/specs/.eval/batch-1-r1.md
         Format:      references/spec-format.md
         Process:     references/spec-generation-skill.md
         Scoping:     references/topic-of-concern.md
         After completion of the above, advance to continue evaluation.

Progress: 1/5 specs completed, 2 queued
```

**Mid-planning:**

```
Session: .forgectl/state/forgectl-state.json
Phase:   planning
State:   EVALUATE
Config:  batch=1, rounds=1-3

Plan:    Service Configuration (launcher)
File:    launcher/.forge_workspace/implementation_plan/plan.json
Round:   1/3

Action:  Run evaluation sub-agent against the plan (round 1/3).
         Sub-agent: forgectl eval
         Advance with --verdict PASS|FAIL --eval-report <path>.

Progress: round 1 of 3
```

**Mid-implementing:**

```
Session: .forgectl/state/forgectl-state.json
Phase:   implementing
State:   IMPLEMENT
Config:  batch=2, rounds=1-3

Item:    [daemon.io] PID file I/O operations (2 of 2)
Layer:   L1 Core (2/5 layers)
Round:   0

Action:  Implement this item.
         After completion of the above, advance to continue.

Progress: 3/5 passed, 0 failed, 2 remaining
```

**Started at implementing directly (`--phase implementing`):**

```
Session: .forgectl/state/forgectl-state.json
Phase:   implementing (started here)
State:   EVALUATE
Config:  batch=2, rounds=1-3

Layer:   L0 Foundation (1/3 layers)
Batch:   1/2
Round:   1/3
Items:   [config.types], [config.load]

Action:  Ask the evaluation sub-agent to verify batch items against their tests.
         The sub-agent should run: forgectl eval
         After reviewing the eval report, run:
           forgectl advance --eval-report <path> --verdict PASS|FAIL

Progress: 0/5 passed, 0 failed, 5 remaining
```

**Verbose mode (`status --verbose` or `status -v`) adds sections after the progress line:**

The verbose output appends the full session overview below the compact output. For mid-implementing, this includes:

```
--- Specifying ---

  Complete (5 specs)

--- Planning ---

  Accepted (2 rounds)
    Round 1: FAIL — launcher/.forge_workspace/implementation_plan/evals/round-1.md
    Round 2: PASS — launcher/.forge_workspace/implementation_plan/evals/round-2.md

--- Implementing ---

  Layer L0 (Foundation): in progress
    [bootstrap]     passed  (1 round)
    [config.types]  done    (1 round)
    [config.load]   done    (1 round)
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance` or `status` or `eval` called before `init` | Error. Exit code 1. | State file must exist |

---

## Behavior

### Atomic Writes

Every state mutation follows this sequence:
1. Write new state to `forgectl-state.json.tmp`.
2. Rename `forgectl-state.json` → `forgectl-state.json.bak`.
3. Rename `forgectl-state.json.tmp` → `forgectl-state.json`.

Steps 2 and 3 are filesystem renames — atomic on POSIX. If the process crashes between steps, the startup recovery logic handles it.

### Startup Recovery

On every command (before reading state), the scaffold checks within the configured `state_dir`:

| Condition | Action |
|-----------|--------|
| `.json` exists, `.tmp` does not | Normal. Proceed. |
| `.json` missing, `.bak` exists | Crashed between step 2 and 3. Rename `.bak` → `.json`. Warn user. |
| `.json` missing, `.tmp` exists | Crashed between step 1 and 2. Rename `.tmp` → `.json`. Warn user. |
| `.json` exists, `.tmp` exists | Crashed after step 1, before cleanup. Delete `.tmp`. Proceed with `.json`. |
| `.json` corrupt (invalid JSON) | Rename `.json` → `.json.corrupt`, rename `.bak` → `.json`. Warn user. |
| None exist | No state. Only `init` is valid. |

### Session Archiving

Completed session state files are archived to a permanent directory within `state_dir`:

```
.forgectl/state/sessions/
├── optimizer-2026-03-15.json
├── launcher-2026-03-21.json
└── ...
```

- The active `forgectl-state.json` is gitignored (ephemeral working state).
- Archived sessions are committed to git (permanent audit trail).
- Naming convention: `<domain>-<date>.json`.
- Archive before starting a new session. The active state file must be deleted (or the scaffold will reject `init`).

### State File Schema

```json
{
  "phase": "implementing",
  "state": "IMPLEMENT",
  "config": {
    "domains": [
      { "name": "optimizer", "path": "optimizer" },
      { "name": "portal", "path": "portal" }
    ],
    "specifying": {
      "batch": 3,
      "commit_strategy": "all-specs",
      "eval": { "min_rounds": 1, "max_rounds": 3, "model": "opus", "type": "eval", "count": 1, "enable_eval_output": false },
      "cross_reference": {
        "min_rounds": 1,
        "max_rounds": 2,
        "model": "haiku",
        "type": "explore",
        "count": 3,
        "user_review": false,
        "eval": { "model": "opus", "type": "eval", "count": 1 }
      },
      "reconciliation": { "min_rounds": 0, "max_rounds": 3, "model": "opus", "type": "eval", "count": 1, "user_review": false }
    },
    "planning": {
      "batch": 1,
      "commit_strategy": "strict",
      "self_review": false,
      "plan_all_before_implementing": false,
      "study_code": { "model": "haiku", "type": "explore", "count": 3 },
      "eval": { "min_rounds": 1, "max_rounds": 3, "model": "opus", "type": "eval", "count": 1, "enable_eval_output": false },
      "refine": { "model": "opus", "type": "refine", "count": 1 }
    },
    "implementing": {
      "batch": 2,
      "commit_strategy": "scoped",
      "eval": { "min_rounds": 1, "max_rounds": 3, "model": "opus", "type": "eval", "count": 1, "enable_eval_output": false }
    },
    "paths": {
      "state_dir": ".forgectl/state",
      "workspace_dir": ".forge_workspace"
    },
    "general": {
      "user_guided": true,
      "enable_commits": false
    },
    "logs": {
      "enabled": true,
      "retention_days": 90,
      "max_files": 50
    }
  },
  "session_id": "a3f1b2c4-7e2d-4f01-b4c8-e312d9f01234",
  "started_at_phase": "specifying",

  "specifying": {
    "current_specs": null,
    "queue": [],
    "completed": [
      {
        "id": 1,
        "name": "Configuration Models",
        "domain": "optimizer",
        "file": "optimizer/specs/configuration-models.md",
        "rounds_taken": 2,
        "commit_hashes": ["a1b2c3d"],
        "evals": [
          { "round": 1, "verdict": "FAIL", "eval_report": "optimizer/specs/.eval/configuration-models-r1.md" },
          { "round": 2, "verdict": "PASS", "eval_report": "optimizer/specs/.eval/configuration-models-r2.md" }
        ]
      }
    ],
    "domains": {
      "optimizer": {
        "code_search_roots": ["optimizer/", "lib/shared/"]
      }
    },
    "reconcile": {
      "round": 1,
      "evals": [
        { "round": 1, "verdict": "PASS" }
      ]
    }
  },

  "planning": {
    "current_plan": {
      "id": 1,
      "name": "Launcher Implementation Plan",
      "domain": "launcher",
      "file": "launcher/.forge_workspace/implementation_plan/plan.json",
      "specs": ["launcher/specs/service-configuration.md"],
      "spec_commits": ["7cede10", "8743b1d"],
      "code_search_roots": ["launcher/"]
    },
    "round": 2,
    "evals": [
      { "round": 1, "verdict": "FAIL", "eval_report": "launcher/.forge_workspace/implementation_plan/evals/round-1.md" },
      { "round": 2, "verdict": "PASS", "eval_report": "launcher/.forge_workspace/implementation_plan/evals/round-2.md" }
    ],
    "queue": [],
    "completed": []
  },

  "implementing": {
    "current_layer": { "id": "L0", "name": "Foundation" },
    "batch_number": 2,
    "current_batch": {
      "items": ["config.types", "config.load"],
      "current_item_index": 0,
      "eval_round": 0,
      "evals": []
    },
    "layer_history": [
      {
        "layer_id": "L0",
        "batches": [
          {
            "batch_number": 1,
            "items": ["bootstrap"],
            "eval_rounds": 1,
            "evals": [
              { "round": 1, "verdict": "PASS", "eval_report": "launcher/.forge_workspace/implementation_plan/evals/batch-1-round-1.md" }
            ]
          }
        ]
      }
    ]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | `specifying`, `generate_planning_queue`, `planning`, or `implementing` |
| `state` | string | Current state within the active phase |
| `started_at_phase` | string | Which phase the session was initialized at (for display) |
| **Config** | | Mirrors `.forgectl/config` TOML structure. See `docs/configurations.md`. |
| `config.domains` | array | Optional. Configured domains with `name` and `path`. Empty array if none configured. |
| `config.domains[].name` | string | Domain name |
| `config.domains[].path` | string | Domain directory path relative to project root |
| `config.specifying.batch` | integer | Specs per specifying cycle (domain-grouped) |
| `config.specifying.commit_strategy` | string | Git staging strategy for specifying commits: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `all-specs`) |
| `config.specifying.eval.enable_eval_output` | boolean | Whether eval sub-agents write report files (default: `false`) |
| `config.specifying.eval.*` | object | Eval round limits and agent config for specifying |
| `config.specifying.cross_reference.*` | object | Cross-reference round limits, agent config, and user_review flag |
| `config.specifying.cross_reference.eval.*` | object | Agent config for cross-reference evaluation |
| `config.specifying.reconciliation.*` | object | Reconciliation round limits and agent config |
| `config.planning.batch` | integer | Plans per planning cycle (>1 not yet supported, reserved for future use) |
| `config.planning.commit_strategy` | string | Git staging strategy for planning commits: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `strict`) |
| `config.planning.self_review` | boolean | Whether SELF_REVIEW state is entered between validation and EVALUATE (default: `false`) |
| `config.planning.plan_all_before_implementing` | boolean | When `false` (default): interleaved plan-implement per domain. When `true`: all planning then all implementing. |
| `config.planning.study_code.*` | object | Agent config for codebase exploration |
| `config.planning.eval.enable_eval_output` | boolean | Whether eval sub-agents write report files (default: `false`) |
| `config.planning.eval.*` | object | Eval round limits and agent config for planning |
| `config.planning.refine.*` | object | Agent config for plan refinement |
| `config.implementing.batch` | integer | Plan items per implementing batch |
| `config.implementing.commit_strategy` | string | Git staging strategy for implementing commits: `strict`, `all-specs`, `scoped`, `tracked`, `all` (default: `scoped`) |
| `config.implementing.eval.enable_eval_output` | boolean | Whether eval sub-agents write report files (default: `false`) |
| `config.implementing.eval.*` | object | Eval round limits and agent config for implementing |
| `config.paths.state_dir` | string | State file directory |
| `config.paths.workspace_dir` | string | Domain artifact directory name |
| `config.general.user_guided` | boolean | Whether guided pauses are active |
| `config.general.enable_commits` | boolean | Whether scaffold auto-commits at commit points. See `docs/auto-committing.md`. |
| `config.logs.enabled` | boolean | Whether activity logging is active |
| `config.logs.retention_days` | integer | Log file age limit for pruning |
| `config.logs.max_files` | integer | Maximum log file count for pruning |
| `session_id` | string | UUID v4 generated at init, stable for session lifetime |
| **Specifying** | | |
| `specifying.current_specs` | array/null | The spec batch being worked on |
| `specifying.queue` | array | Remaining specs (each with `domain_path`) |
| `specifying.completed` | array | Finished specs with eval history and commit hashes |
| `specifying.completed[].domain_path` | string | Filesystem path to the domain directory |
| `specifying.completed[].commit_hashes` | string[] | Git commit hashes registered via auto-commit at COMPLETE when `enable_commits: true` |
| `specifying.completed[].evals` | array | Full eval trail per spec |
| `specifying.cross_reference` | object | Per-domain cross-reference round and eval history |
| `specifying.domains` | object | Per-domain metadata (code_search_roots from set-roots) |
| `specifying.domains[<domain>].code_search_roots` | string[] | Code search roots set via set-roots command |
| `specifying.reconcile` | object | Reconciliation round and eval history |
| **Generate Planning Queue** | | |
| `generate_planning_queue.plan_queue_file` | string | Path to the auto-generated plan queue file (`<state_dir>/plan-queue.json`) |
| **Planning** | | |
| `planning.current_plan` | object/null | The plan being worked on |
| `planning.round` | integer | Current planning eval round |
| `planning.evals` | array | Planning eval history |
| `planning.queue` | array | Remaining plans |
| `planning.completed` | array | Finished plans |
| **Implementing** | | |
| `implementing.plan_queue` | array | Plans remaining to implement (populated when `plan_all_before_implementing: true`). Each entry has `domain`, `name`, `file`. |
| `implementing.completed_plans` | array | Plans that have been implemented. |
| `implementing.current_plan` | object | The plan currently being implemented (`domain`, `name`, `file`). |
| `implementing.current_layer` | object | Active layer |
| `implementing.batch_number` | integer | Global batch counter (1-indexed) |
| `implementing.current_batch` | object | Active batch state |
| `implementing.current_batch.items` | string[] | Item IDs in batch |
| `implementing.current_batch.current_item_index` | integer | 0-based index |
| `implementing.current_batch.eval_round` | integer | Current eval round for batch |
| `implementing.current_batch.evals` | array | Batch eval history |
| `implementing.layer_history` | array | Completed batches and layers |

Phase sections that haven't been reached yet are `null` in the state file. When starting at a later phase (`--phase planning`), earlier phase sections remain `null`. The `generate_planning_queue` section is `null` when skipped via `--from` at specifying PHASE_SHIFT or when starting at `--phase planning`.

---

## Invariants

1. **Phase is authoritative.** The `phase` field determines which states are valid and how shared state names behave.
2. **State file is durable.** Atomic writes with backup prevent corruption. Startup recovery handles interrupted writes.
3. **Eval history is append-only.** Eval records accumulate and are never deleted or modified.
4. **Config locked at init.** The `config` object is written at init and not re-read from `.forgectl/config` during the session. Only `config.general.user_guided` is mutable (via `--guided`/`--no-guided` on `advance`).

---

## Edge Cases

- **Scenario:** Process crashes after writing `.tmp` but before renaming `.bak`.
  - **Expected:** On next startup, `.tmp` is renamed to `.json`. State recovered.
  - **Rationale:** The `.tmp` file contains the most recent intended state; recovering it preserves the latest mutation.

- **Scenario:** State file contains invalid JSON but `.bak` is intact.
  - **Expected:** Corrupt file renamed to `.json.corrupt`, `.bak` restored as `.json`. Warning printed.
  - **Rationale:** Rolling back to the previous known-good state is safer than failing. The corrupt file is preserved for debugging.

- **Scenario:** Both `.json` and `.tmp` exist.
  - **Expected:** `.tmp` deleted, `.json` used as-is.
  - **Rationale:** The `.json` file was not yet replaced, so it holds the last committed state. The orphaned `.tmp` is from an incomplete write.

- **Scenario:** `state_dir` is an absolute path.
  - **Expected:** State files are read/written at that absolute path.
  - **Rationale:** Allows shared or external state storage.

- **Scenario:** `state_dir` is a relative path.
  - **Expected:** Resolved relative to the project root (directory containing `.forgectl/`).
  - **Rationale:** Relative paths are portable across machines.

---

## Testing Criteria

### Recovery from crash between backup and rename
- **Verifies:** Startup recovery handles missing `.json` with existing `.bak`.
- **Given:** `.json` missing, `.bak` exists in `state_dir`.
- **When:** Any command runs.
- **Then:** `.bak` renamed to `.json`. Warning printed. Command proceeds.

### Recovery from corrupt state file
- **Verifies:** Startup recovery handles corrupt JSON with valid backup.
- **Given:** `.json` contains invalid JSON, `.bak` exists in `state_dir`.
- **When:** Any command runs.
- **Then:** `.json` renamed to `.json.corrupt`. `.bak` renamed to `.json`. Warning printed.

### State file created in configured state_dir
- **Verifies:** State file respects `paths.state_dir` config.
- **Given:** `.forgectl/config` with `state_dir = ".forgectl/state"`.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** State file created at `<project_root>/.forgectl/state/forgectl-state.json`.

### Project root discovery from subdirectory
- **Verifies:** Directory hierarchy walk finds `.forgectl/`.
- **Given:** `.forgectl/` at `/project/`, current directory is `/project/api/internal/`.
- **When:** `forgectl status`
- **Then:** State file read from `/project/.forgectl/state/forgectl-state.json`.

---

## Implements
- Atomic state file writes with backup and startup recovery
- State file schema for all four phases with phase-scoped `config` object
- Project root discovery via `.forgectl/` directory walk
- Configurable state directory (`paths.state_dir`)
- Domain artifacts in configurable workspace directory (`paths.workspace_dir` = `.forge_workspace`)
- Session archiving to `state_dir/sessions/`
- Status command: session overview assembled from all phase sections
