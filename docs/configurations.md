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
[[domains]]
name = "optimizer"
path = "optimizer"

[[domains]]
name = "portal"
path = "portal"

[specifying]
batch = 3
commit_strategy = "all-specs"

[specifying.eval]
min_rounds = 1
max_rounds = 3
type = "eval"
model = "opus"
count = 1

[specifying.cross_reference]
min_rounds = 1
max_rounds = 2
type = "explore"
model = "haiku"
count = 3
user_review = false

[specifying.cross_reference.eval]
type = "eval"
model = "opus"
count = 1

[specifying.reconciliation]
min_rounds = 0
max_rounds = 3
type = "eval"
model = "opus"
count = 1

[planning]
batch = 1
commit_strategy = "strict"
self_review = false
plan_all_before_implementing = false

[planning.study_code]
type = "explore"
model = "haiku"
count = 3

[planning.eval]
min_rounds = 1
max_rounds = 3
type = "eval"
model = "opus"
count = 1

[planning.refine]
type = "refine"
model = "opus"
count = 1

[implementing]
batch = 2
commit_strategy = "scoped"

[implementing.eval]
min_rounds = 1
max_rounds = 3
type = "eval"
model = "opus"
count = 1

[paths]
state_dir = ".forgectl/state"
workspace_dir = ".forge_workspace"

[general]
user_guided = true
enable_commits = false
```

## Configuration Parameters

### Domains

#### `[[domains]]`

- **Type:** array of tables (optional)
- **Default:** none

Declares known domains with their names and paths. Domains are optional metadata — projects can operate without defining any. When configured, domain resolution for `add-queue-item` and spec queue validation uses these entries.

```toml
[[domains]]
name = "emails"
path = "domains/emails"

[[domains]]
name = "databases"
path = "pkg/internal/databases"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Domain name (used in spec queue `domain` field and `add-queue-item --domain`) |
| `path` | string | yes | Domain directory path relative to project root. Specs live in `<path>/specs/`. |

**Constraints:**
- No domain path is a prefix of another domain path. E.g., `domains/users` and `domains/users/employees` is rejected.
- Domain names must be unique.

**Behavior:**
- Domains are read-only in config. `add-queue-item --domain` with a new domain registers it in the session state only — the config file is never modified by forgectl.
- When domains are configured, spec queue entries at init must reference a configured domain name, and the file path must match the domain's `<path>/specs/` prefix.

### Sub-Agent Configuration

Every configuration section that involves spawning sub-agents includes `type`, `model`, and `count`:

- **`type`** — The role of the sub-agent at this spawn point. Valid values: `"eval"`, `"explore"`, `"refine"`.
- **`model`** — A string identifying the model for the sub-agent. Can be a model name (e.g., `"opus"`, `"haiku"`) or a descriptive phrase (e.g., `"opus explorer"`, `"spec-eval-expert"`).
- **`count`** — An integer specifying how many sub-agents to spawn. Must be >= 1.

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

#### `specifying.eval.type`

- **Type:** string
- **Default:** `"eval"`

Sub-agent role for spec batch evaluation at EVALUATE state.

#### `specifying.eval.model`

- **Type:** string
- **Default:** `"opus"`

Model name for spec batch evaluation at EVALUATE state.

#### `specifying.eval.count`

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

#### `specifying.cross_reference.type`

- **Type:** string
- **Default:** `"explore"`

Sub-agent role for cross-referencing domain specs at CROSS_REFERENCE state.

#### `specifying.cross_reference.model`

- **Type:** string
- **Default:** `"haiku"`

Model name for cross-referencing domain specs at CROSS_REFERENCE state.

#### `specifying.cross_reference.count`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= 1

Number of sub-agents to spawn for domain cross-referencing.

#### `specifying.cross_reference.user_review`

- **Type:** boolean
- **Default:** false

When true, the scaffold pauses at CROSS_REFERENCE_REVIEW after the first CROSS_REFERENCE_EVAL (regardless of verdict). This fires even when `general.user_guided` is false. The pause asks the architect to review with their user before continuing. Only fires once per domain — subsequent rounds skip the review pause.

#### `specifying.cross_reference.eval.type`

- **Type:** string
- **Default:** `"eval"`

Sub-agent role for evaluating cross-reference work at CROSS_REFERENCE_EVAL state.

#### `specifying.cross_reference.eval.model`

- **Type:** string
- **Default:** `"opus"`

Model name for evaluating cross-reference work at CROSS_REFERENCE_EVAL state.

#### `specifying.cross_reference.eval.count`

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

#### `specifying.reconciliation.type`

- **Type:** string
- **Default:** `"eval"`

Sub-agent role for reconciliation evaluation at RECONCILE_EVAL state.

#### `specifying.reconciliation.model`

- **Type:** string
- **Default:** `"opus"`

Model name for reconciliation evaluation at RECONCILE_EVAL state.

#### `specifying.reconciliation.count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for reconciliation evaluation.

#### `specifying.reconciliation.user_review`

- **Type:** boolean
- **Default:** false

When true, RECONCILE_REVIEW action includes "STOP please review and discuss with user before continuing." When false, it says "Reconciliation review complete." The RECONCILE_REVIEW state is entered either way — this only controls the output message.

#### `specifying.commit_strategy`

- **Type:** string
- **Default:** `"all-specs"`
- **Valid values:** `strict`, `all-specs`, `scoped`, `tracked`, `all`

Controls which files are staged when the scaffold auto-commits during the specifying phase. A single commit is made at COMPLETE. See `docs/auto-committing.md` for full behavior details.

### Planning Phase

#### `planning.batch`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of plans processed per planning cycle. Values > 1 are not yet supported. Reserved for future use.

#### `planning.plan_all_before_implementing`

- **Type:** boolean
- **Default:** `false`

Controls how domains are processed across planning and implementing phases.

When `false` (default, interleaved): each domain is planned and then immediately implemented before the next domain begins. The flow cycles between planning and implementing per domain. PHASE_SHIFT fires at every phase and domain boundary.

When `true` (all planning first): all domains are planned first (with PHASE_SHIFT between each domain), then all domains are implemented (with PHASE_SHIFT between each domain).

#### `planning.study_code.type`

- **Type:** string
- **Default:** `"explore"`

Sub-agent role for codebase exploration at STUDY_CODE state.

#### `planning.study_code.model`

- **Type:** string
- **Default:** `"haiku"`

Model name for codebase exploration at STUDY_CODE state.

#### `planning.study_code.count`

- **Type:** integer
- **Default:** 3
- **Constraint:** >= 1

Number of sub-agents to spawn for codebase exploration.

#### `planning.self_review`

- **Type:** boolean
- **Default:** `false`

When true, SELF_REVIEW state is entered between validation and EVALUATE on every round. The agent reviews their plan against specs and study notes, revising plan.json and notes as needed before evaluation. When false, SELF_REVIEW is skipped.

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

#### `planning.eval.type`

- **Type:** string
- **Default:** `"eval"`

Sub-agent role for plan evaluation at EVALUATE state.

#### `planning.eval.model`

- **Type:** string
- **Default:** `"opus"`

Model name for plan evaluation at EVALUATE state.

#### `planning.eval.count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for plan evaluation.

#### `planning.refine.type`

- **Type:** string
- **Default:** `"refine"`

Sub-agent role for updating plan from eval findings at REFINE state.

#### `planning.refine.model`

- **Type:** string
- **Default:** `"opus"`

Model name for updating plan from eval findings at REFINE state.

#### `planning.refine.count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for plan refinement.

#### `planning.commit_strategy`

- **Type:** string
- **Default:** `"strict"`
- **Valid values:** `strict`, `all-specs`, `scoped`, `tracked`, `all`

Controls which files are staged when the scaffold auto-commits during the planning phase. A per-plan commit is made at ACCEPT. See `docs/auto-committing.md` for full behavior details.

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

#### `implementing.eval.type`

- **Type:** string
- **Default:** `"eval"`

Sub-agent role for implementation evaluation at EVALUATE state.

#### `implementing.eval.model`

- **Type:** string
- **Default:** `"opus"`

Model name for implementation evaluation at EVALUATE state.

#### `implementing.eval.count`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of sub-agents to spawn for implementation evaluation.

#### `implementing.commit_strategy`

- **Type:** string
- **Default:** `"scoped"`
- **Valid values:** `strict`, `all-specs`, `scoped`, `tracked`, `all`

Controls which files are staged when the scaffold auto-commits during the implementing phase. A per-item commit is made at IMPLEMENT (first round only) and a per-batch commit is made at COMMIT. See `docs/auto-committing.md` for full behavior details.

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

When `false` (default): COMMIT states remain as pause points but `--message` (`-m`) is not required or prompted. No git operations are performed. The engineer commits manually.

When `true`: `--message` / `-m` is required and validated at the following commit points:
- **Specifying:** single commit at COMPLETE
- **Planning:** per-plan commit at ACCEPT
- **Implementing:** per-item commit at IMPLEMENT (first round only) + per-batch commit at COMMIT

See `docs/auto-committing.md` for full auto-commit behavior details.

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
    "domains": [
      { "name": "optimizer", "path": "optimizer" },
      { "name": "portal", "path": "portal" }
    ],
    "specifying": {
      "batch": 3,
      "commit_strategy": "all-specs",
      "eval": { "min_rounds": 1, "max_rounds": 3, "type": "eval", "model": "opus", "count": 1 },
      "cross_reference": {
        "min_rounds": 1,
        "max_rounds": 2,
        "type": "explore",
        "model": "haiku",
        "count": 3,
        "user_review": false,
        "eval": { "type": "eval", "model": "opus", "count": 1 }
      },
      "reconciliation": { "min_rounds": 0, "max_rounds": 3, "type": "eval", "model": "opus", "count": 1, "user_review": false }
    },
    "planning": {
      "batch": 1,
      "commit_strategy": "strict",
      "self_review": false,
      "plan_all_before_implementing": false,
      "study_code": { "type": "explore", "model": "haiku", "count": 3 },
      "eval": { "min_rounds": 1, "max_rounds": 3, "type": "eval", "model": "opus", "count": 1 },
      "refine": { "type": "refine", "model": "opus", "count": 1 }
    },
    "implementing": {
      "batch": 2,
      "commit_strategy": "scoped",
      "eval": { "min_rounds": 1, "max_rounds": 3, "type": "eval", "model": "opus", "count": 1 }
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
| `--message <text>`, `-m <text>` | COMPLETE (specifying), ACCEPT (planning), IMPLEMENT first round (implementing), COMMIT (implementing) — when `enable_commits: true` | Commit message |
| `--from <path>` | PHASE_SHIFT (specifying→generate_planning_queue, generate_planning_queue→planning) | Plan queue input file |

## Non-Config Session Fields

These fields live at the top level of the state file, outside the `config` object.

| Field | Description |
|-------|-------------|
| `phase` | Active phase: `specifying`, `planning`, `implementing` |
| `state` | Current state within the active phase |
| `started_at_phase` | Which phase the session was initialized at (display only) |
| `phase_shift` | Records from/to during PHASE_SHIFT transitions |
