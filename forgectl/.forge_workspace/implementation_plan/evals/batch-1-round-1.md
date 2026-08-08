# Evaluation Report

**Round:** 1
**Batch:** 1
**Layer:** L0 Foundations

VERDICT: PASS

## Items Evaluated

### [git.message-synthesis] Synthesize commit messages from plan item descriptions

**Files reviewed:** `forgectl/state/git.go` (lines 74-133), `forgectl/state/git_test.go` (lines 271-384), `forgectl/specs/batch-implementation.md`, `forgectl/specs/ui-batch-implementation.md`, `docs/auto-committing.md` (lines 28-35, treated per `notes/commit-flow.md` as the already-rewritten target-behavior reference), `notes/commit-flow.md`

#### Test Results

- [PASS] The per-item message equals the plan item's description field exactly, with no prefix, suffix, or reformatting
  - `ItemCommitMessage` (`git.go:82-91`) returns `item.Description` verbatim, resolved via `findItem`. `TestItemCommitMessageIsDescriptionVerbatim` (`git_test.go:290-298`) pins this.
- [PASS] The batch-terminal message for a two-item batch is 'Batch <N>: <first description>; <second description>', preserving batch item order
  - `BatchCommitMessage` (`git.go:101-116`) iterates `itemIDs` in caller-supplied order and joins resolved descriptions with `"; "`, prefixed with `fmt.Sprintf("Batch %d: ", batchNumber)`. `TestBatchCommitMessageJoinsDescriptionsInBatchOrder` (`git_test.go:316-324`) confirms the exact string `"Batch 2: Load YAML, apply defaults, validate strictly; ServiceEndpoint and ServicesConfig structs"`.
- [PASS] Supplying --message text appends it as a second paragraph separated by a blank line; the synthesized first paragraph is unchanged and is not replaced
  - `AppendSuppliedMessage` (`git.go:123-133`) returns `synthesized + "\n\n" + supplied` when `supplied` is non-blank. `TestAppendSuppliedMessageAppendsAsSecondParagraph` (`git_test.go:358-370`) verifies the exact joined string and that the result still has the synthesized text as a prefix.
- [PASS] With no --message supplied, the returned message is the synthesized message alone, with no trailing blank line
  - When `supplied` trims to empty, `AppendSuppliedMessage` returns `synthesized` unchanged. `TestAppendSuppliedMessageWithoutSuppliedTextIsUnchanged` (`git_test.go:372-384`) checks this for `""`, `"   "`, and `"\n\t "`, and asserts no trailing newline.
- [PASS] A single-item batch produces a batch-terminal message with no trailing separator
  - `TestBatchCommitMessageSingleItemHasNoTrailingSeparator` (`git_test.go:326-337`) confirms `"Batch 1: ServiceEndpoint and ServicesConfig structs"` with no trailing `;`.

#### Notes

- Step 5 (findItem to avoid panics on a missing item ID) is honored in both `ItemCommitMessage` and `BatchCommitMessage`; a `nil` plan or an unresolvable ID falls back to the item ID (per-item) or `"Batch <N>"` (batch, when nothing resolves), covered by `TestItemCommitMessageMissingItemFallsBackToID` and `TestBatchCommitMessageSkipsUnresolvableItems`.
- `AutoCommit`'s signature (`git.go:17`) is untouched, matching step 4 — synthesis happens entirely in the new helpers, not inside `AutoCommit`.
- This item's declared `files` are limited to `git.go`/`git_test.go`; wiring these helpers into the `advance.go` call sites described in `notes/commit-flow.md` (dropping the `--message` requirement and the `EvalRound == 0` guard at IMPLEMENT, using the synthesized message at the terminal commit points) is not part of this item's steps or files and was correctly left undone — grep confirms `ItemCommitMessage`/`BatchCommitMessage`/`AppendSuppliedMessage` are not yet referenced anywhere in `advance.go`. This is expected for an L0 Foundations batch and not a deficiency of this item.

### [state.workspace-inspection] Inspect domain workspace cleanliness

**Files reviewed:** `forgectl/state/readiness.go`, `forgectl/state/readiness_test.go`, `forgectl/specs/planning-readiness-gate.md`, `notes/readiness-gate.md`

#### Test Results

- [PASS] A domain whose workspace directory does not exist is reported clean with a nil error
  - `InspectDomainWorkspace` (`readiness.go:93-99`) treats `os.IsNotExist` from `os.Lstat` as clean with `Error` left at its zero value (`""`). `TestInspectDomainWorkspaceAbsentDirectoryIsClean` passes.
- [PASS] A workspace containing a single regular file at the top level is reported dirty
  - `TestInspectDomainWorkspaceTopLevelFileIsDirty` passes; the `WalkDir` callback (`readiness.go:112-128`) sets `Clean = false` on the first `d.Type().IsRegular()` hit and also asserts `Error == ""` for a readable dirty workspace, which holds.
- [PASS] A workspace containing a regular file nested several directories deep is reported dirty
  - `TestInspectDomainWorkspaceNestedFileIsDirty` passes; the walk traverses full depth via `filepath.WalkDir`.
- [PASS] The reported workspace_path is project-root-relative and honors a non-default paths.workspace_dir value from config rather than a hardcoded .forge_workspace
  - `DomainWorkspacePath` (`readiness.go:57-63`) reads `cfg.Paths.WorkspaceDir`, defaulting only when empty. `TestInspectDomainWorkspaceHonorsConfiguredWorkspaceDir` confirms a `.forge_scratch` config value is honored and that a same-named default-dir sibling does not affect the result; also confirms the returned path is relative.
- [PASS] A domain name is resolved to its configured [[domains]] path; with no domains configured it falls back to the domain name as the path
  - `DomainPath` (`readiness.go:42-49`) matches `cfg.Domains` by name, else `filepath.Clean(domain)`. `TestInspectDomainWorkspaceResolvesConfiguredDomainPath` covers both branches.
- [PASS] A domain name containing a path separator, such as 'protocols/ws1', resolves to 'protocols/ws1/<workspace_dir>/' and that nested directory is the one inspected
  - `TestInspectDomainWorkspaceDomainNameWithSeparator` confirms the nested path is used and a sibling one level up is not mistaken for it.
- [PASS] A workspace containing only empty subdirectories, at any nesting depth, is reported clean
  - `TestInspectDomainWorkspaceEmptySubdirectoriesAreClean` passes; the walk only flips `Clean` on a regular file, never on a directory entry.
- [PASS] A workspace whose only content is a dotfile such as .gitkeep is reported dirty, not clean
  - `TestInspectDomainWorkspaceDotfileOnlyIsDirty` passes; `d.Type().IsRegular()` does not special-case dotfiles.
- [PASS] An existing workspace directory that cannot be traversed is reported dirty with the OS error detail retained in the error field
  - `TestInspectDomainWorkspaceUntraversableIsDirtyWithError` (chmod 0 on a subdirectory) passes; the walk callback's `err != nil` branch (`readiness.go:113-120`) sets `Clean = false` and `Error = err.Error()`, and the test confirms the offending path name ("locked") appears in the message.
- [PASS] Inspecting a workspace creates, modifies, and deletes nothing on disk — the directory tree is byte-identical before and after
  - `TestInspectDomainWorkspaceWritesNothing` snapshots the tree (path, mode, size) before and after three `InspectDomainWorkspace` calls (dirty, clean-but-present, absent domains) and asserts no diff. `InspectDomainWorkspace` itself contains no `os.Mkdir`/`os.Create`/`os.Remove` calls — only `os.Lstat` and `filepath.WalkDir`.

#### Notes

- Matches step 5 precisely: the walk returns `filepath.SkipAll` on the first regular file found (`readiness.go:125`), so a workspace with many files answers as fast as one with a single file.
- The `!info.IsDir()` branch (`readiness.go:104-110`, workspace path occupied by a non-directory) is untested but is a reasonable, non-spec-contradicting extension of "cannot be traversed"; not a deficiency.
- `os.Lstat` (rather than `os.Stat`) is used to resolve the workspace path itself, so a workspace path that is itself a symlink would report "not a directory" rather than following it. This is an unexercised corner case with no corresponding spec requirement or test — noted for awareness, not a blocking issue.
- Item scope is correctly limited to pure inspection, matching its description ("no verdict formatting and no enforcement"): no `preflight` command, no enforcement wiring at `init`/phase-shift entries, and no `ReadinessVerdict` aggregation type are present yet, consistent with this being L0 Foundations batch 1.

## Summary

Both items satisfy every declared acceptance criterion with exact-match unit tests, and both are properly scoped to foundations-only work (helpers/inspection primitives), leaving caller wiring and higher-level aggregation/enforcement for later batches, as their `files` and `steps` prescribe. `go build ./...` and `go test ./...` pass cleanly, including all new tests run individually and verbosely.
