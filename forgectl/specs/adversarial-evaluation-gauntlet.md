# Adversarial Evaluation Gauntlet

## Topic of Concern
> A generated workflow implements each plan batch with one primary pass and then hardens it through an evaluator that mutates the code directly, looping a bounded number of rounds until a round makes no changes.

## Context

This spec defines the **run-time contract** of the workflow emitted by **workflow-generation** — what the script *does* when invoked, independent of how it was produced. It is the behavioral counterpart to that generator spec: where workflow-generation commits to *writing a conforming script*, this spec commits to *what conformance means*.

The gauntlet differs deliberately from forgectl's interactive `implementing` phase. There, evaluation is verdict-driven: an evaluator writes a report or speaks a verdict, the engineer records `PASS`/`FAIL` with `forgectl advance`, and the scaffold never parses the evaluation. In the gauntlet there is no human and no scaffold at run time. The evaluator is **adversarial and mutating**: it does not report — it *fixes*. The signal is not a verdict word but the **presence or absence of a code change**. A round in which the evaluator changes nothing means the work has converged; a round in which it changes something means the previous pass was insufficient and another round is warranted.

The whole batch is presented to every agent at once — unlike the interactive phase, which presents one item per advance. This keeps the generated script simple ("easy" by design): one primary call, then a single bounded loop.

## Depends On
- **workflow-generation** — produces the script that embodies this contract and bakes in the batches, prompts, and loop parameters.
- **plan-production** — defines the per-item fields (`description`, `steps`, `files`, `specs`, `tests`) that compose the agent prompts.
- **config-scaffolding** — defines the `[implementing]` and `[general]` fields whose generation-time values parameterize the loop.

## Integration Points

| Adjacent concern | Relationship |
|------------------|-------------|
| workflow-generation | Owns artifact production; this spec owns artifact behavior. The two share the batch model and the config-to-behavior mapping. |
| plan-production | The primary prompt is built from item fields; the gauntlet treats each item's `tests` as the acceptance criteria the evaluator enforces. |
| config-scaffolding | `implementing.batch`, `implementing.implement.{model,type}`, `implementing.eval.{model,type,min_rounds,max_rounds}`, and `general.enable_commits` are baked in at generation and govern this behavior. |
| Embedded evaluator prompt | The evaluator agent runs from a dedicated prompt (authored in source and embedded in the binary) instructing it to adversarially review the batch and correct the code directly. |
| Claude Code workflow harness | Behavior is expressed through `agent()` calls (primary, evaluator, change-detector), `log()`, and `phase()`. The change-detector uses an agent with `Bash` because the workflow sandbox cannot run git itself. |

---

## Data Models

### Batch
Owned by **workflow-generation** (referenced, not redefined). The running script receives each batch baked in; the whole batch — every item's full record — is passed to every agent in the loop.

### Round Outcome
The externally observable result of one evaluator round.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| round number | integer | yes | no | 1-based within a batch. |
| changed | boolean | yes | no | Whether the evaluator modified the working tree this round, as determined by the change-detector (git is authoritative). |

---

## Interface

### Inputs
- **The codebase** as a git working tree — the substrate the primary and evaluator read and modify.
- **The baked batches** — ordered item groups embedded in the script.
- **The baked parameters** — `min_rounds`, `max_rounds`, primary and evaluator `model`/`type`, `batch` size, and the `enable_commits` flag, all fixed at generation time.

### Outputs
- **A mutated working tree**: the primary's implementation plus every accepted evaluator correction.
- **Commits** (one per batch boundary, or per the baked commit instruction) **only when `general.enable_commits` was true** at generation.
- **Progress logs**: batch start, per-round changed/clean status, convergence, and force-accept events.

### Rejection
The gauntlet does not reject inputs at run time — the script was validated and compiled at generation. Its only terminal "failure" mode is **force-accept**: a batch that never converges within `max_rounds` is accepted in whatever state the last round left it, with a warning. There is no run-time path that aborts the whole workflow on a non-converging batch.

---

## Behavior

### Implement a Batch

#### Preconditions
- The batch's items are available in the script.
- The working tree is in a runnable state (prior batches, if any, completed).

#### Steps
1. **Primary pass (once).** Spawn one primary agent with the configured `implement` model/type, presenting the **whole batch**: for each item, its `description`, `steps`, `files`, `specs`, and `tests` (acceptance criteria). The primary implements all items in the batch. The primary runs **exactly once per batch** — it is not re-run on subsequent rounds.
2. **Evaluator loop.** Run rounds `1..max_rounds`:
   1. Spawn one evaluator agent with the configured `eval` model/type, presenting the whole batch and the embedded adversarial-review instructions. The evaluator reviews the primary's (and any prior round's) work against each item's `tests` and **edits the code directly** to correct any deficiency it finds.
   2. Run the **change-detector**: a lightweight agent (haiku) that inspects the working tree with git and reports whether the evaluator changed anything. When `enable_commits` was baked true, this agent also commits the change. Git is the authoritative signal — the evaluator's self-description is not relied upon.
   3. If `round >= min_rounds` **and** the round's `changed` is false, **end the loop** — the batch has converged.
   4. Otherwise continue to the next round.
3. **Force-accept.** If the loop reaches `max_rounds` while rounds are still changing, accept the batch in its current state and log a force-accept warning.
4. Proceed to the next batch in run order.

#### Postconditions
- The primary ran exactly once for the batch.
- The evaluator ran at least `min_rounds` times (and at least once when `max_rounds >= 1`) and at most `max_rounds` times.
- The batch ended either by convergence (a round at or beyond `min_rounds` that changed nothing) or by force-accept at `max_rounds`.
- If `enable_commits` was true, the batch's accepted state is committed.

#### Error Handling
- **Primary agent fails / returns null:** the round still proceeds to evaluation; the evaluator operates on whatever the primary produced. A wholly empty primary result leaves the evaluator to implement against the batch's tests. The batch is not aborted.
- **Evaluator agent fails / returns null:** the change-detector treats the round as **no change** (nothing was applied); subject to the `min_rounds` floor this can end the loop. The workflow does not abort.
- **Change-detector cannot run git:** the round is treated as **changed** (conservative), forcing the loop to continue toward `max_rounds` rather than declaring a false convergence.

### Convergence Signal

#### Preconditions
- At least `min_rounds` rounds have completed.

#### Steps
- A round whose change-detector reports `changed = false` is a convergence round.

#### Postconditions
- The first convergence round at or beyond `min_rounds` ends the batch; no further evaluator rounds run for that batch.

#### Error Handling
- A `changed = false` round **before** `min_rounds` is reached does **not** end the loop; the loop continues until the floor is satisfied.

---

## Configuration

All values are fixed at generation time (see workflow-generation). At run time they are constants.

| Parameter | Type | Default | Effect on run-time behavior |
|-----------|------|---------|------------------------------|
| `implementing.batch` | int | 2 | Items per batch; the whole batch is presented to the primary and the evaluator each round. |
| `implementing.implement.model` | string | `sonnet` | Model of the single primary agent. |
| `implementing.implement.type` | string | `general-purpose` | Agent type of the primary. |
| `implementing.eval.model` | string | `sonnet` | Model of the evaluator agent. |
| `implementing.eval.type` | string | `general-purpose` | Agent type of the evaluator. |
| `implementing.eval.min_rounds` | int | 1 | Floor: the evaluator runs at least this many rounds before an unchanged round can end the batch. |
| `implementing.eval.max_rounds` | int | 3 | Ceiling: after this many rounds the batch force-accepts. |
| `general.enable_commits` | bool | false | When true, prompts carry commit instructions and the change-detector commits each accepted batch; when false, no commits and no commit instructions. |

`implementing.eval.count` does not affect this behavior: exactly one evaluator runs per round. Round count — not evaluator fan-out — is the only loop dimension. Config validation guarantees `min_rounds <= max_rounds`.

---

## Observability

### Logging
| Level | What is logged |
|-------|---------------|
| INFO | Batch started (index, item ids); each round's outcome (round number, changed/clean); batch converged at round N; batch committed (when `enable_commits`). |
| WARN | Force-accept: a batch reached `max_rounds` still changing and was accepted as-is. |
| ERROR | Change-detector could not run git (round treated as changed); primary or evaluator agent error surfaced for diagnostics. |
| DEBUG | The model/type used for each agent; the resolved `min_rounds`/`max_rounds` bounds. |

### Metrics
None — the script reports through `log()` lines in the workflow progress view.

---

## Invariants

- The primary agent runs **exactly once per batch**, regardless of how many evaluator rounds occur.
- The evaluator runs **at least `min_rounds`** rounds and **never more than `max_rounds`** rounds per batch. When `max_rounds >= 1`, the loop always runs at least one evaluator round before the convergence check, so the effective floor is `max(min_rounds, 1)` for any non-degenerate configuration.
- A batch ends in exactly one of two ways: **convergence** (a round at or beyond `min_rounds` with no change) or **force-accept** (at `max_rounds`).
- The **change-detector's git result**, not any agent's self-report, determines whether a round changed the code.
- Commits occur **iff** `general.enable_commits` was true at generation time.
- Every agent in the loop receives the **whole batch** — never a single item in isolation.
- Exactly **one** evaluator runs per round; `count` never increases evaluator fan-out in a generated workflow.

---

## Edge Cases

- **Scenario:** `min_rounds = 1`, `max_rounds = 3`, and the evaluator changes nothing on round 1.
  **Expected behavior:** The batch ends after one evaluator round.
  **Rationale:** The floor is satisfied and the round converged.

- **Scenario:** `min_rounds = 2` and the evaluator changes nothing on round 1.
  **Expected behavior:** A second round runs anyway; if it also changes nothing the batch ends at round 2.
  **Rationale:** The floor forces at least two rounds even when the first converges.

- **Scenario:** The evaluator changes code on every round through `max_rounds`.
  **Expected behavior:** At `max_rounds` the batch is force-accepted in its current state with a WARN; the workflow proceeds to the next batch.
  **Rationale:** A self-contained run has no human to halt for; bounded looping prevents an infinite gauntlet.

- **Scenario:** `min_rounds = 0`.
  **Expected behavior:** The evaluator may end the batch on the first unchanged round with no floor; if it changes something it still loops to at most `max_rounds`.
  **Rationale:** A zero floor means "no minimum"; the ceiling still bounds the loop.

- **Scenario:** The evaluator believes it made a change but git shows none (e.g., it rewrote a file identically).
  **Expected behavior:** The change-detector reports `changed = false`; the round counts as converged.
  **Rationale:** Git is authoritative; a no-op edit is not a real change.

- **Scenario:** `general.enable_commits` is false and the gauntlet force-accepts.
  **Expected behavior:** The mutated working tree is left uncommitted for the operator; no commit is made and no prompt mentioned committing.
  **Rationale:** Commit behavior is strictly gated on the baked flag.

- **Scenario:** `max_rounds = 0` (with `min_rounds = 0`, which config validation permits).
  **Expected behavior:** No evaluator round runs; the batch is accepted on the primary pass alone, logged as a force-accept.
  **Rationale:** The loop bound is zero, so there is nothing to run; the primary's output stands, surfaced as force-accept so the absence of review is visible.

- **Scenario:** The evaluator agent errors or returns null on a round at or beyond `min_rounds`.
  **Expected behavior:** The change-detector observes no working-tree change, the round is recorded `changed = false`, and the batch converges — even though no real review occurred. The failure is surfaced at ERROR.
  **Rationale:** Optimistic convergence keeps a self-contained run progressing rather than stalling; the trade-off is that an evaluator crash can end a batch without review, which the ERROR log makes diagnosable. This is intentionally asymmetric with a change-detector git failure, which is treated conservatively as `changed` to avoid a false convergence on the authoritative signal.

---

## Testing Criteria

The loop-control criteria below (primary count, convergence, minimum rounds, force-accept, count-ignored) are verified against **deterministic agent stand-ins** — test doubles whose change/no-change behavior per round is fixed — since real agent output is non-deterministic. The stand-ins exercise the loop's control flow; the agents' actual reasoning is out of scope for these criteria.

### Primary runs once
- **Verifies:** Primary-once invariant.
- **Given:** A generated workflow for a batch with `max_rounds = 3`.
- **When:** The batch runs and the evaluator changes code across all three rounds.
- **Then:** Exactly one primary agent invocation occurs; three evaluator invocations occur.

### Convergence ends the loop
- **Verifies:** Convergence Signal; round-bound invariant.
- **Given:** `min_rounds = 1`, `max_rounds = 5`; the evaluator changes nothing on round 2 (after changing something on round 1).
- **When:** The batch runs.
- **Then:** Exactly two evaluator rounds run and the batch is recorded as converged at round 2.

### Minimum rounds enforced
- **Verifies:** `min_rounds` floor edge case.
- **Given:** `min_rounds = 2`; the evaluator changes nothing on round 1.
- **When:** The batch runs.
- **Then:** A second evaluator round runs before the batch can end.

### Force-accept at ceiling
- **Verifies:** Force-accept behavior; round-bound invariant.
- **Given:** `max_rounds = 3`; the evaluator changes code every round.
- **When:** The batch runs.
- **Then:** Exactly three evaluator rounds run, the batch is accepted in its final state, and a WARN force-accept line is logged.

### Git is authoritative for change detection
- **Verifies:** Change-detector invariant.
- **Given:** An evaluator whose self-report claims a change but whose edits leave the working tree identical.
- **When:** The change-detector runs.
- **Then:** The round is recorded as `changed = false`.

### Commits gated by config
- **Verifies:** Commit-gating invariant; enable_commits edge cases.
- **Given:** Two generated workflows differing only in `general.enable_commits`.
- **When:** Each runs an identical converging batch.
- **Then:** The `enable_commits = true` run produces a commit for the batch; the `false` run produces none and its prompts contain no commit instructions.

### Whole batch presented
- **Verifies:** Whole-batch invariant.
- **Given:** A batch of two items.
- **When:** The primary and evaluator are invoked.
- **Then:** Each agent's prompt contains both items' `description`/`steps`/`files`/`specs`/`tests`.

### One evaluator per round regardless of count
- **Verifies:** Count-exclusion invariant (runtime).
- **Given:** A workflow generated from a config with `implementing.eval.count = 4`.
- **When:** A batch runs a round.
- **Then:** Exactly one evaluator agent is invoked per round; the configured count does not increase evaluator fan-out.

---

## Implements
- The adversarial mutating evaluation loop: a once-through primary implementation followed by a bounded, convergence-terminated evaluator gauntlet that corrects code directly, parameterized by the implementing configuration.
