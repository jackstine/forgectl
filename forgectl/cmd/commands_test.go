package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

// resolvedStateDir returns the expected state dir for a project root using DefaultConfig.
func resolvedStateDir(projectRoot string) string {
	cfg := state.DefaultConfig()
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
	cfg := state.DefaultConfig()
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
		Phase: state.PhasePlanning, State: state.StateOrient, Config: state.DefaultConfig(),
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
		Phase: state.PhasePlanning, State: state.StateOrient, Config: state.DefaultConfig(),
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
	cfg := state.DefaultConfig()
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
