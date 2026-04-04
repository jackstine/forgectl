package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindProjectRoot verifies root discovery by walking up the directory tree.
func TestFindProjectRoot(t *testing.T) {
	// Create a temp tree: root/.forgectl/  root/sub/deep/
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	// Starting from a sub-directory should find the root.
	got, err := FindProjectRoot(deep)
	if err != nil {
		t.Fatalf("FindProjectRoot from subdir: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}

	// Starting from the root itself should also work.
	got, err = FindProjectRoot(dir)
	if err != nil {
		t.Fatalf("FindProjectRoot from root: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// TestFindProjectRootNotFound verifies an error when .forgectl is absent.
func TestFindProjectRootNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindProjectRoot(dir)
	if err == nil {
		t.Fatal("expected error when .forgectl not found, got nil")
	}
	if err.Error() != "No .forgectl directory found." {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLoadConfigMissing verifies an error is returned when config file is missing.
func TestLoadConfigMissing(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error when .forgectl/config is missing, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// TestLoadConfigEmptyFile verifies defaults are returned when config file is empty.
func TestLoadConfigEmptyFile(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig with empty config: %v", err)
	}

	def := DefaultForgeConfig()
	if cfg.Specifying.Batch != def.Specifying.Batch {
		t.Errorf("specifying.batch: got %d, want %d", cfg.Specifying.Batch, def.Specifying.Batch)
	}
	if cfg.Paths.StateDir != def.Paths.StateDir {
		t.Errorf("paths.state_dir: got %q, want %q", cfg.Paths.StateDir, def.Paths.StateDir)
	}
}

// TestLoadConfigToml verifies TOML values override defaults.
func TestLoadConfigToml(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `
[general]
enable_commits = true
user_guided    = true

[specifying]
batch           = 5
commit_strategy = "scoped"

[specifying.eval]
min_rounds        = 2
max_rounds        = 4
model             = "haiku"
type              = "eval"
count             = 2
enable_eval_output = true

[implementing]
batch = 3

[paths]
state_dir     = ".custom/state"
workspace_dir = ".custom_workspace"

[logs]
enabled        = false
retention_days = 30
max_files      = 10
`
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if !cfg.General.EnableCommits {
		t.Error("general.enable_commits: want true")
	}
	if cfg.Specifying.Batch != 5 {
		t.Errorf("specifying.batch: got %d, want 5", cfg.Specifying.Batch)
	}
	if cfg.Specifying.CommitStrategy != "scoped" {
		t.Errorf("specifying.commit_strategy: got %q, want %q", cfg.Specifying.CommitStrategy, "scoped")
	}
	if cfg.Specifying.Eval.MinRounds != 2 {
		t.Errorf("specifying.eval.min_rounds: got %d, want 2", cfg.Specifying.Eval.MinRounds)
	}
	if cfg.Specifying.Eval.Model != "haiku" {
		t.Errorf("specifying.eval.model: got %q, want %q", cfg.Specifying.Eval.Model, "haiku")
	}
	if !cfg.Specifying.Eval.EnableEvalOutput {
		t.Error("specifying.eval.enable_eval_output: want true")
	}
	if cfg.Implementing.Batch != 3 {
		t.Errorf("implementing.batch: got %d, want 3", cfg.Implementing.Batch)
	}
	if cfg.Paths.StateDir != ".custom/state" {
		t.Errorf("paths.state_dir: got %q, want %q", cfg.Paths.StateDir, ".custom/state")
	}
	if cfg.Logs.Enabled {
		t.Error("logs.enabled: want false")
	}
	if cfg.Logs.RetentionDays != 30 {
		t.Errorf("logs.retention_days: got %d, want 30", cfg.Logs.RetentionDays)
	}
}

// TestLoadConfigInvalidToml verifies an error is returned for malformed TOML.
func TestLoadConfigInvalidToml(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte("[[not valid toml..."), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

// TestGenerateSessionID verifies UUID v4 format and uniqueness.
func TestGenerateSessionID(t *testing.T) {
	id := GenerateSessionID()
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("session ID %q: expected 5 parts separated by '-', got %d", id, len(parts))
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Errorf("session ID %q: unexpected segment lengths", id)
	}
	// Version 4: third segment starts with '4'
	if parts[2][0] != '4' {
		t.Errorf("session ID %q: version nibble must be '4', got %c", id, parts[2][0])
	}
	// Uniqueness: two calls must differ
	id2 := GenerateSessionID()
	if id == id2 {
		t.Error("two GenerateSessionID calls returned the same value")
	}
}

// TestValidateConfigValid verifies no errors for a valid config.
func TestValidateConfigValid(t *testing.T) {
	cfg := DefaultForgeConfig()
	errs := ValidateConfig(cfg)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

// TestValidateConfigBadStrategy verifies commit strategy validation.
func TestValidateConfigBadStrategy(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.Implementing.CommitStrategy = "unknown"
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected error for invalid commit_strategy")
	}
}

// TestValidateConfigMinExceedsMax verifies min_rounds <= max_rounds constraint.
func TestValidateConfigMinExceedsMax(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.Planning.Eval.MinRounds = 5
	cfg.Planning.Eval.MaxRounds = 3
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected error when planning.eval.min_rounds > max_rounds")
	}
}

// TestValidateConfigNestedDomains verifies nested domain paths are rejected.
func TestValidateConfigNestedDomains(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.Domains = []DomainConfig{
		{Name: "parent", Path: "apps"},
		{Name: "child", Path: "apps/sub"},
	}
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected error for nested domain paths")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "Domain paths must not be nested:") && strings.Contains(e, "apps") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected spec-format nesting error, got: %v", errs)
	}
}

// TestValidateConfigBatchBelowOne verifies batch < 1 is rejected.
func TestValidateConfigBatchBelowOne(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.Specifying.Batch = 0
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected violation for specifying.batch=0")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "specifying.batch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected specifying.batch violation, got: %v", errs)
	}
}

// TestValidateConfigLogsRetentionDaysNegative verifies negative retention days are rejected.
func TestValidateConfigLogsRetentionDaysNegative(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.Logs.RetentionDays = -1
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected violation for logs.retention_days=-1")
	}
}

// TestDefaultForgeConfigReverseEngineeringDefaults verifies reverse_engineering defaults are valid.
func TestDefaultForgeConfigReverseEngineeringDefaults(t *testing.T) {
	cfg := DefaultForgeConfig()
	re := cfg.ReverseEngineering

	if re.Mode != "self_refine" {
		t.Errorf("reverse_engineering.mode: got %q, want %q", re.Mode, "self_refine")
	}
	if re.Drafter.Model == "" {
		t.Error("reverse_engineering.drafter.model must not be empty")
	}
	if re.Drafter.Subagents.Count < 1 {
		t.Errorf("reverse_engineering.drafter.subagents.count: got %d, must be >= 1", re.Drafter.Subagents.Count)
	}
	if re.Survey.Count < 1 {
		t.Errorf("reverse_engineering.survey.count: got %d, must be >= 1", re.Survey.Count)
	}
	if re.GapAnalysis.Count < 1 {
		t.Errorf("reverse_engineering.gap_analysis.count: got %d, must be >= 1", re.GapAnalysis.Count)
	}
	if re.Reconcile.MinRounds > re.Reconcile.MaxRounds {
		t.Errorf("reverse_engineering.reconcile: min_rounds (%d) > max_rounds (%d)", re.Reconcile.MinRounds, re.Reconcile.MaxRounds)
	}
	if re.SelfRefine == nil {
		t.Error("default mode is self_refine, SelfRefine config must be non-nil")
	}

	// No errors from ValidateConfig.
	errs := ValidateConfig(cfg)
	if len(errs) > 0 {
		t.Errorf("ValidateConfig on defaults: unexpected errors: %v", errs)
	}
}

// TestLoadConfigReverseEngineeringToml verifies TOML parsing for reverse_engineering section.
func TestLoadConfigReverseEngineeringToml(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `
[reverse_engineering]
mode = "multi_pass"

[reverse_engineering.drafter]
model = "haiku"

[reverse_engineering.drafter.subagents]
model = "sonnet"
type  = "explorer"
count = 5

[reverse_engineering.multi_pass]
passes = 3

[reverse_engineering.reconcile]
min_rounds = 1
max_rounds = 2
colleague_review = true

[reverse_engineering.survey]
model = "haiku"
type  = "explorer"
count = 3
`
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	re := cfg.ReverseEngineering
	if re.Mode != "multi_pass" {
		t.Errorf("mode: got %q, want multi_pass", re.Mode)
	}
	if re.Drafter.Model != "haiku" {
		t.Errorf("drafter.model: got %q, want haiku", re.Drafter.Model)
	}
	if re.Drafter.Subagents.Count != 5 {
		t.Errorf("drafter.subagents.count: got %d, want 5", re.Drafter.Subagents.Count)
	}
	if re.MultiPass == nil || re.MultiPass.Passes != 3 {
		t.Error("multi_pass.passes: expected 3")
	}
	// Inactive mode config must not be populated.
	if re.SelfRefine != nil {
		t.Error("self_refine must be nil when mode=multi_pass")
	}
	if re.PeerReview != nil {
		t.Error("peer_review must be nil when mode=multi_pass")
	}
	if !re.Reconcile.ColleagueReview {
		t.Error("reconcile.colleague_review: want true")
	}
	if re.Survey.Count != 3 {
		t.Errorf("survey.count: got %d, want 3", re.Survey.Count)
	}
}

// TestValidateConfigReverseEngineeringInvalidMode verifies invalid mode is rejected.
func TestValidateConfigReverseEngineeringInvalidMode(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.ReverseEngineering.Mode = "turbo"
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected error for invalid reverse_engineering.mode")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "reverse_engineering.mode") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected reverse_engineering.mode error, got: %v", errs)
	}
}

// TestValidateConfigReverseEngineeringReconcileMinExceedsMax verifies min > max is rejected.
func TestValidateConfigReverseEngineeringReconcileMinExceedsMax(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.ReverseEngineering.Reconcile.MinRounds = 5
	cfg.ReverseEngineering.Reconcile.MaxRounds = 2
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected error when reconcile.min_rounds > max_rounds")
	}
}

// TestValidateConfigReverseEngineeringSubAgentCountBelowOne verifies count < 1 is rejected.
func TestValidateConfigReverseEngineeringSubAgentCountBelowOne(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.ReverseEngineering.Drafter.Subagents.Count = 0
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected error for drafter.subagents.count < 1")
	}
}
