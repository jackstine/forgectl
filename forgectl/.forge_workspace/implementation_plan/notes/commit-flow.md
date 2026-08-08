# Notes — Commit Flow (`state/git.go`, `state/advance.go`)

Reference for the batch auto-commit refinement introduced by spec commit `c00ba80`.

## Current commit machinery

`AutoCommit(projectRoot, strategy string, stageTargets []string, message string) (string, error)` — `forgectl/state/git.go:17`.

- Resolves the git root via `GitRepoRoot` (`git.go:90`), so it works when `projectRoot` is a subdirectory of the repo.
- Converts relative `stageTargets` to absolute paths anchored at `projectRoot`.
- Staging by strategy (`git.go:36-48`):
  - `strict`, `all-specs`, `scoped` — `git add <absTargets...>`; **errors** when `stageTargets` is empty ("commit strategy %q requires at least one stage target…").
  - `tracked` — `git add -u`.
  - `all` — `git add -A`.
- Commit: `git -C <gitRoot> commit -m <message>`.
- **Empty-commit handling already exists** (`git.go:59-64`): when git reports `nothing to commit` or `nothing added to commit`, it prints `notice: nothing to commit, skipping` to stderr and returns `("", nil)` — *not* an error. The batch-terminal inline commit's "silently skipped if nothing staged" requirement is satisfied by this existing path; no new logic is needed, only a caller that tolerates the empty hash.
- Returns the full hash from `git rev-parse HEAD`.

The message is passed through **verbatim**. There is no message-synthesis code anywhere in the codebase.

## Staging-target helpers

| Helper | Location | Notes |
| --- | --- | --- |
| `effectiveImplStrategy(s)` | `advance.go:2081` | resolves `implementing.commit_strategy` |
| `implScopeTargets(impl, item, strategy)` | `advance.go:2091` | `item != nil` → per-item targets; `item == nil` → batch targets |
| `effectiveUIStrategy(s)` | `advance.go:2114` | resolves `ui_implementing.commit_strategy` |
| `uiScopeTargets(ui, item, strategy)` | `advance.go:2124` | same `nil`-item convention |

The `nil`-item call form is what the batch-terminal commit already uses at `advance.go:654` (implementing COMMIT) and `advance.go:1147` (ui COMMIT). The inline terminal commit reuses that same form.

## Current call sites (all four change)

### `advanceImplFromImplement` — `advance.go:749`

```go
// advance.go:759 — first-round --message requirement
if batch.EvalRound == 0 && s.Config.General.EnableCommits && in.Message == "" {
    return fmt.Errorf("--message is required for first-round implementation when enable_commits is true")
}
...
// advance.go:771 — first-round-only auto-commit
if batch.EvalRound == 0 && s.Config.General.EnableCommits {
    strategy := effectiveImplStrategy(s)
    item := findItem(plan, itemID)
    stageTargets := implScopeTargets(impl, item, strategy)
    if _, err := AutoCommit(dir, strategy, stageTargets, in.Message); err != nil { ... }
}
```

Target behavior: drop the `--message` requirement entirely; drop the `batch.EvalRound == 0` guard on the commit so every round commits; pass a synthesized message instead of `in.Message`.

### `advanceUIFromImplement` — `advance.go:960`

Structurally identical; the same two guards live at `advance.go:971` and `advance.go:980`.

### `advanceImplFromEvaluate` — `advance.go:803`

Both terminal branches set `StateCommit` unconditionally — `advance.go:848` (PASS at/above `min_rounds`) and `advance.go:863` (FAIL at/above `max_rounds`). `enable_commits` is never consulted here. `EvalModeFor` is consulted only at `advance.go:809` to decide whether `--eval-report` is required.

### `advanceUIFromE2EVerify` — `advance.go:1104`

Terminal branch sets `StateCommit` at `advance.go:1121`.

## What the implementing COMMIT state does today (`advance.go:649-672`)

Ordering matters — the inline path must reproduce it:

1. Require `--message` when `enable_commits` (line 650).
2. `AutoCommit` with batch stage targets (lines 653-659).
3. `archiveBatch(s)` — `advance.go:1761`.
4. `loadPlan`, then `allLayersComplete(plan)` (`advance.go:1752`) → `StateDone`, else `StateOrient`.

Items are already marked `passed`/`failed` earlier, inside `advanceImplFromEvaluate` (lines 841-844 / 856-859), so the implementing inline path does **not** need to mark items.

## What the ui COMMIT state does today (`advance.go:1140-1184`)

Different, and this is the subtle part:

1. Require `--message` when `enable_commits` (line 1141).
2. `AutoCommit` with batch stage targets (lines 1145-1151).
3. **Terminal item marking** (lines 1176-1183) — this happens *only here*, not in the evaluator states:
   ```go
   forced := batch.CodeForceAccepted || batch.QAForceAccepted || batch.E2EForceAccepted
   status := "passed"
   if forced { status = "failed" }
   for _, id := range batch.Items { setItemPasses(plan, id, status) }
   savePlan(...)
   ```
4. `archiveUIBatch(s)` — `advance.go:1225`.
5. `allLayersComplete` → `StateDone` / `StateOrient`.

Because the three force-accept flags (`CodeForceAccepted`, `QAForceAccepted`, `E2EForceAccepted` — `types.go:646-650`) are only fully known after the e2e loop, the ui inline terminal path at E2E_VERIFY must perform the *same* marking before committing. Factoring this into a shared helper avoids the two paths drifting.

## Message synthesis contract

Defined in `docs/auto-committing.md:30-35` (that doc is already rewritten to the target behavior — treat it as the reference, not as something to update):

- **Per-item IMPLEMENT commit** — the item's `description` field from plan.json, verbatim.
- **Batch-terminal commit** — a summary line listing every item in the batch in order, e.g.
  `Batch 2: Load YAML, apply defaults, validate strictly; ServiceEndpoint and ServicesConfig structs`
  i.e. `Batch <N>: ` followed by item descriptions joined with `; `.
- If `--message` is also supplied, that text is **appended as a second paragraph** separated by a blank line — never substituted.
- specifying COMPLETE (`advance.go:291`) and planning ACCEPT (`advance.go:456`) are explicitly **unaffected**; they keep requiring an explicit `--message`.

`docs/auto-committing.md:26` also states the consequence: when `enable_commits: true`, implementing and ui_implementing never require `--message`, and their COMMIT state is skipped entirely so it can never be reached to demand one.

Batch number for the summary line: `impl.BatchNumber` / `ui.BatchNumber` (used already by `qaStepListPath`, `output.go:1188`).

## `eval_mode` and the direct-mode loop

`EvalModeFor(ec EvalConfig, gen GeneralConfig) string` — `config.go:503`. Resolves `"report"` / `"direct"` / `"conversational"`, falling back to the legacy `enable_eval_output` bool when `eval_mode` is empty. Valid values are enforced by `ValidateConfig` (`config.go:538-564`).

Every existing call is a `--eval-report`-required check: `advance.go:98, 192, 247, 411, 809, 1012, 1044, 1106`. **No transition consults it.**

Target: in the non-terminal branches of the implementing EVALUATE and the ui code-EVALUATE, `"direct"` re-enters `StateEvaluate` while `"report"`/`"conversational"` keep re-entering `StateImplement`. Note the ui code loop's non-terminal branch is `advance.go:1035-1037`.

Round bookkeeping differs between the two phases and must be preserved:

- **implementing** — `EvalRound` is incremented on the IMPLEMENT→EVALUATE transition (`advance.go:797`). A direct-mode EVALUATE→EVALUATE re-entry therefore has to increment `EvalRound` itself, or the loop never reaches `max_rounds` and spins forever.
- **ui_implementing** — the comment at `advance.go:1013` records the convention: each loop counter is incremented when the batch *enters* that loop's evaluator state. `EvalRound++` happens at `advance.go:1006`; `QARound++` on entry to QA_TEST (`advance.go:1029`) and on UI_REFINE re-entry (`advance.go:1091`); `E2ERound++` on entry to E2E_VERIFY (`advance.go:1099`) and on E2E_REMEDIATE re-entry (`advance.go:1136`). A direct-mode EVALUATE→EVALUATE re-entry is an entry into the code-eval loop and must increment `EvalRound` to stay consistent with that convention.

This is the single easiest thing to get wrong in the whole change: an infinite evaluation loop.
