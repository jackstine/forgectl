# Auto-Committing

This document defines how forgectl handles automatic git commits when `enable_commits` is `true`.

## Overview

When `general.enable_commits` is `true`, forgectl automatically stages and commits files at defined commit points in the lifecycle. At most commit points the message is synthesized by the scaffold itself (see Message Synthesis); the `--message` (`-m`) flag, where accepted, supplements that synthesized message rather than replacing it. After a successful commit, the resulting hash is automatically registered against the relevant completed specs or plan items in the state file.

When `general.enable_commits` is `false` (default), forgectl does not perform any git operations. The user commits manually.

## `--message` / `-m` Flag Behavior

| Commit point | `enable_commits` | `--message` provided | Behavior |
|--------------|-----------------|---------------------|----------|
| Implementing/`ui_implementing` IMPLEMENT (per item) | `true` | yes | Optional. Appended to the item-description-synthesized message. Scaffold commits. |
| Implementing/`ui_implementing` IMPLEMENT (per item) | `true` | no | Normal. Scaffold commits with the synthesized message alone. |
| Implementing/`ui_implementing` terminal EVALUATE (batch, inline) | `true` | yes | Optional. Appended to the batch-description-synthesized message. Scaffold commits. |
| Implementing/`ui_implementing` terminal EVALUATE (batch, inline) | `true` | no | Normal. Scaffold commits with the synthesized message alone. |
| Specifying COMPLETE / Planning ACCEPT | `true` | yes | Required. Scaffold commits with the message. |
| Specifying COMPLETE / Planning ACCEPT | `true` | no | Error. Exit code 1. |
| any | `false` | yes | Warning: `--message is ignored, commits are not enabled`. Command proceeds. Message discarded. |
| any | `false` | no | Normal. No warning. |

The warning when `--message` is ignored does **not** instruct the user how to enable commits. This is intentional — users who do not need auto-commits should not be prompted to change their configuration.

When `enable_commits: true`, the implementing and `ui_implementing` phases never require `--message`: the IMPLEMENT and terminal-EVALUATE commit points always have a synthesized message available, and their COMMIT state is skipped entirely (see Commit Points below), so it never has to be reached to demand one. `--message` remains required only at the commit points that have no synthesized message of their own — specifying's COMPLETE and planning's ACCEPT — which are unaffected by this change.

## Message Synthesis

For the implementing and `ui_implementing` phases' per-item and batch-terminal commit points, the scaffold synthesizes the commit message rather than requiring one:

- **Per-item IMPLEMENT commit:** the message is the item's `description` field from plan.json, verbatim.
- **Batch-terminal commit (inline at terminal EVALUATE):** the message is a synthesized summary line listing every item in the batch, in order — for example `Batch 2: Load YAML, apply defaults, validate strictly; ServiceEndpoint and ServicesConfig structs`.

In both cases, if the operator or agent also supplies `--message "<text>"` on the `advance` call, that text is appended as a second paragraph (separated by a blank line) rather than replacing the synthesized message. Specifying's COMPLETE and planning's ACCEPT commit points are unaffected by this synthesis rule — they continue to require an explicit `--message`.

## Commit Points

### Specifying Phase

| Commit Point | State | What is committed |
|-------------|-------|------------------|
| End of specifying | COMPLETE | All spec work from the entire specifying phase (drafting, refinement, reconciliation) in a single commit |

One commit for the entire specifying phase. Individual eval rounds, refinements, and reconciliation passes do not produce commits.

### Planning Phase

| Commit Point | State | What is committed |
|-------------|-------|------------------|
| Per plan acceptance | ACCEPT | plan.json + notes for the accepted plan |

One commit per plan. When `planning.batch` > 1 is supported, each plan acceptance produces its own commit.

### Implementing Phase (and `ui_implementing`'s code-eval loop, which reuses this spine unchanged)

| Commit Point | State | What is committed |
|-------------|-------|------------------|
| Per item, every round | IMPLEMENT | Source and test files for that item, message synthesized from the item's `description` |
| Per batch, inline at the terminal EVALUATE transition (`enable_commits: true` only) | EVALUATE (terminal) | Any remaining corrections since the batch's per-item commits, message synthesized from the batch's item descriptions |
| Per batch (`enable_commits: false` only — no git operation, bookkeeping no-op) | COMMIT | — |

Every IMPLEMENT round commits its item immediately — first round and every round after an eval FAIL alike; the previous "first round only" rule no longer applies. This provides crash safety for new work and for every correction pass.

When `enable_commits: true`, the COMMIT state does not appear in the transition path. Instead, the moment EVALUATE reaches a terminal verdict for the batch — PASS with sufficient rounds, or FAIL at `max_rounds` (force-accept) — the scaffold auto-commits any remaining uncommitted corrections inline as part of that same `advance`, then proceeds directly to ORIENT/DONE. This commit follows the same empty-commit skip rule as any other commit point: if every item was already committed per-item and no evaluator round left further changes, the commit is skipped silently.

When `enable_commits: false`, the COMMIT state is unchanged from before: it still appears as a distinct state/transition for bookkeeping and plan-completion purposes, but no git operation occurs there (as with all commit points when commits are disabled).

`ui_implementing`'s QA loop (QA_TEST ⟲ UI_REFINE) and e2e loop (E2E_AUTHOR → E2E_VERIFY ⟲ E2E_REMEDIATE) produce no commits of their own; only the code-eval loop above and the phase's own COMMIT/inline-auto-commit boundary (reached after all three loops terminate) do.

## Staging Strategies

The `commit_strategy` configuration controls which files are staged before each commit. Different phases have different defaults because the nature of the work differs.

| Strategy | Behavior |
|----------|----------|
| `strict` | Only file paths registered in state or plan.json. Nothing else. |
| `all-specs` | All files in `<domain_path>/specs/` for every domain in the session. |
| `scoped` | Registered files + any changed or new files under `<domain_path>/`. |
| `tracked` | `git add -u` — all modified files already tracked by git, repo-wide. Does not stage new untracked files. |
| `all` | `git add -A` — everything in the working tree. |

### Strategy Comparison

| Scenario | strict | all-specs | scoped | tracked | all |
|----------|--------|-----------|--------|---------|-----|
| Registered file modified | Yes | Yes | Yes | Yes | Yes |
| Non-registered file in `<domain>/specs/` | No | **Yes** | Yes | Depends | Yes |
| New file in domain, not registered | No | No | **Yes** | No | Yes |
| Existing tracked file in domain, not registered | No | No | Yes | Yes | Yes |
| Existing tracked file **outside** domain | No | No | No | **Yes** | Yes |
| New untracked file outside domain | No | No | No | No | **Yes** |

### Phase Defaults

| Phase | Default Strategy | Rationale |
|-------|-----------------|-----------|
| Specifying | `all-specs` | Reconciliation can modify existing specs outside the session queue (e.g., adding cross-references). All files in spec directories for session domains must be captured. |
| Planning | `strict` | `plan.json` and `notes/` file paths are fully registered in the state file and plan.json refs. No unregistered files are expected. |
| Implementing | `scoped` | Implementation commonly creates new source files (helpers, tests, generated code) within the domain that are not listed in `items[].files`. Domain-scoped staging captures these while preventing cross-domain bleed. |

### Domain Nesting Constraint

Domains must not have nested paths (e.g., `api/` and `api/internal/` cannot both be domains in the same session). This constraint exists because `scoped` and `all-specs` strategies use domain paths as staging boundaries. Nested domains would cause one domain's staging to include another domain's files.

## Empty Commits

If no files have changed at a commit point, the scaffold skips the commit. No error is raised. The action output notes that no changes were detected and no commit was made.

## Automatic Hash Registration

When the scaffold successfully executes `git commit`, it captures the resulting commit hash and automatically registers it:

- **Specifying:** Hash added to `commit_hashes` on all `specifying.completed[]` entries.
- **Planning:** Hash added to `planning.completed[]` or `planning.current_plan` commit tracking.
- **Implementing:** Hash recorded in `implementing.current_batch` or `implementing.layer_history`.

Hash registration is fully automatic when `enable_commits` is `true`.

## Configuration

```toml
[general]
enable_commits = false          # default: no git operations

[specifying]
commit_strategy = "all-specs"   # strict | all-specs | scoped | tracked | all

[planning]
commit_strategy = "strict"      # strict | all-specs | scoped | tracked | all

[implementing]
commit_strategy = "scoped"      # strict | all-specs | scoped | tracked | all
```

## Git Operations Sequence

When a commit point is reached and `enable_commits` is `true`:

1. Determine the staging strategy from `<phase>.commit_strategy`.
2. Resolve the git repository root via `git rev-parse --show-toplevel` (may differ from the forgectl project root when `.forgectl/` lives inside the repo rather than at its root).
3. Convert stage targets to absolute paths anchored at the project root, then run `git add` from the git root with those paths.
4. Run `git commit -m <message>` from the git root.
5. If `git commit` exits with "nothing to commit" or "nothing added to commit": skip the commit, print a notice to stderr, continue with an empty hash.
6. If `git commit` fails for any other reason: print the error and exit with code 1.
7. Capture the resulting commit hash via `git rev-parse HEAD`.
8. Register the hash in the state file against relevant entries.
9. Write the updated state file.

All git commands run from the repository root. This is required because `git add` resolves relative paths against the command's working directory — running from a subdirectory (such as a domain directory) would mis-scope the paths. `git add` must run before `git commit`: `git commit` without `-a` only includes files already in the index, and new (untracked) files require an explicit `git add` to be committed at all.

If any git operation fails (staging, committing), the scaffold prints the error and exits with code 1. Git failures are not recoverable — the user must resolve the issue manually.

## Output

Successful auto-commits are silent — no output is shown to the user. Hash registration and state file updates happen internally.

### `--message` ignored (enable_commits is false)

```
--message is ignored, commits are not enabled
```

### Git failure

```
Error: STOP there was a failure with auto committing in forgectl, please tell the user: <git error message>
```

Git failures are the only auto-commit scenario that produces output (besides the `--message` warning). The message is directed at the AI agent driving the session, instructing it to surface the error to the human user.
