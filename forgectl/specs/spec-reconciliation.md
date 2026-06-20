# Spec Reconciliation

## Topic of Concern
> The scaffold cross-validates completed specs for cross-domain dependency and integration consistency.

## Context

After all individual specs are completed and all domains have passed cross-referencing (DONE state from spec-lifecycle), the scaffold enters a reconciliation phase. This phase cross-validates dependencies and integration points across all specs and all domains to ensure they are symmetric, consistent, and free of circular references.

Domain-scoped cross-referencing (intra-domain consistency) is handled by CROSS_REFERENCE in spec-lifecycle. Reconciliation focuses on cross-domain concerns: dependencies between specs in different domains, integration point symmetry across domain boundaries, and global naming consistency.

Reconciliation has its own round configuration independent of the per-spec eval rounds. `specifying.reconciliation.min_rounds` (default 0) controls how many rounds must pass before a PASS verdict can complete the phase. `specifying.reconciliation.max_rounds` (default 3) caps the total reconciliation rounds to prevent infinite loops. When `specifying.reconciliation.min_rounds` is 0, a single PASS immediately completes — this is the default behavior.

When `enable_commits` is `true`, the single specifying commit occurs at COMPLETE. The scaffold stages files using the configured `specifying.commit_strategy` (default: `all-specs`) and runs `git commit -m <message>`. The resulting hash is automatically registered on all completed specs. See `docs/auto-committing.md` for full auto-commit behavior and staging strategies.

The scaffold does not spawn sub-agents. It outputs instructions telling the architect what to spawn. The architect (or the skill driving the session) is responsible for spawning them.

## Depends On
- **spec-lifecycle** — provides the completed specs list; DONE state triggers reconciliation.
- **state-persistence** — reads and writes the state file.

## Integration Points

| Spec | Relationship |
|------|-------------|
| phase-transitions | COMPLETE state transitions to PHASE_SHIFT (specifying → generate_planning_queue) |
| Reconciliation evaluation prompt (`evaluators/reconcile-eval.md`, embedded in binary) | Full instructions for the reconciliation evaluation sub-agent: cross-domain consistency checks, report format, verdict rules |
| Reconciliation eval sub-agent | Runs `git diff --staged` and evaluates cross-domain consistency across all specs |

---

## Interface

### Inputs

#### `advance` flags — Reconciliation

| State | Flags |
|-------|-------|
| RECONCILE_EVAL | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `eval_mode: "report"`) |
| RECONCILE_REVIEW | (no flags) |
| COMPLETE | `--message <text>` / `-m` (required when `enable_commits: true`) |

#### `eval` command

| Command | Flags | Description |
|---------|-------|-------------|
| `eval` | none | Output full evaluation context for the sub-agent. Only valid in RECONCILE_EVAL. |

### Outputs

#### `advance` output

**Entering RECONCILE** (after DONE):

```
State:   RECONCILE
Phase:   specifying
Specs:   5 completed across 2 domains
Action:  Cross-validate all specs across domains: verify Depends On entries,
         Integration Points symmetry, naming consistency. Stage changes with git add.
         After completion of the above, advance to begin evaluation.
```

**Entering RECONCILE_EVAL** (after RECONCILE, `eval_mode: "report"`):

```
State:   RECONCILE_EVAL
Phase:   specifying
Round:   1/3
Action:  Please spawn 1 sonnet general-purpose sub-agent to evaluate cross-domain reconciliation.
         The sub-agent should run: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>
```

**Entering RECONCILE_EVAL** (after RECONCILE, `eval_mode: "direct"`):

```
State:   RECONCILE_EVAL
Phase:   specifying
Round:   1/3
Action:  Please spawn 1 sonnet general-purpose sub-agent to evaluate and correct the reconciliation.
         Sub-agent runs: forgectl eval
         Spec files have been staged. Sub-agent makes corrections directly.
         After completion of the above, advance with --verdict PASS|FAIL
```

**Entering RECONCILE_EVAL** (after RECONCILE, `eval_mode: "conversational"`):

```
State:   RECONCILE_EVAL
Phase:   specifying
Round:   1/3
Action:  Please spawn 1 sonnet general-purpose sub-agent to evaluate cross-domain reconciliation.
         The sub-agent should run: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL
```

**Entering RECONCILE_REVIEW** (after first RECONCILE_EVAL PASS >= min_rounds):

RECONCILE_REVIEW always fires once after the first passing reconciliation eval. The action output varies based on `specifying.reconciliation.user_review`:

When `user_review: true`:

```
State:   RECONCILE_REVIEW
Phase:   specifying
Specs:   5 completed across 2 domains
Round:   1/3
Verdict: PASS

Action:  STOP please review and discuss with user before continuing.
         If additional specs are needed,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --domain <domain> --topic <topic> --file <file> [--source <path>...]
           Adding specs here re-enters DONE for the new items before reconciliation restarts.
         After completion of the above, advance to continue.
```

When `user_review: false`:

```
State:   RECONCILE_REVIEW
Phase:   specifying
Specs:   5 completed across 2 domains
Round:   1/3
Verdict: PASS

Action:  Reconciliation review complete.
         If additional specs are needed,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --domain <domain> --topic <topic> --file <file> [--source <path>...]
           Adding specs here re-enters DONE for the new items before reconciliation restarts.
         After completion of the above, advance to continue.
```

**Entering COMPLETE** (after RECONCILE_REVIEW, or RECONCILE_EVAL PASS on round > 1):

When `enable_commits: true`:

```
State:   COMPLETE
Phase:   specifying
Specs:   5 completed, reconciled
Action:  Specifying phase complete.
         Advance with --message "your commit message" to commit and continue.
```

When `enable_commits: false`:

```
State:   COMPLETE
Phase:   specifying
Specs:   5 completed, reconciled
Action:  Specifying phase complete. Advance to continue.
```

#### `eval` output — RECONCILE_EVAL

When `eval_mode: "report"`:

```
=== RECONCILIATION EVALUATION ROUND 1/3 ===

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/reconcile-eval.md>

--- DOMAINS ---

optimizer: 3 specs
portal:    2 specs

--- RECONCILIATION CONTEXT ---

Run: git diff --staged

--- REPORT OUTPUT ---

Write your evaluation report to:
  specs/.eval/reconciliation-r1.md
```

When `eval_mode: "direct"` or `eval_mode: "conversational"`, the `--- REPORT OUTPUT ---` section is omitted.

```
=== RECONCILIATION EVALUATION ROUND 1/3 ===

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/reconcile-eval.md>

--- DOMAINS ---

optimizer: 3 specs
portal:    2 specs

--- RECONCILIATION CONTEXT ---

Run: git diff --staged
```

#### Eval Report Locations

Reconciliation eval reports:
```
specs/.eval/reconciliation-rN.md
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance --verdict` outside of RECONCILE_EVAL | Error naming the current state. Exit code 1. | Verdict is only valid in evaluation states |
| `advance` in RECONCILE_EVAL without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in RECONCILE_EVAL without `--eval-report` when `eval_mode: "report"` | Error. Exit code 1. | Every evaluation must reference its report when eval output is enabled |
| `advance --eval-report` when `eval_mode` is not `"report"` | Warning: `--eval-report is ignored, --eval-report is only used in report mode`. Command proceeds. | Consistent with `--message` warning pattern |
| `advance` in COMPLETE without `--message` when `enable_commits: true` | Error. Exit code 1. | Commit message required for the specifying phase commit |
| `add-queue-item` outside of RECONCILE_REVIEW within reconciliation | Error: "add-queue-item is only valid in RECONCILE_REVIEW during reconciliation (current state: \<state\>)." Exit code 1. | Queue modifications during reconciliation are restricted to the review checkpoint |
| `eval` outside of RECONCILE_EVAL | Error naming current state and phase. Exit code 1. | Eval context only available in RECONCILE_EVAL |

---

## Behavior

### State Machine

```
DONE → RECONCILE → RECONCILE_EVAL
                        │
              ┌─────────┼──────────┐
              │         │          │
        PASS ≥ min   PASS < min  FAIL < max
              │         │          │
              ▼         ▼          ▼
         (continue)  RECONCILE  RECONCILE
              │
         round == 1?         FAIL ≥ max → (continue, forced)
           yes → RECONCILE_REVIEW → COMPLETE
           no → COMPLETE

(continue) = RECONCILE_REVIEW if round == 1 (always),
             otherwise COMPLETE
```

### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| DONE | always | RECONCILE | Initialize reconcile state with round 0. |
| RECONCILE | always | RECONCILE_EVAL | Increment reconcile round. |
| RECONCILE_EVAL | `--verdict PASS`, round >= `specifying.reconciliation.min_rounds`, round == 1 | RECONCILE_REVIEW | Record eval. Always enters RECONCILE_REVIEW on first passing eval. |
| RECONCILE_EVAL | `--verdict PASS`, round >= `specifying.reconciliation.min_rounds`, round > 1 | COMPLETE | Record eval. |
| RECONCILE_EVAL | `--verdict PASS`, round < `specifying.reconciliation.min_rounds` | RECONCILE | Record eval (PASS). Min rounds not met. |
| RECONCILE_EVAL | `--verdict FAIL`, round < `specifying.reconciliation.max_rounds` | RECONCILE | Record eval (FAIL). |
| RECONCILE_EVAL | `--verdict FAIL`, round >= `specifying.reconciliation.max_rounds` | RECONCILE_REVIEW or COMPLETE | Record eval (FAIL). Forced. If round == 1: RECONCILE_REVIEW. Else: COMPLETE. |
| RECONCILE_REVIEW | queue empty | COMPLETE | Advance to completion. |
| RECONCILE_REVIEW | queue non-empty (via add-queue-item) | DONE | Re-enter DONE to process new specs. Reconciliation restarts after new specs complete. |
| COMPLETE | `enable_commits: true` | PHASE_SHIFT | Stage files per `specifying.commit_strategy`, run `git commit -m <message>`, register hash on all completed specs. Set phase shift from specifying → generate_planning_queue. |
| COMPLETE | `enable_commits: false` | PHASE_SHIFT | Set phase shift from specifying → generate_planning_queue. |

### Reconcile Evaluation Sub-Agent

The reconciliation eval focuses on cross-domain concerns. The sub-agent:
1. Runs `git diff --staged` to see all changes
2. Reads all completed spec files across all domains
3. Checks:
   - Every `Depends On` reference points to a spec that exists
   - Every cross-domain dependency has a corresponding `Integration Points` entry in the target spec
   - Integration Points are symmetric across domain boundaries (if A lists B, B lists A)
   - Spec names are consistent across all cross-domain references
   - No circular dependencies exist in the cross-domain dependency graph

Intra-domain consistency (specs within the same domain) is already verified by CROSS_REFERENCE in spec-lifecycle.

---

## Invariants

1. **Reconciliation follows completion.** DONE always transitions to RECONCILE. All specs and domain cross-references must be complete before cross-domain validation.
2. **Reconcile min rounds enforced.** PASS below `specifying.reconciliation.min_rounds` routes back to RECONCILE, not COMPLETE.
3. **Reconcile max rounds enforced.** FAIL at `specifying.reconciliation.max_rounds` forces COMPLETE to prevent indefinite loops.
4. **Independent round config.** `specifying.reconciliation.*` is independent of `specifying.eval.*` and `specifying.cross_reference.*`.
5. **Single specifying commit at COMPLETE.** When `enable_commits` is `true`, `--message` is required at COMPLETE and the scaffold auto-commits. When `enable_commits` is `false`, `--message` is not shown in output; if provided, a warning is printed: `--message is ignored, commits are not enabled`. The warning does not instruct how to enable commits. The commit hash is automatically registered on all completed specs.
6. **Cross-domain focus.** Reconciliation checks cross-domain boundaries. Intra-domain consistency is handled by CROSS_REFERENCE.
7. **Reconciliation review fires once.** RECONCILE_REVIEW is entered exactly once after the first passing (or forced) RECONCILE_EVAL, regardless of `specifying.reconciliation.user_review`.
8. **user_review controls output, not state entry.** When `user_review` is true, RECONCILE_REVIEW includes "STOP please review and discuss with user before continuing." When false, it says "Reconciliation review complete."
9. **add-queue-item at RECONCILE_REVIEW re-enters DONE.** New specs added here go through the full drafting loop, then reconciliation restarts.

---

## Edge Cases

- **Scenario:** Reconciliation eval finds no issues on first pass, `specifying.reconciliation.min_rounds` is 0 (default).
  - **Expected:** RECONCILE → RECONCILE_EVAL with `--verdict PASS` → RECONCILE_REVIEW → COMPLETE.
  - **Rationale:** When `specifying.reconciliation.min_rounds` is 0, a single PASS triggers the review checkpoint, then completes.

- **Scenario:** PASS verdict at round 1 with `specifying.reconciliation.min_rounds` set to 2.
  - **Expected:** RECONCILE_EVAL → RECONCILE (min rounds not met).
  - **Rationale:** Even with a passing verdict, minimum reconciliation rounds must be completed when configured.

- **Scenario:** FAIL verdict at `specifying.reconciliation.max_rounds`, round 1.
  - **Expected:** RECONCILE_EVAL → RECONCILE_REVIEW (forced, but review checkpoint still fires on round 1).
  - **Rationale:** Even on forced completion, the review checkpoint fires once for the architect to add late-discovered specs.

- **Scenario:** FAIL verdict at `specifying.reconciliation.max_rounds`, round > 1.
  - **Expected:** RECONCILE_EVAL → COMPLETE (forced).
  - **Rationale:** Review already happened on round 1. Max rounds exhausted.

- **Scenario:** All specs are in the same domain (no cross-domain boundaries).
  - **Expected:** Reconciliation still runs but the sub-agent finds no cross-domain issues. Single PASS → RECONCILE_REVIEW → COMPLETE.
  - **Rationale:** Reconciliation is mandatory in the state machine. When there are no cross-domain concerns, it passes trivially.

- **Scenario:** `add-queue-item` called at RECONCILE_REVIEW.
  - **Expected:** Queue populated. Advancing from RECONCILE_REVIEW re-enters DONE. New specs go through ORIENT → DRAFT → EVALUATE → ACCEPT → CROSS_REFERENCE → CROSS_REFERENCE_REVIEW → DONE → RECONCILE (reconciliation restarts).
  - **Rationale:** Cross-domain gaps discovered during reconciliation require new specs that must be drafted, evaluated, cross-referenced, then reconciled again.

- **Scenario:** RECONCILE_REVIEW with `user_review: false`.
  - **Expected:** State entered. Action says "Reconciliation review complete." No user review prompt. add-queue-item available.
  - **Rationale:** Review checkpoint always fires for add-queue-item. user_review only controls the review prompt.

- **Scenario:** COMPLETE with `enable_commits: true` and no files changed.
  - **Expected:** Commit skipped. Notice printed. Advance to PHASE_SHIFT proceeds.
  - **Rationale:** Empty commits are not created. See `docs/auto-committing.md`.

- **Scenario:** COMPLETE with `enable_commits: false` and `--message` provided.
  - **Expected:** Warning: `--message is ignored, commits are not enabled`. Command proceeds. The warning does not instruct how to enable commits.
  - **Rationale:** Users who do not need auto-commits should not be confused or prompted to change configuration.

---

## Testing Criteria

### DONE transitions to RECONCILE
- **Verifies:** Reconciliation is mandatory after spec completion.
- **Given:** All specs accepted, all domains cross-referenced, state is DONE.
- **When:** `advance`
- **Then:** State is RECONCILE.

### Reconcile PASS with default reconcile_min_rounds (0)
- **Verifies:** PASS at round 1 enters RECONCILE_REVIEW.
- **Given:** RECONCILE_EVAL, `specifying.reconciliation.min_rounds: 0`, round 1.
- **When:** `advance --verdict PASS --eval-report .eval/reconciliation-r1.md`
- **Then:** State is RECONCILE_REVIEW.

### Reconcile PASS below reconcile_min_rounds
- **Verifies:** PASS below min rounds loops back to RECONCILE.
- **Given:** RECONCILE_EVAL, `specifying.reconciliation.min_rounds: 2`, round 1.
- **When:** `advance --verdict PASS --eval-report .eval/reconciliation-r1.md`
- **Then:** State is RECONCILE.

### Reconcile PASS at reconcile_min_rounds round 2
- **Verifies:** PASS at min rounds on round > 1 skips review, goes to COMPLETE.
- **Given:** RECONCILE_EVAL, `specifying.reconciliation.min_rounds: 2`, round 2.
- **When:** `advance --verdict PASS --eval-report .eval/reconciliation-r2.md`
- **Then:** State is COMPLETE.

### Reconcile FAIL below reconcile_max_rounds
- **Verifies:** FAIL verdict loops back to RECONCILE.
- **Given:** RECONCILE_EVAL, `specifying.reconciliation.max_rounds: 3`, round 1.
- **When:** `advance --verdict FAIL --eval-report .eval/reconciliation-r1.md`
- **Then:** State is RECONCILE.

### Reconcile FAIL at reconcile_max_rounds round 1
- **Verifies:** Forced completion at round 1 still enters RECONCILE_REVIEW.
- **Given:** RECONCILE_EVAL, `specifying.reconciliation.max_rounds: 1`, round 1.
- **When:** `advance --verdict FAIL --eval-report .eval/reconciliation-r1.md`
- **Then:** State is RECONCILE_REVIEW (forced, but review still fires).

### Reconcile FAIL at reconcile_max_rounds round 3
- **Verifies:** FAIL at max rounds on round > 1 forces COMPLETE.
- **Given:** RECONCILE_EVAL, `specifying.reconciliation.max_rounds: 3`, round 3.
- **When:** `advance --verdict FAIL --eval-report .eval/reconciliation-r3.md`
- **Then:** State is COMPLETE (forced).

### RECONCILE_REVIEW with user_review true
- **Verifies:** Review prompt shown when user_review is true.
- **Given:** RECONCILE_REVIEW, `specifying.reconciliation.user_review: true`.
- **When:** `status`
- **Then:** Action includes "STOP please review and discuss with user before continuing."

### RECONCILE_REVIEW with user_review false
- **Verifies:** No review prompt when user_review is false.
- **Given:** RECONCILE_REVIEW, `specifying.reconciliation.user_review: false`.
- **When:** `status`
- **Then:** Action includes "Reconciliation review complete." No user review prompt.

### RECONCILE_REVIEW add-queue-item re-enters DONE
- **Verifies:** Adding specs at RECONCILE_REVIEW restarts the cycle.
- **Given:** RECONCILE_REVIEW, queue empty.
- **When:** `add-queue-item --name "New Spec" --domain portal --topic "..." --file portal/specs/new-spec.md`, then `advance`.
- **Then:** State is DONE. New spec must be drafted, evaluated, cross-referenced before reconciliation restarts.

### RECONCILE_REVIEW advances to COMPLETE when queue empty
- **Verifies:** Clean advance through review to completion.
- **Given:** RECONCILE_REVIEW, queue empty.
- **When:** `advance`
- **Then:** State is COMPLETE.

### COMPLETE transitions to PHASE_SHIFT
- **Verifies:** Reconciliation completion triggers phase transition.
- **Given:** COMPLETE, `enable_commits: false`.
- **When:** `advance`
- **Then:** State is PHASE_SHIFT.

### COMPLETE auto-commits when enable_commits is true
- **Verifies:** Single specifying commit at COMPLETE.
- **Given:** COMPLETE, `enable_commits: true`, `specifying.commit_strategy: "all-specs"`. Spec files modified.
- **When:** `advance --message "Complete specifying phase"`
- **Then:** Files staged per `all-specs` strategy. `git commit` executed. Hash registered on all completed specs. State is PHASE_SHIFT.

### COMPLETE rejects missing --message when enable_commits is true
- **Verifies:** Commit message required.
- **Given:** COMPLETE, `enable_commits: true`.
- **When:** `advance` (no `--message`)
- **Then:** Exit code 1.

### COMPLETE ignores --message when enable_commits is false
- **Verifies:** Warning printed, command proceeds.
- **Given:** COMPLETE, `enable_commits: false`.
- **When:** `advance --message "ignored"`
- **Then:** Warning: `--message is ignored, commits are not enabled`. State is PHASE_SHIFT. Warning does not instruct how to enable commits.

### COMPLETE skips commit when no files changed
- **Verifies:** Empty commits are not created.
- **Given:** COMPLETE, `enable_commits: true`. No spec files modified.
- **When:** `advance --message "no changes"`
- **Then:** Commit skipped. Notice printed. State is PHASE_SHIFT.

---

## Implements
- Cross-domain reconciliation after all individual specs and domain cross-references are complete
- Standard PASS/FAIL eval loop with min/max rounds
- RECONCILE_REVIEW checkpoint after first passing/forced eval (always fires, regardless of user_review)
- add-queue-item at RECONCILE_REVIEW for cross-domain gaps (re-enters DONE to process new specs)
- Independent reconciliation round configuration (`specifying.reconciliation.min_rounds`, `specifying.reconciliation.max_rounds`)
- Single specifying commit at COMPLETE with auto-staging per `specifying.commit_strategy` and automatic hash registration
