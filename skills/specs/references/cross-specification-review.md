# Cross-Specification Review

You are reviewing a set of interconnected specification documents for contradictions, ambiguities, gaps, and stale references. Your goal is to surface questions and findings for the stakeholder — not to fix problems or make design decisions.

---

## Phase 1 — Scope Discovery and Study

### Resolve Scope

The stakeholder provides scope as either a directory pattern (e.g., `specs/*`) or an explicit file list. Before reading anything, resolve the scope into a concrete file list:

- [ ] Enumerate every file in the provided path or list
- [ ] Exclude any directories or files the stakeholder has marked as out of scope
- [ ] Present the complete file list to the stakeholder for confirmation
- [ ] Note the total file count — this determines how many sub-agents to spawn in Phase 2

### Study

Read every specification file in the confirmed scope. Build a mental model of the full system before looking for issues. You cannot find cross-cutting problems if you only understand individual pieces.

Familiarize yourself with [../../shared/spec-format.md](../../shared/spec-format.md) — it defines the structural contract all specs follow (sections, ordering, principles). Use it as the baseline for what a well-formed spec looks like during analysis.

---

## Phase 2 — Parallel Analysis

Spawn sub-agents to investigate the specifications. Divide work by **concern type**, not by file. Each agent receives a broad concern area, a set of mandatory checks, and an open investigation mandate.

### Agent Assignment Rules

- Every specification file must be assigned to at least one agent. After defining agents and their concern areas, verify full coverage — list every spec file and confirm which agent(s) will read it. If any file is unassigned, adjust.
- Each agent reads **all** specs relevant to its concern area (typically most or all of them). Agents may overlap on files — this is expected and desirable.
- Target approximately 1 agent per 4 specification files, scaling with the total spec count.

### Concern Areas

Adapt these to the system under review. The following are starting points:

**Agent A — Identity, Lifecycle, and State Coherence**
Investigates whether entities, their states, and their transitions are defined consistently across specs.

**Agent B — Configuration, Parameters, and Numeric Consistency**
Investigates whether configuration values, defaults, timeouts, units, and naming conventions are consistent and non-contradictory.

**Agent C — Cross-References, Dependencies, and Completeness**
Investigates whether all references resolve, all dependencies are documented, and all referenced concepts are defined somewhere.

**Agent D — Data Flow, Formats, and Edge Cases**
Investigates whether data produced by one spec matches what the next spec expects to consume, whether message/data formats are consistent, and whether error and edge cases are handled.

### Agent Prompt Structure

Each agent receives:

1. **The concern area** — a short description of what they're investigating (gives scope).
2. **Mandatory checks** — specific items the agent MUST verify. These are known patterns worth checking in any specification review:
   - Do identifier formats match across specs?
   - Do shared terms have consistent definitions?
   - Do data structures referenced in one spec match their definition in another?
   - Do configuration parameters follow the declared naming and ownership conventions?
   - Do cross-references point to existing specs and sections?
3. **Open investigation mandate** — "Beyond the mandatory checks, investigate anything else within your concern area that you discover while reading. Report all findings."
4. **The file list** — explicit list of every spec file the agent must read.

### Agent Output Format

Each agent returns findings using this structure:

```
Finding ID:       <agent letter>-<number>  (e.g., A-001)
Type:             contradiction | ambiguity | gap | stale reference
Specs involved:   [list of spec files]
Evidence A:       {spec, section or line, quoted text}
Evidence B:       {spec, section or line, quoted text}  (if applicable)
Description:      One-sentence summary of the issue
Impact:           What goes wrong if this isn't resolved
```

Agents should report every finding they discover. Duplicate findings across agents are expected — when multiple agents independently find the same issue, it is corroborating evidence that strengthens confidence in the finding.

---

## Phase 3 — Synthesis

After all agents report back, consolidate findings into a single report for the stakeholder:

1. **Group by spec boundary** — organize findings by which spec pair or group is affected. This helps the stakeholder see which areas of the system have the most tension.
2. **Identify dependency chains** — some findings are downstream consequences of others. If resolving finding A-002 would automatically resolve B-007 and D-003, note this. Present foundational findings before their downstream consequences.
3. **Preserve the taxonomy** — use the type labels (contradiction, ambiguity, gap, stale reference) so the stakeholder knows what kind of response each finding needs:
   - **Contradiction**: two specs say incompatible things — one must change.
   - **Ambiguity**: a spec can be read multiple ways — needs a design decision.
   - **Gap**: something is referenced but never defined — needs new content.
   - **Stale reference**: points to something removed or renamed — mechanical fix.
4. **Note corroborated findings** — when multiple agents independently report the same issue, note this. It is signal, not noise.
5. **Present findings as questions** — frame each finding as something the stakeholder needs to decide or confirm, not as a prescription. The review surfaces the right questions; the stakeholder makes the design decisions.

---

## Phase 4 — Stakeholder Discussion

Walk through findings with the stakeholder. Expect that:

- Some findings will be resolved with a single sentence.
- Some findings will open design discussions that reshape the system.
- Some findings will be intentional and need no action.
- Foundational decisions may cascade — resolving one question may resolve several downstream findings.

Present foundational questions first. Let the stakeholder's decisions inform how you present subsequent findings.

The stakeholder may at any point:

- Ask for clarification or deeper investigation on a specific finding.
- Dismiss a finding as intentional — this is normal, not a failure of the review.
- Redirect the discussion to a related concern the findings didn't cover.
- Defer a finding to a later discussion.

---

## Phase 5 — Action Plan

After discussion, compile all agreed-upon changes into a structured plan organized by file. For each file, list:

- What changes
- Why (which finding or design decision drives it)
- What other files are affected by this change (propagation)

Include a propagation checklist: after making primary changes, verify that every downstream reference has been updated to match.

When executing the action plan, use the methodology in [cross-cutting-changes.md](cross-cutting-changes.md) to propagate changes systematically across the affected specs.
