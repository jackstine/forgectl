# Generate Planning Queue

> How forgectl auto-generates a plan queue from completed specs.

---

## When to Use

Use the `generate_planning_queue` phase when:
- You have completed a specifying phase (all specs accepted/reconciled)
- `forgectl-state.json` exists with completed specifying data
- You want forgectl to auto-group specs by domain rather than building the queue manually

This phase sits between specifying and planning. It cannot be initialized directly — it requires a completed specifying phase or a PHASE_SHIFT from specifying's COMPLETE state.

---

## How It Works

### ORIENT

On entry, forgectl:

1. Groups completed specs by domain (order follows the spec queue — domains appear in the order they were first seen).
2. For each domain, produces a plan entry:
   - `name`: `"<Domain> Implementation Plan"` (domain name capitalized)
   - `domain`: the domain name
   - `file`: `<domain>/.forge_workspace/implementation_plan/plan.json`
   - `specs`: all completed spec file paths for this domain
   - `spec_commits`: deduplicated commit hashes from the domain's completed specs
   - `code_search_roots`: from `specifying.domains[<domain>].code_search_roots` if set via `set-roots`, otherwise `["<domain>/"]`
3. Writes the plan queue to `<state_dir>/plan-queue.json`.

```bash
forgectl advance
```

### REFINE

The architect reviews `<state_dir>/plan-queue.json`. You can:
- Reorder domains (change plan processing order)
- Adjust plan names
- Modify `code_search_roots` to include cross-domain directories
- Add or remove specs from a plan entry
- Leave it unchanged

Advancing validates the file. If validation fails, errors are printed and state stays at REFINE. Fix and re-advance.

```bash
forgectl advance
```

### PHASE_SHIFT (generate_planning_queue -> planning)

Advancing consumes the validated plan queue and transitions to planning ORIENT.

You may optionally override with a different file at this point:

```bash
# Use the auto-generated queue:
forgectl advance

# Or override with a custom queue:
forgectl advance --from <custom-queue.json>
```

---

## Skipping This Phase

If you already have a plan queue file, you can skip `generate_planning_queue` entirely at the specifying PHASE_SHIFT:

```bash
forgectl advance --from <plan-queue.json>
```

This jumps directly from specifying to planning ORIENT.

Alternatively, initialize a fresh session at planning:

```bash
forgectl init --phase planning --from <plan-queue.json>
```

See [creating-plan-queue.md](creating-plan-queue.md) for how to build the file manually.

---

## Plan Queue Schema

The auto-generated file follows the same schema as manually constructed queues:

```json
{
  "plans": [
    {
      "name": "Optimizer Implementation Plan",
      "domain": "optimizer",
      "file": "optimizer/.forge_workspace/implementation_plan/plan.json",
      "specs": [
        "optimizer/specs/cost-function.md",
        "optimizer/specs/constraint-solver.md"
      ],
      "spec_commits": ["7cede10", "8743b1d"],
      "code_search_roots": ["optimizer/", "lib/shared/"]
    }
  ]
}
```

See [plan-queue-format.md](plan-queue-format.md) for the full schema reference.
