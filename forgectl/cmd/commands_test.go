package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"forgectl/state"
)

// setupProjectRoot creates a .forgectl/ directory in dir so FindProjectRoot succeeds.
func setupProjectRoot(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755); err != nil {
		t.Fatalf("creating .forgectl: %v", err)
	}
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	setupProjectRoot(t, dir)

	// Write spec queue.
	input := state.SpecQueueInput{
		Specs: []state.SpecQueueEntry{
			{Name: "Spec A", Domain: "test", Topic: "topic A", File: "specs/a.md", PlanningSources: []string{}, DependsOn: []string{}},
			{Name: "Spec B", Domain: "test", Topic: "topic B", File: "specs/b.md", PlanningSources: []string{}, DependsOn: []string{}},
		},
	}
	data, _ := json.Marshal(input)
	queueFile := filepath.Join(dir, "specs-queue.json")
	os.WriteFile(queueFile, data, 0644)

	// Run init.
	stateDir = dir
	initFrom = queueFile
	initPhase = "specifying"
	initGuided = true
	initNoGuided = false

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runInit(initCmd, nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Load state and verify.
	s, err := state.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if s.Phase != state.PhaseSpecifying {
		t.Errorf("phase = %s, want specifying", s.Phase)
	}
	if s.State != state.StateOrient {
		t.Errorf("state = %s, want ORIENT", s.State)
	}
	if len(s.Specifying.Queue) != 2 {
		t.Errorf("queue has %d specs, want 2", len(s.Specifying.Queue))
	}
	if s.SessionID == "" {
		t.Error("session_id must be set after init")
	}
}

func TestInitRejectsExistingState(t *testing.T) {
	dir := t.TempDir()
	setupProjectRoot(t, dir)

	// Create existing state.
	s := &state.ForgeState{Phase: state.PhaseSpecifying, State: state.StateOrient}
	state.Save(dir, s)

	stateDir = dir
	initFrom = "dummy"
	initPhase = "specifying"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for existing state file")
	}
}

func TestInitRejectsGeneratePlanningQueuePhase(t *testing.T) {
	dir := t.TempDir()
	setupProjectRoot(t, dir)

	stateDir = dir
	initFrom = "dummy"
	initPhase = "generate_planning_queue"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Fatal("expected error for generate_planning_queue phase")
	}
	if err.Error() != "generate_planning_queue requires a completed specifying phase. Use --phase specifying instead." {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitRejectsInvalidPhase(t *testing.T) {
	dir := t.TempDir()
	setupProjectRoot(t, dir)

	stateDir = dir
	initFrom = "dummy"
	initPhase = "invalid"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for invalid phase")
	}
}

func TestInitRejectsBadConfigMinMaxRounds(t *testing.T) {
	dir := t.TempDir()
	setupProjectRoot(t, dir)

	// Write a config with min > max.
	tomlContent := `
[implementing.eval]
min_rounds = 5
max_rounds = 2
`
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(tomlContent), 0644)

	stateDir = dir
	initFrom = "dummy"
	initPhase = "specifying"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for min_rounds > max_rounds in config")
	}
}

func TestInitSetsSessionID(t *testing.T) {
	dir := t.TempDir()
	setupProjectRoot(t, dir)

	input := state.SpecQueueInput{
		Specs: []state.SpecQueueEntry{
			{Name: "Spec A", Domain: "x", Topic: "t", File: "specs/a.md", PlanningSources: []string{}, DependsOn: []string{}},
		},
	}
	data, _ := json.Marshal(input)
	queueFile := filepath.Join(dir, "queue.json")
	os.WriteFile(queueFile, data, 0644)

	stateDir = dir
	initFrom = queueFile
	initPhase = "specifying"
	initGuided = false
	initNoGuided = false

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("init: %v", err)
	}

	s, _ := state.Load(dir)
	if s.SessionID == "" {
		t.Error("session_id not set")
	}
}

func TestStatusCommand(t *testing.T) {
	dir := t.TempDir()
	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateOrient,
		StartedAtPhase: state.PhaseSpecifying,
		Config: state.ForgeConfig{
			General:    state.GeneralConfig{UserGuided: true},
			Specifying: state.SpecifyingConfig{Batch: 1, CommitStrategy: "all-specs", Eval: state.EvalConfig{MinRounds: 1, MaxRounds: 3}},
		},
		Specifying: state.NewSpecifyingState([]state.SpecQueueEntry{}),
	}
	state.Save(dir, s)

	stateDir = dir
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runStatus(statusCmd, nil)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("status should produce output")
	}
}
