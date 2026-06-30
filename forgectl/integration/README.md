# forgectl integration tests

Black-box, end-to-end tests that build the **real** forgectl binary and drive it
over a **real** filesystem and git repo. Where the in-process unit suite calls
`state.Advance()` directly and builds state in-memory with the same types it
serializes with, this suite exercises the seams the unit tests can't: `main.go`,
the cobra command layer, `go:embed` assets, exit codes, and the on-disk side
effects (state file, moved/created files, git commits, activity logs).

This is the realization of the integration test plan (`integration-test-plan.md`).

## Running

```bash
make test-integration
# or
cd forgectl && go test -tags=integration ./integration/...
```

The whole package is gated behind the `integration` build tag, so the fast unit
run (`go test ./...` / `make test`) is unaffected. `TestMain` builds the binary
once and shares it across every test.

## Isolation (plan §1a)

Isolation is baked into the shared fixture so no test can forget it:

- **Sandbox `$HOME`.** Every `Project` gets its own `HOME` (a `t.TempDir()`), so
  the activity logger and `PruneLogs` write to `<sandbox>/.forgectl/logs/` and
  never touch the developer's real `~/.forgectl/logs/`. It also detaches git from
  the real `~/.gitconfig`.
- **Repo-local git identity.** Each project pins `user.email`/`user.name` so
  commits don't depend on global config.
- **Everything else is project-root-relative** and contained by `t.TempDir()`.

## Harness (plan §2)

`harness_test.go` provides:

- **`NewProject(t)`** — an isolated temp project with a git repo and sandbox HOME.
- **`p.forge(args...)` / `p.mustForge(...)`** — run the built binary in the
  project, capturing `Stdout`, `Stderr`, and `Exit` separately. `forgeAt(dir,...)`
  runs from a subdirectory (used for project-root-discovery tests).
- **Fixture builders** — `WriteConfig`, `WriteFile`, `Path`, `Exists`, `ReadFile`.
- **Trifecta inspectors** — `State()` / `RawState()` / `AssertAt(phase,state)`
  (the persisted state), `GitLog()` / `GitShowStat()` (commits), `LogFiles()` /
  `LogEntries()` (activity logs).
- **Eval handshake** — `ExtractReportPath(out)` pulls the exact report path
  forgectl prints in an EVALUATE-family state (paths vary by eval kind/domain, so
  tests extract rather than hardcode); `PassEval(prev, verdict)` collapses the
  eval two-actor handshake (eval emits path → agent writes file → advance
  consumes path) into one call.
- **Scripted-session DSL** (`dsl_test.go`) — `Step` + `RunScript` so lifecycles
  read as data; `advStep` / `evalPassStep` are common-case builders.

## What's covered

| File | Plan items |
|------|-----------|
| `lifecycle_specifying_test.go` | **A1** specifying full walk (init → PHASE_SHIFT) with the trifecta; smoke test of the built binary (`K2` `--version`) |
| `dsl_test.go` | **§2.5** scripted-session DSL + a data-driven specifying happy path |
| `validate_corpus_test.go` | **§4.2** hand-authored corpus (spec-queue / plan-queue / plan, valid + one-violation-each) through `validate`; **K1** validate CLI surface |
| `config_scaffold_test.go` | **G1/G2/G3** scaffolding, byte-preservation, partial-merge + config lock, runtime-mutable `user_guided`; **§4.4** `default-config.toml` ⟷ `DefaultForgeConfig()` equivalence |
| `persistence_test.go` | **E1** atomic write/backup + absolute `state_dir`; **E2** backup/corrupt/stale-tmp recovery; **E3** root discovery stops at the git boundary |
| `oracle_specifying_test.go` | **§4.1** independent state-machine oracle for the specifying eval loop; **C1/C2** min-rounds-loops / force-accept |
| `logging_test.go` | **H1** only init/advance log; **H2** stable log filename across a session |
| `pipeline_multidomain_test.go` | **B1/B4** multi-domain pipeline (two code domains + one ui), interleaved, planning→implementing/ui_implementing chains driven through the CLI in one session, asserted against an independent cross-phase trace oracle; exercises the **A2** planning, **A3** implementing, and **A4** ui_implementing lifecycles in passing; **F2/F4** per-domain commits (planning ACCEPT + per-item + batch). Two **KNOWN BUG** skips: **B6** `plan_all_before_implementing`, and ui_implementing commits |
| `round_budget_test.go` | **C1** implementing min-rounds loop (PASS r1→IMPLEMENT, PASS r2→COMMIT, items "passed"); **C2** implementing force-accept (FAIL r1→IMPLEMENT, FAIL r2≥max→COMMIT, items "failed"); **C3a/C3b/C3c** ui_implementing per-loop force-accept: eval max_rounds, QA max_rounds, and e2e max_rounds exhaustion each independently flip their `*ForceAccepted` flag and mark items "failed" at COMMIT |
| `pipeline_front_test.go` | **B2** specifying PHASE_SHIFT without `--from` auto-generates `.forgectl/state/plan-queue.json` (one entry per domain, `code_search_roots` defaults to `["<domain>/"]`), enters `generate_planning_queue`, and hands off to planning; **B3** specifying PHASE_SHIFT with `--from <file>` skips `generate_planning_queue` entirely and lands at planning ORIENT in one step using the provided queue |

## Conventions for adding tests

- Tag every file `//go:build integration` and keep `package integration`.
- Drive only through `p.forge(...)` (never call into `forgectl/state` to *mutate*
  — reading the persisted file for assertions is fine). For *contract/oracle*
  tests, derive expectations independently of `advance.go` (e.g. a hand-encoded
  transition table), per the plan's anti-circularity goal.
- Give per-file helpers unique names to avoid clashes within the package.

## Findings surfaced while writing these

These are real discrepancies the suite uncovered (the plan frames such tests as
"waiting to fail"):

1. **`[planning.study_specs]` config is silently dropped.** `tomlPlanningConfig`
   has no `study_specs` field, so `mergeTomlConfig` ignores any operator-set
   `[planning.study_specs]`; the loaded value stays at the default. The shipped
   `default-config.toml` documents the block as configurable, and
   `DefaultForgeConfig` has a `StudySpecs` field — so it's a latent
   `default-config.toml` ⟷ loader mismatch. Captured by
   `TestPlanningStudySpecsConfigIsHonored` (skipped with a `KNOWN BUG` note;
   un-skip when the loader is fixed). The §4.4 equivalence test still passes
   because the template's `study_specs` values happen to equal the defaults.

2. **`docs/default-config.toml` is ahead of the embedded template.** The repo's
   existing unit test `state.TestEmbeddedTemplateMatchesDocs` fails on the current
   tree: `docs/default-config.toml` carries `general.prompt_domains` and a
   `[reverse_engineering]` section (also with `prompt_domains`) that the embedded
   `forgectl/state/default-config.toml` lacks — and neither field exists in the Go
   config structs. Pre-existing; not introduced here.

3. **Documentation drift in `OPERATING_MANUAL.md` (specifying).** Two transitions
   the manual states differ from the binary's actual behavior:
   - `RECONCILE_EVAL --verdict PASS` goes to **`RECONCILE_REVIEW`**, not directly
     to `COMPLETE` as the manual's table says.
   - The commit-message flag is **`--message` only** — there is no `-m`
     shorthand, though docs/skills reference one.
   The oracle and lifecycle tests encode the binary's actual (verified) contract;
   these drifts are noted here for the manual/skills to be reconciled (plan §6).

4. **`plan_all_before_implementing: true` implements only the last planned
   domain.** The spec (`plan-production.md` §"ACCEPT/DONE", lines 656–658) says
   all-planning-first should plan every domain (planning→planning boundaries),
   then on the final ACCEPT (queue empty) enter `DONE → PHASE_SHIFT` and
   implement **every** completed plan in turn. The binary instead takes the final
   ACCEPT straight to `PHASE_SHIFT{To: implementing}` for that one plan
   (`advance.go:474–495`); it never enters `DONE` and never builds an implementing
   queue from the completed plans, so all but the last domain are planned and then
   dropped. Captured by `TestPipelinePlanAllBeforeImplementing` (skipped with a
   `KNOWN BUG` note; un-skip when fixed).

5. **`ui_implementing` requires `--message` but never commits.** With
   `enable_commits: true`, the ui phase rejects a missing `--message` at the
   first-round `IMPLEMENT` and at `COMMIT` (`advance.go:942–944`, `1101–1103`),
   exactly like the implementing phase — but neither path calls `AutoCommit`, so
   the message is demanded and then ignored and a commits-on ui batch produces
   **zero** git commits. (The implementing phase does commit at the symmetric
   points, as `TestPipelineMultiDomainCodeCommits` proves.) Captured by
   `TestUIImplementingCommitsWhenEnabled` (skipped with a `KNOWN BUG` note).

6. **`PHASE_SHIFT` label is hardcoded to "planning → implementing".** Even for a
   `kind:"ui"` plan that routes to ui_implementing (and is validated against the
   `ui_implementing` config), the PHASE_SHIFT output reads `From: planning →
   implementing` — the `To` field is set to `PhaseImplementing` regardless of
   `kind` (`advance.go:490–494`), while the actual routing uses `kind`. Cosmetic
   output drift; the multi-domain test asserts the routing reaches the
   `ui_implementing` phase rather than relying on the (wrong) label.

## Not yet implemented (remaining plan items)

The plan is large; this suite delivers the harness, the top of the recommended
sequencing, and the multi-domain cross-phase pipeline. The planning (**A2**),
implementing (**A3**), and ui_implementing (**A4**) lifecycles are now driven
end-to-end inside `pipeline_multidomain_test.go` (via the `drivePipeline` walker),
though dedicated single-phase files asserting each phase's full trifecta in
isolation are still worth adding. Still open: **A5** reverse-engineering, the
specifying→generate_planning_queue front of the pipeline (**B2/B3**), **C3/C4**,
batch/layer/dependency **D**, the full git-strategy matrix **F1/F3/F5**, **H3–H5**,
eval-report handshake **I**, the rest of ui specifics **J1/J2** (the QA-step-list
existence gate is exercised in passing here), remaining standalone commands
**K3/K4**, idempotency **L**, schema oracle **§4.3**, generator round-trip
**§4.5**, skill↔binary contract **§4.6**, fuzzing **§4.7**, and the gated
live-agent suite **§4.8**. The harness (fixture, trifecta, eval handshake, DSL,
oracle pattern, and now the `drivePipeline` multi-phase walker) is built to extend
to all of them.
