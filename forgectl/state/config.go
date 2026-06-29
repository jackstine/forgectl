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
	EnableEvalOutput *bool           `toml:"enable_eval_output"`
	EvalMode         *string         `toml:"eval_mode"` // pointer so 'unset' is distinguishable
	Eval             tomlAgentConfig `toml:"eval"`
}

// tomlCrossRefConfig mirrors CrossRefConfig for TOML decoding.
type tomlCrossRefConfig struct {
	MinRounds  int             `toml:"min_rounds"`
	MaxRounds  int             `toml:"max_rounds"`
	Model      string          `toml:"model"`
	Type       string          `toml:"type"`
	Count      int             `toml:"count"`
	UserReview *bool           `toml:"user_review"`
	Eval       tomlAgentConfig `toml:"eval"`
}

// tomlReconciliationConfig mirrors ReconciliationConfig for TOML decoding.
type tomlReconciliationConfig struct {
	MinRounds  int    `toml:"min_rounds"`
	MaxRounds  int    `toml:"max_rounds"`
	Model      string `toml:"model"`
	Type       string `toml:"type"`
	Count      int    `toml:"count"`
	UserReview *bool  `toml:"user_review"`
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
	Batch                     int                 `toml:"batch"`
	CommitStrategy            string              `toml:"commit_strategy"`
	SelfReview                *bool               `toml:"self_review"`
	PlanAllBeforeImplementing *bool               `toml:"plan_all_before_implementing"`
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

// tomlUIAppConfig mirrors UIAppConfig for TOML decoding.
type tomlUIAppConfig struct {
	LaunchCommand       string `toml:"launch_command"`
	URL                 string `toml:"url"`
	ReadyTimeoutSeconds int    `toml:"ready_timeout_seconds"`
}

// tomlUIE2EConfig mirrors UIE2EConfig for TOML decoding. tomlEvalConfig is
// embedded so the loop's round fields sit at the same [ui_implementing.e2e]
// level as test_command/test_dir.
type tomlUIE2EConfig struct {
	tomlEvalConfig        // embedded: min/max rounds, model, type, count, eval_mode
	TestCommand    string `toml:"test_command"`
	TestDir        string `toml:"test_dir"`
}

// tomlUIImplementingConfig mirrors UIImplementingConfig for TOML decoding.
type tomlUIImplementingConfig struct {
	Batch          int             `toml:"batch"`
	CommitStrategy string          `toml:"commit_strategy"`
	App            tomlUIAppConfig `toml:"app"`
	Eval           tomlEvalConfig  `toml:"eval"`
	QA             tomlEvalConfig  `toml:"qa"`
	E2E            tomlUIE2EConfig `toml:"e2e"`
}

// tomlREReconcileConfig mirrors REReconcileConfig for TOML decoding.
type tomlREReconcileConfig struct {
	MinRounds       int             `toml:"min_rounds"`
	MaxRounds       int             `toml:"max_rounds"`
	ColleagueReview *bool           `toml:"colleague_review"` // pointer so an explicit false overrides the default
	Eval            tomlAgentConfig `toml:"eval"`
}

// tomlReverseEngineeringConfig mirrors ReverseEngineeringConfig for TOML decoding.
type tomlReverseEngineeringConfig struct {
	Execute     tomlAgentConfig       `toml:"execute"`
	Survey      tomlAgentConfig       `toml:"survey"`
	GapAnalysis tomlAgentConfig       `toml:"gap_analysis"`
	Reconcile   tomlREReconcileConfig `toml:"reconcile"`
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
	Enabled       *bool `toml:"enabled"`
	RetentionDays int   `toml:"retention_days"`
	MaxFiles      int   `toml:"max_files"`
}

// tomlGeneralConfig mirrors GeneralConfig for TOML decoding.
type tomlGeneralConfig struct {
	EnableCommits *bool `toml:"enable_commits"`
	UserGuided    *bool `toml:"user_guided"`
}

// tomlForgeConfig is the intermediate struct for TOML decoding of .forgectl/config.
type tomlForgeConfig struct {
	General        tomlGeneralConfig        `toml:"general"`
	Domains        []tomlDomainConfig       `toml:"domains"`
	Specifying     tomlSpecifyingConfig     `toml:"specifying"`
	Planning       tomlPlanningConfig       `toml:"planning"`
	Implementing   tomlImplementingConfig   `toml:"implementing"`
	UIImplementing tomlUIImplementingConfig `toml:"ui_implementing"`

	ReverseEngineering tomlReverseEngineeringConfig `toml:"reverse_engineering"`

	Paths tomlPathsConfig `toml:"paths"`
	Logs  tomlLogsConfig  `toml:"logs"`
}

// FindProjectRoot walks up from startDir looking for a directory containing
// .forgectl/. The walk stops at the first of three conditions: a .forgectl/
// directory is found, a .git entry is found (directory or worktree file,
// marking the git-root boundary), or the filesystem root is reached. A
// .forgectl/ that lives above the git root is never discovered.
func FindProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		// .forgectl/ takes priority — check it before .git so a directory that
		// contains both is still recognized as the project root.
		candidate := filepath.Join(dir, ".forgectl")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return dir, nil
		}
		// Stop at the git root: any .git entry (directory or worktree file)
		// marks the repository boundary. A .forgectl/ above this point must not
		// be used. Git worktrees have a .git file rather than a .git directory,
		// so we check existence only, not IsDir().
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return "", fmt.Errorf("No .forgectl directory found.")
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
			return cfg, fmt.Errorf(".forgectl/config not found at %s", configPath)
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
	if raw.General.EnableCommits != nil {
		cfg.General.EnableCommits = *raw.General.EnableCommits
	}
	if raw.General.UserGuided != nil {
		cfg.General.UserGuided = *raw.General.UserGuided
	}

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
	if raw.Planning.SelfReview != nil {
		cfg.Planning.SelfReview = *raw.Planning.SelfReview
	}
	if raw.Planning.PlanAllBeforeImplementing != nil {
		cfg.Planning.PlanAllBeforeImplementing = *raw.Planning.PlanAllBeforeImplementing
	}
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

	// UI Implementing
	if raw.UIImplementing.Batch > 0 {
		cfg.UIImplementing.Batch = raw.UIImplementing.Batch
	}
	if raw.UIImplementing.CommitStrategy != "" {
		cfg.UIImplementing.CommitStrategy = raw.UIImplementing.CommitStrategy
	}
	if raw.UIImplementing.App.LaunchCommand != "" {
		cfg.UIImplementing.App.LaunchCommand = raw.UIImplementing.App.LaunchCommand
	}
	if raw.UIImplementing.App.URL != "" {
		cfg.UIImplementing.App.URL = raw.UIImplementing.App.URL
	}
	if raw.UIImplementing.App.ReadyTimeoutSeconds > 0 {
		cfg.UIImplementing.App.ReadyTimeoutSeconds = raw.UIImplementing.App.ReadyTimeoutSeconds
	}
	mergeEvalConfig(&cfg.UIImplementing.Eval, &raw.UIImplementing.Eval)
	mergeEvalConfig(&cfg.UIImplementing.QA, &raw.UIImplementing.QA)
	mergeEvalConfig(&cfg.UIImplementing.E2E.EvalConfig, &raw.UIImplementing.E2E.tomlEvalConfig)
	if raw.UIImplementing.E2E.TestCommand != "" {
		cfg.UIImplementing.E2E.TestCommand = raw.UIImplementing.E2E.TestCommand
	}
	if raw.UIImplementing.E2E.TestDir != "" {
		cfg.UIImplementing.E2E.TestDir = raw.UIImplementing.E2E.TestDir
	}

	// Reverse engineering
	mergeReverseEngineeringConfig(&cfg.ReverseEngineering, &raw.ReverseEngineering)

	// Paths
	if raw.Paths.StateDir != "" {
		cfg.Paths.StateDir = raw.Paths.StateDir
	}
	if raw.Paths.WorkspaceDir != "" {
		cfg.Paths.WorkspaceDir = raw.Paths.WorkspaceDir
	}

	// Logs
	if raw.Logs.Enabled != nil {
		cfg.Logs.Enabled = *raw.Logs.Enabled
	}
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
	if src.EnableEvalOutput != nil {
		dst.EnableEvalOutput = *src.EnableEvalOutput
	}
	if src.EvalMode != nil {
		dst.EvalMode = *src.EvalMode
	}
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
	if src.UserReview != nil {
		dst.UserReview = *src.UserReview
	}
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

// mergeAgentConfig copies non-zero model/type/count from a TOML agent config.
func mergeAgentConfig(dst *AgentConfig, src *tomlAgentConfig) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Type != "" {
		dst.Type = src.Type
	}
	if src.Count > 0 {
		dst.Count = src.Count
	}
}

func mergeReverseEngineeringConfig(dst *ReverseEngineeringConfig, src *tomlReverseEngineeringConfig) {
	mergeAgentConfig(&dst.Execute, &src.Execute)
	mergeAgentConfig(&dst.Survey, &src.Survey)
	mergeAgentConfig(&dst.GapAnalysis, &src.GapAnalysis)

	if src.Reconcile.MinRounds > 0 {
		dst.Reconcile.MinRounds = src.Reconcile.MinRounds
	}
	if src.Reconcile.MaxRounds > 0 {
		dst.Reconcile.MaxRounds = src.Reconcile.MaxRounds
	}
	if src.Reconcile.ColleagueReview != nil {
		dst.Reconcile.ColleagueReview = *src.Reconcile.ColleagueReview
	}
	mergeAgentConfig(&dst.Reconcile.Eval, &src.Reconcile.Eval)
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
	if src.UserReview != nil {
		dst.UserReview = *src.UserReview
	}
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

// EvalModeFor returns the effective eval mode for the given eval block, honoring
// back-compat with the legacy enable_eval_output boolean. An explicit eval_mode
// always wins. For new sessions the default is "report" (seeded by
// DefaultForgeConfig). For legacy sessions locked before eval_mode existed, an
// empty eval_mode resolves to "report" when enable_eval_output was on, else
// "conversational" — preserving the prior no-report behavior.
func EvalModeFor(ec EvalConfig, gen GeneralConfig) string {
	if ec.EvalMode != "" {
		return ec.EvalMode
	}
	if ec.EnableEvalOutput || gen.EnableEvalOutput {
		return "report"
	}
	return "conversational"
}

// ValidateConfig returns a list of constraint violations in cfg.
func ValidateConfig(cfg ForgeConfig) []string {
	var errs []string

	validStrategies := map[string]bool{
		"strict":    true,
		"all-specs": true,
		"scoped":    true,
		"tracked":   true,
		"all":       true,
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
	if cfg.UIImplementing.CommitStrategy != "" && !validStrategies[cfg.UIImplementing.CommitStrategy] {
		errs = append(errs, fmt.Sprintf("ui_implementing.commit_strategy: invalid value %q", cfg.UIImplementing.CommitStrategy))
	}

	validEvalModes := map[string]bool{
		"report":         true,
		"direct":         true,
		"conversational": true,
	}

	// An empty eval_mode is valid: the resolution helper supplies the back-compat default.
	if cfg.Specifying.Eval.EvalMode != "" && !validEvalModes[cfg.Specifying.Eval.EvalMode] {
		errs = append(errs, fmt.Sprintf("specifying.eval.eval_mode: invalid value %q", cfg.Specifying.Eval.EvalMode))
	}
	if cfg.Planning.Eval.EvalMode != "" && !validEvalModes[cfg.Planning.Eval.EvalMode] {
		errs = append(errs, fmt.Sprintf("planning.eval.eval_mode: invalid value %q", cfg.Planning.Eval.EvalMode))
	}
	if cfg.Implementing.Eval.EvalMode != "" && !validEvalModes[cfg.Implementing.Eval.EvalMode] {
		errs = append(errs, fmt.Sprintf("implementing.eval.eval_mode: invalid value %q", cfg.Implementing.Eval.EvalMode))
	}
	// ui_implementing carries three independent eval loops, each with its own eval_mode.
	uiLoops := map[string]EvalConfig{
		"eval": cfg.UIImplementing.Eval,
		"qa":   cfg.UIImplementing.QA,
		"e2e":  cfg.UIImplementing.E2E.EvalConfig,
	}
	for _, name := range []string{"eval", "qa", "e2e"} {
		if mode := uiLoops[name].EvalMode; mode != "" && !validEvalModes[mode] {
			errs = append(errs, fmt.Sprintf("ui_implementing.%s.eval_mode: invalid value %q", name, mode))
		}
	}

	if cfg.Specifying.Batch < 1 {
		errs = append(errs, "specifying.batch must be >= 1")
	}
	if cfg.Planning.Batch < 1 {
		errs = append(errs, "planning.batch must be >= 1")
	}
	if cfg.Implementing.Batch < 1 {
		errs = append(errs, "implementing.batch must be >= 1")
	}
	if cfg.UIImplementing.Batch < 1 {
		errs = append(errs, "ui_implementing.batch must be >= 1")
	}
	if cfg.Logs.RetentionDays < 0 {
		errs = append(errs, "logs.retention_days must be >= 0")
	}
	if cfg.Logs.MaxFiles < 0 {
		errs = append(errs, "logs.max_files must be >= 0")
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
	for _, name := range []string{"eval", "qa", "e2e"} {
		if uiLoops[name].MinRounds > uiLoops[name].MaxRounds {
			errs = append(errs, fmt.Sprintf("ui_implementing.%s.min_rounds cannot exceed max_rounds", name))
		}
	}
	if cfg.ReverseEngineering.Reconcile.MinRounds > cfg.ReverseEngineering.Reconcile.MaxRounds {
		errs = append(errs, "reverse_engineering.reconcile.min_rounds cannot exceed max_rounds")
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
				errs = append(errs, fmt.Sprintf("Domain paths must not be nested: %s is a prefix of %s.", d1.Path, d2.Path))
			}
		}
	}

	return errs
}
