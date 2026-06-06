# Forgectl Workflow Reference

The skill defines *what* you do — two steps, Generation and Reconciliation (see
[../SKILL.md](../SKILL.md)). This reference says *where in the forgectl workflow* each happens.

Forgectl drives a state machine, does the bookkeeping, and prints an `Action:` block at every state.
**You** do the work; forgectl runs no subprocess. The state you are in assigns your role — read only
the references for that role.

> **The state is your role.** `EXECUTE_REVERSE_ENGINEER` = isolated, one-topic **Generation**.
> `RECONCILE` = global-knowledge **Reconciliation**. Do not blur the two.

## State → Skill Step Map

| State | What you do | Governing reference |
|-------|-------------|---------------------|
| ORIENT | Confirm concept and domain order. | — |
| SURVEY | Inventory the domain's existing specs. | — |
| GAP_ANALYSIS | Find behavior no spec covers; note partial coverage. | [dedup-specs.md](dedup-specs.md) |
| DECOMPOSE | Decide topics + create-vs-update action per spec. | [../../shared/topic-of-concern.md](../../shared/topic-of-concern.md), [dedup-specs.md](dedup-specs.md) |
| QUEUE | Write the queue file. | — |
| **EXECUTE_REVERSE_ENGINEER** | **Step 1 — Generation:** author one spec from code, in isolation. | **[reverse-engineering-one-spec.md](reverse-engineering-one-spec.md)** |
| POST_REVERSE_ENGINEER | Checkpoint; clear context before the next item. | — |
| **RECONCILE** | **Step 2 — Reconciliation:** cross-reference and mesh specs. | **[cross-spec-shared-behavior.md](cross-spec-shared-behavior.md)**, [dedup-specs.md](dedup-specs.md) |
| RECONCILE_EVAL | Sub-agents score consistency via `forgectl eval`. | — |
| COLLEAGUE_REVIEW | Human gate (only if enabled). | — |
| RECONCILE_ADVANCE | Transition between domains. | — |
| DONE | Workflow complete. | — |

The two **bold** states are this skill's two steps; everything above EXECUTE is preparatory scoping.

## Commands

```bash
forgectl init --phase reverse_engineering --from <input.json>   # start (concept + ordered domains)
forgectl status                                                 # current state + Action block
forgectl advance                                                # next state
forgectl advance --verdict PASS|FAIL --eval-report <path>       # RECONCILE_EVAL only
forgectl eval                                                   # RECONCILE_EVAL only — emits evaluator prompt
forgectl add-domain <domain>                                    # QUEUE only
```

Run from the project root. Sub-agent counts/models and reconcile round limits live in
`.forgectl/config`. QUEUE takes **no** `--file` flag — the queue is fixed at
`.forgectl/state/reverse-engineering-queue.json`.

The full state machine — preconditions, action output, transitions, error handling — is specified in
`forgectl/specs/reverse-engineering.md` (authoritative). Always read the live `Action:` block.

## Key rules

- **The state names your role** — Generation in EXECUTE, Reconciliation in RECONCILE. Don't mix.
- **Generation records boundaries as observed** — no dedup, no canonical names; that is Step 2.
- **Reconciliation wires existing specs** — it does not re-read code for new behavior.
- **Read-only codebase**, one file per EXECUTE item, failed items skipped silently.
- `forgectl eval` is gated to RECONCILE_EVAL; `forgectl add-domain` to QUEUE.
