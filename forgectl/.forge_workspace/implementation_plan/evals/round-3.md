# Evaluation Report — Round 3

## Verdict: FAIL

## Note on prior rounds
`forgectl eval` reports Round 1 and Round 2 as FAIL, but the only file present under
`forgectl/.forge_workspace/implementation_plan/evals/` besides this report is
`round-1.md`, whose content evaluates an unrelated plan (`ui-batch-implementation` /
`session-init` / `phase-transitions`, verdict PASS) — not
`workflow-generation`/`adversarial-evaluation-gauntlet`. No `round-2.md` exists. Those
files appear to be stale leftovers from a different planning cycle in this workspace
and contain no usable prior-deficiency context for the current plan. This evaluation
was therefore performed from scratch against `plan.json`, `docs/schemas/plan-json.md`
(the plan-format reference; `forgectl/PLAN_FORMAT.md` referenced by the evaluator
instructions and `forgectl/evaluators/plan-eval.md` does not exist anywhere in the
repository — it was deleted in an earlier commit and never replaced, so
`docs/schemas/plan-json.md` was used as the closest current authority, backed by
`forgectl/state/validate.go`), and both target specs.

## Summary
- Dimensions passed: 5/11
- Total spec requirements checked: ~85 (workflow-generation.md ~40, adversarial-evaluation-gauntlet.md ~45)
- Total covered: ~76
- Deficiencies: 6 dimensions with specific gaps (9 distinct missing-coverage items)

## Dimension Results

### 1. Behavior — PASS
Every step of `workflow-generation.md` §Behavior (Generate Workflow steps 1–8,
Batch Computation steps 1–4) maps to an item or test: parse/validate → `cmd.generate-workflow`
rejection tests; load config → `notes/cmd.md` field table consumed by `workflow.script-render`/
`cmd.generate-workflow`; compute batches → `workflow.batch-compute`; build prompts
(primary/evaluator/change-detector, commit gating) → `workflow.script-render`; render script
→ `workflow.script-render`; resolve name/collisions → `workflow.output-name`; atomic write +
print → `cmd.generate-workflow`. Postconditions (single new file, no overwrite, plan/config
unchanged) are each tested. The gauntlet's Implement-a-Batch steps (primary once, evaluator
loop, force-accept, next batch) are addressed at the item-description level in
`workflow.script-render` ("an evaluator loop bounded by min_rounds/max_rounds... error recovery
paths"). Item-level coverage is present even though some of the runtime-behavior *tests* for
this same loop are incomplete (see Invariants/Edge Cases/Testing Criteria below).

### 2. Error Handling — PASS
`workflow-generation.md` §Error Handling (malformed JSON, validation failure, missing config
block, mid-write filesystem failure) map 1:1 to `cmd.generate-workflow`'s rejection/edge_case
tests, including the parse-error-names-location requirement. `adversarial-evaluation-gauntlet.md`
§Error Handling (primary null, evaluator null, change-detector git failure) and the Convergence
Signal §Error Handling row (pre-floor unchanged round doesn't end loop) are each named explicitly
in `workflow.script-render`'s edge_case tests.

### 3. Rejection — PASS
All 5 rows of `workflow-generation.md` §Rejection are covered: nonexistent plan path, validation
failure, missing config, empty `context.domain`/`module` (delegated to reused plan validation,
consistent with the spec's own "Caught by plan validation" note), and `.claude/workflows/`
write failure (via the mid-write edge_case test). `adversarial-evaluation-gauntlet.md`'s
§Rejection (no run-time rejection; force-accept is the only terminal mode) is addressed
structurally by the loop-bound tests, modulo the force-accept-at-ceiling gap counted under
Edge Cases/Testing Criteria rather than here.

### 4. Interface — PASS
`forgectl generate-workflow <plan.json>` argument shape, output naming/collision rules, and the
Generated Workflow Script's `meta`/body shape are all represented in items and tests
(`workflow.output-name`, `workflow.script-render` test 1 for the pure-literal `meta`/phases
array, test 2 for the forbidden-API surface). The three `agent()` roles (primary, evaluator,
change-detector) are each individually asserted (tests 3, 5, 7).

### 5. Configuration — FAIL
`workflow-generation.md` §Configuration / `adversarial-evaluation-gauntlet.md` §Configuration
list 8 parameters baked into the script. Two are not covered by any test:
- `implementing.eval.model` and `implementing.eval.type` ("baked into the evaluator `agent()`
  call") have no corresponding test. `workflow.script-render` test 3 verifies only the
  **primary** agent's model/type against `implement.model`/`implement.type`; no analogous test
  asserts the **evaluator** agent's `model`/`agentType` reflect `eval.model`/`eval.type`. The
  only evaluator-related test (test 7) checks prompt *content* (the embedded `GauntletEval`
  text), not the baked model/type parameters.
- `general.enable_commits = true` → "commit instructions appear in the baked prompts" is
  untested. `workflow.script-render` test 6 tests only the negative case ("When enable_commits
  is false, no commit instruction appears..."); there is no positive-case test verifying commit
  instructions are actually baked into the primary/evaluator/change-detector prompts when the
  flag is true, despite this being explicitly named in `workflow-generation.md`'s own "Config
  values are baked" Testing Criteria entry.

### 6. Observability — FAIL
Both specs' Logging tables include a DEBUG row:
- `workflow-generation.md`: "Resolved config values baked into the script; per-batch item ids
  in run order."
- `adversarial-evaluation-gauntlet.md`: "The model/type used for each agent; the resolved
  `min_rounds`/`max_rounds` bounds."

No item or test in the plan references DEBUG-level logging at all — searching all `tests[]`
entries across `evaluators.gauntlet-prompt`, `workflow.batch-compute`, `workflow.output-name`,
`workflow.script-render`, and `cmd.generate-workflow` turns up zero mentions of "DEBUG". INFO
and WARN rows are reasonably covered (INFO via `cmd.generate-workflow` test 2 and
`workflow.script-render` test 8; WARN-collision via `workflow.output-name`), but the DEBUG row
of both tables is entirely unaddressed, which is a full omission, not partial coverage.

### 7. Integration Points — PASS
`workflow-generation.md` §Integration Points (plan-production read-only consumption,
validate-command reuse, config-scaffolding field reads, adversarial-evaluation-gauntlet runtime
contract, Claude Code workflow harness discovery) and `adversarial-evaluation-gauntlet.md`
§Integration Points (shared batch model, plan-production item fields as acceptance criteria,
config-scaffolding parameterization, embedded evaluator prompt, harness primitives/Bash for git)
are each reflected in the plan's `depends_on` graph and item descriptions/refs.

### 8. Invariants — FAIL
`adversarial-evaluation-gauntlet.md` §Invariants includes runtime control-flow invariants:
"The primary agent runs **exactly once per batch**, regardless of how many evaluator rounds
occur," "The evaluator runs **at least `min_rounds`**... and **never more than `max_rounds`**,"
and "A batch ends in exactly one of two ways: convergence... or force-accept." The spec's own
§Testing Criteria section states these loop-control properties "are verified against
**deterministic agent stand-ins** — test doubles whose change/no-change behavior per round is
fixed — since real agent output is non-deterministic. The stand-ins exercise the loop's control
flow." No item or test in the plan constructs any such stand-in harness or executes the
generated script to observe actual invocation counts / round outcomes. Every
`workflow.script-render` test is phrased as a static property of the emitted script's source
(e.g., "the emitted script contains...", "the primary `agent()` call... uses...", "the change-
detector's git result is changed=false and the round counts as converged" — describing generated
text, not an observed execution). A test asserting the loop *body* contains `round < MAX_ROUNDS`
does not demonstrate that the loop actually terminates at the right round when run; the
enforcement mechanism the spec commits to (deterministic stand-in execution) has no
corresponding plan item.

### 9. Edge Cases — FAIL
`adversarial-evaluation-gauntlet.md` §Edge Cases: "The evaluator changes code on every round
through `max_rounds`. Expected behavior: At `max_rounds` the batch is force-accepted in its
current state with a WARN; the workflow proceeds to the next batch." This is the spec's
mainline/canonical force-accept scenario. The plan's only force-accept-related test is
`workflow.script-render`'s edge_case: "When max_rounds is 0, the emitted script contains no
evaluator loop; the batch is accepted after the primary with a force-accept log line" — this
covers only the degenerate `max_rounds = 0` case, not the general case where the loop actually
runs through all `max_rounds` rounds with continued changes and then force-accepts. No test in
the plan exercises this scenario.

### 10. Testing Criteria — FAIL
`adversarial-evaluation-gauntlet.md` §Testing Criteria lists 8 entries; two are not represented:
- **"Force-accept at ceiling"** (`max_rounds = 3`; evaluator changes every round; expect exactly
  3 evaluator rounds, WARN force-accept logged) — no corresponding test (same gap as Edge Cases
  above).
- **"Commits gated by config"** (two workflows differing only in `enable_commits`; the `true` run
  produces a commit, the `false` run does not and has no commit instructions in its prompts) —
  only the `false`-run half is tested (`workflow.script-render` test 6); the `true`-run
  commit-instructions-present half is untested (same root cause as the Configuration-dimension
  gap above).

Additionally, `workflow-generation.md`'s own "Config values are baked" Testing Criteria entry
("`general.enable_commits = true`... commit instructions appear in the baked prompts") is only
partially satisfied for the same reason.

### 11. Dependencies & Format — FAIL
Structural checks pass: 5 unique item IDs, each in exactly one of 4 layers; `depends_on` targets
all exist and stay within equal-or-earlier layers (`workflow.script-render` → L0 `evaluators.
gauntlet-prompt` + L1 `workflow.batch-compute`; `cmd.generate-workflow` → L1/L1/L2, all
earlier-or-equal); the dependency graph is acyclic; top-level keys are limited to
`{context, refs, layers, items}`; all 5 top-level `refs` resolve on disk relative to the plan
directory with no `#anchor` fragments; all `items[].refs` (`notes/*.md`) resolve; all test
categories are in `{functional, rejection, edge_case}`; no `passes`/`rounds` fields present.

However, there is a real path-convention inconsistency within the plan itself. Per
`docs/schemas/plan-json.md` (the current plan-format authority, since `PLAN_FORMAT.md` does not
exist): `items[].specs` and `items[].files` are both "relative to the project root (the
directory containing `.forgectl/`)" — which in this repo is `/Users/jake/Projects/forgectl-root/main`
(one level above the `forgectl/` module directory). The plan's `items[].specs` entries correctly
include the `forgectl/` prefix (e.g. `"forgectl/specs/adversarial-evaluation-gauntlet.md"`), but
every `items[].files` entry omits it: `evaluators/gauntlet-eval.md`, `evaluators/evaluators.go`,
`cmd/generateworkflow.go` (should be `forgectl/evaluators/gauntlet-eval.md`,
`forgectl/evaluators/evaluators.go`, `forgectl/cmd/generateworkflow.go`). `ValidatePlanJSON` does
not existence-check `files`/`specs` on disk, so this would not be caught by `forgectl validate`,
but it is an internal inconsistency against the same root-relative convention the plan uses
correctly for `specs`, and would misdirect an implementer to create files at
`<repo-root>/evaluators/...` and `<repo-root>/cmd/...` instead of inside `forgectl/`.

## Deficiency List (FAIL only)

| # | Dimension | Spec Section | Missing Coverage |
|---|-----------|--------------|-------------------|
| 1 | Configuration | workflow-generation.md §Configuration (`implementing.eval.model`, `implementing.eval.type`); adversarial-evaluation-gauntlet.md §Configuration (same rows) | No test verifies the evaluator `agent()` call's `model`/`agentType` are baked from `eval.model`/`eval.type` (only the primary's model/type is tested, in `workflow.script-render` test 3). |
| 2 | Configuration | workflow-generation.md §Configuration (`general.enable_commits`) + §Testing Criteria "Config values are baked" | No test verifies that `enable_commits = true` causes commit instructions to appear in the baked prompts; only the `false` → absent case is tested. |
| 3 | Observability | workflow-generation.md §Observability (DEBUG row); adversarial-evaluation-gauntlet.md §Observability (DEBUG row) | No item or test in the plan references DEBUG-level logging (resolved config values, per-batch item ids, per-agent model/type, resolved min/max rounds) at all. |
| 4 | Invariants | adversarial-evaluation-gauntlet.md §Invariants (primary-exactly-once; evaluator rounds bounded `[min_rounds, max_rounds]`; batch ends by convergence-or-force-accept) | The spec mandates these be verified via deterministic agent stand-ins executing the generated script; every `workflow.script-render` test is a static assertion on the emitted script's source text, not an execution against stand-ins. No item constructs a stand-in test harness. |
| 5 | Edge Cases | adversarial-evaluation-gauntlet.md §Edge Cases ("evaluator changes code on every round through `max_rounds`") | Only the degenerate `max_rounds = 0` force-accept case is tested (`workflow.script-render` edge_case); the mainline force-accept-at-ceiling scenario (loop runs all `max_rounds` rounds with continued changes, then force-accepts with WARN) has no test. |
| 6 | Testing Criteria | adversarial-evaluation-gauntlet.md §Testing Criteria "Force-accept at ceiling" | No test: `max_rounds = 3`, evaluator changes every round → exactly 3 evaluator rounds, WARN force-accept logged. |
| 7 | Testing Criteria | adversarial-evaluation-gauntlet.md §Testing Criteria "Commits gated by config" | Only the `enable_commits = false` half is tested; the `true`-run commit-instructions-present / commit-occurs half is untested. |
| 8 | Dependencies & Format | docs/schemas/plan-json.md (`items[].files` path resolution, relative to project root) | `items[].files` entries (`evaluators/gauntlet-eval.md`, `evaluators/evaluators.go`, `cmd/generateworkflow.go`) omit the `forgectl/` domain prefix that `items[].specs` entries correctly include for the same items, inconsistent with the actual project-root-relative path convention and the repo's real file layout. |
