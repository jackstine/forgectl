# Reverse Engineering Specifications from Code

## Topic of Concern
> The scaffold reverse-engineers specifications from an existing codebase by surveying existing specs, identifying unspecified behavior, deriving topics of concern, and producing new spec files.

## Context

When a codebase has implemented behavior that predates or was never captured in specifications, the system needs a structured workflow to extract those implicit contracts and make them explicit. This is the inverse of the normal spec-first flow: instead of code implementing specs, specs are derived from code.

The reverse engineering workflow takes a general idea of upcoming work as its starting point, uses that to scope which parts of the codebase are relevant, identifies gaps between existing specs and implemented behavior, and produces new spec files that close those gaps. The result is a spec corpus that accurately reflects what the code does, enabling future changes to proceed spec-first.

This workflow is distinct from writing specs from plans. Plans propose new behavior; reverse engineering captures existing behavior. The output format is identical — both produce spec files conforming to the standard spec format — but the input is code, not planning documents.

The user provides the domains and their order at init time. Forgectl loops SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE per domain before advancing to the execution loop. The user performs all the work; forgectl tracks state and tells the user what action to take next. Forgectl does not collect or store findings, and does not draft specifications itself — the user (the orchestrating agent) holds context between states and writes the spec files. Forgectl invokes no external subprocess; the entire workflow is driven from the Go CLI through action output.

A single reverse engineering queue JSON file accumulates entries across all domains. Each domain's QUEUE state adds that domain's entries to the file.

**Scope exclusions:**
- Writing specs from planning documents (covered by the standard specifying workflow).
- Modifying source code. Reverse engineering is read-only with respect to the codebase.

## Depends On
- **state-persistence** — provides the state file read/write mechanism for tracking reverse engineering workflow progress.
- **session-init** — populates the reverse engineering session during `init --phase reverse_engineering`.

## Integration Points

| Spec | Relationship |
|------|-------------|
| session-init | `init --phase reverse_engineering` creates the initial state with concept, domains, and configuration |
| state-persistence | State file tracks current state, domain index, queue file path, content hash, execute item index, reconcile round, colleague_review flag |
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

A JSON file listing every spec to be created or updated, written to a fixed convention path: `<project_root>/.forgectl/state/reverse-engineering-queue.json`. The path is not user-supplied; forgectl owns it and records it in the state file on the first QUEUE advance. All entry paths are relative to the domain root (`<project_root>/<domain>/`).

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
| `specs[].domain` | string | yes | Domain grouping. The entry's `file` and `code_search_roots` paths resolve against the domain root `<project_root>/<domain>/` |
| `specs[].topic` | string | yes | One-sentence topic of concern |
| `specs[].file` | string | yes | Target spec file path, relative to the domain root |
| `specs[].action` | string | yes | `"create"` or `"update"`. For updates, `file` is both the source and destination. |
| `specs[].code_search_roots` | string[] | yes | Directories forming the root of the core code for this spec's topic of concern, relative to the domain root; may not be empty. See definition below. |
| `specs[].depends_on` | string[] | yes | Names of specs this one depends on; may be empty array. Used by RECONCILE for cross-referencing. Ignored by the execution loop. |

**`code_search_roots` definition.** Each root is the directory that forms the *root of the core code* implementing this spec's topic of concern. Go as deep into the tree as needed: the root is the deepest directory that still contains **all** the files for that one capability. It is often a nested package several levels down (for example `net/http/internal/httpcommon/` — four levels deep — for shared HTTP request/response handling), not a broad top-level directory. A single topic of concern may span multiple such directories, which is why this is a list. Each root is searched recursively to its full depth — every file beneath it is in scope. Roots are directories, not single files, relative to the domain root.

No additional fields are permitted.

### Outputs

#### Action Output
At each state, forgectl outputs the current state, domain context (if applicable), and an action block telling the user what to do. See Behavior section for the action output per state.

#### New Spec Files
One spec file per identified topic of concern, written in the standard spec format by the orchestrating agent during the execution loop. Each spec captures the contracts, behaviors, invariants, edge cases, and testing criteria that the code currently implements.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| No concept provided | Error: "A concept is required to scope the reverse engineering effort." | Without scope, the system cannot determine which code to examine. |
| Empty domains list | Error: "At least one domain is required." | Nothing to process. |
| Duplicate domain in list | Error identifying the duplicate. | Each domain is processed once. |
| `code_search_roots` directory does not exist | Error identifying the invalid path. Validated at QUEUE. | Cannot reverse-engineer from a directory that does not exist. |

---

## State Machine

The reverse engineering workflow follows this state machine. SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE loop per domain in the order provided at init. A single queue JSON file accumulates entries across all domains. After QUEUE, EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER loop once per queued item across all domains. RECONCILE → RECONCILE_EVAL → (optional COLLEAGUE_REVIEW) → RECONCILE_ADVANCE loop per domain after the execution loop.

```
ORIENT
  ↓
SURVEY (domain 1) → GAP_ANALYSIS (domain 1) → DECOMPOSE (domain 1) → QUEUE (domain 1)
  ↓
SURVEY (domain 2) → GAP_ANALYSIS (domain 2) → DECOMPOSE (domain 2) → QUEUE (domain 2)
  ↓
  ... (repeat for each domain)
  ↓
EXECUTE_REVERSE_ENGINEER (item 1) → POST_REVERSE_ENGINEER (item 1)
  ↓
EXECUTE_REVERSE_ENGINEER (item 2) → POST_REVERSE_ENGINEER (item 2)
  ↓
  ... (repeat for each queued item; POST loops back to EXECUTE_REVERSE_ENGINEER)
  ↓ (all items done)
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

The state file tracks: current state, current domain index (used for both the SURVEY-QUEUE loop and the RECONCILE loop), total domain count, execute item index (used for the EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER loop), reconcile round, queue file path (set on first QUEUE advance to the fixed convention path), queue file content hash (for change detection on subsequent QUEUE advances), colleague_review enabled flag.

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

  Please spawn {survey.count} {survey.model} {survey.type} sub-agent(s)
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

  Please spawn {gap_analysis.count} {gap_analysis.model} {gap_analysis.type}
  sub-agent(s) scoped to the {domain} source code.

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
    - Code search roots:
        - The directory (or directories) forming the root of the core
          code implementing this topic of concern.
        - Go as deep as needed: the root is the deepest directory that
          still contains ALL the files for that one capability — often a
          nested package several levels down (e.g. net/http/internal/
          httpcommon/), not a broad top-level directory.
        - A topic may span multiple such directories; list each one.
        - Directories, not single files; relative to the domain root.
        - Must be non-empty for every spec.
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

The queue file lives at a fixed convention path owned by forgectl: `<project_root>/.forgectl/state/reverse-engineering-queue.json`. The user never supplies the path; `forgectl advance` takes no `--file` flag in the QUEUE state.

#### First Advance (domain 1 — content hash not yet set)

The user writes the queue file at the convention path. On the first advance, forgectl reads it, records the content hash in the state file, and validates.

```
Phase: reverse_engineering
State: QUEUE
Domain: {domain} ({index}/{N})
Concept: "{concept}"
Queue file: .forgectl/state/reverse-engineering-queue.json

Action:
  Write the reverse engineering queue file with entries for domain {domain}
  at: .forgectl/state/reverse-engineering-queue.json

  Requirements:
    - All paths relative to domain root (<project_root>/{domain}/)
    - Order entries by dependency: specs with no dependencies first
    - code_search_roots must be non-empty for every entry
    - No circular dependencies

  Advance when the file is written:
    forgectl advance
```

#### Subsequent Advances (domains 2+ — content hash already set)

The user updates the existing queue file by adding entries for the current domain. Forgectl re-reads from the convention path, checks for changes, and validates.

```
Phase: reverse_engineering
State: QUEUE
Domain: {domain} ({index}/{N})
Concept: "{concept}"
Queue file: .forgectl/state/reverse-engineering-queue.json

Action:
  Add entries for domain {domain} to the existing queue file.

  Update the queue file at: .forgectl/state/reverse-engineering-queue.json
  Add new entries for this domain alongside existing entries.

  Advance when the file is updated:
    forgectl advance
```

#### Advance Behavior

`forgectl advance` never accepts a `--file` flag in QUEUE; the path is fixed.

| Advance | Behavior |
|---------|----------|
| First (no stored hash) | Read the file at the convention path, compute and store content hash, validate schema, validate `code_search_roots` directories exist. If valid → advance. If invalid → error with violations. |
| Subsequent (hash stored) | Re-read the file from the convention path. Compare content hash. If unchanged → error: "Queue file has not changed. Update the file and retry." If changed → recompute hash, validate schema, validate `code_search_roots` directories exist. If valid → advance. If invalid → error with violations. |

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
- Queue JSON file exists at the convention path with entries for all processed domains.
- All entries pass schema validation.
- All `code_search_roots` directories exist on disk.
- State advances to SURVEY for the next domain, or EXECUTE_REVERSE_ENGINEER (item 1) if all domains are complete.

#### Error Handling
- `--file` flag supplied: error: "forgectl advance takes no --file flag in QUEUE. The queue file is fixed at .forgectl/state/reverse-engineering-queue.json."
- Queue file not found at the convention path: error with the expected path.
- Schema validation failure: error listing violations. User corrects and retries `forgectl advance`.
- `code_search_roots` directory does not exist: error identifying the entry and the missing path.
- Content unchanged (subsequent advance): error: "Queue file has not changed. Update the file and retry."

---

### EXECUTE_REVERSE_ENGINEER

EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER form a loop that runs once per queued item, in queue order, across all domains. Forgectl tracks the current item via the execute item index in the state file. This state writes the actual specification for the current item — there is no draft, no evaluation, and no self-review.

#### Preconditions
- The reverse engineering queue JSON file exists and is validated.
- All `code_search_roots` directories were verified at QUEUE time.
- An empty queue is rejected on entry to the loop (see Error Handling), so every item processed corresponds to a validated queue entry.

#### Item Setup
Before emitting the action output for item `i`, forgectl ensures the target domain's `specs/` directory exists, creating `<project_root>/<domain>/specs/` if absent.

#### Action Output

```
Phase: reverse_engineering
State: EXECUTE_REVERSE_ENGINEER
Item: {i}/{M}
Domain: {domain}
Spec: {name}  ({action})
Target file: {domain}/{file}
Topic of concern: "{topic}"

code_search_roots:
  - {domain}/{root_1}
  - {domain}/{root_2}

  code_search_roots are the directories forming the root of the core
  code that implements this spec's topic of concern. A topic of concern
  may span multiple directories within a module — every listed root, and
  every file recursively beneath it, is in scope. These are directories,
  not single files, relative to the domain root. Read the core code in
  full before writing.

Action:
  {action == create: Create | action == update: Update} the specification
  at {domain}/{file} from the code under the search roots above.

  Please spawn {execute.count} {execute.model} {execute.type} sub-agent(s)
  scoped to the search roots to examine the implementation.

  Write the specification in the standard spec format, capturing the
  contracts, behaviors, invariants, edge cases, and testing criteria
  that the code currently implements for this topic of concern.
  {if action == update: The file already exists — preserve content that
   is still correct and revise the rest to match the current implementation.}

  Advance when the specification file is written:
    forgectl advance
```

#### Postconditions
- The orchestrating agent has written (or, for `update`, revised) the spec file at `<domain>/<file>` in the standard spec format, or — if it could not — left no file. Forgectl does not verify the file's existence.
- State advances to POST_REVERSE_ENGINEER for the same item.

#### Error Handling
- Queue contains zero entries: error: "Queue contains zero entries. Nothing to execute." State stays at EXECUTE_REVERSE_ENGINEER (no item to process). This condition is detected when the loop is entered after QUEUE.
- Domain `specs/` directory creation fails (permissions, disk): error with the path.
- If the agent fails to write the spec file for this item, forgectl takes no action: the item is treated as if no file was ever created. The loop proceeds normally via POST_REVERSE_ENGINEER. There is no failure state.

---

### POST_REVERSE_ENGINEER

POST_REVERSE_ENGINEER closes each execution-loop iteration. It exists to checkpoint the just-written spec and to prompt a context-window reset before the next item, because each iteration reads a whole topic's worth of core code.

#### Preconditions
- EXECUTE_REVERSE_ENGINEER for the current item has completed.

#### Action Output

```
Phase: reverse_engineering
State: POST_REVERSE_ENGINEER
Item: {i}/{M}
Spec: {name}
Target file: {domain}/{file}

STOP ensure you have created the specification that you need,
please tell your user to clear your context window for the next iteration.

Advance to {continue with item {i+1}/{M} | proceed to RECONCILE}:
  forgectl advance
```

#### Postconditions
- If more queued items remain: state advances to EXECUTE_REVERSE_ENGINEER, execute item index increments to the next item.
- If this was the last item: state advances to RECONCILE for domain 1, round 1.

#### Error Handling
- None. This is a checkpoint and transition state.

---

### RECONCILE

RECONCILE runs per domain, looping with RECONCILE_EVAL until PASS or max rounds.

#### Preconditions
- The execution loop is complete (for domain 1) or RECONCILE_ADVANCE from previous domain.
- The execution loop has run for every queued item. Spec files exist for all items the agent successfully wrote; items it skipped have no file (see RECONCILE error handling for missing files).

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

  Please spawn {reconcile.eval.count} {reconcile.eval.model} {reconcile.eval.type}
  sub-agent(s) to evaluate the reconciliation.

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

### Full Configuration Reference

```toml
[reverse_engineering]

# Sub-agents the orchestrating agent spawns during EXECUTE_REVERSE_ENGINEER
# to examine the core code and write each spec (action output only)
[reverse_engineering.execute]
model = "haiku"
type = "explorer"
count = 3

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
| `execute` | Forgectl action output | EXECUTE_REVERSE_ENGINEER | Displayed to the agent — what sub-agents to spawn to examine code and write each spec |
| `reconcile` | Forgectl | RECONCILE_EVAL | Min/max rounds for reconciliation eval loop |
| `reconcile.colleague_review` | Forgectl | After RECONCILE_EVAL | Whether COLLEAGUE_REVIEW gate is enabled (default: false) |
| `reconcile.eval` | Forgectl action output | RECONCILE_EVAL | Sub-agents for reconciliation evaluation |
| `survey` | Forgectl action output | SURVEY | Displayed to user — what sub-agents to spawn |
| `gap_analysis` | Forgectl action output | GAP_ANALYSIS | Displayed to user — what sub-agents to spawn |

All configuration blocks are consumed by forgectl, either to drive the state machine (`reconcile`) or to populate action output (`execute`, `reconcile.eval`, `survey`, `gap_analysis`). The orchestrating agent reads the action output and performs the work; forgectl invokes no subprocess and constructs no agent sessions itself.

---

## Observability

### Logging

| Level | What is logged |
|-------|---------------|
| INFO | Workflow started with concept; domain processing started (domain name, index); SURVEY complete for domain; GAP_ANALYSIS complete for domain; QUEUE file produced (spec count); execution loop started (item count); EXECUTE_REVERSE_ENGINEER entered for an item (item index, spec name); POST_REVERSE_ENGINEER entered for an item; RECONCILE complete for domain; RECONCILE_EVAL verdict (PASS/FAIL, round); COLLEAGUE_REVIEW entered (when enabled); RECONCILE_ADVANCE domain transition; workflow DONE |
| WARN | Domain has no `specs/` directory during SURVEY; empty source directory during GAP_ANALYSIS |
| ERROR | Queue JSON validation failure; `code_search_roots` directory not found; circular dependency detected; queue contains zero entries at execution-loop entry; missing spec file during RECONCILE verification |
| DEBUG | Domain index progression; execute item index progression; sub-agent config applied; queue entry details |

---

## Invariants

1. **Read-only codebase.** The reverse engineering workflow never modifies source code. It reads code to produce specs.
2. **One topic per spec.** Every spec produced passes the topic-of-concern test. No spec covers multiple unrelated responsibilities.
3. **Spec format compliance.** Every spec produced conforms to the standard spec format, regardless of whether it was written from a plan or reverse-engineered from code.
4. **Dependency ordering.** The spec queue is ordered such that no spec appears before a spec it depends on.
5. **Domain-root scoping.** All paths in the queue (`file`, `code_search_roots`) are relative to `<project_root>/<domain>/` and resolve against that domain root.
6. **Single-file write per item.** Each EXECUTE_REVERSE_ENGINEER item writes or edits exactly one file — the `file` specified in its queue entry. No other files are modified.
7. **Spec directory pre-exists.** The domain's `specs/` directory exists before an item's EXECUTE_REVERSE_ENGINEER action output is emitted. Forgectl creates it if absent.
8. **Sequential domain processing.** Domains are processed in the order provided at init. SURVEY, GAP_ANALYSIS, DECOMPOSE, and QUEUE complete for domain N before domain N+1 begins.
9. **Single queue file at a fixed path.** One reverse engineering queue JSON file at `<project_root>/.forgectl/state/reverse-engineering-queue.json` accumulates entries across all domains. The path is a convention owned by forgectl, not user-supplied.
10. **Queue file change detection.** On subsequent QUEUE advances, forgectl rejects unchanged files. The user must modify the file before advancing.
11. **Forgectl drives everything; no subprocess.** The entire workflow runs from the Go CLI. Forgectl invokes no external subprocess and constructs no agent sessions. Every unit of work is performed by the orchestrating agent in response to action output.
12. **Path validation at QUEUE.** All `code_search_roots` directories are verified to exist on disk during QUEUE validation. Invalid paths are rejected before the execution loop.
13. **`depends_on` is RECONCILE metadata.** The `depends_on` field in queue entries is used by RECONCILE to wire up cross-references. It is ignored by the execution loop.
14. **Per-item execution loop.** EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER runs exactly once per queued item, in queue order, across all domains. The execute item index advances by one per iteration.
15. **Direct write, no draft or evaluation.** EXECUTE_REVERSE_ENGINEER writes the final spec for an item in a single pass. There is no draft, self-review, or evaluation step within the loop.
16. **Failed items are skipped silently.** If an item produces no spec file, forgectl does not error or enter a failure state. The loop continues to the next item.
17. **Per-domain reconciliation.** RECONCILE, RECONCILE_EVAL, and (if enabled) COLLEAGUE_REVIEW run per domain in the same order as the SURVEY-QUEUE loop.
18. **Reconcile eval bounded.** RECONCILE_EVAL loops at most `max_rounds` times per domain. At max rounds, the workflow advances to COLLEAGUE_REVIEW (if enabled) or RECONCILE_ADVANCE (if disabled) regardless of verdict.
19. **Colleague review is optional.** COLLEAGUE_REVIEW is disabled by default. When disabled, the state is skipped entirely — RECONCILE_EVAL advances directly to RECONCILE_ADVANCE. When enabled, it runs exactly once per domain.
20. **`forgectl eval` is state-gated.** The `forgectl eval` command is only active during RECONCILE_EVAL. It outputs the embedded evaluator prompt populated with the current domain's spec list.
21. **Queue entries match initialized domains.** Every entry's `domain` field in the queue must match a domain in the initialized domain list. Unrecognized domains are rejected at QUEUE validation.
22. **`forgectl add-domain` is state-gated.** The `forgectl add-domain` command is only available during the QUEUE state.

---

## Edge Cases

- **Scenario:** The entire codebase is already fully specified.
  - **Expected behavior:** GAP_ANALYSIS across all domains finds no unspecified behavior. DECOMPOSE produces an empty spec list. QUEUE produces a JSON with zero entries. When the execution loop is entered with an empty queue, forgectl errors: "Queue contains zero entries. Nothing to execute." State stays at EXECUTE_REVERSE_ENGINEER.
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

- **Scenario:** A single domain appears in the queue but the user provided three domains at init.
  - **Expected behavior:** SURVEY, GAP_ANALYSIS, DECOMPOSE, and QUEUE still run for all three domains. The queue produced at QUEUE may only contain specs for the one domain where gaps were found.
  - **Rationale:** All domains are surveyed and analyzed regardless of whether gaps are found. The queue reflects only actionable work.

- **Scenario:** `code_search_roots` directory exists at QUEUE time but is deleted before its EXECUTE_REVERSE_ENGINEER item runs.
  - **Expected behavior:** The agent cannot read the missing directory and does not write the spec file. The item is treated as if no file was ever created. The loop proceeds to POST_REVERSE_ENGINEER and on to the next item. Forgectl does not error.
  - **Rationale:** QUEUE validates what it can. With failures ignored, a runtime-missing directory simply yields no spec for that item rather than halting the loop.

- **Scenario:** The agent cannot write the spec for a queued item (irrelevant code, read failure, or any other reason).
  - **Expected behavior:** No file is written for that item. POST_REVERSE_ENGINEER still advances. The item is silently skipped; no failure state is entered.
  - **Rationale:** For now, a missing output is acceptable — the loop never blocks on a single item. RECONCILE verification will surface any spec the queue expected but that does not exist.

---

## Testing Criteria

### Init validates domains
- **Verifies:** Init input validation.
- **Given:** Input JSON with `domains: ["api", "api"]` (duplicate).
- **When:** `forgectl init --phase reverse_engineering --from input.json`
- **Then:** Error identifying the duplicate. Init fails.

### ORIENT displays domain order
- **Verifies:** ORIENT action output.
- **Given:** Init with `domains: ["optimizer", "api", "portal"]`.
- **When:** State is ORIENT, `forgectl status` is run.
- **Then:** Output lists all three domains in order with indices.

### SURVEY action uses configured sub-agents
- **Verifies:** SURVEY action output reflects config.
- **Given:** Config with `survey.count = 4`, `survey.model = "sonnet"`, `survey.type = "explorer"`.
- **When:** State is SURVEY, `forgectl status` is run.
- **Then:** Action output says "Please spawn 4 sonnet explorer sub-agents".

### GAP_ANALYSIS action uses configured sub-agents
- **Verifies:** GAP_ANALYSIS action output reflects config.
- **Given:** Config with `gap_analysis.count = 3`, `gap_analysis.model = "opus"`, `gap_analysis.type = "explorer"`.
- **When:** State is GAP_ANALYSIS, `forgectl status` is run.
- **Then:** Action output says "Please spawn 3 opus explorer sub-agents".

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

### Last domain advances to the execution loop
- **Verifies:** Transition from last domain to EXECUTE_REVERSE_ENGINEER.
- **Given:** Domains: ["optimizer", "api"]. State: QUEUE, domain index 2. Queue file validated with at least one entry.
- **When:** User advances.
- **Then:** State becomes EXECUTE_REVERSE_ENGINEER, execute item index 1.

### QUEUE advance rejects --file flag
- **Verifies:** The queue path is fixed, not user-supplied.
- **Given:** State is QUEUE. The queue file lives at the convention path.
- **When:** `forgectl advance --file some-other-queue.json`
- **Then:** Error: "forgectl advance takes no --file flag in QUEUE. The queue file is fixed at .forgectl/state/reverse-engineering-queue.json."

### QUEUE validates reverse engineering queue schema
- **Verifies:** Queue JSON validation.
- **Given:** Queue file at the convention path with an entry missing `code_search_roots`.
- **When:** `forgectl advance`
- **Then:** Error listing the validation violation.

### QUEUE rejects entries with unrecognized domains
- **Verifies:** Domain validation at QUEUE.
- **Given:** Init with `domains: ["optimizer", "api"]`. Queue file contains an entry with `domain: "portal"`.
- **When:** `forgectl advance`
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

### Execution loop rejects empty queue
- **Verifies:** Empty queue error.
- **Given:** Queue JSON has zero entries.
- **When:** The execution loop is entered after QUEUE.
- **Then:** Error: "Queue contains zero entries. Nothing to execute." State stays at EXECUTE_REVERSE_ENGINEER.

### QUEUE validates code_search_roots exist on disk
- **Verifies:** Path validation at QUEUE.
- **Given:** Queue file with `code_search_roots: ["src/nonexistent/"]`. Directory does not exist.
- **When:** `forgectl advance`
- **Then:** Error identifying the entry and the missing directory.

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

### EXECUTE_REVERSE_ENGINEER creates specs directory before item action
- **Verifies:** Spec directory pre-creation.
- **Given:** The current item's domain is "optimizer". `optimizer/specs/` does not exist.
- **When:** EXECUTE_REVERSE_ENGINEER is entered for that item.
- **Then:** `optimizer/specs/` is created before the action output is emitted.

### EXECUTE_REVERSE_ENGINEER action reflects the current item and config
- **Verifies:** Per-item action output.
- **Given:** Config with `execute.count = 3`, `execute.model = "haiku"`, `execute.type = "explorer"`. Current item is spec "Repository Loading" (domain "optimizer", action "create", file "specs/repository-loading.md", roots ["src/repo/"]).
- **When:** State is EXECUTE_REVERSE_ENGINEER, `forgectl status` is run.
- **Then:** Action output shows the item index, domain, spec name, action, target file, topic, and the `code_search_roots` with the root-of-core-code definition. It says "Please spawn 3 haiku explorer sub-agents".

### EXECUTE_REVERSE_ENGINEER advances to POST_REVERSE_ENGINEER
- **Verifies:** Per-item transition into the checkpoint state.
- **Given:** State is EXECUTE_REVERSE_ENGINEER for item 1 of 3.
- **When:** `forgectl advance`
- **Then:** State becomes POST_REVERSE_ENGINEER for item 1.

### POST_REVERSE_ENGINEER outputs the STOP / clear-context message
- **Verifies:** POST_REVERSE_ENGINEER action output.
- **Given:** State is POST_REVERSE_ENGINEER for item 1.
- **When:** `forgectl status` is run.
- **Then:** Output contains "STOP ensure you have created the specification that you need, please tell your user to clear your context window for the next iteration."

### POST_REVERSE_ENGINEER loops to the next item
- **Verifies:** Loop continuation.
- **Given:** State is POST_REVERSE_ENGINEER, execute item index 1 of 3.
- **When:** `forgectl advance`
- **Then:** State becomes EXECUTE_REVERSE_ENGINEER, execute item index 2.

### POST_REVERSE_ENGINEER on last item advances to RECONCILE
- **Verifies:** Loop exit.
- **Given:** State is POST_REVERSE_ENGINEER, execute item index 3 of 3.
- **When:** `forgectl advance`
- **Then:** State advances to RECONCILE for domain 1, round 1.

### Failed item is skipped without error
- **Verifies:** Failures are ignored.
- **Given:** State is EXECUTE_REVERSE_ENGINEER for an item; the agent wrote no spec file (e.g. roots unreadable).
- **When:** `forgectl advance`
- **Then:** State advances to POST_REVERSE_ENGINEER normally. No error is raised; no failure state is entered.

### Execution loop works from any working directory
- **Verifies:** CWD portability.
- **Given:** Forgectl is invoked from `/tmp/` (outside the project). Project root is `/project/`.
- **When:** EXECUTE_REVERSE_ENGINEER emits an item's action output.
- **Then:** The domain root and target file paths resolve against the absolute project root `/project/`, regardless of the current working directory.

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
- State machine: ORIENT → (SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE per domain) → (EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER per queued item) → (RECONCILE → RECONCILE_EVAL → optional COLLEAGUE_REVIEW → RECONCILE_ADVANCE per domain) → DONE
- Init input schema with concept and ordered domain list
- Reverse engineering queue JSON schema with domain-relative paths, single file at the fixed convention path `.forgectl/state/reverse-engineering-queue.json` accumulating across domains
- code_search_roots defined as the root(s) of the core code for a topic of concern — deepest encompassing directory, possibly several, searched recursively
- SURVEY and GAP_ANALYSIS action outputs with configurable sub-agent settings
- QUEUE advance logic: fixed convention path (no --file flag), change detection and re-validation on subsequent advances
- QUEUE domain validation: entry domains must match initialized domain list
- QUEUE path validation: code_search_roots directories verified to exist on disk
- `forgectl add-domain` command: state-gated to QUEUE, appends a domain to the initialized list
- Per-item execution loop driven entirely from the Go CLI: forgectl emits per-item action output, the orchestrating agent writes the spec, no subprocess and no execute.json
- EXECUTE_REVERSE_ENGINEER: writes the final spec for one queued item (no draft, no evaluation), with a configurable `execute` sub-agent block
- POST_REVERSE_ENGINEER: per-item checkpoint that emits the STOP / clear-context message and loops to the next item or to RECONCILE
- Failed items skipped silently — no failure state
- Per-domain reconciliation: RECONCILE → RECONCILE_EVAL (bounded loop) → optional COLLEAGUE_REVIEW → RECONCILE_ADVANCE
- COLLEAGUE_REVIEW: disabled by default, enabled via config, once per domain, human review gate
- `forgectl eval` command: state-gated to RECONCILE_EVAL, outputs embedded evaluator prompt with spec list and depends_on
- Reconciliation evaluation: 7-dimension checklist (completeness, depends_on validity, symmetry, correspondence, naming, no cycles, topic of concern)
- RECONCILE_ADVANCE: explicit domain transition state between reconciliation domains
- depends_on as RECONCILE metadata, ignored by the execution loop
- CWD portability: all path resolution uses absolute paths
