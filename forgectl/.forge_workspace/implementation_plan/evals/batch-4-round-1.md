# Evaluation Report

**Round:** 1
**Batch:** 4
**Layer:** L2 UI Phase Parity & Gate Wiring

VERDICT: PASS

## Items Evaluated

### [ui.per-item-commit] Commit each ui_implementing item on every round

**Files reviewed:** forgectl/state/advance.go (advanceUIFromImplement, lines 1011-1057), forgectl/state/advance_test.go

#### Test Results

- [PASS] With enable_commits true, advancing out of ui IMPLEMENT on a post-first round produces a commit — `advanceUIFromImplement` calls `AutoCommit` unconditionally under `EnableCommits`, with no `batch.EvalRound == 0` guard. Verified by `TestUIImplementCommitsOnEveryRound` (advance_test.go:4185), which drives a FAIL round back to IMPLEMENT and asserts a second commit lands.
- [PASS] The ui per-item commit message is the item's description when no --message is supplied — `ItemCommitMessage(plan, itemID)` passed through `AppendSuppliedMessage`. Verified by `TestUIImplementCommitMessageIsItemDescription` (advance_test.go:4224), which asserts the commit message equals the plan item's `desc a` description.
- [PASS] Advancing out of ui IMPLEMENT with enable_commits true and no --message succeeds rather than erroring — the `--message` requirement was removed; no such check exists in `advanceUIFromImplement`. Verified by `TestUIImplementDoesNotRequireMessage` (advance_test.go:4242), which drives a 2-item batch with no `--message` on any round.

#### Notes
The implementation matches the reference notes (commit-flow.md) exactly: commit-then-save ordering preserved (AutoCommit before savePlan), staging via `effectiveUIStrategy`/`uiScopeTargets` with the `nil`-item batch-terminal convention reused elsewhere.

### [ui.direct-mode-reentry] Re-enter the ui code-eval loop directly under eval_mode direct

**Files reviewed:** forgectl/state/advance.go (advanceUIFromEvaluate, lines 1059-1103), forgectl/state/advance_test.go

#### Test Results

- [PASS] Under ui eval_mode direct, a code-eval FAIL below max_rounds transitions EVALUATE to EVALUATE rather than to IMPLEMENT — confirmed in code (the `direct` branch sets `StateEvaluate` and increments `EvalRound`) and by `TestUIDirectModeCodeEvalFailReentersEvaluate` (advance_test.go:4267).
- [PASS] Under ui eval_mode report, a code-eval FAIL below max_rounds still transitions EVALUATE to IMPLEMENT — verified by `TestUIReportModeCodeEvalFailReentersImplement` (advance_test.go:4289), which asserts state is IMPLEMENT with `CurrentItemIndex` reset to 0.
- [PASS] QA_TEST below its budget still transitions to UI_REFINE, and E2E_VERIFY below its budget still transitions to E2E_REMEDIATE, under every eval_mode — `advanceUIFromQATest` and `advanceUIFromE2EVerify` are untouched by the direct-mode branch (confirmed by inspection); verified across all three eval_mode values by `TestUIQAAndE2ELoopsIterateUnderEveryEvalMode` (advance_test.go:4314).
- [PASS] A direct-mode code-eval loop that FAILs every round reaches max_rounds, force-accepts, and proceeds into the QA loop rather than looping indefinitely — verified by `TestUIDirectModeCodeEvalTerminatesAndReachesQA` (advance_test.go:4350), which bounds the loop at 10 iterations, asserts termination at exactly 3 rounds (max_rounds), transition to QA_TEST, and `CodeForceAccepted` set.

#### Notes
The `EvalRound++` increment inside the direct-mode branch is present and is the detail flagged in commit-flow.md as "the single easiest thing to get wrong" — without it the loop would never reach max_rounds. Confirmed correct and covered by the termination test above.

### [cmd.preflight] Add the preflight command

**Files reviewed:** forgectl/cmd/preflight.go, forgectl/cmd/preflight_test.go, forgectl/state/readiness.go, forgectl/integration/cli_surface_test.go

#### Test Results

- [PASS] preflight --from over a queue whose domains are all clean prints the READY output and exits 0 — `TestPreflightFromAllCleanIsReady`.
- [PASS] preflight --from over a queue with a dirty domain prints the BLOCKED output listing that domain and exits non-zero — `TestPreflightFromDirtyDomainIsBlocked` (also asserts the clean domain is NOT listed, and that blocked output goes out via the error, not stdout).
- [PASS] preflight with no --from uses the plan queue recorded in the active session's generate_planning_queue state — `TestPreflightWithoutFromUsesSessionPlanQueue`.
- [PASS] preflight --from succeeds in a project with no state file at all — `TestPreflightFromWorksWithoutASession`; `resolvePreflightQueue` falls back to cwd + default config rather than requiring `state.Scaffold`.
- [PASS] preflight --from naming a nonexistent file errors with the path and exits non-zero — `TestPreflightFromNonexistentFileErrors`.
- [PASS] preflight --from naming a file that is not a valid plan queue errors with the validation detail and exits non-zero — `TestPreflightFromInvalidQueueErrors`.
- [PASS] preflight with no --from and no active session errors with the exact 'No plan queue to check' message and exits non-zero — `TestPreflightWithoutFromAndNoSessionErrors`; message matches `noPlanQueueMessage` constant verbatim.
- [PASS] preflight with no --from and an active session that has no pending plan queue errors with the exact message — `TestPreflightWithoutFromAndNoPendingQueueErrors`, table-driven over both "state absent" and "path empty" sub-cases.
- [PASS] preflight error paths write to stderr rather than stdout — `TestPreflightErrorPathsWriteNothingToStdout` (unit) and CLI-surface paths 5-7 (integration, cli_surface_test.go).
- [PASS] Running preflight leaves the filesystem unchanged on both ready and blocked paths — `TestPreflightLeavesFilesystemUnchanged`, which snapshots the directory tree (path, mode, size) before/after and asserts no diff, plus asserts no state file was created.

#### Notes
`InspectDomainWorkspace` (readiness.go) correctly implements the cleanliness rule: absent directory or only-empty-subdirectories is clean; any regular file (including dotfiles) is dirty; unreadable directories are dirty-with-error via `filepath.WalkDir`'s error callback. `Render()` reproduces the spec's READY/BLOCKED output format exactly, including the singular/plural "domain workspace(s)" wording.

### [gate.enforce-init] Enforce the readiness gate at init --phase planning

**Files reviewed:** forgectl/cmd/init.go (lines 144-190), forgectl/cmd/commands_test.go

#### Test Results

- [PASS] init --phase planning with all incoming domain workspaces clean proceeds and creates the state file as before — `TestInitPlanningWithCleanWorkspacesProceeds`.
- [PASS] init --phase planning with a dirty incoming domain workspace fails with the dirty-domain list and close-out remediation, and exits non-zero — `TestInitPlanningBlockedByDirtyWorkspace`.
- [PASS] A blocked init --phase planning writes no state file — `TestInitPlanningBlockedWritesNoStateFile`, which also asserts the state directory has zero entries afterward.
- [PASS] init --phase implementing and init --phase specifying are unaffected by a dirty domain workspace — `TestInitNonPlanningPhasesAreUngated`, table-driven over both phases.
- [PASS] A plan queue naming several domains fails init when any one of them is dirty, and names every dirty domain rather than only the first — `TestInitPlanningNamesEveryDirtyDomain`, three domains with two dirty, asserts both dirty paths present and the clean one absent.

#### Notes
The gate check sits after `ValidatePlanQueue`/unmarshal and before `s.Planning = state.NewPlanningState(...)` — i.e., before any state mutation, matching the spec's ordering requirement. The verdict text is printed to `cmd.OutOrStdout()` and a short generic error is returned for the exit code — this mirrors the pre-existing validation-failure pattern already used earlier in the same function (schema validation errors), so it is consistent with established convention rather than a deviation.

### [gate.enforce-phase-shift] Enforce the readiness gate at the cold-start phase shifts into planning

**Files reviewed:** forgectl/state/advance.go (lines 1388-1467, 1544-1611), forgectl/state/advance_test.go

#### Test Results

- [PASS] The generate_planning_queue to planning shift proceeds to planning ORIENT when every incoming domain workspace is clean — `TestPhaseShiftGenqueueToPlanningPassesGateWhenClean`.
- [PASS] The generate_planning_queue to planning shift with a dirty incoming domain prints the dirty domains and remediation, does not transition, and exits non-zero — `TestPhaseShiftGenqueueToPlanningBlockedByDirtyDomain`, asserts a `*ReadinessError` and message content.
- [PASS] A blocked phase shift leaves the session at its pre-planning phase and state with the plan queue unpopulated — `TestPhaseShiftBlockedLeavesSessionUntouched`, asserts phase/state/`PhaseShift` survive and `s.Planning` stays nil.
- [PASS] The specifying to generate_planning_queue shift with --from is blocked by a dirty incoming domain — `TestPhaseShiftSpecifyingFromSkipBlockedByDirtyDomain`.
- [PASS] The planning to planning domain-boundary shift reaches planning ORIENT even though sibling domain workspaces now hold completed plan.json files — `TestPhaseShiftPlanningToPlanningIsUngated`.
- [PASS] The implementing to planning and ui_implementing to planning re-entries reach planning ORIENT with a non-empty workspace for the just-implemented domain — `TestPhaseShiftImplementingToPlanningIsUngated` and `TestPhaseShiftUIImplementingToPlanningIsUngated`.
- [PASS] A multi-domain interleaved session runs planning through implementing and back to planning for every domain without the gate blocking any domain boundary after the cold start — `TestMultiDomainInterleavedRunGatesOnlyAtColdStart`, three-domain queue, cold start once then two boundary crossings each with a freshly-dirtied prior domain.
- [PASS] Queue [a, b] clean at cold start; a foreign write into b's workspace before the cycle reaches it does not block the boundary into b — `TestForeignWriteIntoUnplannedDomainDoesNotBlockBoundary`.

#### Notes
`EvaluateReadiness` is called in exactly the two gated case blocks (`PhaseSpecifying→PhaseGeneratePlanningQueue` `--from` branch, and `PhaseGeneratePlanningQueue→PhasePlanning`), both after validation/unmarshal and before any state field is mutated, returning `*ReadinessError` which subsequent code never reaches. The three exempt case blocks (`planning→planning`, `implementing→implementing`... actually `implementing/ui_implementing→planning`) contain no `EvaluateReadiness` call, confirmed by inspection — this is the primary regression risk called out in the reference notes, and it is correctly avoided.

## Summary

All five items are fully and correctly implemented. Every listed acceptance test has a corresponding, correctly-targeted unit or integration test, and the implementation matches both the specs and the detailed reference notes (commit-flow.md, readiness-gate.md, testing.md). `go test -count=1 ./...` and `go test -tags=integration -count=1 ./integration/...` both pass with no failures. No stubs, partial implementations, or missing edge-case coverage were found.
