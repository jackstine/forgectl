# Config Notes

## New Dependencies

Two new packages are required:

- `github.com/BurntSushi/toml` — TOML parsing for `.forgectl/config`
- `github.com/google/uuid` — UUID v4 generation for `session_id`

Run in `forgectl/`:
```
go get github.com/BurntSushi/toml
go get github.com/google/uuid
```

## TOML Config Structure

`.forgectl/config` is a TOML file read at `init` time and never re-read during a session.

### Full schema (all fields with defaults)

```toml
[specifying]
batch = 3

[specifying.eval]
min_rounds = 1
max_rounds = 3
agent_type = "opus"
agent_count = 1

[specifying.cross_reference]
min_rounds = 1
max_rounds = 2
agent_type = "haiku"
agent_count = 3
user_review = false

[specifying.cross_reference.eval]
agent_type = "opus"
agent_count = 1

[specifying.reconciliation]
min_rounds = 0
max_rounds = 3
agent_type = "opus"
agent_count = 1
user_review = false

[planning]
batch = 1

[planning.study_code]
agent_type = "haiku"
agent_count = 3

[planning.eval]
min_rounds = 1
max_rounds = 3
agent_type = "opus"
agent_count = 1

[planning.refine]
agent_type = "opus"
agent_count = 1

[implementing]
batch = 2

[implementing.eval]
min_rounds = 1
max_rounds = 3
agent_type = "opus"
agent_count = 1

[paths]
state_dir = ".forgectl/state"
workspace_dir = ".forge_workspace"

[general]
user_guided = true
enable_commits = false

[logs]
enabled = true
retention_days = 90
max_files = 50
```

## Go Types

```go
// Config is the top-level TOML config structure locked into state at init.
type Config struct {
    Specifying    SpecifyingConfig    `toml:"specifying"    json:"specifying"`
    Planning      PlanningConfig      `toml:"planning"      json:"planning"`
    Implementing  ImplementingConfig  `toml:"implementing"  json:"implementing"`
    Paths         PathsConfig         `toml:"paths"         json:"paths"`
    General       GeneralConfig       `toml:"general"       json:"general"`
    Logs          LogsConfig          `toml:"logs"          json:"logs"`
}

type EvalConfig struct {
    MinRounds  int    `toml:"min_rounds"  json:"min_rounds"`
    MaxRounds  int    `toml:"max_rounds"  json:"max_rounds"`
    AgentType  string `toml:"agent_type"  json:"agent_type"`
    AgentCount int    `toml:"agent_count" json:"agent_count"`
}

type AgentConfig struct {
    AgentType  string `toml:"agent_type"  json:"agent_type"`
    AgentCount int    `toml:"agent_count" json:"agent_count"`
}

type CrossReferenceConfig struct {
    MinRounds  int         `toml:"min_rounds"  json:"min_rounds"`
    MaxRounds  int         `toml:"max_rounds"  json:"max_rounds"`
    AgentType  string      `toml:"agent_type"  json:"agent_type"`
    AgentCount int         `toml:"agent_count" json:"agent_count"`
    UserReview bool        `toml:"user_review" json:"user_review"`
    Eval       AgentConfig `toml:"eval"        json:"eval"`
}

type ReconciliationConfig struct {
    MinRounds  int    `toml:"min_rounds"  json:"min_rounds"`
    MaxRounds  int    `toml:"max_rounds"  json:"max_rounds"`
    AgentType  string `toml:"agent_type"  json:"agent_type"`
    AgentCount int    `toml:"agent_count" json:"agent_count"`
    UserReview bool   `toml:"user_review" json:"user_review"`
}

type SpecifyingConfig struct {
    Batch          int                  `toml:"batch"          json:"batch"`
    Eval           EvalConfig           `toml:"eval"           json:"eval"`
    CrossReference CrossReferenceConfig `toml:"cross_reference" json:"cross_reference"`
    Reconciliation ReconciliationConfig `toml:"reconciliation" json:"reconciliation"`
}

type PlanningConfig struct {
    Batch     int         `toml:"batch"      json:"batch"`
    StudyCode AgentConfig `toml:"study_code" json:"study_code"`
    Eval      EvalConfig  `toml:"eval"       json:"eval"`
    Refine    AgentConfig `toml:"refine"     json:"refine"`
}

type ImplementingConfig struct {
    Batch int        `toml:"batch" json:"batch"`
    Eval  EvalConfig `toml:"eval"  json:"eval"`
}

type PathsConfig struct {
    StateDir     string `toml:"state_dir"     json:"state_dir"`
    WorkspaceDir string `toml:"workspace_dir" json:"workspace_dir"`
}

type GeneralConfig struct {
    UserGuided     bool `toml:"user_guided"     json:"user_guided"`
    EnableCommits  bool `toml:"enable_commits"  json:"enable_commits"`
}

type LogsConfig struct {
    Enabled       bool `toml:"enabled"        json:"enabled"`
    RetentionDays int  `toml:"retention_days" json:"retention_days"`
    MaxFiles      int  `toml:"max_files"      json:"max_files"`
}
```

## Default Config Function

```go
func DefaultConfig() Config {
    return Config{
        Specifying: SpecifyingConfig{
            Batch: 3,
            Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3, AgentType: "opus", AgentCount: 1},
            CrossReference: CrossReferenceConfig{
                MinRounds: 1, MaxRounds: 2,
                AgentType: "haiku", AgentCount: 3,
                UserReview: false,
                Eval: AgentConfig{AgentType: "opus", AgentCount: 1},
            },
            Reconciliation: ReconciliationConfig{
                MinRounds: 0, MaxRounds: 3,
                AgentType: "opus", AgentCount: 1,
                UserReview: false,
            },
        },
        Planning: PlanningConfig{
            Batch:     1,
            StudyCode: AgentConfig{AgentType: "haiku", AgentCount: 3},
            Eval:      EvalConfig{MinRounds: 1, MaxRounds: 3, AgentType: "opus", AgentCount: 1},
            Refine:    AgentConfig{AgentType: "opus", AgentCount: 1},
        },
        Implementing: ImplementingConfig{
            Batch: 2,
            Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3, AgentType: "opus", AgentCount: 1},
        },
        Paths: PathsConfig{
            StateDir:     ".forgectl/state",
            WorkspaceDir: ".forge_workspace",
        },
        General: GeneralConfig{UserGuided: true, EnableCommits: false},
        Logs:    LogsConfig{Enabled: true, RetentionDays: 90, MaxFiles: 50},
    }
}
```

## Config Validation Rules

Per phase, enforce:
- `specifying.batch >= 1`
- `specifying.eval.min_rounds >= 1`
- `specifying.eval.min_rounds <= specifying.eval.max_rounds`
- `specifying.cross_reference.min_rounds <= specifying.cross_reference.max_rounds`
- `specifying.reconciliation.min_rounds <= specifying.reconciliation.max_rounds`
- `planning.batch >= 1`
- `planning.eval.min_rounds >= 1`
- `planning.eval.min_rounds <= planning.eval.max_rounds`
- `implementing.batch >= 1`
- `implementing.eval.min_rounds >= 1`
- `implementing.eval.min_rounds <= implementing.eval.max_rounds`
- `logs.retention_days >= 0`
- `logs.max_files >= 0`

## Project Root Discovery

```go
// FindProjectRoot walks up from cwd until .forgectl/ is found.
// Returns the directory containing .forgectl/ or an error.
func FindProjectRoot() (string, error) {
    dir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    for {
        if fi, err := os.Stat(filepath.Join(dir, ".forgectl")); err == nil && fi.IsDir() {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return "", fmt.Errorf("No .forgectl directory found.")
        }
        dir = parent
    }
}
```

## Config Parsing

```go
// LoadConfig reads .forgectl/config from projectRoot, applies defaults, and validates.
func LoadConfig(projectRoot string) (Config, error) {
    cfg := DefaultConfig()
    path := filepath.Join(projectRoot, ".forgectl", "config")
    _, err := toml.DecodeFile(path, &cfg)
    if err != nil {
        return cfg, fmt.Errorf("reading .forgectl/config: %w", err)
    }
    return cfg, nil
}
```

Note: `toml.DecodeFile` into a pre-populated struct naturally applies defaults — only explicitly set keys override.
