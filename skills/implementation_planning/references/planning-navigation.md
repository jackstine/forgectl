# Navigating Implementation Planning in Forgectl

> How to move through the planning phase state machine — from orientation through acceptance.

---

## State Flow

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

---

## State-by-State Guide

### ORIENT

Read the plan metadata and understand the domain. Run `forgectl status` to see plan name, domain, file target, and specs list.

```bash
forgectl advance
```

### STUDY_SPECS

Study every spec listed in the plan's `specs` field plus the SPEC_MANIFEST.md. Read full spec files and review git diffs for recent spec commits.

```bash
forgectl advance
```

### STUDY_CODE

Explore the codebase using sub-agents (3 agents) within the plan's `code_search_roots`. Identify existing implementations, TODOs, placeholders, and patterns.

```bash
forgectl advance
```

### STUDY_PACKAGES

Study the project's technical stack: package manifests (`go.mod`, `package.json`, etc.), library docs, and CLAUDE.md references.

```bash
forgectl advance
```

### REVIEW

Checkpoint before drafting. Review study findings and the plan format reference (`forgectl/docs/PLAN_FORMAT.md`). When guided mode is on, stop and discuss with the user before continuing.

```bash
forgectl advance
```

### DRAFT

Generate the implementation plan:
- Write `plan.json` following the schema in `references/plan-format.json` (the authoritative schema derived from forgectl's Go types)
- Write `notes/<package>.md` files for implementation guidance
- Output location: `<domain>/.forge_workspace/implementation_plan/`

When you advance, forgectl automatically validates `plan.json`. If valid, you go straight to EVALUATE. If invalid, you enter VALIDATE. See VALIDATE section below for common failures.

```bash
forgectl advance
```

### VALIDATE

Only entered when `plan.json` fails structural validation. Forgectl prints specific errors with field descriptions. Fix the issues and advance to re-validate.

**Common validation failures:**
- `cannot unmarshal string into Go struct field PlanJSON.refs of type state.PlanRef` — `refs` entries must be objects `{"id": "...", "path": "..."}`, not plain strings.
- `refs: path "..." does not exist` — Paths are resolved relative to the project root (the directory containing `.forgectl/`). Use full paths from root like `api/specs/foo.md`.
- `items[N]: refs path "notes/foo.md#section" does not exist` — Remove `#anchor` fragments from `refs` paths. Forgectl runs `os.Stat()` on the raw string including the fragment.
- `missing required field "tests"` or `"depends_on"` — These fields must be arrays, never null. Use `[]` for empty.

```bash
# Fix plan.json, then:
forgectl advance
```

### EVALUATE

Spawn an evaluation sub-agent to review the plan against all referenced specs. Use `forgectl eval` to get the full evaluation context (evaluator instructions, plan references, report target).

The evaluator assesses 11 dimensions: Behavior, Error Handling, Rejection, Interface, Configuration, Observability, Integration Points, Invariants, Edge Cases, Testing Criteria, and Dependencies & Format.

```bash
# Get eval context for the sub-agent:
forgectl eval

# After evaluation, record the verdict:
forgectl advance --verdict PASS --eval-report <path>
# or
forgectl advance --verdict FAIL --eval-report <path>
```

Eval reports are written to: `<domain>/.forge_workspace/implementation_plan/evals/round-N.md`

### REFINE

Entered after FAIL (or PASS below min_rounds). Spawn a sub-agent to update the plan and notes based on the eval report. Advancing runs the validation gate again.

```bash
forgectl advance
```

### ACCEPT

Plan is accepted (either by passing evaluation or forced at max rounds).

When `enable_commits` is true in `.forgectl/config`, advance with a commit message for auto-commit:

```bash
forgectl advance --message "Accept implementation plan for <domain>"
```

When `enable_commits` is false, just advance:

```bash
forgectl advance
```

After ACCEPT, forgectl transitions to ORIENT (if more plans remain in the queue) or DONE (if all plans are complete). From DONE, advancing triggers PHASE_SHIFT (planning → implementing).

---

## CLI Quick Reference

```bash
# Initialize planning session
forgectl init --phase planning --from plan-queue.json

# See current state and what to do next
forgectl status

# Move to next state (most transitions)
forgectl advance

# Get evaluation context for sub-agent (EVALUATE only)
forgectl eval

# Record evaluation verdict (EVALUATE only)
forgectl advance --verdict PASS --eval-report <path>
forgectl advance --verdict FAIL --eval-report <path>

# Accept plan (ACCEPT only, when enable_commits is true)
forgectl advance --message "<commit message>"

# Accept plan (ACCEPT only, when enable_commits is false)
forgectl advance
```

### Flag Reference

| Flag | Used in | Description |
|------|---------|-------------|
| `--verdict PASS\|FAIL` | EVALUATE | Evaluation result (required) |
| `--eval-report <path>` | EVALUATE | Path to evaluation report file (required when `enable_eval_output` is true, must exist) |
| `--message <text>` | ACCEPT | Commit message for accepted plan (required when `enable_commits` is true) |

---

## Round Enforcement

- **min_rounds**: PASS before this threshold sends you to REFINE for another cycle.
- **max_rounds**: FAIL at this threshold forces ACCEPT — prevents infinite loops.
- Rounds increment on each REFINE → EVALUATE cycle.

---

## Phase Transition

After the last plan is accepted (queue empty), forgectl enters DONE. Advancing from DONE triggers PHASE_SHIFT (planning → implementing):

1. Reads `plan.json` from `current_plan.file`
2. Validates the plan structure
3. Adds `passes: "pending"` and `rounds: 0` to every plan item
4. Writes the updated `plan.json` back to disk
5. Sets phase to `implementing`, state to `ORIENT`

No `--from` flag needed — the plan path is already known from the planning session.

```bash
forgectl advance
```
