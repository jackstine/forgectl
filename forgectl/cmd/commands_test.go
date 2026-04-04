package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgectl/state"
)

// setupProjectDir creates a temp dir with .forgectl/ and an empty config,
// changes cwd into it, and registers cleanup to restore cwd.
func setupProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	// Empty config — all defaults apply.
	if err := os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return dir
}

// resolvedStateDir returns the expected state dir for a project root using DefaultForgeConfig.
func resolvedStateDir(projectRoot string) string {
	cfg := state.DefaultForgeConfig()
	return state.StateDir(projectRoot, cfg)
}

func TestInitCommand(t *testing.T) {
	dir := setupProjectDir(t)

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

	initFrom = queueFile
	initPhase = "specifying"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runInit(initCmd, nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Load state from the resolved state dir and verify.
	sd := resolvedStateDir(dir)
	s, err := state.Load(sd)
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
		t.Error("session_id should be set")
	}
}

func TestInitLocksConfig(t *testing.T) {
	dir := setupProjectDir(t)

	// Write a custom config to verify it gets locked in.
	customCfg := `[specifying]
batch = 5

[implementing]
batch = 7
`
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(customCfg), 0644)

	input := state.SpecQueueInput{
		Specs: []state.SpecQueueEntry{
			{Name: "Spec A", Domain: "test", Topic: "t", File: "a.md", PlanningSources: []string{}, DependsOn: []string{}},
		},
	}
	data, _ := json.Marshal(input)
	queueFile := filepath.Join(dir, "queue.json")
	os.WriteFile(queueFile, data, 0644)

	initFrom = queueFile
	initPhase = "specifying"

	err := runInit(initCmd, nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	sd := resolvedStateDir(dir)
	s, err := state.Load(sd)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if s.Config.Specifying.Batch != 5 {
		t.Errorf("config.specifying.batch = %d, want 5", s.Config.Specifying.Batch)
	}
	if s.Config.Implementing.Batch != 7 {
		t.Errorf("config.implementing.batch = %d, want 7", s.Config.Implementing.Batch)
	}
}

func TestInitRejectsExistingState(t *testing.T) {
	dir := setupProjectDir(t)

	// Create existing state in the resolved state dir.
	sd := resolvedStateDir(dir)
	if err := os.MkdirAll(sd, 0755); err != nil {
		t.Fatal(err)
	}
	s := &state.ForgeState{Phase: state.PhaseSpecifying, State: state.StateOrient}
	state.Save(sd, s)

	initFrom = "dummy"
	initPhase = "specifying"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for existing state file")
	}
}

func TestInitRejectsGeneratePlanningQueuePhase(t *testing.T) {
	setupProjectDir(t)

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
	setupProjectDir(t)
	initFrom = "dummy"
	initPhase = "invalid"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for invalid phase")
	}
}

func TestInitRejectsInvalidConfig(t *testing.T) {
	dir := setupProjectDir(t)

	// Write config with constraint violation.
	badCfg := `[specifying.eval]
min_rounds = 5
max_rounds = 2
`
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(badCfg), 0644)

	initFrom = "dummy"
	initPhase = "specifying"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

// --- reverse engineering init tests ---

func TestInitReverseEngineering(t *testing.T) {
	dir := setupProjectDir(t)

	input := state.ReverseEngineeringInitInput{
		Concept: "auth middleware refactor",
		Domains: []string{"optimizer", "api"},
	}
	data, _ := json.Marshal(input)
	inputFile := filepath.Join(dir, "re-init.json")
	os.WriteFile(inputFile, data, 0644)

	initFrom = inputFile
	initPhase = "reverse_engineering"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runInit(initCmd, nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	sd := resolvedStateDir(dir)
	s, err := state.Load(sd)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if s.Phase != state.PhaseReverseEngineering {
		t.Errorf("phase = %s, want reverse_engineering", s.Phase)
	}
	if s.State != state.StateOrient {
		t.Errorf("state = %s, want ORIENT", s.State)
	}
	if s.ReverseEngineering == nil {
		t.Fatal("reverse_engineering state must not be nil")
	}
	if s.ReverseEngineering.Concept != "auth middleware refactor" {
		t.Errorf("concept = %q, want %q", s.ReverseEngineering.Concept, "auth middleware refactor")
	}
	if len(s.ReverseEngineering.Domains) != 2 {
		t.Errorf("domains len = %d, want 2", len(s.ReverseEngineering.Domains))
	}
	if s.ReverseEngineering.TotalDomains != 2 {
		t.Errorf("total_domains = %d, want 2", s.ReverseEngineering.TotalDomains)
	}
	if s.ReverseEngineering.CurrentDomain != 0 {
		t.Errorf("current_domain = %d, want 0", s.ReverseEngineering.CurrentDomain)
	}
	if s.SessionID == "" {
		t.Error("session_id must be set")
	}
}

func TestInitReverseEngineeringLocksConfig(t *testing.T) {
	dir := setupProjectDir(t)

	// Write config with peer_review mode.
	tomlContent := `
[reverse_engineering]
mode = "peer_review"

[reverse_engineering.peer_review]
reviewers = 3
rounds = 1

[reverse_engineering.peer_review.subagents]
model = "opus"
type = "explorer"
`
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(tomlContent), 0644)

	input := state.ReverseEngineeringInitInput{
		Concept: "test",
		Domains: []string{"api"},
	}
	data, _ := json.Marshal(input)
	inputFile := filepath.Join(dir, "re-init.json")
	os.WriteFile(inputFile, data, 0644)

	initFrom = inputFile
	initPhase = "reverse_engineering"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	err := runInit(initCmd, nil)
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	sd := resolvedStateDir(dir)
	s, err := state.Load(sd)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if s.Config.ReverseEngineering.Mode != "peer_review" {
		t.Errorf("config.reverse_engineering.mode = %q, want peer_review", s.Config.ReverseEngineering.Mode)
	}
	// Inactive mode configs must not be present.
	if s.Config.ReverseEngineering.SelfRefine != nil {
		t.Error("self_refine must be nil when mode=peer_review")
	}
	if s.Config.ReverseEngineering.MultiPass != nil {
		t.Error("multi_pass must be nil when mode=peer_review")
	}
}

func TestInitReverseEngineeringRejectsInvalidInput(t *testing.T) {
	dir := setupProjectDir(t)

	// Missing concept.
	bad := []byte(`{"domains": ["optimizer"]}`)
	inputFile := filepath.Join(dir, "bad.json")
	os.WriteFile(inputFile, bad, 0644)

	initFrom = inputFile
	initPhase = "reverse_engineering"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for missing concept")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("expected 'input validation failed', got: %v", err)
	}
}

func TestInitReverseEngineeringRejectsDuplicateDomains(t *testing.T) {
	dir := setupProjectDir(t)

	bad, _ := json.Marshal(state.ReverseEngineeringInitInput{
		Concept: "test",
		Domains: []string{"optimizer", "optimizer"},
	})
	inputFile := filepath.Join(dir, "bad.json")
	os.WriteFile(inputFile, bad, 0644)

	initFrom = inputFile
	initPhase = "reverse_engineering"

	err := runInit(initCmd, nil)
	if err == nil {
		t.Error("expected error for duplicate domains")
	}
}

// --- add-queue-item tests ---

// setupSpecifyingState saves a specifying state at the given state and returns the state dir.
func setupSpecifyingState(t *testing.T, dir string, forgeState *state.ForgeState) string {
	t.Helper()
	sd := resolvedStateDir(dir)
	if err := os.MkdirAll(sd, 0755); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(sd, forgeState); err != nil {
		t.Fatal(err)
	}
	return sd
}

func newSpecifyingForgeState(st state.StateName) *state.ForgeState {
	cfg := state.DefaultForgeConfig()
	spec := state.NewSpecifyingState([]state.SpecQueueEntry{})
	// Add one completed spec so set-roots can use the domain.
	spec.Completed = append(spec.Completed, state.CompletedSpec{
		ID: 1, Name: "Existing Spec", Domain: "test", File: "test/specs/existing.md",
	})
	spec.CurrentDomain = "test"
	// Set up CurrentSpecs for DRAFT.
	if st == state.StateDraft {
		spec.CurrentSpecs = []*state.ActiveSpec{
			{ID: 2, Name: "Current Spec", Domain: "test", File: "test/specs/current.md"},
		}
	}
	return &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          st,
		Config:         cfg,
		StartedAtPhase: state.PhaseSpecifying,
		Specifying:     spec,
	}
}

func TestAddQueueItemInDraftState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDraft)
	sd := setupSpecifyingState(t, dir, forgeState)

	// Create the spec file.
	specFile := filepath.Join(dir, "test", "specs", "new-spec.md")
	os.MkdirAll(filepath.Dir(specFile), 0755)
	os.WriteFile(specFile, []byte("# New Spec"), 0644)

	addQueueItemName = "New Spec"
	addQueueItemDomain = ""
	addQueueItemTopic = "Some Topic"
	addQueueItemFile = specFile
	addQueueItemSources = nil

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runAddQueueItem(addQueueItemCmd, nil); err != nil {
		t.Fatalf("add-queue-item: %v", err)
	}

	s, _ := state.Load(sd)
	if len(s.Specifying.Queue) != 1 {
		t.Errorf("expected 1 queue item, got %d", len(s.Specifying.Queue))
	}
	if s.Specifying.Queue[0].Name != "New Spec" {
		t.Errorf("expected queue item name 'New Spec', got %q", s.Specifying.Queue[0].Name)
	}
}

func TestAddQueueItemInCrossReferenceReviewState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	sd := setupSpecifyingState(t, dir, forgeState)

	specFile := filepath.Join(dir, "test", "specs", "another.md")
	os.MkdirAll(filepath.Dir(specFile), 0755)
	os.WriteFile(specFile, []byte("# Another Spec"), 0644)

	addQueueItemName = "Another Spec"
	addQueueItemDomain = ""
	addQueueItemTopic = "Another Topic"
	addQueueItemFile = specFile
	addQueueItemSources = nil

	if err := runAddQueueItem(addQueueItemCmd, nil); err != nil {
		t.Fatalf("add-queue-item: %v", err)
	}

	s, _ := state.Load(sd)
	if len(s.Specifying.Queue) != 1 {
		t.Errorf("expected 1 queue item, got %d", len(s.Specifying.Queue))
	}
	if s.Specifying.Queue[0].Domain != "test" {
		t.Errorf("expected domain 'test', got %q", s.Specifying.Queue[0].Domain)
	}
}

func TestSetRootsInCrossReferenceReviewState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	sd := setupSpecifyingState(t, dir, forgeState)

	setRootsDomain = ""

	if err := runSetRoots(setRootsCmd, []string{"test/", "lib/"}); err != nil {
		t.Fatalf("set-roots: %v", err)
	}

	s, _ := state.Load(sd)
	meta, ok := s.Specifying.Domains["test"]
	if !ok {
		t.Fatal("expected domain 'test' in Domains map")
	}
	if len(meta.CodeSearchRoots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(meta.CodeSearchRoots))
	}
}

func TestSetRootsInDoneState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDone)
	sd := setupSpecifyingState(t, dir, forgeState)

	setRootsDomain = "test"

	if err := runSetRoots(setRootsCmd, []string{"test/"}); err != nil {
		t.Fatalf("set-roots: %v", err)
	}

	s, _ := state.Load(sd)
	meta, ok := s.Specifying.Domains["test"]
	if !ok {
		t.Fatal("expected domain 'test' in Domains map")
	}
	if meta.CodeSearchRoots[0] != "test/" {
		t.Errorf("expected root 'test/', got %q", meta.CodeSearchRoots[0])
	}
}

func TestAddQueueItemAtDoneWithDomain(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDone)
	sd := setupSpecifyingState(t, dir, forgeState)

	specFile := filepath.Join(dir, "test", "specs", "done-spec.md")
	os.MkdirAll(filepath.Dir(specFile), 0755)
	os.WriteFile(specFile, []byte("# Done Spec"), 0644)

	addQueueItemName = "Done Spec"
	addQueueItemDomain = "test"
	addQueueItemTopic = "Some Topic"
	addQueueItemFile = specFile
	addQueueItemSources = nil

	if err := runAddQueueItem(addQueueItemCmd, nil); err != nil {
		t.Fatalf("add-queue-item at DONE: %v", err)
	}

	s, _ := state.Load(sd)
	if len(s.Specifying.Queue) != 1 {
		t.Errorf("expected 1 queue item, got %d", len(s.Specifying.Queue))
	}
	if s.Specifying.Queue[0].Domain != "test" {
		t.Errorf("expected domain 'test', got %q", s.Specifying.Queue[0].Domain)
	}
}

// --- add-queue-item rejection tests ---

func TestAddQueueItemRejectsWrongPhase(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)
	s := &state.ForgeState{
		Phase: state.PhasePlanning, State: state.StateOrient, Config: state.DefaultForgeConfig(),
		Planning: &state.PlanningState{CurrentPlan: &state.ActivePlan{Name: "p", Domain: "d", File: "plan.json"}},
	}
	state.Save(sd, s)

	addQueueItemName = "X"
	addQueueItemTopic = "t"
	addQueueItemFile = "x.md"

	err := runAddQueueItem(addQueueItemCmd, nil)
	if err == nil || err.Error()[:len("add-queue-item is only valid in the specifying phase")] != "add-queue-item is only valid in the specifying phase" {
		t.Errorf("expected phase error, got %v", err)
	}
}

func TestAddQueueItemRejectsWrongState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateEvaluate)
	setupSpecifyingState(t, dir, forgeState)

	addQueueItemName = "X"
	addQueueItemTopic = "t"
	addQueueItemFile = "x.md"

	err := runAddQueueItem(addQueueItemCmd, nil)
	if err == nil {
		t.Error("expected error for wrong state")
	}
}

func TestAddQueueItemRejectsMissingFile(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDraft)
	setupSpecifyingState(t, dir, forgeState)

	addQueueItemName = "X"
	addQueueItemTopic = "t"
	addQueueItemFile = filepath.Join(dir, "nonexistent.md")

	err := runAddQueueItem(addQueueItemCmd, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestAddQueueItemRejectsDuplicateNameInQueue(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDraft)
	// Pre-populate queue with duplicate name.
	forgeState.Specifying.Queue = append(forgeState.Specifying.Queue, state.SpecQueueEntry{
		Name: "Duplicate", Domain: "test", Topic: "t", File: "test/specs/dup.md",
	})
	setupSpecifyingState(t, dir, forgeState)

	specFile := filepath.Join(dir, "test", "specs", "new.md")
	os.MkdirAll(filepath.Dir(specFile), 0755)
	os.WriteFile(specFile, []byte("spec"), 0644)

	addQueueItemName = "Duplicate"
	addQueueItemTopic = "t"
	addQueueItemFile = specFile

	err := runAddQueueItem(addQueueItemCmd, nil)
	if err == nil {
		t.Error("expected error for duplicate queue name")
	}
}

func TestAddQueueItemRejectsDuplicateNameInCompleted(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDraft)
	// Completed already has "Existing Spec" from helper.
	setupSpecifyingState(t, dir, forgeState)

	specFile := filepath.Join(dir, "test", "specs", "new.md")
	os.MkdirAll(filepath.Dir(specFile), 0755)
	os.WriteFile(specFile, []byte("spec"), 0644)

	addQueueItemName = "Existing Spec"
	addQueueItemTopic = "t"
	addQueueItemFile = specFile

	err := runAddQueueItem(addQueueItemCmd, nil)
	if err == nil {
		t.Error("expected error for name already in completed specs")
	}
}

func TestAddQueueItemRequiresDomainAtDone(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDone)
	setupSpecifyingState(t, dir, forgeState)

	specFile := filepath.Join(dir, "test", "specs", "new.md")
	os.MkdirAll(filepath.Dir(specFile), 0755)
	os.WriteFile(specFile, []byte("spec"), 0644)

	addQueueItemName = "Brand New"
	addQueueItemDomain = "" // not set
	addQueueItemTopic = "t"
	addQueueItemFile = specFile

	err := runAddQueueItem(addQueueItemCmd, nil)
	if err == nil {
		t.Error("expected error for missing --domain at DONE")
	}
}

// --- set-roots rejection tests ---

func TestSetRootsRejectsWrongPhase(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)
	s := &state.ForgeState{
		Phase: state.PhasePlanning, State: state.StateOrient, Config: state.DefaultForgeConfig(),
		Planning: &state.PlanningState{CurrentPlan: &state.ActivePlan{Name: "p", Domain: "d", File: "plan.json"}},
	}
	state.Save(sd, s)

	setRootsDomain = "test"
	err := runSetRoots(setRootsCmd, []string{"test/"})
	if err == nil {
		t.Error("expected error for wrong phase")
	}
}

func TestSetRootsRejectsWrongState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDraft)
	setupSpecifyingState(t, dir, forgeState)

	setRootsDomain = "test"
	err := runSetRoots(setRootsCmd, []string{"test/"})
	if err == nil {
		t.Error("expected error for wrong state")
	}
}

func TestSetRootsRejectsNoPaths(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	setupSpecifyingState(t, dir, forgeState)

	setRootsDomain = "test"
	err := runSetRoots(setRootsCmd, []string{}) // no paths
	if err == nil {
		t.Error("expected error for no path arguments")
	}
}

func TestSetRootsRejectsDomainWithNoCompletedSpecs(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	// Override current domain to one that has no completed specs.
	forgeState.Specifying.CurrentDomain = "unknown-domain"
	setupSpecifyingState(t, dir, forgeState)

	setRootsDomain = ""
	err := runSetRoots(setRootsCmd, []string{"unknown-domain/"})
	if err == nil {
		t.Error("expected error for domain with no completed specs")
	}
}

func TestStatusCommand(t *testing.T) {
	dir := setupProjectDir(t)

	// Save state to the resolved state dir.
	sd := resolvedStateDir(dir)
	if err := os.MkdirAll(sd, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := state.DefaultForgeConfig()
	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateOrient,
		Config:         cfg,
		StartedAtPhase: state.PhaseSpecifying,
		Specifying:     state.NewSpecifyingState([]state.SpecQueueEntry{}),
	}
	state.Save(sd, s)

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

// TestStatusWithoutVerboseOmitsQueueAndCompletedSections verifies that status
// without --verbose does not include queue or completed sections.
func TestStatusWithoutVerboseOmitsQueueAndCompletedSections(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateOrient,
		StartedAtPhase: state.PhaseSpecifying,
		Config: state.ForgeConfig{
			Specifying: state.SpecifyingConfig{Batch: 1, Eval: state.EvalConfig{MinRounds: 1, MaxRounds: 3}},
		},
		Specifying: &state.SpecifyingState{
			Queue: []state.SpecQueueEntry{
				{Name: "Spec A", Domain: "test", Topic: "topic", File: "spec-a.md"},
			},
			Completed: []state.CompletedSpec{
				{ID: 1, Name: "spec-x.md", Domain: "test", RoundsTaken: 1},
			},
		},
	}
	state.Save(sd, s)

	statusVerbose = false
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runStatus(statusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "--- Queue ---") {
		t.Errorf("non-verbose status should not contain '--- Queue ---', got:\n%s", output)
	}
	if strings.Contains(output, "--- Completed ---") {
		t.Errorf("non-verbose status should not contain '--- Completed ---', got:\n%s", output)
	}
}

// TestStatusVerboseSpecifyingShowsCompletedWithEvalHistory verifies that
// status --verbose in specifying phase shows completed specs with eval history.
func TestStatusVerboseSpecifyingShowsCompletedWithEvalHistory(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateOrient,
		StartedAtPhase: state.PhaseSpecifying,
		Config: state.ForgeConfig{
			Specifying: state.SpecifyingConfig{Batch: 1, Eval: state.EvalConfig{MinRounds: 1, MaxRounds: 3}},
		},
		Specifying: &state.SpecifyingState{
			Completed: []state.CompletedSpec{
				{
					ID:           1,
					Name:         "repository-loading.md",
					Domain:       "optimizer",
					RoundsTaken:  2,
					CommitHashes: []string{"abc1234"},
					Evals: []state.EvalRecord{
						{Round: 1, Verdict: "FAIL"},
						{Round: 2, Verdict: "PASS"},
					},
				},
			},
		},
	}
	state.Save(sd, s)

	statusVerbose = true
	defer func() { statusVerbose = false }()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runStatus(statusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Completed ---") {
		t.Errorf("verbose status should contain '--- Completed ---', got:\n%s", output)
	}
	if !strings.Contains(output, "repository-loading.md") {
		t.Errorf("verbose status should show spec name, got:\n%s", output)
	}
	if !strings.Contains(output, "Round 1: FAIL") {
		t.Errorf("verbose status should show eval history, got:\n%s", output)
	}
	if !strings.Contains(output, "Round 2: PASS") {
		t.Errorf("verbose status should show eval history, got:\n%s", output)
	}
}

// TestStatusVerboseImplementingShowsPerItemDetail verifies that status -v
// in implementing phase shows per-item passes/rounds detail.
func TestStatusVerboseImplementingShowsPerItemDetail(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	planPath := filepath.Join(dir, "impl", "plan.json")
	os.MkdirAll(filepath.Dir(planPath), 0755)
	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "test", Module: "test"},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Foundation", Items: []string{"item.a"}}},
		Items: []state.PlanItem{
			{ID: "item.a", Name: "Item A", Description: "desc", Passes: "passed", Rounds: 2},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)

	s := &state.ForgeState{
		Phase: state.PhaseImplementing,
		State: state.StateOrient,
		Config: state.ForgeConfig{
			Implementing: state.ImplementingConfig{
				Batch: 1,
				Eval:  state.EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &state.PlanningState{
			CurrentPlan: &state.ActivePlan{ID: 1, Name: "Test Plan", Domain: "test", File: "impl/plan.json"},
			Evals: []state.EvalRecord{
				{Round: 1, Verdict: "PASS"},
			},
		},
		Implementing: state.NewImplementingState(),
	}
	state.Save(sd, s)

	statusVerbose = true
	defer func() { statusVerbose = false }()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runStatus(statusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- Implementing ---") {
		t.Errorf("verbose status should contain '--- Implementing ---', got:\n%s", output)
	}
	if !strings.Contains(output, "item.a") {
		t.Errorf("verbose status should show item ID, got:\n%s", output)
	}
	if !strings.Contains(output, "passed") {
		t.Errorf("verbose status should show item passes status, got:\n%s", output)
	}
	if !strings.Contains(output, "2 rounds") {
		t.Errorf("verbose status should show item rounds count, got:\n%s", output)
	}
}

// --- eval command tests ---

// TestEvalCommandReconcileEvalOutputsReconciliationContext verifies that eval in
// specifying RECONCILE_EVAL state outputs reconciliation context.
func TestEvalCommandReconcileEvalOutputsReconciliationContext(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	s := &state.ForgeState{
		Phase: state.PhaseSpecifying,
		State: state.StateReconcileEval,
		Config: state.ForgeConfig{
			Specifying: state.SpecifyingConfig{
				Reconciliation: state.ReconciliationConfig{MinRounds: 1, MaxRounds: 2},
			},
		},
		Specifying: &state.SpecifyingState{
			Reconcile: &state.ReconcileState{Round: 1},
			Completed: []state.CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "test", File: "test/specs/spec-a.md", RoundsTaken: 1},
			},
		},
	}
	state.Save(sd, s)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runEval(evalCmd, nil); err != nil {
		t.Fatalf("eval: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "RECONCILIATION EVALUATION") {
		t.Errorf("expected reconciliation context output, got:\n%s", output)
	}
	if !strings.Contains(output, "--- RECONCILIATION CONTEXT ---") {
		t.Errorf("expected reconciliation context section, got:\n%s", output)
	}
}

// TestEvalCommandCrossRefEvalOutputsCrossReferenceContext verifies that eval in
// specifying CROSS_REFERENCE_EVAL state outputs cross-reference context.
func TestEvalCommandCrossRefEvalOutputsCrossReferenceContext(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	s := &state.ForgeState{
		Phase: state.PhaseSpecifying,
		State: state.StateCrossReferenceEval,
		Config: state.ForgeConfig{
			Specifying: state.SpecifyingConfig{
				CrossReference: state.CrossRefConfig{MinRounds: 1, MaxRounds: 2},
			},
		},
		Specifying: &state.SpecifyingState{
			CurrentDomain: "test",
			CrossReference: map[string]*state.CrossReferenceState{
				"test": {Domain: "test", Round: 1},
			},
			Completed: []state.CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "test", File: "test/specs/spec-a.md", RoundsTaken: 1},
			},
		},
	}
	state.Save(sd, s)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runEval(evalCmd, nil); err != nil {
		t.Fatalf("eval: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "CROSS-REFERENCE EVALUATION") {
		t.Errorf("expected cross-reference context output, got:\n%s", output)
	}
	if !strings.Contains(output, "--- DOMAIN ---") {
		t.Errorf("expected domain section in cross-reference eval output, got:\n%s", output)
	}
}

// TestEvalCommandInDraftReturnsErrorNamingState verifies that eval in specifying
// DRAFT state returns an error that names the current state.
func TestEvalCommandInDraftReturnsErrorNamingState(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	s := &state.ForgeState{
		Phase: state.PhaseSpecifying,
		State: state.StateDraft,
		Config: state.ForgeConfig{
			Specifying: state.SpecifyingConfig{
				Eval: state.EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Specifying: state.NewSpecifyingState([]state.SpecQueueEntry{
			{Name: "Spec A", Domain: "test", Topic: "t", File: "spec-a.md"},
		}),
	}
	state.Save(sd, s)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runEval(evalCmd, nil)
	if err == nil {
		t.Fatal("expected error when calling eval in DRAFT state")
	}
	if !strings.Contains(err.Error(), "DRAFT") {
		t.Errorf("expected error to name current state 'DRAFT', got: %v", err)
	}
}

// --- validate command tests ---

func writeValidSpecQueueFile(t *testing.T, dir string) string {
	t.Helper()
	input := state.SpecQueueInput{
		Specs: []state.SpecQueueEntry{
			{Name: "Spec A", Domain: "test", Topic: "topic", File: "specs/a.md", PlanningSources: []string{}, DependsOn: []string{}},
		},
	}
	data, _ := json.Marshal(input)
	path := filepath.Join(dir, "spec-queue.json")
	os.WriteFile(path, data, 0644)
	return path
}

func writeValidPlanQueueFile(t *testing.T, dir string) string {
	t.Helper()
	input := state.PlanQueueInput{
		Plans: []state.PlanQueueEntry{
			{Name: "Test Plan", Domain: "test", File: "test/plan.json"},
		},
	}
	data, _ := json.Marshal(input)
	path := filepath.Join(dir, "plan-queue.json")
	os.WriteFile(path, data, 0644)
	return path
}

func writeValidPlanFileForValidate(t *testing.T, dir string) string {
	t.Helper()
	notesDir := filepath.Join(dir, "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "notes.md"), []byte("notes"), 0644)

	plan := map[string]interface{}{
		"context": map[string]interface{}{"domain": "test", "module": "mod"},
		"layers":  []interface{}{map[string]interface{}{"id": "L0", "name": "Base", "items": []string{"item.a"}}},
		"items": []interface{}{map[string]interface{}{
			"id": "item.a", "name": "Item A", "description": "desc",
			"depends_on": []string{}, "passes": "pending", "rounds": 0,
			"refs": []string{"notes/notes.md"},
			"tests": []interface{}{map[string]interface{}{"category": "functional", "description": "works"}},
		}},
	}
	data, _ := json.Marshal(plan)
	path := filepath.Join(dir, "plan.json")
	os.WriteFile(path, data, 0644)
	return path
}

func TestValidateSpecQueueValid(t *testing.T) {
	dir := t.TempDir()
	file := writeValidSpecQueueFile(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""
	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Detected: spec-queue") {
		t.Errorf("expected 'Detected: spec-queue' in output, got: %s", out)
	}
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors' in output, got: %s", out)
	}
}

func TestValidatePlanQueueValid(t *testing.T) {
	dir := t.TempDir()
	file := writeValidPlanQueueFile(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""
	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Detected: plan-queue") {
		t.Errorf("expected 'Detected: plan-queue' in output, got: %s", out)
	}
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors' in output, got: %s", out)
	}
}

func TestValidatePlanValid(t *testing.T) {
	dir := t.TempDir()
	file := writeValidPlanFileForValidate(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""
	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Detected: plan") {
		t.Errorf("expected 'Detected: plan' in output, got: %s", out)
	}
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors' in output, got: %s", out)
	}
}

func TestValidateTypeOverride(t *testing.T) {
	dir := t.TempDir()
	file := writeValidSpecQueueFile(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = "spec-queue"
	defer func() { validateType = "" }()

	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors', got: %s", out)
	}
}

func TestValidateSpecQueueMissingRequiredField(t *testing.T) {
	// Spec queue entry missing 'file' field — validate via state package directly.
	data := []byte(`{"specs":[{"name":"X","domain":"d","topic":"t"}]}`)
	errs := state.ValidateSpecQueue(data)
	if len(errs) == 0 {
		t.Error("expected validation errors for missing 'file' field")
	}
}

func TestValidatePlanMissingItems(t *testing.T) {
	dir := t.TempDir()
	// Plan layer references non-existent item.
	plan := map[string]interface{}{
		"context": map[string]interface{}{"domain": "test", "module": "mod"},
		"layers":  []interface{}{map[string]interface{}{"id": "L0", "name": "Base", "items": []string{"missing.item"}}},
		"items":   []interface{}{},
	}
	data, _ := json.Marshal(plan)
	errs := state.ValidatePlanJSON(data, dir)
	if len(errs) == 0 {
		t.Error("expected validation error for layer referencing non-existent item")
	}
}

func TestValidateUnknownTypeFlag(t *testing.T) {
	dir := t.TempDir()
	file := writeValidSpecQueueFile(t, dir)

	validateType = "unknown-type"
	defer func() { validateType = "" }()

	err := runValidate(validateCmd, []string{file})
	if err == nil {
		t.Error("expected error for unknown --type flag")
	}
	if !strings.Contains(err.Error(), "--type must be") {
		t.Errorf("expected '--type must be' in error, got: %v", err)
	}
}

func TestValidateUndetectableJSON(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"foo":"bar"}`)
	path := filepath.Join(dir, "weird.json")
	os.WriteFile(path, data, 0644)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""

	err := runValidate(validateCmd, []string{path})
	if err == nil {
		t.Fatal("expected error for undetectable JSON type")
	}
	if !strings.Contains(err.Error(), "cannot detect file type") {
		t.Errorf("expected 'cannot detect file type' in error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cannot detect file type") {
		t.Errorf("expected 'cannot detect file type' in output, got: %s", out)
	}
	if !strings.Contains(out, "Hint: use --type") {
		t.Errorf("expected hint about --type, got: %s", out)
	}
}

func TestValidateNonexistentFile(t *testing.T) {
	validateType = ""
	err := runValidate(validateCmd, []string{"/nonexistent/path.json"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestValidatePlanAutoDetect(t *testing.T) {
	dir := t.TempDir()
	file := writeValidPlanFileForValidate(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""

	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Detected: plan") {
		t.Errorf("expected 'Detected: plan' in auto-detect output, got: %s", out)
	}
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors', got: %s", out)
	}
}

func TestValidatePlanQueueAutoDetect(t *testing.T) {
	dir := t.TempDir()
	file := writeValidPlanQueueFile(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""

	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Detected: plan-queue") {
		t.Errorf("expected 'Detected: plan-queue' in auto-detect output, got: %s", out)
	}
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors', got: %s", out)
	}
}

func TestValidateInvalidSpecQueueShowsFailOutput(t *testing.T) {
	dir := t.TempDir()
	// Spec queue entry missing required 'file' field.
	data := []byte(`{"specs":[{"name":"X","domain":"d","topic":"t"}]}`)
	path := filepath.Join(dir, "bad-queue.json")
	os.WriteFile(path, data, 0644)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""

	err := runValidate(validateCmd, []string{path})
	if err == nil {
		t.Fatal("expected error for invalid spec queue")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected 'validation failed' in error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Error: validation failed with") {
		t.Errorf("expected 'Error: validation failed with' in output, got: %s", out)
	}
}

func TestValidateTypeOverrideConflictFails(t *testing.T) {
	dir := t.TempDir()
	// Write a spec-queue file but try to validate it as a plan.
	file := writeValidSpecQueueFile(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = "plan"
	defer func() { validateType = "" }()

	err := runValidate(validateCmd, []string{file})
	if err == nil {
		t.Fatal("expected error for type/key mismatch")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("expected 'type mismatch' in error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "--type plan expects") {
		t.Errorf("expected '--type plan expects' in output, got: %s", out)
	}
}

func TestValidateTypePlanExplicit(t *testing.T) {
	dir := t.TempDir()
	file := writeValidPlanFileForValidate(t, dir)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = "plan"
	defer func() { validateType = "" }()

	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Validated:") || !strings.Contains(out, "no errors") {
		t.Errorf("expected 'Validated:' with 'no errors' with explicit --type plan, got: %s", out)
	}
}

func TestValidateEmptyObjectFailsAutoDetect(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{}`)
	path := filepath.Join(dir, "empty.json")
	os.WriteFile(path, data, 0644)

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = ""

	err := runValidate(validateCmd, []string{path})
	if err == nil {
		t.Fatal("expected error for empty {} object")
	}
	if !strings.Contains(err.Error(), "cannot detect file type") {
		t.Errorf("expected 'cannot detect file type' in error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "cannot detect file type") {
		t.Errorf("expected 'cannot detect file type' in output, got: %s", out)
	}
}
