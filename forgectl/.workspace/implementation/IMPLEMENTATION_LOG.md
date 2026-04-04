
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
