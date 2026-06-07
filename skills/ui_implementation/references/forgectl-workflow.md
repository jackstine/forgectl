# Forgectl Workflow Reference — `ui_implementing` Phase

Complete reference for using the forgectl scaffold during the UI implementation phase. This phase extends the implementing phase with two verification loops after each batch's code evaluation: a **QA placement loop** and an **end-to-end loop**.

## Initializing a Session

When no `forgectl-state.json` exists, initialize from the UI plan:

```bash
forgectl init \
  --from {domain}/.forgectl_workspace/ui_plan/plan.json \
  --phase ui_implementing
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--from` | yes | — | Path to the UI plan.json (`kind: ui`) |
| `--phase` | no | specifying | Set to `ui_implementing` to start at UI implementation |

`init --phase ui_implementing` is **rejected** if any of these config keys is empty — the QA loop needs a running app and URL, and the e2e loop needs a runner and a directory:

- `ui_implementing.app.launch_command`
- `ui_implementing.app.url`
- `ui_implementing.e2e.test_command`
- `ui_implementing.e2e.test_dir`

All batch sizes, round limits, eval modes, app config, and e2e config live in `.forgectl/config` (TOML) and are locked into the state file at init time.

## Key Commands

| Command | Who runs it | When | What it does |
|---------|-------------|------|-------------|
| `forgectl status` | engineer | anytime | Shows current state, the resolved paths, progress, and action guidance |
| `forgectl advance` | engineer | after completing the current state's work | Transitions to the next state; records verdicts in evaluator states |
| `forgectl eval` | **sub-agent** | EVALUATE, QA_TEST, E2E_VERIFY | Outputs full evaluation context for the sub-agent |
| `forgectl handoff <file>…` | **sub-agent** | EVALUATE, QA_TEST, E2E_VERIFY | Registers the files the sub-agent produced so the engineer is told to review them |

Always run from the project root. **Never assume a workspace path — use the paths forgectl prints.**

---

## State Machine

```
ORIENT → IMPLEMENT(1..n) → EVALUATE ──► QA_TEST ⟲ UI_REFINE ──► E2E_AUTHOR → E2E_VERIFY ⟲ E2E_REMEDIATE ──► COMMIT → ORIENT → … → DONE
                            (code loop)   (qa loop)                            (e2e loop)
```

Three sequential gates per batch — **code eval, then QA, then e2e** — before COMMIT. Each loop has its own round budget (`eval`, `qa`, `e2e` — each `min_rounds`/`max_rounds`). A loop terminates on PASS at/above its `min_rounds`, or force-accepts at its `max_rounds`. **Any force-accept marks the batch's items `failed` at COMMIT but does not abort the phase.**

### Transition Table (abridged — see the spec for the authoritative table)

| From | Flags | To | Condition |
|------|-------|----|-----------|
| ORIENT | — | IMPLEMENT | Batch selected (resets `eval_round`, `qa_round`, `e2e_round`) |
| ORIENT | — | DONE | All layers complete |
| IMPLEMENT (round 1) | `--message` | IMPLEMENT / EVALUATE | More items / last item in batch |
| IMPLEMENT (round 2+) | — | IMPLEMENT / EVALUATE | More items / last item |
| EVALUATE | `--verdict --eval-report` | QA_TEST | PASS ≥ min, or FAIL force-accept at max |
| EVALUATE | `--verdict --eval-report` | IMPLEMENT | PASS < min, or FAIL < max |
| QA_TEST | `--verdict --eval-report` | E2E_AUTHOR | PASS ≥ min, or FAIL force-accept at max (**step list must be present**) |
| QA_TEST | `--verdict --eval-report` | UI_REFINE | PASS < min, or FAIL < max |
| UI_REFINE | — | QA_TEST | Increment `qa_round` |
| E2E_AUTHOR | — | E2E_VERIFY | Increment `e2e_round` |
| E2E_VERIFY | `--verdict --eval-report` | COMMIT | PASS ≥ min, or FAIL force-accept at max |
| E2E_VERIFY | `--verdict --eval-report` | E2E_REMEDIATE | PASS < min, or FAIL < max |
| E2E_REMEDIATE | — | E2E_VERIFY | Increment `e2e_round` |
| COMMIT | `--message` | ORIENT / DONE | More work / finished |

---

## State-by-State Instructions

Every output prints an `Action:` line and the resolved paths. **Always read them.**

### ORIENT
The scaffold has selected a batch or is moving between layers.
1. Read the output — layer, batch items, app launch/url, and config.
2. If guided mode is on, **stop and discuss with the user** before continuing.
3. `forgectl advance`

### IMPLEMENT (round 1 — first time seeing items)
Build the UI item. The output shows item ID, name, description, steps, files, specs, refs, and test count.
1. Read the item's `specs` (the contract) and `refs` (notes for approach, components, libraries).
2. Search the codebase (via subagent) to confirm the feature/component doesn't already exist.
3. Implement completely — no placeholders or stubs.
4. Run the tests for what you changed; fix failures, including unrelated ones, as part of this increment.
5. Advance (**forgectl auto-commits on round 1**): `forgectl advance --message "<what you implemented>"`
6. Handle the next state (another IMPLEMENT, or EVALUATE when the batch is complete).

### IMPLEMENT (round 2+ — after a code eval)
The output shows the eval report path.
1. **Study the eval report** for specific deficiencies.
2. Fix them; if it was a PASS below min rounds, verify and harden.
3. Run tests. Advance (**no message on round 2+**): `forgectl advance`

### EVALUATE (code loop)
All batch items are implemented; evaluate the code.
1. Spawn a sub-agent (see [subagent-usage.md](subagent-usage.md)). It:
   - runs `forgectl eval` to get context (items, specs, refs, evaluator instructions, report path);
   - reads the implementation, specs, and refs;
   - writes the eval report to the given path;
   - in `report` mode, runs `forgectl handoff <impl-eval-report>`;
   - returns the verdict and report path.
2. Review the handed-off report, then advance:
   `forgectl advance --eval-report <path> --verdict PASS|FAIL`
3. On a terminal code verdict (PASS ≥ min, or force-accept), the batch moves to **QA_TEST**.

### QA_TEST (qa loop) — the running UI, driven by the Playwright MCP
The output shows the app launch command, URL, the batch items, and the step-list path.
1. Spawn a QA sub-agent. It:
   - runs `forgectl eval` to get the QA evaluator prompt, items, app URL, step-list path, and (in `report` mode) report path;
   - **drives the running app through the Playwright MCP** — navigate to the URL, snapshot to judge placement and controls, exercise controls, read console errors, capture screenshots (see [qa-playwright.md](qa-playwright.md));
   - **always writes the e2e step list** to the given path;
   - produces its verdict per `eval_mode` (writes a QA report / corrects placement directly / relays verbally);
   - runs `forgectl handoff <qa-report> <step-list>` (the step list alone in `conversational` mode).
2. A re-render (`forgectl status`) now shows a `Review:` line naming the handed-off files. Review them.
3. Advance: `forgectl advance --eval-report <qa-report> --verdict PASS|FAIL`
   - PASS ≥ `qa.min_rounds` (or FAIL force-accept at `qa.max_rounds`) → **E2E_AUTHOR** (the step list must exist or advance is rejected).
   - otherwise → **UI_REFINE**.

> If the app will not launch or the URL will not load, that is a real defect. The QA sub-agent records a **FAIL** with the launch/navigation failure (and still hands off a step list, possibly empty); fix it in UI_REFINE.

### UI_REFINE (qa loop — the fixer)
1. Study the QA report (`report` mode) / review unstaged QA changes (`direct`) / apply the verbal corrections (`conversational`).
2. Iterate on placement and controls — move, resize, relabel, reorder, rewire. No commit, no `--message`.
3. `forgectl advance` → back to QA_TEST (`qa_round` increments).

### E2E_AUTHOR (bridge — author Playwright test files)
The output shows the step-list path and its scenario count, the test dir, and the run command.
1. Read the most recent QA step list. **Author one Playwright test file per scenario** into `ui_implementing.e2e.test_dir` — map each scenario's `steps` to Playwright actions and each `expected` to an assertion. Do not invent scenarios not in the list.
2. Run the suite via `ui_implementing.e2e.test_command`.
3. Advance regardless of pass/fail/runner-error (the verdict is E2E_VERIFY's job): `forgectl advance` → **E2E_VERIFY**.
4. **Zero scenarios:** author nothing and advance directly — the e2e loop passes vacuously.

### E2E_VERIFY (e2e loop)
1. Spawn an e2e sub-agent. It:
   - runs `forgectl eval` to get the e2e evaluator prompt, step-list path, and test command;
   - confirms the authored Playwright tests execute, pass, and cover the step list;
   - in `report` mode, writes its report and runs `forgectl handoff <e2e-report>`;
   - returns the verdict.
2. Review the handed-off report, then advance:
   `forgectl advance --eval-report <e2e-report> --verdict PASS|FAIL`
   - PASS ≥ `e2e.min_rounds` (or FAIL force-accept at max) → **COMMIT**.
   - otherwise → **E2E_REMEDIATE**.

### E2E_REMEDIATE (e2e loop — the fixer)
1. A failure may be a real UI defect **or** a faulty test — diagnose which.
2. Fix the UI under test or the test; re-run the suite. No commit, no `--message`.
3. `forgectl advance` → back to E2E_VERIFY (`e2e_round` increments).

### COMMIT
The batch is terminal after the e2e loop. Items are marked `passed` (all three loops passed within budget) or `failed` (any loop force-accepted).
1. Add a log entry to the implementation log.
2. Advance: `forgectl advance --message "<commit message>"` (when `enable_commits: true`; the scaffold stages per `commit_strategy` and commits). When `false`, `forgectl advance`.
3. Next state: ORIENT (more work) or DONE.

### DONE
All layers and items complete. The summary reports per-loop round totals (`code`, `qa`, `e2e`). Add a final log entry. If plans remain, the scaffold transitions via PHASE_SHIFT.

---

## The `handoff` command — how "review the agent's file" happens

`handoff` is the sub-agent's artifact-return command, the counterpart to `eval`:

- `eval` hands context **in** to the sub-agent.
- `handoff` hands the sub-agent's files **back** to the scaffold.

After producing its files, the sub-agent runs `forgectl handoff <file>…`. The scaffold verifies each file exists, registers the paths for the current round, and surfaces them on the `Review:` line of the next `status`/`advance` — so you review exactly what the agent produced before recording a verdict. `handoff` carries no verdict; you still record it with `advance --verdict`.

`handoff` **coexists** with `--eval-report` — they are independent. If your `--eval-report` path differs from the report that was handed off, forgectl WARNs (it does not block) and records the `--eval-report` path.

Rejections (exit 1): `handoff` outside an evaluator state; `handoff` with no files; `handoff` naming a file that does not exist.

---

## Important Details

- **Round 1 IMPLEMENT** requires `--message` (auto-commit per item). **Round 2+ IMPLEMENT** does not.
- **ORIENT** is a guided pause when `general.user_guided` is true — stop and discuss.
- The three loops count rounds **independently** (`eval_round`, `qa_round`, `e2e_round`), reset when a batch is selected.
- The **step list is always produced** every QA round, in every `eval_mode`, and must be present to leave the QA loop toward E2E_AUTHOR.
- **E2E tests derive only from the QA step list** — the most recent one wins.
- Forgectl tracks `passes` and `rounds` in plan.json automatically. Do not edit those fields manually.
- Subsequent QA_TEST / E2E_VERIFY `eval` output includes a `--- PREVIOUS EVALUATIONS ---` section with prior round verdicts and report paths.
