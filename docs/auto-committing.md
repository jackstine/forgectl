# Auto-Committing

This document defines how forgectl handles automatic git commits when `enable_commits` is `true`.

## Overview

When `general.enable_commits` is `true`, forgectl automatically stages and commits files at defined commit points in the lifecycle. The `--message` (`-m`) flag provides the commit message. After a successful commit, the resulting hash is automatically registered against the relevant completed specs or plan items in the state file.

When `general.enable_commits` is `false` (default), forgectl does not perform any git operations. The user commits manually.

## `--message` / `-m` Flag Behavior

| `enable_commits` | `--message` provided | Behavior |
|-----------------|---------------------|----------|
| `true` | yes | Required. Scaffold commits with the message. |
| `true` | no | Error. Exit code 1. |
| `false` | yes | Warning: `--message is ignored, commits are not enabled`. Command proceeds. Message discarded. |
| `false` | no | Normal. No warning. |

The warning when `--message` is ignored does **not** instruct the user how to enable commits. This is intentional — users who do not need auto-commits should not be prompted to change their configuration.

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

### Implementing Phase

| Commit Point | State | What is committed |
|-------------|-------|------------------|
| Per item (first round only) | IMPLEMENT | Source and test files for the implemented item |
| Per batch (after terminal eval) | COMMIT | Corrections from evaluation rounds 2+ |

First-round implementation commits provide crash safety for new work. The COMMIT state after evaluation captures any corrections. Subsequent-round IMPLEMENT states (after eval FAIL) do not produce commits — corrections accumulate and are committed at the batch COMMIT state.

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
2. Stage files according to the strategy.
3. Check `git status --porcelain` for staged changes.
4. If no staged changes: skip commit, print notice, continue.
5. If staged changes exist: run `git commit -m <message>`.
6. Capture the commit hash from git output.
7. Register the hash in the state file against relevant entries.
8. Write the updated state file.

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
