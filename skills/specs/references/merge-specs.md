# Merge Specs: Decompose, Specify, and Reconcile

A systematic methodology for breaking a monolithic specification into single-concern spec documents and ensuring cross-cutting consistency.

---

## Phase 1: Decomposition

### Objective
Identify every distinct topic of concern in the source document.

### Topic of Concern Test
Apply the topic of concern test to each candidate. See [../../shared/topic-of-concern.md](../../shared/topic-of-concern.md) for the full scoping rules and examples.

### Process
1. Read the source document in full.
2. List every behavior, capability, or responsibility described.
3. Apply the topic test to each. Split any that fail.
4. Number the topics in dependency order (producers before consumers).
5. Present the topic list to the stakeholder for review before proceeding.

### Output
A numbered list of topics, each with a one-sentence description.

---

## Phase 2: Spec Generation

### Objective
Produce one specification document per topic of concern.

### Per-Topic Spec Requirements
Each spec must follow the structure defined in [../../shared/spec-format.md](../../shared/spec-format.md). The format defines all required and optional sections (Interface, Behavior, Configuration, Observability, Invariants, Edge Cases, Testing Criteria, etc.).

### Execution
- Spawn one subagent per topic, all in parallel.
- Each subagent receives:
  - The full source document (for context)
  - Its assigned topic of concern
  - Instructions to generate a spec scoped strictly to that topic
- Each subagent writes its spec to a numbered file (e.g., `01-topic-name.md`).

### Output
One spec file per topic. Commit after all are written.

---

## Phase 3: Cross-Cutting Review

### Objective
Find issues introduced by splitting a unified document into separate specs.

### Issue Categories

| Category | Definition |
|----------|------------|
| **Schema Conflict** | Two specs define the same data structure differently |
| **Missing Handoff** | Spec A produces output that Spec B consumes, but the interface is inconsistent |
| **Duplicated Responsibility** | Two specs claim ownership of the same behavior |
| **Gap** | A necessary behavior that no spec covers, or that falls between specs |
| **Inconsistent Configuration** | Config keys, defaults, or structures that conflict across specs |
| **Undefined Dependency** | Spec A references something from Spec B that Spec B does not define |

### Execution
- Spawn a single subagent that reads every spec file plus the original source document.
- The subagent produces a structured report with:
  - Issue ID (e.g., ISS-001)
  - Category (from the table above)
  - Specs involved (by number)
  - Description
  - Suggested resolution

### Output
An issue manifest. Do not fix anything yet.

---

## Phase 4: Remediation Planning

### Objective
Classify every issue into the correct fix strategy before making any changes.

### Fix Strategies

**Wave 1 — Create missing documents**
Issues categorized as **Gap** that require net-new specs or shared reference documents.
- New integration specs (e.g., a top-level orchestration spec that ties phases together)
- New shared convention docs (identity schemes, timestamp formats, retry policies, config namespace rules)
- New specs for behaviors that fell between existing specs

**Wave 2 — Fix existing documents**
Issues categorized as **Schema Conflict**, **Missing Handoff**, **Duplicated Responsibility**, **Inconsistent Configuration**, or **Undefined Dependency**.
- Each existing spec that needs changes gets a precise issue list
- Changes are scoped to: remove duplicated sections, align schemas, add cross-references, correct defaults

### Dependency Rule
Wave 2 depends on Wave 1. The new documents from Wave 1 are what Wave 2's fixes will reference. Wave 1 items may run in parallel with each other.

### Output
A remediation plan mapping each issue ID to a wave, a target file, and a description of the change.

---

## Phase 5: Wave 1 Execution

### Objective
Create all missing specs and shared reference documents.

### Execution
- Spawn one subagent per new document, all in parallel.
- Each subagent reads all existing specs for full context.
- Each subagent writes its assigned document.

### Output
New spec files. **Commit.**

---

## Phase 6: Wave 2 Execution

### Objective
Fix all existing specs to resolve the remaining issues.

### Execution
- Spawn one subagent per spec file that needs edits, all in parallel.
- Each subagent receives:
  - Its assigned spec file
  - The new documents from Wave 1 (shared conventions, integration specs)
  - The exact list of issues to fix, with descriptions
  - Instruction: **fix only the listed issues; do not add, expand, or restructure beyond what is required**

### Output
Edited spec files. **Commit.**

---

## Phase 7: Review

### Objective
Verify that Wave 2 edits are correct, complete, and did not introduce drift.

### Drift Defined
Drift is any change that is not traceable to a listed issue. Examples:
- A new section that wasn't requested
- An expanded schema with fields not in any issue
- A removed section that wasn't supposed to be removed
- A changed default value not listed in any issue
- A new responsibility claimed by the spec

### Execution
- Obtain the diff of all changes from Wave 2 (e.g., `git diff` against the Wave 1 commit).
- Spawn a single reviewer subagent that receives:
  - The diff (not the full files — just the changes)
  - The issue manifest from Phase 3
  - The remediation plan from Phase 4
- The reviewer performs two checks:
  1. **Per-file check**: Every diff hunk must trace to a listed issue. Flag anything extra.
  2. **Cross-file check**: References between specs are consistent (if Spec A says "see Spec B's schema," Spec B defines that schema with matching field names).
- The reviewer produces a report of findings.

### Resolution
- If findings are clean: done.
- If findings contain drift: the orchestrator (not a subagent) makes targeted fixes based on the reviewer's report. This keeps the fix scope minimal and avoids another round of potential drift.

### Output
Review report. Fixes if needed. **Commit.**

---

## Principles

1. **One concern per spec.** If you need "and" to describe it, split it. See [../../shared/topic-of-concern.md](../../shared/topic-of-concern.md).
2. **Ownership is singular.** Every behavior, schema, and config key has exactly one spec that owns it. All other specs reference, never redefine.
3. **Produce before consuming.** Number specs so that producers come before consumers. Fix producers before consumers.
4. **Commit between phases.** Every phase boundary is a commit. This gives you rollback points and clean diffs for review.
5. **Subagents create, the orchestrator corrects.** Parallel subagents are good for generation (creative, independent work). The orchestrator handles corrections (precise, cross-cutting work).
6. **Review against a manifest, not vibes.** The reviewer checks changes against a concrete list of issues, not subjective quality judgment.
7. **Drift is the primary risk.** When a subagent is told to fix X, it may also "improve" Y. The entire review phase exists to catch this.
