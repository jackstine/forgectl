# Plan Production

## Topic of Concern
> The scaffold produces a validated implementation plan through a study-draft-evaluate cycle.

## Context

The planning phase guides the architect through studying specs, codebase, and packages, then drafting an implementation plan as structured JSON (`plan.json`) with accompanying notes. The plan is validated against a structural schema and evaluated through iterative sub-agent review rounds until accepted (or force-accepted at max rounds).

## Depends On
- **phase-transitions** — the generate_planning_queue→planning phase shift provides the plans queue and triggers ORIENT.
- **session-init** — alternatively, `init --phase planning` starts the planning phase directly.
- **state-persistence** — reads and writes the state file.

## Integration Points

| Spec | Relationship |
|------|-------------|
| SPEC_MANIFEST.md | STUDY_SPECS reads this manifest to locate spec files relevant to the plan |
| Plan format definition (`PLAN_FORMAT.md`) | Defines the JSON schema for `plan.json` and conventions for notes files. Referenced during REVIEW, DRAFT, and validation. |
| Plan evaluator prompt (`evaluators/plan-eval.md`, embedded in binary) | Full instructions for the planning evaluation sub-agent: dimensions, report format, verdict rules |
| phase-transitions | ACCEPT transitions to PHASE_SHIFT (planning → implementing) |

---

## Interface

### Inputs

#### `advance` flags — Planning Phase

| State | Flags |
|-------|-------|
| EVALUATE | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `eval_mode: "report"`) |
| ACCEPT | `--message <text>` / `-m` (required when `enable_commits: true`) |

#### `eval` command

| Command | Flags | Description |
|---------|-------|-------------|
| `eval` | none | Output full evaluation context for the sub-agent. Only valid in planning EVALUATE. |

### Outputs

#### `advance` output

**Entering ORIENT** (after generate_planning_queue PHASE_SHIFT or init with `--phase planning`):

```
State:   ORIENT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Action:  After completion of the above, advance to begin studying specs.
```

**Entering STUDY_SPECS** (after ORIENT):

```
State:   STUDY_SPECS
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Specs:   launcher/specs/service-configuration.md, ...
Commits: abc1234, def5678
Roots:   launcher/, api/
Action:  Study the specs: launcher/specs/service-configuration.md, ...
         Review git diffs for spec commits: abc1234, def5678.
         After completion of the above, advance to continue.
```

**Entering STUDY_CODE** (after STUDY_SPECS):

```
State:   STUDY_CODE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Roots:   launcher/, api/
Specs:   launcher/specs/service-configuration.md
         launcher/specs/config-validation.md
Action:  Please spawn 3 haiku explore sub-agents to explore the codebase.
         Search roots: launcher/, api/.
         Focus: find code relevant to the specs listed above.
         After completion of the above, advance to continue.
```

**Entering STUDY_PACKAGES** (after STUDY_CODE):

```
State:   STUDY_PACKAGES
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Action:  Study the packages and libraries needed to implement the specs under study.
         Use git diffs for spec commits: abc1234, def5678.
         Identify relevant packages from: package manifests, CLAUDE.md references, library docs.
         For each package, write a notes file with code examples relevant to the specs.
         Output: notes/ at launcher/.forgectl_workspace/implementation_plan/
         After completion of the above, advance to continue.
```

**Entering REVIEW** (after STUDY_PACKAGES, only when `user_guided: true`):

```
State:   REVIEW
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Notes:   launcher/.forgectl_workspace/implementation_plan/notes/
Action:  Review the package notes for completeness.
         Plan format: PLAN_FORMAT.md
         STOP please review and discuss with user before continuing.
         After completion of the above, advance to begin drafting.
```

**Entering DRAFT** (after REVIEW):

```
State:   DRAFT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Notes:   launcher/.forgectl_workspace/implementation_plan/notes/
Action:  Draft the implementation plan using the package notes as reference.
         Use git diffs for spec commits: abc1234, def5678.
         Output: plan.json at launcher/.forgectl_workspace/implementation_plan/
         Update notes as needed.
         Format: PLAN_FORMAT.md
         After completion of the above, advance to validate.
```

**Entering VALIDATE** (after DRAFT or REFINE, when validation fails):

```
State:   VALIDATE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Action:  Plan validation failed. Fix the plan.
         After completion of the above, advance to re-validate.
         Format: PLAN_FORMAT.md

FAIL: 3 errors in plan.json

  items[2]: missing required field "depends_on"
    depends_on (string[]): Item IDs that must be complete before this item can begin.

  items[5]: unexpected field "status"
    status is not a valid field. Item status is computed from tests, not stored.

  layers[1].items[3]: references non-existent item "config.typez"
    Layer items must reference valid item IDs from the items array.
```

**Entering SELF_REVIEW** (after VALIDATE passes, when `planning.self_review: true`):

```
State:   SELF_REVIEW
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Specs:   launcher/specs/service-configuration.md
         launcher/specs/config-validation.md
Notes:   launcher/.forgectl_workspace/implementation_plan/notes/
Action:  Review your plan against the specs and your study notes.
         Verify coverage, dependency ordering, and layer structure.
         Revise plan.json and notes as needed before evaluation.
         After completion of the above, advance to continue.
```

**Entering EVALUATE** (after SELF_REVIEW, or after VALIDATE passes when `planning.self_review: false`, `eval_mode: "report"`):

```
State:   EVALUATE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Please spawn 1 opus eval sub-agent to evaluate the plan.
         Sub-agent runs: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>
```

**Entering EVALUATE** (`eval_mode: "direct"`):

```
State:   EVALUATE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Please spawn 1 opus eval sub-agent to evaluate and correct the plan.
         Sub-agent runs: forgectl eval
         Plan files have been staged. Sub-agent makes corrections directly.
         After completion of the above, advance with --verdict PASS|FAIL
```

**Entering EVALUATE** (`eval_mode: "conversational"`):

```
State:   EVALUATE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Please spawn 1 opus eval sub-agent to evaluate the plan.
         Sub-agent runs: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL
```

**Entering REFINE** (after EVALUATE with FAIL verdict, `eval_mode: "report"`):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Eval:    launcher/.forgectl_workspace/implementation_plan/evals/round-1.md
Action:  Study the eval file "launcher/.forgectl_workspace/implementation_plan/evals/round-1.md"
         and implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         After completion of the above, advance to continue.
```

**Entering REFINE** (after EVALUATE with FAIL verdict, `eval_mode: "direct"`):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Review unstaged changes from the evaluator (git diff).
         Accept, revise, or revert corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         After completion of the above, advance to continue.
```

**Entering REFINE** (after EVALUATE with FAIL verdict, `eval_mode: "conversational"`):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Make corrections based off communication with the evaluator.
         Implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         After completion of the above, advance to continue.
```

**Entering REFINE** (after EVALUATE with PASS verdict, below min_rounds, `eval_mode: "report"`):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Eval:    launcher/.forgectl_workspace/implementation_plan/evals/round-1.md
Action:  Minimum evaluation rounds not met.
         Study the eval file "launcher/.forgectl_workspace/implementation_plan/evals/round-1.md"
         and implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         After completion of the above, advance to continue.
```

**Entering REFINE** (after EVALUATE with PASS verdict, below min_rounds, `eval_mode: "direct"`):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Minimum evaluation rounds not met.
         Review unstaged changes from the evaluator (git diff).
         Accept, revise, or revert corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         After completion of the above, advance to continue.
```

**Entering REFINE** (after EVALUATE with PASS verdict, below min_rounds, `eval_mode: "conversational"`):

```
State:   REFINE
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3
Action:  Minimum evaluation rounds not met.
         Make corrections based off communication with the evaluator.
         Implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         After completion of the above, advance to continue.
```

**Entering ACCEPT** (after EVALUATE with PASS verdict, at or above min_rounds):

When `enable_commits: true`:

```
State:   ACCEPT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   2/3
Action:  Plan accepted.
         Advance with --message "your commit message" to commit and continue.
```

When `enable_commits: false`:

```
State:   ACCEPT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   2/3
Action:  Plan accepted.
         After completion of the above, advance to continue.
```

**Entering ACCEPT** (forced, after EVALUATE with FAIL verdict at max_rounds):

```
State:   ACCEPT
Phase:   planning
Plan:    Service Configuration
Domain:  launcher
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   3/3
Action:  Plan accepted (max rounds reached).
         After completion of the above, advance to continue.
```

**Entering DONE** (after last plan accepted, queue empty):

```
State:   DONE
Phase:   planning
Summary:
  optimizer: Optimizer Implementation Plan — accepted (2 rounds)
  portal:    Portal Implementation Plan — accepted (1 round)
  Total:     2 plans, 3 eval rounds
Action:  All plans complete. Advance to continue.
```

#### `eval` output

When `eval_mode: "report"`:

```
=== PLAN EVALUATION ROUND 1/3 ===
Plan:   Service Configuration
Domain: launcher
File:   launcher/.forgectl_workspace/implementation_plan/plan.json

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/plan-eval.md>

--- PLAN REFERENCES ---

Plan:    launcher/.forgectl_workspace/implementation_plan/plan.json
Format:  PLAN_FORMAT.md
Specs:
  - launcher/specs/service-configuration.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.forgectl_workspace/implementation_plan/evals/round-1.md
```

Subsequent rounds with `eval_mode: "report"` include previous evaluations:

```
=== PLAN EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: FAIL — launcher/.forgectl_workspace/implementation_plan/evals/round-1.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  launcher/.forgectl_workspace/implementation_plan/evals/round-2.md
```

When `eval_mode: "direct"`, the `--- REPORT OUTPUT ---` section is included but instructs the evaluator to make corrections directly rather than write a report file. The `--- PREVIOUS EVALUATIONS ---` section is included on subsequent rounds.

```
=== PLAN EVALUATION ROUND 1/3 ===
Plan:   Service Configuration
Domain: launcher
File:   launcher/.forgectl_workspace/implementation_plan/plan.json

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/plan-eval.md>

--- PLAN REFERENCES ---

Plan:    launcher/.forgectl_workspace/implementation_plan/plan.json
Format:  PLAN_FORMAT.md
Specs:
  - launcher/specs/service-configuration.md

--- REPORT OUTPUT ---

Make corrections directly to the plan files.
```

Subsequent rounds with `eval_mode: "direct"`:

```
=== PLAN EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: FAIL

--- REPORT OUTPUT ---

Make corrections directly to the plan files.
```

When `eval_mode: "conversational"`, the `--- REPORT OUTPUT ---` and `--- PREVIOUS EVALUATIONS ---` sections are omitted. The eval sub-agent receives plan references and evaluator instructions but does not write a file. It communicates its verdict directly to the architect.

```
=== PLAN EVALUATION ROUND 1/3 ===
Plan:   Service Configuration
Domain: launcher
File:   launcher/.forgectl_workspace/implementation_plan/plan.json

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/plan-eval.md>

--- PLAN REFERENCES ---

Plan:    launcher/.forgectl_workspace/implementation_plan/plan.json
Format:  PLAN_FORMAT.md
Specs:
  - launcher/specs/service-configuration.md
```

Subsequent rounds with `eval_mode: "conversational"`:

```
=== PLAN EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: FAIL
```

#### Eval Report Locations

Planning eval reports:
```
<domain>/.forgectl_workspace/implementation_plan/evals/round-N.md
```

#### `status` output — Planning (compact)

The compact `status` output for planning shows the current plan, round, action, and progress:

When `eval_mode: "report"`:

```
Plan:    Service Configuration (launcher)
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3

Action:  Run evaluation sub-agent against the plan (round 1/3).
         Sub-agent: forgectl eval
         Advance with --verdict PASS|FAIL --eval-report <path>.

Progress: round 1 of 3
```

When `eval_mode: "direct"` or `eval_mode: "conversational"`:

```
Plan:    Service Configuration (launcher)
File:    launcher/.forgectl_workspace/implementation_plan/plan.json
Round:   1/3

Action:  Run evaluation sub-agent against the plan (round 1/3).
         Sub-agent: forgectl eval
         Advance with --verdict PASS|FAIL.

Progress: round 1 of 3
```

#### `status --verbose` output — Planning section

With `--verbose`, the eval history is appended.

When `eval_mode: "report"`:

```
--- Planning ---

  Accepted (2 rounds)
    Round 1: FAIL — launcher/.forgectl_workspace/implementation_plan/evals/round-1.md
    Round 2: PASS — launcher/.forgectl_workspace/implementation_plan/evals/round-2.md
```

When `eval_mode: "direct"` or `eval_mode: "conversational"`:

```
--- Planning ---

  Accepted (2 rounds)
    Round 1: FAIL
    Round 2: PASS
```

When no evals yet:

```
--- Planning ---

  Evals: (none yet)
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance` in planning EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in planning EVALUATE without `--eval-report` when `eval_mode: "report"` | Error. Exit code 1. | Every evaluation must reference its report when in report mode |
| `advance --eval-report` pointing to non-existent file | Error naming the path. Exit code 1. | Report must exist to be recorded |
| `advance --eval-report` when `eval_mode` is not `"report"` | Warning: `--eval-report is ignored, --eval-report is only used in report mode`. Command proceeds. | Consistent with `--message` warning pattern |
| `advance` in planning ACCEPT without `--message` when `enable_commits: true` | Error. Exit code 1. | Accepted plans need a commit message when commits are enabled |
| `eval` outside of planning EVALUATE | Error naming current state and phase. Exit code 1. | Eval context only available in EVALUATE |
| `advance` in DONE with any flags | Error: "DONE is a pass-through state. No flags accepted." Exit code 1. | DONE only transitions to PHASE_SHIFT |

---

## Behavior

### State Machine

```
ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW* → DRAFT
                                                                  │
                                                        ┌─────────┴─────────┐
                                                   plan valid          plan invalid
                                                        │                   │
                                                        ▼                   ▼
                                                  SELF_REVIEW*         VALIDATE
                                                        │                   │
                                                        ▼             fix + advance
                                                   EVALUATE                │
                                                        │                   │
                                              ┌─────────┼─────────┐        │
                                              │         │         │        │
                                        PASS ≥ min  PASS < min  FAIL < max │
                                              │         │         │        │
                                              ▼         ▼         ▼        │
                                           ACCEPT    REFINE    REFINE ◄────┘

* SELF_REVIEW only entered when planning.self_review is true. Otherwise skipped.
* REVIEW only entered when config.general.user_guided is true. Otherwise skipped.
                                              │         │         │
                                         ┌────┘         └────┬────┘
                                         │                   │
                                   PHASE_SHIFT          plan valid → EVALUATE
                                         │              plan invalid → VALIDATE
                              ┌──────────┴──────────┐   FAIL ≥ max → ACCEPT (forced)
                              │                     │
                    plan_all: false        plan_all: true
                              │                     │
                              ▼              queue empty?
                     implementing             yes → DONE → PHASE_SHIFT → implementing
                                              no → PHASE_SHIFT → planning (next domain)
```

### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | always | STUDY_SPECS | — |
| STUDY_SPECS | always | STUDY_CODE | — |
| STUDY_CODE | always | STUDY_PACKAGES | — |
| STUDY_PACKAGES | `user_guided: true` | REVIEW | — |
| STUDY_PACKAGES | `user_guided: false` | DRAFT | — |
| REVIEW | always | DRAFT | — |
| DRAFT | plan.json valid, `self_review: true` | SELF_REVIEW | Set round to 1. Two transitions in one advance. |
| DRAFT | plan.json valid, `self_review: false` | EVALUATE | Set round to 1. Two transitions in one advance. |
| DRAFT | plan.json invalid | VALIDATE | Set round to 1. Print errors. |
| VALIDATE | plan.json valid, `self_review: true` | SELF_REVIEW | — |
| VALIDATE | plan.json valid, `self_review: false` | EVALUATE | — |
| VALIDATE | plan.json invalid | _(stays VALIDATE)_ | Print errors. Exit code 1. |
| SELF_REVIEW | plan.json valid | EVALUATE | Validation gate runs on advance (agent may have revised). |
| SELF_REVIEW | plan.json invalid | VALIDATE | Agent broke plan during revision. Print errors. |
| EVALUATE | `--verdict PASS`, round >= `planning.eval.min_rounds` | ACCEPT | Record eval. |
| EVALUATE | `--verdict PASS`, round < `planning.eval.min_rounds` | REFINE | Record eval. Min rounds not met. |
| EVALUATE | `--verdict FAIL`, round < `planning.eval.max_rounds` | REFINE | Record eval. |
| EVALUATE | `--verdict FAIL`, round >= `planning.eval.max_rounds` | ACCEPT | Record eval. Forced acceptance. |
| REFINE | plan.json valid, `self_review: true` | SELF_REVIEW | Increment round. Two transitions in one advance. |
| REFINE | plan.json valid, `self_review: false` | EVALUATE | Increment round. Two transitions in one advance. |
| REFINE | plan.json invalid | VALIDATE | Increment round. Print errors. |
| ACCEPT | `enable_commits: true` | _(commit then continue)_ | Stage files per `planning.commit_strategy`, run `git commit -m <message>`, register hash. |
| ACCEPT | `plan_all_before_implementing: true`, queue non-empty | PHASE_SHIFT | Move plan to completed. PHASE_SHIFT (planning → planning, domain boundary). |
| ACCEPT | `plan_all_before_implementing: true`, queue empty | DONE | Move plan to completed. |
| ACCEPT | `plan_all_before_implementing: false` | PHASE_SHIFT | Move plan to completed. PHASE_SHIFT (planning → implementing). |
| DONE | always (`plan_all_before_implementing: true` only) | PHASE_SHIFT | Set phase shift from planning → implementing. Populate implementing plan queue from completed plans. |

### Study Phases

Three study phases build context before drafting. No flags required — the architect studies, then advances.

#### STUDY_SPECS
Study the specs listed in `current_plan.specs` and the SPEC_MANIFEST.md: full spec files, git diffs, dependencies, cross-references. When `SpecCommits` is empty, the `Commits:` line and commit hash references in the action text are omitted.

#### STUDY_CODE
Explore the codebase using sub-agents within `current_plan.code_search_roots`, focused on finding code relevant to the specs in `current_plan.specs`. The output lists both the search roots and the spec file paths so sub-agents know what to look for. Sub-agent count is configured via `planning.study_code.count` (default 3).

#### STUDY_PACKAGES
Study the packages and libraries needed to implement the specs under study. Identify relevant packages from package manifests, CLAUDE.md references, and library docs. For each package, write a notes file with code examples relevant to the specs. Notes are written to `<domain>/<workspace_dir>/implementation_plan/notes/`. When `SpecCommits` is non-empty, the commit hashes are included in the action text so the architect can use git diffs to understand spec changes. When `SpecCommits` is empty, the commit hash line is omitted.

### REVIEW Phase

Checkpoint before drafting, entered only when `config.general.user_guided` is `true`. The architect reviews the package notes for completeness and discusses with the user before advancing to DRAFT. When `user_guided` is `false`, STUDY_PACKAGES transitions directly to DRAFT and REVIEW is skipped.

### DRAFT Phase

The architect drafts the implementation plan as structured JSON, using the package notes created during STUDY_PACKAGES as reference. Notes are updated as needed during drafting. When `SpecCommits` is non-empty, the commit hashes are included in the action text. The output structure:

```
<domain>/.forgectl_workspace/implementation_plan/
├── plan.json
└── notes/
    ├── <package>.md
    └── ...
```

### Validation Gate

Fires automatically when advancing from DRAFT, REFINE, or SELF_REVIEW. Not a phase where the architect does work.

#### Validation Checks

| Check | Description |
|-------|-------------|
| JSON parse | File exists and contains valid JSON |
| Top-level fields | `context`, `refs`, `layers`, `items` present and correctly typed |
| Context fields | `domain` and `module` are non-empty strings |
| Refs exist | Every path in `refs` resolves to an existing file |
| Item schema | Every item has `id`, `name`, `description`, `depends_on`, `specs`, `refs`, `files`, `tests` |
| Item ID uniqueness | No duplicate item IDs |
| Layer coverage | Every item in exactly one layer; every layer item ID exists |
| Layer ordering | Items only depend on items in equal or earlier layers |
| DAG validity | `depends_on` references are valid; no cycles |
| Test schema | Every test has `category`, `description`, `passes` with correct types |
| Test categories | One of: `functional`, `rejection`, `edge_case` |
| Notes files | Every path in `items[].refs` resolves to an existing notes file (relative to plan.json directory) |

**On pass:** transitions directly to EVALUATE. VALIDATE is never visible.

**On fail:** enters VALIDATE, prints errors with field descriptions. Loops until valid.

### EVALUATE Phase

Uses `eval` command to output full evaluation context for the sub-agent. The evaluator prompt (`evaluators/plan-eval.md`) defines 11 assessment dimensions.

### REFINE Phase

Action varies by verdict and `planning.eval.eval_mode`:
- When `eval_mode: "report"`: outputs the eval report path with "Study the eval file" instruction.
- When `eval_mode: "direct"`: "Review unstaged changes from the evaluator (git diff)."
- When `eval_mode: "conversational"`: "Make corrections based off communication with the evaluator."
- After PASS below min_rounds: prefixed with "Minimum evaluation rounds not met."

Advancing from REFINE runs the validation gate.

---

## Invariants

1. **Study phases are sequential.** STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES. No study phase is skipped. REVIEW is entered after STUDY_PACKAGES only when `user_guided` is `true`; otherwise STUDY_PACKAGES transitions directly to DRAFT.
2. **Validation precedes evaluation.** The validation gate runs before every EVALUATE entry (and before SELF_REVIEW when enabled).
3. **Self-review is optional.** When `planning.self_review` is `true`, SELF_REVIEW is entered between validation and EVALUATE on every round. When `false` (default), SELF_REVIEW is skipped. The validation gate also runs on advance from SELF_REVIEW in case the agent revised plan.json.
4. **Round monotonicity.** The planning round counter only increments.
5. **Min rounds enforced.** PASS below `planning.eval.min_rounds` forces another cycle.
6. **Max rounds enforced.** FAIL at `planning.eval.max_rounds` forces acceptance.
7. **REVIEW is guided-only.** REVIEW is only entered when `config.general.user_guided` is `true`. REVIEW output always includes "STOP please review and discuss with user before continuing."
8. **Per-plan commits at ACCEPT.** When `enable_commits` is `true`, `--message` is required at ACCEPT and the scaffold auto-commits per `planning.commit_strategy`. When `enable_commits` is `false`, `--message` is not shown in output; if provided, a warning is printed: `--message is ignored, commits are not enabled`. The warning does not instruct how to enable commits. The commit hash is automatically registered on the accepted plan.
9. **Batch limitation.** `planning.batch` > 1 is not yet supported. Reserved for future use.
10. **DONE only reachable when `plan_all_before_implementing: true`.** When `false`, ACCEPT always transitions to PHASE_SHIFT (planning → implementing) — DONE is never entered. When `true`, DONE is entered when the queue is empty.
11. **No plan addition at DONE.** Unlike specifying's DONE which allows `add-queue-item`, planning's DONE does not accept new plans. The rationale is that the `plan-queue.json` is the authoritative source for all plans that need to be created in this session.
12. **Domain boundaries are phase shifts.** When `plan_all_before_implementing: true`, PHASE_SHIFT fires between domains within planning. This ensures context refresh when switching codebases.
13. **Path resolution is context-dependent.** `refs[].path` and `items[].refs` are resolved relative to the plan.json directory (`<domain>/<workspace_dir>/implementation_plan/`). `items[].files` and `items[].specs` are resolved relative to the project root.

---

## Edge Cases

- **Scenario:** Validation passes on first try after DRAFT, `self_review: false`.
  - **Expected:** Transitions directly to EVALUATE in one `advance` call.
  - **Rationale:** VALIDATE is a gate, not a user-facing state. When the plan is valid and self-review is disabled, the architect never sees VALIDATE.

- **Scenario:** Validation passes on first try after DRAFT, `self_review: true`.
  - **Expected:** Transitions to SELF_REVIEW in one `advance` call.
  - **Rationale:** Self-review gives the agent a chance to revise before evaluation.

- **Scenario:** Agent revises plan.json during SELF_REVIEW and introduces validation errors.
  - **Expected:** Advancing from SELF_REVIEW runs the validation gate. Validation fails, enters VALIDATE.
  - **Rationale:** The validation gate always runs before EVALUATE, even after self-review.

- **Scenario:** Validation fails after REFINE.
  - **Expected:** Enters VALIDATE loop.
  - **Rationale:** Refinement may introduce structural errors; the validation gate catches them before re-evaluation.

- **Scenario:** plan.json does not exist when validation runs.
  - **Expected:** Validation fails with file-not-found.
  - **Rationale:** The plan must physically exist to be validated. Missing file is the first check.

- **Scenario:** plan.json has a dependency cycle.
  - **Expected:** Validation fails listing the cycle.
  - **Rationale:** Cycles make layer ordering impossible. The DAG check detects and reports them.

- **Scenario:** `--eval-report` points to non-existent file.
  - **Expected:** Error. State unchanged.
  - **Rationale:** Eval reports are audit artifacts. Accepting a reference to a non-existent file would leave a broken trail.

- **Scenario:** ACCEPT with `enable_commits: true` and no files changed.
  - **Expected:** Commit skipped. Notice printed. Advance proceeds.
  - **Rationale:** Empty commits are not created. See `docs/auto-committing.md`.

- **Scenario:** ACCEPT with `enable_commits: false` and `--message` provided.
  - **Expected:** Warning: `--message is ignored, commits are not enabled`. Command proceeds. The warning does not instruct how to enable commits.
  - **Rationale:** Users who do not need auto-commits should not be confused or prompted to change configuration.

- **Scenario:** Multiple plans in queue (future: `planning.batch` > 1).
  - **Expected:** ACCEPT loops to ORIENT for next plan. DONE reached when queue empty.
  - **Rationale:** Each plan is an independent domain artifact. Per-plan commits at ACCEPT keep the git history clean.

---

## Testing Criteria

### Study phases advance sequentially
- **Verifies:** Sequential state progression through study phases.
- **Given:** ORIENT (planning), `user_guided: true`.
- **When:** advance through STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT.
- **Then:** Each transitions in order.

### STUDY_PACKAGES skips REVIEW when not guided
- **Verifies:** REVIEW is conditional on user_guided.
- **Given:** STUDY_PACKAGES, `user_guided: false`.
- **When:** `advance`
- **Then:** State is DRAFT. REVIEW is never entered.

### DRAFT with valid plan goes to EVALUATE when self_review is false
- **Verifies:** Validation gate passes transparently, self-review skipped.
- **Given:** DRAFT, plan.json valid, `planning.self_review: false`.
- **When:** `advance`
- **Then:** State is EVALUATE. Round is 1.

### DRAFT with valid plan goes to SELF_REVIEW when self_review is true
- **Verifies:** Self-review entered after validation passes.
- **Given:** DRAFT, plan.json valid, `planning.self_review: true`.
- **When:** `advance`
- **Then:** State is SELF_REVIEW. Round is 1.

### SELF_REVIEW with valid plan goes to EVALUATE
- **Verifies:** Self-review advances to evaluation.
- **Given:** SELF_REVIEW, plan.json valid.
- **When:** `advance`
- **Then:** State is EVALUATE.

### SELF_REVIEW with invalid plan enters VALIDATE
- **Verifies:** Validation gate catches errors after self-review revision.
- **Given:** SELF_REVIEW, agent revised plan.json and introduced errors.
- **When:** `advance`
- **Then:** State is VALIDATE.

### DRAFT with invalid plan enters VALIDATE
- **Verifies:** Validation gate catches errors.
- **Given:** DRAFT, plan.json invalid.
- **When:** `advance`
- **Then:** State is VALIDATE.

### VALIDATE loops until valid
- **Verifies:** VALIDATE rejects until plan is fixed.
- **Given:** VALIDATE, plan.json still invalid.
- **When:** `advance`
- **Then:** Stays VALIDATE. Exit code 1.

### Planning EVALUATE PASS at min_rounds → ACCEPT
- **Verifies:** PASS with sufficient rounds triggers acceptance.
- **Given:** EVALUATE, min_rounds: 1, round 1.
- **When:** `advance --verdict PASS --eval-report evals/round-1.md`
- **Then:** State is ACCEPT.

### Planning EVALUATE FAIL at max_rounds → ACCEPT (forced)
- **Verifies:** FAIL at max rounds forces acceptance.
- **Given:** EVALUATE, max_rounds: 2, round 2.
- **When:** `advance --verdict FAIL --eval-report evals/round-2.md`
- **Then:** State is ACCEPT.

### Planning ACCEPT → DONE (queue empty)
- **Verifies:** Acceptance with empty queue reaches DONE.
- **Given:** ACCEPT (planning), queue empty.
- **When:** `advance`
- **Then:** State is DONE.

### Planning DONE → PHASE_SHIFT
- **Verifies:** DONE transitions to phase shift.
- **Given:** DONE (planning).
- **When:** `advance`
- **Then:** State is PHASE_SHIFT.

### Planning ACCEPT auto-commits when enable_commits is true
- **Verifies:** Per-plan commit at ACCEPT.
- **Given:** ACCEPT, `enable_commits: true`, `planning.commit_strategy: "strict"`. Plan files modified.
- **When:** `advance --message "Accept service configuration plan"`
- **Then:** Files staged per `strict` strategy. `git commit` executed. Hash registered on accepted plan. State is DONE.

### Planning ACCEPT ignores --message when enable_commits is false
- **Verifies:** Warning printed, command proceeds.
- **Given:** ACCEPT, `enable_commits: false`.
- **When:** `advance --message "ignored"`
- **Then:** Warning: `--message is ignored, commits are not enabled`. State is DONE. Warning does not instruct how to enable commits.

### Planning eval command outputs context
- **Verifies:** Eval command assembles full evaluation context.
- **Given:** EVALUATE (planning), round 1.
- **When:** `forgectl eval`
- **Then:** Output includes plan-eval.md contents, plan references, report target.

---

## Implements
- Planning phase: structured study → draft → validate → self-review (optional) → evaluate → accept
- Plan validation gate with 12 structural checks
- Eval round enforcement (`planning.eval.min_rounds`/`max_rounds`) with forced acceptance
- Per-plan auto-commit at ACCEPT with staging per `planning.commit_strategy` and automatic hash registration
- DONE state after all plans accepted (no plan addition at DONE — plan-queue.json is authoritative)
- Domain artifacts in `.forgectl_workspace/`
- Dual evaluator prompts: plan-eval.md for planning sub-agent
- `planning.batch` > 1: reserved for future use
