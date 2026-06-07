# Notes: Config TOML Decode / Merge / Validate

Backs the L0 config-toml and config-validate items. Files relative to `forgectl/` domain root.

## TOML decode + merge — `state/config.go`

- TOML mirror structs: `tomlImplementingConfig` at `config.go:87-91`, `tomlForgeConfig` at `config.go:135-146`. Optional scalar fields use pointer types (`*bool`, `*string`) so "unset" is distinguishable from zero (see `tomlEvalConfig.EvalMode *string` at `config.go:27`).
- Add `tomlUIImplementingConfig` (parallel to `tomlImplementingConfig`) with nested `[ui_implementing.app]`, `[ui_implementing.eval]`, `[ui_implementing.qa]`, `[ui_implementing.e2e]` tables. Add field `UIImplementing tomlUIImplementingConfig \`toml:"ui_implementing"\`` to `tomlForgeConfig`.
- `mergeTomlConfig` (`config.go:190-283`, implementing block at `253-260`): add a UIImplementing merge block. Pattern: non-zero guard for ints/strings, pointer-deref for bools; reuse `mergeEvalConfig` (`config.go:285-307`) for each of `Eval`/`QA`/`E2E.EvalConfig`. Merge `App.*`, `E2E.TestCommand`, `E2E.TestDir` with non-empty guards.

## Validation — `ValidateConfig` in `state/config.go:422-504`

Structural checks (always run, mirror the `implementing` checks):
- `ui_implementing.batch >= 1` else error.
- For each of `eval`/`qa`/`e2e`: `min_rounds <= max_rounds` else error naming the loop.
- `ui_implementing.commit_strategy` in `validStrategies`; each loop's `eval_mode` in `validEvalModes`.
- Use the same error string style, e.g. `ui_implementing.qa.min_rounds (%d) > ui_implementing.qa.max_rounds (%d)`.

Required-key checks (phase-conditional) are NOT here — they belong at the init/phase-shift boundary where the phase is known (see notes/commands.md and notes/state-machine.md). `ValidateConfig` runs for every phase, so the four UI keys cannot be unconditionally required here.

## The four required UI keys

`ui_implementing.app.launch_command`, `ui_implementing.app.url`, `ui_implementing.e2e.test_command`, `ui_implementing.e2e.test_dir` must each be non-empty when entering `ui_implementing` (via `init --phase ui_implementing` or the planning→ui_implementing phase shift). Empty → error naming each missing key, exit 1, stay at the boundary (session-init.md + phase-transitions.md rejection tables; ui-batch-implementation.md "init rejects each missing required UI config key").

## Docs to update (see notes/docs.md)

`docs/configurations.md` (+`[ui_implementing]` section, `--phase`/`--verdict` updates) and `docs/default-config.toml` (+`[ui_implementing]` block with inline comments) — these mirror the struct/default decisions above and must stay in sync (CLAUDE.md "Keeping Docs in Sync").

## Tests (`state/config_test.go`)

- TOML load/override for a full `[ui_implementing]` block (mirror `TestLoadConfigToml`).
- Defaults applied when `[ui_implementing]` absent (mirror "Init applies defaults").
- `ValidateConfig` rejections: batch<1; min>max for each loop; bad commit_strategy; bad eval_mode.
