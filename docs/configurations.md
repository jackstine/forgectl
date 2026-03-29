# Forgectl Configuration Reference

## Overview

Configuration lives in `.forgectl/config` (TOML format) at the project root. The `.forgectl/` directory acts as a project marker — the scaffold discovers it by walking up the directory hierarchy from the current working directory, similar to how git finds `.git/`.

At `forgectl init`, the scaffold reads `.forgectl/config` and locks the effective configuration into the state file. From that point on, the session uses the state file's `config` object. Edits to `.forgectl/config` mid-session have no effect on the active session.

The scaffold does not spawn sub-agents. It outputs instructions telling the architect what to spawn. The architect (or the skill driving the session) is responsible for spawning them. See `docs/sub-agent-spawn-points.md` for the full spawn point reference.

## Directory Structure

```
<project_root>/
├── .forgectl/
│   ├── config                              ← TOML project configuration
│   └── state/                              ← default state directory
│       ├── forgectl-state.json             ← active session state (gitignored)
│       ├── forgectl-state.json.bak         ← previous state (gitignored)
│       └── sessions/                       ← archived completed sessions (git tracked)
├── <domain>/
│   ├── .forge_workspace/                   ← domain artifacts (plans, notes)
│   │   └── implementation_plan/
│   └── specs/
└── ...
```

### Project Root Discovery

The scaffold resolves the project root by walking up from the current directory:

1. Check current directory for `.forgectl/`
2. Check parent directory
3. Continue until `.forgectl/` is found or filesystem root is reached
4. If not found: error — "No .forgectl directory found."

`.forgectl/` is created by the user before any session.

## Config File Format (TOML)

See `docs/default-config.toml` for the complete default config with comments.

```toml
[specifying]
batch = 3

[specifying.eval]
min_rounds = 1
max_rounds = 3
agent_type = "opus"
agent_count = 1

[specifying.cross_reference]
min_rounds = 1
max_rounds = 2
agent_type = "haiku"
agent_count = 3
user_review = false

[specifying.cross_reference.eval]
agent_type = "opus"
agent_count = 1

[specifying.reconciliation]
min_rounds = 0
max_rounds = 3
agent_type = "opus"
agent_count = 1

[planning]
batch = 1

[planning.study_code]
agent_type = "haiku"
agent_count = 3

[planning.eval]
min_rounds = 1
max_rounds = 3
agent_type = "opus"
agent_count = 1

[planning.refine]
agent_type = "opus"
agent_count = 1

[implementing]
batch = 2

[implementing.eval]
min_rounds = 1
max_rounds = 3
agent_type = "opus"
agent_count = 1

[paths]
state_dir = ".forgectl/state"
workspace_dir = ".forge_workspace"

[general]
user_guided = true
enable_commits = false
```

## Configuration Parameters

### Sub-Agent Configuration

Every configuration section that involves spawning sub-agents includes `agent_type` and `agent_count`:

- **`agent_type`** — A string identifying the type of sub-agent to spawn. Can be a model name (e.g., `"opus"`, `"haiku"`) or a descriptive phrase (e.g., `"opus explorer"`, `"spec-eval-expert"`).
- **`agent_count`** — An integer specifying how many sub-agents to spawn. Must be >= 1.

The scaffold outputs these values in spawn instructions: `"Please spawn {count} {type} sub-agent(s) to {purpose}."`

### Specifying Phase

#### `specifying.batch`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= 1

Number of specs processed per specifying cycle. Specs are grouped by domain — a batch never mixes domains. If a domain has more specs than the batch size, it produces multiple batches before moving to the next domain.

#### `specifying.eval.min_rounds`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1, <= specifying.eval.max_rounds

Minimum evaluation rounds before a PASS verdict can accept a spec batch. PASS below this threshold forces another REFINE cycle.

#### `specifying.eval.max_rounds`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= specifying.eval.min_rounds

Maximum evaluation rounds for a spec batch. FAIL at this threshold forces acceptance.

#### `specifying.eval.agent_type`

- **Type:** string
- **Default:** `"opus"`

Sub-agent type for spec batch evaluation at EVALUATE state.

#### `specifying.eval.agent_count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for spec batch evaluation.

#### `specifying.cross_reference.min_rounds`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1, <= specifying.cross_reference.max_rounds

Minimum evaluation rounds for domain cross-reference. PASS below this threshold forces another cycle.

#### `specifying.cross_reference.max_rounds`

- **Type:** integer
- **Default:** 2
- **Constraint:** >= specifying.cross_reference.min_rounds

Maximum evaluation rounds for domain cross-reference. FAIL at this threshold forces acceptance.

#### `specifying.cross_reference.agent_type`

- **Type:** string
- **Default:** `"haiku"`

Sub-agent type for cross-referencing domain specs at CROSS_REFERENCE state.

#### `specifying.cross_reference.agent_count`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= 1

Number of sub-agents to spawn for domain cross-referencing.

#### `specifying.cross_reference.user_review`

- **Type:** boolean
- **Default:** false

When true, the scaffold pauses at CROSS_REFERENCE_REVIEW after the first CROSS_REFERENCE_EVAL (regardless of verdict). This fires even when `general.user_guided` is false. The pause asks the architect to review with their user before continuing. Only fires once per domain — subsequent rounds skip the review pause.

#### `specifying.cross_reference.eval.agent_type`

- **Type:** string
- **Default:** `"opus"`

Sub-agent type for evaluating cross-reference work at CROSS_REFERENCE_EVAL state.

#### `specifying.cross_reference.eval.agent_count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for cross-reference evaluation.

#### `specifying.reconciliation.min_rounds`

- **Type:** integer
- **Default:** 0
- **Constraint:** >= 0, <= specifying.reconciliation.max_rounds

Minimum reconciliation eval rounds before a PASS verdict can complete the specifying phase. When 0 (default), a single PASS immediately transitions to COMPLETE.

#### `specifying.reconciliation.max_rounds`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= specifying.reconciliation.min_rounds

Maximum reconciliation eval rounds. FAIL at this threshold forces completion.

#### `specifying.reconciliation.agent_type`

- **Type:** string
- **Default:** `"opus"`

Sub-agent type for reconciliation evaluation at RECONCILE_EVAL state.

#### `specifying.reconciliation.agent_count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for reconciliation evaluation.

#### `specifying.reconciliation.user_review`

- **Type:** boolean
- **Default:** false

When true, RECONCILE_REVIEW action includes "STOP please review and discuss with user before continuing." When false, it says "Reconciliation review complete." The RECONCILE_REVIEW state is entered either way — this only controls the output message.

### Planning Phase

#### `planning.batch`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of plans processed per planning cycle. TODO: values > 1 are not yet supported by the planning state machine. Reserved for future use.

#### `planning.study_code.agent_type`

- **Type:** string
- **Default:** `"haiku"`

Sub-agent type for codebase exploration at STUDY_CODE state.

#### `planning.study_code.agent_count`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= 1

Number of sub-agents to spawn for codebase exploration.

#### `planning.eval.min_rounds`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1, <= planning.eval.max_rounds

Minimum evaluation rounds before a PASS verdict can accept a plan.

#### `planning.eval.max_rounds`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= planning.eval.min_rounds

Maximum evaluation rounds for a plan. FAIL at this threshold forces acceptance.

#### `planning.eval.agent_type`

- **Type:** string
- **Default:** `"opus"`

Sub-agent type for plan evaluation at EVALUATE state.

#### `planning.eval.agent_count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for plan evaluation.

#### `planning.refine.agent_type`

- **Type:** string
- **Default:** `"opus"`

Sub-agent type for updating plan from eval findings at REFINE state.

#### `planning.refine.agent_count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for plan refinement.

### Implementing Phase

#### `implementing.batch`

- **Type:** integer
- **Default:** 2
- **Constraint:** >= 1

Maximum number of unblocked plan items delivered per implementation batch. Items are selected in dependency order from the current layer.

#### `implementing.eval.min_rounds`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1, <= implementing.eval.max_rounds

Minimum evaluation rounds before a PASS verdict can accept an implementation batch.

#### `implementing.eval.max_rounds`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= implementing.eval.min_rounds

Maximum evaluation rounds for an implementation batch. FAIL at this threshold forces acceptance.

#### `implementing.eval.agent_type`

- **Type:** string
- **Default:** `"opus"`

Sub-agent type for implementation evaluation at EVALUATE state.

#### `implementing.eval.agent_count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for implementation evaluation.

### Paths

#### `paths.state_dir`

- **Type:** string
- **Default:** `.forgectl/state`

Directory where session state files are stored. Resolution:
1. If absolute path — use directly
2. If relative path — resolve relative to project root

#### `paths.workspace_dir`

- **Type:** string
- **Default:** `.forge_workspace`

Directory name for domain artifacts (plans, notes, manifests). Created inside each domain directory as `<domain>/<workspace_dir>/`.

### General

#### `general.user_guided`

- **Type:** boolean
- **Default:** true
- **Mutable:** yes (via `--guided` / `--no-guided` on any `advance` call)

When true, the scaffold inserts pause points at SELECT (specifying), REVIEW (planning), and ORIENT (implementing) with "Stop and review and discuss with user before continuing."

#### `general.enable_commits`

- **Type:** boolean
- **Default:** false

Controls whether the scaffold requires and executes git commits.

When `false` (default): COMMIT states remain as pause points but `--message` is not required or prompted. No git operations are performed. The engineer commits manually.

When `true`: `--message` is required and validated at COMMIT states and first-round IMPLEMENT advances. TODO: automatic `git commit` execution is not yet implemented — the flag is validated and stored but no git operation occurs.

### Logs

#### `logs.enabled`

- **Type:** boolean
- **Default:** true

Enable or disable activity logging.

#### `logs.retention_days`

- **Type:** integer
- **Default:** 90
- **Constraint:** >= 0

Delete log files older than N days. When set to 0, log files are kept forever.

#### `logs.max_files`

- **Type:** integer
- **Default:** 50
- **Constraint:** >= 0

Maximum log files to keep. Oldest files are deleted first. When set to 0, the number of files is unlimited.

**Note:** Log files are stored in `~/.forgectl/logs/` (user home directory, not project directory). The logs directory is created automatically if it does not exist. Retention constraints are validated at init time.

## State File Config Structure

After `init`, the effective configuration is stored in the state file's `config` object, mirroring the TOML structure:

```json
{
  "config": {
    "specifying": {
      "batch": 3,
      "eval": { "min_rounds": 1, "max_rounds": 3, "agent_type": "opus", "agent_count": 1 },
      "cross_reference": {
        "min_rounds": 1,
        "max_rounds": 2,
        "agent_type": "haiku",
        "agent_count": 3,
        "user_review": false,
        "eval": { "agent_type": "opus", "agent_count": 1 }
      },
      "reconciliation": { "min_rounds": 0, "max_rounds": 3, "agent_type": "opus", "agent_count": 1, "user_review": false }
    },
    "planning": {
      "batch": 1,
      "study_code": { "agent_type": "haiku", "agent_count": 3 },
      "eval": { "min_rounds": 1, "max_rounds": 3, "agent_type": "opus", "agent_count": 1 },
      "refine": { "agent_type": "opus", "agent_count": 1 }
    },
    "implementing": {
      "batch": 2,
      "eval": { "min_rounds": 1, "max_rounds": 3, "agent_type": "opus", "agent_count": 1 }
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
  "phase": "specifying",
  "state": "ORIENT",
  "started_at_phase": "specifying",
  ...
}
```

## CLI Interface

### `init` command

| Flag | Required | Description |
|------|----------|-------------|
| `--from <path>` | yes | Input file (spec queue, plan queue, or plan.json) |
| `--phase <specifying\|planning\|implementing>` | no (default: specifying) | Starting phase |

All other configuration is read from `.forgectl/config`.

### `advance` command

| Flag | Context | Description |
|------|---------|-------------|
| `--guided` / `--no-guided` | any state | Toggle guided mode (updates `config.general.user_guided` in state) |
| `--verdict PASS\|FAIL` | EVALUATE, RECONCILE_EVAL, CROSS_REFERENCE_EVAL | Evaluation verdict |
| `--eval-report <path>` | EVALUATE, RECONCILE_EVAL, CROSS_REFERENCE_EVAL | Path to evaluation report |
| `--message <text>` | COMMIT, ACCEPT (when `enable_commits: true`) | Commit message |
| `--from <path>` | PHASE_SHIFT (specifying→planning) | Plan queue input file |

## Non-Config Session Fields

These fields live at the top level of the state file, outside the `config` object.

| Field | Description |
|-------|-------------|
| `phase` | Active phase: `specifying`, `planning`, `implementing` |
| `state` | Current state within the active phase |
| `started_at_phase` | Which phase the session was initialized at (display only) |
| `phase_shift` | Records from/to during PHASE_SHIFT transitions |
