# Implementation Log

Log of implementation updates across sessions.

---

## Entries

### 2026-06-20 — Fix AutoCommit running git commands from wrong directory

- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`, pre-existing `TestEvalContextReportModeSections` failure unrelated to this change)
- **Notes:** `AutoCommit` in `state/git.go` used `git -C projectRoot` for all operations, but `projectRoot` (where `.forgectl/` lives) can be a subdirectory of the git repository root — causing relative stage targets to resolve against the wrong base. Fix: call `GitRepoRoot(projectRoot)` first, convert each stage target to an absolute path via `filepath.Join(projectRoot, t)`, then run all git operations with `-C gitRoot`. This is a no-op when `projectRoot == gitRoot` (the common case) and correct when it isn't. Added `TestAutoCommitScopedFromSubdirectory` to verify the subdirectory case. Updated `docs/auto-committing.md` Git Operations Sequence to document the git root resolution step and explain why git commands must run from the repository root (not from a domain subdirectory).

### 2026-06-20 — Switch eval/refine sub-agents from opus to sonnet general-purpose
- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`)
- **Notes:** Applied spec changes across batch-implementation.md, plan-production.md, spec-lifecycle.md, spec-reconciliation.md, and state-persistence.md. All eval spawn points (specifying.eval, planning.eval, implementing.eval, specifying.cross_reference.eval, specifying.reconciliation) and planning.refine now default to model="sonnet" type="general-purpose". Updated: `state/types.go` DefaultForgeConfig(), `state/output.go` fallback defaults in StateCrossReferenceEval, both `state/default-config.toml` and `docs/default-config.toml` TOML templates, `docs/configurations.md` Default values and valid-values list (added "general-purpose"), and `state/types_config_test.go` round-trip and defaults test pins.

### 2026-06-19 — IMPLEMENT output: per-spec git-show Read command + review reminders
- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`)
- **Notes:** Implemented spec 236a69e (batch-implementation.md). In `state/output.go` `StateImplement` rendering: (1) under each `Specs:` entry, when the current plan's `spec_commits` is non-empty, emit an indented `Read: git show <commits> -- '**/<file>'` line — `<commits>` is the space-joined `spec_commits`, `<file>` is the spec entry with its `#anchor` stripped (`strings.Cut(spec, "#")`) and wrapped in a `'**/<file>'` pathspec glob. Uses `git show` (not `git log -p`) so output is bounded to exactly the named commits; the glob self-filters. When `spec_commits` is empty, the line is omitted. (2) Added `writeImplementReviewReminders(w, indent, hasSpecCommits, hasRefs)` helper, called in both IMPLEMENT action branches (first round and post-eval round 2+). Always emits the spec-review reminder — git-command variant when spec_commits present, "read the spec file(s) listed above" fallback when empty — and the Refs-review reminder iff the item has `Refs`. Added 5 tests to `state/output_test.go` (`implSpecReadState` helper + per-criterion tests). The `docs/diagrams/03-implementing-phase.txt` diagram was already updated as part of the spec commit, so no derived-doc work was needed.

### 2026-06-19 — git-root boundary in config-scaffolding ancestor walk
- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`)
- **Notes:** Implemented spec a424859 (git-root boundary for `FindProjectRoot`). Modified `state/config.go`: within the walk loop, after checking for `.forgectl/`, now also checks for `.git/` — if found without `.forgectl/`, returns "not found" immediately, preventing inheritance of a `.forgectl/` above the repository boundary. A `.forgectl/` co-located with `.git/` is still recognized (checked first). Outside a git repo, the walk continues to the filesystem root unchanged. Added three new tests to `state/scaffold_test.go`: `TestScaffoldStopsWalkAtGitRoot`, `TestScaffoldUsesForgetclColocatedWithGitDir`, `TestScaffoldFallsBackToFullWalkOutsideGitRepo` — covering all three new spec edge-case scenarios.

### 2026-06-19 — scaffold-only mode + --phase-without-from rejection
- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`)
- **Notes:** Implemented spec 64aca45 (scaffold-only mode for `forgectl init`). Changes to `cmd/init.go`: (1) Removed `MarkFlagRequired("from")` so bare `forgectl init` is valid. (2) Split run path into scaffold-only mode (`initFrom == ""`) and session mode. Scaffold-only runs `state.Scaffold(cwd)`, prints the fresh-default notice if config was written, then exits 0 — no state file created. (3) Added rejection: `--phase` provided without `--from` (`cmd.Flags().Changed("phase") && initFrom == ""`) → error "--from is required when --phase is set." Using `initFrom != ""` for `fromSet` (not `Changed("from")`) so existing direct-call tests remain valid; using `Changed("phase")` for `phaseSet` since only cobra-set flags trigger the new rejection. Added three new tests: `TestBareInitCreatesScaffolding`, `TestBareInitIsIdempotent`, `TestPhaseWithoutFromIsRejected`. The last test uses `initCmd.Flags().Set("phase", ...)` + cleanup of `.Changed` to simulate explicit flag passing without full cobra invocation.

### 2026-06-19 — config-scaffolding + session-init scaffolding
- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`)
- **Notes:** Implemented `config-scaffolding` spec (commit 2b73d36). New `state/Scaffold(cwd)` guarantees `.forgectl/` and `.forgectl/config` exist: reuses an ancestor `.forgectl/` directory if found, else creates one at cwd; writes the embedded default template (`state/default-config.toml`, a verbatim copy of `docs/default-config.toml`, drift-guarded by a test) atomically only when no config file exists; never reads/overwrites an existing config; partial-bootstrap is resumable. Wired into `init` (`cmd/init.go`) as the first step, replacing the old `resolveSession()` path; prints the fresh-default notice before any session output. Reconciled the documented toml-vs-`DefaultForgeConfig()` mismatch (spec invariant #3, defaults-equivalence): the template/`docs/configurations.md` were already canonical (`user_guided=true`, `specifying.batch=3`, `implementing.batch=2`, plus cross_reference/reconciliation/study_code/refine sub-agent configs); `DefaultForgeConfig()` was the stale/incomplete side and was completed to match. Updated the stale `TestDefaultForgeConfigValues` pin. Added `state/scaffold_test.go` (all 9 spec testing criteria) and `cmd` integration test `TestInitScaffoldsConfigWhenAbsent`. No derived-doc changes needed — docs already matched the template.
