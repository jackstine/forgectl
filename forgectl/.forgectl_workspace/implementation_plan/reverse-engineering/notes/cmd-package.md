# Notes — `cmd/` package changes

The cmd/ changes are thin wrappers; transition/validation logic lives in `state/`.

## init.go — accept the reverse_engineering phase

Existing: `--phase` (default specifying) parsed via `state.ParsePhase`; `--from`
dispatched per phase in a switch (specifying→`ValidateSpecQueue`,
planning→`ValidatePlanQueue`, implementing→`ValidatePlan`); state built via
`state.NewState(...)`; logging + pruning already wired.

Add:
- Once `ParsePhase` accepts `reverse_engineering`, add a switch case that:
  - reads `--from`, runs `state.ValidateReverseEngineeringInput`,
  - parses `{concept, domains}`,
  - constructs the state with `ReverseEngineering` populated: Concept, Domains,
    DomainIndex=1, DomainCount=len(domains), state=`ORIENT`, ColleagueReview from
    locked config, ReconcileRound=0, ExecuteItemIndex=0, QueueFilePath/Hash empty.
  - locks the `[reverse_engineering]` config (already loaded into `Config`).
- All existing init plumbing (root discovery, config validation, session_id, log
  file + pruning, "state file already exists" rejection) is reused unchanged.

## advance.go — no structural change

Existing flags `--verdict --eval-report --message --file --guided/--no-guided`
are forwarded to `state.Advance` via `AdvanceOpts`. The QUEUE `--file` rejection
is handled inside `advanceReverseEngineering` (it inspects `opts.File`). The
advance command just needs to keep forwarding `--file`. Ensure the RE advance
result is logged (detail with domain/state) — see logging note.

## adddomain.go — NEW command

Create `cmd/adddomain.go` (no separators, matching `addqueueitem.go`/
`setroots.go`) mirroring an existing simple command: `var addDomainCmd =
&cobra.Command{Use:"add-domain <domain>", Args: cobra.ExactArgs(1), RunE:
runAddDomain}`; `init()` → `rootCmd.AddCommand(addDomainCmd)`.

Behavior (reverse-engineering.md "add-domain"):
- Load state. Require phase==reverse_engineering AND state==QUEUE, else error
  `"forgectl add-domain is only available during the QUEUE state."`.
- Arg `<domain>` required.
- Reject duplicate (already in `Domains`) → `domain "<d>" already exists`.
- Append to `Domains`, update `DomainCount`, persist via the state write layer.
- The SURVEY→QUEUE loop does NOT re-run for the added domain (user already did
  the analysis). add-domain is read/write of state only; it is NOT a logged
  command (only init/advance are logged) — consistent with set-roots/add-queue-item.

## validate.go — NO new type (per recorded decision)

The RE queue's top-level key is `"specs"`, colliding with spec-queue
auto-detection; validate-command.md's concrete interface defines only
`spec-queue`, `plan-queue`, `plan`. The RE validators are exposed as shared
functions (used by init + QUEUE advance), satisfying the "same validation logic"
integration point. Do not add a `reverse-engineering-queue` detection key or
`--type` value. (If a future decision wants standalone RE-queue validation, it
must be override-only because auto-detection cannot disambiguate `"specs"`.)

## eval.go — activate during RECONCILE_EVAL

Existing `runEval` (cmd/eval.go) switches on (phase, state): specifying
RECONCILE_EVAL → `state.PrintReconcileEvalOutput`, specifying
CROSS_REFERENCE_EVAL → `PrintCrossRefEvalOutput`, planning/implementing →
`PrintEvalOutput`; default errors.

Add a case for `phase==reverse_engineering && state==RECONCILE_EVAL` → the RE
eval renderer (see evaluators note; rendering lives in `state/output.go`). Keep
it blocked in other RE states with the spec message "forgectl eval is only
available during RECONCILE_EVAL." `eval` remains read-only (no log entry).
