# Evaluation Report — Round 2

## Verdict: FAIL

## Summary
- Dimensions passed: 10/11
- Total spec requirements checked: full genuinely-new-work delta across all six specs (batch-terminal inline commit, per-round per-item commit, message synthesis, `eval_mode: direct` re-entry for both phases, and the entire planning readiness gate subsystem — workspace inspection, verdict assembly, `preflight`, and enforcement at the three cold-start entry points). Already-implemented functionality (`kind` routing, the ui_implementing loop machinery itself, `passes`/`rounds` mutation, config validation trees, `AutoCommit`'s core staging/empty-commit handling, `handoff`/`eval`, and the already-rewritten `docs/auto-committing.md` / `docs/schemas/plan-queue.md`) was verified present in code and correctly excluded from the plan, per the supplied scope context.
- Deficiencies: 1

## Round 1 Deficiency Verification

All five round-1 deficiencies were checked against the actual current plan.json content, not just against the round-2 prompt's claims:

1. **Observability (readiness gate logging unwired).** Fixed correctly. The new L2 item `gate.logging` (`forgectl/state/readiness.go`, `forgectl/cmd/preflight.go`, `forgectl/cmd/init.go`, `forgectl/state/advance.go`) wires an INFO entry (domain count + verdict), an ERROR entry (dirty domains + workspace paths, emitted in addition to INFO only when blocked), and a DEBUG entry per inspected domain (path + clean/dirty) — matching planning-readiness-gate.md §Observability verbatim. It correctly routes through the existing `state.Logger`/`LogEntry` mechanism (no new sink) and correctly relies on `NewLogger`'s existing no-session no-op rather than adding special-case suppression logic. I verified in `forgectl/cmd/init.go` that `sessionID := state.GenerateSessionID()` (line 99) executes before the `PhasePlanning` branch (line 143) where the gate check will be inserted, so the init cold-start entry has a valid session context to log against — the "session-less" no-op path is correctly scoped to `preflight` only, consistent with the spec's own observability note.
2. **Edge Cases — dirty workspace outside the incoming queue ignored.** Fixed. `state.readiness-gate` gained the edge_case test: "Domain 'a' has a full workspace but the plan queue lists only domain 'b' ... verdict is READY and domain 'a' is never inspected."
3. **Edge Cases — mid-cycle foreign write into a not-yet-reached domain.** Fixed. `gate.enforce-phase-shift` gained the edge_case test distinguishing this from the already-tested "domain A holds its own completed plan.json" case: "Queue [a, b] was clean at cold start; while domain a is being planned, something outside the scaffold writes a file into b's workspace before the cycle reaches it ... planning ORIENT for b is entered with no re-inspection and no error."
4. **Testing Criteria — "continuation queue ignores already-planned domains."** Fixed by the same test as #2 (`state.readiness-gate`), which now also satisfies this independently-named Testing Criteria entry.
5. **Testing Criteria — ui inline-terminal-commit missing "nothing staged" test.** Fixed. `ui.inline-terminal-commit` gained the edge_case test: "When the per-item commits already captured every change, the inline batch commit at terminal E2E_VERIFY finds nothing staged and is skipped silently, and the transition still completes."

The two "also applied" items from the round-2 prompt are both present and correct: `output.commit-flow.depends_on` now includes `impl.per-item-commit` and `ui.per-item-commit` (previously missing despite the item's first step being about `--message` becoming optional, which is exactly those items' behavior change); `cmd.preflight` now has the rejection test "active session exists but has no pending plan queue (`generate_planning_queue` state absent, or its plan queue file path empty)" as a trigger distinct from "no active session."

None of the five fixes are superficial — each adds a test whose `Given`/`Then` reproduces the exact scenario the spec or round-1 report named, not a restatement of an already-passing case.

## Dimension Results

### 1. Behavior — PASS
Every Behavior-section step for the new work has plan coverage. planning-readiness-gate.md's Readiness Query (4 steps) and Enforcement at Planning Entry (4 steps) procedures are reproduced in `state.readiness-gate`, `gate.enforce-init`, `gate.enforce-phase-shift`'s steps, including "abort before any state mutation." batch-implementation.md's State Machine/Transition Table deltas (inline terminal commit, per-round per-item commit, direct-mode re-entry) map to `impl.per-item-commit`, `impl.inline-terminal-commit`, `impl.direct-mode-reentry`; the ui-batch-implementation.md analogs map to the `ui.*` items, correctly scoped to the code-eval loop only (the QA and e2e loops are explicitly left untouched in `ui.direct-mode-reentry`'s steps, matching the spec's "QA and e2e loops are unaffected — they have their own iteration states").

### 2. Error Handling — PASS
The Error Handling subsections under planning-readiness-gate.md's three Behavior blocks are covered: unreadable/invalid plan queue (`cmd.preflight` rejection tests), dirty-domain abort as rejection-not-crash (`gate.enforce-init`/`gate.enforce-phase-shift`), and untraversable-workspace-as-dirty-with-OS-detail (`state.workspace-inspection` edge_case, `state.readiness-gate` edge_case).

### 3. Rejection — PASS
All five planning-readiness-gate.md Rejection rows map to tests. session-init.md's readiness-gate rejection row maps to `gate.enforce-init`. phase-transitions.md's cold-start rejection row maps to `gate.enforce-phase-shift`. The batch/ui-batch Rejection-table rows relevant to the delta (none require `--message` at `enable_commits: true`) are tested in `impl.inline-terminal-commit`, `ui.inline-terminal-commit`, `impl.per-item-commit`, `ui.per-item-commit`.

### 4. Interface — PASS
`state.workspace-inspection` defines the Domain Workspace Status fields (`domain`, `workspace_path`, `clean`, `error`) exactly per spec. `state.readiness-gate`'s READY/BLOCKED strings are reproduced verbatim in its steps. Minor, non-blocking observation (carried over from round 1, unchanged): the spec's "Readiness Verdict" data model names a `dirty_domains` field explicitly, and no plan item step calls out that field by name — the BLOCKED-output tests make its presence functionally necessary, so this is a documentation-precision gap rather than a missing capability.

### 5. Configuration — PASS
The only new-work configuration touchpoint, `paths.workspace_dir`, is handled in `state.workspace-inspection` with a dedicated non-default-value test. No new config keys are introduced elsewhere, matching the specs. All `ui_implementing.*` config table entries are pre-existing and correctly excluded.

### 6. Observability — PASS
Resolved per round-1 deficiency #1 above. No further observability gaps found: ui-batch-implementation.md's Observability section (the only other spec with one) describes generic per-transition logging (batch selected, loop entered, verdict recorded, batch committed) that is unaffected by the inline-commit branching change, since the generic `cmd/advance.go` logging wrapper fires on every `advance` call regardless of the internal state-machine branch taken — confirmed no plan item needed to touch it.

### 7. Integration Points — PASS
`gate.enforce-init` and `gate.enforce-phase-shift` both depend on `state.readiness-gate`; `cmd.preflight` depends on `state.readiness-gate`; `gate.logging` depends on both enforcement items plus `state.readiness-gate`, matching activity-logging's Integration Points row. `output.commit-flow`'s dependency edges now correctly include `impl.per-item-commit`/`ui.per-item-commit` (round-1's non-blocking observation, now fixed). `docs.sync` depends on every rendering/gate item it documents.

### 8. Invariants — PASS
batch-implementation.md invariants 6/7/12/14 and their ui-batch-implementation.md analogs (12/17/18/19) all have enforcement mechanisms in the corresponding plan items. planning-readiness-gate.md invariants 1–5 map to `state.readiness-gate`/`gate.enforce-init`/`gate.enforce-phase-shift`/`cmd.preflight`; invariant 5 ("uniform cold-start enforcement... the same evaluation logic backs every one") is enforced by construction — all four call sites (`cmd.preflight`, `gate.enforce-init`, `gate.enforce-phase-shift`, and implicitly the Readiness Query itself) route through the single shared `state.readiness-gate` function per their steps, rather than needing a dedicated cross-entry-point consistency test.

### 9. Edge Cases — FAIL
Both round-1 deficiencies are resolved (verified above). One new, previously-unflagged edge case has no corresponding test:

planning-readiness-gate.md §Edge Cases: **"A domain name contains a path separator (e.g., `protocols/ws1`)."** Expected behavior: "The gate resolves the workspace to `protocols/ws1/<workspace_dir>/` and inspects that directory." No test in `state.workspace-inspection` (or anywhere else in the plan) exercises a domain name containing a `/`. The closest existing test — "A domain name is resolved to its configured `[[domains]]` path; with no domains configured it falls back to the domain name as the path" — verifies the general resolution mechanism (configured-path vs. name-fallback) but not this specific, spec-named scenario of a nested/slashed domain name being joined correctly with `workspace_dir` and walked as a real nested directory rather than, say, being sanitized, rejected, or mis-joined. Given the spec calls this out as its own bullet with precise expected behavior, and the plan's granularity elsewhere names each edge case individually, this is a genuine gap rather than an over-strict reading.

### 10. Testing Criteria — PASS
All 13 planning-readiness-gate.md Testing Criteria entries map to tests (round-1 deficiency #4 resolved). All batch-implementation.md and ui-batch-implementation.md Testing Criteria entries relevant to the delta map to tests, including the exact FAIL→FAIL→PASS direct-mode termination sequences for both phases (`impl.direct-mode-reentry`'s edge_case and functional tests; `ui.direct-mode-reentry`'s edge_case and functional tests) and the negative case that intra-session re-entries stay ungated (`gate.enforce-phase-shift`'s two functional tests for planning→planning and implementing/ui_implementing→planning). No Testing Criteria entry independently names the path-separator domain scenario from Edge Cases finding above, so this dimension is not doubly failed by it.

### 11. Dependencies & Format — PASS
All 15 items appear in exactly one layer (L0: 2, L1: 4, L2: 7, L3: 2 = 15, matching the `items` array). All `depends_on` IDs resolve to existing items. No item depends on a later-layer item; same-layer dependencies (e.g., `ui.inline-terminal-commit` → `ui.per-item-commit`, both L2; `gate.logging` → `gate.enforce-init`/`gate.enforce-phase-shift`, both L2; `docs.sync` → `output.commit-flow`, both L3) are legal per `plan-format.json`'s "same or earlier layer" rule. No cycles. All test categories are valid. All top-level `refs` and item `refs` paths (five notes files, six specs) exist on disk and were read in full during this evaluation. Format matches `plan-format.json`'s required shape (`context`/`refs`/`layers`/`items` only, no extraneous top-level fields).

## Deficiency List (FAIL only)

| # | Dimension | Spec Section | Missing Coverage |
|---|-----------|-------------|-----------------|
| 1 | Edge Cases | planning-readiness-gate.md §Edge Cases — "A domain name contains a path separator (e.g., `protocols/ws1`)" | No test verifies that a domain name containing a `/` (in the no-`[[domains]]`-configured fallback path) resolves to `<domain-name>/<workspace_dir>/` and is inspected as that nested directory. `state.workspace-inspection`'s existing domain-resolution test covers the configured-vs-fallback mechanism generically but not this specific, spec-named nested-name scenario. |
