# Evaluation Report — Round 3

## Verdict: FAIL

## Summary
- Dimensions passed: 8/11
- Total spec requirements checked: full genuinely-new-work delta across all six specs (batch-terminal inline commit, per-round per-item commit, message synthesis, `eval_mode: direct` re-entry for both phases, and the entire planning readiness gate subsystem — workspace inspection, verdict assembly, `preflight`, and enforcement at the three cold-start entry points). Already-implemented functionality (`kind` routing, the ui_implementing loop machinery itself, `passes`/`rounds` mutation, config validation trees, `AutoCommit`'s core staging/empty-commit handling, `handoff`/`eval`, and the already-rewritten `docs/auto-committing.md` / `docs/schemas/plan-queue.md`) was verified present in code and correctly excluded from the plan, per the supplied scope context.
- Deficiencies: 1 (new; spans three dimensions below)

## Round 2 Deficiency Verification

Round 2 failed on exactly one deficiency: planning-readiness-gate.md §Edge Cases — "A domain name contains a path separator (e.g., `protocols/ws1`)" had no test. This is **genuinely resolved**:

- `state.workspace-inspection` gained the edge_case test: "A domain name containing a path separator, such as 'protocols/ws1', resolves to 'protocols/ws1/<workspace_dir>/' and that nested directory is the one inspected" — reproducing the spec's exact scenario and expected path, not a restatement of the general resolution-mechanism test that was already present.
- `notes/readiness-gate.md`'s domain-resolution section gained the paragraph: "A domain name may itself contain a path separator (spec §Edge Cases: `protocols/ws1`). It resolves to `protocols/ws1/<workspace_dir>/` and that nested directory is the one inspected — so the resolution must join path segments rather than treat the name as a single flat directory component," giving the implementer the correct implementation guidance (path-joining, not flat concatenation) to satisfy the new test.

No regression: the item's other tests (configured-vs-fallback resolution, empty-subdirectories-clean, dotfile-dirty, untraversable-with-OS-error) are all still present and unchanged.

## New Deficiency Found (Round 3)

While re-checking Interface/Outputs coverage for the `eval_mode: "direct"` re-entry work (`impl.direct-mode-reentry`, `ui.direct-mode-reentry`), a gap surfaced that rounds 1 and 2 did not catch: **the plan never wires the direct-mode EVALUATE self-loop's rendered output**, only its state transition.

batch-implementation.md's §Interface → Outputs gives a fully worked example, **"Entering EVALUATE (implementing phase, subsequent round, `eval_mode: "direct"`)"** (lines 228–246):

```
State:    EVALUATE
...
Round:    2/3
Note:     FAIL recorded for round 1. Corrections were made directly to batch files.
Items:
  ...
Action:   Please spawn 1 sonnet general-purpose sub-agent to evaluate and correct the batch.
          ...
```

and states explicitly: "Under `eval_mode: "direct"`, every round after the first re-enters EVALUATE in this same form — Round increments, a `Note:` line records the prior round's verdict, and no intervening IMPLEMENT state is presented." ui-batch-implementation.md's EVALUATE output section (line 163) defers to this same form with an added `Loop: code` line.

I verified in code that this rendering does not exist today, and is dead code that the fix must activate:
- `state/output.go`'s `printImplementingOutput`, `case StateEvaluate` (`output.go:1067-1100`) prints `State`, `Phase`, `Layer`, `Batch`, `Round`, `Items`, and the eval-entry `Action` — **no `Note:` line at all**, under any `eval_mode`.
- `state/output.go`'s `printUIImplementingOutput`, `case StateEvaluate` (`output.go:1401-1425`) is structurally identical — same omission.
- Both differ from the `StateImplement` cases (`output.go:1038`, `output.go:1374`), which *do* build a `"%s recorded for round %d."` note — but that pattern is IMPLEMENT-specific (it's followed by "Minimum rounds not yet met" text, not "Corrections were made directly to batch files.") and is only reached today because report/conversational mode re-enters IMPLEMENT, never EVALUATE.
- `grep` of `state/output_test.go` for "Corrections were made directly" or any EVALUATE-state Note assertion returns nothing — this text is untested today because the transition that would trigger it (EVALUATE→EVALUATE under direct mode) is exactly the bug being fixed.

This is not an oversight I can attribute to out-of-scope/already-implemented status: `notes/output-rendering.md`'s own "What this change touches" section enumerates exactly three touch points — "IMPLEMENT action text," "COMMIT state rendering," and "Terminal EVALUATE / E2E_VERIFY → ORIENT or DONE" — and omits the EVALUATE-state Note-line requirement entirely, so even the plan's supporting reference material never identified it.

Checking plan coverage directly:
- `impl.direct-mode-reentry` and `ui.direct-mode-reentry`'s `files` are `state/advance.go` / `state/advance_test.go` only — transition logic, no rendering.
- `output.commit-flow` (the item that touches `state/output.go`) depends on both direct-mode-reentry items (the dependency edge correctly anticipates that rendering work is needed), but its five steps and five tests cover only: `--message` optional at IMPLEMENT, the unconditional spec-review reminder, the COMMIT `enable_commits: true` arm removal, the terminal-transition inline-commit report, and the ORIENT rendering not implying an outstanding commit. None of the steps or tests touch the EVALUATE state's `Round:`/`Note:` rendering for a direct-mode self-loop re-entry.

Consequently no plan item would produce the spec-mandated output text, and no test in the plan would catch its absence if the state-machine fix were implemented without it — an engineer following `impl.direct-mode-reentry` and `output.commit-flow` verbatim would ship a direct-mode EVALUATE re-entry that transitions correctly (satisfying the `advance.go` tests) but renders with no `Note:` line and a `Round:` display that was never validated against the "2/3" worked example.

This single gap fails three dimensions, detailed below.

## Dimension Results

### 1. Behavior — FAIL
Every other Behavior-section step for the new work has plan coverage (readiness gate's Readiness Query and Enforcement procedures fully reproduced in `state.readiness-gate`/`gate.enforce-init`/`gate.enforce-phase-shift`; batch-implementation/ui-batch-implementation state-machine deltas reproduced in `impl.*`/`ui.*` items). However, the direct-mode EVALUATE self-loop's rendering behavior — an explicit, worked part of batch-implementation.md's Behavior-adjacent Outputs — has no corresponding plan item or step (see New Deficiency above).

### 2. Error Handling — PASS
The Error Handling subsections under planning-readiness-gate.md's three Behavior blocks are covered: unreadable/invalid plan queue (`cmd.preflight` rejection tests), dirty-domain abort as rejection-not-crash (`gate.enforce-init`/`gate.enforce-phase-shift`), and untraversable-workspace-as-dirty-with-OS-detail (`state.workspace-inspection` edge_case, `state.readiness-gate` edge_case).

### 3. Rejection — PASS
All five planning-readiness-gate.md Rejection rows map to tests (`gate.enforce-init`, `gate.enforce-phase-shift`, `cmd.preflight`). session-init.md's and phase-transitions.md's readiness-gate rejection rows map to the same items. batch-implementation.md and ui-batch-implementation.md's Rejection tables contain no rows for the delta that lack coverage — the "first-round `--message` required" rejection no longer exists in either spec (both now describe `--message` as always-optional), and the plan's tests correctly assert the *absence* of that requirement (`impl.inline-terminal-commit`, `ui.inline-terminal-commit`, `impl.per-item-commit`, `ui.per-item-commit`).

### 4. Interface — FAIL
`state.workspace-inspection` defines the Domain Workspace Status fields exactly per spec; `state.readiness-gate`'s READY/BLOCKED strings are reproduced verbatim in its steps (with the literal per-domain line format pinned down in the referenced `notes/readiness-gate.md`, an acceptable location since it's a validated `refs` entry — carried over as a non-blocking observation from rounds 1–2, same as the `dirty_domains` field-naming observation). The `preflight` CLI interface, `AutoCommit`'s unchanged signature, and the message-synthesis helper contracts all match their spec definitions. However, the §Interface → Outputs worked example for "Entering EVALUATE (implementing phase, subsequent round, `eval_mode: "direct"`)" — and its ui-batch-implementation.md analog — has no plan item that produces it, per the New Deficiency above. This is a concrete, spec-printed output format with no corresponding coverage, not a minor documentation gap.

### 5. Configuration — PASS
The only new-work configuration touchpoint, `paths.workspace_dir`, is handled in `state.workspace-inspection` with a dedicated non-default-value test. No new config keys are introduced elsewhere, matching the specs. `ui_implementing.*` config table entries are pre-existing and correctly excluded.

### 6. Observability — PASS
`gate.logging` wires the INFO (domain count + verdict), ERROR (dirty domains + workspace paths, in addition to INFO only when blocked), and DEBUG (per-domain path + clean/dirty) log entries exactly per planning-readiness-gate.md §Observability, routed through the existing `state.Logger`/`LogEntry` mechanism with no new sink, and correctly relying on `NewLogger`'s existing no-session no-op for `preflight`. ui-batch-implementation.md's Observability section describes generic per-transition logging unaffected by the inline-commit/direct-mode branching changes (the logging wrapper fires per `advance` call regardless of internal branch); no plan item needed to touch it, and none does.

### 7. Integration Points — PASS
`gate.enforce-init` and `gate.enforce-phase-shift` depend on `state.readiness-gate`; `cmd.preflight` depends on `state.readiness-gate`; `gate.logging` depends on both enforcement items plus `state.readiness-gate`. `output.commit-flow`'s dependency edges correctly include `impl.per-item-commit`/`ui.per-item-commit` alongside the inline-commit and direct-mode-reentry items (the edges anticipate all the rendering work that should follow from those items — even though, per the New Deficiency, the direct-mode-reentry edge's content was not fully honored). `docs.sync` depends on every rendering/gate item it documents.

### 8. Invariants — PASS
batch-implementation.md invariants 6/7/12/14 and their ui-batch-implementation.md analogs (12/17/18/19) all have enforcement mechanisms in the corresponding plan items — invariant 12/18 ("direct-mode eval loop skips re-implementation") is specifically about the *state transition* skipping IMPLEMENT, which `impl.direct-mode-reentry`/`ui.direct-mode-reentry` do enforce; the invariant text does not itself mandate the rendered Note line (that's an Interface/Outputs requirement, scored above), so this dimension is not doubly failed by the same gap. planning-readiness-gate.md invariants 1–5 map to `state.readiness-gate`/`gate.enforce-init`/`gate.enforce-phase-shift`/`cmd.preflight`.

### 9. Edge Cases — PASS
All edge cases from rounds 1 and 2 remain resolved (dirty-workspace-outside-queue, mid-cycle-foreign-write-into-not-yet-planned-domain, and now the path-separator domain name). No new edge case gap was found in this round's re-read of planning-readiness-gate.md, batch-implementation.md, or ui-batch-implementation.md §Edge Cases sections.

### 10. Testing Criteria — FAIL
All 13 planning-readiness-gate.md Testing Criteria entries map to tests. All batch-implementation.md and ui-batch-implementation.md Testing Criteria entries relevant to the delta map to tests **at the state-transition level**, including "EVALUATE FAIL within max_rounds → EVALUATE (direct)" (covered by `impl.direct-mode-reentry`'s tests) and "Direct-mode batch never returns to IMPLEMENT after round 1" (covered by both `impl.direct-mode-reentry` and `ui.direct-mode-reentry`'s edge_case tests). However, no test anywhere in the plan verifies the *rendered output* of a direct-mode EVALUATE re-entry — the `Round: 2/3` display and the `Note: FAIL recorded for round 1. Corrections were made directly to batch files.` line that batch-implementation.md's worked example requires verbatim. `output.commit-flow`'s test list has no entry for it. This is exactly the category of gap the instructions flag: "test criteria specific enough to evaluate unambiguously ... especially direct-mode loop termination" — the *termination* (state machine) is tested; the *rendering* of that termination path's intermediate rounds is not.

### 11. Dependencies & Format — PASS
All 15 items appear in exactly one layer (L0: 2, L1: 4, L2: 7, L3: 2 = 15). All `depends_on` IDs resolve to existing items; no item depends on a later-layer item; same-layer dependencies are legal per `plan-format.json`. No cycles. All test categories are valid (`functional`/`rejection`/`edge_case`). All `refs` paths (top-level and item-level) exist on disk — verified against the five notes files and six specs. Top-level shape matches `plan-format.json` (`context`/`refs`/`layers`/`items` only). One non-blocking observation, related to but distinct from the New Deficiency: `output.commit-flow`'s dependency edges on `impl.direct-mode-reentry`/`ui.direct-mode-reentry` correctly signal that rendering work follows from those items, but the edge is not backed by matching step/test content for the EVALUATE-state rendering — the DAG is legal, but in this one place an edge implies more content coverage than the item actually delivers.

## Deficiency List (FAIL only)

| # | Dimension | Spec Section | Missing Coverage |
|---|-----------|-------------|-----------------|
| 1 | Behavior | batch-implementation.md §Interface → Outputs, "Entering EVALUATE (implementing phase, subsequent round, `eval_mode: "direct"`)" (and the analogous line in ui-batch-implementation.md's EVALUATE output section) | No plan item renders the `Round:`/`Note:` output for a direct-mode EVALUATE self-loop re-entry. `printImplementingOutput`'s and `printUIImplementingOutput`'s `case StateEvaluate` (`state/output.go:1067-1100`, `1401-1425`) currently emit no `Note:` line under any `eval_mode`; this is dead code today because direct mode never re-enters EVALUATE prior to this fix. `impl.direct-mode-reentry`/`ui.direct-mode-reentry` touch only `state/advance.go`; `output.commit-flow` touches `state/output.go` but its steps never mention this rendering. `notes/output-rendering.md`'s "What this change touches" section omits it as well. |
| 2 | Interface | Same spec sections as above | The exact output text specified by the spec's worked example (`Round: 2/3`, `Note: FAIL recorded for round 1. Corrections were made directly to batch files.`) has no corresponding plan coverage — not the `Round` display, not the `Note` line, for either the implementing or ui_implementing phase. |
| 3 | Testing Criteria | Same spec sections as above | No test in `output.commit-flow` (or anywhere else in the plan) verifies the rendered `Round:`/`Note:` text for a direct-mode EVALUATE self-loop re-entry, distinct from the already-covered state-transition tests in `impl.direct-mode-reentry`/`ui.direct-mode-reentry`. |
