
## Spec-Alignment Fixes — Post-Batch (2026-04-04)

**Scope:** Cross-cutting spec-alignment fixes across config.go, logging.go, validate.go, and cmd/validate.go.

**Changes:**

1. **config.go — missing config file errors** (session-init spec, rejection table): `LoadConfig` now returns an error when `.forgectl/config` is missing, instead of silently returning defaults. Updated `TestLoadConfigMissing` to expect error; added `TestLoadConfigEmptyFile` for empty-file-returns-defaults.

2. **config.go — boolean TOML defaults preserved** (session-init spec, edge case): Converted all `bool` fields in `toml*Config` structs to `*bool` pointers. `mergeTomlConfig` now only overwrites defaults when the TOML value is explicitly set (`!= nil`). Prevents omitted booleans from overwriting `true` defaults with `false`.

3. **config.go — ValidateConfig constraint checks** (session-init spec, steps + config tables): Added missing validations: `batch >= 1` for all three phases, `logs.retention_days >= 0`, `logs.max_files >= 0`.

4. **config.go — domain path nesting error format** (session-init spec, line 198): Changed error message from `domain "X" path is a prefix of domain "Y" path (nested paths not allowed)` to spec-required `Domain paths must not be nested: <path1> is a prefix of <path2>.`

5. **logging.go — stderr warnings** (activity-logging spec, lines 105-108, 142-144): `WriteLogEntry` and `PruneLogFiles` now print warnings to stderr on failure instead of silently returning. Added `fmt` import.

6. **logging.go — Detail always serialized** (activity-logging spec, line 72): Removed `omitempty` from `Detail` JSON tag. `WriteLogEntry` initializes nil `Detail` to `map[string]any{}` before marshaling.

7. **cmd/validate.go — output format** (validate-command spec, output section): Rewrote `runValidate` to match spec output formats: `Detected:` line with key name, `Validated:` success line with entry count, numbered `Error: validation failed with N errors:` listing, auto-detection failure with expected-keys table and `Hint:`, `--type` mismatch error with hint about correct type. Updated all validate command tests to match new output format.

---

## Batch 10 — L4 logging.core (2026-04-03)

**Item:** `[logging.core]` Activity logging: JSONL session logs and pruning  
**Commit:** fbb5dec  
**Result:** PASS (round 2)

Added `state/logging.go` with `LogEntry`, `WriteLogEntry` (append-mode JSONL, best-effort), `PruneLogFiles` (age + count pruning), `NowTS`, `LogDir`, and `LogFileName`. Wired best-effort logging into `cmd/init.go` (with prune-on-init) and `cmd/advance.go` (pre-capture snapshot, post-advance log write). All gated on `s.Config.Logs.Enabled`.

Round 1 FAIL: `PruneLogFiles` with `retentionDays=0` deleted all files — `time.Now().AddDate(0,0,0)` equals "now", so every file was "before now". Fixed with zero-value `time.Time` sentinel: only compute cutoff when `retentionDays > 0`, guard with `!cutoff.IsZero()` before comparing.

**Notable:** `go:embed` path restriction (`../` not allowed) required creating `forgectl/evaluators/evaluators.go` as a dedicated package co-located with the `.md` files (done in batch 8/9). JSONL log file named `<startPhase>-<first-segment-of-sessionID>.jsonl` for grouping by session prefix.

---

## Batch 1 — L0 types.config (2026-04-03)

**Item:** `[types.config]` ForgeConfig struct hierarchy  
**Commit:** 1fd7e3c  
**Result:** PASS (round 2)

Added the full ForgeConfig struct hierarchy to `state/types.go` and wired it into ForgeState. Round 1 FAIL flagged that ForgeState was missing Config ForgeConfig and still had old flat fields — both fixed in round 2 along with updating all callers (advance.go, output.go, cmd/init.go, all tests).

**Notable:** AgentConfig uses Go struct embedding to produce flat JSON promotion (model/type/count at same level as parent fields), matching state-persistence.md schema. DefaultForgeConfig() provides spec-defined defaults.
