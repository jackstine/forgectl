# forgectl — Bugs Found (multi-domain pipeline integration work)

These were surfaced by **driving the real built binary end-to-end** through a
multi-domain `planning → implementing / ui_implementing` pipeline (the work in
`forgectl/integration/pipeline_multidomain_test.go`). Each was confirmed at the
**source level** in `forgectl/state/advance.go`, not merely inferred from output.

| # | Bug | Severity | Where | Status |
|---|-----|----------|-------|--------|
| 1 | `plan_all_before_implementing: true` implements only the **last** planned domain | **High** (silent loss of work) | `advance.go` (orig. 474–495) | **Fixed** — commit `0823bf0`; `TestPipelinePlanAllBeforeImplementing` un-skipped and passing |
| 2 | `ui_implementing` **requires** `--message` but never commits | **High** (silent loss of commits) | `advance.go` (orig. 942–944, 1101–1103) | **Fixed** — commit `0823bf0`; `TestUIImplementingCommitsWhenEnabled` un-skipped and passing |
| 3 | `PHASE_SHIFT` label hardcoded to "planning → implementing" for `kind:"ui"` | Low (cosmetic/contract drift) | `output.go` | **Fixed** — commit `0823bf0`; label now reads phase from `PhaseShiftInfo.To` |
| 4 | `EvalRound` incremented on verdict (exit from EVALUATE) instead of on entry | **Medium** (counter off-by-one; min_rounds comparison uses wrong round) | `advance.go` `advanceImplFromEvaluate` | **Fixed** — see below; `TestImplementingMinRoundsLoops` now passing |
| 5 | `[planning.study_specs]` config silently dropped by loader | **Medium** (operator config ignored) | `config.go` `tomlPlanningConfig` | **Fixed** — commit `f0496e8`; `TestPlanningStudySpecsConfigIsHonored` un-skipped and passing |
| 6 | Embedded default-config.toml behind docs canonical (missing `specifying.prompt_domains` + `[reverse_engineering]`) | **Low** (template/docs drift; unit test failing) | `state/default-config.toml` | **Fixed** — commit `f0496e8`; `TestEmbeddedTemplateMatchesDocs` passing; `go test ./...` fully green |
| 7 | `OPERATING_MANUAL.md` specifying drift (`RECONCILE_EVAL PASS → COMPLETE`; `-m` shorthand) | Low (doc drift) | `OPERATING_MANUAL.md` | **Remaining** |

---

## Bug 1 — `plan_all_before_implementing: true` drops every domain but the last

### Severity: High — planned work is silently never implemented

With `planning.plan_all_before_implementing = true`, an operator expects forgectl
to plan **all** domains first, then implement **all** of them. It plans all of
them, then implements **only the last domain in the queue**; every earlier domain
is planned and then dropped on the floor. No error, no warning — the session just
reports "complete".

### The spec'd contract (what *should* happen)

`forgectl/specs/plan-production.md:656–658`:

```
| ACCEPT | plan_all_before_implementing: true, queue empty | DONE | Move plan to completed. |
| ACCEPT | plan_all_before_implementing: false           | PHASE_SHIFT | … (planning → implementing). |
| DONE   | always (plan_all_before_implementing: true only) | PHASE_SHIFT | Set phase shift from planning → implementing. Populate implementing plan queue from completed plans. |
```

So the intended flow is:
1. Plan domain A → ACCEPT (queue non-empty) → PHASE_SHIFT (planning → planning).
2. Plan domain B → ACCEPT (queue non-empty) → PHASE_SHIFT (planning → planning).
3. Plan domain C → ACCEPT (**queue empty**) → **DONE**.
4. DONE → PHASE_SHIFT (planning → implementing), **populating the implementing
   queue from all completed plans (A, B, C)**.
5. Implement A, B, C in turn (each routed by its `kind`).

### What the binary actually does

`forgectl/state/advance.go:474–495` (planning `StateAccept`):

```go
if s.Config.Planning.PlanAllBeforeImplementing && len(s.Planning.Queue) > 0 {
    // Pop next plan from queue and continue planning. (planning → planning)
    ...
    s.State = StatePhaseShift
    s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhasePlanning}
} else {
    s.State = StatePhaseShift
    s.PhaseShift = &PhaseShiftInfo{From: PhasePlanning, To: PhaseImplementing}
}
```

On the **last** plan the queue is empty, so it takes the `else` branch:
`PHASE_SHIFT{To: implementing}` for **the current (last) plan only**. It never
enters `StateDone`, and **never populates an implementing queue from
`s.Planning.Completed`**. The `StateDone` case (`advance.go:497–503`) which is the
one that the spec describes — the place that would set up the all-domains
implementation — is therefore **unreachable** in this mode.

### Reproduction

```bash
cd <scratch>; git init -q; git config user.email t@e.com; git config user.name t
mkdir -p .forgectl core api
cat > .forgectl/config <<'EOF'
[general]
enable_commits = false
[planning]
plan_all_before_implementing = true
[planning.eval]
min_rounds = 1
max_rounds = 3
[implementing.eval]
min_rounds = 1
max_rounds = 3
EOF
cat > plan-queue.json <<'EOF'
{"plans":[
 {"name":"Core","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"},
 {"name":"Api","domain":"api","file":"api/plan.json","specs":[],"spec_commits":[],"code_search_roots":["api/"],"kind":"code"}
]}
EOF
# (write trivial single-item core/plan.json and api/plan.json)
export HOME=<throwaway>
forgectl init --phase planning --from plan-queue.json
# drive: plan core (PASS eval) -> plan api (PASS eval) -> implement ...
```

**Observed** at session end:
- `planning.completed` = `[core, api]` (both planned ✓)
- `implementing` state section **absent**; only the **last** domain entered an
  implementation phase. The earlier domain's items stay `pending` forever.

### Suggested fix

In the planning `StateAccept` `else` branch (queue empty + `plan_all_before`),
transition to `StateDone` rather than `StatePhaseShift`. Then implement the
`StateDone` handler for this mode to: build the implementing plan queue from
`s.Planning.Completed`, and PHASE_SHIFT into the first plan's implementation phase
(routed by its `kind`). The implementing/ui DONE handlers already know how to pull
the next plan from that queue (interleaved mode proves the per-domain chaining
works), so the missing piece is populating it.

### Captured by

`TestPipelinePlanAllBeforeImplementing` — asserts **both** domains end `passed`.
Currently `t.Skip`'d with a `KNOWN BUG` note; un-skip when fixed.

---

## Bug 2 — `ui_implementing` requires `--message` but never commits

### Severity: High — UI commits are silently never made

In `ui_implementing` with `enable_commits = true`, forgectl **rejects** a missing
`--message` at the first-round `IMPLEMENT` and at `COMMIT` — exactly like the
`implementing` phase — but then **never calls `AutoCommit`**. The operator is
forced to supply a commit message that is then thrown away: a commits-on UI batch
produces **zero** git commits.

### The asymmetry (implementing is correct, ui is not)

`implementing` phase — message gate **and** commit:
- `advance.go:731` message required for first-round implementation → `advance.go:746` `AutoCommit(...)`
- `advance.go:622` message required in COMMIT → `advance.go:627` `AutoCommit(...)`

`ui_implementing` phase — message gate, **no** commit:
- `advance.go:942–944` (in the UI IMPLEMENT path):
  ```go
  if batch.EvalRound == 0 && s.Config.General.EnableCommits && in.Message == "" {
      return fmt.Errorf("--message is required for first-round implementation when enable_commits is true")
  }
  // ... marks item done, saves plan — NO AutoCommit anywhere below
  ```
- `advance.go:1101–1103` (`advanceUIFromCommit`):
  ```go
  if s.Config.General.EnableCommits && in.Message == "" {
      return fmt.Errorf("--message is required in COMMIT state when enable_commits is true")
  }
  // ... finalizes item passes, archives batch — NO AutoCommit anywhere below
  ```

There is **no `AutoCommit` call anywhere** in `advanceUIImplementing` (the whole
function spans roughly `advance.go:855–1134`; the `AutoCommit` calls in the file
are all at 300/461/627/746, i.e. specifying/planning/implementing only).

### Reproduction

```bash
forgectl init --phase ui_implementing --from portal/plan.json   # commits on, ui config set
forgectl advance                          # ORIENT -> IMPLEMENT
forgectl advance                          # rc=1: "--message is required for first-round implementation…"
forgectl advance --message "impl"         # rc=0 -> EVALUATE
# … pass eval / qa / e2e …
forgectl advance --message "commit ui"    # rc=0 -> DONE
git log                                    # fatal: …does not have any commits yet
```

The `implementing` phase under the same conditions **does** commit (proven green
by `TestPipelineMultiDomainCodeCommits`).

### Suggested fix

Add the two `AutoCommit` calls to `ui_implementing`, mirroring `implementing`:
- first-round `IMPLEMENT`: per-item commit staging the item's `Files` (crash-safety
  granularity), after the message gate at `advance.go:942–944`;
- `COMMIT` (`advanceUIFromCommit`): batch commit per the effective ui commit
  strategy, after the gate at `advance.go:1101–1103`.

Use the existing `effectiveImplStrategy`/scope-target helpers (or a ui equivalent)
— `ui_implementing.commit_strategy` already exists in config (`types.go`).

### Captured by

`TestUIImplementingCommitsWhenEnabled` — asserts `git log` is non-empty after a
commits-on UI batch. Currently `t.Skip`'d; un-skip when fixed.

---

## Bug 3 — `PHASE_SHIFT` label always says "planning → implementing"

### Severity: Low — cosmetic, but it is a contract surface skills/operators read

When a plan's `kind` is `"ui"`, the planning→implementation PHASE_SHIFT still
prints:

```
State:   PHASE_SHIFT
From:    planning → implementing
```

even though it actually routes into **ui_implementing** (and validates the
`ui_implementing` config — the B5 gate fires on the ui keys, proving routing knows
it's UI). The label is wrong.

### Cause

`advance.go:490–494` sets `PhaseShiftInfo{To: PhaseImplementing}` unconditionally
in both ACCEPT branches; the `To` field never consults `plan.Kind`. The actual
routing at PHASE_SHIFT *consumption* uses `kind`, so behavior is correct — only
the displayed `To` is stale.

### Suggested fix

Set `To` from the plan's `kind`: `"ui"` → `PhaseUIImplementing`, else
`PhaseImplementing`. (This is the same `To` shown in the PHASE_SHIFT output, so it
fixes the label and keeps state self-consistent.)

### Note for testers

Because the label is wrong, tests must assert the **routing reaches the
`ui_implementing` phase** (i.e. inspect the resulting `phase`), not the PHASE_SHIFT
text. `TestPipelineMultiDomainInterleaved` does exactly this.

---

## Behavioral clarifications (NOT bugs, but they contradicted assumptions)

While mapping the flows by hand I corrected two assumptions that a reader (or a
spec-derived map) might hold:

- **Planning `REVIEW` is unconditional**, not `user_guided`-gated.
  `advance.go:393–394`: `StateStudyPackages` → `StateReview` always, then
  `StateReview` → `StateDraft`. A run with `user_guided = false` still passes
  through `REVIEW`. (A code-reading sub-agent had guessed REVIEW was gated; it is
  not.)
- **`ui_implementing` eval states do not print a concrete eval-report path** in the
  `advance`/`status` action block — they print a `<path>` placeholder. The concrete
  path is surfaced by `forgectl eval` under `--- REPORT OUTPUT ---`. `--eval-report`
  only checks the file *exists*, so any stub path is accepted. (Contrast: planning
  and implementing eval states **do** print the exact path, which
  `Project.ExtractReportPath` parses.)

---

## Bug 4 — `EvalRound` incremented on verdict, not on entry to EVALUATE

### Severity: Medium — min_rounds comparison uses the wrong round number

In the `implementing` phase, `EvalRound` was incremented inside
`advanceImplFromEvaluate` (when processing the verdict), so when the scaffold
entered `StateEvaluate` the counter still reflected the *previous* round.
`TestImplementingMinRoundsLoops` exposed this: after the first IMPLEMENT→EVALUATE
transition `EvalRound` read 0 (want 1); after the second it read 1 (want 2).

### The spec'd contract (`batch-implementation.md:793–797`)

```
### IMPLEMENT last item → EVALUATE
- Then: Item done. Rounds incremented. State is EVALUATE.
```

The round increment is listed as part of the IMPLEMENT→EVALUATE transition, so
`EvalRound` must be 1 when the state is first observed as EVALUATE.

### What the binary was doing

`advanceImplFromImplement` set `s.State = StateEvaluate` without touching
`EvalRound`. Then `advanceImplFromEvaluate` ran `batch.EvalRound++` as its first
act — meaning the counter was one behind from the agent's perspective whenever
it read the state file in EVALUATE.

The `ui_implementing` phase already did it correctly (incrementing `EvalRound` on
the IMPLEMENT→EVALUATE transition at line 1005).

### Fix applied

Moved `batch.EvalRound++` from `advanceImplFromEvaluate` to the
IMPLEMENT→EVALUATE else-branch in `advanceImplFromImplement` (just before
`s.State = StateEvaluate`), matching the ui_implementing pattern. The
`EvalRound == 0` commit guards earlier in the same function are unaffected.

### Captured by / status

`TestImplementingMinRoundsLoops` (round_budget_test.go) — **was failing before
the fix, passes now**. Committed alongside this handoff update.

---

## Pre-existing bugs (found by the earlier session, not this one)

For a complete picture — these are documented in `integration_handoff.md §5` and
`forgectl/integration/README.md`, and were **not** introduced by this work:

- **`[planning.study_specs]` config is silently dropped** — **Fixed** — commit
  `f0496e8`; added `tomlStudySpecsConfig` and merge logic to `config.go`;
  `TestPlanningStudySpecsConfigIsHonored` un-skipped and passing.
- **`docs/default-config.toml` is ahead of the embedded template** — **Fixed** —
  commit `f0496e8`; synced `forgectl/state/default-config.toml` to the docs
  canonical (`specifying.prompt_domains` + `[reverse_engineering]` section were
  missing; both fields already existed in Go structs and the loader);
  `TestEmbeddedTemplateMatchesDocs` now passes and `go test ./...` is fully green.
- **`OPERATING_MANUAL.md` specifying drift** — manual says `RECONCILE_EVAL PASS →
  COMPLETE` but the binary goes to `RECONCILE_REVIEW` first; manual/skills reference
  a `-m` shorthand that doesn't exist (only `--message`). **Remaining — not yet fixed.**
