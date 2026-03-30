# Activity Logging Notes

## Overview

JSONL logging to `~/.forgectl/logs/`. One log file per session. Best-effort: failures print warning to stderr, never fail commands.

## Log File Location

```
~/.forgectl/logs/<phase>-<session_id_prefix>.jsonl
```

Where `session_id_prefix` = first 8 characters of UUID.
File name is determined at init and does not change when phases shift.

## Log Entry Format

Each line is a JSON object:

```json
{"ts":"2026-03-29T14:32:01Z","cmd":"init","phase":"specifying","state":"ORIENT","detail":{...}}
{"ts":"2026-03-29T14:33:12Z","cmd":"advance","phase":"specifying","prev_state":"ORIENT","state":"SELECT","detail":{...}}
```

Fields:
- `ts`: ISO 8601 UTC timestamp
- `cmd`: `init`, `advance`, `add-commit`, `reconcile-commit`
- `phase`: current phase
- `prev_state`: state before transition (advance only)
- `state`: state after command completes
- `detail`: command-specific map (may be empty `{}`)

## Commands That Log

- `init` — logs after state file created
- `advance` — logs after every state transition
- `add-commit` — logs after successful registration
- `reconcile-commit` — logs after registration

## Commands That Do NOT Log

- `status`, `eval`, `validate`, `--version`, `add-queue-item`, `set-roots`

## Detail Fields

**init:**
```json
{"from":"spec-queue.json","batch_size":3,"rounds":"1-3","guided":true}
```

**advance (SELECT at ORIENT→SELECT):**
```json
{"batch":["repository-loading","snapshot-diffing"],"domain":"optimizer"}
```

**advance (EVALUATE→REFINE):**
```json
{"round":1,"verdict":"FAIL","eval_report":"optimizer/specs/.eval/batch-1-r1.md"}
```

**add-commit:**
```json
{"spec_id":1,"spec_name":"Repository Loading","hash":"7cede10"}
```

**reconcile-commit:**
```json
{"hash":"8743b1d","matched_specs":[{"id":2,"name":"Snapshot Diffing"}]}
```

## Logger Implementation

```go
type Logger struct {
    enabled bool
    path    string  // absolute path to log file
}

func NewLogger(cfg LogsConfig, phase string, sessionID string) *Logger {
    if !cfg.Enabled {
        return &Logger{enabled: false}
    }
    prefix := sessionID[:8]
    filename := fmt.Sprintf("%s-%s.jsonl", phase, prefix)
    home, err := os.UserHomeDir()
    if err != nil {
        warnf("activity logging: cannot get home dir: %v", err)
        return &Logger{enabled: false}
    }
    logDir := filepath.Join(home, ".forgectl", "logs")
    return &Logger{enabled: true, path: filepath.Join(logDir, filename)}
}

func (l *Logger) Write(entry LogEntry) {
    if !l.enabled {
        return
    }
    data, err := json.Marshal(entry)
    if err != nil {
        warnf("activity logging: marshal: %v", err)
        return
    }
    f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        warnf("activity logging: open: %v", err)
        return
    }
    defer f.Close()
    f.Write(append(data, '\n'))
}
```

## Pruning

Runs at `init`, before new log file is created:

```go
func PruneLogs(cfg LogsConfig) {
    if !cfg.Enabled {
        return
    }
    home, _ := os.UserHomeDir()
    logDir := filepath.Join(home, ".forgectl", "logs")
    files, err := filepath.Glob(filepath.Join(logDir, "*.jsonl"))
    if err != nil || len(files) == 0 {
        return
    }
    // Sort by modification time (oldest first)
    sort.Slice(files, func(i, j int) bool {
        si, _ := os.Stat(files[i])
        sj, _ := os.Stat(files[j])
        return si.ModTime().Before(sj.ModTime())
    })
    now := time.Now()
    // Age-based deletion
    if cfg.RetentionDays > 0 {
        cutoff := now.AddDate(0, 0, -cfg.RetentionDays)
        var remaining []string
        for _, f := range files {
            si, _ := os.Stat(f)
            if si.ModTime().Before(cutoff) {
                os.Remove(f)
            } else {
                remaining = append(remaining, f)
            }
        }
        files = remaining
    }
    // Count-based deletion
    if cfg.MaxFiles > 0 {
        for len(files) >= cfg.MaxFiles {
            os.Remove(files[0])
            files = files[1:]
        }
    }
}
```

## Log File Resolution for Subsequent Commands

After init, subsequent commands (`advance`, `add-commit`, `reconcile-commit`) must resolve the log file path from the state file:
- `s.SessionID` provides the UUID
- `s.Phase` (initial phase) must be stored for the filename

The log file name uses the **initial phase** (phase at init time), not the current phase. Store the initial phase in state (already have `StartedAtPhase`) and derive the log filename from `StartedAtPhase` + `SessionID`.

So: `logPath = ~/.forgectl/logs/<started_at_phase>-<session_id[:8]>.jsonl`

## Init Log Entry Detail

The `detail` for `init` uses values from the locked config:
```go
detail := map[string]interface{}{
    "from":       initFrom,
    "batch_size": getBatchSize(s.Config, phase),  // phase-appropriate batch
    "rounds":     fmt.Sprintf("%d-%d", getMinRounds(...), getMaxRounds(...)),
    "guided":     s.Config.General.UserGuided,
}
```
