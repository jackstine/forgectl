# Activity Logging

Forgectl logs session activity automatically to track all operations performed during a session lifecycle.

## Storage

Forgectl stores activity logs in `~/.forgectl/logs/` in JSONL (JSON Lines) format. Each session generates one log file named:

```
<phase>-<session_id_prefix>.jsonl
```

Where:
- `phase` — initial phase name (`specifying`, `planning`, or `implementing`)
- `session_id_prefix` — first 8 characters of the UUID v4 generated at session init

Example: `planning-a1b2c3d4.jsonl`

## Logged Operations

The following commands and state transitions are logged:

| Operation | Details |
|-----------|---------|
| `init` | Session initialization with configuration snapshot |
| `advance` | State transition with phase, previous state, new state, and command-specific details |

### Read-Only Commands (Not Logged)

The following commands do not produce log entries:

- `status` — read-only operation
- `eval` — read-only evaluation
- `validate` — validation run, no session context
- `--version` — version display

### State File Modifications (Not Logged)

The following state file operations do not produce activity log entries:

- `add-queue-item` — queue manipulation
- `set-roots` — root path modification

These operations modify the persistent state file but do not represent user-driven activity flows.

## Configuration

Log behavior is controlled in `.forgectl/config` (project root) under the `[logs]` section:

```toml
[logs]
enabled = true          # default: true; set to false to disable all logging
retention_days = 90     # default: 90; delete files older than N days (0 = keep forever)
max_files = 50          # default: 50; max log files before oldest deletion (0 = unlimited)
```

### Configuration Options

- **enabled**: Master switch for activity logging. When `false`, no activity logs are written.
- **retention_days**: Age-based pruning threshold. Log files older than this many days are deleted. Set to `0` to keep logs indefinitely.
- **max_files**: Count-based pruning limit. If the log directory contains more than this many files, oldest files are deleted first. Set to `0` for unlimited log files.

## Log Pruning

Log pruning runs once at session `init` time and applies these rules in order:

1. **Age-based deletion** — Remove files older than `retention_days`
2. **Count-based deletion** — If remaining files exceed `max_files`, delete oldest files until limit is met

Active sessions are never deleted during pruning; only completed log files are candidates for removal.

## Log Entry Format

Each log entry is a JSON object on a single line, with the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `ts` | string | ISO 8601 timestamp in UTC (e.g., `2026-03-29T14:22:30Z`) |
| `cmd` | string | Command that triggered the entry: `init`, `advance` |
| `phase` | string | Current phase at time of command |
| `prev_state` | string | Previous state name (`advance` only) |
| `state` | string | Current state name |
| `detail` | object | Command-specific details (see examples below) |

### Example Log File

```jsonl
{"ts":"2026-03-29T14:32:01Z","cmd":"init","phase":"specifying","state":"ORIENT","detail":{"from":"spec-queue.json","batch_size":3,"rounds":"1-3","guided":true}}
{"ts":"2026-03-29T14:33:12Z","cmd":"advance","phase":"specifying","prev_state":"ORIENT","state":"SELECT","detail":{"batch":["repository-loading","snapshot-diffing","cache-invalidation"],"domain":"optimizer"}}
{"ts":"2026-03-29T14:45:00Z","cmd":"advance","phase":"specifying","prev_state":"SELECT","state":"DRAFT","detail":{}}
{"ts":"2026-03-29T15:10:33Z","cmd":"advance","phase":"specifying","prev_state":"DRAFT","state":"EVALUATE","detail":{"round":1}}
{"ts":"2026-03-29T15:12:01Z","cmd":"advance","phase":"specifying","prev_state":"EVALUATE","state":"REFINE","detail":{"round":1,"verdict":"FAIL","eval_report":"optimizer/specs/.eval/batch-1-r1.md"}}
{"ts":"2026-03-29T15:30:00Z","cmd":"advance","phase":"specifying","prev_state":"EVALUATE","state":"ACCEPT","detail":{"round":2,"verdict":"PASS","forced":false}}
```

## Viewing Logs

Log files are plain text JSONL format and can be viewed with standard tools:

```bash
# View entire log file
cat ~/.forgectl/logs/planning-a1b2c3d4.jsonl

# Pretty-print log entries
cat ~/.forgectl/logs/planning-a1b2c3d4.jsonl | jq .

# Filter by command type
cat ~/.forgectl/logs/planning-a1b2c3d4.jsonl | jq 'select(.cmd == "advance")'

# View state transitions only
cat ~/.forgectl/logs/planning-a1b2c3d4.jsonl | jq 'select(.cmd == "advance") | {ts, phase, prev_state, state, verdict: .detail.verdict}'
```
