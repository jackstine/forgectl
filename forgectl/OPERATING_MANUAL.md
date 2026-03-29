# Operating Manual

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize a session from a validated input file |
| `advance` | Transition from current state to the next |
| `status` | Print current state with action guidance and full session overview |
| `eval` | Output full evaluation context for the sub-agent (EVALUATE states only) |
| `add-commit` | Register a commit hash to a completed spec by ID |
| `reconcile-commit` | Auto-register a commit to all specs whose files were touched |

## `init` Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--from <path>` | yes | — | Path to input file (schema varies by `--phase`) |
| `--phase` | no | specifying | Starting phase: `specifying`, `planning`, `implementing` |

All batch sizes, round limits, and guided settings are configured in `.forgectl/config` (TOML file).

## `advance` Flags

| Flag | Description |
|------|-------------|
| `--verdict PASS\|FAIL` | Evaluation verdict |
| `--eval-report <path>` | Path to evaluation report file |
| `--message <text>` | Commit message or acceptance message (required when `enable_commits` is true in config) |
| `--file <path>` | Override spec file path (specifying DRAFT only) |
| `--from <path>` | Plan queue input file (specifying→planning phase shift only) |
| `--guided` / `--no-guided` | Update guided setting (accepted on any advance) |

## `add-commit` Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--id N` | yes | Completed spec ID |
| `--hash <hash>` | yes | Git commit hash (validated against repo) |

## `reconcile-commit` Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--hash <hash>` | yes | Git commit hash (matched against spec file paths) |

---

## State Machine Reference

### Specifying Phase

| State | Advance Flags | Next State | Notes |
|-------|---------------|------------|-------|
| ORIENT | — | SELECT | Pulls next spec from queue |
| SELECT | — | DRAFT | Review topic and sources. Guided pause. |
| DRAFT | `--file` (optional) | EVALUATE | Draft the spec. Round set to 1. |
| EVALUATE | `--verdict PASS`, `--eval-report`, `--message` | ACCEPT | When round >= `specifying.eval.min_rounds`. Auto-commits. |
| EVALUATE | `--verdict PASS`, `--eval-report` | REFINE | When round < `specifying.eval.min_rounds` |
| EVALUATE | `--verdict FAIL`, `--eval-report` | REFINE | When round < `specifying.eval.max_rounds` |
| EVALUATE | `--verdict FAIL`, `--eval-report` | ACCEPT | When round >= `specifying.eval.max_rounds` (forced) |
| REFINE | — | EVALUATE | Increments round |
| ACCEPT | — | ORIENT | When queue non-empty. Moves spec to completed. |
| ACCEPT | — | DONE | When queue empty |
| DONE | — | RECONCILE | Begins cross-reference reconciliation |
| RECONCILE | — | RECONCILE_EVAL | Increments reconcile round |
| RECONCILE_EVAL | `--verdict PASS`, `--message` | COMPLETE | Reconciliation accepted |
| RECONCILE_EVAL | `--verdict FAIL` | RECONCILE_REVIEW | Issues found |
| RECONCILE_REVIEW | — or `--verdict PASS` | COMPLETE | Accept as-is |
| RECONCILE_REVIEW | `--verdict FAIL` | RECONCILE | Fix and re-evaluate |
| COMPLETE | — | PHASE_SHIFT | Specifying phase complete |

### Phase Shifts

| From | To | Advance Flags | Notes |
|------|----|---------------|-------|
| PHASE_SHIFT (specifying→planning) | ORIENT (planning) | `--from <plans-queue.json>` | Required. Validates plan queue. |
| PHASE_SHIFT (planning→implementing) | ORIENT (implementing) | — | Validates plan.json. Adds `passes`/`rounds` to items. |

### Planning Phase

| State | Advance Flags | Next State | Notes |
|-------|---------------|------------|-------|
| ORIENT | — | STUDY_SPECS | Begin studying specs |
| STUDY_SPECS | — | STUDY_CODE | Study spec files and git diffs |
| STUDY_CODE | — | STUDY_PACKAGES | Explore codebase with sub-agents |
| STUDY_PACKAGES | — | REVIEW | Study technical stack |
| REVIEW | — | DRAFT | Review findings. Guided pause. |
| DRAFT | — | EVALUATE | If plan.json valid. Round set to 1. |
| DRAFT | — | VALIDATE | If plan.json invalid. Round set to 1. |
| VALIDATE | — | EVALUATE | If plan.json now valid |
| VALIDATE | — | VALIDATE | If still invalid (exit code 1) |
| EVALUATE | `--verdict PASS`, `--eval-report` | ACCEPT | When round >= `planning.eval.min_rounds` |
| EVALUATE | `--verdict PASS`, `--eval-report` | REFINE | When round < `planning.eval.min_rounds` |
| EVALUATE | `--verdict FAIL`, `--eval-report` | REFINE | When round < `planning.eval.max_rounds` |
| EVALUATE | `--verdict FAIL`, `--eval-report` | ACCEPT | When round >= `planning.eval.max_rounds` (forced) |
| REFINE | — | EVALUATE | If plan.json valid. Increments round. |
| REFINE | — | VALIDATE | If plan.json invalid. Increments round. |
| ACCEPT | `--message` | PHASE_SHIFT | Plan accepted |

### Implementing Phase

| State | Advance Flags | Next State | Notes |
|-------|---------------|------------|-------|
| ORIENT | — | IMPLEMENT | Selects batch of unblocked items. Guided pause. |
| ORIENT | — | ORIENT | All layer items terminal → advance to next layer |
| ORIENT | — | DONE | All layers complete |
| IMPLEMENT (first round) | `--message` | IMPLEMENT | More items in batch. Marks item `done`. Commits. |
| IMPLEMENT (first round) | `--message` | EVALUATE | Last item in batch. Increments rounds. |
| IMPLEMENT (round 2+) | — | IMPLEMENT | More items. No commit needed. |
| IMPLEMENT (round 2+) | — | EVALUATE | Last item. Increments rounds. |
| EVALUATE | `--verdict PASS`, `--eval-report` | COMMIT | When rounds >= `implementing.eval.min_rounds`. Items `passed`. |
| EVALUATE | `--verdict PASS`, `--eval-report` | IMPLEMENT | When rounds < `implementing.eval.min_rounds`. Re-present items. |
| EVALUATE | `--verdict FAIL`, `--eval-report` | IMPLEMENT | When rounds < `implementing.eval.max_rounds`. Re-present items. |
| EVALUATE | `--verdict FAIL`, `--eval-report` | COMMIT | When rounds >= `implementing.eval.max_rounds`. Items `failed`. |
| COMMIT | `--message` | ORIENT | More items or layers remain |
| COMMIT | `--message` | DONE | All layers complete |
| DONE | — | Error | Terminal state |
