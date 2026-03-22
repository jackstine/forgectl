# Implementation Log

Log of implementation updates across sessions.

---

## Entries

### 2026-03-22 (final review pass)
- **Errors:** None
- **All Tests Pass:** Yes
- **Changes:**
  - **output.go**: Fixed planning REFINE `Round:` display and eval file path. Was using `plan.Round - 1` (incorrectly offsetting the round number on rounds >= 2). Now uses `plan.Round` directly, matching specifying REFINE behavior and spec output format.
  - **output.go**: Fixed specifying REFINE eval file path. Was using `cs.Round - 1` for the eval report filename. Now uses `cs.Round` directly. The round number at REFINE time equals the eval that just occurred (increment happens on REFINE advance, not before).
  - **output.go**: Fixed implementing ORIENT action output alignment. Initial ORIENT (no current layer) now uses `Action:  ` (2-space padding) matching spec's narrower alignment. Non-initial ORIENT keeps `Action:   ` (3-space padding) matching spec's wider alignment for `Progress:` etc.
  - **output.go**: Fixed guided/non-guided action text for implementing ORIENT. When `user_guided` is false, the "Stop and review" line is omitted and the batch selection text becomes the `Action:` line directly.
  - **output.go**: Added singular/plural for "round" vs "rounds" in: implementing status item display, specifying status completed specs, planning status accepted display.
  - **output.go**: Planning status now always shows `--- Queue ---` section, printing `empty` when queue is empty (matching spec mid-planning example).
  - **output.go**: Removed `(started at <phase>)` annotation from status phase line. Spec only shows `(started here)` for direct-start phases, never shows `(started at ...)` for normal progression.
  - **advance.go**: Implementing DONE error message now includes trailing period: `"session complete."` (matching spec's quoted error string).
- **Known Gaps:**
  - Specifying ACCEPT output does not include `Commit:` line shown in spec. The spec implies auto-commit via git during EVALUATE->ACCEPT transition, but the implementation uses separate `add-commit`/`reconcile-commit` commands for commit tracking. Adding auto-commit would require git commit execution, which is a feature addition beyond the current scope.
  - Session archiving to `sessions/` directory is documented in spec but not automated by the scaffold. Archiving is a manual step.

### 2026-03-22 (review pass)
- **Errors:** None
- **All Tests Pass:** Yes
- **Changes:**
  - **advance.go**: Planning DRAFT now sets `Round=1` even when validation fails (DRAFT->VALIDATE). Previously round was only set when DRAFT->EVALUATE succeeded.
  - **advance.go**: Added `--eval-report` file existence check (`checkEvalReportExists`) to specifying phase EVALUATE. Planning and implementing already had this check.
  - **advance.go**: Added `--verdict PASS|FAIL` validation to planning EVALUATE, implementing EVALUATE, and RECONCILE_EVAL. Specifying EVALUATE already had this validation.
  - **advance.go**: Changed implementing DONE error message to `"session complete"` to match spec exactly.
  - **output.go**: RECONCILE state now prints `Domain:` line from first completed spec's domain.
  - **output.go**: Implementing EVALUATE output now shows `Batch: N/M` (with total) instead of just `Batch: N`.
  - **output.go**: Implementing COMMIT output now shows `Batch: N/M` and enhanced item status format (e.g., `failed (force-accept, 3/3 rounds)`).
  - **output.go**: Implementing ORIENT output now differentiates between "Selecting next batch", "Selecting first batch", and "Advancing to next layer" action text. Added force-accept notice with failed item names. Added terminal/failed item counting in progress display.
  - **output.go**: Implementing eval command output now shows `Batch: N/M`.
  - **output.go**: Status output now shows `(started here)` when started at current phase (non-specifying), matching spec format.
  - **output.go**: Completed spec status now uses `commit_hashes` array (falling back to `commit_hash` singular).
  - **init.go**: Implementing phase init now populates plan Name and Domain from plan.json context instead of leaving them empty.
  - **advance_test.go**: Added 4 new tests: VALIDATE re-failure stays VALIDATE, VALIDATE success to EVALUATE, eval report must exist (specifying), DRAFT sets round=1 on validation failure.

### 2026-03-22
- **Errors:** None
- **All Tests Pass:** Yes
- **Notes:** Initial implementation of forgectl scaffold CLI. All three phases (specifying, planning, implementing) implemented with state transitions, validation, output formatting, eval command, commit tracking, atomic state file writes with recovery. 50 tests passing across state and cmd packages.
