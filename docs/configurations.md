# Forgectl Configuration Reference

## Overview

Configuration lives in `.forgectl/config` (TOML format) at the project root. The `.forgectl/` directory acts as a project marker — the scaffold discovers it by walking up the directory hierarchy from the current working directory, similar to how git finds `.git/`.

At `forgectl init`, the scaffold reads `.forgectl/config` and locks the effective configuration into the state file. From that point on, the session uses the state file's `config` object. Edits to `.forgectl/config` mid-session have no effect on the active session.

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

[specifying.reconciliation]
min_rounds = 0
max_rounds = 3

[planning]
batch = 1

[planning.eval]
min_rounds = 1
max_rounds = 3

[implementing]
batch = 2

[implementing.eval]
min_rounds = 1
max_rounds = 3

[paths]
state_dir = ".forgectl/state"
workspace_dir = ".forge_workspace"

[general]
user_guided = true
enable_commits = false
```

## Configuration Parameters

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

### Planning Phase

#### `planning.batch`

- **Type:** integer
- **Default:** 1
- **Constraint:** >= 1

Number of plans processed per planning cycle. TODO: values > 1 are not yet supported by the planning state machine. Reserved for future use.

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

## State File Config Structure

After `init`, the effective configuration is stored in the state file's `config` object, mirroring the TOML structure:

```json
{
  "config": {
    "specifying": {
      "batch": 3,
      "eval": { "min_rounds": 1, "max_rounds": 3 },
      "reconciliation": { "min_rounds": 0, "max_rounds": 3 }
    },
    "planning": {
      "batch": 1,
      "eval": { "min_rounds": 1, "max_rounds": 3 }
    },
    "implementing": {
      "batch": 2,
      "eval": { "min_rounds": 1, "max_rounds": 3 }
    },
    "paths": {
      "state_dir": ".forgectl/state",
      "workspace_dir": ".forge_workspace"
    },
    "general": {
      "user_guided": true,
      "enable_commits": false
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
| `--verdict PASS\|FAIL` | EVALUATE, RECONCILE_EVAL | Evaluation verdict |
| `--eval-report <path>` | EVALUATE, RECONCILE_EVAL | Path to evaluation report |
| `--message <text>` | COMMIT, ACCEPT (when `enable_commits: true`) | Commit message |
| `--file <path>` | specifying DRAFT | Override spec file path |
| `--from <path>` | PHASE_SHIFT (specifying→planning) | Plan queue input file |

## Non-Config Session Fields

These fields live at the top level of the state file, outside the `config` object.

| Field | Description |
|-------|-------------|
| `phase` | Active phase: `specifying`, `planning`, `implementing` |
| `state` | Current state within the active phase |
| `started_at_phase` | Which phase the session was initialized at (display only) |
| `phase_shift` | Records from/to during PHASE_SHIFT transitions |
