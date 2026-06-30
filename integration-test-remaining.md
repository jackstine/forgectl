# Integration Test — Remaining Items

Items from the integration-test-plan that are not yet implemented. Each entry names the plan section, the test file it belongs in, and what to assert. Items with `[ ]` are open; skip this doc once they are all checked.

---

## A. Full phase lifecycles (A2–A5)

- [ ] **A2 Planning full walk** (`lifecycle_planning_test.go`)
  Drive `init --phase planning --from plan-queue.json` through the complete sequence: `ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT → (VALIDATE) → (SELF_REVIEW) → EVALUATE(PASS) → ACCEPT → DONE → PHASE_SHIFT`. Assert: per-plan commit at ACCEPT when `enable_commits:true`; plan added to `completed[]`; `PHASE_SHIFT` fires at the end with correct `.To` label.

- [ ] **A3 Implementing full walk** (`lifecycle_implementing_test.go`)
  Multi-layer plan with several items per layer. Drive `ORIENT → IMPLEMENT(×N) → EVALUATE → COMMIT → ORIENT(next layer) → … → DONE`. Assert: per-item first-round commit; batch eval advances the entire layer; `passes` array transitions `pending→done→passed/failed`; layer N+1 stays locked until layer N is fully terminal; `DONE` fires after the final layer.

- [ ] **A4 ui_implementing full walk** (`lifecycle_ui_implementing_test.go`)
  One batch through all three loops: `IMPLEMENT → EVALUATE → QA_TEST → E2E_AUTHOR → E2E_VERIFY → COMMIT → DONE`. Then drive both refine sub-loops: `UI_REFINE` (after eval FAIL within budget) and `E2E_REMEDIATE` (after e2e FAIL within budget). Assert item is marked `failed` when any loop force-accepts.

- [ ] **A5 Reverse-engineering full walk** (`lifecycle_re_test.go`)
  `init --phase reverse_engineering` → per-domain loop `SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE` → `EXECUTE/POST` loop → per-domain `RECONCILE → RECONCILE_EVAL → RECONCILE_ADVANCE → DONE`. Assert `reverse-engineering-queue.json` consumed correctly; completed entries recorded in state.

---

## B. Cross-phase pipelines (B5)

- [ ] **B5 UI config gate at boundary** (`pipeline_multidomain_test.go` or new file)
  Plan with `kind:"ui"` but `ui_implementing.app.url` empty (or other required field missing). Drive to the `PHASE_SHIFT` boundary from planning. Assert: advance exits non-zero, forgectl **stays at PHASE_SHIFT**, and the error message names the missing config key.

---

## C. Round / counter / budget logic (C4)

- [ ] **C4 Counter reset per batch/domain** (`round_budget_test.go`)
  Drive two consecutive batches in specifying (or implementing). Assert `EvalRound` resets to 0 at the start of each new batch. Also assert `EvalRound` persists correctly across `EVALUATE ↔ REFINE` within the same batch.

---

## D. Batch selection, layers, and dependencies (D1–D3)

- [ ] **D1 Batch never mixes domains; never exceeds `batch`** (`batch_test.go`)
  Seed a `spec-queue.json` with specs from multiple domains interleaved; set `batch:2`. Assert every `SELECT` advance picks at most 2 specs all from the same domain. Drive until all specs are processed and assert no batch ever contained mixed domains.

- [ ] **D2 Implementing selects only unblocked items** (`batch_test.go`)
  Plan with two layers: layer 0 has items A, B; layer 1 has item C with `depends_on:[A,B]`. Drive layer 0 to completion. Assert item C is not presented until both A and B are terminal. Assert during layer 0 that C is never returned by IMPLEMENT.

- [ ] **D3 Item order within a layer preserved** (`batch_test.go`)
  Plan with three items in the same layer, declared in order X→Y→Z. Assert forgectl presents them in exactly that order across successive IMPLEMENT advances.

---

## E. State persistence & recovery (E4)

- [ ] **E4 Session archived at terminal states** (`persistence_test.go`)
  Drive a specifying session to `COMPLETE`. Assert a file exists at `<state_dir>/sessions/<domain>-<date>.json` and that it contains a snapshot of the completed session.

---

## F. Git auto-commit integration (F2, F3, F5)

- [ ] **F2 `enable_commits:true` requires `--message` at commit points** (`git_commit_test.go`)
  With `enable_commits:true`, drive to a commit point (specifying COMPLETE, planning ACCEPT, implementing first-round IMPLEMENT). Call `advance` without `--message`. Assert: exit non-zero, error on stderr, **state unchanged** (no mutation).

- [ ] **F3 "Nothing to commit" path** (`git_commit_test.go`)
  With `enable_commits:true`, reach a commit point with a clean working tree (no staged changes). Assert: advance exits 0, emits the "nothing to commit" notice, and still transitions state.

- [ ] **F5 `enable_commits:false` suppresses git calls** (`git_commit_test.go`)
  With `enable_commits:false`, drive through a commit point while passing `--message`. Assert: no git commit is created (`git log` unchanged), `--message` is silently ignored (or a warning is emitted), and state transitions normally.

---

## G. Config & scaffolding (G4)

- [ ] **G4 ValidateConfig rejection surface** (`config_scaffold_test.go`)
  Call `init` (or `advance`) with configs that should be rejected. Assert exit non-zero and the documented error string for each case:
  - `commit_strategy` set to an unknown value
  - `eval_mode` set to an unknown value
  - `batch < 1`
  - `min_rounds > max_rounds`
  - Nested domain paths (a domain that is an ancestor/descendant of another)

---

## H. Activity logging (H3–H5)

- [ ] **H3 Error entries logged before non-zero exit** (`logging_test.go`)
  Trigger a known error path (e.g. `advance` with a missing required flag). Assert the `.jsonl` log contains an entry with `cmd:"error"` written before forgectl exits non-zero.

- [ ] **H4 Read-only log dir → warning, still exits 0** (`logging_test.go`)
  Set `$HOME` to a temp dir where `~/.forgectl/logs` is created with mode `0444`. Run `advance`. Assert: exits 0, a warning about the log dir is emitted to stderr, and the command otherwise succeeds.

- [ ] **H5 Pruning runs only at init** (`logging_test.go`)
  Seed `~/.forgectl/logs/` with files that exceed `retention_days` and `max_files`. Run `status` and `eval` — assert no files are removed. Run `init` — assert the old files are pruned. Assert `advance` also does not prune.

---

## I. Eval-report contract (I1–I3)

- [ ] **I1 Deterministic report path in eval output** (`eval_report_test.go`)
  In `report` eval mode, run `eval` when forgectl is in EVALUATE. Assert the path shown in the `Action:` line and the path shown in the `--- REPORT OUTPUT ---` block are byte-identical.

- [ ] **I2 `advance --eval-report` path validation** (`eval_report_test.go`)
  - Nonexistent path → exit non-zero.
  - Prose-looking value (no path separators, no `.md`) → exit non-zero, error includes the "path not prose" hint.
  - Valid path → recorded in `EvalRecord.EvalReport` in state.

- [ ] **I3 Mode-gating: direct/conversational don't surface report** (`eval_report_test.go`)
  With `eval_mode:"direct"` or `eval_mode:"conversational"`, run `eval` in EVALUATE state. Assert: no `--- REPORT OUTPUT ---` block, no report path in output, and `advance` does not require `--eval-report`.

---

## J. ui_implementing specifics (J1–J3)

- [ ] **J1 QA step list must exist to exit QA_TEST → E2E_AUTHOR** (`ui_implementing_test.go`)
  Drive to QA_TEST. Delete the QA step list file that was registered. Call `advance --verdict pass`. Assert: exits non-zero, error names the missing file path, state stays at QA_TEST.

- [ ] **J2 Empty step list passes e2e vacuously** (`ui_implementing_test.go`)
  Register a QA step list that contains zero scenarios (valid JSON, empty `steps` array). Drive through E2E_AUTHOR → E2E_VERIFY. Assert: forgectl accepts it without error and transitions to COMMIT.

- [ ] **J3 `handoff` registers files** (`ui_implementing_test.go`)
  Call `forgectl handoff <file>` with an existing file; assert state unchanged and the file appears under `Review:` in the next `status` output. Call `handoff` again with a different file (latest-wins); assert the new file is shown and the old one is not. Call `handoff` with a nonexistent file; assert exit non-zero. Assert `handoff` does not transition state in any case.

---

## K. Standalone commands & CLI surface (K3–K4)

- [ ] **K3 `add-queue-item` / `set-roots` / `add-domain` guards** (`cli_surface_test.go`)
  - `add-queue-item`: requires an active session in the correct phase; duplicate name → error; file existence checked when a file ref is provided.
  - `set-roots`: domain inferred from current session; rejected outside planning.
  - `add-domain`: rejected when a domain with the same name already exists.

- [ ] **K4 Every error path exits non-zero to stderr** (`cli_surface_test.go`)
  Enumerate the documented error paths (missing required flag, wrong phase, bad artifact, unknown command). For each: assert exit code ≠ 0 and error text goes to stderr (not stdout).

---

## L. Idempotency & rejection invariants (L1–L3)

- [ ] **L1 Failed `advance` never half-mutates state** (`idempotency_test.go`)
  Before each error-path advance call, snapshot `forgectl-state.json`. Trigger the error. Assert the state file is byte-identical to the snapshot. Exception: the documented VALIDATE-error save (advance saves state before returning the validation error) — assert only that specific mutation occurs.

- [ ] **L2 `init` over existing state refuses** (`idempotency_test.go`)
  Run `init` successfully. Run `init` again. Assert: exits non-zero, error includes "delete to reinitialize", original state file unchanged.

- [ ] **L3 Flag-context guards** (`idempotency_test.go`)
  - `--file` outside specifying DRAFT → error.
  - `--verdict` outside an eval state → error.
  - `--file` at QUEUE state → error.
  Assert each exits non-zero and does not mutate state.

---

## §4 Independent oracles (4.3, 4.5–4.7)

- [ ] **4.3 Independent schema oracle** (`validate_corpus_test.go` or `schema_oracle_test.go`)
  Author a JSON Schema for each artifact type (`spec-queue`, `plan-queue`, `plan`, `reverse-engineering-queue`, eval report, QA step list) derived from the spec docs — not from `types.go`. Assert agreement with `forgectl validate`: every artifact forgectl accepts passes the independent schema; every artifact it rejects fails it. Disagreement pinpoints a spec/validate.go mismatch.

- [ ] **4.5 Generator-output round-trip through independent validator** (`validate_corpus_test.go`)
  Drive generate_planning_queue to produce `plan-queue.json`. Feed the output through: (a) `forgectl validate --type plan-queue`, (b) `init --phase planning --from <it>`, and (c) the independent schema from 4.3. Assert all three accept it.

- [ ] **4.6 Skill/binary contract tests** (`skill_contract_test.go`)
  Parse the `skills/` Markdown files and extract every literal `forgectl` invocation and output cue string (e.g. `Review:`, `--- REPORT OUTPUT ---`, specific flags). Assert: (a) every flag/subcommand the skills reference exists in `forgectl --help` / cobra's command tree, and (b) forgectl actually emits each cue string in the relevant state's output.

- [ ] **4.7 Property/fuzz inputs** (`fuzz_test.go`)
  Use Go native fuzzing / `testing/quick` to generate random valid and malformed JSON for each artifact. Assert: `validate.go` never panics on any input; valid-by-construction inputs are accepted; the advance decision function is total over arbitrary verdict/round combinations. Focus adversarial input on dependency-graph paths (cycles, later-layer deps, dangling refs).

---

## Not tracked here (intentionally deferred)

- **4.8 Live real-agent e2e** — gated, nightly/manual suite; not part of normal CI. See §4.8 of the plan for container/VM requirements.
- **4.1 Oracle for planning/implementing/ui/RE** — the oracle approach (`TestOracleSpecifying*`) exists; extending it to the remaining phases is part of A2–A5 above.
