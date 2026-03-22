# Implementation Scaffold CLI

## Topic of Concern
> The implementation scaffold CLI guides a Senior Software Engineer through batched implementation of plan items using a JSON-backed state machine with layer-ordered delivery, one-item-at-a-time implementation, per-batch evaluation with min/max round enforcement, and plan.json progress tracking.

## Context

The implementation process consumes a `plan.json` produced by the implementation planning scaffold. The plan contains items organized into dependency layers, each with steps, tests, and file references. The scaffold delivers items one at a time within a batch, the engineer implements each, then an evaluation sub-agent verifies the batch. The scaffold enforces evaluation rounds (min and max), tracks progress in both its state file and plan.json, and provides unambiguous next-step guidance.

The scaffold has two user roles:
- **The engineer** uses `advance` to progress through implementation and to provide eval files and verdicts after reviewing them.
- **The evaluation sub-agent** uses `eval` to receive item details and evaluation instructions, then produces an eval report file.

The scaffold respects layer ordering — all items in a layer must reach a terminal state (`passed` or `failed`) before items in the next layer are eligible. Within a layer, items are batched up to the configured batch size and delivered in dependency order. Each batch is evaluated after all its items are implemented. On FAIL, the engineer re-implements the same batch items using the eval report as guidance. On PASS before min_rounds, the engineer re-implements the same batch items to find and apply any remaining corrections.

## Depends On
- None. The implementation scaffold is a standalone CLI tool with no runtime dependencies on other project components.

## Integration Points

| Component | Relationship |
|-----------|-------------|
| plan.json | The plan defines items, layers, steps, tests, and files; the scaffold delivers items, tracks completion, and updates `passes` and `rounds` on each item |
| Evaluation sub-agent | Uses the `eval` command to receive evaluation instructions; produces an eval report file |
| Evaluator prompt (`EVALUATOR_PROMPT.md`) | Embedded in `eval` output; instructs the sub-agent on evaluation dimensions, report format, and verdict rules |
| PLAN_FORMAT.md | Defines the JSON schema for plan.json; the scaffold validates against this during init |

---

## Interface

### Inputs

#### Plan Input File (provided via `--from` on `init`)

A `plan.json` file conforming to the schema defined in `PLAN_FORMAT.md`. The scaffold validates the full plan structure during init. The scaffold adds `passes` and `rounds` fields to each item during init.

Key fields consumed per item:

| Field | Usage |
|-------|-------|
| `id` | Unique identifier, used in state tracking and output |
| `name` | Display name in output |
| `description` | Shown when presenting items to the engineer |
| `spec` | Shown for reference — where the contract is defined |
| `ref` | Shown for reference — implementation guidance in notes |
| `depends_on` | Used to determine which items are unblocked |
| `files` | Shown when presenting items — scope of changes |
| `steps` | Shown when presenting items — what to implement |
| `tests` | Shown when presenting items — acceptance criteria |

Fields added by the scaffold during init:

| Field | Type | Description |
|-------|------|-------------|
| `passes` | string | Lifecycle state. Set to `pending` on init. Updated to `done`, `passed`, or `failed`. |
| `rounds` | integer | Evaluation round counter. Set to `0` on init. Incremented on each EVALUATE. |

#### CLI Arguments

| Command | Flags | Description |
|---------|-------|-------------|
| `init` | `--from <path>` (required), `--batch-size N` (required), `--min-rounds N` (default 1), `--max-rounds N` (required) | Initialize state file from a validated plan. Adds `passes` and `rounds` to items. Prints initial state. |
| `advance` | `--eval-file <path> --verdict PASS\|FAIL` (EVALUATE only) | Transition from current state to next; prints the new state and action guidance |
| `eval` | none | Output full item details and evaluation instructions for the sub-agent. Only valid in EVALUATE state. |
| `status` | none | Print full session state: current layer, current batch, item progress, completed layers |

### Outputs

All output is to stdout. The scaffold writes state changes to `implement-state.json` and updates `passes` and `rounds` fields in plan.json.

#### `advance` output

Every `advance` call prints a structured state block after transitioning.

**Entering IMPLEMENT (first item in batch, no prior eval):**

```
State:   IMPLEMENT
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
Action:  Implement this item. When complete, run: implctl advance
```

**Entering IMPLEMENT (next item in same batch, no prior eval):**

```
State:   IMPLEMENT
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
Action:  Implement this item. When complete, run: implctl advance
```

**Entering IMPLEMENT (first item in batch, after eval — PASS or FAIL):**

```
State:   IMPLEMENT
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
         When complete, run: implctl advance
```

**Entering IMPLEMENT (next item in batch, after eval):**

```
State:   IMPLEMENT
Layer:   L0 Foundation
Batch:   1/2
Round:   1/3
Eval:    launcher/.workspace/implementation_plan/evals/batch-1-round-1.md
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
Action:  Study the eval file "launcher/.workspace/implementation_plan/evals/batch-1-round-1.md"
         and implement any corrections as needed. If none found during the eval,
         please verify and look for corrections. Apply them.
         When complete, run: implctl advance
```

**Entering EVALUATE:**

```
State:    EVALUATE
Layer:    L0 Foundation
Batch:    1/2
Round:    1/3
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Action:   Ask the evaluation sub-agent to verify batch items against their tests.
          The sub-agent should run: implctl eval
          After reviewing the eval report, run:
            implctl advance --eval-file <path> --verdict PASS|FAIL
Sub-agent: implctl eval
```

**Entering EVALUATE (additional round after PASS, rounds < min_rounds):**

```
State:    EVALUATE
Layer:    L0 Foundation
Batch:    1/2
Round:    2/3
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Note:     PASS recorded for round 1. Minimum rounds not yet met (1/2).
Action:   Ask the evaluation sub-agent to verify batch items against their tests.
          The sub-agent should run: implctl eval
          After reviewing the eval report, run:
            implctl advance --eval-file <path> --verdict PASS|FAIL
Sub-agent: implctl eval
```

**ORIENT (after PASS with sufficient rounds, more items in layer):**

```
State:    ORIENT
Layer:    L0 Foundation
Progress: 2/3 items passed
Action:   Selecting next batch. Run: implctl advance
```

**ORIENT (layer complete, advancing):**

```
State:    ORIENT
Layer:    L0 Foundation
Progress: 3/3 items passed — layer complete
Action:   Advancing to next layer. Run: implctl advance
```

**ORIENT (force-accept after max_rounds FAIL):**

```
State:    ORIENT
Layer:    L1 Core
          FORCE ACCEPT: 2 items marked failed (max rounds 3/3 reached)
          - [daemon.types] Daemon state types and PID file struct
          - [daemon.io] PID file I/O operations
Progress: 2/2 items terminal (0 passed, 2 failed) — layer complete
Action:   Advancing to next layer. Run: implctl advance
```

**DONE:**

```
State:   DONE
Summary:
  L0 Foundation:  3/3 passed
  L1 Core:        2/2 passed
  Total:          5/5 items passed
  Eval rounds:    7 across 3 batches
Action:  All items complete. Session done.
```

#### `eval` output

The `eval` command is used by the evaluation sub-agent. It outputs the full evaluation context for the current batch. Only valid in EVALUATE state.

**First round:**

```
=== EVALUATION ROUND 1/3 ===
Layer: L0 Foundation
Batch: 1/2

--- EVALUATOR INSTRUCTIONS ---

<contents of EVALUATOR_PROMPT.md>

--- ITEMS TO EVALUATE ---

[1] config.types — ServiceEndpoint and ServicesConfig structs
    Description: Go structs for validated service endpoint configuration.
                 Three named fields enforce the three-services invariant at compile time.
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
    Description: Parse spectacular.yml, apply default host/port values,
                 reject invalid config with structured errors naming the offending field.
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

**Subsequent rounds (includes previous eval history):**

```
=== EVALUATION ROUND 2/3 ===
Layer: L0 Foundation
Batch: 1/2

--- EVALUATOR INSTRUCTIONS ---

<contents of EVALUATOR_PROMPT.md>

--- ITEMS TO EVALUATE ---

[1] config.types — ServiceEndpoint and ServicesConfig structs
    ...same item details...

[2] config.load — Load YAML, apply defaults, validate strictly
    ...same item details...

--- PREVIOUS EVALUATIONS ---

Round 1: PASS — launcher/.workspace/implementation_plan/evals/batch-1-round-1.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.workspace/implementation_plan/evals/batch-1-round-2.md
```

#### `advance --eval-file --verdict` in EVALUATE

When advancing from EVALUATE, the engineer must provide the eval report file and verdict:

```
implctl advance --eval-file launcher/.workspace/implementation_plan/evals/batch-1-round-1.md --verdict PASS
```

The scaffold:
1. Verifies the eval file exists.
2. Records the eval (round, verdict, eval_file path).
3. Transitions based on the verdict and round logic.

The scaffold does not parse the eval file contents. The verdict is provided by the engineer via `--verdict`. The eval file is stored as a reference in the state file.

#### `init` validation output (on failure)

When plan.json fails validation, the scaffold prints:
1. Each validation error (missing field, extra field, wrong type) with the path to the offending location.
2. Reference to PLAN_FORMAT.md for the valid schema.

The scaffold exits with a non-zero code on validation failure.

#### `status` output

```
Session: launcher/.workspace/implementation_plan/plan.json
Config:  batch_size=2, rounds=1-3

Current:
  State:   IMPLEMENT
  Layer:   L1 Core (2 items)
  Batch:   1/1
  Item:    [daemon.io] PID file I/O operations (2 of 2)
  Round:   0

Layer L0 (Foundation): complete
  [bootstrap]     passed  (1 round)
  [config.types]  passed  (1 round)
  [config.load]   passed  (2 rounds)

Layer L1 (Core): in progress
  [daemon.types]  done    (0 rounds)
  [daemon.io]     pending (0 rounds)
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `init` called when `implement-state.json` already exists | Error: "State file already exists. Delete it to reinitialize." Exit code 1. | Prevents accidental loss of in-progress state |
| `--from` file fails plan.json validation | Error listing violations. Exit code 1. | Engineer needs to see what's wrong |
| `--batch-size` < 1 | Error: "--batch-size must be at least 1." Exit code 1. | Invalid configuration |
| `--min-rounds` < 1 | Error: "--min-rounds must be at least 1." Exit code 1. | At least one eval round required |
| `--min-rounds` > `--max-rounds` | Error: "--min-rounds cannot exceed --max-rounds." Exit code 1. | Invalid configuration |
| `advance` in EVALUATE without `--eval-file` or `--verdict` | Error: "EVALUATE requires --eval-file <path> --verdict PASS\|FAIL." Exit code 1. | Engineer must provide both |
| `advance --eval-file` outside of EVALUATE state | Error naming the current state. Exit code 1. | Eval file is only valid in EVALUATE |
| `advance --eval-file` with non-existent file | Error naming the missing path. Exit code 1. | Report must exist |
| `advance --verdict` with value other than PASS or FAIL | Error: "Invalid verdict. Must be PASS or FAIL." Exit code 1. | Only two valid verdicts |
| `eval` outside of EVALUATE state | Error naming the current state. Exit code 1. | Eval context only available in EVALUATE |
| `eval` or `advance` or `status` called before `init` | Error. Exit code 1. | State file must exist |

---

## Behavior

### Batch Calculation

Batches are groups of items drawn from the current layer. The scaffold selects up to `batch_size` unblocked items from the layer, respecting `depends_on` ordering.

An item is **unblocked** when:
1. All items in prior layers have a terminal `passes` value (`passed` or `failed`)
2. All items in its `depends_on` list have a terminal `passes` value

Items are selected in the order they appear in the layer's `items` array.

### Initializing a Session

#### Preconditions
- No `implement-state.json` exists.
- `--from`, `--batch-size`, and `--max-rounds` are provided.
- `--min-rounds` <= `--max-rounds`.

#### Steps
1. Read and parse the file at `--from`.
2. Validate against the plan.json schema (PLAN_FORMAT.md).
3. If validation fails: print errors, exit code 1.
4. If validation passes: add `passes: "pending"` and `rounds: 0` to each item, write updated plan.json, create `implement-state.json` with state ORIENT.

#### Postconditions
- plan.json items have `passes` and `rounds` fields.
- State file exists with `batch_size`, `min_rounds`, `max_rounds` set, `current_layer` pointing to the first layer.

---

### State Machine

```
ORIENT → IMPLEMENT(item 1) → IMPLEMENT(item 2) → ... → EVALUATE
                                                           │
                                              ┌────────────┼────────────┐
                                              │            │            │
                                        PASS + rounds   FAIL + rounds  FAIL + rounds
                                        >= min_rounds   < max_rounds   >= max_rounds
                                              │            │            │
                                              ▼            │            ▼
                                        items → passed     │      items → failed
                                              │            │            │
                                        (more items        │      (continue to
                                         in layer?)        │       next batch)
                                          yes → ORIENT     │            │
                                          no → (more       ▼            ▼
                                               layers?) IMPLEMENT → EVALUATE
                                                yes → ORIENT  (same items,
                                                no → DONE      with eval file
                                                               as guidance)
                                              PASS + rounds
                                              < min_rounds
                                                    │
                                                    ▼
                                              IMPLEMENT → EVALUATE
                                              (same items, with eval
                                               file as guidance)
```

### Advancing State

#### Preconditions
- `implement-state.json` exists.

#### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | unblocked items exist in current layer | IMPLEMENT | Select up to `batch_size` unblocked items. Set `current_batch`. Present first item. |
| ORIENT | no unblocked items, all layer items terminal | ORIENT (next layer) | Advance `current_layer` to next layer. |
| ORIENT | all layers complete | DONE | — |
| IMPLEMENT | more items in batch not yet implemented | IMPLEMENT | Mark current item `done` in plan.json. Present next item in batch. |
| IMPLEMENT | last item in batch | EVALUATE | Mark current item `done` in plan.json. Increment `rounds` on all batch items. |
| EVALUATE | `--verdict PASS`, rounds >= min_rounds | ORIENT | Mark batch items `passed` in plan.json. Record eval. |
| EVALUATE | `--verdict PASS`, rounds < min_rounds | IMPLEMENT | Record eval (PASS). Re-present first item in batch with eval file as guidance. |
| EVALUATE | `--verdict FAIL`, rounds < max_rounds | IMPLEMENT | Record eval (FAIL). Re-present first item in batch with eval file as guidance. |
| EVALUATE | `--verdict FAIL`, rounds >= max_rounds | ORIENT | Mark batch items `failed` in plan.json. Record eval (FAIL + force-accept). |
| DONE | — | Error: "session complete." | Terminal state. |

#### Item `passes` Transitions

| Event | `passes` change |
|-------|----------------|
| Engineer advances past item in IMPLEMENT | `pending` → `done` |
| EVALUATE verdict PASS + rounds >= min_rounds | `done` → `passed` |
| EVALUATE verdict FAIL + rounds >= max_rounds | `done` → `failed` |
| EVALUATE verdict FAIL + rounds < max_rounds | stays `done` (re-implement cycle) |
| EVALUATE verdict PASS + rounds < min_rounds | stays `done` (re-implement cycle) |

#### Postconditions
- State file reflects the new state.
- plan.json `passes` and `rounds` fields updated.
- Eval records accumulate in state file.

#### Error Handling
- Invalid flags for state: specific error per state.
- Missing eval file or verdict: error with usage guidance.

---

### IMPLEMENT Behavior

IMPLEMENT presents **one item at a time**. Within a batch of N items, the scaffold cycles through IMPLEMENT N times before reaching EVALUATE. Each `advance` from IMPLEMENT either presents the next item or transitions to EVALUATE if it was the last item.

The scaffold displays the item's full context: name, description, steps, files, spec, ref, and test summary.

**When no prior eval exists** (first time through the batch): the Action instructs the engineer to implement the item.

**When a prior eval exists** (re-implementing after EVALUATE): the Action instructs the engineer to study the eval file and implement any corrections. The eval file path is shown in the output. This applies regardless of whether the prior verdict was PASS or FAIL — on PASS with insufficient rounds, the engineer still reviews for corrections.

### EVALUATE Behavior

After all items in a batch are implemented, the scaffold transitions to EVALUATE. This state involves two actors:

#### The Sub-Agent (`eval`)

The sub-agent runs `implctl eval` to receive:
1. Full details for every item in the batch (description, spec, ref, files, steps, tests).
2. The evaluator prompt instructions (from `EVALUATOR_PROMPT.md`).
3. The target path for the eval report file.
4. Previous evaluation history (on round 2+).

The sub-agent:
1. Reads the implementation (the files listed in each item).
2. Evaluates against each item's tests (acceptance criteria).
3. Writes an eval report file to the specified path.

#### The Engineer (`advance --eval-file --verdict`)

After the sub-agent writes the eval report, the engineer:
1. Reviews the report.
2. Runs `implctl advance --eval-file <path> --verdict PASS|FAIL`.

The scaffold records the eval file path and verdict, then transitions based on the round logic:

- **PASS + rounds >= min_rounds**: Items marked `passed`. Scaffold moves to ORIENT for next batch.
- **PASS + rounds < min_rounds**: Re-enter IMPLEMENT for the same batch items with the eval file as guidance.
- **FAIL + rounds < max_rounds**: Re-enter IMPLEMENT for the same batch items with the eval file as guidance.
- **FAIL + rounds >= max_rounds**: Items are force-accepted as `failed`. The scaffold records the failure and moves to ORIENT.

---

## Eval Report Location

Eval reports are written to a predictable path derived from the batch and round:

```
<plan_directory>/evals/batch-<N>-round-<M>.md
```

Example: `launcher/.workspace/implementation_plan/evals/batch-1-round-1.md`

The scaffold outputs this path in both the EVALUATE `advance` output and the `eval` output, so both the engineer and sub-agent know where the report should go.

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `--batch-size` | integer | none (required) | Maximum number of items delivered per batch |
| `--min-rounds` | integer | 1 | Minimum evaluation rounds per batch. Even if PASS, another round runs until rounds >= min_rounds. |
| `--max-rounds` | integer | none (required) | Maximum evaluation rounds per batch. On FAIL at max_rounds, items are force-accepted as `failed`. |
| `--from` | string | none (required on init) | Path to plan.json file |

---

## State File Schema

```json
{
  "batch_size": 2,
  "min_rounds": 1,
  "max_rounds": 3,
  "plan_file": "launcher/.workspace/implementation_plan/plan.json",
  "state": "IMPLEMENT",
  "current_layer": {
    "id": "L0",
    "name": "Foundation"
  },
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
            { "round": 1, "verdict": "PASS", "eval_file": "launcher/.workspace/implementation_plan/evals/batch-1-round-1.md" }
          ]
        }
      ]
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `batch_size` | integer | Max items per batch |
| `min_rounds` | integer | Minimum eval rounds per batch |
| `max_rounds` | integer | Maximum eval rounds per batch |
| `plan_file` | string | Path to plan.json (read and updated by scaffold) |
| `state` | string | ORIENT, IMPLEMENT, EVALUATE, DONE |
| `current_layer` | object | The active layer (id and name) |
| `batch_number` | integer | Global batch counter (1-indexed, increments across layers) |
| `current_batch.items` | string[] | Item IDs in the active batch |
| `current_batch.current_item_index` | integer | Index of the item currently being implemented (0-based) |
| `current_batch.eval_round` | integer | Current evaluation round for this batch (0 = not yet evaluated) |
| `current_batch.evals` | array | Evaluation history for the current batch |
| `current_batch.evals[].round` | integer | Which eval round |
| `current_batch.evals[].verdict` | string | PASS or FAIL |
| `current_batch.evals[].eval_file` | string | Path to the eval report file |
| `layer_history` | array | Completed layer and batch records with eval trail |

---

## Reference Files

| File | Location | Used In | Purpose |
|------|----------|---------|---------|
| Evaluator Prompt | `EVALUATOR_PROMPT.md` | `eval` command output | Full instructions for the evaluation sub-agent: what to check, report format, verdict rules |

---

## Invariants

1. **Layer ordering enforced.** All items in layer N must have a terminal `passes` before any item in layer N+1 is delivered.
2. **Dependency ordering enforced.** An item is only delivered when all its `depends_on` items have a terminal `passes`.
3. **Item order preserved.** Items are delivered in the order they appear in the layer's `items` array.
4. **One item at a time.** IMPLEMENT presents a single item per advance call.
5. **plan.json is the progress record.** The `passes` and `rounds` fields on each item in plan.json reflect current state.
6. **State file tracks the process.** The scaffold state file tracks batches, evaluations, and layer history — the audit trail.
7. **No implicit state.** All information for transitions is in the state file and plan.json.
8. **Eval history is append-only.** Eval records accumulate and are never deleted.
9. **Min rounds enforced.** A PASS verdict before min_rounds triggers a re-implement cycle.
10. **Max rounds enforced.** A FAIL verdict at max_rounds force-accepts items as `failed`.
11. **Eval files must exist.** The scaffold verifies the `--eval-file` file exists before recording it.
12. **Two actors, two commands.** The engineer uses `advance`; the sub-agent uses `eval`. Neither uses the other's command.
13. **Scaffold does not parse eval files.** The verdict is provided by the engineer via `--verdict`. The eval file is stored as a reference path only.

---

## Edge Cases

- **Scenario:** Layer has fewer items than `batch_size`.
  - **Expected behavior:** A single batch contains all layer items.
  - **Rationale:** Small layers complete in one batch.

- **Scenario:** Batch has one item.
  - **Expected behavior:** IMPLEMENT presents the item, then advances directly to EVALUATE.
  - **Rationale:** Single-item batch is valid.

- **Scenario:** EVALUATE returns PASS but rounds < min_rounds.
  - **Expected behavior:** Re-enter IMPLEMENT for same batch items with eval file as guidance.
  - **Rationale:** Min rounds ensures sufficient verification.

- **Scenario:** EVALUATE returns FAIL and rounds < max_rounds.
  - **Expected behavior:** Re-enter IMPLEMENT for same batch items with eval file as guidance.
  - **Rationale:** Engineer uses eval report to find and fix issues.

- **Scenario:** EVALUATE returns FAIL and rounds >= max_rounds.
  - **Expected behavior:** Items marked `failed` in plan.json. Scaffold moves on.
  - **Rationale:** Max rounds prevents infinite loops.

- **Scenario:** All items in a layer are terminal (`passed` or `failed`).
  - **Expected behavior:** ORIENT advances to the next layer.
  - **Rationale:** Layer is complete regardless of individual item outcomes.

- **Scenario:** All layers complete.
  - **Expected behavior:** State goes to DONE.
  - **Rationale:** No more items to process.

- **Scenario:** Item has empty `depends_on` but is in L1.
  - **Expected behavior:** Still blocked until all L0 items are terminal. Layer ordering supersedes.
  - **Rationale:** Layers are sequential dependency tiers.

- **Scenario:** Item depends on an item that `failed`.
  - **Expected behavior:** The dependent item is still unblocked — `failed` is terminal.
  - **Rationale:** Force-accept allows progress. The failure is recorded but doesn't block downstream work.

- **Scenario:** `eval` called outside of EVALUATE state.
  - **Expected behavior:** Error naming the current state. Exit code 1.
  - **Rationale:** Eval context is only meaningful in EVALUATE.

- **Scenario:** Engineer runs `advance` in EVALUATE without `--eval-file` or `--verdict`.
  - **Expected behavior:** Error with usage: "implctl advance --eval-file <path> --verdict PASS|FAIL". Exit code 1.
  - **Rationale:** Clear usage guidance.

- **Scenario:** `advance --eval-file` with non-existent file.
  - **Expected behavior:** Error naming the missing path. Exit code 1.
  - **Rationale:** Eval file must exist to be recorded.

---

## Testing Criteria

### Init creates state file and adds tracking fields
- **Given:** Valid plan.json (no `passes` or `rounds` on items), `--batch-size 2 --min-rounds 1 --max-rounds 3`
- **When:** `init --from plan.json --batch-size 2 --max-rounds 3`
- **Then:** State file has `batch_size: 2`, `min_rounds: 1`, `max_rounds: 3`, state is ORIENT. plan.json items now have `passes: "pending"` and `rounds: 0`.

### Init rejects existing state
- **Given:** State file already exists
- **When:** `init`
- **Then:** Exit code 1.

### Init rejects invalid plan
- **Given:** plan.json with missing `layers` field
- **When:** `init`
- **Then:** Exit code 1, validation errors printed.

### Init rejects min exceeding max
- **Given:** `--min-rounds 5 --max-rounds 2`
- **When:** `init`
- **Then:** Exit code 1.

### ORIENT selects first batch of unblocked items
- **Given:** State is ORIENT, L0 has 4 items, batch_size is 2
- **When:** `advance`
- **Then:** State is IMPLEMENT. First item in batch presented. Action says "Implement this item."

### IMPLEMENT presents items one at a time
- **Given:** State is IMPLEMENT, batch has 2 items, on item 1
- **When:** `advance`
- **Then:** Item 1 marked `done` in plan.json. State is IMPLEMENT with item 2 presented.

### IMPLEMENT last item advances to EVALUATE
- **Given:** State is IMPLEMENT, on last item in batch
- **When:** `advance`
- **Then:** Item marked `done`. `rounds` incremented on all batch items. State is EVALUATE.

### EVALUATE requires eval-file and verdict
- **Given:** State is EVALUATE
- **When:** `advance` (no --eval-file or --verdict)
- **Then:** Exit code 1. Error shows correct usage.

### EVALUATE rejects non-existent eval-file
- **Given:** State is EVALUATE
- **When:** `advance --eval-file evals/missing.md --verdict PASS`
- **Then:** Exit code 1. Error names the missing path.

### EVALUATE rejects invalid verdict
- **Given:** State is EVALUATE
- **When:** `advance --eval-file evals/batch-1-round-1.md --verdict MAYBE`
- **Then:** Exit code 1.

### EVALUATE PASS with sufficient rounds marks items passed
- **Given:** State is EVALUATE, rounds >= min_rounds
- **When:** `advance --eval-file evals/batch-1-round-1.md --verdict PASS`
- **Then:** Items marked `passed` in plan.json. State is ORIENT.

### EVALUATE PASS with insufficient rounds re-enters IMPLEMENT
- **Given:** State is EVALUATE, min_rounds is 2, current rounds is 1
- **When:** `advance --eval-file evals/batch-1-round-1.md --verdict PASS`
- **Then:** Eval recorded. State is IMPLEMENT. First item shown with eval file in Action.

### EVALUATE FAIL within max_rounds re-enters IMPLEMENT
- **Given:** State is EVALUATE, rounds < max_rounds
- **When:** `advance --eval-file evals/batch-1-round-1.md --verdict FAIL`
- **Then:** Eval recorded. State is IMPLEMENT. First item shown with eval file in Action.

### EVALUATE FAIL at max_rounds force-accepts as failed
- **Given:** State is EVALUATE, rounds == max_rounds
- **When:** `advance --eval-file evals/batch-1-round-1.md --verdict FAIL`
- **Then:** Items marked `failed` in plan.json. State is ORIENT.

### IMPLEMENT after eval shows eval file in Action
- **Given:** State is IMPLEMENT, entered after EVALUATE with eval file
- **When:** Output displayed
- **Then:** Action contains "Study the eval file" with the eval file path.

### IMPLEMENT first time has no eval reference
- **Given:** State is IMPLEMENT, first time through batch (no prior eval)
- **When:** Output displayed
- **Then:** Action says "Implement this item." No eval file reference.

### eval command outputs item details
- **Given:** State is EVALUATE, batch has 2 items
- **When:** `eval`
- **Then:** Output includes evaluator prompt, full item details, and report target path.

### eval command shows previous evals on round 2+
- **Given:** State is EVALUATE, round 2, previous eval exists
- **When:** `eval`
- **Then:** Output includes PREVIOUS EVALUATIONS section with round 1 verdict and file path.

### eval command rejects non-EVALUATE state
- **Given:** State is IMPLEMENT
- **When:** `eval`
- **Then:** Exit code 1.

### Layer advances when all items terminal
- **Given:** All L0 items have terminal `passes`, L1 exists
- **When:** `advance` (in ORIENT)
- **Then:** `current_layer` advances to L1.

### Failed items don't block dependents
- **Given:** Item A is `failed`, item B depends on A
- **When:** Scaffold checks if B is unblocked
- **Then:** B is unblocked (failed is terminal).

### All layers complete goes to DONE
- **Given:** All items in all layers are terminal
- **When:** `advance` (in ORIENT)
- **Then:** State is DONE.

### DONE cannot advance
- **Given:** State is DONE
- **When:** `advance`
- **Then:** Error.

### Full lifecycle
- **Given:** Init with plan.json (2 layers, 3 items in L0, 2 in L1), `--batch-size 2 --min-rounds 1 --max-rounds 3`
- **When:** ORIENT → IMPLEMENT(A) → IMPLEMENT(B) → EVALUATE(PASS) → ORIENT → IMPLEMENT(C) → EVALUATE(PASS) → ORIENT(L1) → IMPLEMENT(D) → IMPLEMENT(E) → EVALUATE(PASS) → DONE
- **Then:** All items `passed`. State is DONE. 3 eval files in evals/ directory.

### Full lifecycle with FAIL and re-implement
- **Given:** Init with plan.json (1 layer, 2 items), `--batch-size 2 --min-rounds 1 --max-rounds 3`
- **When:** ORIENT → IMPLEMENT(A) → IMPLEMENT(B) → EVALUATE(FAIL) → IMPLEMENT(A) → IMPLEMENT(B) → EVALUATE(PASS) → DONE
- **Then:** All items `passed`. 2 eval rounds recorded. 2 eval files.

### Full lifecycle with min_rounds > 1
- **Given:** Init with plan.json (1 layer, 1 item), `--batch-size 1 --min-rounds 2 --max-rounds 3`
- **When:** ORIENT → IMPLEMENT(A) → EVALUATE(PASS) → IMPLEMENT(A) → EVALUATE(PASS) → DONE
- **Then:** Item `passed`. `rounds: 2`. 2 eval files.

### Full lifecycle with force-accept
- **Given:** Init with plan.json (1 layer, 1 item), `--batch-size 1 --min-rounds 1 --max-rounds 2`
- **When:** ORIENT → IMPLEMENT(A) → EVALUATE(FAIL) → IMPLEMENT(A) → EVALUATE(FAIL) → ORIENT → DONE
- **Then:** Item `failed` in plan.json. `rounds: 2`. Force-accept recorded. 2 eval files.

---

## Implements
- Implementation scaffold state machine for guiding Senior Software Engineers through plan.json item delivery
- Layer-ordered batching with dependency enforcement
- One-item-at-a-time IMPLEMENT presentation
- Item lifecycle tracking (`pending` → `done` → `passed`/`failed`) in plan.json
- Evaluation round tracking with `rounds` per item
- Min/max round enforcement (min for sufficient verification, max for force-accept on persistent failure)
- Dual-command interface: `advance` for engineer, `eval` for sub-agent
- Eval file reference tracking (scaffold stores path, does not parse contents)
- Re-implement cycle with eval file as guidance for corrections
