# Forgectl Workflow Reference

Complete reference for using the forgectl scaffold during the implementing phase.

## Initializing a Session

When no `forgectl-state.json` exists, initialize from the plan:

```bash
forgectl init \
  --from {domain}/.forge_workspace/implementation_plan/plan.json \
  --phase implementing
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--from` | yes | — | Path to plan.json |
| `--phase` | no | specifying | Set to `implementing` to start at implementation |

All batch sizes, round limits, and guided settings are configured in `.forgectl/config` (TOML) and locked into the state file at init time.

## Key Commands

| Command | When | What it does |
|---------|------|-------------|
| `forgectl status` | Anytime | Shows current state, progress, action guidance |
| `forgectl advance` | After completing current state's work | Transitions to next state |
| `forgectl eval` | EVALUATE state only | Outputs full evaluation context for the subagent |

Always run from the project root: `forgectl <command>`

---

## State Machine

```
ORIENT → IMPLEMENT → IMPLEMENT → ... → EVALUATE → COMMIT → ORIENT → ...
                                            ↓
                                   IMPLEMENT (round 2+)
```

### Transition Table

| From | Flags | To | Condition |
|------|-------|----|-----------|
| ORIENT | — | IMPLEMENT | Batch selected |
| ORIENT | — | ORIENT | Layer complete, advancing to next |
| ORIENT | — | DONE | All layers complete |
| IMPLEMENT (round 1) | `--message` | IMPLEMENT | More items in batch |
| IMPLEMENT (round 1) | `--message` | EVALUATE | Last item in batch |
| IMPLEMENT (round 2+) | — | IMPLEMENT | More items in batch |
| IMPLEMENT (round 2+) | — | EVALUATE | Last item in batch |
| EVALUATE | `--verdict PASS --eval-report` | COMMIT | rounds >= min_rounds |
| EVALUATE | `--verdict PASS --eval-report` | IMPLEMENT | rounds < min_rounds |
| EVALUATE | `--verdict FAIL --eval-report` | IMPLEMENT | rounds < max_rounds |
| EVALUATE | `--verdict FAIL --eval-report` | COMMIT | rounds >= max_rounds (force) |
| COMMIT | `--message` | ORIENT | More items or layers remain |
| COMMIT | `--message` | DONE | All layers complete |
| DONE | — | (terminal) | Session finished |

---

## State-by-State Instructions

Every `forgectl advance` and `forgectl status` prints an `Action:` line. **Always read it.** The instructions below expand on what to do in each state.

### ORIENT

The scaffold has selected a batch or is transitioning between layers.

1. Read the forgectl output — it shows layer, progress, and what's coming next.
2. If guided mode is on, **stop and discuss with the user** before continuing.
3. When ready:
   ```bash
   forgectl advance
   ```

### IMPLEMENT (round 1 — first time seeing items)

You have been assigned an item. The forgectl output shows: item ID, name, description, steps, files, specs, refs, and test count.

1. Read the item's `specs` (specification references) to understand the contract.
2. Read the item's `refs` (notes file paths at `{domain}/.forge_workspace/implementation_plan/notes/`) for implementation guidance.
3. Search the codebase using subagents to confirm the feature doesn't already exist.
4. Implement the functionality completely. No placeholders, no stubs.
5. Run the tests for the code you changed or added.
6. If tests fail, diagnose and fix. Use extended thinking if needed.
7. If tests unrelated to your work fail, resolve them as part of this increment.
8. When tests pass, advance (**forgectl auto-commits on first round**):
   ```bash
   forgectl advance --message "<what you implemented>"
   ```
9. Forgectl prints the next state — either another IMPLEMENT (next item in batch) or EVALUATE (batch complete). Handle accordingly.

### IMPLEMENT (round 2+ — after evaluation)

The batch has been evaluated and returned for another round. The forgectl output shows the eval report path.

1. **Study the eval report** — it contains specific deficiencies to address.
2. Read the item's specs and refs again if needed.
3. Fix the deficiencies identified in the eval report.
4. If the eval was PASS but minimum rounds weren't met, verify the implementation and look for improvements.
5. Run the tests.
6. When tests pass, advance (**no message needed on round 2+**):
   ```bash
   forgectl advance
   ```
7. Handle the next state (another IMPLEMENT or EVALUATE).

### EVALUATE

All items in the current batch have been implemented. Time to evaluate.

1. Spawn a subagent to perform the evaluation:
   - The subagent runs `forgectl eval` to get full evaluation context (items, specs, refs, evaluator instructions, report output path).
   - The subagent reads the implementation files, specs, and refs listed in the eval output.
   - The subagent writes an evaluation report to the path specified in the eval output.
   - The subagent returns the verdict (PASS or FAIL) and the report path.
2. Advance with the verdict:
   ```bash
   forgectl advance --eval-report <path> --verdict PASS
   # or
   forgectl advance --eval-report <path> --verdict FAIL
   ```
3. Forgectl transitions based on the verdict and round count (see transition table above).

### COMMIT

The batch is terminal (passed or force-accepted). Commit your work.

1. Stage and commit changes:
   ```bash
   git add -A && git commit -m "<descriptive message>"
   ```
2. Add a log entry to `{domain}/.forge_workspace/implementation/IMPLEMENTATION_LOG.md`.
3. Advance:
   ```bash
   forgectl advance --message "<commit message>"
   ```
4. Forgectl prints the next state — either ORIENT (more work) or DONE (finished).

### DONE

All layers and items are complete.

1. Add a final summary log entry to `{domain}/.forge_workspace/implementation/IMPLEMENTATION_LOG.md`.
2. The session is finished.

---

## Important Details

- **Round 1 IMPLEMENT** requires `--message` — forgectl auto-commits per item.
- **Round 2+ IMPLEMENT** does not need `--message` — no auto-commit, just fixing deficiencies.
- **ORIENT** is a guided pause when `general.user_guided` is true in the config. Stop and discuss with the user.
- **EVALUATE round 2+** output includes a `--- PREVIOUS EVALUATIONS ---` section listing prior round reports.
- Forgectl tracks `passes` and `rounds` in plan.json automatically. Do not modify these fields manually.
- The `--guided` / `--no-guided` flags can be passed on `advance` calls to override the config's guided setting for that transition.
