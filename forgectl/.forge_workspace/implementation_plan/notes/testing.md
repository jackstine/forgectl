# Notes — Testing (`forgectl/*_test.go`, `forgectl/integration/`)

## Unit tests

`forgectl/state/`: `advance_test.go` (largest — state machine transitions), `config_test.go`, `types_config_test.go`, `validate_test.go`, `git_test.go`, `output_test.go`, `scaffold_test.go`, `state_test.go`, `types_test.go`, `hash_test.go`, `logger_test.go`.

`forgectl/cmd/`: `commands_test.go` (with `setupProjectDir()` at `commands_test.go:16` and `resolvedStateDir()` at `:39`), `validate_test.go`, `generateworkflow_test.go`.

Patterns: table-driven cases, `t.TempDir()` for isolation.

## Integration tests

Build tag `//go:build integration` (`harness_test.go:1`). The binary is compiled once in `TestMain` (`harness_test.go:46`) and reused.

Harness: the `Project` fixture (`harness_test.go:79`, constructed by `NewProject` at `:93`) sets up an isolated HOME and git identity — essential for anything exercising commits, since `AutoCommit` shells out to real git.

Key helpers a new test reuses:

- `p.AssertAt(phase, state)` — assert current phase/state
- `p.mustForge(...)` — run a command, fail the test on error
- `p.forge(...)` — run a command and capture the result (use for expected-failure paths)
- `p.WriteConfig(...)`, `p.WriteFile(...)` — fixture setup
- `updateGolden` flag (`harness_test.go:42`) — golden-file regeneration

Relevant existing files: `git_commit_test.go` (commit behavior), `lifecycle_implementing_test.go`, `lifecycle_ui_implementing_test.go`, `round_budget_test.go` (min/max round enforcement — the natural home for direct-mode loop-termination coverage), `batch_test.go`, `config_scaffold_test.go`, `pipeline_multidomain_test.go` (multi-domain continuation — the regression surface for over-gating the readiness gate), `cli_surface_test.go`, `idempotency_test.go`, `persistence_test.go`.

## `cli_surface_test.go` — must be updated for a new command

Asserts two invariants:

- **K3** — guard conditions on state-modifying commands. One test per command: `TestK3AddQueueItemGuards` (`:43`), `TestK3SetRootsGuards` (`:118`), `TestK3SetCommitHashesGuards` (`:154`), `TestK3AddDomainGuards` (`:188`).
- **K4** — `TestK4ErrorPathsExitNonZeroToStderr` (`:228`): error paths exit non-zero and write to **stderr, not stdout**.

`preflight` is a read-only query, so it has no phase/state guard to assert under K3 — but its two rejection paths (unreadable `--from`, and no-`--from`-without-session) belong under K4's stderr/exit-code contract.

## Coverage the change needs

Commit flow — `enable_commits: true` and `false` are genuinely different state paths now, so both need coverage:

- With commits on: COMMIT is never entered; the terminal EVALUATE lands on ORIENT/DONE.
- With commits off: COMMIT is still entered and is a no-op advance.
- A commit lands on *every* IMPLEMENT round, not just the first.
- `--message` is accepted-but-optional at IMPLEMENT, and its text is appended rather than substituted.
- The inline batch-terminal commit is silently skipped when nothing is staged (the `AutoCommit` empty-commit path at `git.go:59`).

Direct mode: the loop must **terminate**. A direct-mode FAIL sequence has to reach `max_rounds` and force-accept rather than spin — that is the assertion that catches a missing round increment.

Readiness gate: clean/dirty determination (absent dir, empty dirs only, dotfile-only, nested file, unreadable dir), all three enforcement points aborting with no mutation, and — the important negative — that intra-session domain-boundary re-entries into planning ORIENT are **not** gated.
