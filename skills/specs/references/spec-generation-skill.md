# Spec Generation Skill

> Instructions for generating specifications from planning documents.
> This is the process guide — `spec-format.md` defines the output structure.

---

## Role

You are a **Systems Architect**. You write contracts that define what "correct" means. You do not manage timelines, coordinate teams, or write tutorials. Your output is precise, testable, and authoritative.

---

## Inputs

- **Planning documents** — found in `.workspace/planning/`. These are read-only. You consume them but never modify or delete them.
- **Existing specs** — found in `<project>/specs/` directories. These tell you what is already covered.
- **Spec queue** — a JSON file listing the specs to generate with topics of concern, domains, file paths, planning source references, and dependency ordering. This is the input to `scaffold init`.
- **User domain knowledge** — gathered through clarifying questions during the session.

## Outputs

- One or more spec files written to the appropriate `<project>/specs/` directory.
- If a broad topic naturally decomposes into multiple focused topics, produce multiple spec files rather than one overloaded spec.

---

## Scaffold

The spec generation process is managed by the `forgectl` CLI tool that tracks state in a JSON file. The architect invokes forgectl commands to see what to do next and to record transitions.

**Full specification:** see the forgectl specs directory for the complete CLI spec.

### Quick Reference

```bash
# Initialize session — validate queue, set rounds
forgectl init --min-rounds 1 --max-rounds 3 --batch-size 1 --from queue.json --guided

# Full session overview
forgectl status

# Advance to next state (no flags needed for most transitions)
forgectl advance
forgectl advance --file <domain>/specs/<spec-name>.md            # DRAFT only (override path)
forgectl advance --verdict PASS --eval-report <path> --message "Add repo loading spec"  # EVALUATE: accept
forgectl advance --verdict FAIL --eval-report <path>            # EVALUATE: fail → REFINE

# Add a commit to a completed spec by ID
forgectl add-commit --id 5 --hash "7cede10"

# Auto-register a commit to all specs it touched (reconciliation, shared fixes)
forgectl reconcile-commit --hash "8743b1d"
```

### CLI Flag Reference

**Global:** `--dir <path>` — directory containing state file (default `.`)

**`init`:** `--from` (required), `--batch-size` (required), `--max-rounds` (required), `--min-rounds` (default 1), `--guided` (default) / `--no-guided`, `--phase` (default `specifying`)

**`advance`:** `--verdict PASS|FAIL`, `--eval-report <path>` (absolute path to eval file), `--file <path>`, `--message <string>`, `--from <path>`, `--guided` / `--no-guided`

**`add-commit`:** `--id` (required), `--hash` (required)

**`reconcile-commit`:** `--hash` (required)

### State Flow

```
INIT → ORIENT → SELECT → DRAFT → EVALUATE ⇄ REFINE → REVIEW → ACCEPT → (next spec or DONE)
                                                         ↑         │
                                                         └─────────┘ (grant extra round)
```

- **INIT**: Create state file from validated queue. Set `--min-rounds`, `--max-rounds`, `--batch-size`, `--guided`.
- **ORIENT**: Architect reads plans and existing specs. Builds mental model.
- **SELECT**: Pull next topic from queue. If `--guided`, discuss with user.
- **DRAFT**: Write the spec file. Optionally override path with `--file`.
- **EVALUATE**: Spawn Opus sub-agent evaluator. Record `--verdict` and `--eval-report`. PASS requires `--message`.
- **REFINE**: Fix deficiencies. Advance back to EVALUATE when done (no special flags needed).
- **REVIEW**: Max rounds reached (or past min rounds). Human decides: accept or `--verdict FAIL` for extra round.
- **ACCEPT**: Spec finalized. Next spec loops to ORIENT. Empty queue → DONE.
- **DONE**: All individual specs complete. Advance to begin reconciliation.
- **RECONCILE**: Fix cross-references across all specs. Stage files. Advance.
- **RECONCILE_EVAL**: Sub-agent evaluates `git diff --staged` for cross-spec consistency.
- **RECONCILE_REVIEW**: Human reviews eval. Accept or grant another pass.
- **COMPLETE**: Session fully done.

### Queue Input File

The architect generates this JSON before `init`. The scaffold validates it strictly — no extra fields, no missing fields.

```json
{
  "specs": [
    {
      "name": "Reservation Booking",
      "domain": "accounting",
      "topic": "The accounting service processes reservation bookings and records settlement outcomes",
      "file": "accounting/specs/reservation-booking.md",
      "planning_sources": [".workspace/planning/accounting/transaction-processing.md"],
      "depends_on": []
    }
  ]
}
```

---

## Evaluation Sub-Agent Protocol

When the scaffold reaches the EVALUATE state, spawn an Opus sub-agent using the Agent tool. The sub-agent receives a structured prompt with file references, not inline content.

### Sub-Agent Framing

The evaluator is an **adversarial reviewer**, not a rubber stamp. Its job is to find deficiencies. A spec that passes all 10 dimensions on the first round is unusual and should be verified thoroughly before a blanket PASS is issued.

Key principles:
- **Evidence over assertion.** Every verdict — PASS or FAIL — must cite specific sections, behaviors, or models from the spec and planning sources. "PASS — —" is not a valid output.
- **Read before judging.** The agent must summarize what it read from each document before evaluating. This proves comprehension and prevents skimming.
- **Completeness is a checklist, not a vibe.** The agent must enumerate every behavior/model from the planning sources and check each one against the spec.
- **Silence is a finding.** If the spec is missing an entire section that spec-format.md defines (e.g., Observability), that's a format compliance failure even if the planning docs don't mention it — the agent must flag whether the omission is justified.

### Sub-Agent Prompt Format

```markdown
# Spec Evaluation

You are an adversarial specification reviewer. Your job is to find deficiencies
in a draft spec. You are not here to validate — you are here to stress-test.
A blanket PASS across all dimensions on the first round is unusual. Verify
thoroughly before concluding that nothing needs work.

## Reference Documents

Read these files completely before evaluating:

- **Spec format and principles:** `references/spec-format.md`
- **Generation skill (constraints and anti-patterns):** `references/spec-generation-skill.md`

## Spec Under Review

- **Spec file:** `<path-to-spec-file>`

## Planning Sources (read-only context)

These are the planning documents the spec was derived from. Use them to check
completeness — is every relevant behavior from the plans covered or explicitly
excluded?

- `<path-to-planning-file-1>`
- `<path-to-planning-file-2>`
- ...

---

## Step 1: Document Summary (REQUIRED)

Before evaluating, summarize what you read. This proves comprehension and
prevents skimming. For each document:

### Planning Source: `<filename>`
- Key models/behaviors defined: [list them]
- Key constraints/rules: [list them]

### Spec Under Review: `<filename>`
- Topic of concern: [quote it]
- Models defined: [list them]
- Behaviors defined: [list them]
- Number of testing criteria: [count]
- Number of edge cases: [count]
- Number of invariants: [count]

---

## Step 2: Completeness Checklist (REQUIRED)

Enumerate every model, behavior, constraint, and rule from the planning
sources. For each one, state whether it is:
- **Covered** — present in the spec (cite the section)
- **Excluded** — not in the spec, with rationale given
- **Missing** — not in the spec, no rationale given (this is a deficiency)
- **Out of scope** — belongs to a different spec's topic of concern

Use a table:

| Item from Planning | Status | Spec Section (if covered) | Notes |
|--------------------|--------|---------------------------|-------|
| [model/behavior]   | Covered/Excluded/Missing/Out of scope | [section] | [notes] |

---

## Step 3: Dimension Evaluation

Evaluate the spec across these dimensions. For each dimension, assign PASS
or FAIL. **Every dimension requires a justification of 2-4 sentences citing
specific evidence from the spec, even on PASS.** A bare "—" is not acceptable.

| Dimension | What to check |
|-----------|---------------|
| **Completeness** | Every behavior in the planning sources is covered in the spec or explicitly excluded with rationale. No silent omissions. Use your checklist from Step 2. |
| **Testability** | Every behavior, invariant, and edge case has a corresponding testing criterion with Given/When/Then. Count them: behaviors without tests are a FAIL. |
| **Precision** | No vague language: "appropriately", "as needed", "handled", "properly", "correctly". Every qualifier resolves to a concrete condition. Quote any vague phrases you find. |
| **Voice** | Declarative throughout. No "should", "could", "might", "would". The spec states what the system *does*, not what it *should do*. Quote any violations. |
| **Error Exhaustiveness** | Every step in every behavior that can fail has a named failure mode and a named response. Silence means the step cannot fail — verify this is actually true for each silent step. |
| **Topic Focus** | The topic of concern passes the one-sentence test. No scope creep into adjacent topics. Name any sections that drift. |
| **Format Compliance** | Follows the structure in spec-format.md. Sections are in the correct order. Required sections are present. If a section from spec-format.md is absent (e.g., Observability, Configuration), state whether the omission is justified for this topic. |
| **No Plan Leakage** | No references to planning file paths, planning directory structure, or planning document names as file references. The `Implements` section may name topics but not file locations. Quote any violations. |
| **Edge Case Coverage** | Boundary conditions, unusual inputs, empty states, and race conditions are addressed. Each has scenario, expected behavior, and rationale. Identify any boundary conditions from the planning sources that are missing. |
| **Invariant Correctness** | Invariants are always-true properties, not postconditions of specific operations. They hold regardless of operation order. For each invariant, verify it cannot be temporarily violated. |

---

## Step 4: Output

Return your evaluation in this exact structure:

### Verdict: [PASS | FAIL]

### Round Summary
2-3 sentences: what is the strongest aspect of this spec, what is the weakest,
and what (if anything) blocks acceptance.

### Dimension Results

| Dimension | Verdict | Evidence |
|-----------|---------|----------|
| Completeness | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Testability | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Precision | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Voice | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Error Exhaustiveness | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Topic Focus | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Format Compliance | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| No Plan Leakage | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Edge Case Coverage | PASS/FAIL | [2-4 sentence justification citing specific sections] |
| Invariant Correctness | PASS/FAIL | [2-4 sentence justification citing specific sections] |

### Deficiency Details

For each FAIL dimension, provide:

#### [Dimension Name]
- **Issue:** What specifically is wrong.
- **Location:** Which section or behavior in the spec.
- **Evidence:** Quote the problematic text or name the missing item.
- **Fix:** What the architect should do to resolve it.

### Observations

Optional: note anything that is technically passing but borderline, or patterns
you noticed that the architect should be aware of for future specs. This section
is advisory — it does not affect the verdict.

Do not rewrite the spec. Your job is to evaluate, not to author.
```

### Sub-Agent Configuration

- **Model:** Opus
- **Type:** general-purpose
- **Description:** "Evaluate spec: `<spec-name>`"
- **Prompt:** The formatted prompt above, with file paths filled in.

The sub-agent reads the reference files itself. Do not paste file contents into the prompt — use file paths so the sub-agent works with the current state on disk.

---

## Reconciliation Eval Sub-Agent Protocol

When the scaffold reaches RECONCILE_EVAL, the architect tells the sub-agent to evaluate cross-spec consistency. Unlike per-spec evals, this eval works from `git diff --staged` to see all reconciliation changes holistically.

### Reconciliation Sub-Agent Prompt Format

```markdown
# Reconciliation Evaluation

You are an adversarial reviewer evaluating cross-reference consistency
across a set of specifications. Your job is to find inconsistencies
between specs, not to evaluate individual spec quality.

## Instructions

1. Run `git diff --staged` to see all reconciliation changes.
2. Read every spec file listed below.
3. Evaluate the cross-reference consistency.

## Spec Files

- `<path-to-spec-1>`
- `<path-to-spec-2>`
- ...

## Evaluation Criteria

| Dimension | What to check |
|-----------|---------------|
| **Dependency completeness** | Every `Depends On` entry references a spec that exists. No dangling references. |
| **Dependency symmetry** | If spec A lists spec B in Integration Points, spec B lists spec A. |
| **Naming consistency** | Spec names match exactly across all Depends On and Integration Points references. No aliases or abbreviations. |
| **No circular dependencies** | The dependency graph is a DAG. No spec depends on itself through any chain. |
| **Integration point accuracy** | Each Integration Points entry correctly describes the relationship (what flows, in which direction). |
| **Scope boundary respect** | No spec defines behavior that another spec claims to own. |

## Output Format

### Verdict: [PASS | FAIL]

### Summary
2-3 sentences on overall consistency.

### Findings

For each issue found:
- **Specs involved:** Which specs have the inconsistency.
- **Issue:** What is wrong.
- **Fix:** What the architect should do.

If no issues found, state what was checked and why it passes.
```

---

## Eval Output Convention

Evaluation sub-agents write their output to a structured directory:

```
<project>/specs/.eval/
├── <spec-name>-r1.md
├── <spec-name>-r2.md
├── reconciliation-r1.md
└── ...
```

- File name: spec name (kebab-case) + round number.
- Reconciliation evals use `reconciliation-rN.md`.
- The scaffold does not read these files — they are for the architect's reference.
- Add `.eval/` to `.gitignore` if eval output should not be committed.

---

## Scoping Multi-Responsibility Plans

When a planning document bundles multiple responsibilities (e.g., a reservation management plan covers reservation creation, booking, fulfillment, and notifications), the architect must:

1. **Identify the distinct topics.** Each responsibility that passes the one-sentence test becomes a separate spec.
2. **Write explicit scope exclusion notes.** In each spec's Context section, state what is excluded and why (e.g., "Booking settlement is specified separately in the Booking Settlement spec").
3. **Use "Out of scope" in completeness checklists.** The eval sub-agent's completeness checklist has an "Out of scope" status for items that belong to another spec's topic.
4. **Cross-reference via Integration Points.** Each spec references the others it was split from, so readers can trace the full picture.

---

## Common Eval Findings (Lessons Learned)

These patterns were found repeatedly during evaluation. Avoid them during drafting:

### Phantom Observability Entries
**Pattern:** The Observability/Logging section references a behavior (e.g., "parsing retry") that no Behavior section defines.
**Fix:** Every log entry must correspond to a defined behavior step or error handling path. If the behavior doesn't include a retry, the log entry shouldn't mention one.

### Unverifiable Invariants
**Pattern:** An invariant describes LLM behavior intent (e.g., "prior ideas are context, not constraints") rather than a verifiable system property.
**Fix:** Invariants must be testable by a test suite at any point. If you can't write a Given/When/Then for it, demote it to an edge case or design note.

### Missing Observability Section
**Pattern:** The spec has no Observability section, even though the behavior involves state transitions, external calls, or error conditions that warrant logging.
**Fix:** Every spec with behaviors that can fail or that report progress needs at minimum INFO for success, ERROR for failures, and DEBUG for diagnostic details.

### Untested Invariants
**Pattern:** Invariants are listed but have no corresponding testing criteria.
**Fix:** Every invariant needs a Given/When/Then test. If you can't test it, it's either not an invariant or it's too vague.

### Internal Architecture as Invariants
**Pattern:** Invariants prescribe concurrency strategy, internal data structures, or threading models (e.g., "WS handler is async, worker is blocking").
**Fix:** Per spec-format.md Principle 5, these are implementation details. Reformulate as externally observable properties (e.g., "command processing is decoupled from execution").

### Silent Omissions from Planning Sources
**Pattern:** A planning document defines a behavior that the spec neither covers nor explicitly excludes.
**Fix:** Every item from the planning source must appear in the spec (covered), in a scope exclusion note (excluded with rationale), or in the eval checklist as "out of scope" (belongs to another spec).

---

## Reconciliation Checklist

During the RECONCILE phase, verify:

- [ ] Every `Depends On` reference points to a spec that exists in `<project>/specs/`
- [ ] Every `Depends On` entry has a corresponding `Integration Points` row in the referenced spec
- [ ] Integration Points are symmetric: if A mentions B, B mentions A
- [ ] Spec names are consistent across all references (no aliases, abbreviations, or stale names)
- [ ] No circular dependencies in the `Depends On` graph
- [ ] Each Integration Points relationship description accurately reflects what flows between specs
- [ ] Leaf dependencies (specs with no Depends On) are correctly identified
- [ ] The `Implements` section in each spec names the correct planning topics

---

## File Naming and Placement

**Convention: kebab-case, no suffix.**

```
<project>/specs/
├── reservation-booking.md
├── ledger-reconciliation.md
├── account-reconciliation.md
```

- The file lives in `specs/` — that already signals what it is.
- Name reflects the topic of concern, not the planning document it came from.
- Place the spec in the `specs/` directory of the project it governs (e.g., `<domain>/specs/`).
- Protocol specs go in `protocols/<name>/specs/`.

---

## Constraints

### Plans are read-only
Planning documents are input, not output. Never modify, delete, or annotate them. Other spec sessions may depend on the same plans.

### No planning references in specs
Do not reference planning file paths in the spec. The `Implements` section may name the planning *topic* but not the file location.

### No open questions in specs
If something is unresolved after discussion with the user, it does not go in the spec. Exclude it or resolve it.

### Split rather than overload
If a topic is too broad, produce multiple spec files. Each spec should pass the one-sentence test.

### Track upstream spec impacts
When generating a spec, identify any existing specs that need updates due to new information, integration points, or dependency relationships introduced by the new spec. Note these in the ACCEPT phase so they can be addressed.

### The architect decides
Do not ask the user clarifying questions during spec generation. Resolve open questions by reading plans carefully and making judgment calls. Document decisions in the spec's edge cases and rationale sections.

### Specs are technology-aware, not technology-coupled
Reference technology when it's part of the interface contract. Don't prescribe internal implementation choices.

---

## Anti-Patterns

| Don't | Do Instead |
|-------|------------|
| Write "the system should..." | Write "the system does..." |
| Reference file paths or module names | Describe behavior and contracts |
| Leave error handling as "errors are handled" | Name each failure mode and its response |
| Write one massive spec for a broad topic | Split into focused specs with clear topics |
| Copy plan structure into spec structure | Restructure around contracts, not proposals |
| Modify planning documents | Leave plans untouched for other sessions |
| Include open questions or TBDs | Resolve with user or exclude from spec |
| Self-evaluate instead of using sub-agent | Always use the evaluation sub-agent |
| Paste file contents into sub-agent prompt | Use file path references |

---

## Checklist Before Finalizing

- [ ] Topic of concern passes the one-sentence test
- [ ] Declarative voice throughout (no "should", "could", "might")
- [ ] Every behavior has testing criteria (Given/When/Then)
- [ ] Error handling is exhaustive — every failure mode named
- [ ] Edge cases capture judgment calls
- [ ] Invariants are always-true properties, not postconditions
- [ ] No references to planning file paths
- [ ] No open questions
- [ ] File is kebab-case, placed in correct project's `specs/` directory
- [ ] Spec follows spec-format.md structure
- [ ] Passed evaluation sub-agent review (or user accepted after max rounds)
