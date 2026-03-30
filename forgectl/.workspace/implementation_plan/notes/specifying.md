# Specifying Phase Notes

## Batch Processing

Specs are processed in batches of up to `config.specifying.batch` (default 3), grouped by domain. A batch never mixes domains. If a domain has more specs than batch size, it produces multiple batches sequentially.

### ORIENT changes

ORIENT now selects a batch (not a single spec):
1. Find next domain with queued specs
2. Take up to `specifying.batch` specs from that domain
3. Set `current_specs` to those specs
4. Transition to SELECT

### SELECT output

Shows all specs in the batch with their topics, sources, files. One-domain focus.

```
State:   SELECT
Domain:  optimizer
Path:    optimizer/
Batch:   3 specs
Specs:
  [1] Repository Loading
      File:    repository-loading.md
      Topic:   ...
      Sources: ...
  ...
Action:  Study each planning source. Study each spec doc that exists.
         STOP please review and discuss with user before continuing.
         After completion of the above, advance to begin drafting.
```

### DRAFT changes

- No `--file` flag (removed)
- Shows batch of specs, instructs to draft all
- `add-queue-item` available here for discovered missing specs

### EVALUATE changes

- Evaluates the whole batch together
- Eval file convention: `<domain>/specs/.eval/batch-<N>-r<M>.md`
- `--message` only required when `enable_commits: true`

### ACCEPT changes

- Accepts all specs in the batch
- Moves all `current_specs` to `completed`
- After ACCEPT: check if domain has more queued specs
  - If yes: ORIENT (next batch for same domain)
  - If no (domain exhausted): CROSS_REFERENCE

## CROSS_REFERENCE States

After the last batch for a domain is accepted, enter CROSS_REFERENCE.

### State machine

```
ACCEPT (domain done) → CROSS_REFERENCE → CROSS_REFERENCE_EVAL
                                               |
                              ┌────────────────┼────────────────┐
                              │                │                 │
                        PASS ≥ min        PASS < min       FAIL < max
                              │                │                 │
                              ▼                ▼                 ▼
                     round==1? CROSS_REF_REVIEW  CROSS_REFERENCE  CROSS_REFERENCE
                       yes→ CROSS_REFERENCE_REVIEW
                       no → next domain or DONE

              FAIL ≥ max → same as PASS ≥ min (forced)
```

### CROSS_REFERENCE output

```
State:   CROSS_REFERENCE
Domain:  optimizer
Path:    optimizer/

Specs in domain:
  [session — completed]
    repository-loading.md (batch 1)
    ...
  [existing — not in queue]
    configuration-models.md
    ...

Action:  Please spawn 3 haiku sub-agents to cross-reference ALL specs in this domain.
         ...
         After completion of the above, advance to begin evaluation.
```

Finding existing specs: walk `<domain_path>/specs/` for `*.md` files not in `current_specs` or `completed`.

### CROSS_REFERENCE_EVAL

Requires `--verdict` and `--eval-report`. Uses `config.specifying.cross_reference.min_rounds` and `max_rounds`.

```
State:   CROSS_REFERENCE_EVAL
Round:   1/2
Action:  Please spawn 1 opus sub-agent to evaluate cross-reference consistency.
         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>
```

### CROSS_REFERENCE_REVIEW

Always fires once after the first passing (or forced) CROSS_REFERENCE_EVAL.

When `user_review: true`:
```
Action:  STOP please review and discuss with user before continuing.
         ...
```

When `user_review: false`:
```
Action:  Domain cross-reference complete.
         ...
```

Both cases: `add-queue-item` and `set-roots` available here.

### After CROSS_REFERENCE_REVIEW

Advance from CROSS_REFERENCE_REVIEW:
- If queue non-empty (ANY domain, not just current domain): ORIENT
- If queue empty (all domains done): DONE

## add-queue-item Command

Valid states: DRAFT, CROSS_REFERENCE_REVIEW, DONE (specifying phase), RECONCILE_REVIEW.

```
forgectl add-queue-item \
  --name "New Spec" \
  --domain optimizer \    # required at DONE; inferred at other states
  --topic "..." \
  --file optimizer/specs/new-spec.md \
  --source path/to/source.md  # repeatable
```

Implementation: append a new `SpecQueueEntry` to `s.Specifying.Queue`.

State effect at DONE: after adding, advancing from DONE re-enters ORIENT to process new specs (reconciliation restarts after new specs complete).

## set-roots Command

Valid states: CROSS_REFERENCE_REVIEW, DONE (specifying phase).

```
forgectl set-roots optimizer/ lib/shared/
forgectl set-roots --domain optimizer optimizer/ lib/shared/  # at DONE
```

Implementation: store `[]string` paths in `s.Specifying.Domains[domain].CodeSearchRoots`.

## SpecifyingState structure

```go
type SpecifyingState struct {
    CurrentSpecs   []*ActiveSpec                   `json:"current_specs"`
    Queue          []SpecQueueEntry                 `json:"queue"`
    Completed      []CompletedSpec                  `json:"completed"`
    Domains        map[string]DomainMeta            `json:"domains,omitempty"`
    CrossReference map[string]*CrossReferenceState  `json:"cross_reference,omitempty"`
    Reconcile      *ReconcileState                  `json:"reconcile,omitempty"`
}
```

## Reconciliation State Machine

Reconciliation runs after all domains have completed CROSS_REFERENCE (i.e., after DONE in spec-lifecycle). It validates cross-domain consistency and is governed by `specifying.reconciliation.*` config (independent of per-spec and cross-reference configs).

### State machine

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
           yes → RECONCILE_REVIEW
           no → COMPLETE

RECONCILE_REVIEW:
  queue empty     → COMPLETE
  queue non-empty → DONE   (re-enter for new specs; reconciliation restarts)
```

### Key changes from previous implementation

The old RECONCILE_EVAL sent PASS→COMPLETE, FAIL→RECONCILE_REVIEW (with --verdict option). The new behavior:

- `--eval-report` is now required on RECONCILE_EVAL (same as CROSS_REFERENCE_EVAL)
- Transitions are round-based using `s.Config.Specifying.Reconciliation.MinRounds/MaxRounds`
- RECONCILE_REVIEW always fires once (on round==1), regardless of PASS/FAIL
- RECONCILE_REVIEW takes **no flags** — transition depends on queue state only
- RECONCILE_REVIEW with queue non-empty re-enters DONE (new specs must be drafted, cross-referenced, then reconciliation restarts)

### RECONCILE output

```
State:   RECONCILE
Phase:   specifying
Specs:   N completed across M domains
Action:  Cross-validate all specs across domains: verify Depends On entries,
         Integration Points symmetry, naming consistency. Stage changes with git add.
         After completion of the above, advance to begin evaluation.
```

Count M from unique domains in `s.Specifying.Completed`.

### RECONCILE_EVAL output

```
State:   RECONCILE_EVAL
Phase:   specifying
Round:   N/max_rounds
Action:  Please spawn 1 opus sub-agent to evaluate cross-domain reconciliation.
         The sub-agent runs git diff --staged and evaluates consistency across all specs.
         After completion of the above, advance with --verdict PASS|FAIL --eval-report <path>
```

Round format is `reconcileState.Round / s.Config.Specifying.Reconciliation.MaxRounds`.

### RECONCILE_REVIEW output

Always shows Specs count, Round, Verdict. Action varies by `user_review`:

```
State:   RECONCILE_REVIEW
Phase:   specifying
Specs:   N completed across M domains
Round:   N/max_rounds
Verdict: PASS|FAIL

Action:  [STOP... OR Reconciliation review complete.]
         If additional specs are needed,
         write the new spec file, then register it:
           forgectl add-queue-item --name <name> --domain <domain> --topic <topic> --file <file> [--source <path>...]
           Adding specs here re-enters DONE for the new items before reconciliation restarts.
         After completion of the above, advance to continue.
```

### ReconcileState type (already in types.go)

```go
type ReconcileState struct {
    Round int          `json:"round"`
    Evals []EvalRecord `json:"evals,omitempty"`
}
```

Round tracks how many RECONCILE_EVAL rounds have completed. Initialized to 0 at DONE→RECONCILE. Incremented at RECONCILE→RECONCILE_EVAL.

## enable_commits gating

When `config.general.enable_commits` is false (default):
- `--message` is NOT required at EVALUATE (specifying), ACCEPT (planning), IMPLEMENT, or COMMIT
- Auto-commit logic: TODO, not yet implemented per spec

When `config.general.enable_commits` is true:
- `--message` IS required at those states
- Validation: error if missing

Affects: `advanceSpecifying` EVALUATE PASS check, `advancePlanning` ACCEPT check, `advanceImplementing` IMPLEMENT and COMMIT checks.
