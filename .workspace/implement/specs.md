# Implementation Scaffold CLI

## Topic of Concern
> The implementation scaffold CLI guides a Senior Software Engineer through batched implementation steps using a JSON-backed state machine with iterative step delivery, confirmation gates, and post-implementation evaluation.

## Context

The implementation process takes a queue of implementation items (derived from an implementation plan) and breaks each item into batches of steps. The Senior Software Engineer implements each batch, confirms completion, and repeats until all batches are delivered. After all batches for an item, an evaluation agent verifies the implementation against the steps. The scaffold enforces this cycle, tracks progress, and provides unambiguous next-step guidance.

The scaffold tracks which steps were delivered in each batch, confirmation status, and evaluation results — creating an audit trail from first step to acceptance.

## Depends On
- None. The implementation scaffold is a standalone CLI tool with no runtime dependencies on other project components.

## Integration Points

| Component | Relationship |
|-----------|-------------|
| Implementation plan | The plan document defines the steps; the scaffold delivers them in batches and tracks completion |
| Evaluation sub-agent | The EVALUATE state is where the Senior Software Engineer asks a sub-agent to verify the implementation against all delivered steps |
| Queue input file | A JSON file conforming to the queue schema; the scaffold validates and ingests it during init |

---

## Interface

### Inputs

#### Queue Input File (provided via `--from` on `init`)

A JSON file conforming to this schema:

```json
{
  "items": [
    {
      "name": "WebSocket Client",
      "steps": [
        "Create WS2 client struct with connection config",
        "Implement Connect() method with retry logic",
        "Add message type discriminator for incoming messages",
        "Implement Send() with type-safe message envelope",
        "Add connection health check and reconnect",
        "Write unit tests for message serialization",
        "Write integration test for connect/disconnect lifecycle"
      ]
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `items` | array | yes | Ordered list of implementation items |
| `items[].name` | string | yes | Display name for the item (used in status output) |
| `items[].steps` | string[] | yes | Ordered list of implementation steps; must not be empty |

No additional fields are permitted.

#### CLI Arguments

| Command | Flags | Description |
|---------|-------|-------------|
| `init` | `--from <path>` (required), `--batch-size N` (required), `--batches N` (required) | Initialize state file from a validated queue |
| `advance` | `--confirm` (CONFIRM only), `--verdict PASS\|FAIL` (EVALUATE only), `--deficiencies <csv>` (with FAIL in EVALUATE), `--fixed <text>` (in EVALUATE after a prior FAIL) | Transition from current state to next; prints the new state and action guidance |
| `status` | none | Print full session state: current item, batch history, queue, completed |

### Outputs

All output is to stdout. The scaffold writes state changes to `implement-state.json`.

#### `advance` output

Every `advance` call prints a structured state block after transitioning:

```
State:   READ_STEPS
ID:      1
Item:    WebSocket Client
Batch:   2/4
Steps:
  1. Implement Send() with type-safe message envelope
  2. Add connection health check and reconnect
  3. Write unit tests for message serialization
Action:  Implement the steps above. When complete, run: advance
```

When advancing to CONFIRM (first call, no `--confirm`):

```
State:   CONFIRM
ID:      1
Item:    WebSocket Client
Batch:   2/4
Review:
  1. Implement Send() with type-safe message envelope
  2. Add connection health check and reconnect
  3. Write unit tests for message serialization
Action:  Review the steps above. Once implemented and passing, run: advance --confirm
```

This is a no-op — it re-displays the current batch steps for review and does not change state. The engineer can call `advance` (without `--confirm`) as many times as needed to re-read the steps.

When advancing to EVALUATE:

```
State:   EVALUATE
ID:      1
Item:    WebSocket Client
Batches: 4/4 completed
Action:  Ask the evaluation sub-agent to verify the implementation against all steps. Advance with --verdict PASS or --verdict FAIL --deficiencies <csv>.
```

#### `init` validation output (on failure)

When the input file fails validation, the scaffold prints:
1. Each validation error (missing field, extra field, wrong type, empty steps) with the path to the offending location.
2. The complete valid schema as a reference.

The scaffold exits with a non-zero code on validation failure.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `init` called when `implement-state.json` already exists | Error: "State file already exists. Delete it to reinitialize." Exit code 1. | Prevents accidental loss of in-progress state |
| `--from` file fails schema validation (missing name, empty steps, extra fields) | Error listing violations. Prints full valid schema. Exit code 1. | Engineer needs to see what's wrong |
| `--batch-size` < 1 | Error: "--batch-size must be at least 1." Exit code 1. | Invalid configuration |
| `--batches` < 1 | Error: "--batches must be at least 1." Exit code 1. | Invalid configuration |
| Item has fewer steps than a single batch | Not an error. A single batch contains all steps. | Small items are valid |
| `advance --confirm` outside of CONFIRM state | Error naming the current state. Exit code 1. | Confirm is only valid in CONFIRM |
| `advance --verdict` outside of EVALUATE state | Error naming the current state. Exit code 1. | Verdict is only valid in EVALUATE |
| `advance` in CONFIRM without `--confirm` | Prints the batch steps and reminds the engineer to run `advance --confirm`. Does not transition. Exit code 0. | Two-step confirmation: first review, then confirm |
| `advance` in EVALUATE without `--verdict` | Error: "EVALUATE requires --verdict (PASS or FAIL)." Exit code 1. | Verdict determines the transition |
| `advance` or `status` called before `init` | Error. Exit code 1. | State file must exist |

---

## Behavior

### Batch Calculation

When an item enters ORIENT, the scaffold calculates batches by dividing the item's steps evenly across the configured number of batches (`--batches`). The last batch absorbs any remainder.

- **batch_size** is determined per-item: `ceil(len(steps) / batches)`
- **batch_count** is `min(batches, len(steps))` (can't have more batches than steps)

Each batch is a contiguous slice of the step list. Steps are never reordered.

Example: 7 steps with `--batches 3` → batches of [3, 2, 2] steps.

### Initializing a Session

#### Preconditions
- No `implement-state.json` exists.
- `--from`, `--batch-size`, `--batches` are provided.

#### Steps
1. Read and parse the file at `--from`.
2. Validate against the queue schema.
3. If validation fails: print errors and schema, exit code 1.
4. If validation passes: create `implement-state.json` with state ORIENT.

#### Postconditions
- State file exists with `batch_size`, `batches` set, queue populated, completed empty.

---

### State Machine

```
ORIENT → READ_STEPS → CONFIRM ──→ READ_STEPS → CONFIRM ──→ ... → EVALUATE → ACCEPT → ORIENT
                                   (repeat for each batch)                              ↓
                                                                                       DONE
                                                                                  (queue empty)
```

### Advancing State

#### Preconditions
- `implement-state.json` exists.

#### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | always | READ_STEPS | Pull next item from queue into `current_item`. Calculate batches. Set batch to 1. Populate `current_steps` with first batch of steps. |
| READ_STEPS | always | CONFIRM | — (steps were displayed on entry to READ_STEPS) |
| CONFIRM | `--confirm` provided, more batches remain | READ_STEPS | Record batch as confirmed. Increment batch counter. Populate `current_steps` with next batch. |
| CONFIRM | `--confirm` provided, no more batches | EVALUATE | Record batch as confirmed. All steps delivered. |
| EVALUATE | `--verdict PASS` | ACCEPT | Record eval (PASS). |
| EVALUATE | `--verdict FAIL` | READ_STEPS | Record eval (FAIL + deficiencies). Deliver the deficiency descriptions as remediation steps in a new batch. Increment eval round. |
| ACCEPT | queue non-empty | ORIENT | Move item to completed (with batch history + eval). |
| ACCEPT | queue empty | DONE | Move item to completed. |
| DONE | — | Error: "session complete." | Terminal state. |

#### Postconditions
- State file reflects the new state.
- Batch confirmation records accumulate on `current_item.batches`.
- Eval records accumulate on `current_item.evals`.

#### Error Handling
- Invalid flags for state: specific error per state.
- Invalid verdict value: error.

---

### EVALUATE Behavior

When all batches for an item are confirmed, the scaffold transitions to EVALUATE. The Senior Software Engineer asks an evaluation sub-agent to:

1. Review all steps that were delivered across all batches.
2. Verify the implementation matches the steps.
3. Report a verdict (PASS or FAIL with deficiencies).

If FAIL: the scaffold re-enters READ_STEPS with a remediation batch containing the deficiency descriptions as new steps. After the engineer confirms the remediation batch, the scaffold returns to EVALUATE for re-assessment.

If PASS: the scaffold transitions to ACCEPT.

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `--batch-size` | integer | none (required) | Maximum number of steps delivered per batch |
| `--batches` | integer | none (required) | Target number of batches per item (steps are divided evenly) |
| `--from` | string | none (required on init) | Path to queue input JSON file |

---

## State File Schema

```json
{
  "batch_size": 3,
  "batches": 4,
  "state": "READ_STEPS",
  "current_item": {
    "id": 1,
    "name": "WebSocket Client",
    "all_steps": [
      "Create WS2 client struct with connection config",
      "Implement Connect() method with retry logic",
      "Add message type discriminator for incoming messages",
      "Implement Send() with type-safe message envelope",
      "Add connection health check and reconnect",
      "Write unit tests for message serialization",
      "Write integration test for connect/disconnect lifecycle"
    ],
    "current_steps": [
      "Implement Send() with type-safe message envelope",
      "Add connection health check and reconnect",
      "Write unit tests for message serialization"
    ],
    "batch": 2,
    "batch_count": 4,
    "confirmed_batches": [
      {
        "batch": 1,
        "steps": [
          "Create WS2 client struct with connection config",
          "Implement Connect() method with retry logic",
          "Add message type discriminator for incoming messages"
        ]
      }
    ],
    "eval_round": 0,
    "evals": []
  },
  "queue": [],
  "completed": [
    {
      "id": 0,
      "name": "Config Models",
      "batches_taken": 3,
      "eval_rounds": 1,
      "evals": [
        { "round": 1, "verdict": "PASS", "deficiencies": null, "fixed": "" }
      ]
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `batch_size` | integer | Max steps per batch |
| `batches` | integer | Target batches per item |
| `state` | string | ORIENT, READ_STEPS, CONFIRM, EVALUATE, ACCEPT, DONE |
| `current_item.all_steps` | string[] | Complete step list for the item |
| `current_item.current_steps` | string[] | Steps in the active batch |
| `current_item.batch` | integer | Current batch number (1-indexed) |
| `current_item.batch_count` | integer | Total batches for this item |
| `current_item.confirmed_batches` | array | History of confirmed batches with their steps |
| `current_item.eval_round` | integer | Current evaluation round (0 = not yet evaluated) |
| `current_item.evals` | array | Evaluation history |
| `current_item.evals[].round` | integer | Which eval round |
| `current_item.evals[].verdict` | string | PASS or FAIL |
| `current_item.evals[].deficiencies` | string[] | Failed items (on FAIL) |
| `current_item.evals[].fixed` | string | What was fixed (populated on re-evaluation after FAIL) |
| `completed[].batches_taken` | integer | How many batches were confirmed |
| `completed[].eval_rounds` | integer | How many evaluation rounds |
| `completed[].evals` | array | Full eval history carried from current_item |

---

## Invariants

1. **Single active item.** At most one item is in `current_item` at any time.
2. **Batch monotonicity.** The batch counter only increments.
3. **Step order preserved.** Steps are delivered in the order defined in the input.
4. **Queue order preserved.** Items are pulled from the front of the queue.
5. **State file is the only mutable artifact.** The queue input file is read once at init.
6. **No implicit state.** All information for transitions is in the state file.
7. **Batch history is append-only.** Confirmed batch records accumulate and are never deleted.
8. **Confirmation is explicit.** The CONFIRM state requires `--confirm` — advancing without it is an error.

---

## Edge Cases

- **Scenario:** Item has fewer steps than `batch_size`.
  - **Expected behavior:** A single batch contains all steps.
  - **Rationale:** Small items should not fail; they just complete in one batch.

- **Scenario:** Item has exactly `batches` steps.
  - **Expected behavior:** Each batch has exactly 1 step.
  - **Rationale:** Evenly divisible case.

- **Scenario:** `advance` called in CONFIRM without `--confirm`.
  - **Expected behavior:** Re-displays the current batch steps and the confirm instruction. No state change. Exit code 0.
  - **Rationale:** Two-step confirmation — the engineer can review steps before explicitly confirming.

- **Scenario:** EVALUATE returns FAIL.
  - **Expected behavior:** Deficiencies become remediation steps in a new READ_STEPS batch. After confirmation, returns to EVALUATE.
  - **Rationale:** The engineer fixes issues and the evaluation sub-agent re-verifies.

- **Scenario:** EVALUATE returns FAIL multiple times.
  - **Expected behavior:** Each failure creates a new remediation batch. Eval round increments. No limit on retries.
  - **Rationale:** The scaffold trusts the engineer and agent to converge.

- **Scenario:** Queue has one item.
  - **Expected behavior:** After ACCEPT, state goes to DONE (not ORIENT).
  - **Rationale:** No more items to process.

---

## Testing Criteria

### Init creates state file
- **Given:** Valid queue file, `--batch-size 3 --batches 4`
- **When:** `init --from queue.json --batch-size 3 --batches 4`
- **Then:** State file has `batch_size: 3`, `batches: 4`, state is ORIENT.

### Init rejects existing state
- **Given:** State file already exists
- **When:** `init`
- **Then:** Exit code 1.

### Init rejects invalid queue
- **Given:** Queue file with missing `steps` field
- **When:** `init`
- **Then:** Exit code 1, validation errors printed.

### Init rejects empty steps
- **Given:** Queue file with `"steps": []`
- **When:** `init`
- **Then:** Exit code 1.

### ORIENT pulls first item and delivers first batch
- **Given:** State is ORIENT with items in queue
- **When:** `advance`
- **Then:** State is READ_STEPS. `current_steps` contains first batch of steps.

### READ_STEPS advances to CONFIRM
- **Given:** State is READ_STEPS
- **When:** `advance`
- **Then:** State is CONFIRM.

### CONFIRM without --confirm re-displays steps
- **Given:** State is CONFIRM
- **When:** `advance` (no --confirm)
- **Then:** Exit code 0. Output shows batch steps and confirm instruction. State unchanged.

### CONFIRM with more batches returns to READ_STEPS
- **Given:** State is CONFIRM, batch 1 of 3
- **When:** `advance --confirm`
- **Then:** State is READ_STEPS. Batch is 2. `current_steps` has next batch.

### CONFIRM on last batch goes to EVALUATE
- **Given:** State is CONFIRM, batch 3 of 3 (last batch)
- **When:** `advance --confirm`
- **Then:** State is EVALUATE.

### EVALUATE requires verdict
- **Given:** State is EVALUATE
- **When:** `advance` (no verdict)
- **Then:** Exit code 1.

### EVALUATE PASS goes to ACCEPT
- **Given:** State is EVALUATE
- **When:** `advance --verdict PASS`
- **Then:** State is ACCEPT.

### EVALUATE FAIL returns to READ_STEPS with remediation
- **Given:** State is EVALUATE
- **When:** `advance --verdict FAIL --deficiencies "Missing error handling,No tests"`
- **Then:** State is READ_STEPS. `current_steps` contains deficiency descriptions as steps.

### ACCEPT with queue goes to ORIENT
- **Given:** State is ACCEPT, queue non-empty
- **When:** `advance`
- **Then:** State is ORIENT. Completed has 1 entry.

### ACCEPT with empty queue goes to DONE
- **Given:** State is ACCEPT, queue empty
- **When:** `advance`
- **Then:** State is DONE. Completed has entry with batch history and evals.

### DONE cannot advance
- **Given:** State is DONE
- **When:** `advance`
- **Then:** Error.

### Batch history carried to completed
- **Given:** Item with 2 batches confirmed, then PASS
- **When:** Item moves to completed
- **Then:** `completed[].batches_taken` is 2. `confirmed_batches` data preserved.

### Eval history carried to completed
- **Given:** Item with FAIL then PASS (2 eval rounds)
- **When:** Item moves to completed
- **Then:** `completed[].evals` has both records.

### Full lifecycle
- **Given:** Init with 1 item, 3 steps, `--batches 2`
- **When:** ORIENT → READ_STEPS → CONFIRM → READ_STEPS → CONFIRM → EVALUATE(PASS) → ACCEPT → DONE
- **Then:** Completed has 1 entry. 2 confirmed batches.

### Full lifecycle with FAIL and remediation
- **Given:** Init with 1 item, 2 steps, `--batches 1`
- **When:** ORIENT → READ_STEPS → CONFIRM → EVALUATE(FAIL) → READ_STEPS → CONFIRM → EVALUATE(PASS) → ACCEPT → DONE
- **Then:** Completed has 1 entry. 2 eval rounds. Remediation batch in history.

---

## Implements
- Implementation scaffold state machine for guiding Senior Software Engineers through batched step delivery
- Confirmation gates for explicit progress tracking
- Evaluation loop with remediation for failed verifications
