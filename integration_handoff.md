# Integration Test Suite — Handoff (updated 2026-06-29)

This document captures where things are, how the harness works, open bugs, and
exactly what remains to implement from `integration-test-plan.md`. Read top to
bottom before continuing.

---

## 1. Current status

**Foundation + a large portion of the plan are DONE and fully green.**
36 test functions across 11 files, 1 deliberate `KNOWN BUG` skip.

```bash
make test-integration
# or
cd forgectl && go test -tags=integration ./integration/...
```

### Covered so far

| Plan items | File | Notes |
|-----------|------|-------|
| A1 specifying full walk | `lifecycle_specifying_test.go` | init→ORIENT→…→PHASE_SHIFT, trifecta |
| A2 planning, A3 implementing, A4 ui_implementing | `pipeline_multidomain_test.go` | exercised "in passing" through the multi-domain pipeline; standalone full-walk files not written |
| B1 full pipeline | `pipeline_multidomain_test.go` | specifying→planning→implementing/ui_implementing end-to-end |
| B2 generate_planning_queue | `pipeline_front_test.go` | auto-generates plan-queue.json at PHASE_SHIFT |
| B3 `--from` override | `pipeline_front_test.go` | skips generate_planning_queue, lands at planning ORIENT |
| B4 `kind` routing | `pipeline_multidomain_test.go` | `kind:"ui"` → ui_implementing, code/absent → implementing |
| B6 plan_all_before_implementing | `pipeline_multidomain_test.go` | bug fixed in HEAD; test passes |
| C1/C2 min_rounds / force-accept | `oracle_specifying_test.go`, `round_budget_test.go` | specifying + implementing |
| C3a/b/c ui_implementing per-loop budgets | `round_budget_test.go` | eval/qa/e2e max_rounds each independently |
| E1/E2/E3 persistence & recovery | `persistence_test.go` | atomic write, backup, root discovery |
| F2 `--message` required at commit points | `pipeline_multidomain_test.go` | missing message → exit 1, no mutation |
| F4 commit hashes in state | `pipeline_multidomain_test.go` | per-item + batch commits |
| G1/G2/G3 config scaffolding | `config_scaffold_test.go` | init scaffold, byte-preserve, partial merge, runtime-mutable |
| H1/H2 activity logging | `logging_test.go` | only init/advance log; stable filename |
| K1 validate surface | `validate_corpus_test.go` | auto-detect type, mismatch error, exit codes |
| K2 `--version` | `lifecycle_specifying_test.go` | smoke test |
| §4.1 specifying oracle | `oracle_specifying_test.go` | independent transition table; randomized walks |
| §4.2 hand-authored corpus | `validate_corpus_test.go` | valid + one-violation-each for spec-queue/plan-queue/plan |
| §4.4 default-config equivalence | `config_scaffold_test.go` | TOML ↔ DefaultForgeConfig() field-by-field |
| §2.5 scripted-session DSL | `dsl_test.go` | Step/RunScript/advStep/evalPassStep |

---

## 2. File map

| Path | What it is |
|------|-----------|
| `forgectl/integration/harness_test.go` | **CRITICAL — shared foundation.** TestMain (builds binary once), Project fixture, forge/mustForge/forgeAt, trifecta inspectors, eval-handshake helpers, output parsing. Edit only to add harness capability needed by multiple tests. |
| `forgectl/integration/dsl_test.go` | Scripted-session DSL (Step, RunScript, advStep, evalPassStep) + a demo test. |
| `forgectl/integration/lifecycle_specifying_test.go` | A1 specifying full walk + binary smoke test. Pattern to copy for A5. |
| `forgectl/integration/validate_corpus_test.go` | §4.2 hand-authored corpus + K1 validate surface. Add corpus fixtures here. |
| `forgectl/integration/config_scaffold_test.go` | G1–G3, §4.4 equivalence, study_specs KNOWN BUG skip. |
| `forgectl/integration/persistence_test.go` | E1–E3 crash recovery / root discovery. |
| `forgectl/integration/oracle_specifying_test.go` | §4.1 specifying oracle (C1/C2). **Pattern to copy** for planning/implementing oracles. |
| `forgectl/integration/logging_test.go` | H1/H2. |
| `forgectl/integration/pipeline_front_test.go` | B2/B3 generate_planning_queue and --from override. |
| `forgectl/integration/pipeline_multidomain_test.go` | B1/B4/B6 multi-domain pipeline; A2/A3/A4 in passing; F2/F4 commits. |
| `forgectl/integration/round_budget_test.go` | C1/C2 implementing rounds; C3a/b/c ui_implementing per-loop force-accept. |
| `forgectl/integration/README.md` | Human-facing overview. **Keep in sync** when adding tests or findings. |
| `Makefile` | `test` and `test-integration` targets. |
| `integration-test-plan.md` | **The spec** — source of truth for what to build. |

**Convention:** `//go:build integration`, `package integration`, unique per-file helper names.

---

## 3. How the harness works

### Black-box binary runner

`TestMain` runs `go build -o <tmp>/forgectl .` once (cwd = module root = `..`
from the package dir). Every test runs that binary via `exec.Command`. This
exercises `main.go`, exit codes, and `go:embed` — things the in-process unit
tests can't reach.

### Isolation by construction

`NewProject(t)` gives you:
- `Root` = `t.TempDir()` with `git init` + repo-local identity (`user.email`/`user.name`).
- `Home` = a separate `t.TempDir()`. Every `forge`/`git` call overrides `HOME`/`USERPROFILE` to `Home`. This keeps `logger.go` and `PruneLogs` out of the real `~/.forgectl/logs/`.

`env()` strips the real `HOME` before appending the override (libc returns the first match; appending alone isn't enough).

### Key helpers

- `p.forge(args...) Result` / `p.mustForge(...)` — `Result{Stdout, Stderr, Exit}` captured separately. `forgeAt(dir, args...)` runs from a subdir.
- `p.WriteConfig(toml)` / `p.WriteFile(rel, content)` — TOML config on disk (not a Go struct; the binary parses it).
- Trifecta: `p.State()`, `p.RawState()`, `p.AssertAt(phase, state)`, `p.GitLog()`, `p.GitShowStat()`, `p.LogFiles()`, `p.LogEntries(name)`.
- **Eval handshake**: `p.ExtractReportPath(out)` parses the path forgectl prints in an EVALUATE-family state; `p.PassEval(prev, "PASS"|"FAIL")` does extract→write stub→`advance --verdict --eval-report` in one call.
- DSL: `RunScript([]Step{...})` with `advStep(...)` and `evalPassStep(...)`.

### Oracle discipline

For *driver* tests, reading `forgectl-state.json` via `p.State()` is fine. For *contract/oracle* tests (§4.1, §4.3), encode expected behavior **independently** of `advance.go` — never import production transition logic to predict production transition logic.

---

## 4. Non-obvious things (save yourself hours)

Drive the binary by hand before writing a new lifecycle: `go build -o /tmp/forgectl-smoke .` in `forgectl/`, then run it in a temp dir with `HOME` pointed at a throwaway.

1. **`init` prints nothing to stdout for specifying ORIENT.** ORIENT is transient; `PrintAdvanceOutput` emits no action block. Assert on persisted state, not init's stdout.
2. **Eval-report paths vary** by eval kind and domain. Never hardcode them — use `ExtractReportPath`. `--eval-report` only checks the file exists, so stub content works.
3. **The actual specifying flow:** `ORIENT → SELECT → DRAFT → EVALUATE → ACCEPT → CROSS_REFERENCE → CROSS_REFERENCE_EVAL → CROSS_REFERENCE_REVIEW → DONE → RECONCILE → RECONCILE_EVAL → RECONCILE_REVIEW → COMPLETE → PHASE_SHIFT`. Commit happens at COMPLETE→PHASE_SHIFT (not ACCEPT). With `enable_commits=true`, that advance **requires `--message`**.
4. **`--message` has no `-m` shorthand.** `advance -m "x"` → `unknown shorthand flag: 'm'`. Skills reference `-m`; binary doesn't register it.
5. **The backup is one save behind.** After init(ORIENT)→advance(SELECT), the `.bak` holds ORIENT. Corrupting `.json` and recovering restores ORIENT, not SELECT. Recovery warnings go to stderr.

---

## 5. Open bugs / findings

1. **`[planning.study_specs]` config is silently dropped.** `tomlPlanningConfig` has no `study_specs` field; `mergeTomlConfig` never merges it. The shipped `default-config.toml` documents the block and `DefaultForgeConfig()` has `StudySpecs`, but an operator setting it is ignored. Captured by `TestPlanningStudySpecsConfigIsHonored` (currently `t.Skip("KNOWN BUG: ...")`). **Un-skip when the loader is fixed.**

2. **Pre-existing unit failure (not ours):** `state.TestEmbeddedTemplateMatchesDocs` fails — `docs/default-config.toml` has `general.prompt_domains` and a `[reverse_engineering]` section that the embedded `forgectl/state/default-config.toml` and Go structs lack. Needs reconciliation (docs file is ahead of the embedded template).

3. **`OPERATING_MANUAL.md` drift:** the manual's table says `RECONCILE_EVAL --verdict PASS → COMPLETE`, but the binary goes to `RECONCILE_REVIEW` first. The manual/skills also reference a `-m` shorthand that doesn't exist. Tests encode the verified binary behavior; manual/skills need reconciling (plan §4.6 will formalize this).

Policy: keep the suite **green**. Confirmed bugs get a `t.Skip("KNOWN BUG: ...")` test that documents the correct behavior and waits to be un-skipped.

---

## 6. What remains (in priority order)

### Priority 1 — Contract oracles (highest bug-yield)

**§4.1 Oracles for planning and implementing** — copy `oracle_specifying_test.go`:
- New file `oracle_planning_test.go`: hand-encode the planning transition table
  (ORIENT→STUDY_SPECS→STUDY_CODE→STUDY_PACKAGES→REVIEW→DRAFT→(VALIDATE)→
  (SELF_REVIEW)→EVALUATE→ACCEPT→DONE→PHASE_SHIFT), run randomized legal walks.
- New file `oracle_implementing_test.go`: hand-encode implementing transitions
  (ORIENT→IMPLEMENT(×N)→EVALUATE→COMMIT→ORIENT(next layer)→…→DONE), including
  the `passes` field transitions (pending→done→passed/failed) and layer gating.

**§4.6 Skill/binary contract tests** — new file `skill_contract_test.go`:
- Parse the skill Markdown files from `../../skills/` and extract every literal
  `forgectl <command> <flags>` invocation and every output cue string (e.g.
  `Review:`, `--- REPORT OUTPUT ---`, report-path patterns, specific flag names).
- Assert (a) every command/flag exists in the binary's `--help` output and
  (b) forgectl actually emits each cue string in the relevant state's output.

**§4.3 Independent schema oracle** — new file `schema_oracle_test.go`:
- Author a JSON Schema (derived from the spec docs, not from `types.go`) for each
  artifact type: spec-queue, plan-queue, plan, reverse-engineering-queue.
- Assert the corpus from `validate_corpus_test.go`: every file forgectl accepts
  passes the independent schema; every file it rejects fails it. Disagreement
  pinpoints whether `validate.go` or the spec is wrong.

### Priority 2 — Missing full-walk lifecycles

**A5 Reverse-engineering full walk** — new file `lifecycle_re_test.go`:
- Drive `init --phase reverse_engineering` through the full cycle:
  per-domain SURVEY→GAP_ANALYSIS→DECOMPOSE→QUEUE, then EXECUTE/POST loop,
  then per-domain RECONCILE→RECONCILE_EVAL→RECONCILE_ADVANCE→DONE.
- First drive by hand to learn the exact flow (same recipe as A1).

**Standalone A2/A3/A4 lifecycle files** — `lifecycle_planning_test.go`,
`lifecycle_implementing_test.go`, `lifecycle_ui_implementing_test.go`:
- The multi-domain pipeline hits these phases but doesn't give them the
  isolated, single-phase treatment A1 got. Add dedicated files that run each
  phase start-to-finish with the full trifecta at every transition.
- Reuse corpus fixtures from `validate_corpus_test.go` (`planQueueValid`, `planValid`).

### Priority 3 — Counter, batch, and git tests

**C4** — Counter reset per batch/domain; round persists across EVALUATE↔REFINE.

**D1/D2/D3** — Batch/dependency layer tests (new file `batch_test.go`):
- D1: batch never mixes domains, never exceeds `batch` config.
- D2: implementing selects only unblocked items; layer N+1 locked until N fully terminal.
- D3: item order within a layer is preserved; one item per IMPLEMENT advance.

**E4** — Session archived to `state_dir/sessions/<domain>-<date>.json` at terminal states.

**F1** — Each `commit_strategy` (`strict`/`all-specs`/`scoped`/`tracked`/`all`) stages
exactly the expected paths — assert via `p.GitShowStat()`.

**F3** — "Nothing to commit" path emits the notice and still advances state.

**F5** — `enable_commits:false` skips all git calls; `--message` accepted but ignored with warning.

### Priority 4 — CLI surface and eval contract

**G4** — `ValidateConfig` rejection paths: bad `commit_strategy`, bad `eval_mode`,
`batch < 1`, `min_rounds > max_rounds`, nested domain paths.

**H3/H4/H5** — Error logging (H3), best-effort logging on read-only `~/.forgectl/logs`
(H4), pruning only at init (H5).

**I1/I2/I3** — Eval-report handshake contract: deterministic path in Action and
REPORT block (I1), nonexistent path → error with hint (I2), mode-gating for
`direct`/`conversational` (I3).

**B5** — UI config gate: `kind:"ui"` at PHASE_SHIFT with empty
`ui_implementing.app.url` fails the shift, stays at PHASE_SHIFT, names the
missing key.

**J1/J2/J3** — ui_implementing specifics: QA step list must exist to advance
QA_TEST→E2E_AUTHOR (J1), empty step list passes e2e vacuously (J2), handoff
registers files and surfaces under `Review:` without transitioning state (J3).

**K3** — `add-queue-item`/`set-roots`/`add-domain` state-gating, domain
inference, duplicate-name rejection, file-existence checks.

**K4** — Every error path exits non-zero to stderr; success guidance to stdout.

**L1/L2/L3** — Idempotency/rejection invariants: no half-mutations on failed
advance (L1), `init` over existing state refuses (L2), flag-context guards
(`--file` only in DRAFT, `--verdict` only in eval states, QUEUE rejects
`--file`) (L3).

### Priority 5 — Property/fuzz and round-trip

**§4.5** — generate_planning_queue round-trip: feed forgectl-produced
`plan-queue.json` through `validate --type plan-queue`, a fresh
`init --phase planning --from <it>`, and the independent schema from §4.3.

**§4.7** — Property/fuzz generation of inputs; assert validate.go never panics,
dependency-graph checks handle adversarial cycles/ordering.

### Last (gated, not in normal CI)

**§4.8** — Live real-agent e2e suite. Runs in a disposable container; actual
sub-agents generate specs/plans/code/eval reports; forgectl drives a real small
project to a built, e2e-passing artifact. Never as a normal `go test`.

---

## 7. Recipe for a new lifecycle test

1. `cd forgectl && go build -o /tmp/forgectl-smoke .`
2. In a scratch temp dir with `HOME` set to a throwaway, drive the phase by hand; note every transition, output cues, and eval-report paths.
3. Encode as `RunScript([]Step{...})` or imperatively with `mustForge`/`PassEval`, asserting the **trifecta** at the points that matter.
4. For contract assertions, hand-encode expectations (don't import `advance.go`).
5. `go test -tags=integration ./integration/...` until green; update README coverage table + any new findings.

---

## 8. Source-of-truth files

- `forgectl/state/types.go` — all JSON schemas, state/phase constants, `DefaultForgeConfig()`.
- `forgectl/state/validate.go` — exact validation error strings.
- `forgectl/state/config.go` — TOML merge logic (study_specs bug lives here).
- `forgectl/state/state.go` — `Load`/`Save`/`Recover` semantics.
- `forgectl/cmd/*.go` — flag definitions and guard/error messages.
- `forgectl/specs/phase-transitions.md` + `forgectl/OPERATING_MANUAL.md` — documented contract for oracles (remember the verified drifts in §5).
- `skills/` — downstream skill invocations and output cues (source for §4.6).
