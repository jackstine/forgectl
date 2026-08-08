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

func TestAutoCommitScopedFromSubdirectory(t *testing.T) {
	// Simulate: .forgectl/ lives inside a domain subdirectory so projectRoot
	// is not the git root. AutoCommit must still stage files correctly by
	// resolving the git root and converting stage targets to absolute paths.
	repoRoot := t.TempDir()
	initTestGitRepo(t, repoRoot)

	// projectRoot is INSIDE the repo (simulates .forgectl/ nested in a subdomain).
	projectRoot := filepath.Join(repoRoot, "backend")
	os.MkdirAll(projectRoot, 0755)

	// A file at projectRoot/api.go — stageTarget "api.go" is relative to projectRoot.
	os.WriteFile(filepath.Join(projectRoot, "api.go"), []byte("package api"), 0644)

	// With old code: git -C <projectRoot> add api.go → stages projectRoot/api.go ✓
	// With new code: abs = projectRoot/api.go, git -C <gitRoot> add <abs> → same result.
	// The important property verified here: gitRoot (repoRoot) != projectRoot, and
	// the commit still succeeds because AutoCommit resolves the root correctly.
	hash, err := AutoCommit(projectRoot, "scoped", []string{"api.go"}, "add api")
	if err != nil {
		t.Fatalf("AutoCommit from subdirectory failed: %v", err)
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

	if len(s.Specifying.Completed[0].CommitHashes) == 0 {
		t.Error("expected CommitHashes to be registered on completed spec")
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

// --- Commit message synthesis -------------------------------------------------
//
// The implementing and ui_implementing commit points no longer require an
// operator-supplied message: the scaffold synthesizes one from plan.json. These
// tests pin the exact strings because the synthesized message is the only
// record of what a batch did — a reformatted or truncated description makes the
// commit history unreadable, and an empty one makes git reject the commit
// outright.

func planForMessages() *PlanJSON {
	return &PlanJSON{
		Items: []PlanItem{
			{ID: "cfg.types", Description: "ServiceEndpoint and ServicesConfig structs"},
			{ID: "cfg.load", Description: "Load YAML, apply defaults, validate strictly"},
			{ID: "cfg.blank", Description: ""},
		},
	}
}

func TestItemCommitMessageIsDescriptionVerbatim(t *testing.T) {
	plan := planForMessages()

	got := ItemCommitMessage(plan, "cfg.load")
	want := "Load YAML, apply defaults, validate strictly"
	if got != want {
		t.Errorf("ItemCommitMessage = %q, want %q", got, want)
	}
}

func TestItemCommitMessageMissingItemFallsBackToID(t *testing.T) {
	plan := planForMessages()

	// A missing ID must not panic, and must not yield an empty message —
	// git commit -m "" aborts.
	if got := ItemCommitMessage(plan, "no.such.item"); got != "no.such.item" {
		t.Errorf("missing item: got %q, want the item ID", got)
	}
	if got := ItemCommitMessage(nil, "cfg.load"); got != "cfg.load" {
		t.Errorf("nil plan: got %q, want the item ID", got)
	}
	if got := ItemCommitMessage(plan, "cfg.blank"); got != "cfg.blank" {
		t.Errorf("blank description: got %q, want the item ID", got)
	}
}

func TestBatchCommitMessageJoinsDescriptionsInBatchOrder(t *testing.T) {
	plan := planForMessages()

	got := BatchCommitMessage(plan, 2, []string{"cfg.load", "cfg.types"})
	want := "Batch 2: Load YAML, apply defaults, validate strictly; ServiceEndpoint and ServicesConfig structs"
	if got != want {
		t.Errorf("BatchCommitMessage = %q, want %q", got, want)
	}
}

func TestBatchCommitMessageSingleItemHasNoTrailingSeparator(t *testing.T) {
	plan := planForMessages()

	got := BatchCommitMessage(plan, 1, []string{"cfg.types"})
	want := "Batch 1: ServiceEndpoint and ServicesConfig structs"
	if got != want {
		t.Errorf("BatchCommitMessage = %q, want %q", got, want)
	}
	if strings.HasSuffix(got, ";") || strings.HasSuffix(got, "; ") {
		t.Errorf("single-item batch message has a trailing separator: %q", got)
	}
}

func TestBatchCommitMessageSkipsUnresolvableItems(t *testing.T) {
	plan := planForMessages()

	// Unresolvable and blank-description items contribute no empty segment.
	got := BatchCommitMessage(plan, 3, []string{"cfg.types", "no.such.item", "cfg.blank"})
	want := "Batch 3: ServiceEndpoint and ServicesConfig structs"
	if got != want {
		t.Errorf("BatchCommitMessage = %q, want %q", got, want)
	}

	// With nothing resolvable at all the message still must not be empty.
	if got := BatchCommitMessage(plan, 4, []string{"no.such.item"}); got != "Batch 4" {
		t.Errorf("all-unresolvable batch: got %q, want %q", got, "Batch 4")
	}
	if got := BatchCommitMessage(nil, 5, []string{"cfg.types"}); got != "Batch 5" {
		t.Errorf("nil plan: got %q, want %q", got, "Batch 5")
	}
}

func TestAppendSuppliedMessageAppendsAsSecondParagraph(t *testing.T) {
	synthesized := "Load YAML, apply defaults, validate strictly"

	got := AppendSuppliedMessage(synthesized, "double-checked against staging config")
	want := "Load YAML, apply defaults, validate strictly\n\ndouble-checked against staging config"
	if got != want {
		t.Errorf("AppendSuppliedMessage = %q, want %q", got, want)
	}
	// The synthesized text is augmented, never replaced.
	if !strings.HasPrefix(got, synthesized) {
		t.Errorf("synthesized message was not preserved as the first paragraph: %q", got)
	}
}

func TestAppendSuppliedMessageWithoutSuppliedTextIsUnchanged(t *testing.T) {
	synthesized := "Load YAML, apply defaults, validate strictly"

	for _, supplied := range []string{"", "   ", "\n\t "} {
		got := AppendSuppliedMessage(synthesized, supplied)
		if got != synthesized {
			t.Errorf("AppendSuppliedMessage(%q) = %q, want %q", supplied, got, synthesized)
		}
		if strings.HasSuffix(got, "\n") {
			t.Errorf("AppendSuppliedMessage(%q) left a trailing newline: %q", supplied, got)
		}
	}
}
