package state

import (
	"encoding/json"
	"testing"
)

// TestForgeConfigJSONRoundTrip verifies ForgeConfig marshals/unmarshals correctly,
// including that AgentConfig fields are embedded (promoted) at the parent JSON level
// rather than nested under an "agent_config" key.
func TestForgeConfigJSONRoundTrip(t *testing.T) {
	original := ForgeConfig{
		General: GeneralConfig{
			EnableCommits: false,
			UserGuided:    true,
		},
		Domains: []DomainConfig{
			{Name: "optimizer", Path: "optimizer"},
			{Name: "portal", Path: "portal"},
		},
		Specifying: SpecifyingConfig{
			Batch:          3,
			CommitStrategy: "all-specs",
			Eval: EvalConfig{
				MinRounds: 1,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
				EnableEvalOutput: false,
			},
			CrossReference: CrossRefConfig{
				MinRounds: 1,
				MaxRounds: 2,
				AgentConfig: AgentConfig{
					Model: "haiku",
					Type:  "explore",
					Count: 3,
				},
				UserReview: false,
				Eval: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
			},
			Reconciliation: ReconciliationConfig{
				MinRounds: 0,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
				UserReview: false,
			},
		},
		Planning: PlanningConfig{
			Batch:                     1,
			CommitStrategy:            "strict",
			SelfReview:                false,
			PlanAllBeforeImplementing: false,
			StudyCode: StudyCodeConfig{
				AgentConfig: AgentConfig{
					Model: "haiku",
					Type:  "explore",
					Count: 3,
				},
			},
			Eval: EvalConfig{
				MinRounds: 1,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
				EnableEvalOutput: false,
			},
			Refine: RefineConfig{
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "refine",
					Count: 1,
				},
			},
		},
		Implementing: ImplementingConfig{
			Batch:          2,
			CommitStrategy: "scoped",
			Eval: EvalConfig{
				MinRounds: 1,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "opus",
					Type:  "eval",
					Count: 1,
				},
				EnableEvalOutput: false,
			},
		},
		Paths: PathsConfig{
			StateDir:     ".forgectl/state",
			WorkspaceDir: ".forge_workspace",
		},
		Logs: LogsConfig{
			Enabled:       true,
			RetentionDays: 90,
			MaxFiles:      50,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify AgentConfig fields are promoted to parent level (not nested under "agent_config").
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to raw: %v", err)
	}

	specifying, ok := raw["specifying"].(map[string]any)
	if !ok {
		t.Fatal("specifying is not an object")
	}
	eval, ok := specifying["eval"].(map[string]any)
	if !ok {
		t.Fatal("specifying.eval is not an object")
	}
	// model/type/count must be at eval level, not under agent_config
	if _, hasModel := eval["model"]; !hasModel {
		t.Error("specifying.eval.model missing — AgentConfig must be embedded (promoted), not nested")
	}
	if _, hasAgentConfig := eval["agent_config"]; hasAgentConfig {
		t.Error("specifying.eval.agent_config must not exist — fields should be promoted by embedding")
	}

	// cross_reference.eval must be a sub-object (separate named field, not embedded)
	crossRef, ok := specifying["cross_reference"].(map[string]any)
	if !ok {
		t.Fatal("specifying.cross_reference is not an object")
	}
	if _, hasCREval := crossRef["eval"]; !hasCREval {
		t.Error("specifying.cross_reference.eval sub-object missing")
	}

	// Round-trip: unmarshal back to ForgeConfig
	var decoded ForgeConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Specifying.Eval.Model != original.Specifying.Eval.Model {
		t.Errorf("specifying.eval.model: got %q, want %q", decoded.Specifying.Eval.Model, original.Specifying.Eval.Model)
	}
	if decoded.Planning.StudyCode.Model != original.Planning.StudyCode.Model {
		t.Errorf("planning.study_code.model: got %q, want %q", decoded.Planning.StudyCode.Model, original.Planning.StudyCode.Model)
	}
	if decoded.Specifying.CrossReference.Eval.Model != original.Specifying.CrossReference.Eval.Model {
		t.Errorf("cross_reference.eval.model: got %q, want %q", decoded.Specifying.CrossReference.Eval.Model, original.Specifying.CrossReference.Eval.Model)
	}
	if len(decoded.Domains) != len(original.Domains) {
		t.Errorf("domains length: got %d, want %d", len(decoded.Domains), len(original.Domains))
	}
}

// TestDefaultForgeConfigValues verifies spec-defined defaults are correct.
func TestDefaultForgeConfigValues(t *testing.T) {
	cfg := DefaultForgeConfig()

	// General defaults
	if cfg.General.EnableCommits != false {
		t.Error("general.enable_commits must default to false")
	}
	if cfg.General.UserGuided != false {
		t.Error("general.user_guided must default to false")
	}

	// Commit strategy defaults
	if cfg.Specifying.CommitStrategy != "all-specs" {
		t.Errorf("specifying.commit_strategy: got %q, want %q", cfg.Specifying.CommitStrategy, "all-specs")
	}
	if cfg.Planning.CommitStrategy != "strict" {
		t.Errorf("planning.commit_strategy: got %q, want %q", cfg.Planning.CommitStrategy, "strict")
	}
	if cfg.Implementing.CommitStrategy != "scoped" {
		t.Errorf("implementing.commit_strategy: got %q, want %q", cfg.Implementing.CommitStrategy, "scoped")
	}

	// Min/max round defaults
	if cfg.Specifying.Eval.MinRounds != 1 {
		t.Errorf("specifying.eval.min_rounds: got %d, want 1", cfg.Specifying.Eval.MinRounds)
	}
	if cfg.Specifying.Eval.MaxRounds != 3 {
		t.Errorf("specifying.eval.max_rounds: got %d, want 3", cfg.Specifying.Eval.MaxRounds)
	}
	if cfg.Planning.Eval.MinRounds != 1 {
		t.Errorf("planning.eval.min_rounds: got %d, want 1", cfg.Planning.Eval.MinRounds)
	}
	if cfg.Planning.Eval.MaxRounds != 3 {
		t.Errorf("planning.eval.max_rounds: got %d, want 3", cfg.Planning.Eval.MaxRounds)
	}
	if cfg.Implementing.Eval.MinRounds != 1 {
		t.Errorf("implementing.eval.min_rounds: got %d, want 1", cfg.Implementing.Eval.MinRounds)
	}
	if cfg.Implementing.Eval.MaxRounds != 3 {
		t.Errorf("implementing.eval.max_rounds: got %d, want 3", cfg.Implementing.Eval.MaxRounds)
	}

	// Path defaults
	if cfg.Paths.StateDir != ".forgectl/state" {
		t.Errorf("paths.state_dir: got %q, want %q", cfg.Paths.StateDir, ".forgectl/state")
	}
	if cfg.Paths.WorkspaceDir != ".forge_workspace" {
		t.Errorf("paths.workspace_dir: got %q, want %q", cfg.Paths.WorkspaceDir, ".forge_workspace")
	}

	// Log defaults
	if !cfg.Logs.Enabled {
		t.Error("logs.enabled must default to true")
	}
	if cfg.Logs.RetentionDays != 90 {
		t.Errorf("logs.retention_days: got %d, want 90", cfg.Logs.RetentionDays)
	}
	if cfg.Logs.MaxFiles != 50 {
		t.Errorf("logs.max_files: got %d, want 50", cfg.Logs.MaxFiles)
	}
}
