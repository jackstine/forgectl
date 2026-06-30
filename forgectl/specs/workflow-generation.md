# Workflow Generation

## Topic of Concern
> The scaffold compiles a validated implementation plan and the active configuration into a single self-contained Claude Code workflow script that encodes the plan as ordered batches, the implementation and evaluation prompts, and the configured loop parameters.

## Context

Forgectl's `implementing` phase drives a human-in-the-loop state machine: the engineer advances one item at a time, spawns evaluators by hand, and records verdicts with `forgectl advance`. The workflow-generation feature offers a second, fully automated path to the same destination. Instead of driving the state machine interactively, the operator asks the scaffold to **emit a Claude Code workflow** — a plain JavaScript file under `.claude/workflows/` — that bakes the plan's work items, the implementation prompts, and the configured loop parameters into one script. Running that workflow implements the whole plan without further scaffold interaction.

This spec defines the generator: the command, what it reads, how it turns a plan into ordered batches, the shape of the artifact it writes, and how it names and protects that file. The *runtime behavior* of the emitted script — the primary-then-evaluator loop, change detection, convergence, and force-accept — is a separate contract owned by **adversarial-evaluation-gauntlet**; this spec commits only to producing a script that conforms to it.

The generator is a compiler: a plan is compiled **once** into a script. The script is self-contained — at run time it does not shell out to `forgectl`, does not read `forgectl-state.json`, and does not re-read `plan.json` or `config`. Everything the run needs is baked in at generation time.

## Depends On
- **plan-production** — defines the `plan.json` (`PlanJSON`) contract this command consumes: `context`, `layers`, `items`, and per-item `description`/`steps`/`files`/`specs`/`tests`.
- **validate-command** — the plan is validated with the same `plan` rules `forgectl validate` applies before any artifact is written.
- **config-scaffolding** — defines `.forgectl/config` and the `[implementing]` block whose fields parameterize the emitted script.
- **adversarial-evaluation-gauntlet** — defines the runtime contract the emitted script must conform to.

## Integration Points

| Adjacent concern | Relationship |
|------------------|-------------|
| plan-production | Consumes the produced `plan.json` read-only: `items[]` become work units, `layers[]` and `depends_on` impose batch ordering, `context.domain`/`context.module` derive the output name. The generator never modifies the plan. |
| validate-command | Reuses the `plan` validation. A plan that `forgectl validate` rejects is rejected here with the same diagnostics, before any file is emitted. |
| config-scaffolding | Reads `implementing.batch`, `implementing.implement.{model,type}`, `implementing.eval.{model,type,min_rounds,max_rounds}`, and `general.enable_commits`. Values are baked into the script; `config` is not consulted at run time. |
| adversarial-evaluation-gauntlet | The emitted script's body is an instance of the gauntlet runtime contract. This spec owns *what is written*; the gauntlet spec owns *what running it does*. |
| Claude Code workflow harness | The emitted artifact is a workflow script: a `meta` pure-literal export plus a body that uses `agent()`, `pipeline()`/loops, `log()`, and `phase()`. The harness discovers it by filename under `.claude/workflows/` and exposes it as a slash command named after `meta.name`. |

---

## Data Models

### Generation Input
The command's inputs, resolved at generation time.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| plan path | string (CLI arg) | yes | no | Filesystem path to a `plan.json`. |
| plan | `PlanJSON` | yes | no | Owned by plan-production; referenced, not redefined. Parsed from the plan path. |
| config | active `.forgectl/config` | yes | no | Owned by config-scaffolding. The `[implementing]` and `[general]` blocks are read. |

### Batch
The unit of work the generator computes from the plan and bakes into the script. A batch is an ordered, non-empty group of plan items.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| index | integer | yes | no | 1-based position in run order. |
| items | array of plan items | yes | no | Between 1 and `implementing.batch` items, in run order. Each item is the full `PlanItem` (`id`, `name`, `description`, `steps`, `files`, `specs`, `tests`). |

Batches are an **interface commitment of the artifact**, not of the on-disk plan: `plan.json` has no `batch` field. The generator derives them deterministically (see *Batch Computation*).

### Generated Workflow Script
The output artifact: a JavaScript file conforming to the Claude Code workflow harness and to **adversarial-evaluation-gauntlet**.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| `meta.name` | string | yes | no | `<domain>-<module>-impl`, sanitized to the slash-command charset. Becomes the workflow's slash command. |
| `meta.description` | string | yes | no | One-line summary naming the domain/module the script implements. |
| `meta.phases` | array (pure literal) | yes | no | One entry per batch phase plus the evaluation phase, matching the `phase()` calls in the body. |
| body | JavaScript | yes | no | Baked batches, baked prompts, and the gauntlet loop parameterized by the baked config values. Emits three `agent()` roles per batch — the primary, the evaluator, and the change-detector (a `Bash`-capable lightweight agent that runs git). Uses only harness primitives — no `fs`, `require`, `Date.now()`, `Math.random()`, or `new Date()`. |

---

## Interface

### Inputs

```
forgectl generate-workflow <plan.json>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<plan.json>` | yes | Path to a produced `plan.json`. Validated before emission. |

The command reads the active `.forgectl/config` from the working tree. No `--force` flag exists; collisions are resolved by renaming (see Outputs).

### Outputs

On success the command **writes one JavaScript file** under `.claude/workflows/` and prints its path and resolved slash-command name.

- **Name:** derived from the plan's `context` as `<domain>-<module>-impl.js`, sanitized to the workflow slash-command charset (lowercase, hyphen-separated).
- **Collision:** if a file with the derived name already exists, the generator does **not** overwrite it. It prepends a disambiguating prefix to produce a fresh, non-colliding name and writes there. The existing file is left byte-for-byte untouched.
- **Content:** a self-contained workflow script (see *Generated Workflow Script*).

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| Plan path does not exist or is unreadable | Non-zero exit; error naming the path | No input to compile. |
| Plan fails `plan` validation (any rule `forgectl validate` enforces) | Non-zero exit; the same diagnostics `validate` emits; **no file written** | A malformed plan must never produce a script. Emission is all-or-nothing. |
| No `.forgectl/config` resolvable from the working tree | Non-zero exit; error stating config is required | The script's loop parameters come from config; without it there is nothing to bake. |
| `context.domain` or `context.module` empty | Caught by plan validation (both are required non-empty) | The output name derives from these. |
| `.claude/workflows/` cannot be created or written | Non-zero exit; error naming the path; no partial file left | A half-written script is worse than none. |

---

## Behavior

### Generate Workflow

#### Preconditions
- The plan path resolves to a readable file.
- A `.forgectl/config` is resolvable from the working tree.

#### Steps
1. **Parse** the plan file as JSON.
2. **Validate** the parsed plan against the `plan` schema — identical rules to `forgectl validate --type plan` (required `context`/`layers`/`items`; unique non-empty item ids; non-empty `description`; present `depends_on` and `tests`; valid test categories; full layer coverage; layer-ordered dependencies; acyclic `depends_on`; on-disk `refs`/notes paths). On any failure, stop and reject without writing.
3. **Load** the active config and read the fields the script will bake: `implementing.batch`, `implementing.implement.{model,type}`, `implementing.eval.{model,type,min_rounds,max_rounds}`, and `general.enable_commits`.
4. **Compute batches** from the plan (see *Batch Computation*).
5. **Build prompts**: for each batch, construct the primary implementation prompt from every item's `description`, `steps`, `files`, `specs`, and `tests`; construct the evaluator prompt referencing the embedded evaluator instructions; construct the change-detector instruction (run git, report changed/clean, commit when permitted). Commit instructions are included in the prompts **only when `general.enable_commits` is true**.
6. **Render** the script: a `meta` pure-literal (name, description, phases) followed by a body that, per batch, runs the primary once and then the evaluator loop bounded by `min_rounds`/`max_rounds` — each round invoking the evaluator and then the change-detector — using the baked models, conforming to **adversarial-evaluation-gauntlet**.
7. **Resolve the output name** from `context` and **resolve collisions** by prefixing if needed.
8. **Write** the file atomically under `.claude/workflows/` and print the final path and slash-command name.

#### Postconditions
- Exactly one new workflow file exists, conforming to the harness and the gauntlet contract.
- No pre-existing file under `.claude/workflows/` was modified or overwritten.
- The plan file and `.forgectl/config` are unchanged.

#### Error Handling
- **Malformed JSON in the plan:** reject with a parse error naming the offending location; no file written.
- **Validation failure:** reject with the validator's diagnostics; no file written.
- **Missing required config block:** reject naming the missing block; no file written.
- **Filesystem write failure mid-render:** the partially written file (if any) is removed or never made visible; the command exits non-zero. The working tree is left without a partial artifact.

### Batch Computation

#### Preconditions
- The plan has passed validation, so `layers` cover every item exactly once and `depends_on` references only equal-or-earlier layers and forms a DAG.

#### Steps
1. Process `layers` in their declared order.
2. Within a layer, order its items by `depends_on` (a stable topological order that preserves declared order as the tiebreak), so a dependency never lands in a later batch than its dependent.
3. Chunk the ordered items into batches of at most `implementing.batch` items, preserving order.
4. Number batches sequentially across all layers, starting at 1.

#### Postconditions
- Every plan item appears in exactly one batch.
- For any item, all of its `depends_on` items appear in the same batch or an earlier-numbered one.
- No batch exceeds `implementing.batch` items; every batch has at least one item.

#### Error Handling
- A dependency cycle or cross-layer-forward dependency cannot occur here: validation rejects such plans before this step runs.

---

## Configuration

The generator reads these fields and **bakes their values** into the emitted script. The script does not re-read config at run time, so regenerating after a config change is the only way to change a script's parameters.

| Parameter | Type | Default | Effect on the emitted script |
|-----------|------|---------|------------------------------|
| `implementing.batch` | int | 2 | Maximum items per batch. The whole batch is given to the primary and to the evaluator. |
| `implementing.implement.model` | string (`opus`/`sonnet`/`haiku`) | `sonnet` | `model` baked into the primary `agent()` call. |
| `implementing.implement.type` | string (`eval`/`explore`/`refine`/`general-purpose`) | `general-purpose` | `agentType` baked into the primary `agent()` call. |
| `implementing.eval.model` | string | `sonnet` | `model` baked into the evaluator `agent()` call. |
| `implementing.eval.type` | string | `general-purpose` | `agentType` baked into the evaluator `agent()` call. |
| `implementing.eval.min_rounds` | int | 1 | Minimum evaluator rounds the loop runs before an unchanged round may end it. |
| `implementing.eval.max_rounds` | int | 3 | Maximum evaluator rounds; the loop force-accepts after this many. |
| `general.enable_commits` | bool | false | When true, commit instructions are baked into the agent prompts and the change-detection agent commits; when false, no commit instructions are emitted. |

`implementing.eval.count` is **not** consulted by this command. The emitted loop runs a single evaluator per round; the number of evaluators is governed solely by `min_rounds`/`max_rounds` (the round count). The field remains valid in config for the interactive `implementing` phase.

Precedence: config values are read once at generation time; there is no CLI override of these values.

---

## Observability

### Logging
| Level | What is logged |
|-------|---------------|
| INFO | Generation started (plan path); plan validated; batch count and total item count; final output path and slash-command name. |
| WARN | Output name collision detected and resolved by prefixing (reports both the intended and the chosen name). |
| ERROR | Plan parse failure; plan validation failure (with diagnostics); missing/unreadable config; filesystem write failure. |
| DEBUG | Resolved config values baked into the script; per-batch item ids in run order. |

### Metrics
None.

---

## Invariants

- A workflow file is written **only if** the plan passes full `plan` validation. An invalid plan never yields an artifact.
- The generator **never overwrites or modifies** an existing file under `.claude/workflows/`; a name collision always produces a new, distinct filename.
- The plan file and `.forgectl/config` are never modified by this command.
- Every plan item appears in exactly one batch in the emitted script, and batch ordering respects `depends_on`.
- The emitted script is self-contained: its source contains no call to `forgectl`, no read of `forgectl-state.json`, and no read of `plan.json` or `config` at run time.
- The emitted script contains no `fs`/`require`/`import`, no `Date.now()`, `Math.random()`, or `new Date()`, and `meta` is a pure literal — it is a valid Claude Code workflow.
- The loop parameters baked into the script equal the config values at generation time; they do not track later config edits.

---

## Edge Cases

- **Scenario:** The plan has a single item.
  **Expected behavior:** One batch of one item is emitted; the script runs the primary once then the evaluator loop.
  **Rationale:** Batch size is a maximum, not a target.

- **Scenario:** `implementing.batch` exceeds the number of items in a layer.
  **Expected behavior:** That layer yields one batch containing all its items.
  **Rationale:** Batches never span layers and never exceed available items.

- **Scenario:** Two items in the same layer where one `depends_on` the other.
  **Expected behavior:** The dependency is ordered before the dependent; if both fall in the same batch, the primary receives both and implements them together (it sees the dependency).
  **Rationale:** Same-batch co-implementation is safe because the whole batch is presented at once.

- **Scenario:** The derived name `<domain>-<module>-impl.js` already exists.
  **Expected behavior:** A prefix is prepended to form a fresh name; the new script is written there; the existing file is untouched; a WARN names both.
  **Rationale:** Regenerating must never destroy a previously emitted (possibly edited) workflow.

- **Scenario:** `context.domain` or `context.module` contains characters illegal in a slash command.
  **Expected behavior:** The name is sanitized to the workflow slash-command charset while remaining derived from `context`.
  **Rationale:** `meta.name` must be a legal slash command.

- **Scenario:** `general.enable_commits` is false.
  **Expected behavior:** No commit instructions appear in any baked prompt; the change-detection agent only reports changed/clean and does not commit.
  **Rationale:** Commit behavior is opt-in via config, surfaced through the generated prompt text.

---

## Testing Criteria

### Valid plan produces a conforming script
- **Verifies:** Generate Workflow; harness-conformance invariants.
- **Given:** A `plan.json` that passes validation and a `.forgectl/config`.
- **When:** `forgectl generate-workflow plan.json` runs.
- **Then:** A file `<domain>-<module>-impl.js` exists under `.claude/workflows/`, exports a pure-literal `meta`, contains no forbidden APIs, and parses as a valid workflow.

### Invalid plan writes nothing
- **Verifies:** Rejection; emission-is-all-or-nothing invariant.
- **Given:** A `plan.json` with a `depends_on` cycle (or any validation failure).
- **When:** The command runs.
- **Then:** It exits non-zero with the validator's diagnostics and no file is created under `.claude/workflows/`.

### Collision never overwrites
- **Verifies:** No-overwrite invariant; collision edge case.
- **Given:** `<domain>-<module>-impl.js` already exists with known contents.
- **When:** The command runs again for the same `context`.
- **Then:** A new file with a prefixed name is created, the original file's bytes are unchanged, and a WARN reports both names.

### Batch size honored
- **Verifies:** Batch Computation; configuration mapping.
- **Given:** A plan whose largest layer has N items and `implementing.batch = k` with `k < N`.
- **When:** The command runs.
- **Then:** No emitted batch contains more than `k` items, and every item appears in exactly one batch.

### Dependency ordering preserved
- **Verifies:** Batch Computation postcondition.
- **Given:** A plan with same-layer items A and B where B `depends_on` A, split across batches.
- **When:** The command runs.
- **Then:** A's batch index is less than or equal to B's batch index.

### Config values are baked
- **Verifies:** Configuration mapping; baking invariant.
- **Given:** A config with `implementing.eval.max_rounds = 5`, `implementing.implement.model = opus`, `general.enable_commits = true`.
- **When:** The command runs.
- **Then:** The emitted script's evaluator loop bound is 5, the primary `agent()` call uses the `opus` model, and commit instructions appear in the baked prompts.

### Inputs are never modified
- **Verifies:** Input-immutability invariant; Generate Workflow postcondition.
- **Given:** A valid `plan.json` and `.forgectl/config` with recorded byte contents.
- **When:** Generation succeeds.
- **Then:** The `plan.json` and `.forgectl/config` files are byte-for-byte unchanged.

### Count is ignored
- **Verifies:** `count` exclusion.
- **Given:** Two configs identical except `implementing.eval.count` (1 vs 4).
- **When:** The command runs against each.
- **Then:** The emitted scripts are equivalent with respect to evaluator fan-out — a single evaluator per round in both.

---

## Implements
- The workflow-generation capability: compiling a produced implementation plan plus the active configuration into a self-contained Claude Code workflow that runs an adversarial mutating evaluation loop.
