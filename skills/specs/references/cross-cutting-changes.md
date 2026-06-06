# Cross-Cutting Specification Changes

You are making cross-cutting changes to a set of interconnected specification documents. The design decisions have already been made. Your job is to execute them completely and correctly.

**Step 1 — Make the primary changes.** Implement the agreed-upon modifications to the directly affected specifications.

**Step 2 — Propagate.** Determine the appropriate strategy for finding every downstream reference affected by your changes, then execute it. The goal is to ensure no specification still describes, references, or depends on the old model. Do not assume any file is unaffected — verify.

Please spawn multiple subagents to help in this matter,  on a good order 1 sub agents per 4 spec files.

**Step 3 — Commit.** Stage all affected files and commit with a message that explains the intent of the change.

**Step 4 — Self-review.** Re-read every modified specification end-to-end. Use [../../shared/spec-format.md](../../shared/spec-format.md) as the structural reference for what a well-formed spec looks like. Check specifically for:
- References to removed or relocated specifications
- Inconsistent numbering or sequencing
- Terminology mismatches with the new model
- Process flows or descriptions that contradict the changes made
- Cross-references that point to specifications no longer in scope
- Depends On entries that reference specs that no longer exist
- Integration Points that are no longer symmetric (see spec-format.md § Integration Points)

**Step 5 — Fix and commit separately.** Address all findings from the review in a distinct commit.
