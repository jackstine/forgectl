# Evaluation Report — Round 1

## Verdict: FAIL

## Summary
- Dimensions passed: 8/11
- Total spec requirements checked: covered the full genuinely-new-work delta across all six specs (per the scope context: `kind` routing, the ui_implementing state machine itself, `passes`/`rounds` mutation, all config validation, `AutoCommit` core, `handoff`/`eval` commands, and the already-rewritten docs were correctly treated as out of scope and excluded from the plan)
- Deficiencies: 5 (3 dimension-level: Observability, Edge Cases, Testing Criteria; plus 2 sub-findings folded into those)

Scope note: this evaluation follows the provided scope context and does **not** fault the plan for omitting already-implemented behavior (the `kind` field/routing, the ui_implementing loop machinery, `passes:"pending"`/`rounds:0` mutation, config validation trees, `AutoCommit`'s core staging/empty-commit handling, `handoff`/`eval`, or `docs/auto-committing.md` and `docs/schemas/plan-queue.md`). Code was spot-checked (`state/advance.go:749,803,960-1183,1280-1470`, `state/git.go:17-70`, `state/config.go:497-562`, `cmd/init.go:120-260`) and the plan's notes files accurately describe current behavior at every point checked.

## Dimension Results

### 1. Behavior — PASS
Every Behavior-section step for the new work (inline batch-terminal commit, per-round per-item commit, direct-mode re-entry, the readiness gate's Readiness Query and Enforcement-at-Planning-Entry procedures) has a corresponding plan item and step. `state.readiness-gate`'s steps reproduce the Readiness Query's 4 steps; `gate.enforce-init`/`gate.enforce-phase-shift` reproduce Enforcement's 4 steps, including "abort before any state mutation."

### 2. Error Handling — PASS
The `#### Error Handling` subsections under planning-readiness-gate.md's three Behavior blocks are covered: unreadable/invalid plan queue (`cmd.preflight` rejection tests), dirty-domain abort as rejection-not-crash (`gate.enforce-init`/`gate.enforce-phase-shift`), and untraversable-workspace-treated-as-dirty-with-OS-detail (`state.workspace-inspection` edge_case, `state.readiness-gate` edge_case).

### 3. Rejection — PASS
All five planning-readiness-gate.md Rejection rows map to tests across `gate.enforce-init`, `gate.enforce-phase-shift`, and `cmd.preflight`. The batch/ui-batch Rejection rows relevant to the delta (no `--message` required at any implementing/ui_implementing state when `enable_commits: true`) are explicitly tested in `impl.inline-terminal-commit`, `ui.inline-terminal-commit`, `impl.per-item-commit`, `ui.per-item-commit`.

### 4. Interface — PASS
`state.workspace-inspection` defines the Domain Workspace Status fields (`domain`, `workspace_path`, `clean`, `error`) exactly per spec. `state.readiness-gate`'s READY string is reproduced verbatim in its steps (`"Planning readiness: READY"` / `"Inspected N domain workspaces — all clean."`). The BLOCKED string's line-by-line structure is described in the plan item's steps but the literal per-domain line format (`  - protocols   protocols/.forge_workspace/`) is only pinned down in the referenced `notes/readiness-gate.md`, not restated in plan.json itself — acceptable since the note is a validated `refs` entry on the item, but noted as a minor precision gap.

### 5. Configuration — PASS
The only new-work configuration touchpoint, `paths.workspace_dir` (planning-readiness-gate.md §Configuration), is explicitly handled in `state.workspace-inspection` ("using cfg.Paths.WorkspaceDir rather than a hardcoded '.forge_workspace' literal") with a dedicated test for a non-default value. No new config keys are introduced elsewhere, consistent with the specs.

### 6. Observability — FAIL
planning-readiness-gate.md §Observability requires:
- INFO: "Planning readiness evaluated: the number of domains inspected and the verdict (ready/blocked)."
- ERROR: "Planning entry blocked by the gate: the list of dirty domains and their workspace paths."
- DEBUG: "Per-domain: the inspected workspace path and whether it was found clean or dirty."

This is genuinely new work — the gate does not exist today, so there is no existing logging to inherit. No plan item (`state.readiness-gate`, `gate.enforce-init`, `gate.enforce-phase-shift`) has a step or test wiring the verdict into `state.Logger`/`LogEntry`. I verified the existing generic logging (`cmd/advance.go:86-124`, `buildAdvanceDetail` at `cmd/advance.go:173-190`) captures only `verdict`, `eval_report`, and reverse-engineering `domain`/`round` — it does not capture domain-inspection counts or per-domain clean/dirty detail, so the specific INFO/DEBUG content is not satisfied incidentally by the generic mechanism. The spec's own Integration Points table names this explicitly: "activity-logging: When logging is enabled and a session is active, the gate result is recorded as a log entry at the cold-start planning-entry point." The plan omits this integration entirely.

### 7. Integration Points — PASS
Cross-spec relationships are reflected in the dependency graph: `gate.enforce-init` and `gate.enforce-phase-shift` both depend on `state.readiness-gate`; `cmd.preflight` depends on `state.readiness-gate`; `docs.sync` depends on the rendering/gate items it documents. `output.commit-flow` depends on both inline-terminal-commit items and both direct-mode-reentry items, matching batch-implementation/ui-batch-implementation's shared output concerns.

### 8. Invariants — PASS
Every relevant invariant has an enforcement mechanism: batch-implementation invariants 6/7/12/14 (COMMIT skip, per-round per-item commit, direct-mode skip, auto-commit synthesis) map to `impl.inline-terminal-commit`, `impl.per-item-commit`, `impl.direct-mode-reentry`, `git.message-synthesis`. The ui-batch analogs (12/17/18/19) map to the `ui.*` items. planning-readiness-gate invariants 1–5 (clean cold-start guarantee, read-only query, statelessness, no-partial-entry, uniform enforcement) map to `state.readiness-gate`, `cmd.preflight`, `gate.enforce-init`, `gate.enforce-phase-shift`.

### 9. Edge Cases — FAIL
Two planning-readiness-gate.md edge cases have no corresponding test anywhere in the plan:
1. **"A dirty workspace exists for a domain that is not in the incoming plan queue"** (§Edge Cases) — expected: ignored, does not block. No item or test in `state.readiness-gate` asserts that a domain outside the queue, even with a dirty workspace, is never inspected and never blocks the verdict. The closest existing test ("several entries naming the same domain inspects it once") tests de-duplication *within* the queue, not exclusion of domains *outside* it — a different property.
2. **"A domain in the incoming plan queue passes cold-start inspection, and something outside the scaffold then writes into that domain's workspace before the cycle reaches it"** (mid-cycle foreign write into a not-yet-planned domain) — expected: planned over, no re-inspection, no error. `gate.enforce-phase-shift`'s tests cover the *already-planned* domain (A) holding a full workspace at the boundary, but not the case of a *foreign write into a domain not yet reached* (B) — a distinct scenario the spec calls out separately because it demonstrates the cold-start-only scope of the guarantee, not just multi-domain continuation.

### 10. Testing Criteria — FAIL
1. planning-readiness-gate.md's own named Testing Criteria entry **"continuation queue ignores already-planned domains"** (domain `a` has a full workspace; queue lists only domain `b`; verdict READY; domain `a` never inspected) has no matching test in `state.readiness-gate`'s test list. This is the same gap as Edge Cases finding #1 above, but it is also independently named as a Testing Criteria entry, so it fails this dimension on its own terms.
2. ui-batch-implementation.md line 289 requires: "if nothing is left to stage, the commit is skipped silently" for the terminal E2E_VERIFY inline commit — the direct UI analog of the implementing-phase "nothing staged" rule. `impl.inline-terminal-commit` has an explicit test for this ("When per-item commits already captured every change, the inline batch commit finds nothing staged and is skipped silently"). `ui.inline-terminal-commit` has no equivalent test — its six tests cover PASS/FAIL/force-accept marking and the `--message`-not-required rejection, but never the empty-commit-skip path specific to the ui phase's terminal transition.
3. (Minor) planning-readiness-gate.md's Rejection-table condition "No `--from`, and no active session **or no pending plan queue**" is two distinct triggers; `cmd.preflight`'s test list only exercises "no active session," not "active session exists but has no pending plan queue" (e.g., `GeneratePlanningQueueState` nil or `PlanQueueFile` empty).

### 11. Dependencies & Format — PASS
All `depends_on` IDs resolve to existing items; every item appears in exactly one layer; no item depends on a later-layer item (verified L0→L3 topology by hand: L1 items depend only on L0; L2 items depend only on L0/L1/same-L2; L3 items depend only on L1/L2/same-L3). No dependency cycles. Test categories are all `functional`/`rejection`/`edge_case`. Plan format matches `plan-format.json` (context/refs/layers/items shape, no extraneous top-level fields).

One non-blocking observation: `output.commit-flow` depends on `impl.inline-terminal-commit`, `ui.inline-terminal-commit`, `impl.direct-mode-reentry`, `ui.direct-mode-reentry`, but not on `impl.per-item-commit`/`ui.per-item-commit` — even though its first step ("present `--message` as optional... drop first-round-only conditioning" in the IMPLEMENT rendering) is squarely about the per-item-commit items' behavior change, not the inline-terminal-commit or direct-mode items. This doesn't break DAG legality (both per-item-commit items sit in the earlier L1/L2), but the dependency edges don't fully reflect the item's actual content.

## Deficiency List (FAIL only)

| # | Dimension | Spec Section | Missing Coverage |
|---|-----------|-------------|-----------------|
| 1 | Observability | planning-readiness-gate.md §Observability | No plan item wires the readiness verdict into `state.Logger`: missing INFO (domain count + verdict), ERROR (dirty domain list at block), and DEBUG (per-domain path + clean/dirty) log entries at `state.readiness-gate`, `gate.enforce-init`, `gate.enforce-phase-shift`. Spec's own Integration Points table names activity-logging as an explicit consumer; the plan has no corresponding step or test. |
| 2 | Edge Cases | planning-readiness-gate.md §Edge Cases — "A dirty workspace exists for a domain that is not in the incoming plan queue" | No test in `state.readiness-gate` verifies a domain outside the plan queue is never inspected and never blocks the verdict, even if dirty. |
| 3 | Edge Cases | planning-readiness-gate.md §Edge Cases — mid-cycle foreign write into a not-yet-planned domain | No test in `gate.enforce-phase-shift` covers a write into domain B's workspace (not yet reached) while domain A is being worked — distinct from the "domain A now has a full workspace" scenario the plan does test. |
| 4 | Testing Criteria | planning-readiness-gate.md §Testing Criteria — "continuation queue ignores already-planned domains" | Same underlying gap as deficiency #2, but also independently named as a Testing Criteria entry with no matching test in `state.readiness-gate`. |
| 5 | Testing Criteria | ui-batch-implementation.md, line 289 (terminal E2E_VERIFY inline commit, "skipped silently if nothing is staged") | `ui.inline-terminal-commit` has no test for the empty-commit-skip path at the ui phase's terminal transition, unlike its implementing-phase counterpart (`impl.inline-terminal-commit`), which does test this. |
