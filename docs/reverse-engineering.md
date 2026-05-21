# Reverse Engineering Guide

How to use forgectl's reverse engineering phase to derive specifications from existing code.

## Overview

The reverse engineering workflow extracts implicit contracts from code and makes them explicit as spec files. It is the inverse of the normal spec-first flow: instead of code implementing specs, specs are derived from code.

**When to use:** When a codebase has implemented behavior that was never captured in specifications, and you need specs before making changes.

## Prerequisites

- forgectl installed and `.forgectl/` directory exists in the project root
- `.forgectl/config` contains a `[reverse_engineering]` section (or defaults are acceptable)
- You have a general concept of the work you will be performing
- You know which domains in the codebase are relevant to the work

## Quick Start

### 1. Create the init input file

```json
{
  "concept": "auth middleware refactor",
  "domains": ["optimizer", "api", "portal"]
}
```

- `concept`: A description of the work. This scopes everything — only code and specs relevant to this concept are examined.
- `domains`: Ordered list of domains to process. Each domain goes through SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE before the next domain starts. Order matters — put the most foundational domain first.

### 2. Initialize the session

```bash
forgectl init --phase reverse_engineering --from input.json
```

### 3. Follow the state machine

Run `forgectl status` at any point to see the current state and what action is needed. Run `forgectl advance` to move to the next state.

## State Machine

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
RECONCILE (domain 1) → RECONCILE_EVAL (domain 1) → [COLLEAGUE_REVIEW (domain 1)] → RECONCILE_ADVANCE
  ↓
RECONCILE (domain 2) → RECONCILE_EVAL (domain 2) → [COLLEAGUE_REVIEW (domain 2)] → RECONCILE_ADVANCE
  ↓
  ... (repeat for each domain)
  ↓
DONE
```

Note: RECONCILE_EVAL can loop back to RECONCILE on FAIL (up to `max_rounds`). COLLEAGUE_REVIEW (in brackets) is disabled by default — when disabled, RECONCILE_EVAL advances directly to RECONCILE_ADVANCE.

## States in Detail

### ORIENT

**What happens:** Forgectl displays the concept, domain list, and order. You confirm readiness.

**What you do:** Review the domain order. Advance when ready.

### SURVEY

**What happens:** Forgectl tells you to survey existing specs in the current domain's `specs/` directory.

**What you do:**
1. Spawn the configured sub-agents (default: 2 haiku explorers) scoped to `{domain}/specs/`.
2. Read all spec files. For each, extract the topic of concern, behaviors, integration points, and dependencies.
3. Identify which specs pertain to your concept.
4. Advance when complete.

### GAP_ANALYSIS

**What happens:** Forgectl tells you to examine the current domain's source code for unspecified behavior.

**What you do:**
1. Spawn the configured sub-agents (default: 5 sonnet explorers) scoped to the domain source code.
2. For each behavior found in code that is not covered by an existing spec:
   - Describe what it does
   - Formulate a topic of concern (single sentence, no "and", describes an activity)
   - Note the code location
   - Note if an existing spec partially covers it
3. Advance when complete.

### DECOMPOSE

**What happens:** Forgectl tells you to synthesize your SURVEY and GAP_ANALYSIS findings for this domain.

**What you do:**
1. Decide which gaps warrant new specs vs. updates to existing specs.
2. Group related behaviors into single-topic specs.
3. For each spec, define: name, topic of concern, file path, action (create/update), code search roots, dependencies.
4. Advance when the spec list for this domain is finalized.

### QUEUE

**What happens:** You write the queue JSON file (or add to it for subsequent domains).

**What you do:**
- **First domain:** Write the queue JSON file and advance with: `forgectl advance --file queue.json`
- **Subsequent domains:** Add entries to the existing file and advance with: `forgectl advance`

Forgectl validates the JSON schema and verifies that all `code_search_roots` directories exist on disk.

#### Queue JSON schema

```json
{
  "specs": [
    {
      "name": "Auth Middleware Validation",
      "domain": "optimizer",
      "topic": "The optimizer validates authentication tokens before processing requests",
      "file": "specs/auth-middleware-validation.md",
      "action": "create",
      "code_search_roots": ["src/middleware/", "src/auth/"],
      "depends_on": []
    }
  ]
}
```

All paths are relative to the domain root (`<project_root>/<domain>/`).

| Field | Description |
|-------|-------------|
| `name` | Display name for the spec |
| `domain` | Which domain this spec belongs to |
| `topic` | One-sentence topic of concern |
| `file` | Spec file path relative to domain root (e.g., `specs/my-spec.md`) |
| `action` | `"create"` for new specs, `"update"` for existing specs with gaps |
| `code_search_roots` | Directories the agent examines (relative to domain root) |
| `depends_on` | Names of specs this one depends on. Used by RECONCILE for cross-referencing. |

### EXECUTE

**What happens:** Forgectl generates `execute.json`, invokes the Python subprocess, and parallel agents draft/update spec files.

**What you do:** Wait. Forgectl handles this automatically.

The Python subprocess runs one Claude Agent SDK session per spec entry. The execution mode (configured in `.forgectl/config`) determines how agents refine their work:

| Mode | What happens |
|------|-------------|
| `single_shot` | Agent drafts once. Done. |
| `self_refine` | Agent drafts, then reviews its own work N times. |
| `multi_pass` | Full batch of agents runs N times. Creates become updates after pass 1. |
| `peer_review` | Agent drafts, then spawns reviewer sub-agents to evaluate. N rounds. |

After the subprocess completes, forgectl reads results from `execute.json`. If all succeed, it advances to RECONCILE. If any fail, it reports which entries failed.

### RECONCILE

Runs per domain — each domain goes through RECONCILE → RECONCILE_EVAL → (optional COLLEAGUE_REVIEW) → RECONCILE_ADVANCE before the next domain starts.

**What happens:** Forgectl lists the specs created/updated for this domain (with their `depends_on`) and tells you to wire up cross-references.

**What you do:**
1. For every spec created or updated, use its `depends_on` to add cross-references to the corresponding specs.
2. Update both the new/updated spec AND the referenced spec (bidirectional).
3. Verify: no dangling references, symmetric integration points, consistent naming, no circular dependencies.
4. Stage changes (`git add`) and advance.

On round 1, forgectl lists all spec files to confirm they exist on disk. On subsequent rounds (after RECONCILE_EVAL FAIL), it tells you to address the evaluation findings.

### RECONCILE_EVAL

**What happens:** You spawn sub-agents (default: 1 opus general-purpose) to evaluate cross-spec consistency.

**What you do:**
1. Spawn the configured sub-agents.
2. Tell them to run `forgectl eval` — this outputs the evaluation prompt with the full spec file list, depends_on references, and the 7-dimension consistency checklist.
3. The sub-agents read the specs, evaluate, and write a report.
4. Advance with the verdict:
   - `forgectl advance --verdict PASS --eval-report <path>`
   - `forgectl advance --verdict FAIL --eval-report <path>`

If FAIL: returns to RECONCILE (up to `max_rounds`). If PASS: advances to COLLEAGUE_REVIEW (if enabled) or RECONCILE_ADVANCE (if disabled).

Eval reports go to: `{domain}/specs/.eval/reconciliation-r{round}.md`

### COLLEAGUE_REVIEW

Disabled by default. Enable with `colleague_review = true` in config.

**What happens:** Forgectl pauses the workflow for a human review gate.

**What you do:** Review the specifications for this domain with your colleague. Advance when complete: `forgectl advance`

### RECONCILE_ADVANCE

**What happens:** Explicit transition between domains.

**What you do:** Advance to proceed to the next domain's RECONCILE, or DONE if all domains are complete.

### DONE

The workflow is complete. All spec files have been produced, verified, and reconciled across all domains.

## Configuration

All configuration lives in `.forgectl/config` under the `[reverse_engineering]` section.

### Full reference

```toml
[reverse_engineering]
# Execution mode: "single_shot", "self_refine", "multi_pass", "peer_review"
mode = "self_refine"

[reverse_engineering.self_refine]
rounds = 2                    # Self-review rounds (default: 2)

[reverse_engineering.multi_pass]
passes = 2                    # Full batch re-runs (default: 2)

[reverse_engineering.peer_review]
reviewers = 3                 # Reviewer sub-agents per drafter (default: 3)
rounds = 1                    # Peer review cycles (default: 1)

# Primary agent model
[reverse_engineering.drafter]
model = "opus"

# Reconciliation eval rounds
[reverse_engineering.reconcile]
min_rounds = 1                # Minimum eval rounds (default: 1)
max_rounds = 3                # Maximum eval rounds (default: 3)
colleague_review = false      # Disabled by default; enable to add a human review gate

# Sub-agents for reconciliation evaluation
[reverse_engineering.reconcile.eval]
count = 1                     # Number of evaluator sub-agents (default: 1)
model = "opus"                # Evaluator model (default: opus)
type = "general-purpose"      # Evaluator type (default: general-purpose)

# Sub-agents for code exploration during drafting
[reverse_engineering.drafter.subagents]
model = "opus"
type = "explorer"
count = 3

# Sub-agents for peer review (peer_review mode only)
[reverse_engineering.peer_review.subagents]
model = "opus"
type = "explorer"

# Sub-agents displayed in SURVEY action output
[reverse_engineering.survey]
model = "haiku"
type = "explorer"
count = 2

# Sub-agents displayed in GAP_ANALYSIS action output
[reverse_engineering.gap_analysis]
model = "sonnet"
type = "explorer"
count = 5
```

### Execution modes

Defaults for each mode are only applied when that mode is selected. Forgectl does not store or pass configuration for inactive modes.

**`single_shot`** — Fastest. Agent drafts once with no review. Use when you trust the output or want speed.
- No mode-specific parameters.

**`self_refine`** (default) — Agent drafts, then critiques and refines its own output N times within the same session. Good balance of quality and cost.
- `rounds = 2` — number of self-review follow-ups after the initial draft (3 total actions: draft + 2 reviews)

**`multi_pass`** — Entire batch of agents runs N times. Each pass builds on the previous output (creates become updates after pass 1). Fresh sessions each pass provide a different perspective.
- `passes = 2` — number of full batch re-runs

**`peer_review`** — Agent drafts, then spawns reviewer sub-agents in parallel to evaluate the spec against the code and format. Multiple rounds allow feedback to compound. Highest quality, highest cost.
- `reviewers = 3` — number of reviewer sub-agents per drafter
- `rounds = 1` — number of peer review cycles
- `subagents.model = "opus"` — model for reviewer sub-agents
- `subagents.type = "explorer"` — role for reviewer sub-agents

### Sub-agent config independence

The system has four independent sub-agent configurations:

| Config | Purpose | When used |
|--------|---------|-----------|
| `drafter.subagents` | Code exploration during spec drafting | EXECUTE — initial prompt |
| `peer_review.subagents` | Spec review during peer review | EXECUTE — peer review follow-up |
| `survey` | Spec directory exploration | SURVEY — action output |
| `gap_analysis` | Source code analysis | GAP_ANALYSIS — action output |

Each can use a different model, type, and count. Code exploration sub-agents and peer review sub-agents are independent — they serve different purposes.

## File Artifacts

| File | Created by | Purpose |
|------|-----------|---------|
| `input.json` | User | Init input with concept and domains |
| `queue.json` | User | Reverse engineering queue with spec entries |
| `execute.json` | Forgectl | Handoff to Python subprocess with config and entries |
| `forgectl-state.json` | Forgectl | Session state tracking |
| `<domain>/specs/*.md` | Python subprocess (agents) | The produced spec files |

## Prompts

The Python package bundles four prompt files. These are not user-configurable — they ship with the package.

| Prompt | Mode | Purpose |
|--------|------|---------|
| `reverse-engineering-prompt.md` | All | Initial draft instructions with interpolation fields |
| `spec-format-reference.md` | All | Spec format structure, principles, anti-patterns |
| `review-work-prompt.md` | `self_refine` | Self-critique follow-up with round awareness |
| `peer-review-prompt.md` | `peer_review` | Spawn reviewer sub-agents in parallel |
