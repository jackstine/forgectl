# Forgectl Scaffold

## Topic of Concern
> The forgectl scaffold manages the full software development lifecycle — from spec generation through implementation planning to code implementation — through a JSON-backed state machine with three phases (specifying, planning, implementing), explicit phase shift checkpoints, validated transitions, structured evaluation loops, and durable state management.

## Context

The scaffold guides three sequential phases of work:

1. **Specifying**: An architect drafts specs from planning documents, evaluates them through sub-agent rounds, refines on deficiency, accepts, and reconciles cross-references across all specs.

2. **Planning**: An architect studies accepted specs, codebase, and packages, then drafts a structured implementation plan (`plan.json`). The plan is validated and evaluated against the specs through iterative rounds until accepted.

3. **Implementing**: A senior engineer receives plan items one at a time within dependency-ordered batches, implements each, then an evaluation sub-agent verifies the batch against acceptance criteria through iterative rounds.

Between phases, a **PHASE_SHIFT** state acts as an explicit checkpoint — the user is told to stop and refresh their context before proceeding. The scaffold can be initialized at any phase, allowing users to skip earlier phases when inputs already exist.

The scaffold is a Go CLI tool (built with Cobra) that reads and writes a single JSON state file (`forgectl-state.json`), enforcing valid transitions and providing unambiguous next-step guidance. A `phase` field in the state file tracks which phase is active. State names (ORIENT, EVALUATE) are reused across phases with phase-specific behavior.

State file writes use atomic rename with backup for crash safety. On startup, the scaffold detects and recovers from interrupted writes.

## Depends On
- None. The scaffold is a standalone tool with no runtime dependencies on other project components.

## Integration Points

| Component | Relationship |
|-----------|-------------|
| Spec generation skill | The skill document describes the spec authoring process; the scaffold enforces the state machine that sequences it |
| Spec generation sub-agent | The specifying EVALUATE state is where the architect spawns a sub-agent; the scaffold tracks round count, verdict, and eval report path |
| SPEC_MANIFEST.md | Planning STUDY_SPECS reads this manifest to locate spec files relevant to the plan |
| Plan format definition (`PLAN_FORMAT.md`) | Defines the JSON schema for `plan.json` and conventions for notes files. Referenced during REVIEW, DRAFT, and validation in the planning phase. |
| Plan evaluator prompt (`evaluators/plan-eval.md`) | Full instructions for the planning evaluation sub-agent: dimensions, report format, verdict rules |
| Implementation evaluator prompt (`evaluators/impl-eval.md`) | Full instructions for the implementation evaluation sub-agent: what to check, report format, verdict rules |
| Eval output directory | The specifying eval sub-agent writes output to `<project>/specs/.eval/`; the scaffold does not read these files but the convention is documented |
| Queue input files | The user generates JSON files conforming to phase-specific queue schemas; the scaffold validates and ingests them during init or at phase shifts |
| plan.json | Planning produces it; implementing consumes and mutates it (adding `passes` and `rounds` fields) |

---

## Interface

### Inputs

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
      "name": "Protocol Implementation",
      "domain": "protocols",
      "topic": "Implementation plan for WS1 and WS2 message contract specs",
      "file": "protocols/.workspace/implementation_plan/plan.json",
      "specs": [
        "protocols/ws1/specs/ws1-message-contract.md",
        "protocols/ws2/specs/ws2-message-contract.md"
      ],
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
| `plans[].topic` | string | yes | One-sentence topic of concern |
| `plans[].file` | string | yes | Target path for plan.json relative to project root |
| `plans[].specs` | string[] | yes | Spec file paths to study; may be empty array |
| `plans[].code_search_roots` | string[] | yes | Directory roots for codebase exploration; may be empty array |

No additional fields are permitted.

#### Plan.json Input File (`--phase implementing`)

A `plan.json` file conforming to the schema defined in `PLAN_FORMAT.md`. The scaffold validates the full plan structure during init and adds `passes` and `rounds` fields to each item.

#### CLI Commands

| Command | Flags | Description |
|---------|-------|-------------|
| `init` | `--from <path>` (required), `--batch-size N` (required), `--min-rounds N` (default 1), `--max-rounds N` (required), `--phase specifying\|planning\|implementing` (default specifying), `--guided` / `--no-guided` (default guided) | Initialize state file from validated input |
| `advance` | `--guided` / `--no-guided` (optional, any state), plus phase-specific flags (see below) | Transition from current state to next |
| `status` | none | Print current state with action guidance + full session overview |
| `eval` | none | Output full evaluation context for the sub-agent. Only valid in EVALUATE states (planning and implementing phases). |
| `add-commit` | `--id N` (required), `--hash <hash>` (required) | Register a commit hash to a completed spec. Hash validated against git. Duplicates rejected. |
| `reconcile-commit` | `--hash <hash>` (required) | Auto-register a commit to all completed specs whose files were touched. Runs `git show --name-only` to match files. |

#### `advance` flags by phase and state

| Phase | State | Flags |
|-------|-------|-------|
| Specifying | DRAFT | `--file <path>` (optional, override file path) |
| Specifying | EVALUATE | `--verdict PASS\|FAIL`, `--eval-report <path>` (required), `--message <text>` (required with PASS) |
| Specifying | REFINE | (no flags) |
| Specifying | RECONCILE_EVAL | `--verdict PASS\|FAIL`, `--message <text>` (required with PASS) |
| Specifying | RECONCILE_REVIEW | `--verdict FAIL` (optional; no verdict = accept) |
| Phase Shift | specifying → planning | `--from <path>` (required, plan queue JSON) |
| Phase Shift | planning → implementing | (no additional flags) |
| Planning | EVALUATE | `--verdict PASS\|FAIL`, `--eval-report <path>` (both required) |
| Planning | ACCEPT | `--message <text>` (required) |
| Implementing | EVALUATE | `--verdict PASS\|FAIL`, `--eval-report <path>` (both required) |
| Implementing | IMPLEMENT | `--message <text>` (required, first round only) |
| Implementing | COMMIT | `--message <text>` (required) |
| *(all other states)* | | no phase-specific flags |

The `--guided` / `--no-guided` flags are accepted on any `advance` call regardless of phase or state. They update the `user_guided` setting in the state file.

### Outputs

All output is to stdout. The scaffold writes state changes to `forgectl-state.json` and (during implementing phase) updates `passes` and `rounds` fields in plan.json.

#### `advance` output — Specifying Phase

**Entering SELECT** (after ORIENT):

```
State:   SELECT
Phase:   specifying
ID:      1
Spec:    Repository Loading
Domain:  optimizer
File:    optimizer/specs/repository-loading.md
Topic:   The optimizer clones or locates a repository and provides its path for downstream modules
Sources: .workspace/planning/optimizer/repo-snapshot-loading.md
Action:  Review topic and planning sources.
         Stop and review and discuss with user before continuing.
         Advance to begin drafting.
```

**Entering DRAFT** (after SELECT):

```
State:   DRAFT
Phase:   specifying
ID:      1
Spec:    Repository Loading
Domain:  optimizer
File:    optimizer/specs/repository-loading.md
Action:  Draft the spec. Advance when ready.
         Use --file <path> if the file path changed.
```

**Entering EVALUATE** (after DRAFT or REFINE):

```
State:   EVALUATE
Phase:   specifying
ID:      1
Spec:    Repository Loading
Domain:  optimizer
File:    optimizer/specs/repository-loading.md
Round:   1/3
Action:  Spawn evaluation sub-agent against the spec.
         Eval output: optimizer/specs/.eval/repository-loading-r1.md
         Advance with --verdict PASS --eval-report <path> --message <commit msg>
           or --verdict FAIL --eval-report <path>
```

**Entering REFINE** (after EVALUATE FAIL or PASS below min_rounds):

```
State:   REFINE
Phase:   specifying
ID:      1
Spec:    Repository Loading
Domain:  optimizer
File:    optimizer/specs/repository-loading.md
Round:   1/3
Action:  Read the eval report and address any findings in the spec file.
         Eval report: optimizer/specs/.eval/repository-loading-r1.md
         When changes are complete, run: forgectl advance
```

**Entering ACCEPT** (after EVALUATE PASS, round >= min_rounds):

```
State:   ACCEPT
Phase:   specifying
ID:      1
Spec:    Repository Loading
Domain:  optimizer
File:    optimizer/specs/repository-loading.md
Round:   2/3
Commit:  a1b2c3d
Action:  Spec accepted. Advance to continue.
```

**Entering DONE** (after ACCEPT, queue empty — all individual specs complete):

```
State:   DONE
Phase:   specifying
Specs:   5 completed
Action:  All individual specs complete. Advance to begin reconciliation.
```

**Entering RECONCILE** (after DONE):

```
State:   RECONCILE
Phase:   specifying
Domain:  optimizer
Specs:   5 completed
Action:  Cross-validate all specs: verify Depends On entries, Integration Points
         symmetry, naming consistency. Stage changes with git add.
         Advance when ready.
```

**Entering RECONCILE_EVAL** (after RECONCILE):

```
State:   RECONCILE_EVAL
Phase:   specifying
Round:   1
Action:  Tell the sub-agent to run git diff --staged and evaluate
         consistency across all specs.
         Advance with --verdict PASS --message <commit msg>
           or --verdict FAIL.
```

**Entering RECONCILE_REVIEW** (after RECONCILE_EVAL FAIL):

```
State:   RECONCILE_REVIEW
Phase:   specifying
Round:   1
Action:  Reconciliation eval found issues.
         Accept: advance (or --verdict PASS)
         Fix and re-evaluate: advance --verdict FAIL
```

**Entering COMPLETE** (after RECONCILE_EVAL PASS or RECONCILE_REVIEW accept):

```
State:   COMPLETE
Phase:   specifying
Specs:   5 completed, reconciled
Action:  Specifying phase complete. Advance to continue.
```

#### `advance` output — Phase Shifts

**Entering PHASE_SHIFT** (specifying → planning):

```
State:   PHASE_SHIFT
From:    specifying → planning

Stop and refresh your context, please.
When ready, run: forgectl advance --from <plans-queue.json>
```

**Entering PHASE_SHIFT** (planning → implementing):

```
State:   PHASE_SHIFT
From:    planning → implementing
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json

Stop and refresh your context, please.
When ready, run: forgectl advance
```

#### `advance` output — Planning Phase

**Entering ORIENT** (after PHASE_SHIFT or init with `--phase planning`):

```
State:   ORIENT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Action:  Advance to begin studying specs.
```

**Entering STUDY_SPECS** (after ORIENT):

```
State:   STUDY_SPECS
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Specs:   launcher/specs/service-configuration.md, ...
Roots:   launcher/, api/
Action:  Study the specs: launcher/specs/service-configuration.md, ...
         Review git diffs for spec commits. Advance when done.
```

**Entering STUDY_CODE** (after STUDY_SPECS):

```
State:   STUDY_CODE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Roots:   launcher/, api/
Action:  Explore the codebase in relation to the specs under study.
         Sub-agents: 3. Search roots: launcher/, api/.
         Advance when done.
```

**Entering STUDY_PACKAGES** (after STUDY_CODE):

```
State:   STUDY_PACKAGES
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Action:  Study the project's technical stack: package manifests, library docs, CLAUDE.md references.
         Advance when done.
```

**Entering REVIEW** (after STUDY_PACKAGES):

```
State:   REVIEW
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Action:  Review study findings before drafting.
         Plan format: PLAN_FORMAT.md
         Stop and review and discuss with user before continuing.
         Advance to begin drafting.
```

**Entering DRAFT** (after REVIEW):

```
State:   DRAFT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Action:  Draft the implementation plan.
         Output: plan.json + notes/ at launcher/.workspace/implementation_plan/
         Format: PLAN_FORMAT.md
         Advance when plan and notes are ready.
```

**Entering EVALUATE** (after DRAFT or REFINE, when validation passes):

```
State:   EVALUATE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Round:   1/3
Action:  Run evaluation sub-agent against the plan (round 1/3).
         Sub-agent: forgectl eval
         Advance with --verdict PASS|FAIL --eval-report <path>.
```

**Entering VALIDATE** (after DRAFT or REFINE, when validation fails):

```
State:   VALIDATE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Action:  Plan validation failed. Fix the plan and advance to re-validate.
         Format: PLAN_FORMAT.md

FAIL: 3 errors in plan.json

  items[2]: missing required field "depends_on"
    depends_on (string[]): Item IDs that must be complete before this item can begin.

  items[5]: unexpected field "status"
    status is not a valid field. Item status is computed from tests, not stored.

  layers[1].items[3]: references non-existent item "config.typez"
    Layer items must reference valid item IDs from the items array.
```

**Entering REFINE** (after EVALUATE with FAIL verdict):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Round:   1/3
Action:  Evaluation found deficiencies. Spawn a sub-agent to update the plan and notes.
         Eval report: launcher/.workspace/implementation_plan/evals/round-1.md
         Advance when plan is updated.
```

**Entering REFINE** (after EVALUATE with PASS verdict, below min_rounds):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Round:   1/3
Action:  Minimum evaluation rounds not met. Spawn a sub-agent to re-evaluate the plan.
         Eval report: launcher/.workspace/implementation_plan/evals/round-1.md
         Advance to proceed to next evaluation round.
```

**Entering ACCEPT** (after EVALUATE with PASS verdict, at or above min_rounds):

```
State:   ACCEPT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Round:   2/3
Action:  Plan accepted.
         Run: forgectl advance --message <commit msg>
```

**Entering ACCEPT** (forced, after EVALUATE with FAIL verdict at max_rounds):

```
State:   ACCEPT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Round:   3/3
Action:  Plan accepted (max rounds reached).
         Run: forgectl advance --message <commit msg>
```

#### `advance` output — Implementing Phase

**Entering ORIENT** (after PHASE_SHIFT or init with `--phase implementing`):

```
State:   ORIENT
Phase:   implementing
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.workspace/implementation_plan/plan.json
Config:  batch_size=2, rounds=1-3

Initialized plan.json for implementation:
  Items:  5 (passes: pending, rounds: 0)
  Layers: 2 (L0 Foundation: 3 items, L1 Core: 2 items)

Action:  Stop and review and discuss with user before continuing.
         Selecting first batch. Run: forgectl advance
```

**Entering IMPLEMENT** (first item in batch, first round — no prior eval):

```
State:   IMPLEMENT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Item:    [config.types] ServiceEndpoint and ServicesConfig structs
         Go structs for validated service endpoint configuration.
         (1 of 2 in batch)
Steps:
  1. Define ServiceEndpoint struct with Host (string) and Port (int) fields
  2. Define ServicesConfig struct with three named ServiceEndpoint fields
  3. Add YAML struct tags for deserialization
Files:   internal/config/types.go
Spec:    service-configuration.md#interface-outputs
Ref:     notes/config.md#types
Tests:   1 functional
Action:  Implement this item.
         When complete, run: forgectl advance --message <commit msg>
```

**Entering IMPLEMENT** (next item in same batch, first round):

```
State:   IMPLEMENT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Item:    [config.load] Load YAML, apply defaults, validate strictly
         Parse spectacular.yml, apply default host/port values.
         (2 of 2 in batch)
Steps:
  1. Implement LoadConfig() using goccy/go-yaml strict mode
  2. Add default port logic (portal=8080, api=8081, optimizer=8082)
  3. Add post-unmarshal validation for port range and empty host
  4. Write table-driven tests for valid, rejection, and edge cases
Files:   internal/config/load.go, internal/config/load_test.go
Spec:    service-configuration.md#behavior-loading
Ref:     notes/config.md#load
Tests:   2 functional, 2 rejection, 2 edge_case
Action:  Implement this item.
         When complete, run: forgectl advance --message <commit msg>
```

**Entering IMPLEMENT** (first item in batch, after eval — round 2+):

```
State:   IMPLEMENT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Round:   1/3
Eval:    launcher/.workspace/implementation_plan/evals/batch-1-round-1.md
Note:    PASS recorded for round 1. Minimum rounds not yet met (1/2).
Item:    [config.types] ServiceEndpoint and ServicesConfig structs
         Go structs for validated service endpoint configuration.
         (1 of 2 in batch)
Steps:
  1. Define ServiceEndpoint struct with Host (string) and Port (int) fields
  2. Define ServicesConfig struct with three named ServiceEndpoint fields
  3. Add YAML struct tags for deserialization
Files:   internal/config/types.go
Spec:    service-configuration.md#interface-outputs
Ref:     notes/config.md#types
Tests:   1 functional
Action:  Study the eval file "launcher/.workspace/implementation_plan/evals/batch-1-round-1.md"
         and implement any corrections as needed. If none found during the eval,
         please verify and look for corrections. Apply them.
         When complete, run: forgectl advance
```

**Entering EVALUATE** (implementing phase):

```
State:    EVALUATE
Phase:    implementing
Layer:    L0 Foundation
Batch:    1/2
Round:    1/3
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Action:   Ask the evaluation sub-agent to verify batch items against their tests.
          The sub-agent should run: forgectl eval
          After reviewing the eval report, run:
            forgectl advance --eval-report <path> --verdict PASS|FAIL
Sub-agent: forgectl eval
```

**Entering COMMIT** (after EVALUATE, batch terminal):

```
State:   COMMIT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Items:
  - [config.types] passed
  - [config.load] passed
Action:  Commit your changes before continuing.
         When ready, run: forgectl advance --message <commit msg>
```

**Entering COMMIT** (after force-accept):

```
State:   COMMIT
Phase:   implementing
Layer:   L1 Core
Batch:   3/3
Items:
  - [daemon.types] failed (force-accept, 3/3 rounds)
  - [daemon.io] failed (force-accept, 3/3 rounds)
Action:  Commit your changes before continuing.
         When ready, run: forgectl advance --message <commit msg>
```

**Entering ORIENT** (after COMMIT, more items in layer):

```
State:    ORIENT
Phase:    implementing
Layer:    L0 Foundation
Progress: 2/3 items passed
Action:   Stop and review and discuss with user before continuing.
          Selecting next batch. Run: forgectl advance
```

**Entering ORIENT** (after COMMIT, layer complete):

```
State:    ORIENT
Phase:    implementing
Layer:    L0 Foundation
Progress: 3/3 items passed — layer complete
Action:   Stop and review and discuss with user before continuing.
          Advancing to next layer. Run: forgectl advance
```

**Entering ORIENT** (force-accept):

```
State:    ORIENT
Phase:    implementing
Layer:    L1 Core
          FORCE ACCEPT: 2 items marked failed (max rounds 3/3 reached)
          - [daemon.types] Daemon state types and PID file struct
          - [daemon.io] PID file I/O operations
Progress: 2/2 items terminal (0 passed, 2 failed) — layer complete
Action:   Advancing to next layer. Run: forgectl advance
```

**DONE** (all items complete, terminal state):

```
State:   DONE
Phase:   implementing
Summary:
  L0 Foundation:  3/3 passed
  L1 Core:        2/2 passed
  Total:          5/5 items passed
  Eval rounds:    7 across 3 batches
Action:  All items complete. Session done.
```

#### `eval` output — Planning Phase

```
=== PLAN EVALUATION ROUND 1/3 ===
Plan:   Service Configuration
Domain: launcher
File:   launcher/.workspace/implementation_plan/plan.json

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/plan-eval.md>

--- PLAN REFERENCES ---

Plan:    launcher/.workspace/implementation_plan/plan.json
Format:  PLAN_FORMAT.md
Specs:
  - launcher/specs/service-configuration.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.workspace/implementation_plan/evals/round-1.md
```

Planning eval, subsequent rounds:

```
=== PLAN EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: FAIL — launcher/.workspace/implementation_plan/evals/round-1.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.workspace/implementation_plan/evals/round-2.md
```

#### `eval` output — Implementing Phase

```
=== IMPLEMENTATION EVALUATION ROUND 1/3 ===
Layer: L0 Foundation
Batch: 1/2

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/impl-eval.md>

--- ITEMS TO EVALUATE ---

[1] config.types — ServiceEndpoint and ServicesConfig structs
    Description: Go structs for validated service endpoint configuration.
    Spec:        service-configuration.md#interface-outputs
    Ref:         notes/config.md#types
    Files:       internal/config/types.go
    Steps:
      1. Define ServiceEndpoint struct with Host (string) and Port (int) fields
      2. Define ServicesConfig struct with three named ServiceEndpoint fields
      3. Add YAML struct tags for deserialization
    Tests:
      [functional] Three named fields, not a map

[2] config.load — Load YAML, apply defaults, validate strictly
    Description: Parse spectacular.yml, apply default host/port values.
    Spec:        service-configuration.md#behavior-loading
    Ref:         notes/config.md#load
    Files:       internal/config/load.go, internal/config/load_test.go
    Steps:
      1. Implement LoadConfig() using goccy/go-yaml strict mode
      2. Add default port logic (portal=8080, api=8081, optimizer=8082)
      3. Add post-unmarshal validation for port range and empty host
      4. Write table-driven tests for valid, rejection, and edge cases
    Tests:
      [functional] Default ports applied when services are empty objects
      [functional] Default host applied when only port specified
      [rejection]  Missing services section rejected
      [rejection]  Port out of range rejected
      [rejection]  Unknown keys rejected
      [edge_case]  Empty file rejected
      [edge_case]  Duplicate ports allowed

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.workspace/implementation_plan/evals/batch-1-round-1.md
```

Implementation eval, subsequent rounds:

```
=== IMPLEMENTATION EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: PASS — launcher/.workspace/implementation_plan/evals/batch-1-round-1.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.workspace/implementation_plan/evals/batch-1-round-2.md
```

#### `status` output

The `status` command prints the current state with action guidance at the top, followed by the full session overview.

**Mid-specifying:**

```
Session: forgectl-state.json
Phase:   specifying
Config:  rounds=1-3, batch_size=2, guided=true

--- Current ---

State:   REFINE
ID:      3
Spec:    Repository Loading (optimizer)
File:    optimizer/specs/repository-loading.md
Round:   1/3
Action:  Read the eval report and address any findings in the spec file.
         Eval report: optimizer/specs/.eval/repository-loading-r1.md
         When changes are complete, run: forgectl advance

--- Queue ---

  [4] Snapshot Diffing (optimizer)
  [5] Portal Rendering (portal)

--- Completed ---

  [1] Configuration Models (optimizer)  — 2 rounds, commit a1b2c3d
       Round 1: FAIL — optimizer/specs/.eval/configuration-models-r1.md
       Round 2: PASS — optimizer/specs/.eval/configuration-models-r2.md
  [2] API Gateway (api)                — 1 round, commit e4f5a6b
       Round 1: PASS — api/specs/.eval/api-gateway-r1.md
```

**Mid-planning:**

```
Session: forgectl-state.json
Phase:   planning
Config:  rounds=1-3, batch_size=2

--- Current ---

State:   EVALUATE
Plan:    Service Configuration (launcher)
File:    launcher/.workspace/implementation_plan/plan.json
Round:   1/3
Action:  Run evaluation sub-agent against the plan (round 1/3).
         Sub-agent: forgectl eval
         Advance with --verdict PASS|FAIL --eval-report <path>.

--- Specifying ---

  Complete (5 specs, reconciled)

--- Planning ---

  Evals: (none yet)

--- Queue ---

  empty
```

**Mid-implementing:**

```
Session: forgectl-state.json
Phase:   implementing
Config:  batch_size=2, rounds=1-3

--- Current ---

State:   IMPLEMENT
Plan:    Service Configuration (launcher)
File:    launcher/.workspace/implementation_plan/plan.json
Layer:   L1 Core (2 items)
Batch:   3/3
Item:    [daemon.io] PID file I/O operations (2 of 2)
Round:   0
Action:  Implement this item.
         When complete, run: forgectl advance --message <commit msg>

--- Specifying ---

  Complete (5 specs)

--- Planning ---

  Accepted (2 rounds)
    Round 1: FAIL — launcher/.workspace/implementation_plan/evals/round-1.md
    Round 2: PASS — launcher/.workspace/implementation_plan/evals/round-2.md

--- Implementing ---

  Layer L0 (Foundation): complete
    [bootstrap]     passed  (1 round)
    [config.types]  passed  (1 round)
    [config.load]   passed  (2 rounds)

  Layer L1 (Core): in progress
    [daemon.types]  done    (0 rounds)
    [daemon.io]     pending (0 rounds)
```

**Started at implementing directly (`--phase implementing`):**

```
Session: forgectl-state.json
Phase:   implementing (started here)
Config:  batch_size=2, rounds=1-3

--- Current ---

State:   EVALUATE
Plan:    launcher/.workspace/implementation_plan/plan.json
Layer:   L0 Foundation (3 items)
Batch:   1/2
Round:   1/3
Items:   [config.types], [config.load]
Action:  Ask the evaluation sub-agent to verify batch items against their tests.
         The sub-agent should run: forgectl eval
         After reviewing the eval report, run:
           forgectl advance --eval-report <path> --verdict PASS|FAIL

--- Implementing ---

  Layer L0 (Foundation): in progress
    [bootstrap]     passed  (1 round)
    [config.types]  done    (1 round)
    [config.load]   done    (1 round)
```

#### `init` validation output (on failure)

When the input file fails validation, the scaffold prints:
1. Each validation error (missing field, extra field, wrong type) with the path to the offending location.
2. The complete valid schema as a reference.

The scaffold exits with a non-zero code on validation failure.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `init` called when `forgectl-state.json` already exists | Error: "State file already exists. Delete it to reinitialize." Exit code 1. | Prevents accidental loss of in-progress state |
| `--from` file fails schema validation | Error listing violations. Prints full valid schema. Exit code 1. | User needs to see what's wrong |
| `--batch-size` < 1 | Error: "--batch-size must be at least 1." Exit code 1. | Invalid configuration |
| `--min-rounds` < 1 | Error: "--min-rounds must be at least 1." Exit code 1. | At least one eval round required |
| `--min-rounds` exceeds `--max-rounds` | Error: "--min-rounds cannot exceed --max-rounds." Exit code 1. | Invalid configuration |
| `--phase` not one of the three values | Error: "--phase must be specifying, planning, or implementing." Exit code 1. | Invalid phase |
| `advance --file` outside of specifying DRAFT | Error naming the current state. Exit code 1. | Flag is meaningless outside DRAFT |
| `advance --verdict` outside of EVALUATE, RECONCILE_EVAL, RECONCILE_REVIEW | Error naming the current state. Exit code 1. | Verdict is only valid in these states |
| `advance` in specifying EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in specifying EVALUATE without `--eval-report` | Error. Exit code 1. | Every evaluation must reference its report |
| `advance --verdict PASS` in specifying EVALUATE without `--message` | Error. Exit code 1. | Accepted specs need a commit message |
| `advance` in planning ACCEPT without `--message` | Error. Exit code 1. | Accepted plans need a commit message |
| `advance` in implementing IMPLEMENT (first round) without `--message` | Error. Exit code 1. | First-round items need a commit message |
| `advance` in implementing COMMIT without `--message` | Error. Exit code 1. | Batch completion needs a commit message |
| `advance` in planning/implementing EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in planning/implementing EVALUATE without `--eval-report` | Error. Exit code 1. | Every evaluation must reference its report |
| `advance --eval-report` pointing to non-existent file | Error naming the path. Exit code 1. | Report must exist to be recorded |
| `eval` outside of planning/implementing EVALUATE | Error naming current state and phase. Exit code 1. | Eval context only available in those states |
| `advance` at PHASE_SHIFT (specifying→planning) without `--from` | Error: "--from <plans-queue.json> is required at this phase shift." Exit code 1. | Planning queue must be provided |
| `add-commit` / `reconcile-commit` with hash not in git | Error: "commit does not exist in the repository." Exit code 1. | Prevents invalid hashes |
| `add-commit` with hash already registered | Error: "commit already registered." Exit code 1. | Prevents duplicates |
| `add-commit` targeting an active (not completed) spec | Error: "spec is still active." Exit code 1. | Commits for completed specs only |
| `advance` or `status` or `eval` called before `init` | Error. Exit code 1. | State file must exist |

---

## Behavior

### State File Durability

#### Atomic Writes

Every state mutation follows this sequence:
1. Write new state to `forgectl-state.json.tmp`.
2. Rename `forgectl-state.json` → `forgectl-state.json.bak`.
3. Rename `forgectl-state.json.tmp` → `forgectl-state.json`.

Steps 2 and 3 are filesystem renames — atomic on POSIX. If the process crashes between steps, the startup recovery logic handles it.

#### Startup Recovery

On every command (before reading state), the scaffold checks:

| Condition | Action |
|-----------|--------|
| `.json` exists, `.tmp` does not | Normal. Proceed. |
| `.json` missing, `.bak` exists | Crashed between step 2 and 3. Rename `.bak` → `.json`. Warn user. |
| `.json` missing, `.tmp` exists | Crashed between step 1 and 2. Rename `.tmp` → `.json`. Warn user. |
| `.json` exists, `.tmp` exists | Crashed after step 1, before cleanup. Delete `.tmp`. Proceed with `.json`. |
| `.json` corrupt (invalid JSON) | Rename `.json` → `.json.corrupt`, rename `.bak` → `.json`. Warn user. |
| None exist | No state. Only `init` is valid. |

#### File Layout

```
forgectl-state.json           ← active state (gitignored)
forgectl-state.json.bak       ← previous state (gitignored)
forgectl-state.json.tmp       ← write-in-progress (transient, gitignored)
sessions/                      ← archived completed sessions (git tracked)
```

### Commit Hash Validation

All commands that accept a commit hash (`add-commit`, `reconcile-commit`, and the auto-commit in specifying `advance --verdict PASS`) validate that the hash exists in git using `git cat-file -t`. The object type must be `commit`. Non-existent hashes, tags, blobs, and tree objects are rejected.

### Registering Commits to Specs

#### add-commit
Appends a commit hash to a specific completed spec by ID. The hash is validated against git and checked for duplicates before appending.

#### reconcile-commit
Runs `git show --name-only <hash>` to determine which files were changed, then matches file paths against `completed[].file`. The hash is appended to every matching spec that doesn't already have it. Reports which specs were updated.

---

### Initializing a Session

#### Preconditions
- No `forgectl-state.json` exists.
- `--from`, `--batch-size`, `--max-rounds` are provided.
- `--min-rounds` <= `--max-rounds`.
- `--batch-size` >= 1, `--min-rounds` >= 1.
- `--phase` is one of `specifying`, `planning`, `implementing` (default: `specifying`).

#### Steps
1. Read and parse the file at `--from`.
2. Validate against the schema for the specified `--phase`.
3. If validation fails: print errors and schema, exit code 1.
4. If validation passes:
   - For `--phase specifying`: create state file with phase `specifying`, state ORIENT, spec queue populated.
   - For `--phase planning`: create state file with phase `planning`, state ORIENT, plan queue populated.
   - For `--phase implementing`: validate plan.json, add `passes: "pending"` and `rounds: 0` to items, create state file with phase `implementing`, state ORIENT.

#### Postconditions
- State file exists with `batch_size`, `min_rounds`, `max_rounds`, `user_guided` set.
- Phase and state reflect the starting point.
- For `--phase implementing`: plan.json items have `passes` and `rounds` fields.

#### Error Handling
- File not found: error with path. Exit code 1.
- Invalid JSON: error with parse details. Exit code 1.
- Schema failure: error listing violations, print valid schema. Exit code 1.

---

## Phase: Specifying

The specifying phase guides the architect through drafting, evaluating, refining, and accepting specs from a queue, followed by cross-reference reconciliation.

### State Machine — Specifying

```
ORIENT → SELECT → DRAFT → EVALUATE
                              │
                    ┌─────────┼──────────┐
                    │         │          │
              PASS ≥ min   FAIL < max  PASS < min
                    │         │          │
                    ▼         ▼          ▼
                 ACCEPT    REFINE     REFINE
                    │         │          │
              ┌─────┘         └────┬─────┘
              │                    │
         queue empty?         EVALUATE
           yes → DONE
           no → ORIENT
                              FAIL ≥ max → ACCEPT (forced)

DONE → RECONCILE → RECONCILE_EVAL
                        │
              ┌─────────┼──────────┐
              │                    │
            PASS                 FAIL
              │                    │
              ▼                    ▼
           COMPLETE         RECONCILE_REVIEW
                              │           │
                           accept        FAIL
                              │           │
                           COMPLETE    RECONCILE
```

### Advancing State — Specifying

#### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | always | SELECT | Pull next from queue into `current_spec` |
| SELECT | always | DRAFT | — |
| DRAFT | always | EVALUATE | If `--file` provided, override file path. Set round to 1. |
| EVALUATE | `--verdict PASS`, round >= `min_rounds` | ACCEPT | Record eval (PASS + eval report). Auto-commit with `--message`. |
| EVALUATE | `--verdict PASS`, round < `min_rounds` | REFINE | Record eval (PASS + eval report). Min rounds not met. |
| EVALUATE | `--verdict FAIL`, round < `max_rounds` | REFINE | Record eval (FAIL + eval report). |
| EVALUATE | `--verdict FAIL`, round >= `max_rounds` | ACCEPT | Record eval (FAIL + eval report). Forced acceptance. |
| REFINE | always | EVALUATE | Increment round. |
| ACCEPT | queue non-empty | ORIENT | Move spec to completed (with eval history + commit hash). |
| ACCEPT | queue empty | DONE | Move spec to completed. |
| DONE | always | RECONCILE | Initialize reconcile state with round 0. |
| RECONCILE | always | RECONCILE_EVAL | Increment reconcile round. |
| RECONCILE_EVAL | `--verdict PASS` | COMPLETE | Record eval. |
| RECONCILE_EVAL | `--verdict FAIL` | RECONCILE_REVIEW | Record eval (FAIL). |
| RECONCILE_REVIEW | no verdict or `--verdict PASS` | COMPLETE | Accept. |
| RECONCILE_REVIEW | `--verdict FAIL` | RECONCILE | Grant another pass. |
| COMPLETE | always | PHASE_SHIFT | Set phase shift from specifying → planning. |

### Eval Output Convention — Specifying

The specifying evaluation sub-agent writes structured markdown to a known directory:

```
<project>/specs/.eval/
├── <spec-name>-r1.md
├── <spec-name>-r2.md
└── ...
```

The scaffold does not read or write these files. This is a convention for the architect and sub-agent.

### Reconciliation Phase

After all individual specs are completed (DONE), the scaffold enters a reconciliation phase that cross-validates dependencies and integration points across all specs.

#### Reconcile Evaluation Sub-Agent

The reconciliation eval differs from per-spec evals. The sub-agent:
1. Runs `git diff --staged` to see all changes
2. Reads all completed spec files
3. Checks:
   - Every `Depends On` reference points to a spec that exists
   - Every dependency has a corresponding `Integration Points` entry in the target spec
   - Integration Points are symmetric (if A lists B, B lists A)
   - Spec names are consistent across all references
   - No circular dependencies exist

---

## Phase Shift: Specifying → Planning

### Entering PHASE_SHIFT

When the architect advances from COMPLETE, the scaffold transitions to PHASE_SHIFT. No work is done — the scaffold prints the phase shift message and waits.

### Advancing from PHASE_SHIFT

The user must provide `--from <plans-queue.json>` with a plan queue input file.

1. Read and validate the plans queue at `--from`.
2. If validation fails: print errors. State remains PHASE_SHIFT.
3. If validation passes:
   - Populate the plans queue in state.
   - Set `phase` to `"planning"`.
   - Set `state` to `ORIENT`.
   - Pull first plan from queue.
   - Print the ORIENT action description.

---

## Phase: Planning

The planning phase guides the architect through studying specs, codebase, and packages before drafting an implementation plan. The plan is then validated and evaluated through iterative rounds.

### State Machine — Planning

```
ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT
                                                                  │
                                                        ┌─────────┴─────────┐
                                                   plan valid          plan invalid
                                                        │                   │
                                                        ▼                   ▼
                                                   EVALUATE            VALIDATE
                                                        │                   │
                                              ┌─────────┼─────────┐   fix + advance
                                              │         │         │        │
                                        PASS ≥ min  PASS < min  FAIL < max │
                                              │         │         │        │
                                              ▼         ▼         ▼        │
                                           ACCEPT    REFINE    REFINE ◄────┘
                                              │         │         │
                                              ▼         └────┬────┘
                                        PHASE_SHIFT          │
                                                        plan valid → EVALUATE
                                                        plan invalid → VALIDATE

                                        FAIL ≥ max → ACCEPT (forced)
```

### Advancing State — Planning

#### Preconditions
- `phase` is `planning`.

#### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | always | STUDY_SPECS | — |
| STUDY_SPECS | always | STUDY_CODE | — |
| STUDY_CODE | always | STUDY_PACKAGES | — |
| STUDY_PACKAGES | always | REVIEW | — |
| REVIEW | always | DRAFT | — |
| DRAFT | plan.json valid | EVALUATE | Set round to 1. Two transitions in one advance. |
| DRAFT | plan.json invalid | VALIDATE | Set round to 1. Print errors. |
| VALIDATE | plan.json valid | EVALUATE | — |
| VALIDATE | plan.json invalid | _(stays VALIDATE)_ | Print errors. Exit code 1. |
| EVALUATE | `--verdict PASS`, round >= `min_rounds` | ACCEPT | Record eval. |
| EVALUATE | `--verdict PASS`, round < `min_rounds` | REFINE | Record eval. Min rounds not met. |
| EVALUATE | `--verdict FAIL`, round < `max_rounds` | REFINE | Record eval. |
| EVALUATE | `--verdict FAIL`, round >= `max_rounds` | ACCEPT | Record eval. Forced acceptance. |
| REFINE | plan.json valid | EVALUATE | Increment round. Two transitions in one advance. |
| REFINE | plan.json invalid | VALIDATE | Increment round. Print errors. |
| ACCEPT | always | PHASE_SHIFT | Set phase shift from planning → implementing. |

### Study Phases

Three study phases build context before drafting. No flags required — the architect studies, then advances.

#### STUDY_SPECS
Study the specs listed in `current_plan.specs` and the SPEC_MANIFEST.md: full spec files, git diffs, dependencies, cross-references.

#### STUDY_CODE
Explore the codebase using sub-agents (count hardcoded to 3) within `current_plan.code_search_roots`.

#### STUDY_PACKAGES
Study the project's technical stack: package manifests, library documentation, CLAUDE.md references.

### REVIEW Phase

Lightweight checkpoint before drafting. Outputs the path to `PLAN_FORMAT.md`. The architect reviews study findings and the plan format, then advances to DRAFT.

### DRAFT Phase

The architect generates the implementation plan as structured JSON with accompanying notes:

```
<domain>/.workspace/implementation_plan/
├── plan.json
└── notes/
    ├── <package>.md
    └── ...
```

### Validation Gate

Fires automatically when advancing from DRAFT or REFINE. Not a phase where the architect does work.

#### Validation Checks

| Check | Description |
|-------|-------------|
| JSON parse | File exists and contains valid JSON |
| Top-level fields | `context`, `refs`, `layers`, `items` present and correctly typed |
| Context fields | `domain` and `module` are non-empty strings |
| Refs exist | Every path in `refs` resolves to an existing file |
| Item schema | Every item has `id`, `name`, `description`, `depends_on`, `tests` |
| Item ID uniqueness | No duplicate item IDs |
| Layer coverage | Every item in exactly one layer; every layer item ID exists |
| Layer ordering | Items only depend on items in equal or earlier layers |
| DAG validity | `depends_on` references are valid; no cycles |
| Test schema | Every test has `category`, `description`, `passes` with correct types |
| Test categories | One of: `functional`, `rejection`, `edge_case` |
| Notes files | Every `ref` in items resolves to an existing notes file |

**On pass:** transitions directly to EVALUATE. VALIDATE is never visible.

**On fail:** enters VALIDATE, prints errors with field descriptions. Loops until valid.

### EVALUATE Phase — Planning

Uses `eval` command to output full evaluation context for the sub-agent. The evaluator prompt (`evaluators/plan-eval.md`) defines 11 assessment dimensions.

### REFINE Phase — Planning

Outputs the eval report path. Action varies:
- After FAIL: "Evaluation found deficiencies. Spawn a sub-agent to update the plan and notes."
- After PASS below min_rounds: "Minimum evaluation rounds not met."

Advancing from REFINE runs the validation gate.

---

## Phase Shift: Planning → Implementing

### Entering PHASE_SHIFT

When the architect advances from planning ACCEPT, the scaffold transitions to PHASE_SHIFT.

### Advancing from PHASE_SHIFT

No `--from` needed — the plan.json path is already known from `current_plan.file`.

1. Read plan.json at `current_plan.file`.
2. Validate the plan structure (same checks as the planning validation gate).
3. If validation fails: print errors. State remains PHASE_SHIFT.
4. If validation passes:
   - Add `passes: "pending"` and `rounds: 0` to every item.
   - Write the updated plan.json.
   - Set `phase` to `"implementing"`.
   - Set `state` to `ORIENT`.
   - Print the ORIENT action description with initialization summary.

---

## Phase: Implementing

The implementing phase delivers plan items one at a time within dependency-ordered batches. After each batch is fully implemented, an evaluation sub-agent verifies against acceptance criteria.

### Batch Calculation

Batches are groups of items drawn from the current layer. The scaffold selects up to `batch_size` unblocked items.

An item is **unblocked** when:
1. All items in prior layers have a terminal `passes` value (`passed` or `failed`)
2. All items in its `depends_on` list have a terminal `passes` value

Items are selected in the order they appear in the layer's `items` array.

### State Machine — Implementing

```
ORIENT → IMPLEMENT(1) → IMPLEMENT(2) → ... → EVALUATE
                                                  │
                                    ┌──────────────┼──────────────┐
                                    │              │              │
                              PASS + rounds    FAIL + rounds   PASS/FAIL
                              >= min_rounds    < max_rounds    at boundary
                                    │              │              │
                                    ▼              ▼              │
                                 COMMIT      IMPLEMENT(1)→...    │
                                    │        (re-implement)      │
                                    ▼                            │
                              ORIENT/DONE ◄──────────────────────┘
                                                FAIL + rounds
                                                >= max_rounds
                                                      │
                                                      ▼
                                                   COMMIT → ORIENT/DONE
```

### Advancing State — Implementing

#### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | unblocked items exist in current layer | IMPLEMENT | Select batch. Present first item. |
| ORIENT | all layer items terminal, more layers | ORIENT (next layer) | Advance `current_layer`. |
| ORIENT | all layers complete | DONE | — |
| IMPLEMENT | more items in batch | IMPLEMENT | Mark current item `done`. Present next item. |
| IMPLEMENT | last item in batch | EVALUATE | Mark current item `done`. Increment `rounds` on all batch items. |
| EVALUATE | PASS, rounds >= min_rounds | COMMIT | Mark items `passed`. Record eval. |
| EVALUATE | PASS, rounds < min_rounds | IMPLEMENT | Record eval. Re-present first item with eval file. |
| EVALUATE | FAIL, rounds < max_rounds | IMPLEMENT | Record eval. Re-present first item with eval file. |
| EVALUATE | FAIL, rounds >= max_rounds | COMMIT | Mark items `failed`. Record eval. Force-accept. |
| COMMIT | more batches or layers | ORIENT | — |
| COMMIT | all layers complete | DONE | — |
| DONE | — | Error: "session complete." | Terminal state. |

#### Item `passes` Transitions

| Event | `passes` change |
|-------|----------------|
| Engineer advances past item in IMPLEMENT | `pending` → `done` |
| EVALUATE PASS + rounds >= min_rounds | `done` → `passed` |
| EVALUATE FAIL + rounds >= max_rounds | `done` → `failed` |
| EVALUATE FAIL + rounds < max_rounds | stays `done` |
| EVALUATE PASS + rounds < min_rounds | stays `done` |

### IMPLEMENT Behavior

Presents **one item at a time**. Displays full context: name, description, steps, files, spec, ref, test summary.

**First round (no prior eval):** Action says "Implement this item." Advance requires `--message` — the scaffold commits after each item.

**Subsequent rounds (after eval):** Action says "Study the eval file and implement any corrections." No `--message` required — corrections are committed at the COMMIT state after the batch passes.

### EVALUATE Behavior — Implementing

Two actors:

**Sub-agent** runs `forgectl eval` to receive full item details, evaluator prompt, report target path, and previous eval history.

**Engineer** reviews the report, runs `forgectl advance --eval-report <path> --verdict PASS|FAIL`.

### COMMIT State

Hard stop after a batch reaches terminal evaluation. Ensures all implementation work is committed before proceeding.

Appears after:
- EVALUATE with PASS + sufficient rounds
- EVALUATE with FAIL at max_rounds (force-accept)

The engineer commits changes, then runs `forgectl advance` to proceed.

---

## Eval Report Locations

Planning eval reports:
```
<domain>/.workspace/implementation_plan/evals/round-N.md
```

Specifying eval reports:
```
<project>/specs/.eval/<spec-name>-rN.md
```

Implementation eval reports:
```
<domain>/.workspace/implementation_plan/evals/batch-N-round-M.md
```

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `--from` | string | none (required) | Path to input file (schema varies by `--phase`) |
| `--batch-size` | integer | none (required) | Max items per batch in implementing phase |
| `--min-rounds` | integer | 1 | Minimum evaluation rounds per cycle. Used in all phases. |
| `--max-rounds` | integer | none (required) | Maximum evaluation rounds per cycle. Used in all phases. |
| `--phase` | string | specifying | Starting phase: `specifying`, `planning`, `implementing` |
| `--guided` | boolean | true | Enable user-guided mode. Can be changed on any `advance`. |
| `--no-guided` | boolean | — | Disable user-guided mode. |

---

## Reference Files

| File | Location | Used In | Purpose |
|------|----------|---------|---------|
| Plan Format | `PLAN_FORMAT.md` | Planning: REVIEW, DRAFT, Validation Gate | JSON schema, item structure, test conventions, notes file organization |
| Plan Evaluator | `evaluators/plan-eval.md` | Planning: `eval` command output | Full instructions for the planning evaluation sub-agent |
| Implementation Evaluator | `evaluators/impl-eval.md` | Implementing: `eval` command output | Full instructions for the implementation evaluation sub-agent |

---

## State File Schema

```json
{
  "phase": "implementing",
  "state": "IMPLEMENT",
  "batch_size": 2,
  "min_rounds": 1,
  "max_rounds": 3,
  "user_guided": true,
  "started_at_phase": "specifying",

  "specifying": {
    "current_spec": null,
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
          { "round": 1, "verdict": "FAIL", "eval_report": "optimizer/specs/.eval/configuration-models-r1.md" },
          { "round": 2, "verdict": "PASS", "eval_report": "optimizer/specs/.eval/configuration-models-r2.md" }
        ]
      }
    ],
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
      "name": "Service Configuration",
      "domain": "launcher",
      "topic": "Implementation plan for service configuration",
      "file": "launcher/.workspace/implementation_plan/plan.json",
      "specs": ["launcher/specs/service-configuration.md"],
      "code_search_roots": ["launcher/"]
    },
    "round": 2,
    "evals": [
      { "round": 1, "verdict": "FAIL", "eval_report": "launcher/.workspace/implementation_plan/evals/round-1.md" },
      { "round": 2, "verdict": "PASS", "eval_report": "launcher/.workspace/implementation_plan/evals/round-2.md" }
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
              { "round": 1, "verdict": "PASS", "eval_report": "launcher/.workspace/implementation_plan/evals/batch-1-round-1.md" }
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
| `phase` | string | `specifying`, `planning`, or `implementing` |
| `state` | string | Current state within the active phase |
| `batch_size` | integer | Max items per batch |
| `min_rounds` | integer | Minimum eval rounds per cycle |
| `max_rounds` | integer | Maximum eval rounds per cycle |
| `user_guided` | boolean | Whether guided pauses are active |
| `started_at_phase` | string | Which phase the session was initialized at (for display) |
| **Specifying** | | |
| `specifying.current_spec` | object/null | The spec being worked on |
| `specifying.queue` | array | Remaining specs |
| `specifying.completed` | array | Finished specs with eval history and commit hashes |
| `specifying.completed[].evals` | array | Full eval trail per spec |
| `specifying.reconcile` | object | Reconciliation round and eval history |
| **Planning** | | |
| `planning.current_plan` | object/null | The plan being worked on |
| `planning.round` | integer | Current planning eval round |
| `planning.evals` | array | Planning eval history |
| `planning.queue` | array | Remaining plans |
| `planning.completed` | array | Finished plans |
| **Implementing** | | |
| `implementing.current_layer` | object | Active layer |
| `implementing.batch_number` | integer | Global batch counter (1-indexed) |
| `implementing.current_batch` | object | Active batch state |
| `implementing.current_batch.items` | string[] | Item IDs in batch |
| `implementing.current_batch.current_item_index` | integer | 0-based index |
| `implementing.current_batch.eval_round` | integer | Current eval round for batch |
| `implementing.current_batch.evals` | array | Batch eval history |
| `implementing.layer_history` | array | Completed batches and layers |

Phase sections that haven't been reached yet are `null` in the state file. When starting at a later phase (`--phase planning`), earlier phase sections remain `null`.

---

## Session Archiving

Completed session state files are archived to a permanent directory:

```
sessions/
├── optimizer-2026-03-15.json
├── launcher-2026-03-21.json
└── ...
```

- The active `forgectl-state.json` is gitignored (ephemeral working state).
- Archived sessions are committed to git (permanent audit trail).
- Naming convention: `<domain>-<date>.json`.
- Archive before starting a new session. The active state file must be deleted (or the scaffold will reject `init`).

---

## Invariants

### General

1. **Phase is authoritative.** The `phase` field determines which states are valid and how shared state names behave.
2. **No implicit state.** All information for transitions is in the state file (and plan.json during implementing).
3. **Eval history is append-only.** Eval records accumulate and are never deleted or modified.
4. **Min rounds enforced.** PASS below `min_rounds` forces another cycle in all phases.
5. **Max rounds enforced.** FAIL at `max_rounds` forces acceptance in all phases.
6. **State file is durable.** Atomic writes with backup prevent corruption. Startup recovery handles interrupted writes.
7. **Queue order preserved.** Specs and plans are pulled from the front of the queue.
8. **Guided setting is mutable.** `--guided` / `--no-guided` can change `user_guided` on any advance call.
9. **Guided pauses.** When `user_guided` is true, the action output includes "Stop and review and discuss with user before continuing." at SELECT (specifying), REVIEW (planning), and ORIENT (implementing). When false, these states advance without the pause message.

### Specifying Phase

9. **Single active spec.** At most one spec in `current_spec` at any time.
10. **Reconciliation follows completion.** DONE always transitions to RECONCILE. Individual specs must be complete before cross-validation.
11. **Round monotonicity.** The specifying round counter only increments.

### Planning Phase

12. **Study phases precede REVIEW.** STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW. No phase is skipped.
13. **Validation precedes evaluation.** The validation gate runs before every EVALUATE entry.
14. **Round monotonicity.** The planning round counter only increments.

### Phase Shifts

15. **Phase shifts are explicit.** The scaffold always stops at PHASE_SHIFT between phases. It never transitions directly across phase boundaries.
16. **specifying→planning requires input.** `--from` with a plans queue is required at this phase shift.
17. **planning→implementing validates.** plan.json is validated and mutated on advance out of PHASE_SHIFT.

### Implementing Phase

18. **Layer ordering enforced.** All items in layer N must be terminal before layer N+1.
19. **Dependency ordering enforced.** Items only delivered when `depends_on` items are terminal.
20. **Item order preserved.** Items delivered in layer's `items` array order.
21. **One item at a time.** IMPLEMENT presents a single item per advance.
22. **plan.json is the progress record.** `passes` and `rounds` reflect current state.
23. **COMMIT precedes progression.** Every batch boundary passes through COMMIT before ORIENT/DONE.
24. **First-round commits.** IMPLEMENT advance requires `--message` and commits on the first round only. Subsequent rounds do not commit (corrections committed at COMMIT state).
25. **Two actors, two commands.** Engineer uses `advance`; sub-agent uses `eval`.
26. **Scaffold does not parse eval files.** Verdict provided via `--verdict`; file stored as path reference.

---

## Edge Cases

### Specifying Phase

- **Scenario:** `advance --verdict FAIL` when round < `max_rounds`.
  - **Expected:** REFINE.

- **Scenario:** `advance --verdict FAIL` when round >= `max_rounds`.
  - **Expected:** ACCEPT (forced).

- **Scenario:** `advance --verdict PASS` when round < `min_rounds`.
  - **Expected:** REFINE (min rounds not met).


### Phase Shifts

- **Scenario:** Advance from specifying→planning PHASE_SHIFT without `--from`.
  - **Expected:** Error. State remains PHASE_SHIFT.

- **Scenario:** Advance from specifying→planning PHASE_SHIFT with invalid plans queue.
  - **Expected:** Validation errors. State remains PHASE_SHIFT.

- **Scenario:** Advance from planning→implementing PHASE_SHIFT with invalid plan.json.
  - **Expected:** Validation errors. State remains PHASE_SHIFT.

- **Scenario:** `--guided` provided at PHASE_SHIFT advance.
  - **Expected:** `user_guided` updated before the phase transition proceeds.

### Planning Phase

- **Scenario:** Validation passes on first try after DRAFT.
  - **Expected:** Transitions directly to EVALUATE in one `advance` call.

- **Scenario:** Validation fails after REFINE.
  - **Expected:** Enters VALIDATE loop.

- **Scenario:** plan.json does not exist when validation runs.
  - **Expected:** Validation fails with file-not-found.

- **Scenario:** plan.json has a dependency cycle.
  - **Expected:** Validation fails listing the cycle.

- **Scenario:** `--eval-report` points to non-existent file.
  - **Expected:** Error. State unchanged.

### Implementing Phase

- **Scenario:** Layer has fewer items than `batch_size`.
  - **Expected:** Single batch contains all items.

- **Scenario:** Batch has one item.
  - **Expected:** IMPLEMENT → EVALUATE directly.

- **Scenario:** EVALUATE PASS but rounds < min_rounds.
  - **Expected:** Re-enter IMPLEMENT. No commit reminder (not first round).

- **Scenario:** EVALUATE FAIL at max_rounds.
  - **Expected:** Items `failed`. COMMIT. ORIENT.

- **Scenario:** Item depends on a `failed` item.
  - **Expected:** Still unblocked — `failed` is terminal.

- **Scenario:** All layers complete.
  - **Expected:** COMMIT → DONE.

- **Scenario:** `eval` called outside EVALUATE.
  - **Expected:** Error.

---

## Testing Criteria

### Init

#### Init defaults to specifying phase
- **When:** `forgectl init --from specs-queue.json --batch-size 2 --max-rounds 3`
- **Then:** `phase: "specifying"`, `state: "ORIENT"`, `started_at_phase: "specifying"`.

#### Init at planning phase
- **When:** `forgectl init --phase planning --from plans-queue.json --batch-size 2 --max-rounds 3`
- **Then:** `phase: "planning"`, `state: "ORIENT"`. Specifying section is null.

#### Init at implementing phase
- **When:** `forgectl init --phase implementing --from plan.json --batch-size 2 --max-rounds 3`
- **Then:** `phase: "implementing"`, `state: "ORIENT"`. plan.json items have `passes` and `rounds`.

#### Init rejects existing state
- **Given:** State file exists
- **Then:** Exit code 1.

#### Init rejects invalid queue
- **Given:** Spec queue missing `file` field
- **Then:** Exit code 1.

#### Init rejects min exceeding max
- **Given:** `--min-rounds 5 --max-rounds 2`
- **Then:** Exit code 1.

#### Init rejects batch-size less than 1
- **Given:** `--batch-size 0`
- **Then:** Exit code 1.

### Specifying Phase

#### Study and draft advance sequentially
- **Given:** ORIENT
- **When:** advance through SELECT → DRAFT → EVALUATE
- **Then:** Each transitions in order.

#### FAIL below max_rounds goes to REFINE
- **Given:** EVALUATE, max_rounds: 3, round 1
- **When:** `advance --verdict FAIL --eval-report .eval/spec-r1.md`
- **Then:** State is REFINE.

#### FAIL at max_rounds forces ACCEPT
- **Given:** EVALUATE, max_rounds: 2, round 2
- **When:** `advance --verdict FAIL --eval-report .eval/spec-r2.md`
- **Then:** State is ACCEPT (forced).

#### PASS below min_rounds goes to REFINE
- **Given:** EVALUATE, min_rounds: 2, round 1
- **When:** `advance --verdict PASS --eval-report .eval/spec-r1.md`
- **Then:** State is REFINE.

#### PASS at min_rounds goes to ACCEPT
- **Given:** EVALUATE, min_rounds: 2, round 2
- **When:** `advance --verdict PASS --eval-report .eval/spec-r2.md --message "Add spec"`
- **Then:** State is ACCEPT. Commit created.

#### PASS requires message
- **Given:** EVALUATE
- **When:** `advance --verdict PASS` without `--message`
- **Then:** Exit code 1.

#### DONE transitions to RECONCILE
- **Given:** All specs accepted, state is DONE
- **When:** `advance`
- **Then:** State is RECONCILE.

#### Reconcile flow PASS
- **Given:** RECONCILE_EVAL
- **When:** `advance --verdict PASS`
- **Then:** State is COMPLETE.

#### Reconcile flow FAIL then fix
- **Given:** RECONCILE_EVAL FAIL → RECONCILE_REVIEW
- **When:** `advance --verdict FAIL`
- **Then:** State is RECONCILE.

#### COMPLETE transitions to PHASE_SHIFT
- **Given:** COMPLETE
- **When:** `advance`
- **Then:** State is PHASE_SHIFT.

### Phase Shifts

#### specifying→planning requires --from
- **Given:** PHASE_SHIFT (specifying→planning)
- **When:** `advance` without `--from`
- **Then:** Exit code 1.

#### specifying→planning with valid queue
- **Given:** PHASE_SHIFT (specifying→planning)
- **When:** `advance --from plans-queue.json`
- **Then:** `phase: "planning"`, `state: "ORIENT"`.

#### specifying→planning with invalid queue
- **Given:** PHASE_SHIFT (specifying→planning)
- **When:** `advance --from invalid.json`
- **Then:** Errors printed. State remains PHASE_SHIFT.

#### planning→implementing with valid plan
- **Given:** PHASE_SHIFT (planning→implementing)
- **When:** `advance`
- **Then:** `phase: "implementing"`, `state: "ORIENT"`. plan.json items mutated.

#### planning→implementing with invalid plan
- **Given:** PHASE_SHIFT (planning→implementing), plan.json has errors
- **When:** `advance`
- **Then:** Errors printed. State remains PHASE_SHIFT.

#### --guided at phase shift
- **Given:** PHASE_SHIFT, user_guided is true
- **When:** `advance --no-guided --from plans-queue.json`
- **Then:** `user_guided: false` after transition.

### Planning Phase

#### Study phases advance sequentially
- **Given:** ORIENT (planning)
- **When:** advance through STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT
- **Then:** Each transitions in order.

#### DRAFT with valid plan goes to EVALUATE
- **Given:** DRAFT, plan.json valid
- **When:** `advance`
- **Then:** State is EVALUATE. Round is 1.

#### DRAFT with invalid plan enters VALIDATE
- **Given:** DRAFT, plan.json invalid
- **When:** `advance`
- **Then:** State is VALIDATE.

#### VALIDATE loops until valid
- **Given:** VALIDATE, plan.json still invalid
- **When:** `advance`
- **Then:** Stays VALIDATE. Exit code 1.

#### Planning EVALUATE PASS at min_rounds → ACCEPT
- **Given:** EVALUATE, min_rounds: 1, round 1
- **When:** `advance --verdict PASS --eval-report evals/round-1.md`
- **Then:** State is ACCEPT.

#### Planning EVALUATE FAIL at max_rounds → ACCEPT (forced)
- **Given:** EVALUATE, max_rounds: 2, round 2
- **When:** `advance --verdict FAIL --eval-report evals/round-2.md`
- **Then:** State is ACCEPT.

#### Planning ACCEPT → PHASE_SHIFT
- **Given:** ACCEPT (planning)
- **When:** `advance`
- **Then:** State is PHASE_SHIFT.

#### Planning eval command outputs context
- **Given:** EVALUATE (planning), round 1
- **When:** `forgectl eval`
- **Then:** Output includes plan-eval.md contents, plan references, report target.

### Implementing Phase

#### ORIENT selects first batch
- **Given:** ORIENT (implementing), L0 has 4 items, batch_size 2
- **When:** `advance`
- **Then:** State is IMPLEMENT. First item presented.

#### IMPLEMENT presents items one at a time
- **Given:** IMPLEMENT, batch has 2 items, on item 1
- **When:** `advance`
- **Then:** Item 1 `done`. Item 2 presented.

#### IMPLEMENT last item → EVALUATE
- **Given:** IMPLEMENT, last item in batch
- **When:** `advance`
- **Then:** Item `done`. Rounds incremented. State is EVALUATE.

#### First-round IMPLEMENT requires --message
- **Given:** IMPLEMENT, first round (no prior eval)
- **When:** `advance` without `--message`
- **Then:** Exit code 1.

#### First-round IMPLEMENT commits with --message
- **Given:** IMPLEMENT, first round (no prior eval)
- **When:** `advance --message "Implement config types"`
- **Then:** Scaffold commits. Next item presented (or EVALUATE if last).

#### Subsequent-round IMPLEMENT does not require --message
- **Given:** IMPLEMENT, entered after EVALUATE (round 2+)
- **When:** `advance`
- **Then:** Advances without committing. No error.

#### EVALUATE PASS with sufficient rounds → COMMIT
- **Given:** EVALUATE, rounds >= min_rounds
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** Items `passed`. State is COMMIT.

#### EVALUATE FAIL at max_rounds → COMMIT
- **Given:** EVALUATE, rounds == max_rounds
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** Items `failed`. State is COMMIT.

#### EVALUATE FAIL within max_rounds → IMPLEMENT
- **Given:** EVALUATE, rounds < max_rounds
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is IMPLEMENT. First item with eval file.

#### COMMIT → ORIENT (more items)
- **Given:** COMMIT, more items in layer
- **When:** `advance`
- **Then:** State is ORIENT.

#### COMMIT → DONE (all complete)
- **Given:** COMMIT, all layers complete
- **When:** `advance`
- **Then:** State is DONE.

#### Implementing eval command outputs item details
- **Given:** EVALUATE (implementing), batch has 2 items
- **When:** `forgectl eval`
- **Then:** Output includes impl-eval.md contents, item details, report target.

#### Failed items don't block dependents
- **Given:** Item A `failed`, item B depends on A
- **Then:** B is unblocked.

#### DONE cannot advance
- **Given:** DONE
- **When:** `advance`
- **Then:** Error.

### Full Lifecycle Tests

#### Specifying only (started at specifying, stopped at PHASE_SHIFT)
- **Given:** Init with 2 specs, `--batch-size 2 --min-rounds 1 --max-rounds 3`
- **When:** Complete both specs → DONE → RECONCILE → RECONCILE_EVAL(PASS) → COMPLETE → PHASE_SHIFT
- **Then:** State is PHASE_SHIFT. Specifying completed with 2 specs.

#### Full three-phase lifecycle
- **Given:** Init with specs queue
- **When:** Specifying: draft + accept 2 specs, reconcile → PHASE_SHIFT → Planning: study, draft plan, evaluate → PHASE_SHIFT → Implementing: batch items, evaluate → DONE
- **Then:** State is DONE. All three phase sections populated in state file.

#### Start at planning, full lifecycle
- **Given:** `init --phase planning --from plans-queue.json`
- **When:** Planning → PHASE_SHIFT → Implementing → DONE
- **Then:** Specifying section is null. Planning and implementing complete.

#### Start at implementing
- **Given:** `init --phase implementing --from plan.json`
- **When:** Implementing → DONE
- **Then:** Specifying and planning sections are null. Implementing complete.

### State File Durability

#### Recovery from crash between backup and rename
- **Given:** `.json` missing, `.bak` exists
- **When:** Any command runs
- **Then:** `.bak` renamed to `.json`. Warning printed. Command proceeds.

#### Recovery from corrupt state file
- **Given:** `.json` contains invalid JSON, `.bak` exists
- **When:** Any command runs
- **Then:** `.json` renamed to `.json.corrupt`. `.bak` renamed to `.json`. Warning printed.

---

## Implements
- Unified three-phase lifecycle scaffold: specifying → planning → implementing
- Specifying phase: queue-driven spec drafting with eval/refine loop and reconciliation
- Planning phase: structured study → draft → validate → evaluate → accept
- Implementing phase: layer-ordered batched item delivery with one-at-a-time presentation
- Explicit PHASE_SHIFT checkpoints between phases with context refresh
- COMMIT state for batch boundary commits in implementing phase
- First-round commit reminders in IMPLEMENT output
- Phase-selectable init (`--phase specifying|planning|implementing`)
- Mutable `--guided` / `--no-guided` setting on any advance
- Phase shift input injection (`--from` at specifying→planning boundary)
- Atomic state file writes with backup and startup recovery
- Dual evaluator prompts: plan-eval.md and impl-eval.md
- Commit hash tracking for completed specs (add-commit, reconcile-commit)
- Session archiving to git-tracked sessions directory
