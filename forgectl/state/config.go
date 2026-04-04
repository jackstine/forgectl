package state

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// tomlAgentConfig mirrors AgentConfig for TOML decoding.
type tomlAgentConfig struct {
	Model string `toml:"model"`
	Type  string `toml:"type"`
	Count int    `toml:"count"`
}

// tomlEvalConfig mirrors EvalConfig for TOML decoding.
type tomlEvalConfig struct {
	MinRounds        int             `toml:"min_rounds"`
	MaxRounds        int             `toml:"max_rounds"`
	Model            string          `toml:"model"`
	Type             string          `toml:"type"`
	Count            int             `toml:"count"`
	EnableEvalOutput bool            `toml:"enable_eval_output"`
	Eval             tomlAgentConfig `toml:"eval"`
}

// tomlCrossRefConfig mirrors CrossRefConfig for TOML decoding.
type tomlCrossRefConfig struct {
	MinRounds  int             `toml:"min_rounds"`
	MaxRounds  int             `toml:"max_rounds"`
	Model      string          `toml:"model"`
	Type       string          `toml:"type"`
	Count      int             `toml:"count"`
	UserReview bool            `toml:"user_review"`
	Eval       tomlAgentConfig `toml:"eval"`
}

// tomlReconciliationConfig mirrors ReconciliationConfig for TOML decoding.
type tomlReconciliationConfig struct {
	MinRounds  int    `toml:"min_rounds"`
	MaxRounds  int    `toml:"max_rounds"`
	Model      string `toml:"model"`
	Type       string `toml:"type"`
	Count      int    `toml:"count"`
	UserReview bool   `toml:"user_review"`
}

// tomlSpecifyingConfig mirrors SpecifyingConfig for TOML decoding.
type tomlSpecifyingConfig struct {
	Batch          int                      `toml:"batch"`
	CommitStrategy string                   `toml:"commit_strategy"`
	Eval           tomlEvalConfig           `toml:"eval"`
	CrossReference tomlCrossRefConfig       `toml:"cross_reference"`
	Reconciliation tomlReconciliationConfig `toml:"reconciliation"`
}

// tomlStudyCodeConfig mirrors StudyCodeConfig for TOML decoding.
type tomlStudyCodeConfig struct {
	Model string `toml:"model"`
	Type  string `toml:"type"`
	Count int    `toml:"count"`
}

// tomlRefineConfig mirrors RefineConfig for TOML decoding.
type tomlRefineConfig struct {
	Model string `toml:"model"`
	Type  string `toml:"type"`
	Count int    `toml:"count"`
}

// tomlPlanningConfig mirrors PlanningConfig for TOML decoding.
type tomlPlanningConfig struct {
	Batch                     int                `toml:"batch"`
	CommitStrategy            string             `toml:"commit_strategy"`
	SelfReview                bool               `toml:"self_review"`
	PlanAllBeforeImplementing bool               `toml:"plan_all_before_implementing"`
	StudyCode                 tomlStudyCodeConfig `toml:"study_code"`
	Eval                      tomlEvalConfig      `toml:"eval"`
	Refine                    tomlRefineConfig    `toml:"refine"`
}

// tomlImplementingConfig mirrors ImplementingConfig for TOML decoding.
type tomlImplementingConfig struct {
	Batch          int            `toml:"batch"`
	CommitStrategy string         `toml:"commit_strategy"`
	Eval           tomlEvalConfig `toml:"eval"`
}

// tomlDomainConfig mirrors DomainConfig for TOML decoding.
type tomlDomainConfig struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// tomlPathsConfig mirrors PathsConfig for TOML decoding.
type tomlPathsConfig struct {
	StateDir     string `toml:"state_dir"`
	WorkspaceDir string `toml:"workspace_dir"`
}

// tomlLogsConfig mirrors LogsConfig for TOML decoding.
type tomlLogsConfig struct {
	Enabled       bool `toml:"enabled"`
	RetentionDays int  `toml:"retention_days"`
	MaxFiles      int  `toml:"max_files"`
}

// tomlGeneralConfig mirrors GeneralConfig for TOML decoding.
type tomlGeneralConfig struct {
	EnableCommits bool `toml:"enable_commits"`
	UserGuided    bool `toml:"user_guided"`
}

// tomlForgeConfig is the intermediate struct for TOML decoding of .forgectl/config.
type tomlForgeConfig struct {
	General      tomlGeneralConfig      `toml:"general"`
	Domains      []tomlDomainConfig     `toml:"domains"`
	Specifying   tomlSpecifyingConfig   `toml:"specifying"`
	Planning     tomlPlanningConfig     `toml:"planning"`
	Implementing tomlImplementingConfig `toml:"implementing"`
	Paths        tomlPathsConfig        `toml:"paths"`
	Logs         tomlLogsConfig         `toml:"logs"`
}

// FindProjectRoot walks up from startDir until it finds a directory containing .forgectl/.
// Returns an error if not found.
func FindProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".forgectl")
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding .forgectl/.
			return "", fmt.Errorf("No .forgectl directory found.")
		}
		dir = parent
	}
}

// LoadConfig reads .forgectl/config from projectRoot, applies defaults for missing fields,
// and returns the merged ForgeConfig.
func LoadConfig(projectRoot string) (ForgeConfig, error) {
	cfg := DefaultForgeConfig()
	configPath := filepath.Join(projectRoot, ".forgectl", "config")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file is optional; return defaults.
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	var raw tomlForgeConfig
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	mergeTomlConfig(&cfg, &raw)
	return cfg, nil
}

// mergeTomlConfig applies non-zero TOML values onto the default ForgeConfig.
// Zero values in the TOML struct are ignored so that defaults are preserved.
func mergeTomlConfig(cfg *ForgeConfig, raw *tomlForgeConfig) {
	// General
	cfg.General.EnableCommits = raw.General.EnableCommits
	cfg.General.UserGuided = raw.General.UserGuided

	// Domains (replace entirely if provided)
	if len(raw.Domains) > 0 {
		cfg.Domains = make([]DomainConfig, len(raw.Domains))
		for i, d := range raw.Domains {
			cfg.Domains[i] = DomainConfig{Name: d.Name, Path: d.Path}
		}
	}

	// Specifying
	if raw.Specifying.Batch > 0 {
		cfg.Specifying.Batch = raw.Specifying.Batch
	}
	if raw.Specifying.CommitStrategy != "" {
		cfg.Specifying.CommitStrategy = raw.Specifying.CommitStrategy
	}
	mergeEvalConfig(&cfg.Specifying.Eval, &raw.Specifying.Eval)
	mergeCrossRefConfig(&cfg.Specifying.CrossReference, &raw.Specifying.CrossReference)
	mergeReconciliationConfig(&cfg.Specifying.Reconciliation, &raw.Specifying.Reconciliation)

	// Planning
	if raw.Planning.Batch > 0 {
		cfg.Planning.Batch = raw.Planning.Batch
	}
	if raw.Planning.CommitStrategy != "" {
		cfg.Planning.CommitStrategy = raw.Planning.CommitStrategy
	}
	cfg.Planning.SelfReview = raw.Planning.SelfReview
	cfg.Planning.PlanAllBeforeImplementing = raw.Planning.PlanAllBeforeImplementing
	mergeEvalConfig(&cfg.Planning.Eval, &raw.Planning.Eval)
	if raw.Planning.StudyCode.Model != "" {
		cfg.Planning.StudyCode.AgentConfig.Model = raw.Planning.StudyCode.Model
	}
	if raw.Planning.StudyCode.Type != "" {
		cfg.Planning.StudyCode.AgentConfig.Type = raw.Planning.StudyCode.Type
	}
	if raw.Planning.StudyCode.Count > 0 {
		cfg.Planning.StudyCode.AgentConfig.Count = raw.Planning.StudyCode.Count
	}
	if raw.Planning.Refine.Model != "" {
		cfg.Planning.Refine.AgentConfig.Model = raw.Planning.Refine.Model
	}
	if raw.Planning.Refine.Type != "" {
		cfg.Planning.Refine.AgentConfig.Type = raw.Planning.Refine.Type
	}
	if raw.Planning.Refine.Count > 0 {
		cfg.Planning.Refine.AgentConfig.Count = raw.Planning.Refine.Count
	}

	// Implementing
	if raw.Implementing.Batch > 0 {
		cfg.Implementing.Batch = raw.Implementing.Batch
	}
	if raw.Implementing.CommitStrategy != "" {
		cfg.Implementing.CommitStrategy = raw.Implementing.CommitStrategy
	}
	mergeEvalConfig(&cfg.Implementing.Eval, &raw.Implementing.Eval)

	// Paths
	if raw.Paths.StateDir != "" {
		cfg.Paths.StateDir = raw.Paths.StateDir
	}
	if raw.Paths.WorkspaceDir != "" {
		cfg.Paths.WorkspaceDir = raw.Paths.WorkspaceDir
	}

	// Logs
	cfg.Logs.Enabled = raw.Logs.Enabled
	if raw.Logs.RetentionDays > 0 {
		cfg.Logs.RetentionDays = raw.Logs.RetentionDays
	}
	if raw.Logs.MaxFiles > 0 {
		cfg.Logs.MaxFiles = raw.Logs.MaxFiles
	}
}

func mergeEvalConfig(dst *EvalConfig, src *tomlEvalConfig) {
	if src.MinRounds > 0 {
		dst.MinRounds = src.MinRounds
	}
	if src.MaxRounds > 0 {
		dst.MaxRounds = src.MaxRounds
	}
	if src.Model != "" {
		dst.AgentConfig.Model = src.Model
	}
	if src.Type != "" {
		dst.AgentConfig.Type = src.Type
	}
	if src.Count > 0 {
		dst.AgentConfig.Count = src.Count
	}
	dst.EnableEvalOutput = src.EnableEvalOutput
}

func mergeCrossRefConfig(dst *CrossRefConfig, src *tomlCrossRefConfig) {
	if src.MinRounds > 0 {
		dst.MinRounds = src.MinRounds
	}
	if src.MaxRounds > 0 {
		dst.MaxRounds = src.MaxRounds
	}
	if src.Model != "" {
		dst.AgentConfig.Model = src.Model
	}
	if src.Type != "" {
		dst.AgentConfig.Type = src.Type
	}
	if src.Count > 0 {
		dst.AgentConfig.Count = src.Count
	}
	dst.UserReview = src.UserReview
	if src.Eval.Model != "" {
		dst.Eval.Model = src.Eval.Model
	}
	if src.Eval.Type != "" {
		dst.Eval.Type = src.Eval.Type
	}
	if src.Eval.Count > 0 {
		dst.Eval.Count = src.Eval.Count
	}
}

func mergeReconciliationConfig(dst *ReconciliationConfig, src *tomlReconciliationConfig) {
	if src.MinRounds > 0 {
		dst.MinRounds = src.MinRounds
	}
	if src.MaxRounds > 0 {
		dst.MaxRounds = src.MaxRounds
	}
	if src.Model != "" {
		dst.AgentConfig.Model = src.Model
	}
	if src.Type != "" {
		dst.AgentConfig.Type = src.Type
	}
	if src.Count > 0 {
		dst.AgentConfig.Count = src.Count
	}
	dst.UserReview = src.UserReview
}

// GenerateSessionID returns a new UUID v4 string using crypto/rand.
func GenerateSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback: use zero bytes (should never happen in practice)
		return "00000000-0000-4000-8000-000000000000"
	}
	// Set version 4 bits (bits 12-15 of byte 6)
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant bits (bits 6-7 of byte 8) to 10xx
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ValidateConfig returns a list of constraint violations in cfg.
func ValidateConfig(cfg ForgeConfig) []string {
	var errs []string

	validStrategies := map[string]bool{
		"strict":   true,
		"all-specs": true,
		"scoped":   true,
		"tracked":  true,
		"all":      true,
	}

	if cfg.Specifying.CommitStrategy != "" && !validStrategies[cfg.Specifying.CommitStrategy] {
		errs = append(errs, fmt.Sprintf("specifying.commit_strategy: invalid value %q", cfg.Specifying.CommitStrategy))
	}
	if cfg.Planning.CommitStrategy != "" && !validStrategies[cfg.Planning.CommitStrategy] {
		errs = append(errs, fmt.Sprintf("planning.commit_strategy: invalid value %q", cfg.Planning.CommitStrategy))
	}
	if cfg.Implementing.CommitStrategy != "" && !validStrategies[cfg.Implementing.CommitStrategy] {
		errs = append(errs, fmt.Sprintf("implementing.commit_strategy: invalid value %q", cfg.Implementing.CommitStrategy))
	}

	if cfg.Specifying.Eval.MinRounds > cfg.Specifying.Eval.MaxRounds {
		errs = append(errs, "specifying.eval.min_rounds cannot exceed max_rounds")
	}
	if cfg.Planning.Eval.MinRounds > cfg.Planning.Eval.MaxRounds {
		errs = append(errs, "planning.eval.min_rounds cannot exceed max_rounds")
	}
	if cfg.Implementing.Eval.MinRounds > cfg.Implementing.Eval.MaxRounds {
		errs = append(errs, "implementing.eval.min_rounds cannot exceed max_rounds")
	}

	// No domain path is a prefix of another domain path.
	for i, d1 := range cfg.Domains {
		for j, d2 := range cfg.Domains {
			if i == j {
				continue
			}
			p1 := filepath.Clean(d1.Path) + string(filepath.Separator)
			p2 := filepath.Clean(d2.Path) + string(filepath.Separator)
			if len(p1) <= len(p2) && p2[:len(p1)] == p1 {
				errs = append(errs, fmt.Sprintf("domain %q path is a prefix of domain %q path (nested paths not allowed)", d1.Name, d2.Name))
			}
		}
	}

	return errs
}
