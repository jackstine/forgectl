# Activity Logging

## Topic of Concern
> The scaffold logs session activity to JSONL files in the user's home directory for audit and debugging.

## Context

Forgectl writes structured log entries to `~/.forgectl/logs/` in JSONL format. Each session gets its own log file, named with the domain, phase, and session UUID. Logging captures state-mutating commands — `init` and `advance` — so the user can reconstruct what happened during a session.

Read-only commands (`status`, `eval`, `validate`, `--version`) do not produce log entries.

Log files accumulate over time. Pruning runs at `init` to clean up old files based on configurable retention and count limits. Active sessions are assumed to never be deleted by pruning.

## Depends On
- **session-init** — session UUID is generated at `init`; `[logs]` config is validated at init; pruning runs at init.
- **state-persistence** — `session_id` is stored in the state file root.

## Integration Points

| Spec | Relationship |
|------|-------------|
| session-init | Generates `session_id` (UUID v4), validates `[logs]` config, triggers pruning at init |
| state-persistence | `session_id` field in state file root; log file name derived from session metadata |
| spec-lifecycle | `advance` in specifying phase produces log entries with batch/domain context |
| phase-transitions | `advance` in generate_planning_queue phase produces log entries with state context |
| plan-production | `advance` in planning phase produces log entries with plan/round context |
| batch-implementation | `advance` in implementing phase produces log entries with item/layer context |
| reverse-engineering | `advance` in reverse_engineering phase produces log entries with domain/state context |

---

## Interface

### Inputs

#### Configuration — `[logs]` section in `.forgectl/config`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `logs.enabled` | boolean | `true` | Enable or disable logging. When false, no log file is created and no pruning runs. |
| `logs.retention_days` | integer | `90` | Delete log files older than N days. 0 = keep forever. |
| `logs.max_files` | integer | `50` | Maximum number of log files to keep. Oldest deleted first when exceeded. 0 = unlimited. |

### Outputs

#### Log Directory

```
~/.forgectl/logs/
├── specifying-a3f1b2c4.jsonl
├── planning-7e2d9f01.jsonl
├── implementing-b4c8e312.jsonl
└── ...
```

File naming convention: `<phase>-<session_id_prefix>.jsonl` where `session_id_prefix` is the first 8 characters of the UUID.

When a session spans multiple phases, the log file name uses the initial phase. The file name does not change when phases shift.

#### Log Entry Format

Each line is a JSON object:

```jsonl
{"ts":"2026-03-29T14:32:01Z","cmd":"init","phase":"specifying","state":"ORIENT","detail":{"from":"spec-queue.json","batch_size":3,"rounds":"1-3","guided":true}}
{"ts":"2026-03-29T14:33:12Z","cmd":"advance","phase":"specifying","prev_state":"ORIENT","state":"SELECT","detail":{"batch":["repository-loading","snapshot-diffing","cache-invalidation"],"domain":"optimizer"}}
{"ts":"2026-03-29T14:45:00Z","cmd":"advance","phase":"specifying","prev_state":"SELECT","state":"DRAFT","detail":{}}
{"ts":"2026-03-29T15:10:33Z","cmd":"advance","phase":"specifying","prev_state":"DRAFT","state":"EVALUATE","detail":{"round":1}}
{"ts":"2026-03-29T15:12:01Z","cmd":"advance","phase":"specifying","prev_state":"EVALUATE","state":"REFINE","detail":{"round":1,"verdict":"FAIL","eval_report":"optimizer/specs/.eval/batch-1-r1.md"}}
{"ts":"2026-03-29T15:30:00Z","cmd":"advance","phase":"specifying","prev_state":"EVALUATE","state":"ACCEPT","detail":{"round":2,"verdict":"PASS","forced":false}}
```

#### Entry Fields

| Field | Type | Present | Description |
|-------|------|---------|-------------|
| `ts` | string (ISO 8601 UTC) | always | Timestamp of the command |
| `cmd` | string | always | Command name: `init`, `advance` |
| `phase` | string | always | Current phase at time of command |
| `prev_state` | string | `advance` only | State before the transition |
| `state` | string | always | State after the command completes |
| `detail` | object | always | Command-specific context (may be empty `{}`) |

#### Detail Fields by Command

**`init`:**
- `from`: input file path
- `batch_size`: configured batch size
- `rounds`: min-max range string
- `guided`: boolean

**`advance`:**
- `domain`: current domain (when applicable)
- `batch`: array of spec/item names (when batch selected at ORIENT)
- `round`: current round number (when in eval states)
- `verdict`: PASS or FAIL (when advancing from eval states)
- `eval_report`: eval report path (when verdict provided)
- `forced`: boolean (when forced acceptance at max rounds)
- `layer`: layer ID (implementing phase)
- `item`: item ID (implementing phase, IMPLEMENT state)
- `unblocked`: count of unblocked items (implementing ORIENT)
- `remaining`: count of remaining items (implementing ORIENT)

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `~/.forgectl/logs/` cannot be created | Warning printed to stderr. Command continues without logging. | Logging failure does not block the primary workflow. |
| Log file cannot be written (permissions, disk full) | Warning printed to stderr. Command continues without logging. | Same — logging is best-effort. |

---

## Behavior

### Log File Creation

At `init`, after the state file is created:

1. If `logs.enabled` is false: skip logging and pruning entirely.
2. Create `~/.forgectl/logs/` directory if it does not exist.
3. Create the log file: `<phase>-<session_id_prefix>.jsonl`.
4. Write the `init` log entry.

For subsequent commands (`advance`):

1. If `logs.enabled` is false: skip.
2. Resolve the log file path from the session metadata in the state file.
3. Append the log entry.

### Pruning

Pruning runs at `init` time, before the new log file is created:

1. If `logs.enabled` is false: skip.
2. List all `.jsonl` files in `~/.forgectl/logs/`.
3. Sort by file creation time (oldest first).
4. If `logs.retention_days` > 0: delete any file older than `now - retention_days`.
5. If `logs.max_files` > 0 and remaining file count exceeds `max_files`: delete oldest files until at or below the limit.
6. Create the new session's log file.

Active sessions are assumed to never be deleted by pruning.

### Logging is Best-Effort

If the log directory cannot be created, the log file cannot be opened, or a write fails, the command prints a warning to stderr and continues. Logging failure never causes a command to fail.

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `logs.enabled` | boolean | `true` | Enable or disable logging entirely |
| `logs.retention_days` | integer | `90` | Delete log files older than N days. 0 = keep forever. |
| `logs.max_files` | integer | `50` | Maximum number of log files to keep. Oldest deleted first. 0 = unlimited. |

---

## Invariants

1. **Read-only commands do not log.** `status`, `eval`, `validate`, and `--version` never write log entries.
2. **Logging is best-effort.** Logging failures print a warning but never cause a command to fail.
3. **Pruning runs at init only.** Not on every command. One cleanup per session start.
4. **Log entries are append-only.** Entries are never modified or deleted by forgectl commands (only by pruning).
5. **Session ID is stable.** The log file name is determined at init and does not change when phases shift.
6. **Active sessions assumed safe.** Pruning does not attempt to detect or protect active sessions; this is an operational assumption.

---

## Edge Cases

- **Scenario:** `~/.forgectl/logs/` does not exist at init.
  - **Expected:** Directory created automatically. Logging proceeds.
  - **Rationale:** First-time users should not need to create the directory manually.

- **Scenario:** `logs.enabled` is false.
  - **Expected:** No log file created. No pruning runs. No warnings.
  - **Rationale:** Explicit opt-out disables all logging behavior.

- **Scenario:** Disk is full, log write fails.
  - **Expected:** Warning to stderr. Command completes normally.
  - **Rationale:** Logging must not block the primary workflow.

- **Scenario:** `logs.retention_days` is 0 and `logs.max_files` is 0.
  - **Expected:** No pruning occurs. Files accumulate indefinitely.
  - **Rationale:** Both set to 0 means no limits.

- **Scenario:** Both `retention_days` and `max_files` apply.
  - **Expected:** Age-based deletion runs first, then count-based deletion on the remaining files.
  - **Rationale:** Both constraints are enforced. Whichever is more restrictive governs.

- **Scenario:** Session spans all phases (specifying → generate_planning_queue → planning → implementing).
  - **Expected:** Single log file for the entire session. File name uses the initial phase.
  - **Rationale:** One session = one log file. Phase shifts are logged as entries, not file boundaries.

- **Scenario:** `add-queue-item` or `set-roots` command.
  - **Expected:** No log entry. These commands modify the state file but are not in the logged command set.
  - **Rationale:** Only `init` and `advance` are logged. Minor state modifications are visible in the state file diff.

---

## Testing Criteria

### init creates log file
- **Verifies:** Log file creation at session start.
- **Given:** `logs.enabled: true`. `~/.forgectl/logs/` exists.
- **When:** `forgectl init --phase specifying --from spec-queue.json --batch-size 3 --max-rounds 3`
- **Then:** Log file created with init entry. Entry has `cmd: "init"`, `phase: "specifying"`, `state: "ORIENT"`.

### advance appends log entry
- **Verifies:** State transition logged.
- **Given:** Active session in DRAFT state.
- **When:** `forgectl advance`
- **Then:** Log entry appended with `cmd: "advance"`, `prev_state: "DRAFT"`, `state: "EVALUATE"`.

### advance with verdict logs detail
- **Verifies:** Verdict and eval report captured in detail.
- **Given:** Active session in EVALUATE state.
- **When:** `forgectl advance --verdict FAIL --eval-report .eval/batch-1-r1.md`
- **Then:** Log entry has `detail.verdict: "FAIL"`, `detail.eval_report: ".eval/batch-1-r1.md"`.

### logging disabled skips everything
- **Verifies:** No log file when disabled.
- **Given:** `logs.enabled: false`.
- **When:** `forgectl init ...`
- **Then:** No log file created. No warnings.

### pruning at init by age
- **Verifies:** Age-based cleanup.
- **Given:** `logs.retention_days: 30`. Three log files: 10 days old, 40 days old, 60 days old.
- **When:** `forgectl init ...`
- **Then:** Two oldest files deleted. One file remains.

### pruning at init by count
- **Verifies:** Count-based cleanup.
- **Given:** `logs.max_files: 2`. Three existing log files (all recent).
- **When:** `forgectl init ...`
- **Then:** Oldest file deleted. Two files remain. New session file created (total 3, but max_files checked before creation).

### log write failure is non-fatal
- **Verifies:** Best-effort logging.
- **Given:** Log directory exists but is read-only.
- **When:** `forgectl advance`
- **Then:** Warning printed to stderr. Command completes normally. Exit code 0.

### status does not log
- **Verifies:** Read-only commands skip logging.
- **Given:** Active session.
- **When:** `forgectl status`
- **Then:** No new log entry appended.

---

## Implements
- JSONL activity logging to `~/.forgectl/logs/`
- Per-session log files named with domain, phase, and session UUID prefix
- Log entries for state-mutating commands: init, advance
- Configurable pruning at init: retention_days, max_files, enabled
- Best-effort logging that never blocks primary workflow
