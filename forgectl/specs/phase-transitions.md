# Phase Transitions

## Topic of Concern
> The scaffold enforces explicit context-refresh checkpoints between lifecycle phases.

## Context

The forgectl scaffold is a Go CLI tool (built with Cobra) that manages the full software development lifecycle through sequential phases — specifying, generate_planning_queue, planning, and an implementation phase — backed by a single JSON state file (`forgectl-state.json`). State names (ORIENT, EVALUATE, etc.) are reused across phases with phase-specific behavior; the `phase` field determines which states are valid and how they behave.

The implementation phase is one of two phases selected per plan: `implementing` (code-only batches) or `ui_implementing` (code batches plus per-batch QA and e2e verification loops). Each plan carries a `kind` field — `code` (default) or `ui` — and the planning→implementation phase shift routes to `implementing` when `kind` is `code` and to `ui_implementing` when `kind` is `ui`. Both consume the same plan.json and share the same DONE/phase-shift exit behavior; references below to "the implementation phase" apply to whichever of the two a plan's `kind` selects.

Between phases, a PHASE_SHIFT state acts as a hard stop — the user is told to refresh their context before proceeding. This prevents stale context from carrying over between fundamentally different activities. PHASE_SHIFT also fires at domain boundaries within the same phase, because switching domains means switching codebases.

The `planning.plan_all_before_implementing` config (default `false`) controls how domains are processed:
- **`false` (default, interleaved):** Each domain is planned and then immediately implemented before the next domain begins. The flow cycles: planning → implementing → planning → implementing.
- **`true` (all planning first):** All domains are planned first, then all domains are implemented. PHASE_SHIFT fires between domains within each phase.

The generate_planning_queue phase auto-generates a plan queue from completed specs and gives the architect an opportunity to review, reorder, and edit it before planning begins. It has three states: ORIENT (generates the file), REFINE (architect reviews/edits), and PHASE_SHIFT (validates and transitions to planning).

The scaffold can be initialized at specifying, planning, or implementing — allowing users to skip earlier phases when inputs already exist. The generate_planning_queue phase cannot be initialized directly; it requires a completed specifying phase.

## Depends On
- **spec-reconciliation** — COMPLETE triggers the specifying→generate_planning_queue phase shift.
- **plan-production** — ACCEPT or DONE triggers the planning→implementation phase shift (depending on `plan_all_before_implementing`).
- **batch-implementation** — DONE triggers implementing→planning phase shift when `plan_all_before_implementing: false` and plans remain.
- **ui-batch-implementation** — DONE triggers ui_implementing→planning phase shift when `plan_all_before_implementing: false` and plans remain; entered from planning when the plan's `kind` is `ui`.
- **state-persistence** — reads and writes the state file.

## Integration Points

| Spec | Relationship |
|------|-------------|
| spec-reconciliation | COMPLETE → PHASE_SHIFT (specifying → generate_planning_queue) |
| plan-production | Receives the plans queue when generate_planning_queue→planning advances; ACCEPT → PHASE_SHIFT (planning → implementation) when `plan_all_before_implementing: false`; DONE → PHASE_SHIFT when `true` |
| batch-implementation | Receives validated plan.json when planning→implementing advances (plan `kind: code`); DONE → PHASE_SHIFT (implementing → planning) when `plan_all_before_implementing: false` and plans remain; DONE → PHASE_SHIFT (implementing → next implementation phase) when `true` and plans remain |
| ui-batch-implementation | Receives validated plan.json when planning→ui_implementing advances (plan `kind: ui`); DONE → PHASE_SHIFT (ui_implementing → planning) when `plan_all_before_implementing: false` and plans remain; DONE → PHASE_SHIFT (ui_implementing → next implementation phase) when `true` and plans remain |
| session-init | Plan queue schema (same validation, including the `kind` routing field) reused at generate_planning_queue→planning shift |
| planning-readiness-gate | The two cold-start phase shifts into planning ORIENT (generate_planning_queue→planning, and the specifying→generate_planning_queue `--from` skip that lands directly in planning) are blocked when an incoming plan-queue domain's workspace is non-empty; the shift does not occur. The intra-session domain-boundary re-entries into planning ORIENT (planning→planning, implementing/ui_implementing→planning) are NOT gated. |

---

## Interface

### Inputs

#### `advance` flags at PHASE_SHIFT

| Phase Shift | Flags |
|-------------|-------|
| specifying → generate_planning_queue | `--from <path>` (optional). If provided, skips generate_planning_queue entirely and transitions directly to planning ORIENT. |
| generate_planning_queue → planning | `--from <path>` (optional). If provided, uses the override file instead of the auto-generated `<state_dir>/plan-queue.json`. |
| planning → implementing (plan `kind: code`) | (no additional flags) |
| planning → ui_implementing (plan `kind: ui`) | (no additional flags) |
| planning → planning (domain boundary) | (no additional flags) |
| implementing → planning | (no additional flags) |
| ui_implementing → planning | (no additional flags) |
| implementing/ui_implementing → implementing/ui_implementing (domain boundary) | (no additional flags) |

The `--guided` / `--no-guided` flags are accepted at phase shifts and update `config.general.user_guided` before the transition proceeds.

#### `advance` flags at generate_planning_queue states

| State | Flags |
|-------|-------|
| ORIENT | (no flags) |
| REFINE | (no flags) |

### Outputs

#### `advance` output

**Entering PHASE_SHIFT** (specifying → generate_planning_queue):

```
State:   PHASE_SHIFT
From:    specifying → generate_planning_queue

Domains:  2 (optimizer, portal)
Specs:    5 completed
Roots:    optimizer → optimizer/, lib/shared/
          portal → portal/ (default)

Stop and refresh your context, please.
When ready:
  forgectl advance                            # generate plan queue from completed specs
  forgectl advance --from <plan-queue.json>   # OR provide a plan queue (skips generation)
```

**Entering ORIENT** (generate_planning_queue):

```
State:   ORIENT
Phase:   generate_planning_queue

Generated: .forgectl/state/plan-queue.json

Advance to continue.
```

**Entering REFINE** (generate_planning_queue):

```
State:   REFINE
Phase:   generate_planning_queue

Stop and review the generated plan queue .forgectl/state/plan-queue.json. Reorder and edit as needed.

Advance when ready.
```

**Entering PHASE_SHIFT** (generate_planning_queue → planning):

```
State:   PHASE_SHIFT
From:    generate_planning_queue → planning

Advance to continue.
```

**Entering PHASE_SHIFT** (planning → implementing, plan `kind: code`):

```
State:   PHASE_SHIFT
From:    planning → implementing
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json

Stop and refresh your context, please.
When ready, run: forgectl advance
```

**Entering PHASE_SHIFT** (planning → ui_implementing, plan `kind: ui`):

```
State:   PHASE_SHIFT
From:    planning → ui_implementing
Plan:    Portal Dashboard
Domain:  portal
Kind:    ui
File:    portal/.forgectl_workspace/ui_plan/plan.json

Stop and refresh your context, please.
When ready, run: forgectl advance
```

**Entering PHASE_SHIFT** (planning → planning, domain boundary, `plan_all_before_implementing: true`):

```
State:   PHASE_SHIFT
From:    planning → planning (next domain)
Completed: launcher — Service Configuration (2 rounds)
Next:      portal — Portal Implementation Plan

Stop and refresh your context, please.
When ready, run: forgectl advance
```

**Entering PHASE_SHIFT** (implementing → planning, `plan_all_before_implementing: false`):

```
State:   PHASE_SHIFT
From:    implementing → planning
Completed: launcher — 5/5 items passed (3 batches)
Next:      portal — Portal Implementation Plan

Stop and refresh your context, please.
When ready, run: forgectl advance
```

**Entering PHASE_SHIFT** (ui_implementing → planning, `plan_all_before_implementing: false`):

```
State:   PHASE_SHIFT
From:    ui_implementing → planning
Completed: portal — 4/4 items passed (2 batches; code 5, qa 4, e2e 6 rounds)
Next:      reports — Reports Implementation Plan

Stop and refresh your context, please.
When ready, run: forgectl advance
```

**Entering PHASE_SHIFT** (implementing → implementing, domain boundary, `plan_all_before_implementing: true`):

```
State:   PHASE_SHIFT
From:    implementing → implementing (next domain)
Completed: launcher — 5/5 items passed (3 batches)
Next:      portal — Portal Implementation Plan

Stop and refresh your context, please.
When ready, run: forgectl advance
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance` at PHASE_SHIFT (specifying→generate_planning_queue) with `--from` pointing to invalid plan queue | Validation errors printed. State remains PHASE_SHIFT. Exit code 1. | Override file must be valid |
| `advance` at REFINE (generate_planning_queue) with invalid `<state_dir>/plan-queue.json` | Validation errors printed. State remains REFINE. Exit code 1. | Plan queue must be valid before transitioning |
| `advance` at PHASE_SHIFT (generate_planning_queue→planning) with `--from` pointing to invalid plan queue | Validation errors printed. State remains PHASE_SHIFT. Exit code 1. | Override file must be valid |
| `advance` at a cold-start PHASE_SHIFT into planning ORIENT (generate_planning_queue→planning, or the specifying→generate_planning_queue `--from` skip) when an incoming plan-queue domain's `<workspace_dir>/` contains prior-cycle artifacts | Dirty domains and the close-out remediation printed. Transition does not occur; state remains at the pre-planning phase. Exit code 1. | A planning cycle must not begin over unarchived prior work. The intra-session domain-boundary re-entries (planning→planning, implementing/ui_implementing→planning) are exempt (see planning-readiness-gate) |

---

## Behavior

### Specifying → Generate Planning Queue

#### Entering PHASE_SHIFT
When the architect advances from COMPLETE (spec-reconciliation), the scaffold transitions to PHASE_SHIFT. No work is done — the scaffold prints the phase shift message and waits.

#### Advancing from PHASE_SHIFT

**Without `--from`:**

1. Set `phase` to `"generate_planning_queue"`.
2. Set `state` to `ORIENT`.
3. Auto-generate the plan queue (see Generate Planning Queue phase below).
4. Print the ORIENT output.

**With `--from` (skip generation):**

1. Read and validate the plan queue at `--from`.
2. If validation fails: print errors. State remains PHASE_SHIFT.
3. If validation passes:
   - Populate the plans queue in state.
   - Set `phase` to `"planning"`.
   - Set `state` to `ORIENT`.
   - Pull first plan from queue.
   - Print the planning ORIENT action description.

### Generate Planning Queue

A three-state phase between specifying and planning. The scaffold auto-generates a plan queue from completed specs, writes it to a file, and gives the architect an opportunity to review and reorder before planning begins.

#### ORIENT

On entry, the scaffold:

1. Group completed specs by domain (order determined by the spec queue — the order domains first appeared).
2. For each domain, produce a plan entry:
   - `name`: `"<Domain> Implementation Plan"` (domain name capitalized).
   - `domain`: the domain name.
   - `kind`: `"code"` (the default). The architect sets `"ui"` during REFINE for domains that need the QA and e2e verification loops.
   - `file`: `<domain>/.forgectl_workspace/implementation_plan/plan.json`.
   - `specs`: all completed spec file paths for this domain.
   - `spec_commits`: deduplicated list of all `commit_hashes` from the domain's completed specs.
   - `code_search_roots`: from `specifying.domains[<domain>].code_search_roots` if set via `set-roots`, otherwise `["<domain>/"]`.
3. Write the plan queue to `<state_dir>/plan-queue.json`.
4. Print the ORIENT output.

Advancing from ORIENT transitions to REFINE.

#### REFINE

The architect reviews `<state_dir>/plan-queue.json`, reorders domains, adjusts entries, or leaves it unchanged.

Advancing from REFINE:

1. Read `<state_dir>/plan-queue.json`.
2. Validate the plan queue (same schema checks as external input).
3. If validation fails: print errors. State remains REFINE.
4. If validation passes: transition to PHASE_SHIFT.

#### PHASE_SHIFT (generate_planning_queue → planning)

Advancing from PHASE_SHIFT:

**Without `--from`:**

1. Read `<state_dir>/plan-queue.json` (already validated at REFINE).
2. Populate the plans queue in state.
3. Set `phase` to `"planning"`.
4. Set `state` to `ORIENT`.
5. Pull first plan from queue.
6. Print the planning ORIENT action description.

**With `--from` (override):**

1. Read and validate the plan queue at `--from`.
2. If validation fails: print errors. State remains PHASE_SHIFT.
3. If validation passes: same steps as without `--from`, using the override file.

**Override mode (`--from` provided):**

1. Read and validate the plans queue at `--from`.
2. If validation fails: print errors. State remains PHASE_SHIFT.
3. If validation passes: same steps as auto-generation step 4.

### Planning → Implementation (implementing or ui_implementing)

The target phase is selected by the plan's `kind`: `code` (default) routes to `implementing`, `ui` routes to `ui_implementing`. The trigger depends on `plan_all_before_implementing`:

- **`false` (default):** PHASE_SHIFT entered after each plan ACCEPT. The remaining plans stay in the planning queue.
- **`true`:** PHASE_SHIFT entered after planning DONE (all plans complete). All completed plans are copied to the implementation plan queue, each retaining its `kind`.

#### Entering PHASE_SHIFT
When `false`: the architect advances from planning ACCEPT, and the scaffold transitions to PHASE_SHIFT.
When `true`: the architect advances from planning DONE, and the scaffold transitions to PHASE_SHIFT.

#### Advancing from PHASE_SHIFT
No `--from` needed — the plan.json path is already known from the current or first completed plan.

1. Read plan.json at the plan's file path.
2. Validate the plan structure (same checks as the planning validation gate in plan-production).
3. If validation fails: print errors. State remains PHASE_SHIFT.
4. If validation passes:
   - Add `passes: "pending"` and `rounds: 0` to every item.
   - Write the updated plan.json.
   - Set `phase` to `"implementing"` when the plan's `kind` is `code`, or `"ui_implementing"` when `kind` is `ui`. For `ui_implementing`, the required `ui_implementing.app.*` and `ui_implementing.e2e.*` config keys must be non-empty; if any is empty, print errors and remain at PHASE_SHIFT.
   - Set `state` to `ORIENT`.
   - Print the ORIENT action description with initialization summary.
   - When `true`: populate the implementation plan queue from remaining completed plans.

### Planning → Planning (Domain Boundary)

Only when `plan_all_before_implementing: true`. After each plan ACCEPT (except the last), a PHASE_SHIFT fires between domains within the planning phase.

#### Entering PHASE_SHIFT
The architect advances from ACCEPT with plans remaining in the queue.

#### Advancing from PHASE_SHIFT
1. Pull next plan from queue.
2. Set `state` to `ORIENT`.
3. Print the planning ORIENT action description.

### Implementation → Planning

Only when `plan_all_before_implementing: false`. After the implementation phase (implementing or ui_implementing) reaches DONE for a domain, if plans remain in the planning queue, a PHASE_SHIFT returns to planning. The behavior is identical whichever implementation phase the domain used.

#### Entering PHASE_SHIFT
The architect advances from the implementation phase's DONE with plans remaining in the planning queue.

#### Advancing from PHASE_SHIFT
1. Set `phase` to `"planning"`.
2. Pull next plan from the planning queue.
3. Set `state` to `ORIENT`.
4. Print the planning ORIENT action description.

### Implementation → Implementation (Domain Boundary)

Only when `plan_all_before_implementing: true`. After the implementation phase reaches DONE for a domain, if plans remain in the implementation plan queue, a PHASE_SHIFT fires between domains. The target phase of the next domain is selected by the next plan's `kind` — `code` → `implementing`, `ui` → `ui_implementing` — so a single all-planning-first run can interleave code and UI domains.

#### Entering PHASE_SHIFT
The architect advances from the implementation phase's DONE with plans remaining in the implementation plan queue.

#### Advancing from PHASE_SHIFT
1. Pull next plan from the implementation plan queue.
2. Read and validate plan.json. Mutate items (add `passes`/`rounds`).
3. Set `phase` to `"implementing"` (next plan `kind: code`) or `"ui_implementing"` (next plan `kind: ui`). For `ui_implementing`, validate the required `ui_implementing.app.*` and `ui_implementing.e2e.*` config keys are non-empty.
4. Set `state` to `ORIENT`.
5. Print the ORIENT action description with initialization summary.

---

## Invariants

1. **Phase shifts are explicit.** The scaffold always stops at PHASE_SHIFT between phases and at domain boundaries within a phase. It never transitions directly across phase or domain boundaries.
2. **Domain boundaries are phase shifts.** Switching domains always fires a PHASE_SHIFT, whether between phases or within the same phase. This ensures context refresh when the codebase changes.
3. **specifying→planning has input.** The plan queue is either auto-generated (via generate_planning_queue phase) or provided via `--from` override at specifying PHASE_SHIFT (which skips generation).
4. **generate_planning_queue is not directly initializable.** `init --phase generate_planning_queue` is rejected. This phase requires completed specifying data.
5. **Architect controls plan queue ordering.** The auto-generated plan queue is written to a file the architect can review and reorder before it is consumed. Domain ordering in the generated file follows spec queue order (order of first appearance).
6. **planning→implementing validates.** plan.json is validated and mutated on advance out of PHASE_SHIFT.
7. **Guided setting is mutable.** `--guided` / `--no-guided` can change `config.general.user_guided` on any advance call, including at phase shifts.
8. **Interleaved mode (default).** When `plan_all_before_implementing: false`, each domain is planned then implemented before the next domain begins. Implementing DONE returns to planning if plans remain.
9. **All-planning-first mode.** When `plan_all_before_implementing: true`, all domains are planned with PHASE_SHIFT between each domain, then all domains are implemented with PHASE_SHIFT between each domain.
10. **One plan per domain.** Each domain has exactly one plan in the queue. No domain appears more than once.
11. **Kind selects the implementation phase.** The planning→implementation phase shift sets `phase` to `implementing` when the plan's `kind` is `code` (or absent) and to `ui_implementing` when `kind` is `ui`. A plan's `kind` is carried unchanged from the plan queue through planning into the implementation plan queue.

---

## Edge Cases

- **Scenario:** Advance from specifying PHASE_SHIFT without `--from`.
  - **Expected:** Transitions to generate_planning_queue ORIENT. Plan queue auto-generated and written to `<state_dir>/plan-queue.json`.
  - **Rationale:** Default flow gives the architect a chance to review and reorder the generated queue.

- **Scenario:** Advance from specifying PHASE_SHIFT with `--from` override.
  - **Expected:** External file validated. If valid, skips generate_planning_queue entirely, transitions directly to planning ORIENT.
  - **Rationale:** Override allows full control when the architect has a pre-built plan queue.

- **Scenario:** Advance from specifying PHASE_SHIFT with invalid `--from`.
  - **Expected:** Validation errors printed. State remains PHASE_SHIFT.
  - **Rationale:** Invalid input is rejected at the boundary.

- **Scenario:** generate_planning_queue ORIENT, domain has no `set-roots`.
  - **Expected:** `code_search_roots` defaults to `["<domain>/"]` for that domain in the generated plan queue.
  - **Rationale:** The domain directory itself is the most common search root.

- **Scenario:** Architect edits `<state_dir>/plan-queue.json` during REFINE to reorder domains.
  - **Expected:** Advancing from REFINE validates the edited file. If valid, transitions to PHASE_SHIFT. Planning processes domains in the edited order.
  - **Rationale:** The architect controls domain ordering, not the scaffold.

- **Scenario:** Architect introduces invalid JSON in `<state_dir>/plan-queue.json` during REFINE.
  - **Expected:** Validation errors printed. State remains REFINE. Architect fixes and re-advances.
  - **Rationale:** The plan queue must be valid before transitioning to planning.

- **Scenario:** Advance from generate_planning_queue PHASE_SHIFT with `--from` override.
  - **Expected:** Override file used instead of the auto-generated `<state_dir>/plan-queue.json`. Validated and consumed.
  - **Rationale:** Last-chance override after reviewing the generated queue.

- **Scenario:** Advance from planning→implementing PHASE_SHIFT with invalid plan.json.
  - **Expected:** Validation errors printed. State remains PHASE_SHIFT.
  - **Rationale:** The implementing phase requires a structurally valid plan; errors caught here prevent failures during batch selection.

- **Scenario:** A plan with `kind: ui` reaches the planning→implementation PHASE_SHIFT.
  - **Expected:** The shift sets `phase` to `ui_implementing`. If the required `ui_implementing.app.*`/`ui_implementing.e2e.*` config keys are non-empty, the phase enters ORIENT; otherwise errors are printed and the state remains PHASE_SHIFT.
  - **Rationale:** UI domains need the QA/e2e loops, which require app-launch and test-runner config; missing config is caught at the boundary, not mid-phase.

- **Scenario:** `plan_all_before_implementing: true`, two domains with `kind: code` and `kind: ui` respectively.
  - **Expected:** Both planned (with a planning→planning domain-boundary shift between them), then implemented in queue order — the code domain via `implementing`, the UI domain via `ui_implementing`, with an implementation→implementation domain-boundary shift between them that re-routes by the next plan's `kind`.
  - **Rationale:** A single all-planning-first run interleaves code and UI domains; `kind` is read at each implementation entry.

- **Scenario:** `--guided` provided at PHASE_SHIFT advance.
  - **Expected:** `config.general.user_guided` updated before the phase transition proceeds.
  - **Rationale:** The guided setting is mutable on any advance, including phase shifts, so users can change behavior at natural boundaries.

- **Scenario:** `plan_all_before_implementing: false`, 2 domains. First domain planned and implemented.
  - **Expected:** Implementing DONE → PHASE_SHIFT (implementing → planning) → planning ORIENT for second domain.
  - **Rationale:** Interleaved mode cycles between planning and implementing per domain.

- **Scenario:** `plan_all_before_implementing: false`, 1 domain.
  - **Expected:** Planning ACCEPT → PHASE_SHIFT → implementing → DONE → session DONE. No implementing→planning transition.
  - **Rationale:** With one domain, there's no cycling — the flow is linear.

- **Scenario:** `plan_all_before_implementing: true`, 2 domains.
  - **Expected:** Planning domain A → PHASE_SHIFT (planning → planning) → planning domain B → DONE → PHASE_SHIFT (planning → implementing) → implementing domain A → PHASE_SHIFT (implementing → implementing) → implementing domain B → DONE → session DONE.
  - **Rationale:** All planning completes with domain-boundary PHASE_SHIFTs, then all implementing with domain-boundary PHASE_SHIFTs.

- **Scenario:** `plan_all_before_implementing: true`, 1 domain.
  - **Expected:** Planning → DONE → PHASE_SHIFT → implementing → DONE → session DONE. No intra-phase PHASE_SHIFTs.
  - **Rationale:** With one domain, no domain boundaries exist within either phase.

- **Scenario:** Implementing DONE with `plan_all_before_implementing: false` and planning queue empty.
  - **Expected:** Session DONE. No transition back to planning.
  - **Rationale:** All domains planned and implemented.

- **Scenario:** Implementing DONE with `plan_all_before_implementing: true` and implementing plan queue empty.
  - **Expected:** Session DONE.
  - **Rationale:** All plans implemented.

---

## Testing Criteria

### specifying PHASE_SHIFT without --from enters generate_planning_queue
- **Verifies:** Default flow enters the generation phase.
- **Given:** PHASE_SHIFT (specifying→generate_planning_queue).
- **When:** `advance` (no `--from`)
- **Then:** `phase: "generate_planning_queue"`, `state: "ORIENT"`. `<state_dir>/plan-queue.json` written.

### specifying PHASE_SHIFT with --from skips generate_planning_queue
- **Verifies:** Override skips generation entirely.
- **Given:** PHASE_SHIFT (specifying→generate_planning_queue).
- **When:** `advance --from custom-queue.json`
- **Then:** `phase: "planning"`, `state: "ORIENT"`. Plan queue matches the external file.

### specifying PHASE_SHIFT with invalid --from
- **Verifies:** Validation failure preserves state.
- **Given:** PHASE_SHIFT (specifying→generate_planning_queue).
- **When:** `advance --from invalid.json`
- **Then:** Errors printed. State remains PHASE_SHIFT.

### generate_planning_queue ORIENT auto-generates plan queue
- **Verifies:** Plan queue auto-generation from completed specs.
- **Given:** ORIENT (generate_planning_queue). 2 domains (optimizer with 3 specs, portal with 2 specs). Optimizer has `set-roots` configured. Portal does not.
- **When:** State entered.
- **Then:** `<state_dir>/plan-queue.json` has 2 entries. Optimizer entry has custom code_search_roots. Portal entry has `["portal/"]` default. Domain order matches spec queue order.

### generate_planning_queue auto-generation includes spec_commits
- **Verifies:** Commit hashes flow from completed specs to plan queue.
- **Given:** ORIENT (generate_planning_queue). Completed specs have commit_hashes `["7cede10", "8743b1d"]` across the domain.
- **When:** State entered.
- **Then:** Generated plan entry has `spec_commits: ["7cede10", "8743b1d"]` (deduplicated).

### generate_planning_queue REFINE validates on advance
- **Verifies:** Edited plan queue is validated before transitioning.
- **Given:** REFINE (generate_planning_queue). Architect edited `<state_dir>/plan-queue.json` with valid changes.
- **When:** `advance`
- **Then:** State is PHASE_SHIFT.

### generate_planning_queue REFINE rejects invalid plan queue
- **Verifies:** Validation failure keeps state at REFINE.
- **Given:** REFINE (generate_planning_queue). `<state_dir>/plan-queue.json` has invalid JSON.
- **When:** `advance`
- **Then:** Errors printed. State remains REFINE.

### generate_planning_queue PHASE_SHIFT transitions to planning
- **Verifies:** Plan queue consumed and planning begins.
- **Given:** PHASE_SHIFT (generate_planning_queue→planning).
- **When:** `advance`
- **Then:** `phase: "planning"`, `state: "ORIENT"`. Plan queue populated in state from `<state_dir>/plan-queue.json`.

### generate_planning_queue PHASE_SHIFT with --from override
- **Verifies:** Override file used instead of auto-generated file.
- **Given:** PHASE_SHIFT (generate_planning_queue→planning).
- **When:** `advance --from custom-queue.json`
- **Then:** `phase: "planning"`, `state: "ORIENT"`. Plan queue matches the override file.

### planning→implementing with valid plan
- **Verifies:** Successful phase transition with plan.json mutation.
- **Given:** PHASE_SHIFT (planning→implementing).
- **When:** `advance`
- **Then:** `phase: "implementing"`, `state: "ORIENT"`. plan.json items mutated.

### planning→implementing with invalid plan
- **Verifies:** Validation failure preserves state.
- **Given:** PHASE_SHIFT (planning→implementing), plan.json has errors.
- **When:** `advance`
- **Then:** Errors printed. State remains PHASE_SHIFT.

### planning→ui_implementing routing on kind: ui
- **Verifies:** `kind` routes the implementation phase shift.
- **Given:** PHASE_SHIFT (planning→implementation), active plan `kind: ui`, valid plan.json, required UI config keys non-empty.
- **When:** `advance`
- **Then:** `phase: "ui_implementing"`, `state: "ORIENT"`. plan.json items mutated.

### planning→ui_implementing rejects missing UI config
- **Verifies:** UI config validated at the routing boundary.
- **Given:** PHASE_SHIFT (planning→implementation), active plan `kind: ui`, `ui_implementing.e2e.test_command` empty.
- **When:** `advance`
- **Then:** Errors printed naming the missing key. State remains PHASE_SHIFT.

### planning→implementing routing on kind: code (default)
- **Verifies:** Absent/`code` kind routes to implementing.
- **Given:** PHASE_SHIFT (planning→implementation), active plan with no `kind` field.
- **When:** `advance`
- **Then:** `phase: "implementing"`, `state: "ORIENT"`.

### all-planning-first interleaves code and ui domains by kind
- **Verifies:** Invariant 11 across a domain boundary.
- **Given:** `plan_all_before_implementing: true`, implementation plan queue `[{kind: code}, {kind: ui}]`.
- **When:** First domain reaches DONE and the architect advances through the domain-boundary PHASE_SHIFT.
- **Then:** First domain used `implementing`; after the boundary shift, `phase: "ui_implementing"` for the second domain.

### ui_implementing→planning transition (interleaved mode)
- **Verifies:** ui_implementing DONE returns to planning when plans remain.
- **Given:** ui_implementing DONE, `plan_all_before_implementing: false`, planning queue has 1 plan.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered (`ui_implementing → planning`). After advancing: `phase: "planning"`, `state: "ORIENT"`.

### implementing→planning transition (interleaved mode)
- **Verifies:** Implementing DONE returns to planning when plans remain.
- **Given:** Implementing DONE, `plan_all_before_implementing: false`, planning queue has 1 plan remaining.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered. After advancing: `phase: "planning"`, `state: "ORIENT"`. Next plan pulled from queue.

### implementing→planning no transition when queue empty
- **Verifies:** Session DONE when no plans remain in interleaved mode.
- **Given:** Implementing DONE, `plan_all_before_implementing: false`, planning queue empty.
- **When:** `advance`
- **Then:** Session DONE.

### planning→planning domain boundary (all-planning-first mode)
- **Verifies:** PHASE_SHIFT between domains within planning phase.
- **Given:** Planning ACCEPT, `plan_all_before_implementing: true`, planning queue has 1 plan remaining.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered with "planning → planning (next domain)". After advancing: planning ORIENT for next domain.

### implementing→implementing domain boundary (all-planning-first mode)
- **Verifies:** PHASE_SHIFT between domains within implementing phase.
- **Given:** Implementing DONE, `plan_all_before_implementing: true`, implementing plan queue has 1 plan remaining.
- **When:** `advance`
- **Then:** PHASE_SHIFT entered with "implementing → implementing (next domain)". After advancing: implementing ORIENT for next domain.

### --guided at phase shift
- **Verifies:** Guided setting mutation at phase boundaries.
- **Given:** PHASE_SHIFT, `config.general.user_guided` is true.
- **When:** `advance --no-guided --from plans-queue.json`
- **Then:** `config.general.user_guided: false` after transition.

### Full Lifecycle: Specifying through generate_planning_queue
- **Verifies:** Complete specifying phase through plan queue generation.
- **Given:** Init with 2 specs, `specifying.batch: 2, specifying.eval.min_rounds: 1, specifying.eval.max_rounds: 3`.
- **When:** Complete both specs → DONE → RECONCILE → RECONCILE_EVAL(PASS) → COMPLETE → PHASE_SHIFT → generate_planning_queue ORIENT → REFINE → PHASE_SHIFT.
- **Then:** State is PHASE_SHIFT (generate_planning_queue→planning). Plan queue file exists at `<state_dir>/plan-queue.json`.

### Full Lifecycle: Interleaved (plan_all_before_implementing: false)
- **Verifies:** End-to-end lifecycle with interleaved planning/implementing, 2 domains.
- **Given:** Init with specs queue (2 domains). `plan_all_before_implementing: false`.
- **When:** Specifying → generate_planning_queue → Planning domain A → PHASE_SHIFT → Implementing domain A → DONE → PHASE_SHIFT → Planning domain B → PHASE_SHIFT → Implementing domain B → DONE.
- **Then:** Session DONE. All phase sections populated.

### Full Lifecycle: All planning first (plan_all_before_implementing: true)
- **Verifies:** End-to-end lifecycle with all planning before implementing, 2 domains.
- **Given:** Init with specs queue (2 domains). `plan_all_before_implementing: true`.
- **When:** Specifying → generate_planning_queue → Planning domain A → PHASE_SHIFT (planning→planning) → Planning domain B → DONE → PHASE_SHIFT (planning→implementing) → Implementing domain A → PHASE_SHIFT (implementing→implementing) → Implementing domain B → DONE.
- **Then:** Session DONE. All phase sections populated.

### Full Lifecycle: Skip generate_planning_queue with --from
- **Verifies:** `--from` at specifying PHASE_SHIFT skips generation.
- **Given:** Specifying PHASE_SHIFT reached.
- **When:** `advance --from plans-queue.json`
- **Then:** `phase: "planning"`, `state: "ORIENT"`. generate_planning_queue phase skipped.

### Full Lifecycle: Start at planning
- **Verifies:** Mid-entry lifecycle starting at planning phase.
- **Given:** `init --phase planning --from plans-queue.json`.
- **When:** Planning → PHASE_SHIFT → Implementing → DONE.
- **Then:** Specifying and generate_planning_queue sections are null. Planning and implementing complete.

### Full Lifecycle: Start at implementing
- **Verifies:** Single-phase lifecycle starting at implementing.
- **Given:** `init --phase implementing --from plan.json`.
- **When:** Implementing → DONE.
- **Then:** Specifying, generate_planning_queue, and planning sections are null. Implementing complete.

---

## Implements
- Explicit PHASE_SHIFT checkpoints between phases and at domain boundaries with context refresh
- generate_planning_queue phase: auto-generates plan queue, writes to `<state_dir>/plan-queue.json`, architect reviews/reorders before planning
- `--from` override at specifying PHASE_SHIFT to skip generate_planning_queue entirely
- `--from` override at generate_planning_queue PHASE_SHIFT for last-chance plan queue replacement
- Plan validation and mutation at planning→implementation boundary
- Plan `kind` routing: planning→implementing (`code`) vs planning→ui_implementing (`ui`), with UI config validated at the boundary
- Interleaved mode (`plan_all_before_implementing: false`): plan-implement-plan-implement per domain with implementation→planning transitions, for both implementing and ui_implementing
- All-planning-first mode (`plan_all_before_implementing: true`): all planning with domain-boundary PHASE_SHIFTs, then all implementation with domain-boundary PHASE_SHIFTs that re-route by `kind`
- Full lifecycle integration across phase and domain boundaries
