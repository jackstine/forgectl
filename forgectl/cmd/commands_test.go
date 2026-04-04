package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"forgectl/state"
)

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()

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
	initBatchSize = 2
	initMinRounds = 1
	initMaxRounds = 3
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
}

func TestInitRejectsExistingState(t *testing.T) {
	dir := t.TempDir()

	// Create existing state.
	s := &state.ForgeState{Phase: state.PhaseSpecifying, State: state.StateOrient}
	state.Save(dir, s)

	stateDir = dir
	initFrom = "dummy"
	initBatchSize = 1
	initMinRounds = 1
	initMaxRounds = 3
	initPhase = "specifying"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for existing state file")
	}
}

func TestInitRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name      string
		batchSize int
		minRounds int
		maxRounds int
	}{
		{"batch-size 0", 0, 1, 3},
		{"min exceeds max", 2, 5, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir = t.TempDir()
			initFrom = "dummy"
			initBatchSize = tt.batchSize
			initMinRounds = tt.minRounds
			initMaxRounds = tt.maxRounds
			initPhase = "specifying"

			err := runInit(initCmd, nil)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestInitRejectsInvalidPhase(t *testing.T) {
	stateDir = t.TempDir()
	initFrom = "dummy"
	initBatchSize = 1
	initMinRounds = 1
	initMaxRounds = 3
	initPhase = "invalid"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for invalid phase")
	}
}

func TestStatusCommand(t *testing.T) {
	dir := t.TempDir()
	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateOrient,
		StartedAtPhase: state.PhaseSpecifying,
		Config: state.ForgeConfig{
			General: state.GeneralConfig{UserGuided: true},
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
