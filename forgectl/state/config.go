package state

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration structure for forgectl.
type Config struct {
	Specifying   SpecifyingConfig   `toml:"specifying"   json:"specifying"`
	Planning     PlanningConfig     `toml:"planning"     json:"planning"`
	Implementing ImplementingConfig `toml:"implementing" json:"implementing"`
	Paths        PathsConfig        `toml:"paths"        json:"paths"`
	General      GeneralConfig      `toml:"general"      json:"general"`
	Logs         LogsConfig         `toml:"logs"         json:"logs"`
}

// EvalConfig configures evaluation rounds and agent settings.
type EvalConfig struct {
	MinRounds  int    `toml:"min_rounds"  json:"min_rounds"`
	MaxRounds  int    `toml:"max_rounds"  json:"max_rounds"`
	AgentType  string `toml:"agent_type"  json:"agent_type"`
	AgentCount int    `toml:"agent_count" json:"agent_count"`
}

// AgentConfig configures agent type and count for a sub-step.
type AgentConfig struct {
	AgentType  string `toml:"agent_type"  json:"agent_type"`
	AgentCount int    `toml:"agent_count" json:"agent_count"`
}

// CrossReferenceConfig configures cross-reference settings for specifying.
type CrossReferenceConfig struct {
	MinRounds  int         `toml:"min_rounds"  json:"min_rounds"`
	MaxRounds  int         `toml:"max_rounds"  json:"max_rounds"`
	AgentType  string      `toml:"agent_type"  json:"agent_type"`
	AgentCount int         `toml:"agent_count" json:"agent_count"`
	UserReview bool        `toml:"user_review" json:"user_review"`
	Eval       AgentConfig `toml:"eval"        json:"eval"`
}

// ReconciliationConfig configures reconciliation settings for specifying.
type ReconciliationConfig struct {
	MinRounds  int    `toml:"min_rounds"  json:"min_rounds"`
	MaxRounds  int    `toml:"max_rounds"  json:"max_rounds"`
	AgentType  string `toml:"agent_type"  json:"agent_type"`
	AgentCount int    `toml:"agent_count" json:"agent_count"`
	UserReview bool   `toml:"user_review" json:"user_review"`
}

// SpecifyingConfig configures the specifying phase.
type SpecifyingConfig struct {
	Batch          int                  `toml:"batch"           json:"batch"`
	Eval           EvalConfig           `toml:"eval"            json:"eval"`
	CrossReference CrossReferenceConfig `toml:"cross_reference" json:"cross_reference"`
	Reconciliation ReconciliationConfig `toml:"reconciliation"  json:"reconciliation"`
}

// PlanningConfig configures the planning phase.
type PlanningConfig struct {
	Batch     int         `toml:"batch"      json:"batch"`
	StudyCode AgentConfig `toml:"study_code" json:"study_code"`
	Eval      EvalConfig  `toml:"eval"       json:"eval"`
	Refine    AgentConfig `toml:"refine"     json:"refine"`
}

// ImplementingConfig configures the implementing phase.
type ImplementingConfig struct {
	Batch int        `toml:"batch" json:"batch"`
	Eval  EvalConfig `toml:"eval"  json:"eval"`
}

// PathsConfig configures directory paths used by forgectl.
type PathsConfig struct {
	StateDir     string `toml:"state_dir"     json:"state_dir"`
	WorkspaceDir string `toml:"workspace_dir" json:"workspace_dir"`
}

// GeneralConfig holds general behavioral settings.
type GeneralConfig struct {
	UserGuided    bool `toml:"user_guided"    json:"user_guided"`
	EnableCommits bool `toml:"enable_commits" json:"enable_commits"`
}

// LogsConfig configures logging behavior.
type LogsConfig struct {
	Enabled       bool `toml:"enabled"        json:"enabled"`
	RetentionDays int  `toml:"retention_days" json:"retention_days"`
	MaxFiles      int  `toml:"max_files"      json:"max_files"`
}

// DefaultConfig returns a Config populated with all spec-defined defaults.
func DefaultConfig() Config {
	return Config{
		Specifying: SpecifyingConfig{
			Batch: 3,
			Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3, AgentType: "opus", AgentCount: 1},
			CrossReference: CrossReferenceConfig{
				MinRounds:  1,
				MaxRounds:  2,
				AgentType:  "haiku",
				AgentCount: 3,
				UserReview: false,
				Eval:       AgentConfig{AgentType: "opus", AgentCount: 1},
			},
			Reconciliation: ReconciliationConfig{
				MinRounds:  0,
				MaxRounds:  3,
				AgentType:  "opus",
				AgentCount: 1,
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

// LoadConfig reads the TOML config at <projectRoot>/.forgectl/config, merging
// values on top of DefaultConfig. If the file does not exist, the defaults are
// returned without error.
func LoadConfig(projectRoot string) (Config, error) {
	cfg := DefaultConfig()
	path := filepath.Join(projectRoot, ".forgectl", "config")
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading .forgectl/config: %w", err)
	}
	return cfg, nil
}

// ValidateConfig checks cfg against all spec-defined constraints and returns a
// slice of human-readable violation strings. An empty slice means the config is
// valid.
func ValidateConfig(cfg Config) []string {
	var violations []string

	// specifying
	if cfg.Specifying.Batch < 1 {
		violations = append(violations, "specifying.batch must be >= 1")
	}
	if cfg.Specifying.Eval.MinRounds < 1 {
		violations = append(violations, "specifying.eval.min_rounds must be >= 1")
	}
	if cfg.Specifying.Eval.MinRounds > cfg.Specifying.Eval.MaxRounds {
		violations = append(violations, "specifying.eval.min_rounds must be <= specifying.eval.max_rounds")
	}
	if cfg.Specifying.CrossReference.MinRounds > cfg.Specifying.CrossReference.MaxRounds {
		violations = append(violations, "specifying.cross_reference.min_rounds must be <= specifying.cross_reference.max_rounds")
	}
	if cfg.Specifying.Reconciliation.MinRounds > cfg.Specifying.Reconciliation.MaxRounds {
		violations = append(violations, "specifying.reconciliation.min_rounds must be <= specifying.reconciliation.max_rounds")
	}

	// planning
	if cfg.Planning.Batch < 1 {
		violations = append(violations, "planning.batch must be >= 1")
	}
	if cfg.Planning.Eval.MinRounds < 1 {
		violations = append(violations, "planning.eval.min_rounds must be >= 1")
	}
	if cfg.Planning.Eval.MinRounds > cfg.Planning.Eval.MaxRounds {
		violations = append(violations, "planning.eval.min_rounds must be <= planning.eval.max_rounds")
	}

	// implementing
	if cfg.Implementing.Batch < 1 {
		violations = append(violations, "implementing.batch must be >= 1")
	}
	if cfg.Implementing.Eval.MinRounds < 1 {
		violations = append(violations, "implementing.eval.min_rounds must be >= 1")
	}
	if cfg.Implementing.Eval.MinRounds > cfg.Implementing.Eval.MaxRounds {
		violations = append(violations, "implementing.eval.min_rounds must be <= implementing.eval.max_rounds")
	}

	// logs
	if cfg.Logs.RetentionDays < 0 {
		violations = append(violations, "logs.retention_days must be >= 0")
	}
	if cfg.Logs.MaxFiles < 0 {
		violations = append(violations, "logs.max_files must be >= 0")
	}

	return violations
}
