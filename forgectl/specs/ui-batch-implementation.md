# UI Batch Implementation

## Topic of Concern
> The scaffold implements UI plan items in dependency-ordered batches, then verifies each batch through a QA placement loop and an end-to-end test loop before committing.

## Context

The `ui_implementing` phase delivers UI plan items one at a time within dependency-ordered batches, exactly as the implementing phase does, and then subjects each completed batch to two additional verification loops that the implementing phase does not have:

1. A **QA placement loop** in which a QA sub-agent drives the running application through the Playwright MCP — navigating to the app URL, reading the accessibility snapshot to judge placement, exercising controls, and reading the console for runtime errors — judges UI placement and controls, and emits a list of end-to-end test scenarios.
2. An **end-to-end (e2e) loop** in which those scenarios are authored into runnable Playwright test files, executed by the Playwright test runner, and verified — with a remediation pass whenever the verification fails.

The QA loop's browser-automation mechanism is **Playwright MCP**; the e2e loop's authored tests are **Playwright test files** run by a Playwright test runner. The scaffold surfaces the QA driver in its rendered output (it does not configure Playwright itself), and surfaces the e2e runner through the configured test command — see Configuration.

The phase reuses the implementing phase's spine — layers, dependency-ordered batches, one-item-at-a-time `IMPLEMENT`, the code `EVALUATE` loop, and the `COMMIT` batch boundary. After the code `EVALUATE` loop reaches a terminal verdict for a batch, the batch passes through the QA loop and then the e2e loop before reaching `COMMIT`. Each of the three loops (code eval, QA, e2e) carries its own round budget and its own force-accept boundary, so they can be tuned independently.

Items progress through `pending` → `done` → `passed`/`failed` states tracked in plan.json. An item's terminal status reflects all three loops: it is `passed` only when the code eval, QA, and e2e loops all reached PASS within budget, and `failed` when any of the three force-accepted at its maximum rounds.

## Depends On
- **phase-transitions** — entry into `ui_implementing` (init or phase shift) validates plan.json, adds tracking fields, and triggers ORIENT; DONE triggers the phase shift out.
- **session-init** — `init --phase ui_implementing --from <plan.json>` starts the phase directly and validates the UI-specific configuration.
- **batch-implementation** — the implementing phase defines the shared batch spine (layers, batch selection, IMPLEMENT, code EVALUATE, COMMIT) that this phase extends.
- **state-persistence** — reads and writes the state file.

## Integration Points

| Adjacent concern | Relationship |
|------------------|-------------|
| plan.json | Planning produces it; `ui_implementing` consumes and mutates it (adding `passes` and `rounds` fields), identically to the implementing phase |
| batch-implementation | Shares the layer/batch selection algorithm, the IMPLEMENT item loop, the code EVALUATE loop, and the COMMIT batch boundary; `ui_implementing` inserts the QA and e2e loops between code EVALUATE and COMMIT |
| Implementation evaluator prompt (`evaluators/impl-eval.md`, embedded in binary) | Drives the code EVALUATE sub-agent — same prompt and same role as in the implementing phase |
| QA evaluator prompt (`evaluators/ui-qa-eval.md`, embedded in binary) | Drives the QA_TEST sub-agent: how to exercise the running UI through the Playwright MCP, what to judge about placement and controls, the QA report format, the verdict rules, and the e2e scenario step-list format and target path |
| E2E evaluator prompt (`evaluators/ui-e2e-eval.md`, embedded in binary) | Drives the E2E_VERIFY sub-agent: how to confirm the authored Playwright tests pass, that they cover the QA step list, the report format, and the verdict rules |
| Playwright MCP | The browser-automation interface the QA_TEST sub-agent drives — navigate, snapshot, click/type, console — to exercise and judge the running app. Reached via `ui_implementing.app.url`; not configured by the scaffold |
| `handoff` command | The sub-agent registers the files it generated (QA report, step list, e2e report) so the scaffold can surface them for engineer review before the verdict is recorded |
| QA step list (`<domain>/.forgectl_workspace/ui_plan/qa/batch-N-steps.json`) | Produced by QA_TEST, consumed by E2E_AUTHOR — the contract between the two loops |
| phase-transitions | Entered from planning when the plan's `kind` is `ui`; DONE → PHASE_SHIFT when plans remain (`ui_implementing` → planning when `plan_all_before_implementing: false`; `ui_implementing` → the next domain's implementation phase, routed by that plan's `kind`, at a domain boundary when `true`) |

---

## Data Models

### QA Step List

A JSON file produced by QA_TEST and consumed by E2E_AUTHOR. It records the end-to-end scenarios the QA sub-agent derived from exercising the batch's UI. It is rewritten on every QA_TEST round and is independent of `eval_mode` (it is always produced, even in `conversational` mode).

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| `batch` | integer | yes | no | Batch number the scenarios belong to |
| `round` | integer | yes | no | The QA round that produced this list |
| `scenarios` | array of QA Scenario | yes | no | May be empty when the batch has no e2e-worthy flow |

### QA Scenario

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| `id` | string | yes | no | Stable identifier for the scenario, unique within the list |
| `name` | string | yes | no | Human-readable scenario title |
| `preconditions` | array of string | no | no | Application state assumed before the steps run |
| `steps` | array of string | yes | no | Ordered user actions to perform, one action per entry |
| `expected` | array of string | yes | no | Observable results that must hold after the steps |
| `priority` | string | no | no | One of `critical`, `normal`, `low`; absent means `normal` |

---

## Interface

### Inputs

#### `advance` flags — `ui_implementing` Phase

| State | Flags |
|-------|-------|
| IMPLEMENT | `--message <text>` (required first round only, when `enable_commits: true`) |
| EVALUATE | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `ui_implementing.eval.eval_mode: "report"`) |
| UI_REFINE | (no flags) |
| QA_TEST | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `ui_implementing.qa.eval_mode: "report"`) |
| E2E_AUTHOR | (no flags) |
| E2E_REMEDIATE | (no flags) |
| E2E_VERIFY | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `ui_implementing.e2e.eval_mode: "report"`) |
| COMMIT | `--message <text>` (required when `enable_commits: true`) |

The `--eval-report` flag is reused across all three evaluator states (EVALUATE, QA_TEST, E2E_VERIFY). The report path differs by state, but the flag is the same. No flags are added beyond those the implementing phase already defines.

#### `eval` command

| Command | Flags | Description |
|---------|-------|-------------|
| `eval` | none | Output full evaluation context for the current sub-agent. Valid only in `ui_implementing` EVALUATE, QA_TEST, and E2E_VERIFY. |

#### `handoff` command

| Command | Arguments | Description |
|---------|-----------|-------------|
| `handoff` | `<file> [<file>…]` (one or more) | The sub-agent's artifact-return command. Registers each generated file (QA report, QA step list, e2e report) with the scaffold as a handed-off artifact for the current evaluator round. Valid only in `ui_implementing` EVALUATE, QA_TEST, and E2E_VERIFY — the same states as `eval`. |

`handoff` is the inverse of `eval`: `eval` hands evaluation context *in* to the sub-agent; `handoff` hands the sub-agent's outputs *back* to the scaffold. Once artifacts are registered, the evaluator state's `status` and `advance` output gains a `Review:` line listing them, so the engineer reviews the agent-provided files before recording a verdict. `handoff` carries no verdict — the verdict is still recorded by the engineer via `advance --verdict`. It **coexists with `--eval-report`**: hand-off drives the `Review:` prompt and enforces artifact presence; `--eval-report` independently records the verdict's backing report in `report` mode. The scaffold verifies each handed-off file exists; a missing file is rejected.

### Outputs

#### `advance` output

**Entering ORIENT** (after entry or after COMMIT, batch available; shown with `config.general.user_guided: true`, which adds the STOP line):

```
State:   ORIENT
Phase:   ui_implementing
Plan:    Portal Dashboard
Domain:  portal
File:    portal/.forgectl_workspace/ui_plan/plan.json
Config:  ui_implementing.batch=2, eval.rounds=1-3, qa.rounds=1-3, e2e.rounds=1-3
App:     launch="npm run dev" url=http://localhost:5173

Initialized plan.json for UI implementation:
  Items:  4 (passes: pending, rounds: 0)
  Layers: 2 (L0 Shell: 2 items, L1 Views: 2 items)

Action:  STOP please review and discuss with user before continuing.
         After completion of the above, advance to select first batch.
```

**Entering IMPLEMENT** (first item in batch, first round — no prior eval):

```
State:   IMPLEMENT
Phase:   ui_implementing
Layer:   L0 Shell
Batch:   1/2
Item:    [shell.layout] App shell and navigation frame
         Top bar, sidebar, and routed content region.
         (1 of 2 in batch)
Steps:
  1. Build the app shell with a top bar and collapsible sidebar
  2. Add the routed content region
  3. Wire navigation links to routes
Files:   src/shell/AppShell.tsx
Specs:   app-shell.md#interface-outputs
Refs:    notes/shell.md#layout
Tests:   1 functional
Action:  Implement this item.
         After completion of the above, advance to continue.
```

The IMPLEMENT output for subsequent items in the batch, and for rounds after a code EVALUATE (under each `eval_mode`), is identical in form to the implementing phase's IMPLEMENT output; see batch-implementation. The only difference is the `Phase:` line reads `ui_implementing`.

**Entering EVALUATE** (code-level evaluation, `eval_mode: "report"`):

```
State:    EVALUATE
Phase:    ui_implementing
Layer:    L0 Shell
Batch:    1/2
Round:    1/3
Loop:     code
Items:
  - [shell.layout] App shell and navigation frame
  - [shell.theme] Theme tokens and color modes
Action:   Please spawn 1 opus sub-agent to evaluate the implementation batch.
          The sub-agent should run: forgectl eval
          After completion of the above, advance with --eval-report <path> --verdict PASS|FAIL
```

The EVALUATE output under `eval_mode: "direct"` and `eval_mode: "conversational"` matches the implementing phase's EVALUATE output, with the added `Loop: code` line.

**Entering QA_TEST** (`qa.eval_mode: "report"`):

```
State:    QA_TEST
Phase:    ui_implementing
Layer:    L0 Shell
Batch:    1/2
Round:    1/3
Loop:     qa
App:      launch="npm run dev" url=http://localhost:5173
Items:
  - [shell.layout] App shell and navigation frame
  - [shell.theme] Theme tokens and color modes
Steps:    portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json
Action:   Please spawn 1 opus sub-agent to QA the running UI.
          The sub-agent should run: forgectl eval
          It drives the running app at the URL above through the Playwright MCP —
          snapshot to judge placement, exercise each control, read the console for errors —
          judges UI placement and controls, writes the e2e step list to the Steps path above,
          and hands its outputs back with: forgectl handoff <qa-report> <step-list>
          After completion of the above, advance with --eval-report <path> --verdict PASS|FAIL
```

Once the sub-agent has run `forgectl handoff`, a re-render of the QA_TEST state (via `status` or a stay-in-state `advance`) adds a `Review:` line so the engineer reviews the agent-provided files before the verdict:

```
Review:   portal/.forgectl_workspace/ui_plan/qa/batch-1-round-1.md (QA report)
          portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json (step list, 3 scenarios)
Action:   Review the file(s) provided by the agent above, then
          advance with --eval-report <path> --verdict PASS|FAIL
```

Under `qa.eval_mode: "direct"`, the action reads "Please spawn 1 opus sub-agent to QA and correct the UI. The sub-agent makes placement corrections directly, writes the e2e step list, and hands off the step list." and the verdict is recorded with `--verdict` only. Under `qa.eval_mode: "conversational"`, the report path is omitted, the sub-agent communicates its verdict verbally, but the `Steps:` path is still shown — and the step list is still handed off — because the step list is always written. In every mode the sub-agent hands off at least the step list.

**Entering UI_REFINE** (`qa.eval_mode: "report"`):

```
State:   UI_REFINE
Phase:   ui_implementing
Layer:   L0 Shell
Batch:   1/2
Round:   1/3
Loop:    qa
QA:      portal/.forgectl_workspace/ui_plan/qa/batch-1-round-1.md
Note:    FAIL recorded for QA round 1.
Items:
  - [shell.layout] App shell and navigation frame
  - [shell.theme] Theme tokens and color modes
Action:  Study the QA report "portal/.forgectl_workspace/ui_plan/qa/batch-1-round-1.md"
         and iterate on UI placement and controls as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work.
         After completion of the above, advance to continue.
```

Under `qa.eval_mode: "direct"`, the action reads "Review unstaged changes from the QA evaluator (git diff). Accept, revise, or revert the placement corrections." Under `qa.eval_mode: "conversational"`, the action reads "Make placement and control corrections based off communication with the QA evaluator."

**Entering E2E_AUTHOR**:

```
State:   E2E_AUTHOR
Phase:   ui_implementing
Layer:   L0 Shell
Batch:   1/2
Loop:    e2e
Steps:   portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json (3 scenarios)
Tests:   e2e/
Run:     npm run e2e
Action:  Author e2e tests from the step list above into the Tests directory, then run them.
         After completion of the above, advance to continue.
```

When the step list has zero scenarios:

```
State:   E2E_AUTHOR
Phase:   ui_implementing
Layer:   L0 Shell
Batch:   1/2
Loop:    e2e
Steps:   portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json (0 scenarios)
Action:  No e2e scenarios were produced for this batch. Advance to continue.
```

**Entering E2E_VERIFY** (`e2e.eval_mode: "report"`):

```
State:    E2E_VERIFY
Phase:    ui_implementing
Layer:    L0 Shell
Batch:    1/2
Round:    1/3
Loop:     e2e
Run:      npm run e2e
Items:
  - [shell.layout] App shell and navigation frame
  - [shell.theme] Theme tokens and color modes
Action:   Please spawn 1 opus sub-agent to verify the e2e tests.
          The sub-agent should run: forgectl eval
          After completion of the above, advance with --eval-report <path> --verdict PASS|FAIL
```

Under `e2e.eval_mode: "direct"` and `e2e.eval_mode: "conversational"`, the action mirrors the implementing phase's EVALUATE output for those modes.

**Entering E2E_REMEDIATE** (`e2e.eval_mode: "report"`):

```
State:   E2E_REMEDIATE
Phase:   ui_implementing
Layer:   L0 Shell
Batch:   1/2
Round:   1/3
Loop:    e2e
Eval:    portal/.forgectl_workspace/ui_plan/e2e/batch-1-round-1.md
Note:    FAIL recorded for e2e round 1.
Run:     npm run e2e
Action:  Study the e2e report above. Fix the failing tests or the UI under test, then re-run.
         Apply "fresh" eyes and a tightened lens when reviewing the work.
         After completion of the above, advance to continue.
```

Under `e2e.eval_mode: "direct"`, the action reads "Review unstaged changes from the e2e evaluator (git diff). Accept, revise, or revert, then re-run." Under `e2e.eval_mode: "conversational"`, the action reads "Make corrections based off communication with the e2e evaluator, then re-run."

**Entering COMMIT** (after E2E_VERIFY, batch terminal, `enable_commits: true`, all loops passed):

```
State:   COMMIT
Phase:   ui_implementing
Layer:   L0 Shell
Batch:   1/2
Items:
  - [shell.layout] passed
  - [shell.theme] passed
Action:  Advance with --message "your commit message" to commit and continue.
```

**Entering COMMIT** (after force-accept in one or more loops, `enable_commits: true`):

```
State:   COMMIT
Phase:   ui_implementing
Layer:   L1 Views
Batch:   2/2
Items:
  - [views.detail] failed (e2e force-accept, 3/3 rounds)
  - [views.list] failed (e2e force-accept, 3/3 rounds)
Action:  Advance with --message "your commit message" to commit and continue.
```

When `enable_commits: false`, the COMMIT action reads "Advance to continue." in both cases, exactly as in the implementing phase.

**Entering ORIENT / DONE** after COMMIT follow the implementing phase's ORIENT and DONE outputs, with the `Phase:` line reading `ui_implementing`. The DONE summary additionally reports QA and e2e round totals:

```
State:   DONE
Phase:   ui_implementing
Domain:  portal
Summary:
  L0 Shell:  2/2 passed
  L1 Views:  2/2 passed
  Total:     4/4 items passed
  Rounds:    code 5, qa 4, e2e 6 (across 2 batches)
Action:  Domain complete. Advance to continue to next domain.
```

#### `eval` output

In EVALUATE, the `eval` output is identical to the implementing phase's `eval` output (it embeds `evaluators/impl-eval.md` and the batch item details).

In QA_TEST, the `eval` output embeds `evaluators/ui-qa-eval.md`, the batch item details, the application launch command and URL, and the target path for the e2e step list:

```
=== UI QA EVALUATION ROUND 1/3 ===
Layer: L0 Shell
Batch: 1/2

--- QA EVALUATOR INSTRUCTIONS ---

<contents of evaluators/ui-qa-eval.md>

--- APPLICATION ---

Launch:        npm run dev
URL:           http://localhost:5173
Ready timeout: 30s
Driver:        Playwright MCP — navigate to the URL, snapshot for placement,
               click/type to exercise controls, read console for runtime errors

--- ITEMS TO QA ---

[1] shell.layout — App shell and navigation frame
    Description: Top bar, sidebar, and routed content region.
    Specs:       app-shell.md#interface-outputs
    ...

--- STEP LIST OUTPUT ---

Write the e2e step list to:
  portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json

--- REPORT OUTPUT ---

Write your QA report to:
  portal/.forgectl_workspace/ui_plan/qa/batch-1-round-1.md

--- HANDOFF ---

When finished, register your generated files with:
  forgectl handoff portal/.forgectl_workspace/ui_plan/qa/batch-1-round-1.md \
                   portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json
```

Under `qa.eval_mode: "direct"`, the `--- REPORT OUTPUT ---` section instructs the sub-agent to make placement corrections directly. Under `qa.eval_mode: "conversational"`, the `--- REPORT OUTPUT ---` section is omitted. The `--- STEP LIST OUTPUT ---` section is present in all three modes. The `--- HANDOFF ---` section is present in all three modes; in `report` mode it lists the report and the step list, and in `direct` and `conversational` mode it lists only the step list (no report file exists — under `direct` mode the placement corrections are unstaged changes the engineer reviews via diff, not a handed-off file).

In E2E_VERIFY, the `eval` output embeds `evaluators/ui-e2e-eval.md`, the QA step list path, the e2e test command, the test directory the authored tests live in, and (in `report` mode) the report target path:

```
=== UI E2E VERIFICATION ROUND 1/3 ===
Layer: L0 Shell
Batch: 1/2

--- E2E EVALUATOR INSTRUCTIONS ---

<contents of evaluators/ui-e2e-eval.md>

--- E2E SUITE ---

Step list:    portal/.forgectl_workspace/ui_plan/qa/batch-1-steps.json
Test command: npm run e2e
Test dir:     e2e/
Runner:       Playwright test runner (the authored tests are Playwright test files)

--- REPORT OUTPUT ---

Write your e2e verification report to:
  portal/.forgectl_workspace/ui_plan/e2e/batch-1-round-1.md

--- HANDOFF ---

When finished, register your report with:
  forgectl handoff portal/.forgectl_workspace/ui_plan/e2e/batch-1-round-1.md
```

The e2e loop has no step-list output of its own, so hand-off here covers only the report. Under `e2e.eval_mode: "direct"` and `"conversational"` no report file is produced, so both the `--- REPORT OUTPUT ---` and `--- HANDOFF ---` sections are omitted and the engineer records the verdict directly.

Subsequent rounds in QA_TEST and E2E_VERIFY include a `--- PREVIOUS EVALUATIONS ---` section listing prior round verdicts and report paths, exactly as the implementing phase's `eval` output does for code evaluation.

#### Report and Artifact Locations

```
<domain>/.forgectl_workspace/ui_plan/plan.json                       — the UI plan
<domain>/.forgectl_workspace/ui_plan/qa/batch-N-steps.json           — QA step list (latest round overwrites)
<domain>/.forgectl_workspace/ui_plan/qa/batch-N-round-M.md           — QA reports
<domain>/.forgectl_workspace/ui_plan/qa/batch-N-round-M-*.png         — QA screenshot evidence (Playwright MCP captures)
<domain>/.forgectl_workspace/ui_plan/e2e/batch-N-round-M.md          — e2e verification reports
```

Authored e2e tests are **Playwright test files**, written to the configured `ui_implementing.e2e.test_dir` within the project, not to the workspace, because they are permanent test assets. QA screenshots the Playwright MCP captures are written alongside the QA report and referenced by it; they are evidence, not test assets.

#### `status` output — `ui_implementing` (compact)

```
Batch:   2/2 — [views.detail], [views.list]
Layer:   L1 Views (2/2 layers)
Loop:    qa  (round 1/3)

Action:  Please spawn 1 opus sub-agent to QA the running UI.
         The sub-agent should run: forgectl eval

Progress: 2/4 passed, 0 failed, 2 remaining
```

In the IMPLEMENT state, the compact output shows the single current `Item:` and its `(n of m)` position, exactly as the implementing phase does. In the QA and e2e loops there is no single active item — the whole batch is under evaluation — so the output shows the `Batch:` item list instead. The `Loop:` line names which of the three loops (`code`, `qa`, `e2e`) the batch is currently in, and the round within that loop. With `--verbose`, the layer-by-item breakdown is appended exactly as in the implementing phase, with an added per-item line summarizing rounds taken in each loop (sourced from batch history per Round Tracking).

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance` in IMPLEMENT (first round) without `--message` when `enable_commits: true` | Error. Exit code 1. | First-round items need a commit message when commits are enabled |
| `advance` in COMMIT without `--message` when `enable_commits: true` | Error. Exit code 1. | Batch completion needs a commit message when commits are enabled |
| `advance` in EVALUATE, QA_TEST, or E2E_VERIFY without `--verdict` | Error. Exit code 1. | The verdict determines the transition |
| `advance` in an evaluator state without `--eval-report` when that loop's `eval_mode: "report"` | Error. Exit code 1. | Every evaluation must reference its report when report output is enabled |
| `advance --eval-report` pointing to a non-existent file | Error naming the path. Exit code 1. | The report must exist to be recorded |
| `advance --eval-report` when the active loop's `eval_mode` is not `"report"` | Warning: `--eval-report is ignored, --eval-report is only used in report mode`. Command proceeds. | Consistent with the implementing phase's warning pattern |
| `advance` out of QA_TEST toward E2E_AUTHOR (either PASS at min rounds or FAIL force-accept at max rounds) when the QA step list file is absent | Error naming the expected step-list path. Exit code 1. | E2E_AUTHOR has no input without it; the QA loop must produce it on every exit path (invariant 6) |
| `eval` outside `ui_implementing` EVALUATE, QA_TEST, or E2E_VERIFY | Error naming the current state and phase. Exit code 1. | Evaluation context only exists in the three evaluator states |
| `handoff` outside `ui_implementing` EVALUATE, QA_TEST, or E2E_VERIFY | Error naming the current state and phase. Exit code 1. | Artifacts can only be handed off from an evaluator round |
| `handoff` with no file arguments | Error. Exit code 1. | Hand-off must name at least one file |
| `handoff <file>` naming a file that does not exist | Error naming the path. Exit code 1. | The scaffold registers only files that exist |
| `advance --eval-report <path>` in `report` mode where a report was handed off at a different path | Warning: `--eval-report differs from the handed-off report <handoff-path>`. Command proceeds, recording the `--eval-report` path. | Hand-off and `--eval-report` coexist; divergence is surfaced but not blocked |
| `init --phase ui_implementing` with any of `ui_implementing.app.launch_command`, `ui_implementing.app.url`, `ui_implementing.e2e.test_command`, or `ui_implementing.e2e.test_dir` empty | Error naming each missing key. Exit code 1. | The QA loop needs a running app and its URL; the e2e loop needs a test runner and a directory to author into |

---

## Behavior

### Batch Calculation

Identical to the implementing phase. Batches are groups of up to `ui_implementing.batch` unblocked items drawn from the current layer, selected in the order they appear in the layer's `items` array. An item is **unblocked** when all items in prior layers are terminal and all items in its `depends_on` list are terminal.

### State Machine

```
ORIENT → IMPLEMENT(1) → ... → IMPLEMENT(n) → EVALUATE
                                                 │  (code loop)
                              ┌──────────────────┼──────────────────┐
                        PASS+rounds         FAIL+rounds         PASS/FAIL
                        >= eval.min         < eval.max          at eval boundary
                              │                  │                   │
                              ▼                  ▼                   │
                          QA_TEST          IMPLEMENT(1)→...          │
                              │             (re-implement)           │
              ┌───────────────┼───────────────┐                     │
        PASS+rounds      FAIL+rounds      PASS/FAIL                  │
        >= qa.min        < qa.max         at qa boundary             │
              │               │                │                     │
              ▼               ▼                │                     │
        E2E_AUTHOR        UI_REFINE            │                     │
              │           (iterate UI)         │                     │
              ▼               │                │                     │
        E2E_VERIFY ◄──────────┘ (→ QA_TEST)    │                     │
              │  (e2e loop)                     │                     │
        ┌─────┼─────────────┐                  │                     │
  PASS+rounds  FAIL+rounds   PASS/FAIL          │                     │
  >= e2e.min   < e2e.max     at e2e boundary    │                     │
        │          │              │             │                     │
        ▼          ▼              │             │                     │
     COMMIT   E2E_REMEDIATE       │             │                     │
        │     (→ E2E_VERIFY)      │             │                     │
        ▼                         ▼             ▼                     ▼
   ORIENT/DONE ◄────────────────────────── (all loop boundaries reach COMMIT)
```

Each batch passes through three sequential loops — code eval, then QA, then e2e — and reaches COMMIT only after the e2e loop terminates. A loop "terminates" when it reaches PASS with rounds at or above its `min_rounds`, or when it force-accepts at its `max_rounds`.

### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | unblocked items exist in current layer | IMPLEMENT | Select batch. Reset `eval_round`, `qa_round`, `e2e_round` to 0. Present first item. |
| ORIENT | all layer items terminal, more layers | ORIENT (next layer) | Advance `current_layer`. |
| ORIENT | all layers complete | DONE | — |
| IMPLEMENT | more items in batch | IMPLEMENT | Mark current item `done`. Present next item. |
| IMPLEMENT | last item in batch | EVALUATE | Mark current item `done`. Increment `eval_round`. |
| EVALUATE | PASS, `eval_round` >= `ui_implementing.eval.min_rounds` | QA_TEST | Record eval. Increment `qa_round`. Present QA action. |
| EVALUATE | PASS, `eval_round` < `ui_implementing.eval.min_rounds` | IMPLEMENT | Record eval. Re-present first item. |
| EVALUATE | FAIL, `eval_round` < `ui_implementing.eval.max_rounds` | IMPLEMENT | Record eval. Re-present first item. |
| EVALUATE | FAIL, `eval_round` >= `ui_implementing.eval.max_rounds` | QA_TEST | Record eval. Flag batch code-eval force-accepted. Increment `qa_round`. |
| QA_TEST | PASS, `qa_round` >= `ui_implementing.qa.min_rounds` | E2E_AUTHOR | Record QA eval. Require step list present. |
| QA_TEST | PASS, `qa_round` < `ui_implementing.qa.min_rounds` | UI_REFINE | Record QA eval. |
| QA_TEST | FAIL, `qa_round` < `ui_implementing.qa.max_rounds` | UI_REFINE | Record QA eval. |
| QA_TEST | FAIL, `qa_round` >= `ui_implementing.qa.max_rounds` | E2E_AUTHOR | Record QA eval. Flag batch qa force-accepted. Require step list present. |
| UI_REFINE | always | QA_TEST | Increment `qa_round`. Re-present QA action. |
| E2E_AUTHOR | always | E2E_VERIFY | Increment `e2e_round`. |
| E2E_VERIFY | PASS, `e2e_round` >= `ui_implementing.e2e.min_rounds` | COMMIT | Record e2e eval. Mark items `passed` (or `failed` if any loop force-accepted). |
| E2E_VERIFY | PASS, `e2e_round` < `ui_implementing.e2e.min_rounds` | E2E_REMEDIATE | Record e2e eval. |
| E2E_VERIFY | FAIL, `e2e_round` < `ui_implementing.e2e.max_rounds` | E2E_REMEDIATE | Record e2e eval. |
| E2E_VERIFY | FAIL, `e2e_round` >= `ui_implementing.e2e.max_rounds` | COMMIT | Record e2e eval. Flag batch e2e force-accepted. Mark items `failed`. |
| E2E_REMEDIATE | always | E2E_VERIFY | Increment `e2e_round`. |
| COMMIT | more batches or layers | ORIENT | — |
| COMMIT | all layers complete | DONE | — |
| DONE | `plan_all_before_implementing: false`, planning queue non-empty | PHASE_SHIFT | `ui_implementing` → planning. Return to planning for the next domain. |
| DONE | `plan_all_before_implementing: true`, implementation plan queue non-empty | PHASE_SHIFT | Domain boundary. Pull next plan; route to `implementing` or `ui_implementing` by that plan's `kind`. |
| DONE | no plans remaining | _(terminal)_ | Session complete. |

### Item `passes` Transitions

| Event | `passes` change |
|-------|----------------|
| Engineer advances past item in IMPLEMENT | `pending` → `done` |
| E2E_VERIFY PASS + `e2e_round` >= min, no loop force-accepted in the batch | `done` → `passed` |
| Any loop force-accepted in the batch, at COMMIT | `done` → `failed` |

An item's terminal status is decided only at COMMIT, after all three loops have terminated. While the batch moves through code eval, QA, and e2e, items remain `done`.

### Round Tracking

The scaffold tracks three per-batch counters — `eval_round`, `qa_round`, `e2e_round` — in the current-batch state. Each is reset to 0 when a batch is selected at ORIENT and incremented as defined in the transition table. Each loop also carries its own ordered evaluation history (the per-round verdicts and report paths), kept separately so the three loops do not share a round namespace.

When a batch reaches COMMIT, its three counters and the three histories are recorded in batch history. The per-loop totals reported by the DONE summary and by `status --verbose` are sourced from this batch history, not from any single aggregate. The plan.json item `rounds` field holds the sum of the batch's `eval_round`, `qa_round`, and `e2e_round` — a compatibility value for tools that read the implementing-phase plan.json shape; it is never the source of the per-loop breakdown.

Tracking the three counters and three histories requires the current-batch and batch-history state to carry per-loop round and evaluation fields beyond the implementing phase's single eval counter.

### IMPLEMENT and Code EVALUATE Behavior

Identical to the implementing phase. IMPLEMENT presents one item at a time with full context; the first round commits each item when `enable_commits: true`; subsequent rounds apply code-eval corrections per `eval_mode` and do not commit. Code EVALUATE spawns the sub-agent that runs `forgectl eval` against `evaluators/impl-eval.md`, and in `report` mode the sub-agent hands its report back with `forgectl handoff <impl-eval-report>` exactly as the QA and e2e sub-agents do — the `handoff` command is shared across all three evaluator states (see Hand-off Behavior). The canonical definition of the code EVALUATE loop, including its hand-off, lives in batch-implementation; this phase reuses it unchanged.

### QA_TEST Behavior

Two actors, mirroring the implementing phase's EVALUATE.

**Sub-agent** runs `forgectl eval` to receive the QA evaluator prompt, the batch's UI items, the application launch command and URL, the step-list target path, and (in `report` mode) the report target path and previous QA verdicts. The sub-agent launches the application and **drives it through the Playwright MCP**: it navigates to `ui_implementing.app.url`, reads the accessibility snapshot to judge placement and control structure, exercises each control (click, type, fill, select), reads the console for runtime errors, and captures screenshots as evidence. It judges placement and controls, **always writes the QA step list** to `<domain>/.forgectl_workspace/ui_plan/qa/batch-N-steps.json`, produces its verdict per `eval_mode` (writes a report, makes placement corrections directly, or relays verbally), and **hands its generated files back** with `forgectl handoff` — the report and the step list in `report`/`direct` mode, the step list alone in `conversational` mode.

The accessibility snapshot is the primary judgment surface: it is structural and deterministic, exposing each control's role, label, and order without relying on a screenshot. Screenshots are supplementary evidence attached to the report.

**Engineer** reviews the agent-provided files named on the `Review:` line (or the unstaged changes / verbal verdict) and runs `forgectl advance --verdict PASS|FAIL` (with `--eval-report` in `report` mode).

The step list is rewritten on every QA round. The most recent step list is the input to E2E_AUTHOR.

### UI_REFINE Behavior

The fixer half of the QA loop. The engineer iterates on UI placement and controls in response to the QA findings — moving, resizing, relabeling, reordering, or rewiring controls and adjusting layout. Under `report` mode the engineer studies the QA report; under `direct` mode the engineer reviews unstaged changes the QA sub-agent made; under `conversational` mode the engineer applies the verbally communicated corrections. No `--message` is required and no commit occurs. Advancing returns to QA_TEST for re-evaluation.

### E2E_AUTHOR Behavior

A single bridging state, not a loop. The engineer reads the QA step list and authors one runnable **Playwright test file** per scenario into `ui_implementing.e2e.test_dir`, mapping each scenario's `steps` to Playwright actions and each `expected` entry to a Playwright assertion, then executes the suite via `ui_implementing.e2e.test_command` (the Playwright test runner). Whether the suite passes, fails, or the test command itself errors (for example, the runner is misconfigured or the app does not start), the engineer still advances to E2E_VERIFY — the verdict on the run is the e2e sub-agent's responsibility, not E2E_AUTHOR's. When the step list has zero scenarios, the engineer authors nothing and advances directly.

### E2E_VERIFY Behavior

The evaluator half of the e2e loop. **Sub-agent** runs `forgectl eval` to receive the e2e evaluator prompt, the QA step list path, the test command, and (in `report` mode) the report target path and previous e2e verdicts. It confirms the authored Playwright tests execute, pass, and cover the scenarios in the step list, produces its verdict per `eval_mode`, and (in `report` mode) **hands its report back** with `forgectl handoff <e2e-report>`. **Engineer** reviews the agent-provided report named on the `Review:` line and records the verdict via `forgectl advance --verdict PASS|FAIL`. When the step list has zero scenarios, the sub-agent's verdict is PASS (there is nothing to fail), and the loop terminates at its minimum rounds.

### E2E_REMEDIATE Behavior

The fixer half of the e2e loop. The engineer fixes the failing tests or the UI under test (a failure may indicate a real UI defect or a faulty test) and re-runs the suite. No `--message` is required and no commit occurs. Advancing returns to E2E_VERIFY for re-verification.

### Hand-off Behavior

`handoff` is the sub-agent's artifact-return command, the counterpart to `eval`. After the sub-agent has produced its files in an evaluator state (EVALUATE, QA_TEST, or E2E_VERIFY), it runs `forgectl handoff <file>…` naming each generated file. The scaffold:

1. Verifies each named file exists; a missing file is rejected naming the path.
2. Records the paths into the current evaluator round's state as handed-off artifacts (replacing any artifacts handed off earlier in the same round — the latest hand-off wins).
3. Surfaces them on the `Review:` line of subsequent `status` and stay-in-state `advance` output, so the engineer reviews the agent-provided files before recording the verdict.

Hand-off carries no verdict and does not itself transition the state machine — the engineer still records the verdict with `advance --verdict`. It coexists with `--eval-report`: the handed-off report and `--eval-report` are independent, and the scaffold warns (without blocking) if a `--eval-report` path in `report` mode differs from the report that was handed off. The scaffold does not read the contents of handed-off files; for the step list it still confirms presence and counts `scenarios` at the QA→e2e boundary exactly as before — hand-off is the mechanism by which that step-list file becomes registered.

### COMMIT State

A hard stop after the e2e loop terminates. When `enable_commits: true`, the engineer runs `forgectl advance --message <commit msg>` to stage per `ui_implementing.commit_strategy` and commit. When `false`, the engineer runs `forgectl advance`. Items are marked terminal here: `passed` when all three loops reached PASS within budget, `failed` when any loop force-accepted at its maximum rounds.

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `ui_implementing.batch` | integer | 1 | Maximum unblocked items per batch |
| `ui_implementing.commit_strategy` | string | `"scoped"` | How files are staged at commit points |
| `ui_implementing.app.launch_command` | string | (none) | Command that starts the application for QA; required |
| `ui_implementing.app.url` | string | (none) | Base URL the running application serves; required |
| `ui_implementing.app.ready_timeout_seconds` | integer | 30 | How long the QA sub-agent waits for the app to become reachable |
| `ui_implementing.eval.min_rounds` | integer | 1 | Minimum code-evaluation rounds |
| `ui_implementing.eval.max_rounds` | integer | 3 | Maximum code-evaluation rounds before force-accept |
| `ui_implementing.eval.model` | string | `"opus"` | Model for the code-evaluation sub-agent |
| `ui_implementing.eval.eval_mode` | string | `"report"` | `"report"` \| `"direct"` \| `"conversational"` |
| `ui_implementing.qa.min_rounds` | integer | 1 | Minimum QA rounds |
| `ui_implementing.qa.max_rounds` | integer | 3 | Maximum QA rounds before force-accept |
| `ui_implementing.qa.model` | string | `"opus"` | Model for the QA sub-agent |
| `ui_implementing.qa.eval_mode` | string | `"report"` | `"report"` \| `"direct"` \| `"conversational"` |
| `ui_implementing.e2e.min_rounds` | integer | 1 | Minimum e2e verification rounds |
| `ui_implementing.e2e.max_rounds` | integer | 3 | Maximum e2e verification rounds before force-accept |
| `ui_implementing.e2e.model` | string | `"opus"` | Model for the e2e verification sub-agent |
| `ui_implementing.e2e.eval_mode` | string | `"report"` | `"report"` \| `"direct"` \| `"conversational"` |
| `ui_implementing.e2e.test_command` | string | (none) | Command that runs the authored e2e suite; required |
| `ui_implementing.e2e.test_dir` | string | (none) | Directory where authored e2e tests are written; required |

The three loops carry independent round budgets and `eval_mode` settings. `enable_commits` and `user_guided` are top-level flags (`config.general.enable_commits`, `config.general.user_guided`), shared across all phases; `commit_strategy` is phase-scoped (`ui_implementing.commit_strategy`). All three follow the same semantics as the implementing phase.

**Playwright requires no configuration of its own.** The QA loop's browser driver is the Playwright MCP, which the QA sub-agent reaches by navigating to `ui_implementing.app.url` — the same URL the scaffold already renders into QA output. The e2e loop runs the authored Playwright tests through `ui_implementing.e2e.test_command` and writes them into `ui_implementing.e2e.test_dir`. No `ui_implementing.qa.driver` or Playwright-specific key exists: the scaffold never invokes Playwright directly; it only surfaces the URL and the test command in its output, and the agent supplies the rest. The driver is named in output (so the agent knows to use it), not in config (which the agent never reads).

---

## Observability

### Logging

| Level | What is logged |
|-------|---------------|
| INFO | Batch selected; each loop entered (code, qa, e2e) with its round; artifacts handed off with their paths and count; verdict recorded per loop; QA step list read at QA→e2e exit with its scenario count; batch committed |
| WARN | `--eval-report` supplied when the active loop is not in report mode; `--eval-report` path differs from the handed-off report; a loop force-accepted at its maximum rounds; QA step list read with zero scenarios |
| ERROR | `advance`/`eval`/`handoff` rejected (missing verdict, missing report, absent step list, missing required config, `eval` or `handoff` outside an evaluator state, `handoff` with no files or a non-existent file) |
| DEBUG | Computed unblocked-item set; per-loop round counters; resolved app launch command, URL, test command, and test directory; resolved artifact paths; registered hand-off artifacts for the current round |

### Metrics

This topic emits no metrics.

---

## Invariants

1. **Layer ordering enforced.** All items in layer N must be terminal before layer N+1.
2. **Dependency ordering enforced.** Items are only delivered when `depends_on` items are terminal.
3. **Item order preserved.** Items are delivered in the layer's `items` array order.
4. **One item at a time.** IMPLEMENT presents a single item per advance.
5. **Three sequential gates.** Every batch passes through code EVALUATE, then the QA loop, then the e2e loop, in that order, before reaching COMMIT. No batch reaches COMMIT having skipped a loop.
6. **QA step list always produced.** Every QA_TEST round writes the step list regardless of `eval_mode`. A QA loop never terminates toward E2E_AUTHOR without a step-list file present.
7. **E2E tests derive from the QA step list.** E2E_AUTHOR authors tests from the most recent QA step list; it does not invent scenarios absent from that list.
8. **Independent round budgets.** Each loop counts and bounds its rounds separately (`eval_round`, `qa_round`, `e2e_round`).
9. **Per-loop force-accept.** A FAIL at a loop's `max_rounds` forces that loop to terminate and the batch to proceed; it does not abort the phase.
10. **Per-loop minimum rounds.** A PASS below a loop's `min_rounds` forces another pass through that loop.
11. **Terminal status reflects all loops.** An item is `passed` only when all three loops reached PASS within budget; it is `failed` when any loop force-accepted.
12. **COMMIT precedes progression.** Every batch boundary passes through COMMIT before ORIENT/DONE.
13. **Two actors, three commands.** The engineer uses `advance`; the sub-agent uses `eval` (context in) and `handoff` (artifacts back). Both `eval` and `handoff` are valid only in EVALUATE, QA_TEST, and E2E_VERIFY.
14. **Scaffold does not parse evaluation reports.** Verdicts are provided via `--verdict`; report paths are stored only in `report` mode. `handoff` registers artifact paths but the scaffold never reads their contents. The scaffold reads the QA step-list file only to confirm its presence and count its `scenarios` array at the QA→e2e boundary; it does not interpret report contents or e2e run results, which reach the scaffold solely through `--verdict`.
15. **Hand-off carries no verdict.** `handoff` only registers files and drives the `Review:` prompt; it never transitions the state machine. The verdict is always recorded by the engineer via `advance --verdict`, independent of any hand-off.
16. **Guided pauses.** When `config.general.user_guided` is true, ORIENT output includes the STOP-and-review line.
17. **Auto-commit at commit points.** When `enable_commits: true`, `--message` is required at IMPLEMENT (first round) and COMMIT; the scaffold stages per `ui_implementing.commit_strategy` and commits.

---

## Edge Cases

- **Scenario:** Code EVALUATE force-accepts at `eval.max_rounds`.
  - **Expected behavior:** The batch still proceeds into QA_TEST; the code-eval force-accept is flagged and surfaces as `failed` at COMMIT.
  - **Rationale:** The UI verification section runs for every batch; QA and e2e may reveal what the code eval could not, and the failure is still recorded.

- **Scenario:** QA_TEST force-accepts at `qa.max_rounds`.
  - **Expected behavior:** The batch proceeds into E2E_AUTHOR using the most recent step list; the qa force-accept surfaces as `failed` at COMMIT.
  - **Rationale:** Placement issues should not block e2e verification indefinitely; the failure is recorded.

- **Scenario:** QA_TEST produces a step list with zero scenarios.
  - **Expected behavior:** E2E_AUTHOR authors nothing and advances; E2E_VERIFY's verdict is PASS; the e2e loop terminates at its minimum rounds. A WARN is logged.
  - **Rationale:** Some batches (e.g., pure theme tokens) have no end-to-end flow; an empty suite is a valid, vacuously passing result.

- **Scenario:** E2E_VERIFY force-accepts at `e2e.max_rounds`.
  - **Expected behavior:** COMMIT. Items marked `failed`. A WARN is logged.
  - **Rationale:** Maximum remediation rounds exhausted; force-accept prevents an indefinite loop, consistent with the other loops.

- **Scenario:** The application fails to launch within `ui_implementing.app.ready_timeout_seconds` during QA_TEST.
  - **Expected behavior:** The Playwright MCP navigation to `ui_implementing.app.url` fails or the page surfaces console errors; the QA sub-agent returns FAIL with the launch/navigation failure as its finding (and still hands off a step list, which may be empty); the loop proceeds to UI_REFINE so the engineer can fix the build or launch path.
  - **Rationale:** An unlaunchable app is a defect the QA loop is meant to surface and drive to a fix; a failed MCP navigation or console errors are how it becomes observable.

- **Scenario:** The sub-agent runs `handoff` for a file it never wrote (typo or wrong path).
  - **Expected behavior:** `handoff` is rejected naming the missing path; no artifact is registered; the sub-agent re-runs `handoff` with the correct path.
  - **Rationale:** Only files that exist can be reviewed; registering a phantom path would break the `Review:` prompt and, for the step list, the QA→e2e boundary check.

- **Scenario:** The engineer passes `--eval-report` pointing at a different file than the one the sub-agent handed off.
  - **Expected behavior:** A WARN names both paths; the command proceeds and records the `--eval-report` path.
  - **Rationale:** Hand-off and `--eval-report` coexist; divergence usually signals a mistake but is the engineer's call, so it is surfaced, not blocked.

- **Scenario:** Advancing out of QA_TEST toward E2E_AUTHOR with the step-list file absent.
  - **Expected behavior:** `advance` is rejected naming the expected path; the state remains QA_TEST.
  - **Rationale:** E2E_AUTHOR has no input without the step list; invariant 6 must hold.

- **Scenario:** Item depends on a `failed` item.
  - **Expected behavior:** Still unblocked — `failed` is terminal.
  - **Rationale:** Consistent with the implementing phase; terminal includes both `passed` and `failed`.

- **Scenario:** Batch has one item.
  - **Expected behavior:** IMPLEMENT → EVALUATE directly, then the QA and e2e loops run normally.
  - **Rationale:** Single-item batches skip the multi-item IMPLEMENT advance loop only; the verification loops are unchanged.

- **Scenario:** `init --phase ui_implementing` with `ui_implementing.e2e.test_command` empty.
  - **Expected behavior:** Init is rejected naming the missing key.
  - **Rationale:** The e2e loop is integral to this phase; without a runner it cannot function.

---

## Testing Criteria

### ORIENT selects first batch
- **Verifies:** Batch selection and round-counter reset.
- **Given:** ORIENT (`ui_implementing`), L0 has 3 items, `ui_implementing.batch: 2`.
- **When:** `advance`
- **Then:** State is IMPLEMENT. First item presented. `eval_round`, `qa_round`, `e2e_round` are 0.

### Code EVALUATE PASS proceeds to QA_TEST
- **Verifies:** Code loop terminates into the QA loop.
- **Given:** EVALUATE, `eval_round` >= `ui_implementing.eval.min_rounds`.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** State is QA_TEST. `qa_round` is 1.

### Code EVALUATE force-accept proceeds to QA_TEST
- **Verifies:** Force-accepted code eval still enters the QA loop.
- **Given:** EVALUATE, FAIL, `eval_round` == `ui_implementing.eval.max_rounds`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is QA_TEST. The batch is flagged code-eval force-accepted.

### QA_TEST FAIL within max rounds → UI_REFINE
- **Verifies:** QA failure triggers UI iteration.
- **Given:** QA_TEST, `qa_round` < `ui_implementing.qa.max_rounds`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is UI_REFINE.

### UI_REFINE returns to QA_TEST
- **Verifies:** QA loop re-evaluation.
- **Given:** UI_REFINE.
- **When:** `advance`
- **Then:** State is QA_TEST. `qa_round` incremented.

### QA_TEST PASS with sufficient rounds → E2E_AUTHOR
- **Verifies:** QA loop terminates into the e2e loop.
- **Given:** QA_TEST, `qa_round` >= `ui_implementing.qa.min_rounds`, step list present.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** State is E2E_AUTHOR.

### QA_TEST PASS with step list absent is rejected
- **Verifies:** Invariant 6 enforced at the QA→e2e boundary.
- **Given:** QA_TEST, `qa_round` >= min, no step-list file present.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** Exit code 1. State remains QA_TEST.

### E2E_AUTHOR advances to E2E_VERIFY
- **Verifies:** Authoring bridges into the e2e verification loop.
- **Given:** E2E_AUTHOR.
- **When:** `advance`
- **Then:** State is E2E_VERIFY. `e2e_round` is 1.

### E2E_VERIFY FAIL within max rounds → E2E_REMEDIATE
- **Verifies:** e2e failure triggers remediation.
- **Given:** E2E_VERIFY, `e2e_round` < `ui_implementing.e2e.max_rounds`.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is E2E_REMEDIATE.

### E2E_REMEDIATE returns to E2E_VERIFY
- **Verifies:** e2e loop re-verification.
- **Given:** E2E_REMEDIATE.
- **When:** `advance`
- **Then:** State is E2E_VERIFY. `e2e_round` incremented.

### E2E_VERIFY PASS with sufficient rounds → COMMIT, items passed
- **Verifies:** Successful e2e termination, all loops passed.
- **Given:** E2E_VERIFY, `e2e_round` >= min, no loop force-accepted.
- **When:** `advance --eval-report ... --verdict PASS`
- **Then:** State is COMMIT. Items `passed`.

### E2E_VERIFY force-accept → COMMIT, items failed
- **Verifies:** e2e force-accept marks items failed.
- **Given:** E2E_VERIFY, FAIL, `e2e_round` == max.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** State is COMMIT. Items `failed`.

### Force-accept in any loop marks items failed at COMMIT
- **Verifies:** Invariant 11.
- **Given:** A batch where QA force-accepted but e2e passed.
- **When:** Batch reaches COMMIT.
- **Then:** Items `failed`. COMMIT output notes the qa force-accept.

### Empty step list passes the e2e loop vacuously
- **Verifies:** Zero-scenario edge case.
- **Given:** QA_TEST produced a step list with zero scenarios; state is E2E_AUTHOR.
- **When:** `advance` (E2E_AUTHOR), then `advance --verdict PASS` (E2E_VERIFY).
- **Then:** E2E loop terminates at min rounds. WARN logged at QA step-list write.

### eval valid in all three evaluator states
- **Verifies:** Invariant 13.
- **Given:** State is QA_TEST (and separately E2E_VERIFY).
- **When:** `forgectl eval`
- **Then:** QA_TEST output embeds `ui-qa-eval.md`, app launch/url, and step-list path. E2E_VERIFY output embeds `ui-e2e-eval.md`, step-list path, and test command.

### eval outside an evaluator state is rejected
- **Verifies:** Invariant 13 negative case.
- **Given:** State is UI_REFINE.
- **When:** `forgectl eval`
- **Then:** Exit code 1.

### handoff registers artifacts and surfaces them for review
- **Verifies:** Invariants 13 and 15 (hand-off registers files, no transition).
- **Given:** State is QA_TEST; the sub-agent has written a QA report and a step list.
- **When:** `forgectl handoff <qa-report> <step-list>`
- **Then:** Exit code 0. State remains QA_TEST. A subsequent `status` shows a `Review:` line listing both paths. No verdict is recorded.

### handoff outside an evaluator state is rejected
- **Verifies:** Invariant 13 negative case for `handoff`.
- **Given:** State is UI_REFINE.
- **When:** `forgectl handoff some/file.md`
- **Then:** Exit code 1 naming the current state and phase.

### handoff naming a non-existent file is rejected
- **Verifies:** Hand-off existence check.
- **Given:** State is E2E_VERIFY; the named report file does not exist.
- **When:** `forgectl handoff e2e/batch-1-round-1.md`
- **Then:** Exit code 1 naming the path. No artifact registered.

### --eval-report diverging from the handed-off report warns and proceeds
- **Verifies:** Hand-off / `--eval-report` coexistence drift guard.
- **Given:** QA_TEST, `report` mode; the sub-agent handed off `qa/batch-1-round-1.md`.
- **When:** `forgectl advance --verdict PASS --eval-report qa/other.md`
- **Then:** WARN naming both paths. The transition proceeds, recording `qa/other.md`.

### init rejects each missing required UI config key
- **Verifies:** Required-config rejection covers all four keys (B3).
- **Given:** Any one of `ui_implementing.app.launch_command`, `ui_implementing.app.url`, `ui_implementing.e2e.test_command`, `ui_implementing.e2e.test_dir` empty.
- **When:** `init --phase ui_implementing --from plan.json`
- **Then:** Exit code 1 naming the missing key.

### Step list absent on the QA force-accept exit is rejected
- **Verifies:** Invariant 6 holds on both QA exit paths.
- **Given:** QA_TEST, FAIL, `qa_round` == `ui_implementing.qa.max_rounds`, no step-list file present.
- **When:** `advance --eval-report ... --verdict FAIL`
- **Then:** Exit code 1 naming the expected step-list path. State remains QA_TEST.

### Every batch passes through all three loops in order
- **Verifies:** Invariant 5 (three sequential gates, no skips).
- **Given:** A batch entering at ORIENT.
- **When:** The batch is driven to COMMIT.
- **Then:** The state sequence includes EVALUATE, then QA_TEST, then E2E_VERIFY, in that order, before COMMIT — none skipped.

### Loops count and bound rounds independently
- **Verifies:** Invariant 8 (independent round budgets).
- **Given:** `eval.max_rounds: 1`, `qa.max_rounds: 2`, `e2e.max_rounds: 3`.
- **When:** Each loop is driven to its maximum.
- **Then:** The code loop force-accepts after 1 round, QA after 2, e2e after 3; each counter advances only while its own loop is active.

### QA round increments on re-entry from UI_REFINE
- **Verifies:** Round-increment semantics on the QA fixer path.
- **Given:** QA_TEST with `qa_round` == 1, FAIL recorded; state advanced to UI_REFINE.
- **When:** `advance` (UI_REFINE → QA_TEST)
- **Then:** State is QA_TEST. `qa_round` is 2.

### E2E round increments on re-entry from E2E_REMEDIATE
- **Verifies:** Round-increment semantics on the e2e fixer path.
- **Given:** E2E_VERIFY with `e2e_round` == 1, FAIL recorded; state advanced to E2E_REMEDIATE.
- **When:** `advance` (E2E_REMEDIATE → E2E_VERIFY)
- **Then:** State is E2E_VERIFY. `e2e_round` is 2.

### --eval-report in a non-report loop warns and proceeds
- **Verifies:** Warning path for a misapplied flag.
- **Given:** QA_TEST with `ui_implementing.qa.eval_mode: "conversational"`.
- **When:** `advance --verdict PASS --eval-report some/path.md`
- **Then:** Warning printed: `--eval-report is ignored, --eval-report is only used in report mode`. The transition proceeds.

### App launch failure surfaces as a QA FAIL
- **Verifies:** Unlaunchable-app edge case.
- **Given:** QA_TEST; the application does not become reachable within `ui_implementing.app.ready_timeout_seconds`.
- **When:** The QA sub-agent runs and the engineer records `advance --verdict FAIL`, `qa_round` < max.
- **Then:** State is UI_REFINE so the launch/build defect can be fixed.

### DONE transitions per plan_all_before_implementing
- **Verifies:** Phase-shift exit mirrors the implementing phase.
- **Given:** DONE (`ui_implementing`), `plan_all_before_implementing: false`, planning queue has 1 plan.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered (`ui_implementing` → planning).

### ui_implementing commits at IMPLEMENT and COMMIT when enable_commits is true
- **Verifies:** Invariant 17 — `--message` is required at IMPLEMENT (first round) and at COMMIT, and both points actually commit. (Surfaced by multi-domain pipeline integration test — `ui_implementing` required `--message` at both points but never called `AutoCommit`, producing zero commits.)
- **Given:** `enable_commits: true`, `ui_implementing.commit_strategy: "scoped"`. IMPLEMENT (first round); file on disk modified by implementation.
- **When:** `advance --message "implement login component"` (IMPLEMENT), then drive through loops to COMMIT, then `advance --message "batch commit"`.
- **Then:** `git log` is non-empty. Both the per-item commit (at first-round IMPLEMENT) and the batch commit (at COMMIT) appear in history.

---

## Implements
- `ui_implementing` phase: layer-ordered batched UI item delivery reusing the implementing-phase spine
- Per-batch QA placement loop (QA_TEST ⟲ UI_REFINE) with independent round budget and force-accept
- QA-generated e2e step list as the contract between the QA and e2e loops
- Per-batch e2e loop (E2E_AUTHOR → E2E_VERIFY ⟲ E2E_REMEDIATE) with independent round budget and force-accept
- Three independent verification loops (code eval, QA, e2e) gating each batch before COMMIT
- Triple evaluator prompts: impl-eval.md (code), ui-qa-eval.md (QA), ui-e2e-eval.md (e2e)
- UI-specific configuration: app launch command/URL and e2e test command/directory
- Playwright MCP as the QA driving mechanism, surfaced in QA output and the `eval` context (no Playwright-specific config)
- Playwright test files as the e2e authoring target, run by the configured Playwright test command
- `handoff` command: the sub-agent's artifact-return path that registers generated files and drives the engineer `Review:` prompt, coexisting with `--eval-report`
