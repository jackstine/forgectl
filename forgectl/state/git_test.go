package state

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestGitRepo creates a git repo in dir with an initial commit, configured for testing.
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	// Create initial commit so HEAD exists.
	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("init"), 0644)
	run("add", "README.md")
	run("commit", "-m", "init")
}

func TestAutoCommitUnknownStrategyReturnsError(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	_, err := AutoCommit(dir, "unknown-strategy", nil, "msg")
	if err == nil {
		t.Error("expected error for unknown strategy")
	}
	if err != nil && !strings.Contains(err.Error(), "unknown commit strategy") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAutoCommitStrictStagesSpecificFiles(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Create a file to stage.
	specFile := filepath.Join(dir, "spec.md")
	os.WriteFile(specFile, []byte("spec content"), 0644)

	hash, err := AutoCommit(dir, "strict", []string{"spec.md"}, "add spec")
	if err != nil {
		t.Fatalf("AutoCommit failed: %v", err)
	}
	if len(hash) == 0 {
		t.Error("expected non-empty commit hash")
	}
}

func TestAutoCommitScopedStagesDomainDir(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Create a file in a domain dir.
	domainDir := filepath.Join(dir, "myapp")
	os.MkdirAll(domainDir, 0755)
	os.WriteFile(filepath.Join(domainDir, "code.go"), []byte("package main"), 0644)

	hash, err := AutoCommit(dir, "scoped", []string{"myapp/"}, "implement myapp")
	if err != nil {
		t.Fatalf("AutoCommit failed: %v", err)
	}
	if len(hash) == 0 {
		t.Error("expected non-empty commit hash")
	}
}

func TestAutoCommitTrackedUsesU(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Modify already-tracked README.
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("updated"), 0644)

	hash, err := AutoCommit(dir, "tracked", nil, "update readme")
	if err != nil {
		t.Fatalf("AutoCommit failed: %v", err)
	}
	if len(hash) == 0 {
		t.Error("expected non-empty commit hash")
	}
}

func TestAutoCommitAllUsesA(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Create an untracked file.
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	hash, err := AutoCommit(dir, "all", nil, "add all")
	if err != nil {
		t.Fatalf("AutoCommit failed: %v", err)
	}
	if len(hash) == 0 {
		t.Error("expected non-empty commit hash")
	}
}

func TestAutoCommitRegistersHashOnCompletedSpecs(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Write a spec file.
	specFile := "specs/a.md"
	os.MkdirAll(filepath.Join(dir, "specs"), 0755)
	os.WriteFile(filepath.Join(dir, specFile), []byte("spec a"), 0644)

	s := &ForgeState{
		Config: ForgeConfig{
			General:    GeneralConfig{EnableCommits: true},
			Specifying: SpecifyingConfig{CommitStrategy: "strict"},
		},
		Specifying: &SpecifyingState{
			Completed: []CompletedSpec{
				{ID: 1, Name: "Spec A", Domain: "test", File: specFile},
			},
		},
	}

	// Simulate advancing from COMPLETE with enable_commits=true.
	s.State = StateComplete
	err := advanceSpecifying(s, AdvanceInput{Message: "spec commit"}, dir)
	if err != nil {
		t.Fatalf("advanceSpecifying COMPLETE failed: %v", err)
	}

	if s.Specifying.Completed[0].CommitHash == "" {
		t.Error("expected CommitHash to be registered on completed spec")
	}
	if len(s.Specifying.Completed[0].CommitHashes) == 0 {
		t.Error("expected CommitHashes to be non-empty")
	}
}

func TestAutoCommitGitFailureDoesNotAdvanceState(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	// Try to commit with nothing staged (no files changed) — git commit should fail.
	s := &ForgeState{
		Config: ForgeConfig{
			General:    GeneralConfig{EnableCommits: true},
			Specifying: SpecifyingConfig{CommitStrategy: "strict"},
		},
		Specifying: &SpecifyingState{
			Completed: []CompletedSpec{
				{ID: 1, Name: "Spec A", Domain: "test", File: "nonexistent.md"},
			},
		},
	}
	s.State = StateComplete

	// Attempt to commit with a file that doesn't exist — git add will fail.
	err := advanceSpecifying(s, AdvanceInput{Message: "commit msg"}, dir)
	if err == nil {
		t.Error("expected error when git commit fails")
	}
	// State should NOT have advanced.
	if s.State != StateComplete {
		t.Errorf("state should remain COMPLETE on git failure, got %s", s.State)
	}
}

// --- Helper to create planning state with a valid plan.json for commit tests ---

func newPlanningStateForCommit(t *testing.T, dir string) *ForgeState {
	t.Helper()
	planFile := "impl/plan.json"
	createValidPlan(t, dir, planFile)

	return &ForgeState{
		Phase: PhasePlanning,
		State: StateAccept,
		Config: ForgeConfig{
			General: GeneralConfig{EnableCommits: false},
			Planning: PlanningConfig{
				CommitStrategy:           "strict",
				PlanAllBeforeImplementing: false,
				Eval:                     EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{
				ID:     1,
				Name:   "Test Plan",
				Domain: "test",
				File:   planFile,
			},
			Queue:     []PlanQueueEntry{},
			Completed: []CompletedPlan{},
		},
	}
}

func TestPlanningAcceptWithCommitsEnabledAutoCommits(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)

	s := newPlanningStateForCommit(t, dir)
	s.Config.General.EnableCommits = true
	s.Config.Planning.CommitStrategy = "strict"
	s.Planning.Round = 1
	s.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "PASS"}}

	// Stage the plan file first so it's tracked.
	cmd := exec.Command("git", "-C", dir, "add", "impl/")
	cmd.Run()
	cmd2 := exec.Command("git", "-C", dir, "commit", "-m", "plan draft")
	cmd2.Dir = dir
	cmd2.Run()

	// Modify plan so there's something to commit.
	planPath := filepath.Join(dir, "impl/plan.json")
	data, _ := os.ReadFile(planPath)
	var plan PlanJSON
	json.Unmarshal(data, &plan)
	plan.Context.Module = "modified"
	data, _ = json.MarshalIndent(plan, "", "  ")
	os.WriteFile(planPath, data, 0644)

	err := advancePlanning(s, AdvanceInput{Message: "accept plan"}, dir)
	if err != nil {
		t.Fatalf("advancePlanning ACCEPT failed: %v", err)
	}
	if s.State != StatePhaseShift {
		t.Errorf("expected PHASE_SHIFT, got %s", s.State)
	}
}
