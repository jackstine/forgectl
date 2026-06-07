package state

import (
	"encoding/json"
	"reflect"
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
					Model: "sonnet",
					Type:  "general-purpose",
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
					Model: "sonnet",
					Type:  "general-purpose",
					Count: 1,
				},
			},
			Reconciliation: ReconciliationConfig{
				MinRounds: 0,
				MaxRounds: 3,
				AgentConfig: AgentConfig{
					Model: "sonnet",
					Type:  "general-purpose",
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
					Model: "sonnet",
					Type:  "general-purpose",
					Count: 1,
				},
				EnableEvalOutput: false,
			},
			Refine: RefineConfig{
				AgentConfig: AgentConfig{
					Model: "sonnet",
					Type:  "general-purpose",
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
					Model: "sonnet",
					Type:  "general-purpose",
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
	// user_guided defaults to true, matching the canonical template in
	// docs/default-config.toml that scaffolding writes (defaults-equivalence).
	if cfg.General.UserGuided != true {
		t.Error("general.user_guided must default to true")
	}

	// Batch defaults match the canonical template.
	if cfg.Specifying.Batch != 3 {
		t.Errorf("specifying.batch: got %d, want 3", cfg.Specifying.Batch)
	}
	if cfg.Planning.Batch != 1 {
		t.Errorf("planning.batch: got %d, want 1", cfg.Planning.Batch)
	}
	if cfg.Implementing.Batch != 2 {
		t.Errorf("implementing.batch: got %d, want 2", cfg.Implementing.Batch)
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

	// Sub-agent spawn-point defaults match the canonical template.
	if cfg.Specifying.CrossReference.Model != "haiku" || cfg.Specifying.CrossReference.Count != 3 {
		t.Errorf("specifying.cross_reference: got model=%q count=%d, want haiku/3", cfg.Specifying.CrossReference.Model, cfg.Specifying.CrossReference.Count)
	}
	if cfg.Specifying.CrossReference.Eval.Model != "sonnet" || cfg.Specifying.CrossReference.Eval.Count != 1 {
		t.Errorf("specifying.cross_reference.eval: got model=%q count=%d, want sonnet/1", cfg.Specifying.CrossReference.Eval.Model, cfg.Specifying.CrossReference.Eval.Count)
	}
	if cfg.Specifying.Reconciliation.Model != "sonnet" || cfg.Specifying.Reconciliation.MaxRounds != 3 {
		t.Errorf("specifying.reconciliation: got model=%q max_rounds=%d, want sonnet/3", cfg.Specifying.Reconciliation.Model, cfg.Specifying.Reconciliation.MaxRounds)
	}
	if cfg.Planning.StudyCode.Model != "haiku" || cfg.Planning.StudyCode.Count != 3 {
		t.Errorf("planning.study_code: got model=%q count=%d, want haiku/3", cfg.Planning.StudyCode.Model, cfg.Planning.StudyCode.Count)
	}
	if cfg.Planning.Refine.Model != "sonnet" || cfg.Planning.Refine.Type != "general-purpose" {
		t.Errorf("planning.refine: got model=%q type=%q, want sonnet/general-purpose", cfg.Planning.Refine.Model, cfg.Planning.Refine.Type)
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

// Functional: DefaultForgeConfig populates the ui_implementing block with batch=1,
// commit_strategy=scoped, app.ready_timeout_seconds=30, and each of the three
// loops (eval/qa/e2e) with min=1, max=3, model=opus, eval_mode=report. The
// required string keys (app.launch_command/url, e2e.test_command/test_dir) default
// empty because they are validated at the phase boundary, not defaulted.
func TestDefaultUIImplementingConfigValues(t *testing.T) {
	ui := DefaultForgeConfig().UIImplementing

	if ui.Batch != 1 {
		t.Errorf("ui_implementing.batch: got %d, want 1", ui.Batch)
	}
	if ui.CommitStrategy != "scoped" {
		t.Errorf("ui_implementing.commit_strategy: got %q, want %q", ui.CommitStrategy, "scoped")
	}
	if ui.App.ReadyTimeoutSeconds != 30 {
		t.Errorf("ui_implementing.app.ready_timeout_seconds: got %d, want 30", ui.App.ReadyTimeoutSeconds)
	}
	if ui.App.LaunchCommand != "" || ui.App.URL != "" {
		t.Errorf("ui_implementing.app required keys must default empty, got launch_command=%q url=%q", ui.App.LaunchCommand, ui.App.URL)
	}
	if ui.E2E.TestCommand != "" || ui.E2E.TestDir != "" {
		t.Errorf("ui_implementing.e2e required keys must default empty, got test_command=%q test_dir=%q", ui.E2E.TestCommand, ui.E2E.TestDir)
	}

	loops := map[string]EvalConfig{
		"eval": ui.Eval,
		"qa":   ui.QA,
		"e2e":  ui.E2E.EvalConfig,
	}
	for name, loop := range loops {
		if loop.MinRounds != 1 {
			t.Errorf("ui_implementing.%s.min_rounds: got %d, want 1", name, loop.MinRounds)
		}
		if loop.MaxRounds != 3 {
			t.Errorf("ui_implementing.%s.max_rounds: got %d, want 3", name, loop.MaxRounds)
		}
		if loop.Model != "opus" {
			t.Errorf("ui_implementing.%s.model: got %q, want opus", name, loop.Model)
		}
		if loop.EvalMode != "report" {
			t.Errorf("ui_implementing.%s.eval_mode: got %q, want report", name, loop.EvalMode)
		}
	}
}

// Functional: UIImplementingConfig marshals and unmarshals to JSON with app/eval/qa/e2e
// nested under ui_implementing, round fields promoted at each loop level (via embedded
// EvalConfig), and the e2e test_command/test_dir alongside its round fields.
func TestUIImplementingConfigJSONRoundTrip(t *testing.T) {
	original := UIImplementingConfig{
		Batch:          2,
		CommitStrategy: "scoped",
		App: UIAppConfig{
			LaunchCommand:       "npm run dev",
			URL:                 "http://localhost:3000",
			ReadyTimeoutSeconds: 45,
		},
		Eval: EvalConfig{MinRounds: 1, MaxRounds: 3, AgentConfig: AgentConfig{Model: "opus", Type: "eval", Count: 1}, EvalMode: "report"},
		QA:   EvalConfig{MinRounds: 2, MaxRounds: 4, AgentConfig: AgentConfig{Model: "sonnet", Type: "eval", Count: 1}, EvalMode: "direct"},
		E2E: UIE2EConfig{
			EvalConfig:  EvalConfig{MinRounds: 1, MaxRounds: 2, AgentConfig: AgentConfig{Model: "opus", Type: "eval", Count: 1}, EvalMode: "report"},
			TestCommand: "npx playwright test",
			TestDir:     "e2e/",
		},
	}

	data, err := json.Marshal(struct {
		UIImplementing UIImplementingConfig `json:"ui_implementing"`
	}{original})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify the JSON shape: app/eval/qa/e2e nested, round fields promoted on each loop.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	ui, ok := raw["ui_implementing"].(map[string]any)
	if !ok {
		t.Fatal("ui_implementing is not an object")
	}
	for _, key := range []string{"app", "eval", "qa", "e2e"} {
		if _, ok := ui[key].(map[string]any); !ok {
			t.Errorf("ui_implementing.%s missing or not an object", key)
		}
	}
	qa := ui["qa"].(map[string]any)
	if _, ok := qa["min_rounds"]; !ok {
		t.Error("ui_implementing.qa.min_rounds must be promoted to the loop level")
	}
	if _, ok := qa["model"]; !ok {
		t.Error("ui_implementing.qa.model must be promoted (embedded AgentConfig)")
	}
	e2e := ui["e2e"].(map[string]any)
	if _, ok := e2e["test_command"]; !ok {
		t.Error("ui_implementing.e2e.test_command missing")
	}
	if _, ok := e2e["min_rounds"]; !ok {
		t.Error("ui_implementing.e2e.min_rounds must be promoted alongside test_command")
	}

	// Round-trip back into the struct.
	var decoded struct {
		UIImplementing UIImplementingConfig `json:"ui_implementing"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := decoded.UIImplementing
	if got.App.URL != original.App.URL {
		t.Errorf("app.url: got %q, want %q", got.App.URL, original.App.URL)
	}
	if got.QA.MinRounds != original.QA.MinRounds || got.QA.EvalMode != original.QA.EvalMode {
		t.Errorf("qa round-trip mismatch: got %+v", got.QA)
	}
	if got.E2E.TestDir != original.E2E.TestDir || got.E2E.MaxRounds != original.E2E.MaxRounds {
		t.Errorf("e2e round-trip mismatch: got %+v", got.E2E)
	}
}

// Functional: NewUIImplementingState returns a zero-valued UIImplementingState
// (nil current_batch, batch_number 0) ready for ORIENT.
func TestNewUIImplementingState(t *testing.T) {
	s := NewUIImplementingState()
	if s == nil {
		t.Fatal("NewUIImplementingState returned nil")
	}
	if s.CurrentBatch != nil {
		t.Errorf("current_batch must be nil at construction, got %+v", s.CurrentBatch)
	}
	if s.BatchNumber != 0 {
		t.Errorf("batch_number must be 0 at construction, got %d", s.BatchNumber)
	}
	if s.CurrentLayer != nil {
		t.Errorf("current_layer must be nil at construction, got %+v", s.CurrentLayer)
	}
	if s.LayerHistory == nil {
		t.Error("layer_history should be initialized (non-nil empty slice)")
	}
}

// Functional: a ForgeState carrying a populated UIImplementing with all three round
// counters and histories round-trips through Save/Load unchanged.
func TestForgeStateUIImplementingSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	original := &ForgeState{
		Phase:          PhaseUIImplementing,
		State:          StateE2EVerify,
		StartedAtPhase: PhaseUIImplementing,
		Config:         DefaultForgeConfig(),
		UIImplementing: &UIImplementingState{
			CurrentLayer: &LayerRef{ID: "L0", Name: "Foundations"},
			BatchNumber:  2,
			CurrentBatch: &UIBatchState{
				Items:              []string{"a", "b"},
				CurrentItemIndex:   1,
				EvalRound:          2,
				QARound:            3,
				E2ERound:           1,
				Evals:              []EvalRecord{{Round: 1, Verdict: "FAIL", EvalReport: "evals/code-r1.md"}, {Round: 2, Verdict: "PASS"}},
				QAEvals:            []EvalRecord{{Round: 1, Verdict: "PASS", EvalReport: "evals/qa-r1.md"}},
				E2EEvals:           []EvalRecord{{Round: 1, Verdict: "PASS"}},
				HandedOffArtifacts: []string{"qa/batch-2-steps.json"},
				CodeForceAccepted:  true,
				QAForceAccepted:    false,
				E2EForceAccepted:   false,
			},
			LayerHistory: []UILayerHistory{
				{
					LayerID: "L0",
					Batches: []UIBatchHistory{
						{
							BatchNumber: 1,
							Items:       []string{"x"},
							EvalRounds:  1, QARounds: 2, E2ERounds: 1,
							Evals:    []EvalRecord{{Round: 1, Verdict: "PASS"}},
							QAEvals:  []EvalRecord{{Round: 1, Verdict: "FAIL"}, {Round: 2, Verdict: "PASS"}},
							E2EEvals: []EvalRecord{{Round: 1, Verdict: "PASS"}},
						},
					},
				},
			},
			CurrentPlanFile:   "forgectl/.forge_workspace/implementation_plan/plan.json",
			CurrentPlanDomain: "forgectl",
		},
	}

	if err := Save(dir, original); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !reflect.DeepEqual(original.UIImplementing, loaded.UIImplementing) {
		t.Errorf("UIImplementing round-trip mismatch:\n original = %+v\n loaded  = %+v", original.UIImplementing, loaded.UIImplementing)
	}
}
