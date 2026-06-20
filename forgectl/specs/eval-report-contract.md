# Eval Report Contract

## Topic of Concern
> Every report-mode evaluation hands a written report file from the eval sub-agent to the engineer through a path the scaffold supplies to both actors.

## Context

Every phase that runs sub-agent evaluation (`specifying` EVALUATE / CROSS_REFERENCE_EVAL / RECONCILE_EVAL, `planning` EVALUATE, `implementing` EVALUATE, `reverse_engineering` RECONCILE_EVAL) shares the same two-actor handoff. The scaffold is a prompt generator: it does not run the evaluation itself, it prints instructions that one agent (the engineer) reads and partly delegates to a second agent (the eval sub-agent). When `eval_mode` is `"report"`, the deliverable that crosses the boundary between those two agents is a **report file on disk**, referenced by **path**.

This spec defines that contract once so every phase emits it identically. Phase specs (`spec-lifecycle`, `plan-production`, `spec-reconciliation`, `batch-implementation`, `reverse-engineering`) describe their state machines and reference this contract for the report-mode eval handoff; they do not redefine it.

The contract exists because two failure modes are otherwise common:

1. The eval sub-agent **describes its findings in prose but never writes the file**, so no report exists at the path.
2. The engineer, lacking a path, **passes the report prose as the `--eval-report` value** — which is a file-path argument — and `advance` rejects it because no such file exists.

Both are prevented by surfacing the same deterministic path to both actors and stating, imperatively, that the sub-agent must create the file and the engineer must pass that path.

## Depends On
- **state-persistence** — the round/batch counters that determine the report path.

## Integration Points

| Spec | Relationship |
|------|-------------|
| spec-lifecycle | EVALUATE and CROSS_REFERENCE_EVAL follow this contract in report mode. |
| spec-reconciliation | RECONCILE_EVAL follows this contract in report mode. |
| plan-production | EVALUATE follows this contract in report mode. |
| batch-implementation | EVALUATE follows this contract in report mode; carries the fully worked example. |
| Evaluator prompts (`evaluators/*.md`, embedded) | Each evaluator prompt's report section follows the shared report-writing instruction defined here. |

---

## Interface

### `eval_mode` applicability

This contract applies only when the phase's resolved `eval_mode` is `"report"`. In `"direct"` mode the sub-agent corrects files in place and no report path is involved; in `"conversational"` mode the sub-agent reports verbally and no `--- REPORT OUTPUT ---` section is emitted.

### The deterministic report path

The scaffold computes the report path from the phase, batch/round counters, and plan/domain directory. It is **deterministic** — the same inputs always yield the same path — so the scaffold can print it before the report is written. Per-phase path shapes:

| Phase / state | Report path |
|---------------|-------------|
| specifying EVALUATE | `<domain>/specs/.eval/batch-N-rM.md` |
| specifying CROSS_REFERENCE_EVAL | (cross-reference eval path, per spec-lifecycle) |
| specifying / reverse_engineering RECONCILE_EVAL | (reconcile eval path, per spec-reconciliation / reverse-engineering) |
| planning EVALUATE | `<plan-dir>/evals/round-N.md` |
| implementing EVALUATE | `<plan-dir>/evals/batch-N-round-M.md` |

### Surfaced in two places, identically

In report mode the scaffold prints the **same** path in two outputs:

1. The **EVALUATE Action** (`forgectl status` / `forgectl advance` output) — the engineer's copy. The Action names the exact path and states that `--eval-report` takes that file path, not the report text.
2. The **`--- REPORT OUTPUT ---` section** of `forgectl eval` — the sub-agent's copy. It instructs the sub-agent to create the file at that path and to return the path and verdict.

The two paths are byte-identical for the same batch/round.

#### EVALUATE Action (report mode)

```
Action:   Please spawn 1 sonnet general-purpose sub-agent to evaluate ...
          The sub-agent should run: forgectl eval
          The sub-agent must write its report to this exact path:
            <report-path>
          After completion of the above, advance with --eval-report <report-path> --verdict PASS|FAIL
          --eval-report takes this file path, not the report text.
```

#### `forgectl eval` REPORT OUTPUT (report mode)

```
--- REPORT OUTPUT ---

Write your evaluation report to this exact path (create the file — do not only
describe it):
  <report-path>

When done, your final message must be only this path and the verdict, e.g.:
  <report-path> FAIL
```

---

## Behavior

### Two actors, one path

- **Eval sub-agent.** Runs `forgectl eval`, reads the evaluator prompt and the `--- REPORT OUTPUT ---` path, performs the evaluation, **creates the report file at that path** using its file-writing tool, and returns the path and verdict as its final message. Describing findings without writing the file is a contract violation.
- **Engineer.** Reads the EVALUATE Action, spawns the sub-agent, then runs `forgectl advance --eval-report <report-path> --verdict PASS|FAIL` using the path from the Action (identical to the path the sub-agent wrote). The engineer does not invent a path the sub-agent did not write, and never passes report prose as the `--eval-report` value.

### Path validation on `advance`

Before recording the eval, `advance` stats the `--eval-report` value. If it is not an existing file, `advance` fails. When the value looks like report prose rather than a path — it contains whitespace and no path separator — the error additionally states that `--eval-report` expects the file path the eval sub-agent wrote, not the report text.

---

## Invariants

1. **One source of truth for the path.** The scaffold computes the report path; neither actor reconstructs it. The path in the EVALUATE Action and the path in `forgectl eval` REPORT OUTPUT are identical for the same batch/round.
2. **Sub-agent writes, does not merely describe.** In report mode the sub-agent must create the file at the printed path. Its return message is the path and verdict.
3. **`--eval-report` is a path, not prose.** The value is always a file path. `advance` validates existence and, for prose-shaped values, emits the path-vs-text hint.
4. **Mode-gated.** This contract applies only in `eval_mode: "report"`. In `"direct"` and `"conversational"` modes no report path is surfaced or required.

---

## Verification

### Report path appears in both the Action and the eval output
- **Verifies:** Invariant 1.
- **Given:** A report-mode eval state with a known batch/round.
- **When:** `forgectl status` (or `advance` into the state) and `forgectl eval` are run.
- **Then:** Both outputs contain the same report path string.

### REPORT OUTPUT instructs file creation and return contract
- **Verifies:** Invariant 2.
- **Given:** A report-mode eval state.
- **When:** `forgectl eval`
- **Then:** The `--- REPORT OUTPUT ---` section instructs creating the file at the path and returning the path and verdict.

### EVALUATE Action states `--eval-report` takes a path
- **Verifies:** Invariant 3.
- **Given:** A report-mode eval state.
- **When:** `forgectl status`
- **Then:** The Action names the exact path and states `--eval-report` takes the file path, not the report text.

### Prose passed to `--eval-report` produces a path-vs-text hint
- **Verifies:** Invariant 3.
- **Given:** A report-mode EVALUATE state.
- **When:** `advance --verdict FAIL --eval-report "<prose with spaces and no slash>"`
- **Then:** Error naming the value with a hint that `--eval-report` expects the file path the sub-agent wrote. Exit code 1.

### Direct and conversational modes omit the contract
- **Verifies:** Invariant 4.
- **Given:** An eval state with `eval_mode: "direct"` or `"conversational"`.
- **When:** `forgectl eval` and `forgectl status`
- **Then:** No report path is surfaced; `--eval-report` is not requested.
