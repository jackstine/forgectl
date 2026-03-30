# State Notes

## ForgeState Schema Changes

### Fields removed from top-level ForgeState
- `BatchSize int` — now `config.specifying.batch`, `config.planning.batch`, `config.implementing.batch`
- `MinRounds int` — now per-phase in config
- `MaxRounds int` — now per-phase in config
- `UserGuided bool` — now `config.general.user_guided`
- `StartedAtPhase PhaseName` — retained

### Fields added to top-level ForgeState
- `Config Config` — full TOML config locked at init
- `SessionID string` — UUID v4 generated at init

### SpecifyingState changes
- `CurrentSpec *ActiveSpec` → `CurrentSpecs []*ActiveSpec` (batch, null when no batch active)
- Add `Domains map[string]DomainMeta` — stores code search roots per domain
- Add `CrossReference map[string]*CrossReferenceState` — per-domain cross-reference state

### New types for specifying

```go
type DomainMeta struct {
    CodeSearchRoots []string `json:"code_search_roots"`
}

type CrossReferenceState struct {
    Domain string       `json:"domain"`
    Round  int          `json:"round"`
    Evals  []EvalRecord `json:"evals,omitempty"`
}
```

### ActiveSpec changes
- Add `Domain string` — domain this spec belongs to (for batch grouping)
- Add `DomainPath string` — filesystem path to domain directory
- Remove: no structural changes beyond domain grouping

### CompletedSpec changes
- Remove `CommitHash string` (legacy single field) — use `CommitHashes []string` only
- Add `DomainPath string` — filesystem path to domain directory

### PlanQueueEntry changes
- Remove `Topic string` — no longer in schema
- Add `SpecCommits []string` — git commit hashes from completed specs

### ActivePlan changes
- Add `SpecCommits []string` — stored in state alongside other plan metadata

### PlanningState changes
- `ActivePlan.SpecCommits []string` stored in current_plan

## State File Location

The state file path is derived from `config.paths.state_dir`:
- If relative: resolved from project root
- If absolute: used as-is
- Default: `.forgectl/state`

All commands must call `FindProjectRoot()` first, then construct the state dir path.

### Updated state.go signatures

```go
// StateDir returns the absolute path to the state directory.
func StateDir(projectRoot string, cfg Config) string {
    if filepath.IsAbs(cfg.Paths.StateDir) {
        return cfg.Paths.StateDir
    }
    return filepath.Join(projectRoot, cfg.Paths.StateDir)
}
```

The `--dir` flag on the root command is removed. All commands resolve state dir via `FindProjectRoot()` + config.

## Session Archiving

Completed sessions archived to: `<state_dir>/sessions/<domain>-<date>.json`

Archive happens when the session reaches terminal state (DONE in implementing, or PHASE_SHIFT reached from complete specifying).

## Status Output Changes

### Compact format (default)
Shows: session path, phase, state, config summary, current action, and one-line Progress.

Progress line varies by phase:
- Specifying: `Progress: N/T specs completed, Q queued`
- Planning: `Progress: round N of M`
- Implementing: `Progress: N/T passed, F failed, R remaining`

### Verbose format (--verbose / -v)
Appends full phase sections after the progress line:
- `--- Specifying ---` with completed specs + eval history
- `--- Planning ---` with eval history
- `--- Implementing ---` with per-layer, per-item status

### Session header change
Old: `Session: forgectl-state.json`
New: `Session: .forgectl/state/forgectl-state.json` (relative to project root, or abs if abs)

### Config display change
Old: `Config:  rounds=1-3, batch_size=2, guided=true`
New: `Config:  batch=N, rounds=min-max, guided=true` (using config object values)
