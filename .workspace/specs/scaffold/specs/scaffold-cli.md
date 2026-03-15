# Scaffold CLI

## Topic of Concern
> The scaffold CLI manages spec generation lifecycle state through a JSON-backed state machine with validated input and deterministic transitions.

## Context

The spec generation process involves multiple states (orient, select, draft, evaluate, refine, review, accept) applied to a queue of specifications. Without persistent state, an architect loses track of progress across sessions. The scaffold is a Go CLI tool (built with Cobra) that reads and writes a single JSON state file, enforcing valid transitions and providing the architect with unambiguous next-step guidance.

The scaffold tracks evaluation history — deficiencies found, fixes applied, and verdicts — creating an audit trail from draft to acceptance.

## Depends On
- None. The scaffold is a standalone tool with no runtime dependencies on other project components.

## Integration Points

| Component | Relationship |
|-----------|-------------|
| Spec generation skill | The skill document describes the process; the scaffold enforces the state machine that drives it |
| Evaluation sub-agent | The EVALUATE state is where the architect spawns a sub-agent; the scaffold tracks round count, verdict, deficiencies, and fixes |
| Queue input file | The architect generates a JSON file conforming to the queue schema; the scaffold validates and ingests it during init |
| Eval output directory | The eval sub-agent writes structured output to `<project>/specs/.eval/`; the scaffold does not read these files but the convention is documented |

---

## Interface

### Inputs

#### Queue Input File (provided via `--from` on `init`)

A JSON file conforming to this schema:

```json
{
  "specs": [
    {
      "name": "Repository Loading",
      "domain": "optimizer",
      "topic": "The optimizer clones or locates a repository and provides its path for downstream modules",
      "file": "optimizer/specs/repository-loading.md",
      "planning_sources": [
        ".workspace/planning/optimizer/repo-snapshot-loading.md"
      ],
      "depends_on": []
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `specs` | array | yes | Ordered list of specs to generate |
| `specs[].name` | string | yes | Display name for the spec (used in status output) |
| `specs[].domain` | string | yes | Domain grouping (e.g., "optimizer", "api", "web-portal", "protocols") |
| `specs[].topic` | string | yes | One-sentence topic of concern |
| `specs[].file` | string | yes | Target file path relative to project root |
| `specs[].planning_sources` | string[] | yes | Planning document paths the spec is derived from; may be empty array |
| `specs[].depends_on` | string[] | yes | Names of specs this one depends on; may be empty array |

No additional fields are permitted.

#### CLI Arguments

| Command | Flags | Description |
|---------|-------|-------------|
| `init` | `--min-rounds N` (default 1), `--max-rounds N` (required), `--from <path>` (required), `--user-guided` (optional, default false) | Initialize state file from a validated queue |
| `next` | none | Print the current state and what the architect does now |
| `advance` | `--file <path>` (optional, DRAFT only), `--verdict PASS\|FAIL` (EVALUATE or REVIEW), `--message <text>` (required with PASS in EVALUATE), `--deficiencies <csv>` (with FAIL in EVALUATE), `--fixed <text>` (in REFINE) | Transition from current state to next |
| `status` | none | Print full session state: current spec, eval history, queue, completed |

### Outputs

All output is to stdout. The scaffold writes state changes to `scaffold-state.json`.

#### `next` output

Prints a structured block:

```
State:   REFINE
ID:      3
Spec:    Repository Loading
Domain:  optimizer
File:    optimizer/specs/repository-loading.md
Round:   1/3
Deficiencies: [Completeness, Format Compliance]
Action:  Address deficiencies from evaluation. Edit the spec file. Advance with --fixed <description>.
```

#### `status` output

Prints session config, current spec with eval history, queue grouped by domain, and completed specs with eval trail.

#### `init` validation output (on failure)

When the input file fails validation, the scaffold prints:
1. Each validation error (missing field, extra field, wrong type) with the path to the offending location.
2. The complete valid schema as a reference.

The scaffold exits with a non-zero code on validation failure.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `init` called when `scaffold-state.json` already exists | Error: "State file already exists. Delete it to reinitialize." Exit code 1. | Prevents accidental loss of in-progress state |
| `--from` file fails schema validation | Error listing violations. Prints full valid schema. Exit code 1. | Architect needs to see what's wrong |
| `--min-rounds` exceeds `--max-rounds` | Error: "--min-rounds cannot exceed --max-rounds." Exit code 1. | Invalid configuration |
| `advance` called with `--file` outside of DRAFT state | Error naming the current state. Exit code 1. | Flag is meaningless outside DRAFT |
| `advance` called with `--verdict` outside of EVALUATE or REVIEW | Error naming the current state. Exit code 1. | Verdict is only valid in these states |
| `advance` called in EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` called with `--verdict PASS` in EVALUATE without `--message` | Error. Exit code 1. | Accepted specs must be committed |
| `next` or `advance` called before `init` | Error. Exit code 1. | State file must exist |

---

## Behavior

### Initializing a Session

#### Preconditions
- No `scaffold-state.json` exists.
- `--from`, `--max-rounds` are provided.
- `--min-rounds` <= `--max-rounds`.

#### Steps
1. Read and parse the file at `--from`.
2. Validate against the queue schema (same rules as before).
3. If validation fails: print errors and schema, exit code 1.
4. If validation passes: create `scaffold-state.json` with state ORIENT.

#### Postconditions
- State file exists with `min_rounds`, `max_rounds`, `user_guided` set, queue populated, completed empty.

#### Error Handling
- File not found, invalid JSON, schema failure: same as before.

---

### Advancing State

#### Preconditions
- `scaffold-state.json` exists.

#### Steps

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | always | SELECT | Pull next from queue into `current_spec` |
| SELECT | always | DRAFT | — |
| DRAFT | always | EVALUATE | If `--file` provided, override file path. Set round to 1. |
| EVALUATE | `--verdict PASS` | ACCEPT | Record eval (PASS). Auto-commit with `--message`. |
| EVALUATE | `--verdict FAIL`, round < `min_rounds` | REFINE | Record eval (FAIL + deficiencies). Auto-refine. |
| EVALUATE | `--verdict FAIL`, round >= `min_rounds` | REVIEW | Record eval (FAIL + deficiencies). Human decides. |
| REFINE | always | EVALUATE | Record `--fixed` on last eval. Increment round. |
| REVIEW | no verdict or `--verdict PASS` | ACCEPT | User accepts. |
| REVIEW | `--verdict FAIL` | REFINE | User grants extra round. |
| ACCEPT | queue non-empty | ORIENT | Move spec to completed (with eval history + commit hash). |
| ACCEPT | queue empty | DONE | Move spec to completed. |

#### Postconditions
- State file reflects the new state.
- Eval records accumulate on `current_spec.evals` and carry to `completed[].evals`.

#### Error Handling
- Invalid flags for state: specific error per state.
- Invalid verdict value: error.

---

### Eval Output Convention

The evaluation sub-agent writes structured markdown to a known directory:

```
<project>/specs/.eval/
├── <spec-name>-r1.md
├── <spec-name>-r2.md
└── ...
```

The scaffold does not read or write these files. This is a convention for the architect and sub-agent. The file name includes the spec name (kebab-case) and round number. Each file contains the sub-agent's full evaluation output.

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `--min-rounds` | integer | 1 | Minimum evaluation rounds before REVIEW |
| `--max-rounds` | integer | none (required) | Maximum evaluation rounds per spec |
| `--user-guided` | boolean | false | When set, SELECT state pauses for user discussion |
| `--from` | string | none (required on init) | Path to queue input JSON file |

---

## State File Schema

```json
{
  "min_rounds": 1,
  "max_rounds": 3,
  "user_guided": true,
  "state": "REFINE",
  "last_commit_hash": "",
  "current_spec": {
    "id": 2,
    "name": "Repository Loading",
    "domain": "optimizer",
    "topic": "The optimizer clones or locates a repository...",
    "file": "optimizer/specs/repository-loading.md",
    "planning_sources": [".workspace/planning/optimizer/repo-snapshot-loading.md"],
    "depends_on": [],
    "round": 1,
    "evals": [
      {
        "round": 1,
        "verdict": "FAIL",
        "deficiencies": ["Completeness", "Format Compliance"],
        "fixed": ""
      }
    ]
  },
  "queue": [],
  "completed": [
    {
      "id": 1,
      "name": "Configuration Models",
      "domain": "optimizer",
      "file": "optimizer/specs/configuration-models.md",
      "rounds_taken": 2,
      "commit_hash": "a1b2c3d",
      "evals": [
        { "round": 1, "verdict": "FAIL", "deficiencies": ["Precision"], "fixed": "Removed phantom WARN log" },
        { "round": 2, "verdict": "PASS", "deficiencies": null, "fixed": "" }
      ]
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `min_rounds` | integer | Minimum eval rounds before REVIEW |
| `max_rounds` | integer | Maximum eval rounds per spec |
| `user_guided` | boolean | Whether SELECT pauses for discussion |
| `state` | string | ORIENT, SELECT, DRAFT, EVALUATE, REFINE, REVIEW, ACCEPT, DONE |
| `current_spec.evals` | array | Evaluation history for the current spec |
| `current_spec.evals[].round` | integer | Which round this eval was |
| `current_spec.evals[].verdict` | string | PASS or FAIL |
| `current_spec.evals[].deficiencies` | string[] | Failed dimension names (on FAIL) |
| `current_spec.evals[].fixed` | string | What was fixed after this eval (populated during REFINE) |
| `completed[].evals` | array | Full eval history carried from current_spec |

---

## Invariants

1. **Single active spec.** At most one spec is in `current_spec` at any time.
2. **Round monotonicity.** The round counter only increments.
3. **Min/max round bounds.** FAIL below `min_rounds` auto-refines. FAIL at or above `min_rounds` goes to REVIEW. REVIEW can grant rounds beyond `max_rounds`.
4. **Queue order preserved.** Specs are pulled from the front of the queue.
5. **State file is the only mutable artifact.** The queue input file is read once at init.
6. **No implicit state.** All information for transitions is in the state file.
7. **Eval history is append-only.** Eval records accumulate and are never deleted or modified (except `fixed` is set during REFINE).

---

## Edge Cases

- **Scenario:** `advance --verdict FAIL` when round < `min_rounds`.
  - **Expected behavior:** Transition to REFINE (auto-refine, no REVIEW).
  - **Rationale:** Below minimum rounds, the architect must try to fix before escalating.

- **Scenario:** `advance --verdict FAIL` when round >= `min_rounds` and <= `max_rounds`.
  - **Expected behavior:** Transition to REVIEW.
  - **Rationale:** Past minimum, the human decides whether to accept or grant another round.

- **Scenario:** REVIEW grants extra round beyond `max_rounds`.
  - **Expected behavior:** REFINE → EVALUATE proceeds. The round counter increments past `max_rounds`.
  - **Rationale:** `max_rounds` controls auto-behavior. The human can override.

- **Scenario:** REVIEW with `--verdict PASS`.
  - **Expected behavior:** Transition to ACCEPT.
  - **Rationale:** Explicit acceptance from REVIEW.

- **Scenario:** REVIEW with no verdict.
  - **Expected behavior:** Transition to ACCEPT (default accept).
  - **Rationale:** Advancing from REVIEW without a verdict means the human reviewed and accepted.

- **Scenario:** Eval has no deficiencies recorded.
  - **Expected behavior:** The `deficiencies` field is an empty array or null. REFINE still works — the architect addresses issues without scaffold-tracked deficiencies.
  - **Rationale:** The `--deficiencies` flag is optional. Not all evaluations may produce structured deficiency names.

---

## Testing Criteria

### Init with min and max rounds
- **Verifies:** Initializing a Session behavior
- **Given:** `--min-rounds 2 --max-rounds 5`
- **When:** `scaffold init`
- **Then:** State file has `min_rounds: 2`, `max_rounds: 5`.

### Min exceeds max rejected
- **Verifies:** Rejection table
- **Given:** `--min-rounds 5 --max-rounds 2`
- **When:** `scaffold init`
- **Then:** Exit code 1.

### FAIL below min_rounds auto-refines
- **Verifies:** Invariant 3, Edge case
- **Given:** `min_rounds: 2`, round 1
- **When:** `advance --verdict FAIL`
- **Then:** State is REFINE (not REVIEW).

### FAIL at min_rounds goes to REVIEW
- **Verifies:** Invariant 3, Edge case
- **Given:** `min_rounds: 1`, round 1
- **When:** `advance --verdict FAIL`
- **Then:** State is REVIEW.

### REVIEW accept
- **Verifies:** Edge case: REVIEW with no verdict
- **Given:** State is REVIEW
- **When:** `advance` (no verdict)
- **Then:** State is ACCEPT.

### REVIEW grants extra round
- **Verifies:** Edge case: REVIEW grants extra round
- **Given:** State is REVIEW
- **When:** `advance --verdict FAIL`
- **Then:** State is REFINE.

### Deficiencies recorded on FAIL
- **Verifies:** Eval history, Invariant 7
- **Given:** State is EVALUATE
- **When:** `advance --verdict FAIL --deficiencies "Completeness,Precision"`
- **Then:** `current_spec.evals` has an entry with `deficiencies: ["Completeness", "Precision"]`.

### Fixed recorded on REFINE
- **Verifies:** Eval history
- **Given:** State is REFINE
- **When:** `advance --fixed "Added Observability section"`
- **Then:** Last eval record has `fixed: "Added Observability section"`.

### Eval history carried to completed
- **Verifies:** Invariant 7
- **Given:** A spec with 2 eval rounds (FAIL then PASS)
- **When:** The spec reaches ACCEPT and is moved to completed
- **Then:** `completed[].evals` has both records.

### PASS requires message
- **Verifies:** Rejection table
- **Given:** State is EVALUATE
- **When:** `advance --verdict PASS` without `--message`
- **Then:** Exit code 1.

### Full lifecycle with REVIEW
- **Verifies:** All states including REVIEW
- **Given:** Init with 1 spec, `--min-rounds 1 --max-rounds 1`
- **When:** ORIENT → SELECT → DRAFT → EVALUATE(FAIL) → REVIEW → ACCEPT → DONE
- **Then:** Completed has 1 entry. Evals show the FAIL.

---

## Implements
- Scaffold state machine design from spec generation skill process
- Eval tracking proposals: deficiency recording, fix tracking, REVIEW state, min/max rounds, eval output convention
