# Notes — `state/` package changes

The bulk of the reverse_engineering work lives here. Confirmed file layout:
phase/state/config/struct definitions are ALL in `types.go`; the state machine is
`advance.go`; output is `output.go`; validation is `validate.go`; TOML
load/merge is `config.go`; constructors + persistence are `state.go`; logging is
`logger.go`.

## types.go — phase, states, config, structs (one file)

### Phase constant (types.go:6-11)
Existing block defines `PhaseSpecifying/PhasePlanning/PhaseGeneratePlanningQueue/
PhaseImplementing`. Add:
- `PhaseReverseEngineering PhaseName = "reverse_engineering"`.

### State constants (types.go:16-40)
`StateName` consts are a single flat set; `StateOrient`, `StateReconcile`,
`StateReconcileEval`, `StateDone` already exist and are **reused**. Add the new
RE-only states:
- `StateSurvey = "SURVEY"`, `StateGapAnalysis = "GAP_ANALYSIS"`,
  `StateDecompose = "DECOMPOSE"`, `StateQueue = "QUEUE"`,
  `StateExecuteReverseEngineer = "EXECUTE_REVERSE_ENGINEER"`,
  `StatePostReverseEngineer = "POST_REVERSE_ENGINEER"`,
  `StateColleagueReview = "COLLEAGUE_REVIEW"`,
  `StateReconcileAdvance = "RECONCILE_ADVANCE"`.

### Config structs (types.go:45-209)
`AgentConfig{Model,Type,Count}` is the shared sub-agent shape. Phase configs are
structs (e.g. `PlanningConfig`) added to `ForgeConfig` and seeded in
`DefaultForgeConfig()`. Add:
- `ReverseEngineeringConfig` with `Execute AgentConfig`, `Survey AgentConfig`,
  `GapAnalysis AgentConfig`, and `Reconcile REReconcileConfig`.
- `REReconcileConfig{ MinRounds, MaxRounds int; ColleagueReview bool;
  Eval AgentConfig }` (eval defaults opus/general-purpose/1).
- Add `ReverseEngineering ReverseEngineeringConfig` to `ForgeConfig` (types.go:143).
- Seed defaults in `DefaultForgeConfig()`: execute=haiku/explorer/3,
  survey=haiku/explorer/2, gap_analysis=sonnet/explorer/5,
  reconcile min=1/max=3/colleague_review=false/eval=opus/general-purpose/1.

### State struct + queue/init schemas (after types.go:439)
Mirror `PlanningState`. Add:
- `ReverseEngineeringInitInput{ Concept string; Domains []string }`.
- `REQueueEntry{ Name, Domain, Topic, File, Action string; CodeSearchRoots
  []string; DependsOn []string }` and `ReverseEngineeringQueueInput{ Specs
  []REQueueEntry }` (top-level key is `specs`).
- `ReverseEngineeringState{ Concept string; Domains []string; DomainIndex int;
  DomainCount int; ExecuteItemIndex int; ReconcileRound int; QueueFilePath
  string; QueueContentHash string; ColleagueReview bool; Queue []REQueueEntry;
  DomainReconcile map[string]*ReconcileState }`. (`ReconcileState{Round int;
  Evals []EvalRecord}` already exists at types.go:347 — reuse it per domain.)
- Add `ReverseEngineering *ReverseEngineeringState json:"reverse_engineering"` to
  `ForgeState` (types.go:452). Pointer stays nil for other phases.

## state.go — constructor + persistence

Existing `NewSpecifyingState/NewPlanningState/NewImplementingState` build the
initial phase state. Add `NewReverseEngineeringState(concept string, domains
[]string) *ReverseEngineeringState` setting DomainIndex=1, DomainCount=len,
ColleagueReview from config (or set in init), empty Queue,
DomainReconcile=map{}. `Load/Save/Exists/Archive` already round-trip the whole
`ForgeState`, so the new pointer persists automatically.

## config.go — TOML decode + merge + validation

Existing: `tomlForgeConfig` mirrors the TOML; `mergeTomlConfig` copies non-zero
values onto `DefaultForgeConfig()`; per-section `mergeXConfig` helpers;
`ValidateConfig` checks commit strategies, batch>=1, min<=max, nested domains.
Add:
- `tomlReverseEngineeringConfig` (+ nested toml structs for execute/survey/
  gap_analysis/reconcile/reconcile.eval) on `tomlForgeConfig`.
- merge logic in `mergeTomlConfig` (copy model/type/count when non-empty/>0;
  `ColleagueReview *bool` so false is explicit; min/max when >0).
- `ValidateConfig`: `reverse_engineering.reconcile.min_rounds` must not exceed
  `max_rounds` (mirror the planning/eval checks at config.go:387-395).

## advance.go — the state machine (3 plan items)

Existing: `Advance(s, in, dir)` switches `s.Phase` → `advancePlanning()` etc.
Each per-phase func switches `s.State`, mutates counters, returns nil or a
`*ValidationError`. `AdvanceInput{Verdict,EvalReport,Message,File,From,Guided}`.

Add `case PhaseReverseEngineering: return advanceReverseEngineering(s, in, dir)`
to the dispatch, and implement `advanceReverseEngineering`. Split across three
plan items (same function, intra-layer deps):

1. **Domain loop** (`transitions.re.domain-loop`): ORIENT→SURVEY (DomainIndex=1);
   SURVEY→GAP_ANALYSIS→DECOMPOSE→QUEUE; QUEUE advance:
   - reject `in.File != ""` with the exact fixed-path error.
   - first advance (QueueContentHash==""): read convention path
     `<projectRoot>/.forgectl/state/reverse-engineering-queue.json`, store
     QueueFilePath, compute+store hash, `ValidateReverseEngineeringQueue`
     (schema + domains ∈ Domains + code_search_roots exist under
     `<projectRoot>/<domain>/`). Parse into `Queue`.
   - subsequent (hash set): re-read; equal hash → "Queue file has not changed.";
     else recompute + re-validate + re-parse.
   - on success: if DomainIndex<DomainCount → SURVEY, DomainIndex++; else →
     EXECUTE_REVERSE_ENGINEER, ExecuteItemIndex=1 (reject empty Queue first).
2. **Execute loop** (`transitions.re.execute-loop`): on entry reject empty Queue;
   before emitting an item ensure `<projectRoot>/<domain>/specs/` exists (mkdir);
   EXECUTE→POST; POST→EXECUTE (ExecuteItemIndex++) while items remain, else
   →RECONCILE (DomainIndex=1, ReconcileRound=1). Failed items skipped silently.
3. **Reconcile loop** (`transitions.re.reconcile-loop`): RECONCILE→RECONCILE_EVAL;
   RECONCILE_EVAL consumes `in.Verdict`/`in.EvalReport`, appends an EvalRecord to
   the current domain's `DomainReconcile`, applies min/max:
   FAIL & round<max → RECONCILE (round++); PASS & round<min → RECONCILE
   (round++); (PASS & round>=min) or (FAIL & round>=max) → COLLEAGUE_REVIEW if
   `ColleagueReview` else RECONCILE_ADVANCE; COLLEAGUE_REVIEW→RECONCILE_ADVANCE;
   RECONCILE_ADVANCE → RECONCILE(next domain, round=1) while domains remain, else
   →DONE.

All path resolution uses the absolute `dir` (project root), never cwd.

## validate.go — RE validators (2 plan items) + hashing

Pattern: validators take `[]byte` (+ context), return `[]string` of messages;
top-level + per-entry required/allowed-field checks; `ValidatePlanJSON` does
`os.Stat` for path existence; `detectCycle` (validate.go:282) is reusable for the
queue's depends_on graph. Add:
- `ValidateReverseEngineeringInput(data) []string` — concept non-empty; domains
  non-empty; no duplicates; no extra top-level fields. Add
  `ReverseEngineeringInitSchema()` string.
- `ValidateReverseEngineeringQueue(data, projectRoot string, validDomains
  []string) []string` — top-level `specs` array, non-empty; each entry's
  required fields (name/domain/topic/file/action/code_search_roots/depends_on);
  no extra fields; `action ∈ {create,update}`; `domain ∈ validDomains` (else
  error suggesting `forgectl add-domain <domain>`); `code_search_roots` non-empty
  and each dir exists under `<projectRoot>/<domain>/`; reuse `detectCycle` over
  depends_on. Add `ReverseEngineeringQueueSchema()` string.
- **content hash**: add a small `HashBytes([]byte) string` (sha256 hex) helper
  (new `state/hash.go`) for QUEUE change detection; unit-test it.

## output.go — action rendering (`output.re` item)

Existing: `PrintAdvanceOutput` switches `s.Phase` → `printPlanningOutput` etc.;
each per-state branch builds text with `fmt.Fprintf`, interpolating config like
`s.Config.Specifying.Eval.Type`. `PrintStatus` prints header + current action +
progress, with verbose per-phase sections. `PrintReconcileEvalOutput`/
`PrintEvalOutput` render the eval prompt for `forgectl eval`.

Add:
- `case PhaseReverseEngineering: printReverseEngineeringOutput(w, s, dir)` in
  `PrintAdvanceOutput`.
- `printReverseEngineeringOutput` switching over all RE states, emitting each
  action block from reverse-engineering.md verbatim in spirit: ORIENT (concept +
  domain list with indices), SURVEY/GAP_ANALYSIS ("Spawn {count} {model} {type}
  sub-agents" from `s.Config.ReverseEngineering.Survey/GapAnalysis` + the
  topic-of-concern rules for GAP_ANALYSIS/DECOMPOSE), QUEUE (first-vs-subsequent
  variants + fixed path), EXECUTE (item i/M, domain, spec, action, target file,
  topic, code_search_roots + root-of-core-code definition + execute spawn line),
  POST (the STOP/clear-context message verbatim), RECONCILE (spec-file listing
  with action + depends_on, round 1 vs subsequent), RECONCILE_EVAL (forgectl eval
  instruction + reconcile.eval spawn line + report path), COLLEAGUE_REVIEW,
  RECONCILE_ADVANCE (next domain / DONE), DONE.
- A verbose "Reverse Engineering" section in `PrintStatus`.
- The RE branch of the eval prompt — see evaluators.md.

## logger.go / cmd advance — logging (`log.re` item)

`logger.go` already writes init/advance/error JSONL named
`<phase>-<sessionID[:8]>.jsonl`, best-effort, with pruning at init. No structural
change. `cmd/advance.go:buildAdvanceDetail` currently only adds verdict +
eval_report. Extend it (or the RE advance path) so RE advances populate `detail`
with `domain` (current domain) and state context (round in RECONCILE_EVAL, item
index in the execute loop). The `reverse_engineering-` filename prefix is
automatic from `StartedAtPhase`.

## git.go — staging during RECONCILE

`AutoCommit(root, strategy, targets, msg)` already stages per strategy. RECONCILE
tells the user to `git add` modified spec files; auto-commit only fires when
`config.general.enable_commits`. No new git functions required.