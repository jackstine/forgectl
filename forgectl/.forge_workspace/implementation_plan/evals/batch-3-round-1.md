# Evaluation Report

**Round:** 1
**Batch:** 3
**Layer:** L1 Implementing Commit Flow & Gate Evaluation

VERDICT: PASS

## Items Evaluated

### [impl.direct-mode-reentry] Re-enter EVALUATE directly under eval_mode direct

**Files reviewed:** forgectl/state/advance.go, forgectl/state/advance_test.go

`advanceImplFromEvaluate` (advance.go:787-852) delegates both non-terminal branches — PASS below `min_rounds` (line 834-835) and FAIL below `max_rounds` (line 847-848) — to the new helper `reenterImplEvaluationLoop` (advance.go:866-874). That helper resolves the mode via `EvalModeFor(s.Config.Implementing.Eval, s.Config.General)`: under `"direct"` it increments `batch.EvalRound` and sets `s.State = StateEvaluate` without touching `CurrentItemIndex`; otherwise it resets `CurrentItemIndex = 0` and sets `s.State = StateImplement`, preserving prior behavior for `"report"`/`"conversational"`. `batch` is `impl.CurrentBatch`, a `*BatchState` (types.go:617), so the mutation is retained on the state struct.

#### Test Results

- [PASS] Under eval_mode direct, a FAIL below max_rounds transitions EVALUATE to EVALUATE rather than to IMPLEMENT — `TestDirectModeFailReentersEvaluate` (advance_test.go:4026-4045); asserts `s.State == StateEvaluate` and `EvalRound == roundBefore+1`.
- [PASS] Under eval_mode direct, a PASS below min_rounds transitions EVALUATE to EVALUATE rather than to IMPLEMENT — `TestDirectModePassBelowMinRoundsReentersEvaluate` (advance_test.go:4047-4067); `MinRounds=2`, verdict PASS at round 1, asserts EVALUATE + incremented round.
- [PASS] Under eval_mode report and under conversational, a FAIL below max_rounds still transitions EVALUATE to IMPLEMENT with the item index reset to 0 — `TestReportAndConversationalModesReenterImplement` (advance_test.go:4069-4097), parametrized over both modes; asserts `StateImplement` and `CurrentItemIndex == 0`.
- [PASS] Under eval_mode direct, IMPLEMENT is entered exactly once for the batch across the whole evaluation loop — `TestDirectModeEntersImplementExactlyOncePerBatch` (advance_test.go:4099-4126); drives a 2-item batch through repeated FAIL verdicts to force-acceptance and counts IMPLEMENT visits (== 2, i.e. once per item, never re-presented).
- [PASS] A direct-mode batch that FAILs every round terminates: the eval round increments on each re-entry, reaches max_rounds, and force-accepts rather than looping indefinitely — `TestDirectModeFailingBatchTerminatesAtMaxRounds` (advance_test.go:4128-4163); bounded loop (10 iterations) terminates at exactly `rounds == 3 == MaxRounds`, lands in `StateCommit`, and the item is marked `"failed"` (force-accept).

#### Notes

- The round-increment placement matches the spec's stated hazard precisely: IMPLEMENT→EVALUATE increments `EvalRound` once per batch pass (advance.go:780), and the direct-mode self-transition reproduces that increment (advance.go:868) so the round counter still climbs toward `max_rounds` even though IMPLEMENT is never re-entered. Verified this is not double-counted — the increment happens only in `reenterImplEvaluationLoop`, not also somewhere in the direct branch of `advanceImplFromEvaluate`.
- A sibling item, `ui.direct-mode-reentry` (plan.json), applies the equivalent fix to `advanceUIFromEvaluate`'s non-terminal branch and depends on this item; it is `pending` and correctly out of scope for this batch — `advanceUIFromEvaluate` (advance.go:1061-1091) still unconditionally resets to IMPLEMENT, uncovered by any direct-mode UI test, which is expected given it is a separate, not-yet-implemented plan item rather than a regression introduced here.
- No existing test was found asserting the old unconditional-IMPLEMENT-reentry behavior while accidentally being exercised under `eval_mode: "direct"` — every test driving this transition sets `eval_mode` explicitly, and the two integration tests that assert unconditional IMPLEMENT reentry (`TestImplementingMinRoundsLoops`, `TestC4EvalRoundResetPerBatch`) never set `eval_mode`, so `EvalModeFor` resolves to `"conversational"` by config default — a correct, not coincidental, pass.

### [state.readiness-gate] Evaluate and report planning readiness

**Files reviewed:** forgectl/state/readiness.go, forgectl/state/readiness_test.go

This item's scope is the shared evaluation engine only (per its description: "the shared evaluation both preflight and the three cold-start enforcement points call") — `preflight` and the three cold-start enforcement points are separate, not-yet-implemented plan items (`cmd.preflight` in layer L2, confirmed absent by `grep -rn "preflight" forgectl/cmd forgectl/state` returning no hits outside readiness.go's doc comment and the spec). No integration was expected or required here.

`DomainPath`/`DomainWorkspacePath` (readiness.go:44-65) resolve a domain to `<domain-path>/<workspace_dir>/`, falling back to the bare name when undeclared and honoring a path-separator-bearing domain name via `filepath.Join`. `InspectDomainWorkspace` (readiness.go:82-137) treats an absent directory as clean, walks existing ones with `filepath.WalkDir`, short-circuits on the first regular file via `filepath.SkipAll`, and reports a traversal error (with the OS error string) as dirty rather than a false clean. `QueueDomains` (readiness.go:161-172) de-duplicates by first occurrence, preserving queue order. `EvaluateReadiness` (readiness.go:177-188) only inspects the domains it is given — a dirty workspace for a domain outside that list is never touched. `Render` (readiness.go:194-226) reproduces the spec's exact READY/BLOCKED text.

#### Test Results

- [PASS] A queue whose domains all have clean workspaces yields a ready verdict and the READY output naming the count of inspected workspaces — `TestEvaluateReadinessAllCleanIsReady` (readiness_test.go:318-338); asserts `Ready`, 0 dirty, 3 statuses, and exact text `"Planning readiness: READY\nInspected 3 domain workspaces — all clean."`.
- [PASS] A queue with two dirty domains yields a blocked verdict whose output lists both domains with their workspace paths and the close-out remediation instruction — `TestEvaluateReadinessTwoDirtyDomainsBlockWithRemediation` (readiness_test.go:340-365); asserts the exact BLOCKED text byte-for-byte, matching the spec's literal example (verified independently against `specs/planning-readiness-gate.md:117-123`, including the column-padding: 3 spaces after `protocols` (9 chars) and 4 after `launcher` (8 chars, padded to width 9) — both derived from the same `%-*s` width computed from the longest dirty-domain name).
- [PASS] The BLOCKED output omits clean domains, listing only the dirty ones — `TestEvaluateReadinessBlockedOutputOmitsCleanDomains` (readiness_test.go:367-391); only `launcher` dirty among three domains, asserts `protocols`/`portal` are absent from rendered output.
- [PASS] One dirty domain among otherwise clean domains is enough to block the verdict — same test as above; asserts `v.Ready == false`.
- [PASS] A plan queue with several entries naming the same domain inspects that workspace once and lists it once in the output — `TestQueueDomainsDeduplicatesPreservingOrder` (readiness_test.go:393-417); three entries with `protocols` repeated, asserts de-duplicated order `[protocols, launcher]`, 2 statuses inspected, and `"- protocols"` appears exactly once in rendered output.
- [PASS] A domain that is dirty because its workspace could not be traversed reports the OS error detail alongside the domain — `TestEvaluateReadinessUntraversableDomainReportsErrorDetail` (readiness_test.go:419-448) and the lower-level `TestInspectDomainWorkspaceUntraversableIsDirtyWithError` (readiness_test.go:203-234); a chmod-0000 subdirectory produces `Clean == false`, a non-empty `Error` naming the offending path, and `"could not inspect:"` in the rendered BLOCKED output. Both tests correctly skip on Windows and when running as root, where permission bits don't block traversal.
- [PASS] Evaluating readiness twice over an unchanged filesystem returns the identical verdict, confirming the gate holds no state between calls — `TestEvaluateReadinessIsStatelessAcrossCalls` (readiness_test.go:473-494); two calls compared on `Ready`, rendered text, and dirty-domain count.
- [PASS] Domain 'a' has a full workspace but the plan queue lists only domain 'b' ... verdict is READY and domain 'a' is never inspected — `TestEvaluateReadinessIgnoresDomainsOutsideTheQueue` (readiness_test.go:450-471); asserts `Ready`, that no status names domain `a`, and the exact singular-grammar READY text for a 1-domain queue.

#### Notes

- Cross-checked the BLOCKED-output column alignment against the spec's literal example text (`cat -e` on `specs/planning-readiness-gate.md:117-123`) rather than trusting the test's expected string alone — the implementation's padding-by-longest-dirty-domain-name algorithm reproduces the spec's example exactly, including the differing space counts for the two differently-sized domain names in that example.
- `InspectDomainWorkspace` additionally handles the case where the workspace path exists but is not a directory (readiness.go:106-112), which is not explicitly called out in the spec's Workspace Cleanliness Determination steps but is a reasonable, non-contradicting extension (reported dirty with detail rather than panicking or silently treating it as clean); not spec-mandated but not a deviation either, and untested by a dedicated unit test — non-blocking.
- Supporting `state`-package tests (`TestInspectDomainWorkspace*`, 10 tests at readiness_test.go:37-297) independently cover the underlying cleanliness rule (absent, top-level file, nested file, configured `workspace_dir`, configured domain path, path-separator domain name, empty-subdirectory tree, dotfile-only, untraversable, and a write-nothing check via before/after tree snapshot) — all pass and reinforce the eight declared acceptance criteria above rather than duplicating them.

## Summary

Both items are correctly and completely implemented against their specs. `impl.direct-mode-reentry` precisely follows the spec's stated non-terminal-branch behavior for `eval_mode: "direct"` (self-transition to EVALUATE with round increment, no item-index reset) while leaving `"report"`/`"conversational"` untouched, and its round-increment relocation is exactly what prevents the infinite-loop hazard the spec and notes call out. `state.readiness-gate` reproduces the spec's READY/BLOCKED output byte-for-byte (verified independently against the spec text), correctly de-duplicates and order-preserves the domain set, scopes inspection strictly to the incoming queue, retains OS error detail on traversal failure, and is verifiably stateless. `go build`, `go vet`, `go test ./...`, and `go test -tags integration ./integration/` all pass; the sole integration failure (`TestSkillContractSubcommands/preflight`) is the expected, out-of-scope `cmd.preflight` absence. No stale-passing regression tests were found for either item, and the ui_implementing equivalent of the direct-mode fix is correctly deferred to its own separate, dependent, still-pending plan item (`ui.direct-mode-reentry`) rather than being silently dropped from this batch.
