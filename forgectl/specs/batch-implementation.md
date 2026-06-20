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
| phase-transitions | DONE → PHASE_SHIFT when plans remain (implementing → planning when `plan_all_before_implementing: false`, implementing → implementing when `true`) |

---

## Interface

### Inputs

#### `advance` flags — Implementing Phase

| State | Flags |
|-------|-------|
| IMPLEMENT | `--message <text>` (required first round only, when `enable_commits: true`) |
| EVALUATE | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `eval_mode: "report"`) |
| COMMIT | `--message <text>` (required when `enable_commits: true`) |

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

**Entering IMPLEMENT** (first item in batch, after eval — round 2+, `eval_mode: "direct"`):

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
Action:  Review unstaged changes from the evaluator (git diff).
         Accept, revise, or revert corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Please review the specification(s) above if you have not already done so —
         run the git command shown under each spec to read its definition.
         Please review the reference file(s) under Refs if you have not already done so.
         After completion of the above, advance to continue.
```

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
Action:   Please spawn 1 opus sub-agent to evaluate the implementation batch.
          The sub-agent should run: forgectl eval
          After completion of the above, advance with --eval-report <path> --verdict PASS|FAIL
```

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
Action:   Please spawn 1 opus sub-agent to evaluate and correct the batch.
          Sub-agent runs: forgectl eval
          Batch files have been staged. Sub-agent makes corrections directly.
          After completion of the above, advance with --verdict PASS|FAIL
```

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
Action:   Please spawn 1 opus sub-agent to evaluate the implementation batch.
          The sub-agent should run: forgectl eval
          After completion of the above, advance with --verdict PASS|FAIL
```

**Entering COMMIT** (after EVALUATE, batch terminal, `enable_commits: true`):

```
State:   COMMIT
Phase:   implementing
Layer:   L0 Foundation
Batch:   1/2
Items:
  - [config.types] passed
  - [config.load] passed
Action:  Advance with --message "your commit message" to commit and continue.
```

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

**Entering COMMIT** (after force-accept, `enable_commits: true`):

```
State:   COMMIT
Phase:   implementing
Layer:   L1 Core
Batch:   3/3
Items:
  - [daemon.types] failed (force-accept, 3/3 rounds)
  - [daemon.io] failed (force-accept, 3/3 rounds)
Action:  Advance with --message "your commit message" to commit and continue.
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

**Entering ORIENT** (after COMMIT, more items in layer):

```
State:    ORIENT
Phase:    implementing
Layer:    L0 Foundation
Progress: 2/4 items passed
Next:     2 unblocked items in next batch
Action:   STOP please review and discuss with user before continuing.
          After completion of the above, advance to select next batch.
```

**Entering ORIENT** (after COMMIT, layer complete, more layers):

```
State:    ORIENT
Phase:    implementing
Layer:    L0 Foundation
Progress: 3/3 items passed — layer complete
Next:     L1 Core — 2 items: [daemon.types], [daemon.io]
Action:   STOP please review and discuss with user before continuing.
          After completion of the above, advance to next layer.
```

**Entering ORIENT** (after COMMIT, layer complete, last layer):

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
                  config-validation.md#behavior-strict-mode
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

Write your evaluation report to:
  launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md
```

Subsequent rounds with `eval_mode: "report"` include previous evaluations:

```
=== IMPLEMENTATION EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: PASS — launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-1.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.forgectl_workspace/implementation_plan/evals/batch-1-round-2.md
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
| `advance` in implementing IMPLEMENT (first round) without `--message` when `enable_commits: true` | Error. Exit code 1. | First-round items need a commit message when commits are enabled |
| `advance` in implementing COMMIT without `--message` when `enable_commits: true` | Error. Exit code 1. | Batch completion needs a commit message when commits are enabled |
| `advance` in implementing EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in implementing EVALUATE without `--eval-report` when `eval_mode: "report"` | Error. Exit code 1. | Every evaluation must reference its report when eval output is enabled |
| `advance --eval-report` pointing to non-existent file | Error naming the path. Exit code 1. | Report must exist to be recorded |
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

### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | unblocked items exist in current layer | IMPLEMENT | Select batch. Present first item. |
| ORIENT | all layer items terminal, more layers | ORIENT (next layer) | Advance `current_layer`. |
| ORIENT | all layers complete | DONE | — |
| IMPLEMENT | more items in batch | IMPLEMENT | Mark current item `done`. Present next item. |
| IMPLEMENT | last item in batch | EVALUATE | Mark current item `done`. Increment `rounds` on all batch items. |
| EVALUATE | PASS, rounds >= `implementing.eval.min_rounds` | COMMIT | Mark items `passed`. Record eval. |
| EVALUATE | PASS, rounds < `implementing.eval.min_rounds` | IMPLEMENT | Record eval. Re-present first item with eval file. |
| EVALUATE | FAIL, rounds < `implementing.eval.max_rounds` | IMPLEMENT | Record eval. Re-present first item with eval file. |
| EVALUATE | FAIL, rounds >= `implementing.eval.max_rounds` | COMMIT | Mark items `failed`. Record eval. Force-accept. |
| COMMIT | more batches or layers | ORIENT | — |
| COMMIT | all layers complete | DONE | — |
| DONE | `plan_all_before_implementing: false`, planning queue non-empty | PHASE_SHIFT | PHASE_SHIFT (implementing → planning). Return to planning for next domain. |
| DONE | `plan_all_before_implementing: true`, implementing plan queue non-empty | PHASE_SHIFT | PHASE_SHIFT (implementing → implementing, domain boundary). Pull next plan. |
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

**First round (no prior eval):** Action says "Implement this item." Advance requires `--message` — the scaffold commits after each item.

**Subsequent rounds (after eval):** When `eval_mode: "report"`, action says "Study the eval file and implement any corrections." When `eval_mode: "direct"`, action says "Review unstaged changes from the evaluator (git diff)." When `eval_mode: "conversational"`, action says "Make corrections based off communication with the evaluator." No `--message` required — corrections are committed at the COMMIT state after the batch passes. The spec `Read:` lines and review reminders are present in every round.

### EVALUATE Behavior

Two actors:

**Sub-agent** runs `forgectl eval` to receive full item details, evaluator prompt, and (when `eval_mode: "report"`) report target path and previous eval history. When `eval_mode: "direct"`, the sub-agent makes corrections directly to batch files. When `eval_mode: "conversational"`, the sub-agent communicates its verdict verbally.

**Engineer** reviews the report (when `eval_mode: "report"`), reviews unstaged changes from the evaluator (when `eval_mode: "direct"`), or receives the sub-agent's verbal verdict (when `eval_mode: "conversational"`). Runs `forgectl advance --verdict PASS|FAIL` (with `--eval-report <path>` when `eval_mode: "report"`).

### COMMIT State

Hard stop after a batch reaches terminal evaluation. Ensures all implementation work is committed before proceeding.

Appears after:
- EVALUATE with PASS + sufficient rounds
- EVALUATE with FAIL at max_rounds (force-accept)

When `enable_commits` is `true`, the engineer runs `forgectl advance --message <commit msg>` to commit and proceed. When `false`, the engineer runs `forgectl advance` to proceed.

---

## Invariants

1. **Layer ordering enforced.** All items in layer N must be terminal before layer N+1.
2. **Dependency ordering enforced.** Items only delivered when `depends_on` items are terminal.
3. **Item order preserved.** Items delivered in layer's `items` array order.
4. **One item at a time.** IMPLEMENT presents a single item per advance.
5. **plan.json is the progress record.** `passes` and `rounds` reflect current state.
6. **COMMIT precedes progression.** Every batch boundary passes through COMMIT before ORIENT/DONE.
7. **First-round commits.** When `enable_commits` is `true`, IMPLEMENT advance requires `--message` and commits on the first round only. When `enable_commits` is `false`, `--message` is not required at IMPLEMENT or COMMIT.
8. **Two actors, two commands.** Engineer uses `advance`; sub-agent uses `eval`.
9. **Scaffold does not parse eval files.** Verdict provided via `--verdict`; when `eval_mode: "report"`, file path stored as reference. When `eval_mode` is `"direct"` or `"conversational"`, no file path is stored.
10. **Min rounds enforced.** PASS below `implementing.eval.min_rounds` forces another implementation cycle.
11. **Max rounds enforced.** FAIL at `implementing.eval.max_rounds` forces acceptance.
12. **Guided pauses.** When `config.general.user_guided` is true, ORIENT output includes "STOP please review and discuss with user before continuing."
13. **Auto-commit at commit points.** When `enable_commits` is `true`, `--message` is required at IMPLEMENT (first round) and COMMIT states. The scaffold stages files per `implementing.commit_strategy` (default: `scoped`) and runs `git commit -m <message>`. When `enable_commits` is `false`, `--message` is not shown in output; if provided, a warning is printed: `--message is ignored, commits are not enabled`. The warning does not instruct how to enable commits. See `docs/auto-committing.md`.
14. **Spec `Read:` command is bounded.** When the current plan's `spec_commits` is non-empty, every `Specs:` entry in IMPLEMENT output is followed by a `Read:` line of the form `git show <commits> -- '**/<file>'`, using `git show` (not `git log -p`) so the command resolves to exactly the named spec commits. When `spec_commits` is empty, no `Read:` line is emitted.
15. **Spec review reminder always present.** Every IMPLEMENT action, on every round, includes the spec-review reminder. The Refs-review reminder is present if and only if the item has `Refs`.

---

## Edge Cases

- **Scenario:** Layer has fewer items than `implementing.batch`.
  - **Expected:** Single batch contains all items.
  - **Rationale:** Batches are capped at `implementing.batch` but may be smaller. No padding or splitting occurs.

- **Scenario:** Batch has one item.
  - **Expected:** IMPLEMENT → EVALUATE directly.
  - **Rationale:** Single-item batches skip the multi-item advance loop; the item is marked `done` and evaluation begins.

- **Scenario:** EVALUATE PASS but rounds < min_rounds.
  - **Expected:** Re-enter IMPLEMENT. No commit reminder (not first round).
  - **Rationale:** Minimum evaluation rounds must be met regardless of verdict. The engineer gets another pass through the items.

- **Scenario:** EVALUATE FAIL at max_rounds.
  - **Expected:** Items `failed`. COMMIT. ORIENT.
  - **Rationale:** The maximum rounds are exhausted. Items are force-accepted as failed to prevent indefinite loops.

- **Scenario:** Item depends on a `failed` item.
  - **Expected:** Still unblocked — `failed` is terminal.
  - **Rationale:** `failed` is a terminal state just like `passed`. Dependent items proceed regardless of whether dependencies passed or failed.

- **Scenario:** All layers complete, no plans remaining.
  - **Expected:** COMMIT → DONE. Session complete.
  - **Rationale:** DONE with no remaining plans is the terminal state.

- **Scenario:** All layers complete, `plan_all_before_implementing: false`, planning queue has plans remaining.
  - **Expected:** COMMIT → DONE → PHASE_SHIFT (implementing → planning).
  - **Rationale:** Interleaved mode returns to planning for the next domain.

- **Scenario:** All layers complete, `plan_all_before_implementing: true`, implementing plan queue has plans remaining.
  - **Expected:** COMMIT → DONE → PHASE_SHIFT (implementing → implementing, next domain).
  - **Rationale:** All-planning-first mode continues implementing the next domain's plan.

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
- **When:** `advance --message "Implement config types"`
- **Then:** Item 1 `done`. Item 2 presented.

### IMPLEMENT last item → EVALUATE
- **Verifies:** Last item triggers evaluation.
- **Given:** IMPLEMENT, last item in batch.
- **When:** `advance --message "Implement config load"`
- **Then:** Item `done`. Rounds incremented. State is EVALUATE.

### First-round IMPLEMENT requires --message when enable_commits is true
- **Verifies:** Commit message required on first round when commits enabled.
- **Given:** IMPLEMENT, first round (no prior eval), `enable_commits: true`.
- **When:** `advance` without `--message`
- **Then:** Exit code 1.

### First-round IMPLEMENT without --message when enable_commits is false
- **Verifies:** No commit message required when commits disabled.
- **Given:** IMPLEMENT, first round (no prior eval), `enable_commits: false`.
- **When:** `advance`
- **Then:** Advances. No error.

### Subsequent-round IMPLEMENT does not require --message
- **Verifies:** No commit on subsequent rounds.
- **Given:** IMPLEMENT, entered after EVALUATE (round 2+).
- **When:** `advance`
- **Then:** Advances without committing. No error.

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

### EVALUATE PASS with sufficient rounds → COMMIT
- **Verifies:** PASS with sufficient rounds marks items passed.
- **Given:** EVALUATE, rounds >= `implementing.eval.min_rounds`.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** Items `passed`. State is COMMIT.

### EVALUATE FAIL at max_rounds → COMMIT
- **Verifies:** FAIL at max rounds marks items failed.
- **Given:** EVALUATE, rounds == `implementing.eval.max_rounds`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** Items `failed`. State is COMMIT.

### EVALUATE FAIL within max_rounds → IMPLEMENT
- **Verifies:** FAIL within max rounds triggers re-implementation.
- **Given:** EVALUATE, rounds < `implementing.eval.max_rounds`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is IMPLEMENT. First item with eval file.

### COMMIT → ORIENT (more items)
- **Verifies:** Batch completion returns to ORIENT for next batch.
- **Given:** COMMIT, more items in layer.
- **When:** `advance`
- **Then:** State is ORIENT.

### COMMIT → DONE (all complete)
- **Verifies:** Final batch completion reaches terminal state.
- **Given:** COMMIT, all layers complete.
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
- COMMIT state for batch boundary pauses
- Commit gating via `enable_commits` configuration
- Domain artifacts in `.forgectl_workspace/`
- Dual evaluator prompts: impl-eval.md for implementation sub-agent
