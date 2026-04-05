# Session Initialization

## Topic of Concern
> The scaffold initializes a session from a validated input file, project configuration, and a specified phase.

## Context

The `init` command creates a new `forgectl-state.json` from a user-provided input file and the project's `.forgectl/config`. The input schema varies by phase: a spec queue for specifying, a plans queue for planning, or a plan.json for implementing. The scaffold discovers the project root by walking up the directory hierarchy to find `.forgectl/`, reads the TOML config, validates the input, rejects malformed data with actionable errors, and sets the starting state for the chosen phase.

All configuration is read from `.forgectl/config` at init time and locked into the state file. CLI flags on `init` are limited to `--from` and `--phase`. See `docs/configurations.md` for the full configuration reference.

Sessions can begin at any of three phases — specifying, planning, or implementing — allowing users to skip earlier phases when inputs already exist. The generate_planning_queue phase cannot be initialized directly; it requires a completed specifying phase.

## Depends On
- **state-persistence** — provides the write mechanism and file layout for the state file.

## Integration Points

| Spec | Relationship |
|------|-------------|
| spec-lifecycle | Consumes the spec queue populated during specifying init |
| plan-production | Consumes the plan queue populated during planning init |
| batch-implementation | Consumes the plan.json validated during implementing init |
| state-persistence | State file schema defines the structure created here; `session_id` stored at root |
| activity-logging | `session_id` generated here; `[logs]` config validated here; pruning triggered here |
| reverse-engineering | Consumes the init input (concept + domains) populated during reverse_engineering init |

---

## Interface

### Inputs

#### CLI Command

| Command | Flags | Description |
|---------|-------|-------------|
| `init` | `--from <path>` (required), `--phase specifying\|planning\|implementing` (default specifying) | Initialize state file from validated input and project config |

All other configuration is read from `.forgectl/config`.

#### Spec Queue Input File (`--phase specifying`)

```json
{
  "specs": [
    {
      "name": "Repository Loading",
      "domain": "optimizer",
      "topic": "The optimizer clones or locates a repository and provides its path for downstream modules",
      "file": "optimizer/specs/repository-loading.md",
      "planning_sources": [
        ".forge_workspace/planning/optimizer/repo-snapshot-loading.md"
      ],
      "depends_on": []
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `specs` | array | yes | Ordered list of specs to generate |
| `specs[].name` | string | yes | Display name for the spec |
| `specs[].domain` | string | yes | Domain grouping |
| `specs[].topic` | string | yes | One-sentence topic of concern |
| `specs[].file` | string | yes | Target file path relative to project root |
| `specs[].planning_sources` | string[] | yes | Planning document paths the spec is derived from; may be empty array |
| `specs[].depends_on` | string[] | yes | Names of specs this one depends on; may be empty array |

No additional fields are permitted.

#### Plan Queue Input File (`--phase planning`)

```json
{
  "plans": [
    {
      "name": "Protocols Implementation Plan",
      "domain": "protocols",
      "file": "protocols/.forge_workspace/implementation_plan/plan.json",
      "specs": [
        "protocols/ws1/specs/ws1-message-contract.md",
        "protocols/ws2/specs/ws2-message-contract.md"
      ],
      "spec_commits": ["7cede10", "8743b1d"],
      "code_search_roots": ["api/", "optimizer/", "portal/"]
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `plans` | array | yes | Ordered list of plans to generate and implement |
| `plans[].name` | string | yes | Display name for the plan |
| `plans[].domain` | string | yes | Domain grouping |
| `plans[].file` | string | yes | Target path for plan.json relative to project root |
| `plans[].specs` | string[] | yes | Spec file paths to study; may be empty array |
| `plans[].spec_commits` | string[] | yes | Git commit hashes associated with specs for viewing diffs; may be empty array |
| `plans[].code_search_roots` | string[] | yes | Directory roots for codebase exploration; may be empty array |

No additional fields are permitted.

#### Plan.json Input File (`--phase implementing`)

A `plan.json` file conforming to the schema defined in `PLAN_FORMAT.md`. The scaffold validates the full plan structure during init and adds `passes` and `rounds` fields to each item.

### Outputs

#### Validation Failure Output

When the input file fails validation, the scaffold prints:
1. Each validation error (missing field, extra field, wrong type) with the path to the offending location.
2. The complete valid schema as a reference.

The scaffold exits with a non-zero code on validation failure.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `.forgectl/` directory not found in hierarchy | Error: "No .forgectl directory found." Exit code 1. | Project root must be established |
| `.forgectl/config` missing or unparseable | Error with parse details. Exit code 1. | Config must exist and be valid TOML |
| Config constraint violation (e.g., eval.min_rounds > eval.max_rounds, invalid commit_strategy, nested domain paths) | Error listing violations. Exit code 1. | Invalid configuration |
| `init` called when state file already exists | Error: "State file already exists. Delete it to reinitialize." Exit code 1. | Prevents accidental loss of in-progress state |
| `--from` file fails schema validation | Error listing violations. Prints full valid schema. Exit code 1. | User needs to see what's wrong |
| `--phase` not one of the three valid values | Error: "--phase must be specifying, planning, or implementing." Exit code 1. | Invalid phase |
| `--phase generate_planning_queue` | Error: "generate_planning_queue requires a completed specifying phase. Use --phase specifying instead." Exit code 1. | Cannot initialize mid-lifecycle phase directly |

---

## Behavior

### Initializing a Session

#### Preconditions
- `.forgectl/` directory exists in the current directory or an ancestor.
- `.forgectl/config` exists and contains valid TOML.
- No state file exists at the configured `state_dir` location.
- `--from` is provided.
- `--phase` is one of `specifying`, `planning`, `implementing` (default: `specifying`). `generate_planning_queue` is not valid.

#### Steps
1. Walk up the directory hierarchy to find `.forgectl/`. This establishes the project root.
2. Read and parse `.forgectl/config` (TOML).
3. Validate all config constraints (e.g., `eval.min_rounds <= eval.max_rounds` per phase, `batch >= 1`).
4. If config validation fails: print errors, exit code 1.
5. Read and parse the file at `--from`.
6. Validate against the schema for the specified `--phase`.
7. If input validation fails: print errors and schema, exit code 1.
8. If validation passes:
   - Generate a `session_id` (UUID v4) and store it at the state file root.
   - Lock the effective configuration into the state file's `config` object.
   - For `--phase specifying`: create state file with phase `specifying`, state ORIENT, spec queue populated.
   - For `--phase planning`: create state file with phase `planning`, state ORIENT, plan queue populated.
   - For `--phase implementing`: validate plan.json, add `passes: "pending"` and `rounds: 0` to items, create state file with phase `implementing`, state ORIENT.
9. If `config.logs.enabled` is true:
   - Run log pruning (delete files exceeding `logs.retention_days` and `logs.max_files`). See activity-logging spec.
   - Create the session log file at `~/.forgectl/logs/<phase>-<session_id_prefix>.jsonl`.
   - Write the `init` log entry.

#### Postconditions
- State file exists at the configured `state_dir` with `config` object mirroring the TOML structure.
- `session_id` is a UUID v4 stored at the state file root.
- Phase and state reflect the starting point.
- For `--phase implementing`: plan.json items have `passes` and `rounds` fields.
- If logging is enabled: log file exists and contains the init entry.

#### Error Handling
- `.forgectl/` not found: error. Exit code 1.
- Config file missing or invalid TOML: error with parse details. Exit code 1.
- Config constraint violation: error listing violations. Exit code 1.
- Input file not found: error with path. Exit code 1.
- Invalid JSON: error with parse details. Exit code 1.
- Schema failure: error listing violations, print valid schema. Exit code 1.

---

## Configuration

All configuration is read from `.forgectl/config` (TOML) and locked into the state file at init time. See `docs/configurations.md` for the full reference and `docs/default-config.toml` for the default config with comments.

The `init` command accepts only two flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--from <path>` | none (required) | Path to input file (schema varies by `--phase`) |
| `--phase` | specifying | Starting phase: `specifying`, `planning`, `implementing` |

The optional `[[domains]]` section in `.forgectl/config` declares known domains:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `domains[].name` | string | yes | Domain name (used in spec queue `domain` field) |
| `domains[].path` | string | yes | Domain directory path relative to project root |

Validation at init:
- No domain path is a prefix of another domain path. E.g., `domains/users` and `domains/users/employees` is rejected. Error: `Domain paths must not be nested: <path1> is a prefix of <path2>.`
- When `--phase specifying`: each spec queue entry's `domain` must match a configured domain name (if domains are configured). The spec's `file` path must start with the domain's `path` + `/specs/`.
- Domains are optional. If no `[[domains]]` section exists, domain resolution falls back to deriving from file paths.

The `commit_strategy` per phase is validated at init:

| Parameter | Type | Default | Constraint |
|-----------|------|---------|------------|
| `specifying.commit_strategy` | string | `all-specs` | One of: `strict`, `all-specs`, `scoped`, `tracked`, `all` |
| `planning.commit_strategy` | string | `strict` | One of: `strict`, `all-specs`, `scoped`, `tracked`, `all` |
| `implementing.commit_strategy` | string | `scoped` | One of: `strict`, `all-specs`, `scoped`, `tracked`, `all` |

The `[logs]` section in `.forgectl/config` is validated at init:

| Parameter | Type | Default | Constraint |
|-----------|------|---------|------------|
| `logs.enabled` | boolean | `true` | — |
| `logs.retention_days` | integer | `90` | >= 0 |
| `logs.max_files` | integer | `50` | >= 0 |

---

## Invariants

1. **No implicit state.** All information for transitions is in the state file (and plan.json during implementing).
2. **Config locked at init.** The state file's `config` object is the single source of truth for the session. `.forgectl/config` is not re-read after init.
3. **Project root required.** `.forgectl/` must exist. The scaffold does not create it.
4. **Session ID generated once.** `session_id` is a UUID v4 created at init and never changes for the lifetime of the session.
5. **Logging is best-effort.** Log file creation failure prints a warning but does not prevent init from completing.

---

## Edge Cases

- **Scenario:** `--phase implementing` with plan.json that has items already containing `passes` or `rounds` fields.
  - **Expected:** Fields are overwritten with `passes: "pending"` and `rounds: 0`.
  - **Rationale:** Init always starts fresh. Pre-existing tracking fields from a previous session are reset.

- **Scenario:** `--from` points to a valid JSON file that does not match any queue schema.
  - **Expected:** Schema validation fails with specific field-level errors and the valid schema printed as reference.
  - **Rationale:** Users need to see both what's wrong and what's expected to fix the file.

- **Scenario:** `.forgectl/config` has a missing section (e.g., no `[implementing]` table).
  - **Expected:** Missing values fall back to defaults defined in `docs/default-config.toml`.
  - **Rationale:** Partial configs are valid — only explicitly set values override defaults.

- **Scenario:** `.forgectl/` found two levels up from current directory.
  - **Expected:** Project root is the directory containing `.forgectl/`. All relative paths resolve from there.
  - **Rationale:** The scaffold supports working from any subdirectory within the project.

- **Scenario:** `[[domains]]` config has two domains with nested paths (e.g., `domains/users` and `domains/users/employees`).
  - **Expected:** Config validation error: `Domain paths must not be nested: domains/users is a prefix of domains/users/employees.` Exit code 1.
  - **Rationale:** Nested domain paths create ambiguity in domain resolution from file paths.

- **Scenario:** Spec queue entry references domain `emails` but no `[[domains]]` entry with that name exists.
  - **Expected:** Error listing the unrecognized domain. Exit code 1.
  - **Rationale:** When domains are configured, all spec queue domains must match. Unconfigured domains are rejected at the boundary.

- **Scenario:** No `[[domains]]` section in config. Spec queue has multiple domains.
  - **Expected:** Init succeeds. Domain paths are derived from spec file paths.
  - **Rationale:** Domain configuration is optional. Without it, domain resolution falls back to file path derivation.

---

## Testing Criteria

### Init defaults to specifying phase
- **Verifies:** Default phase selection.
- **Given:** Valid `.forgectl/config` with defaults.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** `phase: "specifying"`, `state: "ORIENT"`, `started_at_phase: "specifying"`.

### Init at planning phase
- **Verifies:** Phase selection with `--phase planning`.
- **Given:** Valid `.forgectl/config`.
- **When:** `forgectl init --phase planning --from plans-queue.json`
- **Then:** `phase: "planning"`, `state: "ORIENT"`. Specifying section is null.

### Init at implementing phase
- **Verifies:** Phase selection with `--phase implementing` and plan.json mutation.
- **Given:** Valid `.forgectl/config`.
- **When:** `forgectl init --phase implementing --from plan.json`
- **Then:** `phase: "implementing"`, `state: "ORIENT"`. plan.json items have `passes` and `rounds`.

### Init rejects missing .forgectl directory
- **Verifies:** Project root discovery failure.
- **Given:** No `.forgectl/` in current directory or any ancestor.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** Exit code 1.

### Init rejects invalid config
- **Verifies:** Config validation catches constraint violations.
- **Given:** `.forgectl/config` has `specifying.eval.min_rounds = 5` and `specifying.eval.max_rounds = 2`.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** Exit code 1.

### Init rejects existing state
- **Verifies:** Rejection when state file already exists.
- **Given:** State file exists at configured `state_dir`.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** Exit code 1.

### Init rejects invalid queue
- **Verifies:** Schema validation catches missing required fields.
- **Given:** Spec queue missing `file` field.
- **When:** `forgectl init --from bad-queue.json`
- **Then:** Exit code 1.

### Init locks config into state file
- **Verifies:** Config from `.forgectl/config` is persisted in state file.
- **Given:** `.forgectl/config` with `specifying.batch = 5`, `implementing.eval.max_rounds = 7`.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** State file has `config.specifying.batch: 5`, `config.implementing.eval.max_rounds: 7`.

### Init applies defaults for missing config values
- **Verifies:** Partial config falls back to defaults.
- **Given:** `.forgectl/config` with only `[specifying]` section, no `[implementing]`.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** State file has `config.implementing.batch: 2`, `config.implementing.eval.min_rounds: 1`, `config.implementing.eval.max_rounds: 3`.

### Init discovers project root from subdirectory
- **Verifies:** Directory hierarchy walk.
- **Given:** `.forgectl/` at `/project/`, current directory is `/project/api/internal/`.
- **When:** `forgectl init --from specs-queue.json`
- **Then:** Project root is `/project/`. State file created at `/project/.forgectl/state/`.

---

## Implements
- Phase-selectable init (`--phase specifying|planning|implementing`)
- Input validation for spec queue, plan queue, and plan.json schemas
- Project root discovery via `.forgectl/` directory walk
- Config read from `.forgectl/config` (TOML) with defaults
- Phase-scoped config locked into state file at init
- Session ID generation (UUID v4) at init
- `[logs]` config validation and log pruning at init
- Session log file creation at `~/.forgectl/logs/`
