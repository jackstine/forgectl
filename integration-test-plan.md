# Forgectl Integration Test Plan

## 0. The central insight that should shape every test

**Forgectl is a *driver*, not a *generator***. It never writes specs, plans, code, eval reports, QA step lists, or e2e tests — sub-agents (Claude) and operators produce those. Forgectl's job is to:

1. **Consume** externally-produced artifacts (spec-queue.json, plan-queue.json, plan.json, eval reports, QA step lists, reverse-engineering-queue.json, the operator's verdicts/commit messages).
2. **Decide** the next state from a deterministic state machine + config + round counters.
3. **Emit** guidance (status/eval/advance text the skills and operator read) and **persist** side effects (state.json, moved spec files, generated queues, git commits, activity logs).

That gives us two distinct failure classes, and the test strategy must hit both:

| Failure class | Example | What catches it |
|---|---|---|
| **Driver bugs** | wrong transition, counter off-by-one, batch picks a blocked item, commit at the wrong point | End-to-end CLI workflows over a real FS/git, asserting the persisted trifecta |
| **Contract bugs** | forgectl's reader/writer diverges from the documented schema/output that agents & skills depend on | Cross-checks against **independently generated** artifacts and oracles (Section 4) |

The existing Go suite is strong but **partly circular**: it builds state in-memory with the same types forgectl serializes with, and it calls `Advance()` directly rather than the CLI. A serialization or schema-interpretation bug can pass both the writer and the reader. The integration layer must (a) drive the **real CLI** end-to-end, and (b) verify behavior against artifacts and models produced **outside forgectl's own code**.

---

## 1. Coverage gaps the integration layer must close

From the coverage audit, unit tests cover individual transitions, validation rules, config defaults, git strategies, and output rendering. **No test currently drives:**

- A **complete phase lifecycle** start-to-finish (e.g. specifying `ORIENT → … → COMPLETE`).
- Any **cross-phase / PHASE_SHIFT chain** (specifying → planning → implementing).
- The **handoff → status surfacing → verdict** pipeline end-to-end.
- The **eval-report two-actor handshake** (eval emits path → agent writes file → advance consumes path) as a flow.
- **ui_implementing** through a full three-loop batch to COMMIT.
- **reverse_engineering** concept-to-completion.
- **Config scaffolding on init**, git auto-commit + state mutation as one transaction, crash recovery.

These are exactly the integration tests.

---

## 1a. Isolation & side-effect containment (prerequisite for the whole suite)

All deterministic suites (§3, §4.1–4.7) must run fully inside a sandbox. Two seams escape the project root and must be plugged:

- **`$HOME` (mandatory).** `logger.go` resolves the activity log via `os.UserHomeDir()`, **not** the project root: `NewLogger` writes `~/.forgectl/logs/<phase>-<id>.jsonl` (logger.go:42–52), and **`PruneLogs` runs at every `init`** and will `os.Remove` files in the **real** `~/.forgectl/logs/` per the test's `retention_days`/`max_files` (logger.go:87–131). Every test that runs `init`/`advance` must `t.Setenv("HOME", t.TempDir())` (+ `USERPROFILE` on Windows). This also detaches `git` from the real `~/.gitconfig`.
- **git identity.** Set a **repo-local** `user.email`/`user.name` in the temp repo so commits don't depend on global config.
- **Everything else is project-root-relative** → contained by `t.TempDir()` (on macOS under `$TMPDIR`, auto-cleaned): `.forgectl/config`, state files (`.json`/`.bak`/`.tmp`/`.corrupt`), the git repo + commits, generated `plan-queue.json`/`plan.json`, moved spec files, session archives. Build temp projects fresh; never point a test at the live repo.
- **Live real-agent e2e (§4.8) is the sole exception:** it runs generated code and the configured `launch_command`/`test_command` (ports, processes, network). Run it in a disposable container/VM with its own HOME/temp/network policy — never as a normal `go test`.

## 2. Harness to build first (one-time investment)

1. **Black-box CLI runner.** Drive forgectl through the **cobra command layer** (`cmd.Execute` path / `rootCmd.SetArgs`), not `state.Advance()` directly, capturing **stdout, stderr, and exit code** separately. Add a thin smoke suite that runs the **actually built binary** via `exec.Command` so `main.go`, embedding (`go:embed` of evaluators + default-config.toml), and the build itself are exercised. **Bake the `$HOME`+git-identity isolation from §1a into the shared fixture so no individual test can forget it.**
2. **Project fixture builder.** Create a temp project: `git init`, write `.forgectl/config` (TOML on disk — *not* a Go struct), seed domain dirs and spec/plan files. Reuse the existing `t.TempDir`, `initTestGitRepo()`, `setupProjectDir()` helpers.
3. **Assertion trifecta helper.** After every `advance`, assert on all three side-channels at once: (a) CLI output + exit code, (b) `forgectl-state.json` contents (phase/state/counters/queues), (c) external side effects — files moved/created, `git log`, `~/.forgectl/logs/*.jsonl`.
4. **Golden-file infrastructure.** `testdata/` corpus of **hand-authored** input artifacts (see Section 4) + golden status/eval output snapshots with an `-update` flag.
5. **Scripted-session format.** A tiny table/DSL describing a sequence of `(command, flags) → expected (phase, state, exit)` so full lifecycles read as data, not 200-line test bodies.

---

## 3. Integration test catalog (driver correctness — "does it work completely")

### A. Full phase lifecycles (each phase, init → terminal)
- **A1 Specifying full walk:** `init --phase specifying --from spec-queue.json` → `ORIENT → SELECT → DRAFT → EVALUATE(PASS) → ACCEPT → … → CROSS_REFERENCE → CROSS_REFERENCE_EVAL → CROSS_REFERENCE_REVIEW → DONE → RECONCILE → RECONCILE_EVAL → RECONCILE_REVIEW → COMPLETE → PHASE_SHIFT`. Assert spec files moved to `completed[]`, batch is domain-homogeneous, counters, and (with `enable_commits`) a single commit at COMPLETE with the hash registered on every completed spec.
- **A2 Planning full walk:** `ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT → (VALIDATE) → (SELF_REVIEW) → EVALUATE → ACCEPT → DONE → PHASE_SHIFT`. Assert per-plan commit at ACCEPT; plan added to `completed[]`.
- **A3 Implementing full walk:** multi-layer, multi-item plan: `ORIENT → IMPLEMENT(×N items) → EVALUATE → COMMIT → ORIENT(next layer) → … → DONE`. Assert per-item first-round commit, batch eval, `passes` transitions `pending→done→passed/failed`, layer gating.
- **A4 ui_implementing full walk:** one batch through all three loops `IMPLEMENT → EVALUATE → QA_TEST → E2E_AUTHOR → E2E_VERIFY → COMMIT → DONE`, plus the refine sub-loops (`UI_REFINE`, `E2E_REMEDIATE`).
- **A5 Reverse-engineering full walk:** `init --phase reverse_engineering` → per-domain `SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE` loop → `EXECUTE/POST` loop → per-domain `RECONCILE → RECONCILE_EVAL → RECONCILE_ADVANCE → DONE`.

### B. Cross-phase pipelines (PHASE_SHIFT chains)
- **B1 Full pipeline:** specifying → generate_planning_queue → planning → implementing, driven entirely through the CLI, one continuous session.
- **B2 generate_planning_queue auto-generation:** assert `<state_dir>/plan-queue.json` is generated, groups completed specs by domain in order, dedups `spec_commits`, and uses `set-roots` roots when present.
- **B3 `--from` override at specifying PHASE_SHIFT:** skips generate_planning_queue, lands at planning `ORIENT`.
- **B4 `kind` routing:** plan `kind:"ui"` routes to ui_implementing; `kind:"code"`/absent routes to implementing — verified at the PHASE_SHIFT boundary.
- **B5 UI config gate at boundary:** `kind:"ui"` with an empty `ui_implementing.app.url` (etc.) fails the shift, **stays at PHASE_SHIFT**, names the missing key.
- **B6 `plan_all_before_implementing` true vs false:** interleaved (plan→impl→plan→impl) vs all-plan-then-all-impl, with mixed `kind` re-routing at each implementation domain boundary.

### C. Round / counter / budget logic (the decision core)
- **C1** `min_rounds` not met + PASS → loops (REFINE / re-IMPLEMENT), does **not** accept.
- **C2** `max_rounds` reached + FAIL → **force-accept** (items marked `failed`, specs/plans accepted).
- **C3** ui_implementing **three independent budgets**: drive eval/qa/e2e to *different* max_rounds; assert each counter advances only while its loop is active and force-accept of any loop ⇒ item `failed` at COMMIT.
- **C4** Counter reset at each new batch/domain; round persists across EVALUATE↔REFINE.

### D. Batch selection, layers, dependencies
- **D1** Batch never mixes domains (specifying); never exceeds `batch`.
- **D2** Implementing selects only **unblocked** items (`depends_on` all terminal) from the **current layer**; layer N+1 stays locked until layer N fully terminal.
- **D3** Item order within a layer preserved; one item presented per IMPLEMENT advance.

### E. State persistence & recovery (real FS crash injection)
- **E1** Atomic write: `.tmp → .bak → .json` sequence; relative `state_dir` resolved from project root, absolute used as-is.
- **E2** Recovery: delete `.json` leaving `.bak` → next command restores + warns; corrupt `.json` with valid `.bak` → `.json.corrupt` created, restored, warned; stale `.tmp` cleaned.
- **E3** Project-root discovery walks up to `.forgectl/` but **stops at the git boundary** (a `.forgectl/` above the git root is not adopted).
- **E4** Session archived to `state_dir/sessions/<domain>-<date>.json` at terminal states.

### F. Git auto-commit integration
- **F1** Each `commit_strategy` (`strict`, `all-specs`, `scoped`, `tracked`, `all`) stages exactly the expected paths — assert via `git show --stat`.
- **F2** `enable_commits:true` ⇒ `--message` **required** at commit points (specifying COMPLETE, planning ACCEPT, implementing first-round IMPLEMENT + COMMIT); missing message → error, **no state mutation**.
- **F3** "nothing to commit" path emits the notice and still advances.
- **F4** Commit hashes recorded in state (`commit_hashes`); per-item commits in implementing first round (crash-safety granularity).
- **F5** `enable_commits:false` ⇒ no git calls; `--message` ignored with warning.

### G. Config & scaffolding
- **G1** `init` with no flags scaffolds `.forgectl/` + default config, exits 0, prints the domains notice **once**; re-run is a no-op no-notice (idempotent).
- **G2** Existing config never overwritten (byte-for-byte preserved).
- **G3** Partial config merges onto defaults; config **locked into state** at init, only `general.user_guided` mutable at runtime (`--guided`/`--no-guided`).
- **G4** `ValidateConfig` rejections surfaced: bad `commit_strategy`, bad `eval_mode`, `batch < 1`, `min_rounds > max_rounds`, nested domain paths.

### H. Activity logging
- **H1** Only `init`/`advance` log; `status`/`eval`/`validate`/`--version` do not.
- **H2** Log filename `<initial-phase>-<sessionid[:8]>.jsonl`; unchanged across phase shifts.
- **H3** Error entries logged before non-zero exit with `cmd:"error"`.
- **H4** Logging is best-effort: read-only `~/.forgectl/logs` → warning, command still exits 0.
- **H5** Pruning runs **only at init** (age `retention_days`, count `max_files`).

### I. Eval-report contract (two-actor handshake) — see also Section 4
- **I1** `eval` (report mode) surfaces the **same deterministic path** in the Action and the `--- REPORT OUTPUT ---` block — byte-identical.
- **I2** `advance --eval-report <nonexistent>` fails; prose-looking value yields the "path not prose" hint; valid path recorded in `EvalRecord.EvalReport`.
- **I3** Mode-gating: `direct`/`conversational` modes don't surface/require a report path.

### J. ui_implementing specifics
- **J1** QA step list **must exist** to exit QA_TEST → E2E_AUTHOR; delete it after a PASS → error names the path, stays QA_TEST.
- **J2** Empty step list (zero scenarios) passes e2e vacuously.
- **J3** `handoff` registers files (all-or-nothing existence check), latest-wins, surfaces under `Review:` in status, and does **not** transition state.

### K. Standalone commands & CLI surface
- **K1** `validate` auto-detects type by top-level key; `--type` mismatch → hinted error; works with no `.forgectl/`; correct exit codes.
- **K2** `--version` works with no session, before/after init, identical string.
- **K3** `add-queue-item` / `set-roots` / `add-domain` state-gating, domain inference, duplicate-name rejection, file-existence checks.
- **K4** Every error path exits non-zero to **stderr**; success guidance to stdout.

### L. Idempotency & rejection invariants
- **L1** A failed/invalid `advance` never half-mutates state (except the documented VALIDATE-error save).
- **L2** `init` over an existing state file refuses ("delete to reinitialize").
- **L3** Flag-context guards: `--file` only in specifying DRAFT, `--verdict` only in eval states, QUEUE rejects `--file`.

---

## 4. ⭐ Tests that generate artifacts/oracles INDEPENDENTLY of forgectl (the high-value set)

These exist specifically to break the **circularity** of "forgectl writes it, forgectl reads it, both agree." Each derives its truth from a source **outside forgectl's own Go code** — the spec docs, a hand-authored corpus, an independent model, or a live agent. Ranked by value.

### 4.1 — Independent state-machine **oracle** (model-based testing) — *highest value*
`OPERATING_MANUAL.md` and `specs/phase-transitions.md` contain the transition tables in human-readable form. **Encode the transition table independently** (parse the Markdown table directly, or transcribe it into a standalone data table in the test — deliberately *not* importing `advance.go`'s logic). Then:
- Drive forgectl through **randomized but legal** action sequences; after each step assert forgectl's `(phase, state)` and counter equal the oracle's prediction.
- Any divergence is either an implementation bug or a stale manual — both are release blockers.

This is the single most powerful guard against the documented contract and the binary drifting apart, and the table already exists to seed it.

### 4.2 — Hand-authored **golden fixture corpus** (artifacts an agent would produce)
A `testdata/` library of input files written **by hand to the spec**, never produced by forgectl:
- `spec-queue.json`, `plan-queue.json` (incl. `kind:"code"|"ui"|absent`), `plan.json` (multi-layer, refs, deps, all test categories), `reverse-engineering-queue.json`, eval-report `.md` files, QA `batch-N-steps.json`.
- For each: a **valid** variant forgectl must accept and **one-violation-each** invalid variants (missing field, extra field, bad `kind`, dangling `depends_on`, dependency cycle, later-layer dependency, nonexistent ref/file/code_search_root, bad test category).
- Run them through `forgectl validate` **and** through real `init`/`advance`, asserting the exact documented error strings. Because the corpus is independent of forgectl's writer, this tests the **reader against the contract**, not against itself.

### 4.3 — Independent **schema oracle** cross-check
Author a JSON Schema (or equivalent independent validator) for each artifact **derived from the spec docs**, not from `types.go`. Then assert **agreement** on the corpus: every file forgectl accepts passes the independent schema, every file it rejects fails it, and vice-versa. Disagreement pinpoints whether `validate.go` or the spec is wrong. This catches under-validation (forgectl accepts something the contract forbids) that golden negatives might miss.

### 4.4 — `default-config.toml` ⟷ `DefaultForgeConfig()` equivalence
The TOML template is embedded and shipped; `DefaultForgeConfig()` is the Go source of truth. They are **two independent encodings of the same defaults** (and a known mismatch is already suspected — see the config-scaffolding spec). Test: load the embedded TOML, compare field-by-field to `DefaultForgeConfig()`; assert the scaffolded default **passes `ValidateConfig`** and produces the same effective config as an empty config. This is a pure independent-source cross-check and a likely current-bug finder.

### 4.5 — Generator-output-is-valid-input round trip, checked by an *independent* validator
forgectl auto-generates `plan-queue.json` in generate_planning_queue. Feed that **forgectl-produced** file back through (a) `forgectl validate --type plan-queue`, (b) a fresh `init --phase planning --from <it>`, **and** (c) the independent schema from 4.3. The independent validator (c) is the key: it proves the generated artifact satisfies the *documented* contract, not merely forgectl's own round-trip.

### 4.6 — **Skill/binary contract tests** (downstream consumers as independent oracle)
The `skills/` are authored to the spec, independently of (and currently ahead of) the binary. They embed literal forgectl invocations and rely on specific **output cues** (e.g., the `Review:` line, `--- REPORT OUTPUT ---`, report-path format, specific flags). Test: extract those literal commands/cues from the skill Markdown and assert (a) every flag/command the skills use exists in forgectl, and (b) forgectl actually emits each cue string in the relevant state's output. This catches skill↔binary drift from the consumer side — an oracle forgectl's own tests can't provide.

### 4.7 — Property/fuzz generation of inputs
Use Go native fuzzing / `testing/quick` to generate random well-formed and malformed JSON for each artifact (generation logic independent of forgectl's marshaller). Assert: `validate.go` **never panics**; valid-by-construction inputs are accepted; the `advance` decision function is total over arbitrary verdict/round combinations. Especially valuable for the dependency-graph checks (cycles, layer ordering) where adversarial inputs find edges.

### 4.8 — Live **real-agent** end-to-end (the ultimate independent generator) — *gated suite*
A separate, non-deterministic, expensive suite (nightly/manual, not in unit CI) where **actual sub-agents** generate the specs, plans, code, eval reports, QA steps, and e2e tests, and forgectl drives a real small project from `specifying` to a built, e2e-passing artifact. Here every consumed artifact is maximally independent of forgectl. This is the only test that answers "does the *whole loop* work completely," and it should run against the real built binary in a clean container.

---

## 5. Suggested sequencing

1. **Harness + golden corpus (4.2) + black-box runner** — unblocks everything else.
2. **Full lifecycles A1–A5** and **pipeline B1** — close the biggest "works completely" gaps.
3. **Independent oracles 4.1, 4.3, 4.4** — highest contract-bug yield, likely to surface existing defects (config mismatch, manual drift).
4. **Counter/batch/git/persistence C–F**, **eval-report I/4.5**, **ui J**, **logging H**.
5. **Skill contract 4.6**, **fuzz 4.7**.
6. **Live agent e2e 4.8** as a gated release-gate suite.

## 6. Known discrepancies to resolve while writing these (each is a test waiting to fail)
- `default-config.toml` vs `DefaultForgeConfig()` mismatch (4.4).
- Workspace dir naming: config default `.forge_workspace` vs `.forgectl_workspace` dirs present in the tree — confirm path resolution writes/reads the configured name consistently.
- Manual vs implementation: confirm `STUDY_PACKAGES`, `SELF_REVIEW`, `CROSS_REFERENCE*`, and the ui refine/remediate sub-loops in `OPERATING_MANUAL.md` exactly match `advance.go` (4.1 will flush these out).
