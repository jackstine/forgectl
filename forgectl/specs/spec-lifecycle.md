# Spec Lifecycle

## Topic of Concern
> The scaffold sequences batched spec drafting through iterative evaluation rounds, with domain-scoped cross-referencing after each domain completes.

## Context

The specifying phase guides an architect through drafting, evaluating, refining, and accepting specs from a queue. Specs are processed in batches of up to `specifying.batch` (default 3), grouped by domain — a batch never mixes domains. If a domain has more specs than the batch size, it produces multiple batches before moving to the next domain.

The eval sub-agent evaluates all specs in the batch together. The entire batch cycles through DRAFT → EVALUATE → REFINE until accepted (or force-accepted at `specifying.eval.max_rounds`). Accepted specs move to the completed list and the next batch is pulled from the queue.

After the last batch for a domain is accepted, the scaffold enters a domain-scoped CROSS_REFERENCE phase. This cross-references ALL specs in the domain (session specs and existing specs) before moving to the next domain.

When `enable_commits` is `true`, the specifying phase produces a single commit at COMPLETE (in spec-reconciliation) covering all spec work. No commits are made during individual eval rounds, refinements, or cross-referencing. See `docs/auto-committing.md` for full auto-commit behavior and staging strategies.

The scaffold does not spawn sub-agents. It outputs instructions telling the architect what to spawn. The architect (or the skill driving the session) is responsible for spawning them.

This spec covers the spec lifecycle from ORIENT through DONE. Cross-domain reconciliation after all specs are complete is covered by spec-reconciliation.

## Depends On
- **state-persistence** — reads and writes the state file.
- **session-init** — populates the spec queue during `init --phase specifying`.

## Integration Points

| Spec | Relationship |
|------|-------------|
| Specs skill | The DRAFT and REFINE action outputs reference skill files (`references/spec-format.md`, `references/spec-generation-skill.md`, `references/topic-of-concern.md`) that define the authoring process; the scaffold enforces the state machine that sequences it |
| Spec evaluation prompt (`evaluators/spec-eval.md`, embedded in binary) | Full instructions for the spec evaluation sub-agent: what to check, report format, verdict rules |
| Cross-reference evaluation prompt (`evaluators/cross-reference-eval.md`, embedded in binary) | Full instructions for the cross-reference evaluation sub-agent |
| Spec generation sub-agent | The specifying EVALUATE state is where the architect spawns a sub-agent to evaluate the batch; the scaffold tracks round count, verdict, and eval report path |
| spec-reconciliation | Receives the completed specs list when the queue is exhausted (DONE state) |
| spec-reconciliation | Completed specs receive commit hashes (via `commit_hashes` field) through auto-commit at COMPLETE when `enable_commits: true` |
| Eval output directory | The specifying eval sub-agent writes output to `<domain>/specs/.eval/`; the scaffold does not read these files but the convention is documented |
| phase-transitions | Completed specs, commit hashes, and code search roots feed into auto-generated plan-queue at generate_planning_queue phase |

---

## Interface

### Inputs

#### `advance` flags — Specifying Phase

| State | Flags |
|-------|-------|
| DRAFT | (no flags) |
| EVALUATE | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `enable_eval_output: true`) |
| REFINE | (no flags) |
| CROSS_REFERENCE | (no flags) |
| CROSS_REFERENCE_EVAL | `--verdict PASS\|FAIL` (required), `--eval-report <path>` (required when `enable_eval_output: true`) |
| CROSS_REFERENCE_REVIEW | (no flags) |

The `--guided` / `--no-guided` flags are accepted on any `advance` call regardless of state.

#### `eval` command

| Command | Flags | Description |
|---------|-------|-------------|
| `eval` | none | Output full evaluation context for the sub-agent. Only valid in specifying EVALUATE or CROSS_REFERENCE_EVAL. |

#### `add-queue-item` — Add spec to queue

Appends a spec to the specifying queue. Only valid in DRAFT, CROSS_REFERENCE_REVIEW, DONE, or RECONCILE_REVIEW states within the specifying phase.

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | yes | Display name for the spec |
| `--domain` | at DONE only | Domain this spec belongs to. Inferred from current domain at DRAFT and CROSS_REFERENCE_REVIEW. Required at DONE (no current domain). Optional override at other states. |
| `--topic` | yes | One-sentence topic of concern |
| `--file` | yes | Target spec file path (relative to project root) |
| `--source` | no | Planning source path (repeatable for multiple sources) |

#### `set-roots` — Set code search roots for a domain

Stores code search roots for use during the planning phase. Only valid in CROSS_REFERENCE_REVIEW or DONE states within the specifying phase.

| Flag | Required | Description |
|------|----------|-------------|
| `--domain` | at DONE only | Domain to set roots for. Inferred from current domain at CROSS_REFERENCE_REVIEW. Required at DONE (no current domain). |
| (positional) | yes | One or more directory paths |

### Outputs

#### `advance` output

**Entering SELECT** (after ORIENT):

```
State:   SELECT
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Specs:
  [1] Repository Loading
      File:    repository-loading.md
      Topic:   The optimizer clones or locates a repository and provides its path for downstream modules
      Sources: .forge_workspace/planning/optimizer/repo-snapshot-loading.md
  [2] Snapshot Diffing
      File:    snapshot-diffing.md
      Topic:   The optimizer compares repository snapshots to detect meaningful changes
      Sources: .forge_workspace/planning/optimizer/snapshot-diffing.md
  [3] Cache Invalidation
      File:    cache-invalidation.md
      Topic:   The optimizer invalidates cached results when upstream inputs change
      Sources: .forge_workspace/planning/optimizer/cache-invalidation.md
Action:  Study each planning source.
         Study each spec doc that exists.
         STOP please review and discuss with user before continuing.
         After completion of the above, advance to begin drafting.
```

Spec file paths are relative to `<domain_path>/specs/`. Planning source paths are project-root-relative.

**Entering DRAFT** (after SELECT):

```
State:   DRAFT
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Specs:
  [1] repository-loading.md
      Sources: .forge_workspace/planning/optimizer/repo-snapshot-loading.md
  [2] snapshot-diffing.md
      Sources: .forge_workspace/planning/optimizer/snapshot-diffing.md
  [3] cache-invalidation.md
      Sources: .forge_workspace/planning/optimizer/cache-invalidation.md
Action:  Draft all specs in the batch using the spec skill.
         Format:    references/spec-format.md
         Process:   references/spec-generation-skill.md
         Scoping:   references/topic-of-concern.md
         If a topic needs splitting or a missing spec is identified,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --topic <topic> --file <file> [--source <path>...]
         After completion of the above, advance to begin evaluation.
```

**Entering EVALUATE** (after DRAFT or REFINE, `enable_eval_output: true`):

```
State:   EVALUATE
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Round:   1/3
Specs:
  [1] repository-loading.md
  [2] snapshot-diffing.md
  [3] cache-invalidation.md
Action:  Please spawn 1 opus sub-agent to evaluate the spec batch.
         The sub-agent should run: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>
```

**Entering EVALUATE** (after DRAFT or REFINE, `enable_eval_output: false`):

```
State:   EVALUATE
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Round:   1/3
Specs:
  [1] repository-loading.md
  [2] snapshot-diffing.md
  [3] cache-invalidation.md
Action:  Please spawn 1 opus sub-agent to evaluate the spec batch.
         The sub-agent should run: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL
```

**Entering REFINE** (after EVALUATE FAIL or PASS below min_rounds, `enable_eval_output: true`):

```
State:   REFINE
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Round:   1/3
Eval:    optimizer/specs/.eval/batch-1-r1.md
Specs:
  [1] repository-loading.md
  [2] snapshot-diffing.md
  [3] cache-invalidation.md
Action:  Study the eval file "optimizer/specs/.eval/batch-1-r1.md"
         and implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Format:      references/spec-format.md
         Process:     references/spec-generation-skill.md
         Scoping:     references/topic-of-concern.md
         After completion of the above, advance to continue evaluation.
```

**Entering REFINE** (after EVALUATE FAIL or PASS below min_rounds, `enable_eval_output: false`):

```
State:   REFINE
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Round:   1/3
Specs:
  [1] repository-loading.md
  [2] snapshot-diffing.md
  [3] cache-invalidation.md
Action:  Make corrections based off communication with the evaluator.
         Implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Format:      references/spec-format.md
         Process:     references/spec-generation-skill.md
         Scoping:     references/topic-of-concern.md
         After completion of the above, advance to continue evaluation.
```

**Entering ACCEPT** (after EVALUATE PASS, round >= min_rounds):

```
State:   ACCEPT
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs accepted
Round:   2/3
Action:  Batch accepted.
         After completion of the above, advance to continue.
```

**Entering CROSS_REFERENCE** (after ACCEPT, domain complete):

```
State:   CROSS_REFERENCE
Phase:   specifying
Domain:  optimizer
Path:    optimizer/

Specs in domain:
  [session — completed]
    repository-loading.md (batch 1)
    snapshot-diffing.md (batch 1)
    cache-invalidation.md (batch 2)
  [existing — not in queue]
    configuration-models.md
    telemetry-pipeline.md

Action:  Please spawn 3 haiku sub-agents to cross-reference ALL specs in this domain.
         Assign each sub-agent a subset of specs to review against the others.
         Fix any findings.
         After completion of the above, advance to begin evaluation.
```

**Entering CROSS_REFERENCE_EVAL** (after CROSS_REFERENCE, `enable_eval_output: true`):

```
State:   CROSS_REFERENCE_EVAL
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Round:   1/2

Action:  Please spawn 1 opus sub-agent to evaluate cross-reference consistency.
         The sub-agent should run: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>
```

**Entering CROSS_REFERENCE_EVAL** (after CROSS_REFERENCE, `enable_eval_output: false`):

```
State:   CROSS_REFERENCE_EVAL
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Round:   1/2

Action:  Please spawn 1 opus sub-agent to evaluate cross-reference consistency.
         The sub-agent should run: forgectl eval
         After completion of the above, advance with --verdict PASS|FAIL
```

**Entering CROSS_REFERENCE_REVIEW** (after first CROSS_REFERENCE_EVAL):

CROSS_REFERENCE_REVIEW always fires once per domain after the first passing cross-reference eval. The action output varies based on `user_review`:

When `user_review: true`:

```
State:   CROSS_REFERENCE_REVIEW
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Round:   1/2
Verdict: PASS
Eval:    optimizer/specs/.eval/cross-reference-r1.md

Action:  STOP please review and discuss with user before continuing.
         If additional specs are needed for this domain,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --topic <topic> --file <file> [--source <path>...]
         Set code search roots for this domain (used in planning phase):
           forgectl set-roots <path> [<path>...]
         After completion of the above, advance to continue.
```

When `user_review: false`:

```
State:   CROSS_REFERENCE_REVIEW
Phase:   specifying
Domain:  optimizer
Path:    optimizer/
Round:   1/2
Verdict: PASS
Eval:    optimizer/specs/.eval/cross-reference-r1.md

Action:  Domain cross-reference complete.
         If additional specs are needed for this domain,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --topic <topic> --file <file> [--source <path>...]
         Set code search roots for this domain (used in planning phase):
           forgectl set-roots <path> [<path>...]
         After completion of the above, advance to continue.
```

**Entering DONE** (after last domain's cross-reference complete, queue empty):

```
State:   DONE
Phase:   specifying
Specs:   5 completed
Action:  All individual specs complete.
         If additional specs are needed,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --domain <domain> --topic <topic> --file <file> [--source <path>...]
           Adding specs here re-enters ORIENT for the new items before reconciliation.
         Set code search roots for any domain not yet configured (used in planning phase):
           forgectl set-roots --domain <domain> <path> [<path>...]
         When ready, advance to begin reconciliation.
```

#### Eval Report Locations

Specifying eval reports follow this convention:
```
<domain>/specs/.eval/batch-N-rM.md
```

Cross-reference eval reports:
```
<domain>/specs/.eval/cross-reference-rN.md
```

#### `status` output — Specifying (compact)

The compact `status` output for specifying shows the current batch, round, action, and a one-line progress summary.

When `enable_eval_output: true`:

```
Batch:   Repository Loading, Snapshot Diffing, Cache Invalidation (optimizer)
Round:   1/3

Action:  Study the eval file "optimizer/specs/.eval/batch-1-r1.md"
         and implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Format:      references/spec-format.md
         Process:     references/spec-generation-skill.md
         Scoping:     references/topic-of-concern.md
         After completion of the above, advance to continue evaluation.

Progress: 1/5 specs completed, 2 queued
```

When `enable_eval_output: false`:

```
Batch:   Repository Loading, Snapshot Diffing, Cache Invalidation (optimizer)
Round:   1/3

Action:  Make corrections based off communication with the evaluator.
         Implement any corrections as needed.
         Apply "fresh" eyes and a tightened lens when reviewing the work,
         then apply corrections as needed.
         Format:      references/spec-format.md
         Process:     references/spec-generation-skill.md
         Scoping:     references/topic-of-concern.md
         After completion of the above, advance to continue evaluation.

Progress: 1/5 specs completed, 2 queued
```

#### `status --verbose` output — Specifying section

With `--verbose`, the full queue, completed list with eval history, and prior phase summaries are appended.

When `enable_eval_output: true`:

```
--- Queue ---

  [4] Portal Rendering (portal)
  [5] Portal Caching (portal)

--- Completed ---

  [1] Configuration Models (optimizer)  — 2 rounds
       Round 1: FAIL — optimizer/specs/.eval/configuration-models-r1.md
       Round 2: PASS — optimizer/specs/.eval/configuration-models-r2.md
```

When `enable_eval_output: false`:

```
--- Queue ---

  [4] Portal Rendering (portal)
  [5] Portal Caching (portal)

--- Completed ---

  [1] Configuration Models (optimizer)  — 2 rounds
       Round 1: FAIL
       Round 2: PASS
```

#### `eval` output — Specifying EVALUATE

When `enable_eval_output: true`:

```
=== SPEC EVALUATION ROUND 1/3 ===
Domain: optimizer
Batch:  1

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/spec-eval.md>

--- SPECS TO EVALUATE ---

[1] repository-loading.md
    Topic: The scaffold loads repository snapshots for diffing.
    File:  optimizer/specs/repository-loading.md

[2] snapshot-diffing.md
    Topic: The scaffold diffs repository snapshots to detect changes.
    File:  optimizer/specs/snapshot-diffing.md

[3] cache-invalidation.md
    Topic: The scaffold invalidates cached data when specs change.
    File:  optimizer/specs/cache-invalidation.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  optimizer/specs/.eval/batch-1-r1.md
```

Subsequent rounds with `enable_eval_output: true` include previous evaluations:

```
=== SPEC EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: FAIL — optimizer/specs/.eval/batch-1-r1.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  optimizer/specs/.eval/batch-1-r2.md
```

When `enable_eval_output: false`, the `--- REPORT OUTPUT ---` and `--- PREVIOUS EVALUATIONS ---` sections are omitted. The eval sub-agent receives spec details and evaluator instructions but does not write a file. It communicates its verdict directly to the architect.

```
=== SPEC EVALUATION ROUND 1/3 ===
Domain: optimizer
Batch:  1

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/spec-eval.md>

--- SPECS TO EVALUATE ---

[1] repository-loading.md
    Topic: The scaffold loads repository snapshots for diffing.
    File:  optimizer/specs/repository-loading.md

[2] snapshot-diffing.md
    ...

[3] cache-invalidation.md
    ...
```

Subsequent rounds with `enable_eval_output: false`:

```
=== SPEC EVALUATION ROUND 2/3 ===
...

--- PREVIOUS EVALUATIONS ---

Round 1: FAIL
```

#### `eval` output — CROSS_REFERENCE_EVAL

When `enable_eval_output: true`:

```
=== CROSS-REFERENCE EVALUATION ROUND 1/2 ===
Domain: optimizer

--- EVALUATOR INSTRUCTIONS ---

<contents of evaluators/cross-reference-eval.md>

--- SPECS IN DOMAIN ---

[session — completed]
  repository-loading.md   — optimizer/specs/repository-loading.md
  snapshot-diffing.md      — optimizer/specs/snapshot-diffing.md
  cache-invalidation.md    — optimizer/specs/cache-invalidation.md
[existing — not in queue]
  configuration-models.md  — optimizer/specs/configuration-models.md
  telemetry-pipeline.md    — optimizer/specs/telemetry-pipeline.md

--- REPORT OUTPUT ---

Write your evaluation report to:
  optimizer/specs/.eval/cross-reference-r1.md
```

When `enable_eval_output: false`, the `--- REPORT OUTPUT ---` section is omitted.

#### Eval Report Locations

Specifying eval reports:
```
<domain>/specs/.eval/batch-N-rM.md
<domain>/specs/.eval/cross-reference-rN.md
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `advance --verdict` outside of EVALUATE, CROSS_REFERENCE_EVAL | Error naming the current state. Exit code 1. | Verdict is only valid in evaluation states |
| `advance` in specifying EVALUATE without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in specifying EVALUATE without `--eval-report` when `enable_eval_output: true` | Error. Exit code 1. | Every evaluation must reference its report when eval output is enabled |
| `advance --eval-report` pointing to non-existent file | Error naming the path. Exit code 1. | Report must exist to be recorded |
| `advance --eval-report` when `enable_eval_output: false` | Warning: `--eval-report is ignored, eval output is not enabled`. Command proceeds. | Consistent with `--message` warning pattern |
| `advance` in CROSS_REFERENCE_EVAL without `--verdict` | Error. Exit code 1. | Verdict determines the transition |
| `advance` in CROSS_REFERENCE_EVAL without `--eval-report` when `enable_eval_output: true` | Error. Exit code 1. | Every evaluation must reference its report when eval output is enabled |
| `add-queue-item` outside of DRAFT, CROSS_REFERENCE_REVIEW, DONE, or RECONCILE_REVIEW | Error: "add-queue-item is only valid in DRAFT, CROSS_REFERENCE_REVIEW, DONE, or RECONCILE_REVIEW states (current state: \<state\>)." Exit code 1. | Queue modifications are restricted to states where the architect has sufficient context to identify gaps |
| `add-queue-item` outside of specifying phase | Error: "add-queue-item is only valid in the specifying phase (current phase: \<phase\>)." Exit code 1. | Queue items are specs; only the specifying phase manages the spec queue |
| `add-queue-item` with `--file` pointing to non-existent file | Error: "file \<path\> does not exist. add-queue-item registers specs that have already been written. Create the spec file first, then register it." Exit code 1. | The architect must write the spec before registering it; `add-queue-item` is for tracking existing work through evaluation, not queuing future work |
| `add-queue-item` with `--file` not matching `<domain>/specs/<name>.md` | Error: "File path must follow \<domain\>/specs/\<name\>.md convention." Exit code 1. | Spec files must reside in a domain's specs/ directory |
| `add-queue-item` without `--domain` and no matching domain found | Error: "No matching domain for file \<file\>. Use --domain to register a new domain." Exit code 1. | New domains require explicit `--domain` |
| `add-queue-item` with `--domain` that does not match domain derived from `--file` | Error: "Domain in --domain (\<domain\>) does not match the domain in file \<file\> (\<derived\>)." Exit code 1. | Domain name and file path must agree |
| `add-queue-item` with `--domain` introducing a domain whose path nests within or contains an existing domain path | Error: "Domain paths must not be nested: \<path1\> is a prefix of \<path2\>." Exit code 1. | Nested domain paths create ambiguity |
| `set-roots` outside of CROSS_REFERENCE_REVIEW or DONE | Error: "set-roots is only valid in CROSS_REFERENCE_REVIEW or DONE states (current state: \<state\>)." Exit code 1. | Code search roots are collected at domain completion boundaries |
| `set-roots` outside of specifying phase | Error: "set-roots is only valid in the specifying phase (current phase: \<phase\>)." Exit code 1. | Code search roots feed into the planning phase via the generate_planning_queue phase |
| `set-roots` at DONE without `--domain` | Error: "--domain is required at DONE (no current domain)." Exit code 1. | At DONE there is no current domain to infer from |
| `eval` outside of specifying EVALUATE or CROSS_REFERENCE_EVAL | Error naming current state and phase. Exit code 1. | Eval context only available in evaluation states |

---

## Behavior

### Batch Selection

At ORIENT, the scaffold selects up to `specifying.batch` specs from the front of the queue, all from the same domain. Specs are pulled in queue order. Domain boundaries are batch boundaries — the batch ends when the domain changes or the batch size is reached.

### Domain Path

Each spec in the queue includes a `domain_path` — the filesystem path to the domain directory (e.g., `optimizer/`). The scaffold uses this to:
- Display shorter spec file paths (relative to `<domain_path>/specs/`)
- Discover all `.md` files in `<domain_path>/specs/` for cross-referencing
- The state file stores spec filenames only (e.g., `repository-loading.md`), not full paths. Full paths are reconstructed as `<domain_path>/specs/<filename>`.

### State Machine

```
ORIENT → SELECT → DRAFT → EVALUATE
                              │
                    ┌─────────┼──────────┐
                    │         │          │
              PASS ≥ min   FAIL < max  PASS < min
                    │         │          │
                    ▼         ▼          ▼
                 ACCEPT    REFINE     REFINE
                    │         │          │
              ┌─────┘         └────┬─────┘
              │                    │
         domain done?          EVALUATE
           yes → CROSS_REFERENCE
           no → ORIENT            FAIL ≥ max → ACCEPT (forced)

CROSS_REFERENCE → CROSS_REFERENCE_EVAL
                        │
              ┌─────────┼──────────────────┐
              │         │                  │
        PASS ≥ min   PASS < min       FAIL < max
              │         │                  │
              ▼         ▼                  ▼
         (continue)  CROSS_REFERENCE    CROSS_REFERENCE
              │
         queue empty?        FAIL ≥ max → (continue, forced)
           yes → DONE
           no → ORIENT

(continue) = CROSS_REFERENCE_REVIEW if round == 1 (always, regardless of user_review),
             otherwise next domain or DONE
```

### Transition Table

| From State | Condition | To State | Side Effects |
|------------|-----------|----------|-------------|
| ORIENT | always | SELECT | Pull next batch from queue into `current_specs`. Batch grouped by domain, up to `specifying.batch`. |
| SELECT | always | DRAFT | — |
| DRAFT | always | EVALUATE | Set round to 1. |
| EVALUATE | `--verdict PASS`, round >= `specifying.eval.min_rounds` | ACCEPT | Record eval. |
| EVALUATE | `--verdict PASS`, round < `specifying.eval.min_rounds` | REFINE | Record eval (PASS + eval report). Min rounds not met. |
| EVALUATE | `--verdict FAIL`, round < `specifying.eval.max_rounds` | REFINE | Record eval (FAIL + eval report). |
| EVALUATE | `--verdict FAIL`, round >= `specifying.eval.max_rounds` | ACCEPT | Record eval (FAIL + eval report). Forced acceptance. |
| REFINE | always | EVALUATE | Increment round. |
| ACCEPT | domain has more batches in queue | ORIENT | Move batch specs to completed (with eval history + commit hashes). |
| ACCEPT | domain complete, queue non-empty | CROSS_REFERENCE | Move batch specs to completed. Begin domain cross-referencing. |
| ACCEPT | domain complete, queue empty | CROSS_REFERENCE | Move batch specs to completed. Begin domain cross-referencing. |
| CROSS_REFERENCE | always | CROSS_REFERENCE_EVAL | Set cross-reference round to 1 (first entry) or increment round. |
| CROSS_REFERENCE_EVAL | `--verdict PASS`, round >= `specifying.cross_reference.min_rounds`, round == 1 | CROSS_REFERENCE_REVIEW | Record eval. Always enters CROSS_REFERENCE_REVIEW on first passing eval (regardless of `user_review`). |
| CROSS_REFERENCE_EVAL | `--verdict PASS`, round >= `specifying.cross_reference.min_rounds`, round > 1 | ORIENT or DONE | Record eval. If queue non-empty: ORIENT. Else: DONE. |
| CROSS_REFERENCE_EVAL | `--verdict PASS`, round < `specifying.cross_reference.min_rounds` | CROSS_REFERENCE | Record eval. Min rounds not met. |
| CROSS_REFERENCE_EVAL | `--verdict FAIL`, round < `specifying.cross_reference.max_rounds` | CROSS_REFERENCE | Record eval. |
| CROSS_REFERENCE_EVAL | `--verdict FAIL`, round >= `specifying.cross_reference.max_rounds` | ORIENT or DONE | Record eval. Forced acceptance. If queue non-empty: ORIENT. Else: DONE. |
| CROSS_REFERENCE_REVIEW | always | ORIENT or DONE | If queue non-empty: ORIENT. Else: DONE. |
| DONE | queue non-empty (via add-queue-item) | ORIENT | Pull next batch from queue. Re-enters drafting loop. |
| DONE | queue empty | RECONCILE | Begin cross-domain reconciliation (see spec-reconciliation). |

### Eval Output Convention

The specifying evaluation sub-agent writes structured markdown to a known directory:

```
<domain>/specs/.eval/
├── batch-1-r1.md
├── batch-1-r2.md
├── cross-reference-r1.md
├── cross-reference-r2.md
└── ...
```

The scaffold does not read or write these files. This is a convention for the architect and sub-agent. The eval sub-agent evaluates all specs in the batch together in a single report.

### Add Queue Item

`forgectl add-queue-item` appends a spec to the end of the specifying queue. It does not affect the current batch.

1. Validate the current phase is `specifying`.
2. Validate the current state is DRAFT, CROSS_REFERENCE_REVIEW, DONE, or RECONCILE_REVIEW.
3. Validate all required flags: `--name`, `--topic`, `--file`.
4. Validate `--file` points to an existing file. If not: error with message instructing the architect to create the file first.
5. Validate `--file` matches `<something>/specs/<name>.md` pattern. If not: error `File path must follow <domain>/specs/<name>.md convention.`
6. Derive domain from `--file`: extract the path prefix before `/specs/` (e.g., `domains/emails/specs/smtp-relay.md` → `domains/emails`).
7. Resolve domain:
   a. Check configured domains (`config.domains`): does any domain's `path` match the derived path?
   b. If no configured domains, check session domains (queue + completed specs): does any existing domain path match?
   c. **If `--domain` is omitted:**
      - If a matching domain is found (configured or session): use it. Print `Added <name> to specifying queue for domain <domain_name>.`
      - If no match: error `No matching domain for file <file>. Use --domain to register a new domain.`
   d. **If `--domain` is provided:**
      - Derive the domain name from the file path. If `--domain` does not match: error `Domain in --domain (<domain>) does not match the domain in file <file> (<derived_domain>).`
      - If the domain exists (configured or session): use it.
      - If the domain is new: validate the new domain path does not nest within or contain any existing domain path. Register as a session-only domain.
8. Validate `--name` is unique across both the queue and completed specs.
9. Append the new entry to the end of `specifying.queue`.
10. Write the state file.
11. Print confirmation: spec name, domain, domain_path, and queue position. E.g., `Added smtp-relay to specifying queue for domain emails.`

When `add-queue-item` is used at DONE and the queue was previously empty, advancing from DONE re-enters ORIENT for the new items. After those items complete (including their domain's cross-reference), the scaffold returns to DONE. Reconciliation does not begin until the queue is empty again.

### Set Roots

`forgectl set-roots` stores code search roots for a domain. These roots are used during plan-queue generation in the generate_planning_queue phase.

1. Validate the current phase is `specifying`.
2. Validate the current state is CROSS_REFERENCE_REVIEW or DONE.
3. Resolve domain: if `--domain` is provided, use it. Otherwise, infer from the current cross-reference domain at CROSS_REFERENCE_REVIEW. At DONE, `--domain` is required (no current domain).
4. Validate the resolved domain has completed specs.
5. Validate at least one positional path argument is provided.
6. Store the roots in `specifying.domains[<domain>].code_search_roots`.
7. Write the state file.
8. Print confirmation: domain and roots.

Calling `set-roots` for a domain that already has roots overwrites the previous value.

---

## Invariants

1. **Batch is domain-homogeneous.** All specs in a batch share the same domain.
2. **Round monotonicity.** The specifying round counter only increments.
3. **Queue order preserved.** Specs are pulled from the front of the queue.
4. **Min rounds enforced.** PASS below `specifying.eval.min_rounds` forces another cycle.
5. **Max rounds enforced.** FAIL at `specifying.eval.max_rounds` forces acceptance.
6. **Guided pauses.** When `config.general.user_guided` is true, SELECT output includes "STOP please review and discuss with user before continuing."
7. **No commits during specifying lifecycle.** Individual eval rounds, refinements, and cross-referencing do not produce commits. The single specifying commit occurs at COMPLETE (see spec-reconciliation).
8. **Domain cross-reference required.** Every domain passes through CROSS_REFERENCE before the next domain begins or DONE is reached.
9. **Cross-reference scans all domain specs.** CROSS_REFERENCE discovers all `.md` files in `<domain_path>/specs/`, not just session specs.
10. **Domain checkpoint fires once.** CROSS_REFERENCE_REVIEW is entered exactly once per domain (after the first passing CROSS_REFERENCE_EVAL), regardless of `user_review` setting.
11. **user_review controls output, not state entry.** When `user_review` is true, the CROSS_REFERENCE_REVIEW action includes "STOP please review and discuss with user before continuing." When false, it says "Domain cross-reference complete." The state is entered either way.
12. **add-queue-item is state-gated.** Only valid in DRAFT, CROSS_REFERENCE_REVIEW, DONE, or RECONCILE_REVIEW within the specifying phase.
13. **set-roots is state-gated.** Only valid in CROSS_REFERENCE_REVIEW or DONE within the specifying phase.
14. **add-queue-item names are unique.** No duplicate names across queue and completed specs.
15. **DONE re-enters ORIENT when queue is non-empty.** If `add-queue-item` populates the queue at DONE, advancing re-enters ORIENT instead of RECONCILE.

---

## Edge Cases

- **Scenario:** `advance --verdict FAIL` when round < `specifying.eval.max_rounds`.
  - **Expected:** REFINE.
  - **Rationale:** More evaluation rounds remain; the architect gets another chance to address deficiencies.

- **Scenario:** `advance --verdict FAIL` when round >= `specifying.eval.max_rounds`.
  - **Expected:** ACCEPT (forced).
  - **Rationale:** The maximum rounds are exhausted. The batch is accepted as-is to prevent indefinite loops.

- **Scenario:** `advance --verdict PASS` when round < `specifying.eval.min_rounds`.
  - **Expected:** REFINE (min rounds not met).
  - **Rationale:** Even with a passing verdict, minimum evaluation rounds must be completed to ensure sufficient review.

- **Scenario:** Domain has fewer specs remaining than `specifying.batch`.
  - **Expected:** Batch contains all remaining specs for that domain.
  - **Rationale:** Batches are capped at `specifying.batch` but may be smaller. No padding occurs.

- **Scenario:** `enable_commits` is `false` and architect provides `--message` at any specifying state.
  - **Expected:** Warning printed: `--message is ignored, commits are not enabled`. Command proceeds. The warning does not instruct how to enable commits.
  - **Rationale:** Users who do not need auto-commits should not be confused or prompted to change configuration.

- **Scenario:** Domain has no existing specs outside the queue (all specs are new).
  - **Expected:** CROSS_REFERENCE lists only session specs under `[session — completed]`. `[existing — not in queue]` section is empty or omitted.
  - **Rationale:** Cross-referencing still runs to verify consistency among the new specs.

- **Scenario:** CROSS_REFERENCE_EVAL FAIL at `specifying.cross_reference.max_rounds`.
  - **Expected:** Forced acceptance. Advance to next domain (ORIENT) or DONE.
  - **Rationale:** Maximum cross-reference rounds exhausted. Prevents indefinite loops.

- **Scenario:** First CROSS_REFERENCE_EVAL passes, `user_review` is true.
  - **Expected:** CROSS_REFERENCE_REVIEW with "STOP please review and discuss with user before continuing." in action output.
  - **Rationale:** Domain checkpoint always fires. `user_review` adds the review prompt.

- **Scenario:** First CROSS_REFERENCE_EVAL passes, `user_review` is false.
  - **Expected:** CROSS_REFERENCE_REVIEW with "Domain cross-reference complete." in action output. No user review prompt.
  - **Rationale:** Domain checkpoint always fires for set-roots and add-queue-item collection. `user_review` only controls the review prompt.

- **Scenario:** `add-queue-item` called during EVALUATE.
  - **Expected:** Error: "add-queue-item is only valid in DRAFT, CROSS_REFERENCE_REVIEW, DONE, or RECONCILE_REVIEW states (current state: EVALUATE)." Exit code 1.
  - **Rationale:** Queue modifications are restricted to states where the architect is the actor with sufficient context.

- **Scenario:** `add-queue-item` with a name that already exists in the queue.
  - **Expected:** Error: "spec name '<name>' already exists in queue." Exit code 1.
  - **Rationale:** Duplicate names create ambiguity in status output, depends_on references, and completed specs tracking.

- **Scenario:** `add-queue-item` called at DONE when queue is empty, then advance.
  - **Expected:** Advance re-enters ORIENT. The new spec is pulled into a batch. After acceptance and cross-reference, returns to DONE.
  - **Rationale:** DONE only advances to RECONCILE when the queue is empty. Adding items re-opens the drafting loop.

- **Scenario:** `add-queue-item` called during DRAFT for a different domain than the current batch.
  - **Expected:** Item appended to queue. Current batch continues unaffected. The new item is processed when its domain comes up in queue order.
  - **Rationale:** Queue append does not disturb the current batch.

- **Scenario:** `add-queue-item --file domains/emails/specs/smtp-relay.md` without `--domain`. Domain `emails` is configured in `[[domains]]`.
  - **Expected:** Domain resolved to `emails`. Print: `Added smtp-relay to specifying queue for domain emails.`
  - **Rationale:** Configured domains are matched by path prefix.

- **Scenario:** `add-queue-item --file new-service/specs/auth.md` without `--domain`. No configured or session domain matches.
  - **Expected:** Error: `No matching domain for file new-service/specs/auth.md. Use --domain to register a new domain.`
  - **Rationale:** New domains require explicit intent via `--domain`.

- **Scenario:** `add-queue-item --domain portal --file optimizer/specs/foo.md`.
  - **Expected:** Error: `Domain in --domain (portal) does not match the domain in file optimizer/specs/foo.md (optimizer).`
  - **Rationale:** Domain name and file path must agree to prevent misclassification.

- **Scenario:** `add-queue-item --domain users-admin --file domains/users/admin/specs/roles.md`. Existing domain `users` has path `domains/users`.
  - **Expected:** Error: `Domain paths must not be nested: domains/users is a prefix of domains/users/admin.`
  - **Rationale:** Nested domains create ambiguity in file-to-domain resolution.

- **Scenario:** `add-queue-item --file shared/lib/something.md`.
  - **Expected:** Error: `File path must follow <domain>/specs/<name>.md convention.`
  - **Rationale:** Spec files must reside in a domain's `specs/` directory.

- **Scenario:** `add-queue-item` with `--file` pointing to a file that does not exist.
  - **Expected:** Error: "file <path> does not exist. add-queue-item registers specs that have already been written. Create the spec file first, then register it." Exit code 1.
  - **Rationale:** `add-queue-item` is for registering existing work, not queuing future work. The architect must write the spec before registering it.

- **Scenario:** `add-queue-item` with `--file` whose derived `domain_path` conflicts with an existing domain's path.
  - **Expected:** Error naming the conflict. Exit code 1.
  - **Rationale:** Prevents silent domain_path mismatches that would break cross-referencing and file discovery.

- **Scenario:** `set-roots` called for a domain with no completed specs.
  - **Expected:** Error: "domain '<domain>' has no completed specs." Exit code 1.
  - **Rationale:** Roots are collected for domains that have been specified. Setting roots for an unknown domain is likely a typo.

- **Scenario:** `set-roots` called twice for the same domain.
  - **Expected:** Second call overwrites the first. No error.
  - **Rationale:** The architect may refine their understanding of code locations.

- **Scenario:** Phase shift to planning with no `set-roots` called for a domain.
  - **Expected:** `code_search_roots` defaults to `["<domain>/"]` in the generated plan-queue entry.
  - **Rationale:** The domain directory itself is the most common search root. Explicit roots override the default.

---

## Testing Criteria

### Study and draft advance sequentially
- **Verifies:** Sequential state progression through specifying states.
- **Given:** ORIENT.
- **When:** advance through SELECT → DRAFT → EVALUATE.
- **Then:** Each transitions in order.

### ORIENT selects batch by domain
- **Verifies:** Batch selection groups by domain up to specifying.batch.
- **Given:** Queue has 5 optimizer specs, `specifying.batch = 3`.
- **When:** advance from ORIENT.
- **Then:** `current_specs` has 3 optimizer specs. 2 remain in queue.

### Domain boundary ends batch
- **Verifies:** Batch never mixes domains.
- **Given:** Queue has 2 optimizer specs then 3 portal specs, `specifying.batch = 3`.
- **When:** advance from ORIENT.
- **Then:** `current_specs` has 2 optimizer specs (not 3).

### FAIL below max_rounds goes to REFINE
- **Verifies:** FAIL verdict with remaining rounds triggers refinement.
- **Given:** EVALUATE, `specifying.eval.max_rounds: 3`, round 1.
- **When:** `advance --verdict FAIL --eval-report .eval/batch-1-r1.md`
- **Then:** State is REFINE.

### FAIL at max_rounds forces ACCEPT
- **Verifies:** FAIL verdict at max rounds forces acceptance.
- **Given:** EVALUATE, `specifying.eval.max_rounds: 2`, round 2.
- **When:** `advance --verdict FAIL --eval-report .eval/batch-1-r2.md`
- **Then:** State is ACCEPT (forced).

### PASS below min_rounds goes to REFINE
- **Verifies:** PASS verdict below min rounds requires more evaluation.
- **Given:** EVALUATE, `specifying.eval.min_rounds: 2`, round 1.
- **When:** `advance --verdict PASS --eval-report .eval/batch-1-r1.md`
- **Then:** State is REFINE.

### PASS at min_rounds goes to ACCEPT
- **Verifies:** PASS verdict at min rounds triggers acceptance.
- **Given:** EVALUATE, `specifying.eval.min_rounds: 1`, round 1.
- **When:** `advance --verdict PASS --eval-report .eval/batch-1-r1.md`
- **Then:** State is ACCEPT.

### --message ignored when enable_commits is false
- **Verifies:** Warning printed when --message provided with commits disabled.
- **Given:** EVALUATE, `enable_commits: false`.
- **When:** `advance --verdict PASS --eval-report .eval/batch-1-r1.md --message "some message"`
- **Then:** State is ACCEPT. Warning printed: `--message is ignored, commits are not enabled`. No error. Warning does not instruct how to enable commits.

### ACCEPT triggers CROSS_REFERENCE when domain complete
- **Verifies:** Domain completion enters cross-reference before next domain.
- **Given:** ACCEPT, last batch for optimizer domain. Queue has portal specs remaining.
- **When:** `advance`
- **Then:** State is CROSS_REFERENCE. Domain is optimizer.

### ACCEPT triggers ORIENT when domain has more batches
- **Verifies:** Intra-domain batching continues without cross-reference.
- **Given:** ACCEPT, optimizer domain has 2 more specs in queue.
- **When:** `advance`
- **Then:** State is ORIENT.

### CROSS_REFERENCE lists all domain specs
- **Verifies:** Cross-reference discovers session and existing specs.
- **Given:** CROSS_REFERENCE for optimizer. Session completed 3 specs. `optimizer/specs/` also contains 2 existing specs not in queue.
- **When:** `status`
- **Then:** Output shows 3 session specs and 2 existing specs.

### CROSS_REFERENCE_EVAL PASS advances to next domain
- **Verifies:** Successful cross-reference continues to next domain.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.min_rounds: 1`, round 1. Queue has portal specs.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r1.md`
- **Then:** State is ORIENT. Next batch is from portal domain.

### CROSS_REFERENCE_EVAL FAIL below max retries
- **Verifies:** Failed cross-reference loops back.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.max_rounds: 2`, round 1.
- **When:** `advance --verdict FAIL --eval-report .eval/cross-reference-r1.md`
- **Then:** State is CROSS_REFERENCE.

### CROSS_REFERENCE_EVAL FAIL at max forces acceptance
- **Verifies:** Max rounds forces cross-reference acceptance.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.max_rounds: 2`, round 2.
- **When:** `advance --verdict FAIL --eval-report .eval/cross-reference-r2.md`
- **Then:** State is ORIENT (or DONE if queue empty). Forced acceptance.

### CROSS_REFERENCE_REVIEW fires on first pass with user_review true
- **Verifies:** Domain checkpoint with user review prompt.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.user_review: true`, round 1, verdict PASS.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r1.md`
- **Then:** State is CROSS_REFERENCE_REVIEW. Action includes "STOP please review and discuss with user before continuing."

### CROSS_REFERENCE_REVIEW fires on first pass with user_review false
- **Verifies:** Domain checkpoint fires regardless of user_review.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.user_review: false`, round 1, verdict PASS.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r1.md`
- **Then:** State is CROSS_REFERENCE_REVIEW. Action includes "Domain cross-reference complete." No user review prompt.

### CROSS_REFERENCE_REVIEW does not fire on subsequent rounds
- **Verifies:** Domain checkpoint only fires once per domain.
- **Given:** CROSS_REFERENCE_EVAL, round 2, verdict PASS.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r2.md`
- **Then:** State is ORIENT or DONE (not CROSS_REFERENCE_REVIEW).

### ACCEPT triggers CROSS_REFERENCE when domain complete
- **Verifies:** Domain completion enters cross-reference before next domain.
- **Given:** ACCEPT, last batch for optimizer domain. Queue has portal specs remaining.
- **When:** `advance`
- **Then:** State is CROSS_REFERENCE. Domain is optimizer.

### ACCEPT triggers ORIENT when domain has more batches
- **Verifies:** Intra-domain batching continues without cross-reference.
- **Given:** ACCEPT, optimizer domain has 2 more specs in queue.
- **When:** `advance`
- **Then:** State is ORIENT.

### CROSS_REFERENCE lists all domain specs
- **Verifies:** Cross-reference discovers session and existing specs.
- **Given:** CROSS_REFERENCE for optimizer. Session completed 3 specs. `optimizer/specs/` also contains 2 existing specs not in queue.
- **When:** `status`
- **Then:** Output shows 3 session specs and 2 existing specs.

### CROSS_REFERENCE_EVAL PASS advances to next domain
- **Verifies:** Successful cross-reference continues to next domain.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.min_rounds: 1`, round 1. Queue has portal specs.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r1.md`
- **Then:** State is ORIENT. Next batch is from portal domain.

### CROSS_REFERENCE_EVAL FAIL below max retries
- **Verifies:** Failed cross-reference loops back.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.max_rounds: 2`, round 1.
- **When:** `advance --verdict FAIL --eval-report .eval/cross-reference-r1.md`
- **Then:** State is CROSS_REFERENCE.

### CROSS_REFERENCE_EVAL FAIL at max forces acceptance
- **Verifies:** Max rounds forces cross-reference acceptance.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.max_rounds: 2`, round 2.
- **When:** `advance --verdict FAIL --eval-report .eval/cross-reference-r2.md`
- **Then:** State is ORIENT (or DONE if queue empty). Forced acceptance.

### CROSS_REFERENCE_REVIEW fires on first pass with user_review true
- **Verifies:** Domain checkpoint with user review prompt.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.user_review: true`, round 1, verdict PASS.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r1.md`
- **Then:** State is CROSS_REFERENCE_REVIEW. Action includes "STOP please review and discuss with user before continuing."

### CROSS_REFERENCE_REVIEW fires on first pass with user_review false
- **Verifies:** Domain checkpoint fires regardless of user_review.
- **Given:** CROSS_REFERENCE_EVAL, `specifying.cross_reference.user_review: false`, round 1, verdict PASS.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r1.md`
- **Then:** State is CROSS_REFERENCE_REVIEW. Action includes "Domain cross-reference complete." No user review prompt.

### CROSS_REFERENCE_REVIEW does not fire on subsequent rounds
- **Verifies:** Domain checkpoint only fires once per domain.
- **Given:** CROSS_REFERENCE_EVAL, round 2, verdict PASS.
- **When:** `advance --verdict PASS --eval-report .eval/cross-reference-r2.md`
- **Then:** State is ORIENT or DONE (not CROSS_REFERENCE_REVIEW).

### DONE transitions to reconciliation
- **Verifies:** Queue exhaustion triggers reconciliation phase.
- **Given:** All specs accepted, all domains cross-referenced, state is DONE. Queue is empty.
- **When:** `advance`
- **Then:** State is RECONCILE (see spec-reconciliation).

### add-queue-item at DRAFT appends to queue
- **Verifies:** Queue append during drafting.
- **Given:** DRAFT. Queue has 2 items.
- **When:** `add-queue-item --name "New Spec" --domain optimizer --topic "..." --file optimizer/specs/new-spec.md`
- **Then:** Queue has 3 items. Current batch unchanged. State remains DRAFT.

### add-queue-item rejects non-existent file
- **Verifies:** File existence validation.
- **Given:** DRAFT. File `optimizer/specs/does-not-exist.md` does not exist.
- **When:** `add-queue-item --name "Ghost Spec" --topic "..." --file optimizer/specs/does-not-exist.md`
- **Then:** Exit code 1. Error instructs architect to create the file first.

### add-queue-item resolves domain from configured domains
- **Verifies:** Domain resolved from `[[domains]]` config by file path.
- **Given:** DRAFT. `[[domains]]` has `name = "emails", path = "domains/emails"`. File `domains/emails/specs/smtp-relay.md` exists.
- **When:** `add-queue-item --name "SMTP Relay" --topic "..." --file domains/emails/specs/smtp-relay.md`
- **Then:** Queue entry has domain `emails`. Print: `Added SMTP Relay to specifying queue for domain emails.`

### add-queue-item resolves domain from session domains
- **Verifies:** Domain resolved from existing session domains when no config.
- **Given:** DRAFT. No `[[domains]]` config. Domain `optimizer` exists in queue with path `optimizer/`. File `optimizer/specs/cache-eviction.md` exists.
- **When:** `add-queue-item --name "Cache Eviction" --topic "..." --file optimizer/specs/cache-eviction.md`
- **Then:** Queue entry has domain `optimizer`.

### add-queue-item rejects unknown domain without --domain
- **Verifies:** New domains require explicit --domain.
- **Given:** DONE. File `new-service/specs/auth.md` exists. No matching domain.
- **When:** `add-queue-item --name "Auth" --topic "..." --file new-service/specs/auth.md`
- **Then:** Exit code 1. Error: `No matching domain for file new-service/specs/auth.md. Use --domain to register a new domain.`

### add-queue-item registers new domain with --domain
- **Verifies:** New session-only domain created.
- **Given:** DONE. File `new-service/specs/auth.md` exists. No matching domain.
- **When:** `add-queue-item --name "Auth" --domain new-service --topic "..." --file new-service/specs/auth.md`
- **Then:** Queue entry has domain `new-service`. Domain registered in session. Config file not modified.

### add-queue-item rejects --domain mismatch
- **Verifies:** Domain name must match file path.
- **Given:** DRAFT. File `optimizer/specs/foo.md` exists.
- **When:** `add-queue-item --name "Foo" --domain portal --topic "..." --file optimizer/specs/foo.md`
- **Then:** Exit code 1. Error names both domains.

### add-queue-item rejects nested domain paths
- **Verifies:** Nesting prevention.
- **Given:** DONE. Domain `users` exists with path `domains/users`. File `domains/users/admin/specs/roles.md` exists.
- **When:** `add-queue-item --name "Roles" --domain users-admin --topic "..." --file domains/users/admin/specs/roles.md`
- **Then:** Exit code 1. Error: nesting.

### add-queue-item rejects non-convention file path
- **Verifies:** File must follow `<domain>/specs/<name>.md` convention.
- **Given:** DRAFT. File `shared/lib/something.md` exists.
- **When:** `add-queue-item --name "Something" --topic "..." --file shared/lib/something.md`
- **Then:** Exit code 1. Error: file path convention.

### add-queue-item rejected outside valid states
- **Verifies:** State gate enforcement.
- **Given:** EVALUATE.
- **When:** `add-queue-item --name "New Spec" --domain optimizer --topic "..." --file optimizer/specs/new-spec.md`
- **Then:** Exit code 1. Error names current state.

### add-queue-item rejected outside specifying phase
- **Verifies:** Phase gate enforcement.
- **Given:** Planning phase, any state.
- **When:** `add-queue-item --name "New Spec" --domain optimizer --topic "..." --file optimizer/specs/new-spec.md`
- **Then:** Exit code 1. Error names current phase.

### add-queue-item rejects duplicate names
- **Verifies:** Name uniqueness across queue and completed.
- **Given:** DRAFT. Completed spec named "Repository Loading".
- **When:** `add-queue-item --name "Repository Loading" --domain optimizer --topic "..." --file optimizer/specs/repo-loading-v2.md`
- **Then:** Exit code 1. Error names the duplicate.

### add-queue-item at DONE re-enters ORIENT
- **Verifies:** Adding items at DONE reopens the drafting loop.
- **Given:** DONE, queue empty.
- **When:** `add-queue-item --name "New Spec" --domain portal --topic "..." --file portal/specs/new-spec.md`, then `advance`.
- **Then:** State is ORIENT. New spec is pulled into batch.

### DONE with non-empty queue re-enters ORIENT instead of RECONCILE
- **Verifies:** DONE only advances to RECONCILE when queue is empty.
- **Given:** DONE, queue has 1 item (added via add-queue-item).
- **When:** `advance`
- **Then:** State is ORIENT (not RECONCILE).

### set-roots stores roots per domain
- **Verifies:** Root storage in state file.
- **Given:** CROSS_REFERENCE_REVIEW, domain is optimizer.
- **When:** `set-roots --domain optimizer optimizer/ lib/shared/`
- **Then:** `specifying.domains["optimizer"].code_search_roots` is `["optimizer/", "lib/shared/"]`.

### set-roots rejected outside valid states
- **Verifies:** State gate enforcement.
- **Given:** DRAFT.
- **When:** `set-roots --domain optimizer optimizer/`
- **Then:** Exit code 1. Error names current state.

### set-roots overwrites previous value
- **Verifies:** Idempotent overwrite.
- **Given:** CROSS_REFERENCE_REVIEW. Domain optimizer already has roots `["optimizer/"]`.
- **When:** `set-roots --domain optimizer optimizer/ lib/shared/`
- **Then:** Roots updated to `["optimizer/", "lib/shared/"]`.

### set-roots rejects unknown domain
- **Verifies:** Domain must have completed specs.
- **Given:** DONE. No completed specs for domain "unknown".
- **When:** `set-roots --domain unknown unknown/`
- **Then:** Exit code 1. Error names the domain.

---

## Implements
- Specifying phase: queue-driven batched spec drafting with eval/refine loop
- Domain-grouped batch selection up to `specifying.batch`
- Domain path for shorter spec file paths and domain-scoped discovery
- Eval sub-agent evaluates full batch together
- Eval round enforcement (`specifying.eval.min_rounds`/`max_rounds`) with forced acceptance
- Domain-scoped CROSS_REFERENCE after each domain completes
- Cross-reference discovers all `.md` files in `<domain_path>/specs/`
- Cross-reference eval round enforcement (`specifying.cross_reference.min_rounds`/`max_rounds`)
- Configurable CROSS_REFERENCE_REVIEW pause (`specifying.cross_reference.user_review`)
- No commits during specifying lifecycle (single commit at COMPLETE, see spec-reconciliation)
- `add-queue-item`: state-gated queue append with domain resolution from configured domains, session domains, or explicit `--domain` (DRAFT, CROSS_REFERENCE_REVIEW, DONE)
- `set-roots`: state-gated code search root collection per domain (CROSS_REFERENCE_REVIEW, DONE)
- DONE re-enters ORIENT when queue is non-empty (supports late-added specs before reconciliation)
