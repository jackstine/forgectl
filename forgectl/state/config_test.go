package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Specifying.Batch != 3 {
		t.Errorf("specifying.batch = %d, want 3", cfg.Specifying.Batch)
	}
	if cfg.Specifying.Eval.MinRounds != 1 {
		t.Errorf("specifying.eval.min_rounds = %d, want 1", cfg.Specifying.Eval.MinRounds)
	}
	if cfg.Specifying.Eval.MaxRounds != 3 {
		t.Errorf("specifying.eval.max_rounds = %d, want 3", cfg.Specifying.Eval.MaxRounds)
	}
	if cfg.Planning.Batch != 1 {
		t.Errorf("planning.batch = %d, want 1", cfg.Planning.Batch)
	}
	if cfg.Implementing.Batch != 2 {
		t.Errorf("implementing.batch = %d, want 2", cfg.Implementing.Batch)
	}
	if cfg.Paths.StateDir != ".forgectl/state" {
		t.Errorf("paths.state_dir = %q, want .forgectl/state", cfg.Paths.StateDir)
	}
	if !cfg.General.UserGuided {
		t.Error("general.user_guided should default to true")
	}
	if cfg.General.EnableCommits {
		t.Error("general.enable_commits should default to false")
	}
	if !cfg.Logs.Enabled {
		t.Error("logs.enabled should default to true")
	}
	if cfg.Logs.RetentionDays != 90 {
		t.Errorf("logs.retention_days = %d, want 90", cfg.Logs.RetentionDays)
	}
	if cfg.Logs.MaxFiles != 50 {
		t.Errorf("logs.max_files = %d, want 50", cfg.Logs.MaxFiles)
	}
}

func TestLoadConfigPartialOverride(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755)

	// Only override implementing.batch — all other values should remain default.
	toml := `[implementing]
batch = 7
`
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(toml), 0644)

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Implementing.Batch != 7 {
		t.Errorf("implementing.batch = %d, want 7 (overridden)", cfg.Implementing.Batch)
	}
	// Un-overridden fields should retain their defaults.
	if cfg.Specifying.Batch != 3 {
		t.Errorf("specifying.batch = %d, want 3 (default)", cfg.Specifying.Batch)
	}
	if cfg.Implementing.Eval.MaxRounds != 3 {
		t.Errorf("implementing.eval.max_rounds = %d, want 3 (default)", cfg.Implementing.Eval.MaxRounds)
	}
	if !cfg.General.UserGuided {
		t.Error("general.user_guided should remain true (default)")
	}
}

func TestLoadConfigMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755)
	// No config file — LoadConfig should succeed with defaults.
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig with missing file: %v", err)
	}
	if cfg.Implementing.Batch != 2 {
		t.Errorf("expected default implementing.batch=2, got %d", cfg.Implementing.Batch)
	}
}

func TestValidateConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	errs := ValidateConfig(cfg)
	if len(errs) != 0 {
		t.Errorf("ValidateConfig(DefaultConfig()) returned errors: %v", errs)
	}
}

func TestValidateConfigBatchBelowOne(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Specifying.Batch = 0
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected violation for specifying.batch=0")
	}
	found := false
	for _, e := range errs {
		if containsString(e, "specifying.batch") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected specifying.batch violation, got: %v", errs)
	}
}

func TestValidateConfigMinRoundsExceedsMax(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Implementing.Eval.MinRounds = 5
	cfg.Implementing.Eval.MaxRounds = 2
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected violation for min_rounds > max_rounds")
	}
}

func TestValidateConfigLogsRetentionDaysNegative(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logs.RetentionDays = -1
	errs := ValidateConfig(cfg)
	if len(errs) == 0 {
		t.Error("expected violation for logs.retention_days=-1")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
