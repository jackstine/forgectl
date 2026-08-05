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
ORIENT → IMPLEMENT → IMPLEMENT → ... → EVALUATE → ORIENT → ...
                                            ↓
                report/conversational: IMPLEMENT (round 2+)
                direct: EVALUATE (re-evaluate, no re-implement)
```

Every IMPLEMENT advance auto-commits its item when `enable_commits: true` — first round and every subsequent round alike. When EVALUATE reaches a terminal verdict (PASS at or above `min_rounds`, or FAIL at `max_rounds` force-accept):

- `enable_commits: true` — the scaffold auto-commits the batch inline as part of that same `advance` and proceeds straight to ORIENT/DONE. **COMMIT does not appear.**
- `enable_commits: false` — the scaffold proceeds to the COMMIT state (a no-op advance) before ORIENT/DONE.

### Transition Table

| From | Flags | To | Condition |
|------|-------|----|-----------|
| ORIENT | — | IMPLEMENT | Batch selected |
| ORIENT | — | ORIENT | Layer complete, advancing to next |
| ORIENT | — | DONE | All layers complete |
| IMPLEMENT (any round) | `[--message]` (optional) | IMPLEMENT | More items in batch |
| IMPLEMENT (any round) | `[--message]` (optional) | EVALUATE | Last item in batch |
| EVALUATE | `--verdict PASS --eval-report` | ORIENT/DONE (`enable_commits: true`) or COMMIT (`enable_commits: false`) | rounds >= min_rounds |
| EVALUATE | `--verdict PASS --eval-report` | IMPLEMENT (report/conversational) or EVALUATE (direct) | rounds < min_rounds |
| EVALUATE | `--verdict FAIL --eval-report` | IMPLEMENT (report/conversational) or EVALUATE (direct) | rounds < max_rounds |
| EVALUATE | `--verdict FAIL --eval-report` | ORIENT/DONE (`enable_commits: true`) or COMMIT (`enable_commits: false`) | rounds >= max_rounds (force) |
| COMMIT (only when `enable_commits: false`) | — | ORIENT | More items or layers remain |
| COMMIT (only when `enable_commits: false`) | — | DONE | All layers complete |
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
8. When tests pass, advance (**forgectl auto-commits this item when `enable_commits: true`**, message synthesized from the item's `description` in plan.json):
   ```bash
   forgectl advance
   ```
   `--message` is optional — pass it only to append extra context to the synthesized message:
   ```bash
   forgectl advance --message "<extra context, if any>"
   ```
9. Forgectl prints the next state — either another IMPLEMENT (next item in batch) or EVALUATE (batch complete). Handle accordingly.

### IMPLEMENT (round 2+ — after evaluation, `eval_mode: "report"` or `"conversational"` only)

The batch has been evaluated and returned for another round. (Under `eval_mode: "direct"`, IMPLEMENT never returns for round 2+ — a FAIL or below-minimum-rounds PASS re-enters EVALUATE directly instead; see EVALUATE below.) The forgectl output shows the eval report path.

1. **Study the eval report** — it contains specific deficiencies to address.
2. Read the item's specs and refs again if needed.
3. Fix the deficiencies identified in the eval report.
4. If the eval was PASS but minimum rounds weren't met, verify the implementation and look for improvements.
5. Run the tests.
6. When tests pass, advance (**forgectl auto-commits this round too, exactly like round 1** — the "first round only" rule does not apply):
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
   Under `eval_mode: "direct"`, the same `--verdict` advance applies with no `--eval-report`. If the round is not yet terminal, this re-enters EVALUATE directly for another round — there is no intervening IMPLEMENT round under `"direct"`.
3. Forgectl transitions based on the verdict and round count (see transition table above). When the verdict is terminal (PASS at/above `min_rounds`, or FAIL force-accepted at `max_rounds`) and `enable_commits: true`, **this same advance auto-commits the batch inline** (message synthesized from the batch's item descriptions) and proceeds straight to ORIENT/DONE — no separate COMMIT step. When `enable_commits: false`, it proceeds to COMMIT instead.

### COMMIT (only reached when `enable_commits: false`)

The batch is terminal (passed or force-accepted). This state is a no-op advance — no git operation occurs (when `enable_commits: true`, the batch already auto-committed inline at the terminal EVALUATE `advance` above, and this state never appears).

1. Add a log entry to `{domain}/.forge_workspace/implementation/IMPLEMENTATION_LOG.md`.
2. Advance:
   ```bash
   forgectl advance
   ```
3. Forgectl prints the next state — either ORIENT (more work) or DONE (finished).

### DONE

All layers and items are complete.

1. Add a final summary log entry to `{domain}/.forge_workspace/implementation/IMPLEMENTATION_LOG.md`.
2. The session is finished.

---

## Important Details

- **Every IMPLEMENT round** auto-commits its item when `enable_commits: true` — first round and every round after an eval FAIL alike. `--message` is optional; when provided it is appended to the message synthesized from the item's `description`, not a replacement for it.
- **`eval_mode: "direct"`** runs IMPLEMENT exactly once per batch. A FAIL, or a PASS below minimum rounds, loops directly back to EVALUATE for another round instead of re-entering IMPLEMENT.
- **COMMIT** only appears when `enable_commits: false`. When `enable_commits: true`, the batch-terminal commit happens inline as part of the terminal EVALUATE `advance`, and the scaffold proceeds directly to ORIENT/DONE.
- **ORIENT** is a guided pause when `general.user_guided` is true in the config. Stop and discuss with the user.
- **EVALUATE round 2+** output includes a `--- PREVIOUS EVALUATIONS ---` section listing prior round reports.
- Forgectl tracks `passes` and `rounds` in plan.json automatically. Do not modify these fields manually.
- The `--guided` / `--no-guided` flags can be passed on `advance` calls to override the config's guided setting for that transition.
