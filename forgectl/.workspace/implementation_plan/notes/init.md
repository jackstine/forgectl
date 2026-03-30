# Init Command Notes

## Flag Changes

### Removed flags
- `--batch-size` (int, required) — now from `.forgectl/config`
- `--min-rounds` (int) — now from `.forgectl/config`
- `--max-rounds` (int, required) — now from `.forgectl/config`
- `--guided` (bool) — now from `.forgectl/config` (`general.user_guided`)
- `--no-guided` (bool) — same

### Kept flags
- `--from <path>` (required) — path to input file
- `--phase` (default: specifying) — starting phase

### Global flag removed
- `--dir` on root command — replaced by automatic project root discovery

## New Init Sequence

```
1. FindProjectRoot() — walk up for .forgectl/
2. LoadConfig(projectRoot) — read .forgectl/config, apply defaults
3. ValidateConfig(cfg) — check constraints per phase
4. If config invalid: print errors, exit 1
5. Read --from file
6. Validate --from against phase schema
7. If invalid: print errors + schema, exit 1
8. Generate session_id = uuid.New().String()
9. Create ForgeState with Config locked in, session_id set
10. state_dir = StateDir(projectRoot, cfg)
11. os.MkdirAll(state_dir)
12. state.Save(state_dir, s)
13. If cfg.Logs.Enabled:
    a. PruneLogs(cfg.Logs)
    b. CreateLogFile(s.Phase, session_id)
    c. WriteLogEntry(init entry)
14. PrintAdvanceOutput(out, s, state_dir)
```

## Plan Queue Schema Change

The `PlanQueueEntry` struct changes:
- Remove `Topic string`
- Add `SpecCommits []string`

Updated JSON example:
```json
{
  "plans": [
    {
      "name": "Protocols Implementation Plan",
      "domain": "protocols",
      "file": "protocols/.forge_workspace/implementation_plan/plan.json",
      "specs": ["protocols/ws1/specs/ws1-message-contract.md"],
      "spec_commits": ["7cede10", "8743b1d"],
      "code_search_roots": ["api/", "optimizer/"]
    }
  ]
}
```

`ValidatePlanQueue` must be updated:
- `topic` is no longer a valid field (reject if present)
- `spec_commits` is required (must be array, may be empty)

## State File Path

Old: state file at `stateDir` (default `.`, user-specified via `--dir`).
New: state file at `<projectRoot>/<config.paths.state_dir>/forgectl-state.json`.

`os.MkdirAll` must be called on the state dir before saving.

## Config Lock

The entire `Config` struct is stored in `ForgeState.Config`. During the session, only `config.general.user_guided` is mutable (via `--guided`/`--no-guided` on `advance`). All other config fields are read-only after init.

## Existing State Check

The existing `state.Exists(stateDir)` check needs updating to use the new state dir resolution. The error message stays the same: "State file already exists. Delete it to reinitialize."
