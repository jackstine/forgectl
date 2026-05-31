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

// Functional: DefaultForgeConfig seeds the reverse_engineering block with the
// spec-defined defaults.
func TestDefaultReverseEngineeringConfig(t *testing.T) {
	re := DefaultForgeConfig().ReverseEngineering

	checkAgent := func(name string, got AgentConfig, model, typ string, count int) {
		if got.Model != model || got.Type != typ || got.Count != count {
			t.Errorf("%s = %+v, want {model:%s type:%s count:%d}", name, got, model, typ, count)
		}
	}
	checkAgent("execute", re.Execute, "haiku", "explorer", 3)
	checkAgent("survey", re.Survey, "haiku", "explorer", 2)
	checkAgent("gap_analysis", re.GapAnalysis, "sonnet", "explorer", 5)

	if re.Reconcile.MinRounds != 1 || re.Reconcile.MaxRounds != 3 {
		t.Errorf("reconcile rounds = %d-%d, want 1-3", re.Reconcile.MinRounds, re.Reconcile.MaxRounds)
	}
	if re.Reconcile.ColleagueReview {
		t.Error("reconcile.colleague_review: want false by default")
	}
	checkAgent("reconcile.eval", re.Reconcile.Eval, "opus", "general-purpose", 1)
}

// Functional: a [reverse_engineering] TOML block overrides the defaults, and an
// explicit colleague_review = true is honored.
func TestLoadConfigReverseEngineering(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `
[reverse_engineering.execute]
model = "opus"
type  = "general-purpose"
count = 4

[reverse_engineering.reconcile]
min_rounds       = 2
max_rounds       = 5
colleague_review = true

[reverse_engineering.reconcile.eval]
model = "sonnet"
count = 2

[reverse_engineering.survey]
count = 7
`
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	re := cfg.ReverseEngineering

	if re.Execute.Model != "opus" || re.Execute.Type != "general-purpose" || re.Execute.Count != 4 {
		t.Errorf("execute = %+v, want {opus general-purpose 4}", re.Execute)
	}
	if re.Reconcile.MinRounds != 2 || re.Reconcile.MaxRounds != 5 {
		t.Errorf("reconcile rounds = %d-%d, want 2-5", re.Reconcile.MinRounds, re.Reconcile.MaxRounds)
	}
	if !re.Reconcile.ColleagueReview {
		t.Error("reconcile.colleague_review: want true (explicit)")
	}
	// eval.model overridden, count overridden, type falls back to the default.
	if re.Reconcile.Eval.Model != "sonnet" || re.Reconcile.Eval.Count != 2 {
		t.Errorf("reconcile.eval = %+v, want model sonnet / count 2", re.Reconcile.Eval)
	}
	if re.Reconcile.Eval.Type != "general-purpose" {
		t.Errorf("reconcile.eval.type = %q, want default general-purpose", re.Reconcile.Eval.Type)
	}
	// survey.count overridden; model/type retain defaults.
	if re.Survey.Count != 7 || re.Survey.Model != "haiku" || re.Survey.Type != "explorer" {
		t.Errorf("survey = %+v, want count 7 with haiku/explorer defaults", re.Survey)
	}
	// gap_analysis untouched — full defaults.
	if re.GapAnalysis.Model != "sonnet" || re.GapAnalysis.Count != 5 {
		t.Errorf("gap_analysis = %+v, want defaults sonnet/5", re.GapAnalysis)
	}
}

// Rejection: reverse_engineering.reconcile.min_rounds may not exceed max_rounds.
func TestValidateConfigReverseEngineeringMinExceedsMax(t *testing.T) {
	cfg := DefaultForgeConfig()
	cfg.ReverseEngineering.Reconcile.MinRounds = 5
	cfg.ReverseEngineering.Reconcile.MaxRounds = 3
	errs := ValidateConfig(cfg)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "reverse_engineering.reconcile.min_rounds cannot exceed max_rounds") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected reverse_engineering.reconcile min>max violation, got: %v", errs)
	}
}

// TestDefaultConfigEvalModeSeeded verifies DefaultForgeConfig seeds eval_mode="report"
// on the specifying, planning, and implementing eval blocks.
func TestDefaultConfigEvalModeSeeded(t *testing.T) {
	cfg := DefaultForgeConfig()
	for _, tc := range []struct {
		phase string
		mode  string
	}{
		{"specifying", cfg.Specifying.Eval.EvalMode},
		{"planning", cfg.Planning.Eval.EvalMode},
		{"implementing", cfg.Implementing.Eval.EvalMode},
	} {
		if tc.mode != "report" {
			t.Errorf("%s.eval.eval_mode: got %q, want %q", tc.phase, tc.mode, "report")
		}
	}
}

// TestLoadConfigEvalModeOverride verifies a [<phase>.eval] TOML block with
// eval_mode="direct" overrides the seeded default (this is the value locked into
// the state file at init via cmd/init.go Config: cfg).
func TestLoadConfigEvalModeOverride(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `
[implementing.eval]
eval_mode = "direct"
`
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Implementing.Eval.EvalMode != "direct" {
		t.Errorf("implementing.eval.eval_mode: got %q, want %q", cfg.Implementing.Eval.EvalMode, "direct")
	}
}

// TestLoadConfigEvalModeOmittedKeepsDefault verifies that omitting eval_mode in a
// TOML eval block leaves the seeded default in place: a nil pointer source must not
// zero the destination during merge.
func TestLoadConfigEvalModeOmittedKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}

	// An eval block that sets other fields but omits eval_mode entirely.
	tomlContent := `
[implementing.eval]
min_rounds = 2
max_rounds = 4
`
	if err := os.WriteFile(filepath.Join(forgectlDir, "config"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Implementing.Eval.EvalMode != "report" {
		t.Errorf("implementing.eval.eval_mode: got %q, want default %q", cfg.Implementing.Eval.EvalMode, "report")
	}
}
