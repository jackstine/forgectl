# Batch Implementation

## Topic of Concern
> The scaffold implements plan items in dependency-ordered batches through iterative evaluation.

## Context

The implementing phase delivers plan items one at a time within dependency-ordered batches. After each batch is fully implemented, an evaluation sub-agent verifies against acceptance criteria through iterative rounds. Items progress through `pending` → `done` → `passed`/`failed` states tracked in plan.json.

Layers enforce a coarse ordering (all layer N items must be terminal before layer N+1), while `depends_on` within a layer provides fine-grained ordering. Batches are groups of up to `implementing.batch` unblocked items drawn from the current layer.

## Depends On
- **phase-transitions** — the planning→implementing phase shift validates plan.json, adds tracking fields, and triggers ORIENT.
- **session-init** — alternatively, `init --phase implementing` starts the implementing phase directly.
- **state-persistence** — reads and writes the state file.

## Integration Points

| Spec | Relationship |
|------|-------------|
| plan.json | Planning produces it; implementing consumes and mutates it (adding `passes` and `rounds` fields) |
| Implementation evaluator prompt (`evaluators/impl-eval.md`, embedded in binary) | Full instructions for the implementation evaluation sub-agent: what to check, report format, verdict rules |
| phase-transitions | DONE → PHASE_SHIFT when plans remain (implementing → planning when `plan_all_before_implementing: false`; implementing → the next domain's implementation phase, routed by that plan's `kind` to `implementing` or `ui_implementing`, at a domain boundary when `true`) |

---

## Interface

### Inputs

#### `advance` flags — Implementing Phase

| State | Flags |
|-------|-------|
| IMPLEMENT | `--message <text>` (optional, every round, when `enable_commits: true`; appended to the synthesized item-description commit message — see docs/auto-committing.md) |
| EVALUATE | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `eval_mode: "report"`) |
| COMMIT | (no flags; state only appears when `enable_commits: false` — a no-op advance. When `enable_commits: true`, COMMIT is skipped and the batch-terminal commit happens inline at the terminal EVALUATE transition — see COMMIT State) |

#### `eval` command

| Command | Flags | Description |
|---------|-------|-------------|
| `eval` | none | Output full evaluation context for the sub-agent. Only valid in implementing EVALUATE. |

### Outputs

#### `advance` output

**Entering ORIENT** (after PHASE_SHIFT or init with `--phase implementing`):

```
State:   ORIENT
Phase:   implementing
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Config:  implementing.batch=2, eval.rounds=1-3

Initialized plan.json for implementation:
  Items:  5 (passes: pending, rounds: 0)
  Layers: 2 (L0 Foundation: 3 items, L1 Core: 2 items)

Action:  STOP please review and discuss with user before continuing.
         After completion of the above, advance to select first batch.
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
Specs:   service-configuration.md#interface-outputs
         Read: git show abc1234 def5678 -- '**/service-configuration.md'
Refs:    notes/config.md#types
Tests:   1 functional
Action:  Implement this item.
         Please review the specification(s) above if you have not already done so —
         run the git command shown under each spec to read its definition.
         Please review the reference file(s) under Refs if you have not already done so.
         After completion of the above, advance to continue.
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
Specs:   service-configuration.md#behavior-loading
         Read: git show abc1234 def5678 -- '**/service-configuration.md'
         config-validation.md#behavior-strict-mode
         Read: git show abc1234 def5678 -- '**/config-validation.md'
Refs:    notes/config.md#load
Tests:   2 functional, 2 rejection, 2 edge_case
Action:  Implement this item.
         Please review the specification(s) above if you have not already done so —
         run the git command shown under each spec to read its definition.
         Please review the reference file(s) under Refs if you have not already done so.
         After completion of the above, advance to continue.
```

**Entering IMPLEMENT** (first item in batch, after eval — round 2+, `eval_mode: "report"`):

```
State:   IMPLEMENT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Round:   1/3
Eval:    launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md
Note:    PASS recorded for round 1. Minimum rounds not yet met (1/2).
Item:    [config.types] ServiceEndpoint and ServicesConfig structs
         Go structs for validated service endpoint configuration.
         (1 of 2 in batch)
Steps:
  1. Define ServiceEndpoint struct with Host (string) and Port (int) fields
  2. Define ServicesConfig struct with three named ServiceEndpoint fields
  3. Add YAML struct tags for deserialization
Files:   internal/config/types.go
Specs:   service-configuration.md#interface-outputs
         Read: git show abc1234 def5678 -- '**/service-configuration.md'
Refs:    notes/config.md#types
Tests:   1 functional
Action:  Study the eval file "launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md"
         and implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Please review the specification(s) above if you have not already done so —
         run the git command shown under each spec to read its definition.
         Please review the reference file(s) under Refs if you have not already done so.
         After completion of the above, advance to continue.
```

**`eval_mode: "direct"` never re-enters IMPLEMENT after round 1.** IMPLEMENT runs exactly once per batch under `eval_mode: "direct"`. A FAIL, or a PASS below `min_rounds`, re-enters EVALUATE directly for another round instead of cycling back through IMPLEMENT — see the "Entering EVALUATE (implementing phase, subsequent round, `eval_mode: "direct"`)" example below. There is no IMPLEMENT round-2+ example for `eval_mode: "direct"` because that state transition does not occur.

**Entering IMPLEMENT** (first item in batch, after eval — round 2+, `eval_mode: "conversational"`):

```
State:   IMPLEMENT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Round:   1/3
Note:    PASS recorded for round 1. Minimum rounds not yet met (1/2).
Item:    [config.types] ServiceEndpoint and ServicesConfig structs
         Go structs for validated service endpoint configuration.
         (1 of 2 in batch)
Steps:
  1. Define ServiceEndpoint struct with Host (string) and Port (int) fields
  2. Define ServicesConfig struct with three named ServiceEndpoint fields
  3. Add YAML struct tags for deserialization
Files:   internal/config/types.go
Specs:   service-configuration.md#interface-outputs
         Read: git show abc1234 def5678 -- '**/service-configuration.md'
Refs:    notes/config.md#types
Tests:   1 functional
Action:  Make corrections based off communication with the evaluator.
         Implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Please review the specification(s) above if you have not already done so —
         run the git command shown under each spec to read its definition.
         Please review the reference file(s) under Refs if you have not already done so.
         After completion of the above, advance to continue.
```

**Entering EVALUATE** (implementing phase, `eval_mode: "report"`):

```
State:    EVALUATE
Phase:    implementing
Layer:    L0 Foundation
Batch:    1/2
Round:    1/3
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Action:   Please spawn 1 sonnet general-purpose sub-agent to evaluate the implementation batch.
          The sub-agent should run: forgectl eval
          The sub-agent must write its report to this exact path:
            launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md
          After completion of the above, advance with --eval-report launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md --verdict PASS|FAIL
          --eval-report takes this file path, not the report text.
```

The report path printed in the Action is identical to the path printed in the
`--- REPORT OUTPUT ---` section of `forgectl eval` (same batch and round). The
scaffold computes it deterministically, so the engineer can pass it verbatim
whether or not the sub-agent echoes it back.

**Entering EVALUATE** (implementing phase, `eval_mode: "direct"`):

```
State:    EVALUATE
Phase:    implementing
Layer:    L0 Foundation
Batch:    1/2
Round:    1/3
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Action:   Please spawn 1 sonnet general-purpose sub-agent to evaluate and correct the batch.
          Sub-agent runs: forgectl eval
          Batch files have been staged. Sub-agent makes corrections directly.
          After completion of the above, advance with --verdict PASS|FAIL
```

**Entering EVALUATE** (implementing phase, subsequent round, `eval_mode: "direct"`):

```
State:    EVALUATE
Phase:    implementing
Layer:    L0 Foundation
Batch:    1/2
Round:    2/3
Note:     FAIL recorded for round 1. Corrections were made directly to batch files.
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Action:   Please spawn 1 sonnet general-purpose sub-agent to evaluate and correct the batch.
          Sub-agent runs: forgectl eval
          Batch files have been staged. Sub-agent makes corrections directly.
          After completion of the above, advance with --verdict PASS|FAIL
```

Under `eval_mode: "direct"`, every round after the first re-enters EVALUATE in this same form — Round increments, a `Note:` line records the prior round's verdict, and no intervening IMPLEMENT state is presented. This continues until PASS at or above `min_rounds`, or FAIL at `max_rounds` (force-accept).

**Entering EVALUATE** (implementing phase, `eval_mode: "conversational"`):

```
State:    EVALUATE
Phase:    implementing
Layer:    L0 Foundation
Batch:    1/2
Round:    1/3
Items:
  - [config.types] ServiceEndpoint and ServicesConfig structs
  - [config.load] Load YAML, apply defaults, validate strictly
Action:   Please spawn 1 sonnet general-purpose sub-agent to evaluate the implementation batch.
          The sub-agent should run: forgectl eval
          After completion of the above, advance with --verdict PASS|FAIL
```

**Terminal EVALUATE auto-commit** (`enable_commits: true`) — COMMIT does not render:

When `enable_commits` is `true`, the batch-terminal commit happens inline as part of the same `advance` that records the terminal verdict at EVALUATE. No COMMIT state is presented, no `--message` flag is involved, and the very next rendered state is ORIENT or DONE (shown further below) — the engineer's `advance --eval-report ... --verdict PASS` (or `FAIL` at force-accept) both records the verdict and produces the commit in one step. The commit message is synthesized from the batch's item descriptions (see docs/auto-committing.md); if nothing is left to stage (every item already committed per-item and no evaluator left corrections), the commit is skipped silently.

**Entering COMMIT** (after EVALUATE, batch terminal, `enable_commits: false`):

```
State:   COMMIT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Items:
  - [config.types] passed
  - [config.load] passed
Action:  Advance to continue.
```

**Entering COMMIT** (after force-accept, `enable_commits: false`):

```
State:   COMMIT
Phase:   implementing
Layer:   L1 Core
Batch:   3/3
Items:
  - [daemon.types] failed (force-accept, 3/3 rounds)
  - [daemon.io] failed (force-accept, 3/3 rounds)
Action:  Advance to continue.
```

**Entering ORIENT** (batch complete, more items in layer — reached via COMMIT when `enable_commits: false`, or directly from the terminal EVALUATE auto-commit when `enable_commits: true`):

```
State:    ORIENT
Phase:    implementing
Layer:    L0 Foundation
Progress: 2/4 items passed
Next:     2 unblocked items in next batch
Action:   STOP please review and discuss with user before continuing.
          After completion of the above, advance to select next batch.
```

**Entering ORIENT** (batch complete, layer complete, more layers):

```
State:    ORIENT
Phase:    implementing
Layer:    L0 Foundation
Progress: 3/3 items passed — layer complete
Next:     L1 Core — 2 items: [daemon.types], [daemon.io]
Action:   STOP please review and discuss with user before continuing.
          After completion of the above, advance to next layer.
```

**Entering ORIENT** (batch complete, layer complete, last layer):

```
State:    ORIENT
Phase:    implementing
Layer:    L1 Core
Progress: 2/2 items passed — layer complete (final layer)
Action:   STOP please review and discuss with user before continuing.
          After completion of the above, advance to continue.
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
Action:   After completion of the above, advance to next layer.
```

**DONE** (all items complete, more plans remaining):

```
State:   DONE
Phase:   implementing
Domain:  launcher
Summary:
  L0 Foundation:  3/3 passed
  L1 Core:        2/2 passed
  Total:          5/5 items passed
  Eval rounds:    7 across 3 batches
Action:  Domain complete. Advance to continue to next domain.
```

**DONE** (all items complete, no plans remaining — session complete):

```
State:   DONE
Phase:   implementing
Summary:
  launcher:  5/5 items passed (3 batches)
  portal:    3/3 items passed (2 batches)
  Total:     8/8 items passed
Action:  All items complete. Session done.
```

#### `eval` output

When `eval_mode: "report"`:

```
=== IMPLEMENTATION EVALUATION ROUND 1/3 ===
Layer: L0 Foundation
Batch: 1/2

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/impl-eval.md>

--- ITEMS TO EVALUATE ---

[1] config.types — ServiceEndpoint and ServicesConfig structs
    Description: Go structs for validated service endpoint configuration.
    Specs:       service-configuration.md#interface-outputs
    Read:        git show abc1234 def5678 -- '**/service-configuration.md'
    Refs:        notes/config.md#types
    Files:       internal/config/types.go
    Steps:
      1. Define ServiceEndpoint struct with Host (string) and Port (int) fields
      2. Define ServicesConfig struct with three named ServiceEndpoint fields
      3. Add YAML struct tags for deserialization
    Tests:
      [functional] Three named fields, not a map

[2] config.load — Load YAML, apply defaults, validate strictly
    Description: Parse spectacular.yml, apply default host/port values.
    Specs:       service-configuration.md#behavior-loading
    Read:        git show abc1234 def5678 -- '**/service-configuration.md'
                  config-validation.md#behavior-strict-mode
    Read:        git show abc1234 def5678 -- '**/config-validation.md'
    Refs:        notes/config.md#load
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

Write your evaluation report to this exact path (create the file — do not only
describe it):
  launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md

When done, your final message must be only this path and the verdict, e.g.:
  launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md FAIL
```

The `Read:` line under each `Specs:` entry carries the same bounded `git show
<commits> -- '**/<file>'` command described under IMPLEMENT Behavior, using the
current plan's `spec_commits`. It is present in `eval` output in every
`eval_mode` — the eval sub-agent needs to inspect the exact spec definition to
judge or correct code against it, independent of how it reports its verdict.
When `spec_commits` is empty, the `Read:` line is omitted, mirroring IMPLEMENT.

Subsequent rounds with `eval_mode: "report"` include previous evaluations:

```
=== IMPLEMENTATION EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: PASS — launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md

--- REPORT OUTPUT ---

Write your evaluation report to this exact path (create the file — do not only
describe it):
  launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-2.md

When done, your final message must be only this path and the verdict, e.g.:
  launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-2.md PASS
```

When `eval_mode: "direct"`, the `--- REPORT OUTPUT ---` section is included but the sub-agent makes corrections directly to batch files instead of writing a report. The `--- PREVIOUS EVALUATIONS ---` section is included in subsequent rounds.

```
=== IMPLEMENTATION EVALUATION ROUND 1/3 ===
Layer: L0 Foundation
Batch: 1/2

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/impl-eval.md>

--- ITEMS TO EVALUATE ---

[1] config.types — ServiceEndpoint and ServicesConfig structs
    ...

[2] config.load — Load YAML, apply defaults, validate strictly
    ...

--- REPORT OUTPUT ---

Make corrections directly to the batch files.
```

Subsequent rounds with `eval_mode: "direct"`:

```
=== IMPLEMENTATION EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: PASS

--- REPORT OUTPUT ---

Make corrections directly to the batch files.
```

When `eval_mode: "conversational"`, the `--- REPORT OUTPUT ---` and `--- PREVIOUS EVALUATIONS ---` sections are omitted. The eval sub-agent receives item details and evaluator instructions but does not write a file. It communicates its verdict directly to the architect.

```
=== IMPLEMENTATION EVALUATION ROUND 1/3 ===
Layer: L0 Foundation
Batch: 1/2

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/impl-eval.md>

--- ITEMS TO EVALUATE ---

[1] config.types — ServiceEndpoint and ServicesConfig structs
    ...

[2] config.load — Load YAML, apply defaults, validate strictly
    ...
```

Subsequent rounds with `eval_mode: "conversational"`:

```
=== IMPLEMENTATION EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: PASS
```

#### Eval Report Locations

Implementation eval reports:
```
<domain>/.forgectl_workspace/implementation_plan/evals/batch-N-round-M.md
```

#### `status` output — Implementing (compact)

The compact `status` output for implementing shows the current item, layer, round, action, and a one-line progress summary:

```
Item:    [daemon.io] PID file I/O operations (2 of 2)
Layer:   L1 Core (2/5 layers)
Round:   0

Action:  Implement this item.
         After completion of the above, advance to continue.

Progress: 3/5 passed, 0 failed, 2 remaining
```

#### `status --verbose` output — Implementing section

With `--verbose`, the full layer-by-item breakdown is appended, including spec and file references:

```
--- Implementing ---

  Layer L0 (Foundation): complete
    [bootstrap]     passed  (1 round)
      Specs: service-configuration.md#behavior-bootstrap
      Refs:  notes/config.md#bootstrap
      Files: internal/config/bootstrap.go
    [config.types]  passed  (1 round)
      Specs: service-configuration.md#interface-outputs
      Refs:  notes/config.md#types
      Files: internal/config/types.go
    [config.load]   passed  (2 rounds)
      Specs: service-configuration.md#behavior-loading
             config-validation.md#behavior-strict-mode
      Refs:  notes/config.md#load
      Files: internal/config/load.go, internal/config/load_test.go

  Layer L1 (Core): in progress
    [daemon.types]  done    (0 rounds)
      Specs: daemon-lifecycle.md#behavior-types
      Refs:  notes/daemon.md#types
      Files: internal/daemon/types.go
    [daemon.io]     pending (0 rounds)
      Specs: daemon-lifecycle.md#behavior-io
      Refs:  notes/daemon.md#io
      Files: internal/daemon/io.go, internal/daemon/io_test.go
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance` in implementing EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in implementing EVALUATE without `--eval-report` when `eval_mode: "report"` | Error. Exit code 1. | Every evaluation must reference its report when eval output is enabled |
| `advance --eval-report` pointing to non-existent file | Error naming the path. Exit code 1. | Report must exist to be recorded |
| `advance --eval-report` given report prose instead of a path (value has no path separator and contains whitespace) | Error naming the value, with a hint that `--eval-report` expects the file path the eval sub-agent wrote, not the report text. Exit code 1. | Common failure: sub-agent described findings without writing the file, so the engineer passed the prose |
| `advance --eval-report` when `eval_mode` is not `"report"` | Warning: `--eval-report is ignored, --eval-report is only used in report mode`. Command proceeds. | Consistent with `--message` warning pattern |
| `eval` outside of implementing EVALUATE | Error naming current state and phase. Exit code 1. | Eval context only available in EVALUATE |

---

## Behavior

### Batch Calculation

Batches are groups of items drawn from the current layer. The scaffold selects up to `implementing.batch` unblocked items.

An item is **unblocked** when:
1. All items in prior layers have a terminal `passes` value (`passed` or `failed`)
2. All items in its `depends_on` list have a terminal `passes` value

Items are selected in the order they appear in the layer's `items` array.

### State Machine

```
ORIENT → IMPLEMENT(1)* → IMPLEMENT(2)* → ... → EVALUATE
                                                    │
                                      ┌──────────────┼──────────────┐
                                      │              │              │
                                PASS + rounds   FAIL + rounds   PASS/FAIL
                                >= min_rounds   < max_rounds    at boundary
                                      │              │              │
                                      │   report/conversational:    │
                                      │   IMPLEMENT(1)*→...         │
                                      │   (re-implement)             │
                                      │   direct:                   │
                                      │   EVALUATE (re-evaluate,     │
                                      │   no re-implement)           │
                                      ▼                              │
                        [terminal: PASS >= min_rounds, or            │
                         FAIL >= max_rounds (force-accept)] ◄────────┘
                                      │
                        ┌─────────────┴─────────────┐
                        │                            │
             enable_commits: true         enable_commits: false
             auto-commit batch inline     COMMIT
                        │                            │
                        └─────────────┬──────────────┘
                                      ▼
                                ORIENT/DONE

* Each IMPLEMENT item auto-commits immediately when `enable_commits: true`
  (message synthesized from the item's `description`).
```

### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | unblocked items exist in current layer | IMPLEMENT | Select batch. Present first item. |
| ORIENT | all layer items terminal, more layers | ORIENT (next layer) | Advance `current_layer`. |
| ORIENT | all layers complete | DONE | — |
| IMPLEMENT | more items in batch | IMPLEMENT | Mark current item `done`. Auto-commit item when `enable_commits: true` (message synthesized from item `description`, optional `--message` appended). Present next item. |
| IMPLEMENT | last item in batch | EVALUATE | Mark current item `done`. Auto-commit item when `enable_commits: true`. Increment `rounds` on all batch items. |
| EVALUATE | PASS, rounds >= `implementing.eval.min_rounds`, `enable_commits: false` | COMMIT | Mark items `passed`. Record eval. |
| EVALUATE | PASS, rounds >= `implementing.eval.min_rounds`, `enable_commits: true` | ORIENT/DONE | Mark items `passed`. Record eval. Auto-commit batch inline (message synthesized from batch item descriptions; skipped silently if nothing staged). |
| EVALUATE | PASS, rounds < `implementing.eval.min_rounds`, `eval_mode` is `"report"` or `"conversational"` | IMPLEMENT | Record eval. Re-present first item with eval file. |
| EVALUATE | PASS, rounds < `implementing.eval.min_rounds`, `eval_mode: "direct"` | EVALUATE | Record eval. Re-evaluate directly (no re-implement round). |
| EVALUATE | FAIL, rounds < `implementing.eval.max_rounds`, `eval_mode` is `"report"` or `"conversational"` | IMPLEMENT | Record eval. Re-present first item with eval file. |
| EVALUATE | FAIL, rounds < `implementing.eval.max_rounds`, `eval_mode: "direct"` | EVALUATE | Record eval. Re-evaluate directly (no re-implement round). |
| EVALUATE | FAIL, rounds >= `implementing.eval.max_rounds`, `enable_commits: false` | COMMIT | Mark items `failed`. Record eval. Force-accept. |
| EVALUATE | FAIL, rounds >= `implementing.eval.max_rounds`, `enable_commits: true` | ORIENT/DONE | Mark items `failed`. Record eval. Force-accept. Auto-commit batch inline. |
| COMMIT | more batches or layers | ORIENT | — |
| COMMIT | all layers complete | DONE | — |
| DONE | `plan_all_before_implementing: false`, planning queue non-empty | PHASE_SHIFT | PHASE_SHIFT (implementing → planning). Return to planning for next domain. |
| DONE | `plan_all_before_implementing: true`, implementing plan queue non-empty | PHASE_SHIFT | PHASE_SHIFT (implementing → next implementation phase, domain boundary). Pull next plan; route to `implementing` or `ui_implementing` by that plan's `kind`. |
| DONE | no plans remaining | _(terminal)_ | Session complete. |

### Item `passes` Transitions

| Event | `passes` change |
|-------|----------------|
| Engineer advances past item in IMPLEMENT | `pending` → `done` |
| EVALUATE PASS + rounds >= min_rounds | `done` → `passed` |
| EVALUATE FAIL + rounds >= max_rounds | `done` → `failed` |
| EVALUATE FAIL + rounds < max_rounds | stays `done` |
| EVALUATE PASS + rounds < min_rounds | stays `done` |

### IMPLEMENT Behavior

Presents **one item at a time**. Displays full context: name, description, steps, files, specs (each with a `Read:` command), refs, test summary.

**Spec `Read:` command.** Each `Specs:` entry is followed by an indented `Read:` line carrying a copy-pasteable git command for inspecting that spec's definition. The command is `git show <commits> -- '**/<file>'` where:
- `<commits>` is the space-joined list of the current plan's `spec_commits` (the deduplicated set of commit hashes from the domain's specifying phase). `git show` accepts the full list at once; commits that did not touch the spec file render nothing, so the pathspec self-filters the per-domain list down to the relevant commits.
- `<file>` is the spec entry with its `#anchor` stripped, wrapped in a `'**/<file>'` pathspec glob. The glob is required because spec entries are display-only names (with optional `#anchor`), not validated on-disk paths.

`git show` is used rather than `git log -p` so the output is bounded to exactly the named spec commits and does not walk unrelated history. When the current plan's `spec_commits` is empty, the `Read:` line is omitted (mirroring the planning phase's handling of empty spec commits) and the spec-review instruction in the action text reads "read the spec file(s) listed above" instead of referencing the git command.

**Review reminders.** Every IMPLEMENT action — first round and every subsequent round — includes a reminder to review the specs before implementing: "Please review the specification(s) above if you have not already done so — run the git command shown under each spec to read its definition." When the item has `Refs`, a second line is added: "Please review the reference file(s) under Refs if you have not already done so." Both lines are phrased as "if you have not already done so" because the item context is presented on every round and the engineer may have seen it before.

**First round (no prior eval):** Action says "Implement this item." Every advance out of IMPLEMENT auto-commits that item when `enable_commits: true`. `--message` is optional; the commit message is synthesized from the item's `description` field in plan.json, and any supplied `--message` text is appended to (not a replacement for) the synthesized message. See docs/auto-committing.md.

**Subsequent rounds (after eval FAIL, or PASS below min_rounds):** Applies to `eval_mode: "report"` and `eval_mode: "conversational"` — `eval_mode: "direct"` re-enters EVALUATE directly instead of IMPLEMENT (see EVALUATE Behavior) and so has no subsequent IMPLEMENT round. When `eval_mode: "report"`, action says "Study the eval file and implement any corrections." When `eval_mode: "conversational"`, action says "Make corrections based off communication with the evaluator." Every advance still auto-commits the item exactly as the first round does — every IMPLEMENT round commits, not just the first. The spec `Read:` lines and review reminders are present in every round.

### EVALUATE Behavior

Two actors:

**Sub-agent** runs `forgectl eval` to receive full item details, evaluator prompt, and (when `eval_mode: "report"`) report target path and previous eval history. When `eval_mode: "report"`, the sub-agent **must create the report file** at the printed path (using its file-writing tool) and return that path with the verdict — describing findings without writing the file is a contract violation. When `eval_mode: "direct"`, the sub-agent makes corrections directly to batch files. When `eval_mode: "conversational"`, the sub-agent communicates its verdict verbally.

**Engineer** reviews the report (when `eval_mode: "report"`), reviews unstaged changes from the evaluator (when `eval_mode: "direct"`), or receives the sub-agent's verbal verdict (when `eval_mode: "conversational"`). Runs `forgectl advance --verdict PASS|FAIL` (with `--eval-report <path>` when `eval_mode: "report"`). The `--eval-report` value is a **file path**; the engineer passes the exact path shown in the EVALUATE Action (identical to the path the sub-agent wrote). The engineer never passes report prose as the `--eval-report` value, and never invents a path the sub-agent did not write.

The report-mode handoff (deterministic path surfaced to both actors, sub-agent writes the file, engineer passes the path) is the shared **eval-report-contract**; see `specs/eval-report-contract.md`. This section is the fully worked example of that contract.

**`eval_mode: "direct"` retry.** IMPLEMENT runs exactly once per batch under `eval_mode: "direct"`. A FAIL with rounds below `max_rounds`, or a PASS with rounds below `min_rounds`, re-enters EVALUATE directly for another round — the sub-agent runs again and makes further direct corrections — rather than cycling back through IMPLEMENT. `eval_mode: "report"` and `"conversational"` are unaffected by this: FAIL or below-min-rounds PASS in those modes still re-enters IMPLEMENT, re-presenting every item in the batch (each of which auto-commits again on this pass, per IMPLEMENT Behavior).

**Terminal auto-commit.** The moment EVALUATE reaches a terminal outcome — PASS with rounds at or above `min_rounds`, or FAIL at `max_rounds` (force-accept) — and `enable_commits` is `true`, the scaffold commits the batch inline (message synthesized from the batch's item descriptions, per docs/auto-committing.md; skipped silently if nothing is staged) and transitions straight to ORIENT/DONE — the COMMIT state does not appear and no `--message` flag or additional advance is needed. When `enable_commits` is `false`, the transition instead goes to the COMMIT state, unchanged from today.

### COMMIT State

Appears only when `enable_commits` is `false`. It is a bookkeeping no-op hard stop after a batch reaches terminal evaluation — no git operation occurs. The engineer runs `forgectl advance` to proceed to ORIENT/DONE.

Appears after:
- EVALUATE with PASS + sufficient rounds
- EVALUATE with FAIL at max_rounds (force-accept)

When `enable_commits` is `true`, the COMMIT state is skipped entirely — the batch-terminal commit happens inline as part of the terminal EVALUATE transition (see EVALUATE Behavior above), and the scaffold proceeds directly to ORIENT/DONE.

---

## Invariants

1. **Layer ordering enforced.** All items in layer N must be terminal before layer N+1.
2. **Dependency ordering enforced.** Items only delivered when `depends_on` items are terminal.
3. **Item order preserved.** Items delivered in layer's `items` array order.
4. **One item at a time.** IMPLEMENT presents a single item per advance.
5. **plan.json is the progress record.** `passes` and `rounds` reflect current state.
6. **COMMIT precedes progression only when commits are disabled.** When `enable_commits` is `false`, every batch boundary passes through COMMIT before ORIENT/DONE. When `enable_commits` is `true`, COMMIT is skipped — the terminal EVALUATE transition commits inline and proceeds directly to ORIENT/DONE.
7. **Per-item commits, every round.** When `enable_commits` is `true`, every IMPLEMENT advance auto-commits that item — first round and every subsequent round alike; the "first round only" rule no longer applies. `--message` is optional at IMPLEMENT; the commit message is synthesized from the item's `description` field, with any supplied `--message` text appended rather than substituted. When `enable_commits` is `false`, `--message` is not required or shown at IMPLEMENT or COMMIT.
8. **Two actors, two commands.** Engineer uses `advance`; sub-agent uses `eval`.
9. **Scaffold does not parse eval files.** Verdict provided via `--verdict`; when `eval_mode: "report"`, file path stored as reference. When `eval_mode` is `"direct"` or `"conversational"`, no file path is stored.
10. **Min rounds enforced.** PASS below `implementing.eval.min_rounds` forces another evaluation cycle (another IMPLEMENT round under `"report"`/`"conversational"`; another EVALUATE round under `"direct"`).
11. **Max rounds enforced.** FAIL at `implementing.eval.max_rounds` forces acceptance.
12. **Direct-mode eval loop skips re-implementation.** When `eval_mode: "direct"`, IMPLEMENT runs exactly once per batch. A FAIL, or a PASS below `min_rounds`, re-enters EVALUATE directly rather than IMPLEMENT. `eval_mode: "report"` and `"conversational"` retain the IMPLEMENT re-entry on FAIL or below-min-rounds PASS.
13. **Guided pauses.** When `config.general.user_guided` is true, ORIENT output includes "STOP please review and discuss with user before continuing."
14. **Auto-commit at commit points.** When `enable_commits` is `true`, every IMPLEMENT advance auto-commits (message synthesized from the item's `description`, optional `--message` appended), and the terminal EVALUATE transition auto-commits the batch (message synthesized from the batch's item descriptions, optional `--message` appended) before proceeding directly to ORIENT/DONE — no separate COMMIT advance occurs. The scaffold runs `git add` with strategy-appropriate targets (per `implementing.commit_strategy`, default: `scoped`) to stage files, then runs `git commit -m <message>`. The `git add` step must precede `git commit` — committing without staging produces "no changes added to commit" and no commit is created; if nothing is staged, the commit is skipped silently. When `enable_commits` is `false`, `--message` is not shown in output at IMPLEMENT, and the COMMIT state remains a no-op advance; if `--message` is provided anywhere, a warning is printed: `--message is ignored, commits are not enabled`. The warning does not instruct how to enable commits. See `docs/auto-committing.md`.
15. **Spec `Read:` command is bounded.** When the current plan's `spec_commits` is non-empty, every `Specs:` entry in IMPLEMENT and `eval` output is followed by a `Read:` line of the form `git show <commits> -- '**/<file>'`, using `git show` (not `git log -p`) so the command resolves to exactly the named spec commits. When `spec_commits` is empty, no `Read:` line is emitted. This applies uniformly across `eval_mode: "report"`, `"direct"`, and `"conversational"` — the `Read:` line is part of the item body, not the eval-mode-specific handoff section.
16. **Spec review reminder always present.** Every IMPLEMENT action, on every round, includes the spec-review reminder. The Refs-review reminder is present if and only if the item has `Refs`.
17. **Report path surfaced to both actors.** When `eval_mode: "report"`, the scaffold computes the eval report path deterministically (`<plan-dir>/evals/batch-N-round-M.md`) and prints the *same* path in two places: the EVALUATE Action (for the engineer to pass to `--eval-report`) and the `--- REPORT OUTPUT ---` section of `forgectl eval` (for the sub-agent to write). The engineer never needs to invent or reconstruct the path. `--eval-report` is a file path argument; passing report prose is an error (caught by invariant 18).
18. **`--eval-report` value is validated as a path.** The scaffold stats the `--eval-report` value before recording it. If it is not an existing file, `advance` fails. When the value contains no path separator and looks like prose (whitespace, no `/`), the error additionally states that `--eval-report` expects the file path the eval sub-agent wrote, not the report text.

---

## Edge Cases

- **Scenario:** Layer has fewer items than `implementing.batch`.
  - **Expected:** Single batch contains all items.
  - **Rationale:** Batches are capped at `implementing.batch` but may be smaller. No padding or splitting occurs.

- **Scenario:** Batch has one item.
  - **Expected:** IMPLEMENT → EVALUATE directly.
  - **Rationale:** Single-item batches skip the multi-item advance loop; the item is marked `done` and evaluation begins.

- **Scenario:** EVALUATE PASS but rounds < min_rounds, `eval_mode` is `"report"` or `"conversational"`.
  - **Expected:** Re-enter IMPLEMENT. Every item still auto-commits (per-item commits are not gated on round number).
  - **Rationale:** Minimum evaluation rounds must be met regardless of verdict. The engineer gets another pass through the items, and each item's advance continues to commit.

- **Scenario:** EVALUATE PASS but rounds < min_rounds, `eval_mode: "direct"`.
  - **Expected:** Re-enter EVALUATE directly (no IMPLEMENT round).
  - **Rationale:** Under `"direct"`, IMPLEMENT runs exactly once per batch; the minimum-rounds requirement is satisfied by looping within EVALUATE.

- **Scenario:** EVALUATE FAIL at max_rounds, `enable_commits: false`.
  - **Expected:** Items `failed`. COMMIT. ORIENT.
  - **Rationale:** The maximum rounds are exhausted. Items are force-accepted as failed to prevent indefinite loops.

- **Scenario:** EVALUATE FAIL at max_rounds, `enable_commits: true`.
  - **Expected:** Items `failed`. Batch auto-commits inline (COMMIT does not render). ORIENT/DONE.
  - **Rationale:** Force-acceptance still needs to capture any accumulated corrections; the terminal transition itself performs that commit.

- **Scenario:** Item depends on a `failed` item.
  - **Expected:** Still unblocked — `failed` is terminal.
  - **Rationale:** `failed` is a terminal state just like `passed`. Dependent items proceed regardless of whether dependencies passed or failed.

- **Scenario:** All layers complete, no plans remaining, `enable_commits: false`.
  - **Expected:** COMMIT → DONE. Session complete.
  - **Rationale:** DONE with no remaining plans is the terminal state.

- **Scenario:** All layers complete, no plans remaining, `enable_commits: true`.
  - **Expected:** Terminal EVALUATE auto-commits inline → DONE directly (COMMIT skipped). Session complete.
  - **Rationale:** DONE with no remaining plans is the terminal state; the batch-terminal commit still occurs, just without a separate COMMIT advance.

- **Scenario:** All layers complete, `plan_all_before_implementing: false`, planning queue has plans remaining.
  - **Expected:** (`enable_commits: false`) COMMIT → DONE → PHASE_SHIFT (implementing → planning). (`enable_commits: true`) terminal EVALUATE auto-commits inline → DONE → PHASE_SHIFT (implementing → planning), with COMMIT skipped.
  - **Rationale:** Interleaved mode returns to planning for the next domain regardless of commit gating.

- **Scenario:** All layers complete, `plan_all_before_implementing: true`, implementing plan queue has plans remaining.
  - **Expected:** (`enable_commits: false`) COMMIT → DONE → PHASE_SHIFT (implementing → next implementation phase, next domain). (`enable_commits: true`) terminal EVALUATE auto-commits inline → DONE → PHASE_SHIFT, with COMMIT skipped. The next domain enters `implementing` or `ui_implementing` per its plan's `kind`.
  - **Rationale:** All-planning-first mode continues with the next domain's plan, routing by `kind`, regardless of commit gating.

- **Scenario:** `eval` called outside EVALUATE.
  - **Expected:** Error.
  - **Rationale:** Evaluation context is only meaningful in EVALUATE state; the sub-agent has nothing to evaluate otherwise.

---

## Testing Criteria

### ORIENT selects first batch
- **Verifies:** Batch selection from layer items.
- **Given:** ORIENT (implementing), L0 has 4 items, `implementing.batch: 2`.
- **When:** `advance`
- **Then:** State is IMPLEMENT. First item presented.

### IMPLEMENT presents items one at a time
- **Verifies:** Single-item presentation with batch progression.
- **Given:** IMPLEMENT, batch has 2 items, on item 1.
- **When:** `advance`
- **Then:** Item 1 `done`. Item 2 presented.

### IMPLEMENT last item → EVALUATE
- **Verifies:** Last item triggers evaluation.
- **Given:** IMPLEMENT, last item in batch.
- **When:** `advance`
- **Then:** Item `done`. Rounds incremented. State is EVALUATE.

### IMPLEMENT does not require --message on any round
- **Verifies:** `--message` is optional at IMPLEMENT regardless of round or `enable_commits`.
- **Given:** IMPLEMENT, first round and, separately, a subsequent round (after EVALUATE FAIL), `enable_commits: true`.
- **When:** `advance` without `--message`, in each case.
- **Then:** Advances. No error. The item commits with a synthesized message (see next criteria).

### IMPLEMENT auto-commits every round when enable_commits is true
- **Verifies:** Invariant 7 — per-item commits are not limited to the first round.
- **Given:** IMPLEMENT, `enable_commits: true`; first a first-round item, then (after an EVALUATE FAIL cycles back) the same item's subsequent round.
- **When:** `advance` (no `--message`) on each round.
- **Then:** Both advances produce a commit. Neither round is skipped for committing.

### IMPLEMENT commit message is synthesized from item description
- **Verifies:** Point 2 of the commit-message synthesis rule.
- **Given:** IMPLEMENT, `enable_commits: true`, item with `description: "Parse spectacular.yml, apply default host/port values."`.
- **When:** `advance` without `--message`.
- **Then:** The commit message is `"Parse spectacular.yml, apply default host/port values."`.

### IMPLEMENT appends supplied --message to the synthesized description
- **Verifies:** Point 2 — `--message` augments rather than replaces the synthesized message.
- **Given:** IMPLEMENT, `enable_commits: true`, item with `description: "Parse spectacular.yml, apply default host/port values."`.
- **When:** `advance --message "double-checked against staging config"`
- **Then:** The commit message contains both the item description and the supplied text, with the supplied text appended (not substituted).

### IMPLEMENT without enable_commits does not commit
- **Verifies:** No commit occurs when commits are disabled.
- **Given:** IMPLEMENT, first round (no prior eval), `enable_commits: false`.
- **When:** `advance`
- **Then:** Advances. No error. No commit produced.

### First-round IMPLEMENT stages files before committing
- **Verifies:** `git add` runs before `git commit` on first-round advance.
- **Given:** IMPLEMENT, first round, `enable_commits: true`, `commit_strategy: "scoped"`, domain `"api"`, item `description: "Implement config types"`.
- **When:** `advance` (no `--message`)
- **Then:** `git add api/` is run before `git commit -m "Implement config types"`. The commit succeeds. No error.

### Subsequent-round IMPLEMENT stages and commits per item
- **Verifies:** Point 1 — corrections rounds commit exactly like the first round.
- **Given:** IMPLEMENT, entered after EVALUATE (round 2+, `eval_mode: "report"` or `"conversational"`), `enable_commits: true`, `commit_strategy: "scoped"`, domain `"api"`, item `description: "Implement config load"`.
- **When:** `advance` (no `--message`)
- **Then:** `git add api/` is run before `git commit -m "Implement config load"`. The commit succeeds. No error.

### Terminal EVALUATE stages and commits the batch inline when enable_commits is true
- **Verifies:** Point 3 — the COMMIT state is skipped and the batch-terminal commit happens as part of the terminal EVALUATE `advance`.
- **Given:** EVALUATE, rounds >= `implementing.eval.min_rounds`, `enable_commits: true`, `commit_strategy: "scoped"`, domain `"api"`, batch items with descriptions `"Implement config types"` and `"Implement config load"`.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** `git add api/` runs before `git commit` with a message synthesized from both item descriptions. State transitions directly to ORIENT or DONE — COMMIT is never entered.

### Terminal EVALUATE commit is skipped silently when nothing is staged
- **Verifies:** Point 3 — the empty-commit rule applies to the batch-terminal auto-commit.
- **Given:** EVALUATE, rounds >= `implementing.eval.min_rounds`, `enable_commits: true`; every batch item already committed per-item with no further changes since.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** No commit is created. No error. State transitions directly to ORIENT or DONE.

### IMPLEMENT renders a Read command per spec when spec_commits exist
- **Verifies:** Spec `Read:` line is emitted with a bounded `git show` command.
- **Given:** IMPLEMENT, item with specs `["spec-sqlc-schemas.md#x"]`, current plan `spec_commits: ["e742a1b", "694ca99"]`.
- **When:** `advance`
- **Then:** Output contains `Read: git show e742a1b 694ca99 -- '**/spec-sqlc-schemas.md'` directly under the `Specs:` entry. The command uses `git show`, not `git log`.

### IMPLEMENT omits the Read command when spec_commits is empty
- **Verifies:** No `Read:` line without spec commits.
- **Given:** IMPLEMENT, item with specs, current plan `spec_commits: []`.
- **When:** `advance`
- **Then:** Output contains no `Read:` line. The spec-review reminder still appears, worded "read the spec file(s) listed above."

### IMPLEMENT renders one Read command per spec entry
- **Verifies:** Multi-spec items get one `Read:` line each, with anchors stripped.
- **Given:** IMPLEMENT, item with specs `["a.md#x", "b.md#y"]`, current plan `spec_commits: ["abc1234"]`.
- **When:** `advance`
- **Then:** Output contains `Read: git show abc1234 -- '**/a.md'` and `Read: git show abc1234 -- '**/b.md'`.

### eval renders a Read command per spec regardless of eval_mode
- **Verifies:** `forgectl eval` emits the same bounded `git show` Read command as IMPLEMENT, in every eval_mode.
- **Given:** EVALUATE (implementing), item with specs `["spec-sqlc-schemas.md#x"]`, current plan `spec_commits: ["e742a1b", "694ca99"]`, `eval_mode` in turn `"report"`, `"direct"`, `"conversational"`.
- **When:** `eval`
- **Then:** Output contains `Read: git show e742a1b 694ca99 -- '**/spec-sqlc-schemas.md'` directly under the `Specs:` entry, for all three eval_modes.

### eval omits the Read command when spec_commits is empty
- **Verifies:** No `Read:` line in eval output without spec commits.
- **Given:** EVALUATE (implementing), item with specs, current plan `spec_commits: []`.
- **When:** `eval`
- **Then:** Output contains no `Read:` line.

### IMPLEMENT review reminder present every round
- **Verifies:** Spec-review reminder appears on first and subsequent rounds.
- **Given:** IMPLEMENT entered after EVALUATE (round 2+).
- **When:** `advance`
- **Then:** Action text includes "Please review the specification(s) above if you have not already done so."

### IMPLEMENT Refs reminder gated on Refs presence
- **Verifies:** Refs-review reminder appears only when the item has Refs.
- **Given:** IMPLEMENT, one item with `refs` populated and one item with no `refs`.
- **When:** `advance` on each
- **Then:** The item with Refs shows "Please review the reference file(s) under Refs if you have not already done so"; the item without Refs omits that line.

### EVALUATE PASS with sufficient rounds → COMMIT (enable_commits: false)
- **Verifies:** PASS with sufficient rounds marks items passed and enters COMMIT when commits are disabled.
- **Given:** EVALUATE, rounds >= `implementing.eval.min_rounds`, `enable_commits: false`.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** Items `passed`. State is COMMIT.

### EVALUATE PASS with sufficient rounds → ORIENT/DONE directly (enable_commits: true)
- **Verifies:** PASS with sufficient rounds skips COMMIT and auto-commits inline when commits are enabled.
- **Given:** EVALUATE, rounds >= `implementing.eval.min_rounds`, `enable_commits: true`.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** Items `passed`. Batch auto-commits. State is ORIENT or DONE — never COMMIT.

### EVALUATE FAIL at max_rounds → COMMIT (enable_commits: false)
- **Verifies:** FAIL at max rounds marks items failed and enters COMMIT when commits are disabled.
- **Given:** EVALUATE, rounds == `implementing.eval.max_rounds`, `enable_commits: false`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** Items `failed`. State is COMMIT.

### EVALUATE FAIL at max_rounds → ORIENT/DONE directly (enable_commits: true)
- **Verifies:** Force-accept still auto-commits inline and skips COMMIT when commits are enabled.
- **Given:** EVALUATE, rounds == `implementing.eval.max_rounds`, `enable_commits: true`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** Items `failed`. Batch auto-commits. State is ORIENT or DONE — never COMMIT.

### EVALUATE FAIL within max_rounds → IMPLEMENT (report/conversational)
- **Verifies:** FAIL within max rounds triggers re-implementation for `"report"` and `"conversational"` eval_modes.
- **Given:** EVALUATE, rounds < `implementing.eval.max_rounds`, `eval_mode` is `"report"` or `"conversational"`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is IMPLEMENT. First item with eval file.

### EVALUATE FAIL within max_rounds → EVALUATE (direct)
- **Verifies:** Invariant 12 — `eval_mode: "direct"` re-evaluates directly instead of re-implementing.
- **Given:** EVALUATE, rounds < `implementing.eval.max_rounds`, `eval_mode: "direct"`.
- **When:** `advance --verdict FAIL`
- **Then:** State is EVALUATE. Round incremented. No IMPLEMENT state is presented.

### COMMIT → ORIENT (more items, enable_commits: false)
- **Verifies:** Batch completion returns to ORIENT for next batch when the COMMIT state is in use.
- **Given:** COMMIT, `enable_commits: false`, more items in layer.
- **When:** `advance`
- **Then:** State is ORIENT.

### COMMIT → DONE (all complete, enable_commits: false)
- **Verifies:** Final batch completion reaches terminal state when the COMMIT state is in use.
- **Given:** COMMIT, `enable_commits: false`, all layers complete.
- **When:** `advance`
- **Then:** State is DONE.

### Implementing eval command outputs item details
- **Verifies:** Eval command assembles full evaluation context.
- **Given:** EVALUATE (implementing), batch has 2 items, `eval_mode: "report"`.
- **When:** `forgectl eval`
- **Then:** Output includes impl-eval.md contents, item details, report target path.

### Implementing eval command outputs direct correction prompt when eval_mode is direct
- **Verifies:** Report section instructs direct corrections when eval_mode is direct.
- **Given:** EVALUATE (implementing), batch has 2 items, `eval_mode: "direct"`.
- **When:** `forgectl eval`
- **Then:** Output includes impl-eval.md contents, item details, and "Make corrections directly to the batch files."

### Implementing eval command omits report target when eval_mode is conversational
- **Verifies:** No report path when eval_mode is conversational.
- **Given:** EVALUATE (implementing), batch has 2 items, `eval_mode: "conversational"`.
- **When:** `forgectl eval`
- **Then:** Output includes impl-eval.md contents, item details. No report target path.

### Direct-mode batch never returns to IMPLEMENT after round 1
- **Verifies:** Invariant 12 across a full FAIL → FAIL → PASS cycle.
- **Given:** ORIENT, `eval_mode: "direct"`, batch of 2 items, `implementing.eval.max_rounds: 3`.
- **When:** The batch is driven: IMPLEMENT both items → EVALUATE round 1 (`--verdict FAIL`) → EVALUATE round 2 (`--verdict FAIL`) → EVALUATE round 3 (`--verdict PASS`).
- **Then:** IMPLEMENT is entered exactly once for the batch. All three EVALUATE rounds occur consecutively with no intervening IMPLEMENT state.

### Failed items don't block dependents
- **Verifies:** Failed items are terminal for dependency resolution.
- **Given:** Item A `failed`, item B depends on A.
- **Then:** B is unblocked.

### DONE with no plans remaining is terminal
- **Verifies:** Terminal state when all plans implemented.
- **Given:** DONE, no plans remaining (neither planning queue nor implementing plan queue).
- **When:** `advance`
- **Then:** Error: "session complete."

### DONE transitions to planning when interleaved
- **Verifies:** DONE returns to planning in interleaved mode.
- **Given:** DONE, `plan_all_before_implementing: false`, planning queue has 1 plan.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered. After advancing: `phase: "planning"`, `state: "ORIENT"`.

### DONE transitions to next domain when all-planning-first
- **Verifies:** DONE continues implementing next domain.
- **Given:** DONE, `plan_all_before_implementing: true`, implementing plan queue has 1 plan.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered. After advancing: `state: "ORIENT"`. Next plan loaded.

---

## Implements
- Implementing phase: layer-ordered batched item delivery with one-at-a-time presentation
- Batch size controlled by `implementing.batch`
- Eval round enforcement (`implementing.eval.min_rounds`/`max_rounds`) with forced acceptance
- `eval_mode: "direct"` runs IMPLEMENT exactly once per batch; FAIL/below-min-rounds PASS re-enters EVALUATE directly rather than IMPLEMENT
- Per-item auto-commit at every IMPLEMENT round, message synthesized from the item's `description` (optional `--message` appended)
- COMMIT state for batch boundary pauses when `enable_commits: false`; skipped in favor of an inline batch-terminal auto-commit at the terminal EVALUATE transition when `enable_commits: true`
- Commit gating via `enable_commits` configuration
- Domain artifacts in `.forgectl_workspace/`
- Dual evaluator prompts: impl-eval.md for implementation sub-agent
