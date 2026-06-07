# Notes: Derived Docs (diagrams / schemas / config docs)

Backs the L4 docs items. Docs live at the **repo root** `docs/` (outside the `forgectl/` domain); from the domain root they are `../docs/...`. Per CLAUDE.md "Keeping Docs in Sync", docs are derived — update them to match the implemented Go types/specs.

## docs/schemas/ (item: docs-schemas)

- `docs/schemas/forge-state.md`: add `"ui_implementing"` to the `phase` value list; add a `UIImplementingPhaseConfig` section to the ConfigObject table (app.*, eval/qa/e2e loops, batch, commit_strategy); add the `ui_implementing` state-values section (QA_TEST, UI_REFINE, E2E_AUTHOR, E2E_VERIFY, E2E_REMEDIATE); document `UIBatchState` (eval_round/qa_round/e2e_round + the three eval histories + handed_off_artifacts + the three force-accept flags) and the `ui_implementing` ForgeState section.
- `docs/schemas/plan-queue.md`: add the `kind` field (`code`/`ui`, optional, default `code`) to the entry table; update the "exactly the 6 fields" validation rule to include `kind`.
- NEW `docs/schemas/qa-step-list.md`: the QA step-list JSON schema (`batch`, `round`, `scenarios[]`; each scenario `id`, `name`, `preconditions[]`, `steps[]`, `expected[]`, `priority`) — the QA_TEST → E2E_AUTHOR contract (ui-batch-implementation.md §Data Models).
- `docs/schemas/plan-json.md`: no change needed (`kind` lives on the plan-queue entry, not plan.json).

## docs/configurations.md + docs/default-config.toml (item: docs-config)

- `docs/configurations.md`: add a full `[ui_implementing]` section (batch, commit_strategy, app.launch_command/url/ready_timeout_seconds, and eval/qa/e2e loops with min/max/model/type/count/eval_mode, plus e2e.test_command/test_dir); add `ui_implementing` to the `--phase` values; add `QA_TEST, E2E_VERIFY` to the `--verdict` context; add the `ui_plan/` workspace path to the directory structure.
- `docs/default-config.toml`: append a commented `[ui_implementing]` block (+ `[ui_implementing.app]`, `[ui_implementing.eval]`, `[ui_implementing.qa]`, `[ui_implementing.e2e]`) mirroring the `[implementing]` style and the defaults in `DefaultForgeConfig()`.

## docs/diagrams/ (item: docs-diagrams)

Most diagrams were ALREADY updated for the phase in `c18fcf7` (and edits in the working tree): `00-full-lifecycle.txt`, `03-implementing-phase.txt`, `04-cli-commands.txt`, `06-state-machine-complete.txt`, `07-evaluation-loop.txt`, `08-data-flow.txt`, and the new `10-ui-implementing-phase.txt` all already depict the phase, `kind` routing, the three loops, and the config keys. This item is a **reconciliation pass**: verify those diagrams against the final implemented state names / config field names, and check `05-skills-and-roles.txt` (the one diagram not yet confirmed) for a QA/e2e sub-agent role mention. Fix only drift; do not rewrite what is already correct.
