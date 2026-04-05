# Reverse Engineering Specifications from Code

## Topic of Concern
> The scaffold reverse-engineers specifications from an existing codebase by surveying existing specs, identifying unspecified behavior, deriving topics of concern, and producing new spec files.

## Context

When a codebase has implemented behavior that predates or was never captured in specifications, the system needs a structured workflow to extract those implicit contracts and make them explicit. This is the inverse of the normal spec-first flow: instead of code implementing specs, specs are derived from code.

The reverse engineering workflow takes a general idea of upcoming work as its starting point, uses that to scope which parts of the codebase are relevant, identifies gaps between existing specs and implemented behavior, and produces new spec files that close those gaps. The result is a spec corpus that accurately reflects what the code does, enabling future changes to proceed spec-first.

This workflow is distinct from writing specs from plans. Plans propose new behavior; reverse engineering captures existing behavior. The output format is identical — both produce spec files conforming to the standard spec format — but the input is code, not planning documents.

The user provides the domains and their order at init time. Forgectl loops SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE per domain before advancing to EXECUTE. The user performs the analysis work; forgectl tracks state and tells the user what action to take next. Forgectl does not collect or store findings — the user holds context between states.

A single reverse engineering queue JSON file accumulates entries across all domains. Each domain's QUEUE state adds that domain's entries to the file.

**Scope exclusions:**
- Writing specs from planning documents (covered by the standard specifying workflow).
- Modifying source code. Reverse engineering is read-only with respect to the codebase.
- Constructing Claude Agent SDK sessions (covered by agent-construction spec).

## Depends On
- **agent-construction** — provides the factory that builds configured Claude Agent SDK sessions for per-spec reverse engineering.
- **state-persistence** — provides the state file read/write mechanism for tracking reverse engineering workflow progress.
- **session-init** — populates the reverse engineering session during `init --phase reverse_engineering`.

## Integration Points

| Spec | Relationship |
|------|-------------|
| agent-construction | EXECUTE delegates agent construction to the factory; the factory receives execute.json and returns configured agent sessions |
| session-init | `init --phase reverse_engineering` creates the initial state with concept, domains, and configuration |
| state-persistence | State file tracks current state, domain index, queue file path, content hash, reconcile round, colleague_review flag |
| activity-logging | State advances produce log entries with domain and state context |
| validate-command | Init input and reverse engineering queue schemas available for standalone validation |

---

## Interface

### Inputs

#### Init Input File

The user provides this JSON file to `forgectl init --phase reverse_engineering`.

```json
{
  "concept": "auth middleware refactor",
  "domains": ["optimizer", "api", "portal"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `concept` | string | yes | A general description of the work to be performed. Used to scope which areas of the codebase and which existing specs are relevant. |
| `domains` | string[] | yes | Ordered list of domains to process. Each domain is processed sequentially: SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE per domain. Order determines processing sequence. |

No additional fields are permitted.

#### Reverse Engineering Queue (produced at QUEUE state)

A JSON file listing every spec to be created or updated. All paths are relative to the domain root (`<project_root>/<domain>/`).

```json
{
  "specs": [
    {
      "name": "Repository Loading",
      "domain": "optimizer",
      "topic": "The optimizer clones or locates a repository and provides its path for downstream modules",
      "file": "specs/repository-loading.md",
      "action": "create",
      "code_search_roots": ["src/repo/", "src/config/"],
      "depends_on": []
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `specs` | array | yes | Ordered list of specs to create or update |
| `specs[].name` | string | yes | Display name for the spec |
| `specs[].domain` | string | yes | Domain grouping. Determines the agent's working directory: `<project_root>/<domain>/` |
| `specs[].topic` | string | yes | One-sentence topic of concern |
| `specs[].file` | string | yes | Target spec file path, relative to the domain root |
| `specs[].action` | string | yes | `"create"` or `"update"`. For updates, `file` is both the source and destination. |
| `specs[].code_search_roots` | string[] | yes | Directories to examine, relative to the domain root; may not be empty |
| `specs[].depends_on` | string[] | yes | Names of specs this one depends on; may be empty array. Used by RECONCILE for cross-referencing. Ignored by EXECUTE. |

No additional fields are permitted.

### Outputs

#### Action Output
At each state, forgectl outputs the current state, domain context (if applicable), and an action block telling the user what to do. See Behavior section for the action output per state.

#### Execution File (`execute.json`)
Generated by forgectl at EXECUTE start. Contains runtime-specific data for the Python subprocess. See EXECUTE behavior and agent-construction spec for the full schema.

#### New Spec Files
One spec file per identified topic of concern, written in the standard spec format. Each spec captures the contracts, behaviors, invariants, edge cases, and testing criteria that the code currently implements.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| No concept provided | Error: "A concept is required to scope the reverse engineering effort." | Without scope, the system cannot determine which code to examine. |
| Empty domains list | Error: "At least one domain is required." | Nothing to process. |
| Duplicate domain in list | Error identifying the duplicate. | Each domain is processed once. |
| `mode` not one of the four valid values | Error listing valid modes. | Invalid execution mode. |
| `code_search_roots` directory does not exist | Error identifying the invalid path. Validated at QUEUE. | Cannot reverse-engineer from a directory that does not exist. |

---

## State Machine

The reverse engineering workflow follows this state machine. SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE loop per domain in the order provided at init. A single queue JSON file accumulates entries across all domains. RECONCILE → RECONCILE_EVAL → (optional COLLEAGUE_REVIEW) → RECONCILE_ADVANCE loop per domain after EXECUTE.

```
ORIENT
  ↓
SURVEY (domain 1) → GAP_ANALYSIS (domain 1) → DECOMPOSE (domain 1) → QUEUE (domain 1)
  ↓
SURVEY (domain 2) → GAP_ANALYSIS (domain 2) → DECOMPOSE (domain 2) → QUEUE (domain 2)
  ↓
  ... (repeat for each domain)
  ↓
EXECUTE
  ↓
RECONCILE (domain 1, round 1)
  ↓
RECONCILE_EVAL (domain 1) ──FAIL──→ RECONCILE (domain 1, round N)
  ↓ PASS (or max rounds)            [loops until PASS or max rounds]
  ↓
  ├── if colleague_review: COLLEAGUE_REVIEW (domain 1)
  ↓
RECONCILE_ADVANCE (domain 1 → domain 2)
  ↓
RECONCILE (domain 2, round 1)
  ↓
  ... (repeat for each domain)
  ↓
RECONCILE_ADVANCE (domain N → DONE)
  ↓
DONE

Note: COLLEAGUE_REVIEW is disabled by default. When disabled, RECONCILE_EVAL advances directly to RECONCILE_ADVANCE.
```

The state file tracks: current state, current domain index (used for both the SURVEY-QUEUE loop and the RECONCILE loop), total domain count, reconcile round, queue file path (set on first QUEUE advance), queue file content hash (for change detection on subsequent QUEUE advances), execute file path (set at EXECUTE start), colleague_review enabled flag.

---

## Behavior

### ORIENT

#### Preconditions
- `forgectl init --phase reverse_engineering --from <input.json>` has completed successfully.
- The state file exists with concept, domains, and configuration locked in.

#### Action Output

```
Phase: reverse_engineering
State: ORIENT
Concept: "{concept}"
Domains: {domain_1} (1/{N}), {domain_2} (2/{N}), ... {domain_N} ({N}/{N})

Action:
  Prepare for reverse engineering across {N} domains.
  Domain order: {domain_1} → {domain_2} → ... → {domain_N}

  Requirements before advancing:
  - Confirm you are familiar with the work concept scope
  - Confirm domain ordering is correct
    (SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE runs per domain in this order)

  Advance to begin SURVEY on domain: {domain_1}
```

#### Postconditions
- User has confirmed readiness.
- State advances to SURVEY with domain index set to 1.

#### Error Handling
- None. ORIENT is informational.

---

### SURVEY

#### Preconditions
- Previous state is ORIENT (for domain 1) or QUEUE (for domains 2+).
- Current domain index is valid.

#### Action Output

```
Phase: reverse_engineering
State: SURVEY
Domain: {domain} ({index}/{N})
Concept: "{concept}"

Action:
  Survey existing specifications in {domain}/specs/.

  Spawn {survey.count} {survey.model} {survey.type} sub-agents
  scoped to {domain}/specs/.

  Read all spec files in the directory to understand what is specified.
  Identify which specs pertain to the concept.

  For each spec, extract:
    - Spec file name
    - Topic of concern
    - Behaviors defined
    - Integration points
    - Dependencies
    - Relevance: whether this spec pertains to the concept

  Disregard specs that do not pertain to the concept.

  Advance when complete.
```

#### Postconditions
- User has surveyed existing specs for the current domain.
- User understands which specs are relevant to the concept and which are not.
- State advances to GAP_ANALYSIS for the same domain.

#### Error Handling
- Domain has no `specs/` directory: action output notes this. The user proceeds with an empty spec inventory for this domain.

---

### GAP_ANALYSIS

#### Preconditions
- SURVEY for the current domain is complete.
- User holds the spec inventory for this domain.

#### Action Output

```
Phase: reverse_engineering
State: GAP_ANALYSIS
Domain: {domain} ({index}/{N})
Concept: "{concept}"

Action:
  Identify unspecified behavior in the {domain} source code
  that pertains to the concept.

  Spawn {gap_analysis.count} {gap_analysis.model} {gap_analysis.type}
  sub-agents scoped to the {domain} source code.

  For each behavior found in code that is not covered by an existing spec:
    - Describe what the behavior does
    - Identify a topic of concern for it:
        - Must be a single topic that fits in one sentence
        - Must not contain "and" conjoining unrelated capabilities
        - Must describe an activity, not a vague statement
        - Valid:   "The optimizer validates repository URLs before cloning"
        - Invalid: "The optimizer handles repos, validation, and caching"
    - Note where in the code it is implemented
    - Note if an existing spec partially covers it (and what the gap is)

  Advance when complete.
  Next: DECOMPOSE for domain {domain}
```

#### Postconditions
- User has identified unspecified behavior in the current domain's source code.
- Each identified behavior has a candidate topic of concern.
- State advances to DECOMPOSE for the current domain.

#### Error Handling
- Domain source code directory is empty: action output notes this. User advances with no gaps found for this domain.

---

### DECOMPOSE

#### Preconditions
- SURVEY and GAP_ANALYSIS are complete for the current domain.
- User holds the spec inventory and gap findings for this domain.

#### Action Output

```
Phase: reverse_engineering
State: DECOMPOSE
Domain: {domain} ({index}/{N})
Concept: "{concept}"

Action:
  Synthesize findings from domain {domain}.

  From the SURVEY and GAP_ANALYSIS results for this domain,
  determine which specifications need to be created or updated.

  For each spec, define:
    - Name (display name)
    - Domain: {domain}
    - Topic of concern:
        - Must be a single topic that fits in one sentence
        - Must not contain "and" conjoining unrelated capabilities
        - Must describe an activity, not a vague statement
        - Valid:   "The optimizer validates repository URLs before cloning"
        - Invalid: "The optimizer handles repos, validation, and caching"
    - File: target path relative to domain root (specs/<kebab-case-name>.md)
    - Action: "create" for new specs, "update" for existing specs with gaps
    - Code search roots (directories relative to domain root)
    - Dependencies on other specs

  Decide:
    - Which gaps warrant new specs vs. updates to existing specs
    - How to group related behaviors into single-topic specs

  Advance when the spec list for this domain is finalized.
```

#### Postconditions
- User has a finalized list of specs to create or update for the current domain.
- Each spec has a valid topic of concern.
- State advances to QUEUE for the current domain.

#### Error Handling
- None. DECOMPOSE is user-driven synthesis.

---

### QUEUE

QUEUE accumulates entries into a single reverse engineering queue JSON file across all domains.

#### Preconditions
- DECOMPOSE for the current domain is complete.
- User has a finalized spec list for this domain.

#### First Advance (domain 1 — file path not yet set)

The user provides the queue JSON file path on the first advance. Forgectl stores the path and a content hash in the state file.

```
Phase: reverse_engineering
State: QUEUE
Domain: {domain} ({index}/{N})
Concept: "{concept}"

Action:
  Produce the reverse engineering queue JSON file with entries for domain {domain}.

  Requirements:
    - All paths relative to domain root (<project_root>/{domain}/)
    - Order entries by dependency: specs with no dependencies first
    - code_search_roots must be non-empty for every entry
    - No circular dependencies

  Advance with the queue file:
    forgectl advance --file <queue.json>
```

#### Subsequent Advances (domains 2+ — file path already set)

The user updates the existing queue file by adding entries for the current domain. Forgectl re-reads from the stored path, checks for changes, and validates.

```
Phase: reverse_engineering
State: QUEUE
Domain: {domain} ({index}/{N})
Concept: "{concept}"
Queue file: {stored_path}

Action:
  Add entries for domain {domain} to the existing queue file.

  Update the queue file at: {stored_path}
  Add new entries for this domain alongside existing entries.

  Advance when the file is updated:
    forgectl advance
```

#### Advance Behavior

| Advance | `--file` flag | Behavior |
|---------|---------------|----------|
| First (no stored path) | Required | Store path, compute content hash, validate schema, validate `code_search_roots` directories exist. If valid → advance. If invalid → error with violations. |
| Subsequent (path stored) | Not accepted | Re-read file from stored path. Compare content hash. If unchanged → error: "Queue file has not changed. Update the file and retry." If changed → recompute hash, validate schema, validate `code_search_roots` directories exist. If valid → advance. If invalid → error with violations. |

#### Domain Validation

During QUEUE validation, forgectl verifies that every entry's `domain` field matches one of the domains provided at init. Entries with unrecognized domains are rejected.

Error output:

```
Queue entry "{name}" has domain "{domain}" which is not in the
initialized domain list.

Valid domains: {domain_1}, {domain_2}, ... {domain_N}

To add a new domain, run:
  forgectl add-domain <domain>
```

#### `forgectl add-domain` Command (active during QUEUE only)

Adds a domain to the initialized domain list. The new domain is appended to the end of the domain order. This command is only available during the QUEUE state.

```
forgectl add-domain <domain>
```

- The domain must not already exist in the list (error if duplicate).
- The domain is added to the state file's domain list.
- The domain count updates.
- The SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE loop does not re-run for the added domain — the user is responsible for having performed the analysis before adding entries for it.
- Called outside of QUEUE state: error: "forgectl add-domain is only available during the QUEUE state."

#### Path Validation

During QUEUE validation, forgectl resolves each entry's `code_search_roots` paths against the domain root (`<project_root>/<domain>/`) and verifies each directory exists. Missing directories are reported as validation errors.

#### Postconditions
- Queue JSON file exists at the stored path with entries for all processed domains.
- All entries pass schema validation.
- All `code_search_roots` directories exist on disk.
- State advances to SURVEY for the next domain, or EXECUTE if all domains are complete.

#### Error Handling
- First advance missing `--file`: error: "Queue file path required. Use: forgectl advance --file <queue.json>"
- Subsequent advance includes `--file`: error: "Queue file path already set to {stored_path}. Update that file and run: forgectl advance"
- Queue file not found at stored path: error with path.
- Schema validation failure: error listing violations. User corrects and retries `forgectl advance`.
- `code_search_roots` directory does not exist: error identifying the entry and the missing path.
- Content unchanged: error: "Queue file has not changed. Update the file and retry."

---

### EXECUTE

#### Preconditions
- The reverse engineering queue JSON file exists and is validated.
- All `code_search_roots` directories verified at QUEUE time.
- For each entry, the domain's `specs/` directory exists. Forgectl creates this directory before invoking the subprocess if it does not exist.
- The Python subprocess and Claude Agent SDK are available.

#### Steps
1. Forgectl ensures `<project_root>/<domain>/specs/` exists for each domain in the queue.
2. Forgectl generates `execute.json` by:
   a. Copying all spec entries from `queue.json` into the `specs` array, adding `result: null` to each.
   b. Adding `project_root` (absolute path).
   c. Adding `config` with drafter, mode-specific, and (if peer_review) peer review settings from the state file. Only the active mode's config block is included.
3. Forgectl invokes the Python subprocess:
   ```
   python reverse_engineer.py --execute execute.json
   ```
4. The Python subprocess reads `execute.json`, constructs agent sessions using the agent construction factory (see agent-construction spec), runs them according to the configured mode, and writes results back into `execute.json`.
5. When the subprocess exits, forgectl reads `execute.json` and checks each entry's `result`:
   - All `status: "success"` → advance to RECONCILE.
   - Any `status: "failure"` → stay in EXECUTE, report which entries failed.

#### Postconditions
- `execute.json` exists with a non-null `result` for every spec entry.
- A spec file exists at each `file` path for every successful entry.
- Each spec is written in the standard spec format.
- No files other than the designated spec files were modified by any agent.
- State advances to RECONCILE (if all succeed) or stays in EXECUTE (if any fail).

#### Error Handling
- Queue contains zero entries: error: "Queue contains zero entries. Nothing to execute." State stays in EXECUTE. Subprocess is not invoked.
- Domain `specs/` directory creation fails (permissions, disk): error with path. Subprocess is not invoked.
- `execute.json` generation fails: error. Subprocess is not invoked.
- Subprocess exits with non-zero code: forgectl reads `execute.json` for per-entry results. If `execute.json` is unreadable, forgectl outputs the subprocess failure action:

```
Phase: reverse_engineering
State: EXECUTE

STOP there was an issue with the subprocess for reverse engineering.
Please consult with your user and inform them that there was an issue
with the Python subprocess running Claude Agent SDK.

{full stderr/stack trace from the subprocess}
```

- Individual agent failures are captured in `execute.json` per entry. Forgectl reports which entries failed.

---

### RECONCILE

RECONCILE runs per domain, looping with RECONCILE_EVAL until PASS or max rounds.

#### Preconditions
- EXECUTE is complete (for domain 1) or RECONCILE_ADVANCE from previous domain.
- All spec files for the current domain have been produced.

#### Action Output — Round 1

```
Phase: reverse_engineering
State: RECONCILE
Domain: {domain} ({index}/{N})
Concept: "{concept}"
Round: 1

Specs created or updated for this domain:
  - {domain}/{file_1}  ({action_1})
    depends_on: [{dep_1}, {dep_2}]
  - {domain}/{file_2}  ({action_2})
    depends_on: []
  - {domain}/{file_3}  ({action_3})
    depends_on: [{dep_3}]

Action:
  Cross-reference specifications for domain {domain}.

  For every spec that was created or updated, use its depends_on
  to add cross-references to the corresponding specs.
  Update both the new/updated spec and the spec it references:
    - Add Depends On entries in the new/updated spec
    - Add Integration Points in both directions
      (if A depends on B, both A and B reference each other)

  Verify consistency:
    - Every Depends On reference points to a spec that exists
    - Every Depends On has a corresponding Integration Points row
      in the referenced spec
    - Integration Points are symmetric (A ↔ B)
    - Spec names are consistent across all references
    - No circular dependencies in the Depends On graph

  Stage all changes:
    git add the modified spec files.

  Advance when reconciliation is complete and changes are staged.
```

#### Action Output — Subsequent Rounds (after RECONCILE_EVAL FAIL)

```
Phase: reverse_engineering
State: RECONCILE
Domain: {domain} ({index}/{N})
Concept: "{concept}"
Round: {round}

Specs created or updated for this domain:
  - {domain}/{file_1}  ({action_1})
    depends_on: [{dep_1}, {dep_2}]
  - {domain}/{file_2}  ({action_2})
    depends_on: []
  - {domain}/{file_3}  ({action_3})
    depends_on: [{dep_3}]

Action:
  Reconciliation evaluation failed on the previous round.
  Address the findings from the evaluation report and re-reconcile.

  For every spec that was created or updated, use its depends_on
  to add cross-references to the corresponding specs.
  Update both the new/updated spec and the spec it references:
    - Add Depends On entries in the new/updated spec
    - Add Integration Points in both directions
      (if A depends on B, both A and B reference each other)

  Verify consistency:
    - Every Depends On reference points to a spec that exists
    - Every Depends On has a corresponding Integration Points row
      in the referenced spec
    - Integration Points are symmetric (A ↔ B)
    - Spec names are consistent across all references
    - No circular dependencies in the Depends On graph

  Stage all changes:
    git add the modified spec files.

  Advance when reconciliation is complete and changes are staged.
```

#### Postconditions
- All cross-references for the current domain are symmetric and valid.
- No dangling references exist.
- Changes are staged.
- State advances to RECONCILE_EVAL for the current domain.

#### Error Handling
- A spec file from the queue is missing: report the gap. Do not fabricate a spec.
- Reconciliation introduces a conflict (e.g., adding an integration point to an existing spec changes its scope): flag for user review.

---

### RECONCILE_EVAL

#### Preconditions
- RECONCILE for the current domain is complete. Changes are staged.
- Reconcile round is within `max_rounds`.

#### Action Output

```
Phase: reverse_engineering
State: RECONCILE_EVAL
Domain: {domain} ({index}/{N})
Concept: "{concept}"
Round: {round}

Action:
  Evaluate cross-spec consistency for domain {domain}.

  Spawn {reconcile.eval.count} {reconcile.eval.model} {reconcile.eval.type}
  sub-agents to evaluate the reconciliation.

  Instruct your sub-agents to run:
    forgectl eval

  This outputs the evaluation prompt with the full spec files
  and consistency checklist for the sub-agents to review.

  After the sub-agents complete their evaluation, advance with the verdict:
    forgectl advance --verdict PASS --eval-report <path>
    forgectl advance --verdict FAIL --eval-report <path>

  Eval reports are written to: {domain}/specs/.eval/reconciliation-r{round}.md
```

#### `forgectl eval` Command (active during RECONCILE_EVAL)

When a sub-agent runs `forgectl eval`, forgectl outputs the evaluation prompt from the embedded evaluator file (`forgectl/evaluators/reconcile-eval.md`). The output is populated with:

- The list of specs created or updated for this domain, with their `depends_on` references
- The eval report output path: `{domain}/specs/.eval/reconciliation-r{round}.md`
- The current round number

The evaluator prompt instructs the sub-agents to:
1. Read each spec file listed in full
2. Read any spec referenced in `depends_on` that is not in the list
3. Evaluate against 7 dimensions: completeness, depends_on validity, integration points symmetry, depends_on ↔ integration points correspondence, naming consistency, no circular dependencies, topic of concern
4. Write the evaluation report with PASS/FAIL per dimension and an overall verdict

See `forgectl/evaluators/reconcile-eval.md` for the full evaluator prompt.

#### Postconditions
- Evaluation report exists at `{domain}/specs/.eval/reconciliation-r{round}.md`.
- If PASS and round >= `min_rounds`: if `colleague_review` is enabled, state advances to COLLEAGUE_REVIEW. If disabled, state advances to RECONCILE_ADVANCE.
- If PASS and round < `min_rounds`: state returns to RECONCILE for another round (minimum not yet met), round increments.
- If FAIL and round < `max_rounds`: state returns to RECONCILE for corrections, round increments.
- If FAIL and round >= `max_rounds`: if `colleague_review` is enabled, state advances to COLLEAGUE_REVIEW. If disabled, state advances to RECONCILE_ADVANCE.

#### Error Handling
- Sub-agent fails to produce a report: user retries or manually evaluates.
- `forgectl eval` called outside of RECONCILE_EVAL state: error: "forgectl eval is only available during RECONCILE_EVAL."

---

### COLLEAGUE_REVIEW

Disabled by default. Enabled via `colleague_review = true` in config. When disabled, RECONCILE_EVAL advances directly to RECONCILE_ADVANCE, skipping this state entirely.

#### Preconditions
- `colleague_review` is enabled in config.
- RECONCILE_EVAL has produced a PASS verdict, or `max_rounds` has been reached.

#### Action Output

```
Phase: reverse_engineering
State: COLLEAGUE_REVIEW
Domain: {domain} ({index}/{N})
Concept: "{concept}"

Action:
  STOP and review the specifications with your colleague.

  Advance when the review is complete:
    forgectl advance
```

#### Postconditions
- User and colleague have reviewed the specifications for the current domain.
- State advances to RECONCILE_ADVANCE.

#### Error Handling
- None. This is a human review gate.

---

### RECONCILE_ADVANCE

#### Preconditions
- COLLEAGUE_REVIEW for the current domain is complete (if enabled), or RECONCILE_EVAL has completed (if disabled).

#### Action Output

```
Phase: reverse_engineering
State: RECONCILE_ADVANCE
Domain: {domain} ({index}/{N}) → {next_domain | DONE}

Action:
  Domain {domain} reconciliation complete.

  {Next: RECONCILE for domain {next_domain} ({next_index}/{N}) | All domains reconciled. Advancing to DONE.}

  Advance to proceed.
```

#### Postconditions
- If more domains remain: state advances to RECONCILE for the next domain, round resets to 1.
- If all domains are complete: state advances to DONE.

#### Error Handling
- None. This is a transition state.

---

### DONE

The reverse engineering workflow is complete. All spec files have been produced, verified, and reconciled across all domains.

---

## Configuration

All configuration is read from `.forgectl/config` (TOML) and locked into the state file at init time.

### Init Input Validation
- `concept` is non-empty.
- `domains` is non-empty, contains no duplicates.
- `mode` is one of: `single_shot`, `self_refine`, `multi_pass`, `peer_review`.

### Mode Defaults

Each mode has its own default values. Defaults are only applied when that mode is selected. Forgectl does not populate, store, or pass configuration for inactive modes.

**`single_shot`** — No mode-specific parameters.

**`self_refine`** (default mode):
| Parameter | Default | Description |
|-----------|---------|-------------|
| `rounds` | `2` | Number of self-review follow-ups after the initial draft |

**`multi_pass`**:
| Parameter | Default | Description |
|-----------|---------|-------------|
| `passes` | `2` | Number of full batch re-runs. Creates become updates after pass 1. |

**`peer_review`**:
| Parameter | Default | Description |
|-----------|---------|-------------|
| `reviewers` | `3` | Number of reviewer sub-agents spawned in parallel per drafter |
| `rounds` | `1` | Number of peer review cycles |
| `subagents.model` | `opus` | Model for reviewer sub-agents |
| `subagents.type` | `explorer` | Role for reviewer sub-agents |

### Full Configuration Reference

```toml
[reverse_engineering]
# Execution mode: exactly one of the four
# Options: "single_shot", "self_refine", "multi_pass", "peer_review"
mode = "self_refine"

[reverse_engineering.self_refine]
rounds = 2                    # only applied when mode = "self_refine"

[reverse_engineering.multi_pass]
passes = 2                    # only applied when mode = "multi_pass"

[reverse_engineering.peer_review]
reviewers = 3                 # only applied when mode = "peer_review"
rounds = 1                    # only applied when mode = "peer_review"

# Primary agent that drafts the spec
[reverse_engineering.drafter]
model = "opus"

# Sub-agents the drafter spawns to explore code during drafting
[reverse_engineering.drafter.subagents]
model = "opus"
type = "explorer"
count = 3

# Sub-agents the drafter spawns for peer review (only used in peer_review mode)
[reverse_engineering.peer_review.subagents]
model = "opus"
type = "explorer"
# count comes from peer_review.reviewers

# Reconciliation eval rounds
[reverse_engineering.reconcile]
min_rounds = 1
max_rounds = 3
colleague_review = false   # disabled by default; enable to add a human review gate after reconciliation eval

# Sub-agents for reconciliation evaluation
[reverse_engineering.reconcile.eval]
count = 1
model = "opus"
type = "general-purpose"

# Sub-agents the user spawns during SURVEY (action output only)
[reverse_engineering.survey]
model = "haiku"
type = "explorer"
count = 2

# Sub-agents the user spawns during GAP_ANALYSIS (action output only)
[reverse_engineering.gap_analysis]
model = "sonnet"
type = "explorer"
count = 5
```

### Configuration Purpose Map

| Config Block | Consumed By | When | Purpose |
|-------------|-------------|------|---------|
| `mode` | Forgectl + Python | Init validation, EXECUTE | Determines which execution flow runs |
| `self_refine.*` | Python (via execute.json) | EXECUTE | How many self-review rounds |
| `multi_pass.*` | Python (via execute.json) | EXECUTE | How many full re-runs |
| `peer_review.*` | Python (via execute.json) | EXECUTE | How many reviewers, how many rounds |
| `peer_review.subagents` | Primary agent (via prompt) | EXECUTE peer review | Reviewer sub-agent model and type |
| `drafter` | Python (via execute.json) | EXECUTE | Model for the primary SDK agent |
| `drafter.subagents` | Primary agent (via prompt) | EXECUTE drafting | Code exploration sub-agents during spec writing |
| `reconcile` | Forgectl | RECONCILE_EVAL | Min/max rounds for reconciliation eval loop |
| `reconcile.colleague_review` | Forgectl | After RECONCILE_EVAL | Whether COLLEAGUE_REVIEW gate is enabled (default: false) |
| `reconcile.eval` | Forgectl action output | RECONCILE_EVAL | Sub-agents for reconciliation evaluation |
| `survey` | Forgectl action output | SURVEY | Displayed to user — what sub-agents to spawn |
| `gap_analysis` | Forgectl action output | GAP_ANALYSIS | Displayed to user — what sub-agents to spawn |

Note: tool list, permission mode, CLAUDE.md setting, and prompt files are constants owned by the Python package. They do not appear in forgectl config or the state file.

### execute.json Config Structure (per mode)

Only the active mode's config block is included in `execute.json`.

**single_shot:**
```json
{
  "config": {
    "mode": "single_shot",
    "drafter": {
      "model": "opus",
      "subagents": { "model": "opus", "type": "explorer", "count": 3 }
    }
  }
}
```

**self_refine:**
```json
{
  "config": {
    "mode": "self_refine",
    "drafter": {
      "model": "opus",
      "subagents": { "model": "opus", "type": "explorer", "count": 3 }
    },
    "self_refine": { "rounds": 2 }
  }
}
```

**multi_pass:**
```json
{
  "config": {
    "mode": "multi_pass",
    "drafter": {
      "model": "opus",
      "subagents": { "model": "opus", "type": "explorer", "count": 3 }
    },
    "multi_pass": { "passes": 2 }
  }
}
```

**peer_review:**
```json
{
  "config": {
    "mode": "peer_review",
    "drafter": {
      "model": "opus",
      "subagents": { "model": "opus", "type": "explorer", "count": 3 }
    },
    "peer_review": {
      "reviewers": 3,
      "rounds": 1,
      "subagents": { "model": "opus", "type": "explorer" }
    }
  }
}
```

---

## Observability

### Logging

| Level | What is logged |
|-------|---------------|
| INFO | Workflow started with concept; domain processing started (domain name, index); SURVEY complete for domain; GAP_ANALYSIS complete for domain; QUEUE file produced (spec count); EXECUTE started (mode, agent count); each spec file produced; RECONCILE complete for domain; RECONCILE_EVAL verdict (PASS/FAIL, round); COLLEAGUE_REVIEW entered (when enabled); RECONCILE_ADVANCE domain transition; workflow DONE |
| WARN | Domain has no `specs/` directory during SURVEY; empty source directory during GAP_ANALYSIS; agent produced minimal spec due to irrelevant code |
| ERROR | Queue JSON validation failure; `code_search_roots` directory not found; circular dependency detected; agent session failure; missing spec file during RECONCILE verification |
| DEBUG | Domain index progression; sub-agent config applied; queue entry details; execution mode and parameters; execute.json generation |

---

## Invariants

1. **Read-only codebase.** The reverse engineering workflow never modifies source code. It reads code to produce specs.
2. **One topic per spec.** Every spec produced passes the topic-of-concern test. No spec covers multiple unrelated responsibilities.
3. **Spec format compliance.** Every spec produced conforms to the standard spec format, regardless of whether it was written from a plan or reverse-engineered from code.
4. **Dependency ordering.** The spec queue is ordered such that no spec appears before a spec it depends on.
5. **Domain-root scoping.** All paths in the queue (`file`, `code_search_roots`) are relative to `<project_root>/<domain>/`. Each agent's working directory is the domain root.
6. **Single-file write.** Each agent writes or edits exactly one file — the `file` specified in its queue entry. No other files are modified.
7. **Spec directory pre-exists.** The domain's `specs/` directory exists before any agent is invoked. Forgectl creates it if absent.
8. **Sequential domain processing.** Domains are processed in the order provided at init. SURVEY, GAP_ANALYSIS, DECOMPOSE, and QUEUE complete for domain N before domain N+1 begins.
9. **Single queue file.** One reverse engineering queue JSON file accumulates entries across all domains. The file path is set on the first QUEUE advance and reused for all subsequent domains.
10. **Queue file change detection.** On subsequent QUEUE advances, forgectl rejects unchanged files. The user must modify the file before advancing.
11. **Exactly one execution mode.** One of `single_shot`, `self_refine`, `multi_pass`, or `peer_review` is active per session. No combinations.
12. **Path validation at QUEUE.** All `code_search_roots` directories are verified to exist on disk during QUEUE validation. Invalid paths are rejected before EXECUTE.
13. **`depends_on` is RECONCILE metadata.** The `depends_on` field in queue entries is used by RECONCILE to wire up cross-references. It is ignored by EXECUTE and the Python subprocess.
14. **Per-domain reconciliation.** RECONCILE, RECONCILE_EVAL, and (if enabled) COLLEAGUE_REVIEW run per domain in the same order as the SURVEY-QUEUE loop.
15. **Reconcile eval bounded.** RECONCILE_EVAL loops at most `max_rounds` times per domain. At max rounds, the workflow advances to COLLEAGUE_REVIEW (if enabled) or RECONCILE_ADVANCE (if disabled) regardless of verdict.
16. **Colleague review is optional.** COLLEAGUE_REVIEW is disabled by default. When disabled, the state is skipped entirely — RECONCILE_EVAL advances directly to RECONCILE_ADVANCE. When enabled, it runs exactly once per domain.
17. **`forgectl eval` is state-gated.** The `forgectl eval` command is only active during RECONCILE_EVAL. It outputs the embedded evaluator prompt populated with the current domain's spec list.
18. **Inactive mode config is not applied.** Forgectl only populates, stores, and passes configuration for the active execution mode. Default values for inactive modes are not loaded into the state file or `execute.json`.
19. **Queue entries match initialized domains.** Every entry's `domain` field in the queue must match a domain in the initialized domain list. Unrecognized domains are rejected at QUEUE validation.
20. **`forgectl add-domain` is state-gated.** The `forgectl add-domain` command is only available during the QUEUE state.

---

## Edge Cases

- **Scenario:** The entire codebase is already fully specified.
  - **Expected behavior:** GAP_ANALYSIS across all domains finds no unspecified behavior. DECOMPOSE produces an empty spec list. QUEUE produces a JSON with zero entries. When EXECUTE begins with an empty queue, forgectl errors: "Queue contains zero entries. Nothing to execute." State stays in EXECUTE.
  - **Rationale:** An empty queue means no work to perform. Erroring is clearer than silently skipping to RECONCILE with nothing to reconcile.

- **Scenario:** A behavior is split across multiple code directories with no single obvious home.
  - **Expected behavior:** The behavior is assigned to the spec whose topic of concern most closely aligns. The code search roots for that spec include all relevant directories.
  - **Rationale:** Code organization does not dictate spec organization. The spec reflects the logical concern, not the physical layout.

- **Scenario:** Existing spec partially covers a behavior, but the code has diverged (code does more than the spec describes).
  - **Expected behavior:** GAP_ANALYSIS records the divergence. The user decides during DECOMPOSE whether to update the existing spec or create a new spec for the additional behavior.
  - **Rationale:** Reverse engineering identifies gaps but does not unilaterally modify existing specs — that is a design decision.

- **Scenario:** A domain has no `specs/` directory.
  - **Expected behavior:** SURVEY notes the absence. The user proceeds with an empty spec inventory for this domain. GAP_ANALYSIS treats all behavior as unspecified.
  - **Rationale:** This is a valid starting state — the domain has never been specified.

- **Scenario:** Two unspecified behaviors are tightly coupled but logically distinct.
  - **Expected behavior:** Two separate specs are created during DECOMPOSE, each with its own topic of concern. Integration points link them.
  - **Rationale:** Tight coupling in code does not justify combining specs. Each spec has one topic.

- **Scenario:** Code contains dead code or unreachable paths.
  - **Expected behavior:** Dead code is excluded from GAP_ANALYSIS findings. Only reachable, exercised behavior is reverse-engineered into specs.
  - **Rationale:** Specs capture what the system does, not what it contains. Dead code does nothing.

- **Scenario:** `multi_pass` with `passes: 3`.
  - **Expected behavior:** The subprocess runs 3 times. Pass 1 uses the original queue actions. Passes 2 and 3 override all `action: "create"` entries to `action: "update"`.
  - **Rationale:** Subsequent passes refine existing output rather than recreating from scratch.

- **Scenario:** `peer_review` with `reviewers: 3` and `rounds: 2`.
  - **Expected behavior:** After the initial draft, the drafter spawns 3 reviewer sub-agents in parallel. This review cycle repeats 2 times. Each round reviews the spec as it stands after the previous round's edits.
  - **Rationale:** Multiple rounds allow reviewer feedback to compound.

- **Scenario:** A single domain appears in the queue but the user provided three domains at init.
  - **Expected behavior:** SURVEY, GAP_ANALYSIS, DECOMPOSE, and QUEUE still run for all three domains. The queue produced at QUEUE may only contain specs for the one domain where gaps were found.
  - **Rationale:** All domains are surveyed and analyzed regardless of whether gaps are found. The queue reflects only actionable work.

- **Scenario:** `code_search_roots` directory exists at QUEUE time but is deleted before EXECUTE.
  - **Expected behavior:** The agent reports failure for that entry. The directory was validated at QUEUE but is no longer present at EXECUTE. Forgectl reports the failure from `execute.json`.
  - **Rationale:** QUEUE validates what it can. Filesystem changes between states are runtime failures, not validation failures.

---

## Testing Criteria

### Init validates domains
- **Verifies:** Init input validation.
- **Given:** Input JSON with `domains: ["api", "api"]` (duplicate).
- **When:** `forgectl init --phase reverse_engineering --from input.json`
- **Then:** Error identifying the duplicate. Init fails.

### Init validates execution mode
- **Verifies:** Mode validation.
- **Given:** Config with `mode = "invalid_mode"`.
- **When:** `forgectl init --phase reverse_engineering --from input.json`
- **Then:** Error listing valid modes. Init fails.

### ORIENT displays domain order
- **Verifies:** ORIENT action output.
- **Given:** Init with `domains: ["optimizer", "api", "portal"]`.
- **When:** State is ORIENT, `forgectl status` is run.
- **Then:** Output lists all three domains in order with indices.

### SURVEY action uses configured sub-agents
- **Verifies:** SURVEY action output reflects config.
- **Given:** Config with `survey.count = 4`, `survey.model = "sonnet"`, `survey.type = "explorer"`.
- **When:** State is SURVEY, `forgectl status` is run.
- **Then:** Action output says "Spawn 4 sonnet explorer sub-agents".

### GAP_ANALYSIS action uses configured sub-agents
- **Verifies:** GAP_ANALYSIS action output reflects config.
- **Given:** Config with `gap_analysis.count = 3`, `gap_analysis.model = "opus"`, `gap_analysis.type = "explorer"`.
- **When:** State is GAP_ANALYSIS, `forgectl status` is run.
- **Then:** Action output says "Spawn 3 opus explorer sub-agents".

### GAP_ANALYSIS action includes topic-of-concern rules
- **Verifies:** GAP_ANALYSIS action output includes topic formatting rules.
- **Given:** Current domain is "optimizer".
- **When:** State is GAP_ANALYSIS, `forgectl status` is run.
- **Then:** Action output includes topic-of-concern requirements: single sentence, no "and", describes an activity. Valid/invalid examples are shown.

### Domain loop advances correctly
- **Verifies:** Sequential domain processing.
- **Given:** Domains: ["optimizer", "api"]. State: QUEUE, domain index 1. Queue file validated.
- **When:** User advances.
- **Then:** State becomes SURVEY, domain index advances to 2 (api).

### Last domain advances to EXECUTE
- **Verifies:** Transition from last domain to EXECUTE.
- **Given:** Domains: ["optimizer", "api"]. State: QUEUE, domain index 2. Queue file validated.
- **When:** User advances.
- **Then:** State becomes EXECUTE.

### QUEUE first advance requires --file
- **Verifies:** File path required on first QUEUE advance.
- **Given:** State is QUEUE, domain index 1. No queue file path stored.
- **When:** `forgectl advance` (no `--file` flag).
- **Then:** Error: "Queue file path required."

### QUEUE validates reverse engineering queue schema
- **Verifies:** Queue JSON validation.
- **Given:** Queue JSON with an entry missing `code_search_roots`.
- **When:** `forgectl advance --file queue.json`
- **Then:** Error listing the validation violation.

### QUEUE rejects entries with unrecognized domains
- **Verifies:** Domain validation at QUEUE.
- **Given:** Init with `domains: ["optimizer", "api"]`. Queue JSON contains an entry with `domain: "portal"`.
- **When:** `forgectl advance --file queue.json`
- **Then:** Error identifies the entry and lists valid domains ("optimizer", "api"). Error suggests `forgectl add-domain portal`.

### forgectl add-domain adds a domain during QUEUE
- **Verifies:** add-domain command.
- **Given:** State is QUEUE. Init domains: ["optimizer", "api"].
- **When:** `forgectl add-domain portal`
- **Then:** Domain list becomes ["optimizer", "api", "portal"]. Subsequent QUEUE validation accepts entries with `domain: "portal"`.

### forgectl add-domain rejects duplicate domain
- **Verifies:** add-domain duplicate check.
- **Given:** State is QUEUE. Init domains: ["optimizer", "api"].
- **When:** `forgectl add-domain api`
- **Then:** Error: domain "api" already exists.

### forgectl add-domain blocked outside QUEUE
- **Verifies:** State-gating of add-domain.
- **Given:** State is SURVEY.
- **When:** `forgectl add-domain portal`
- **Then:** Error: "forgectl add-domain is only available during the QUEUE state."

### EXECUTE rejects empty queue
- **Verifies:** Empty queue error.
- **Given:** Queue JSON has zero entries.
- **When:** EXECUTE begins.
- **Then:** Error: "Queue contains zero entries. Nothing to execute." State stays in EXECUTE.

### QUEUE validates code_search_roots exist on disk
- **Verifies:** Path validation at QUEUE.
- **Given:** Queue JSON with `code_search_roots: ["src/nonexistent/"]`. Directory does not exist.
- **When:** `forgectl advance --file queue.json`
- **Then:** Error identifying the entry and the missing directory.

### QUEUE subsequent advance rejects --file flag
- **Verifies:** File path cannot be changed after first QUEUE.
- **Given:** State is QUEUE, domain index 2. Queue file path already stored.
- **When:** `forgectl advance --file other-queue.json`
- **Then:** Error: "Queue file path already set."

### QUEUE subsequent advance detects unchanged file
- **Verifies:** Change detection on subsequent QUEUE advances.
- **Given:** State is QUEUE, domain index 2. Queue file has not changed since last validation.
- **When:** `forgectl advance`
- **Then:** Error: "Queue file has not changed."

### QUEUE subsequent advance validates changed file
- **Verifies:** Re-validation after file change.
- **Given:** State is QUEUE, domain index 2. User has added entries and the file content hash differs.
- **When:** `forgectl advance`
- **Then:** File is re-validated (schema + path existence). If valid, state advances.

### EXECUTE creates specs directories
- **Verifies:** Spec directory pre-creation.
- **Given:** Queue contains domain "optimizer". `optimizer/specs/` does not exist.
- **When:** EXECUTE begins.
- **Then:** `optimizer/specs/` is created before the subprocess is invoked.

### EXECUTE generates execute.json with active mode config only
- **Verifies:** Execution file contains only the active mode's config.
- **Given:** Config with `mode = "peer_review"`, `peer_review.reviewers = 3`, `self_refine.rounds = 2`.
- **When:** EXECUTE generates `execute.json`.
- **Then:** `execute.json` contains `config.peer_review` block. Does not contain `config.self_refine`.

### Inactive mode defaults are not stored in state file
- **Verifies:** Inactive mode config isolation.
- **Given:** Config with `mode = "single_shot"`. Config file also has `[reverse_engineering.self_refine]` with `rounds = 5` and `[reverse_engineering.peer_review]` with `reviewers = 4`.
- **When:** `forgectl init --phase reverse_engineering --from input.json`
- **Then:** State file contains `mode: "single_shot"`. State file does not contain `self_refine` or `peer_review` config blocks.

### Subprocess failure outputs full error
- **Verifies:** Subprocess failure action output.
- **Given:** The Python subprocess exits with non-zero code and `execute.json` is unreadable.
- **When:** Forgectl reads the subprocess result.
- **Then:** Forgectl outputs the STOP message with the full stderr/stack trace from the subprocess.

### EXECUTE reads results from execute.json
- **Verifies:** Result consumption.
- **Given:** Subprocess has completed. `execute.json` has 2 entries with `status: "success"` and 1 with `status: "failure"`.
- **When:** Forgectl reads `execute.json` after subprocess exit.
- **Then:** Forgectl reports the failed entry. State stays in EXECUTE.

### EXECUTE advances on all success
- **Verifies:** Transition to RECONCILE.
- **Given:** Subprocess has completed. All entries in `execute.json` have `status: "success"`.
- **When:** Forgectl reads `execute.json` after subprocess exit.
- **Then:** State advances to RECONCILE.

### EXECUTE works from any working directory
- **Verifies:** CWD portability.
- **Given:** Forgectl is invoked from `/tmp/` (outside the project). Project root is `/project/`.
- **When:** EXECUTE generates `execute.json` and invokes the Python subprocess.
- **Then:** `project_root` in `execute.json` is the absolute path `/project/`. Domain roots resolve correctly.

### multi_pass flips action after first pass
- **Verifies:** Action override on subsequent passes.
- **Given:** `mode: "multi_pass"`, `passes: 2`. Queue entry with `action: "create"`.
- **When:** Pass 2 begins.
- **Then:** The entry's action is overridden to `"update"`.

### Fully specified codebase produces empty queue
- **Verifies:** Idempotency against already-specified behavior.
- **Given:** Every implemented behavior has a corresponding existing spec.
- **When:** GAP_ANALYSIS finds no gaps across all domains. User produces empty queue at QUEUE.
- **Then:** Queue JSON has zero entries.

### Cross-referencing detects asymmetric integration points
- **Verifies:** RECONCILE catches missing bidirectional references.
- **Given:** New spec A lists existing spec B in Integration Points, but spec B does not reference A.
- **When:** RECONCILE runs.
- **Then:** The asymmetry is identified and corrected.

### RECONCILE wires depends_on from queue
- **Verifies:** depends_on metadata used during reconciliation.
- **Given:** Queue entry for spec A has `depends_on: ["Spec B"]`. Both specs exist.
- **When:** RECONCILE runs.
- **Then:** Spec A's `Depends On` section references Spec B. Spec B's `Integration Points` references Spec A.

### RECONCILE round 1 lists spec files
- **Verifies:** Spec file listing on first round.
- **Given:** Domain "optimizer" has 3 specs in the queue.
- **When:** RECONCILE round 1 action output is displayed.
- **Then:** All 3 spec file paths are listed with their action and depends_on.

### RECONCILE subsequent round shows depends_on
- **Verifies:** depends_on shown on all rounds.
- **Given:** Domain "optimizer", round 2 after RECONCILE_EVAL FAIL.
- **When:** RECONCILE round 2 action output is displayed.
- **Then:** Spec file paths are listed with their depends_on.

### RECONCILE_EVAL outputs forgectl eval instruction
- **Verifies:** RECONCILE_EVAL action output references forgectl eval.
- **Given:** State is RECONCILE_EVAL, domain "optimizer", round 1.
- **When:** `forgectl status` is run.
- **Then:** Action output instructs user to tell sub-agents to run `forgectl eval`. Sub-agent config is displayed.

### forgectl eval outputs evaluator prompt
- **Verifies:** forgectl eval command during RECONCILE_EVAL.
- **Given:** State is RECONCILE_EVAL, domain "optimizer" with 3 specs.
- **When:** `forgectl eval` is run.
- **Then:** Outputs the reconcile-eval evaluator prompt populated with the 3 spec paths, depends_on, round number, and eval report path.

### forgectl eval blocked outside RECONCILE_EVAL
- **Verifies:** State-gating of forgectl eval.
- **Given:** State is RECONCILE (not RECONCILE_EVAL).
- **When:** `forgectl eval` is run.
- **Then:** Error: "forgectl eval is only available during RECONCILE_EVAL."

### RECONCILE_EVAL FAIL loops back to RECONCILE
- **Verifies:** FAIL loop behavior.
- **Given:** State is RECONCILE_EVAL, round 1, `max_rounds: 3`. Verdict is FAIL.
- **When:** `forgectl advance --verdict FAIL --eval-report report.md`
- **Then:** State returns to RECONCILE. Round increments to 2.

### RECONCILE_EVAL PASS before min_rounds loops back
- **Verifies:** Minimum rounds enforcement.
- **Given:** State is RECONCILE_EVAL, round 1, `min_rounds: 2`. Verdict is PASS.
- **When:** `forgectl advance --verdict PASS --eval-report report.md`
- **Then:** State returns to RECONCILE. Round increments to 2.

### RECONCILE_EVAL max rounds skips COLLEAGUE_REVIEW when disabled
- **Verifies:** Max rounds with colleague_review disabled.
- **Given:** State is RECONCILE_EVAL, round 3, `max_rounds: 3`, `colleague_review: false`. Verdict is FAIL.
- **When:** `forgectl advance --verdict FAIL --eval-report report.md`
- **Then:** State advances to RECONCILE_ADVANCE, skipping COLLEAGUE_REVIEW.

### RECONCILE_EVAL max rounds advances to COLLEAGUE_REVIEW when enabled
- **Verifies:** Max rounds with colleague_review enabled.
- **Given:** State is RECONCILE_EVAL, round 3, `max_rounds: 3`, `colleague_review: true`. Verdict is FAIL.
- **When:** `forgectl advance --verdict FAIL --eval-report report.md`
- **Then:** State advances to COLLEAGUE_REVIEW despite FAIL verdict.

### RECONCILE_EVAL PASS skips COLLEAGUE_REVIEW when disabled
- **Verifies:** Default flow skips colleague review.
- **Given:** State is RECONCILE_EVAL, round 1, `min_rounds: 1`, `colleague_review: false`. Verdict is PASS.
- **When:** `forgectl advance --verdict PASS --eval-report report.md`
- **Then:** State advances to RECONCILE_ADVANCE, skipping COLLEAGUE_REVIEW.

### RECONCILE_EVAL PASS advances to COLLEAGUE_REVIEW when enabled
- **Verifies:** Enabled colleague review gate.
- **Given:** State is RECONCILE_EVAL, round 1, `min_rounds: 1`, `colleague_review: true`. Verdict is PASS.
- **When:** `forgectl advance --verdict PASS --eval-report report.md`
- **Then:** State advances to COLLEAGUE_REVIEW.

### COLLEAGUE_REVIEW advances to RECONCILE_ADVANCE
- **Verifies:** COLLEAGUE_REVIEW transition.
- **Given:** State is COLLEAGUE_REVIEW, domain "optimizer".
- **When:** `forgectl advance`
- **Then:** State advances to RECONCILE_ADVANCE.

### RECONCILE_ADVANCE transitions to next domain
- **Verifies:** Domain transition.
- **Given:** State is RECONCILE_ADVANCE, domains ["optimizer", "api"]. Current domain is "optimizer".
- **When:** `forgectl advance`
- **Then:** State advances to RECONCILE for domain "api". Round resets to 1.

### RECONCILE_ADVANCE from last domain advances to DONE
- **Verifies:** Final domain transition.
- **Given:** State is RECONCILE_ADVANCE, domains ["optimizer", "api"]. Current domain is "api" (last).
- **When:** `forgectl advance`
- **Then:** State advances to DONE.

---

## Implements
- Reverse engineering workflow for deriving specifications from existing code
- State machine: ORIENT → (SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE per domain) → EXECUTE → (RECONCILE → RECONCILE_EVAL → optional COLLEAGUE_REVIEW → RECONCILE_ADVANCE per domain) → DONE
- Init input schema with concept and ordered domain list
- Reverse engineering queue JSON schema with domain-relative paths, single file accumulating across domains
- SURVEY and GAP_ANALYSIS action outputs with configurable sub-agent settings
- QUEUE advance logic: --file required on first advance, change detection and re-validation on subsequent advances
- QUEUE domain validation: entry domains must match initialized domain list
- QUEUE path validation: code_search_roots directories verified to exist on disk
- `forgectl add-domain` command: state-gated to QUEUE, appends a domain to the initialized list
- execute.json handoff: forgectl generates from queue + config (active mode only), Python subprocess reads and writes results back
- Prompt ownership split: forgectl embeds its own prompts (including reconcile-eval evaluator), Python package bundles reverse engineering prompts
- Four execution modes: single_shot, self_refine, multi_pass, peer_review (exactly one per session)
- Configurable sub-agents per purpose: drafter exploration, peer review, survey, gap analysis, reconciliation evaluation
- Per-domain reconciliation: RECONCILE → RECONCILE_EVAL (bounded loop) → optional COLLEAGUE_REVIEW → RECONCILE_ADVANCE
- COLLEAGUE_REVIEW: disabled by default, enabled via config, once per domain, human review gate
- `forgectl eval` command: state-gated to RECONCILE_EVAL, outputs embedded evaluator prompt with spec list and depends_on
- Reconciliation evaluation: 7-dimension checklist (completeness, depends_on validity, symmetry, correspondence, naming, no cycles, topic of concern)
- RECONCILE_ADVANCE: explicit domain transition state between reconciliation domains
- depends_on as RECONCILE metadata, ignored by EXECUTE
- CWD portability: all path resolution uses absolute paths or package resources
