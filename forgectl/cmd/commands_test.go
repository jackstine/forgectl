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

// TestStatusWithoutVerboseOmitsQueueAndCompletedSections verifies that status
// without --verbose does not include queue or completed sections.
func TestStatusWithoutVerboseOmitsQueueAndCompletedSections(t *testing.T) {
	dir := t.TempDir()
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
	state.Save(dir, s)

	stateDir = dir
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
	dir := t.TempDir()
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
					ID:          1,
					Name:        "repository-loading.md",
					Domain:      "optimizer",
					RoundsTaken: 2,
					CommitHash:  "abc1234",
					Evals: []state.EvalRecord{
						{Round: 1, Verdict: "FAIL"},
						{Round: 2, Verdict: "PASS"},
					},
				},
			},
		},
	}
	state.Save(dir, s)

	stateDir = dir
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
	dir := t.TempDir()

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
	state.Save(dir, s)

	stateDir = dir
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
	dir := t.TempDir()
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
	state.Save(dir, s)

	stateDir = dir
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
	dir := t.TempDir()
	s := &state.ForgeState{
		Phase: state.PhaseSpecifying,
		State: state.StateCrossReferenceEval,
		Config: state.ForgeConfig{
			Specifying: state.SpecifyingConfig{
				CrossReference: state.CrossRefConfig{MinRounds: 1, MaxRounds: 2},
			},
		},
		Specifying: &state.SpecifyingState{
			CrossReference: &state.CrossReferenceState{Domain: "test", Round: 1},
			Completed: []state.CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "test", File: "test/specs/spec-a.md", RoundsTaken: 1},
			},
		},
	}
	state.Save(dir, s)

	stateDir = dir
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
	dir := t.TempDir()
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
	state.Save(dir, s)

	stateDir = dir
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
	if !strings.Contains(out, "valid spec-queue") {
		t.Errorf("expected 'valid spec-queue' in output, got: %s", out)
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
	if !strings.Contains(out, "valid plan-queue") {
		t.Errorf("expected 'valid plan-queue' in output, got: %s", out)
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
	if !strings.Contains(out, "valid plan") {
		t.Errorf("expected 'valid plan' in output, got: %s", out)
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
	if !strings.Contains(out, "valid spec-queue") {
		t.Errorf("expected 'valid spec-queue', got: %s", out)
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
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected 'unknown type' in error, got: %v", err)
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
		t.Error("expected error for undetectable JSON type")
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
	if !strings.Contains(out, "valid plan") {
		t.Errorf("expected 'valid plan' in auto-detect output, got: %s", out)
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
	if !strings.Contains(out, "valid plan-queue") {
		t.Errorf("expected 'valid plan-queue' in auto-detect output, got: %s", out)
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

	// Override osExit so the test process doesn't actually exit.
	exited := false
	origExit := osExit
	osExit = func(code int) { exited = true }
	defer func() { osExit = origExit }()

	err := runValidate(validateCmd, []string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exited {
		t.Error("expected osExit(1) to be called")
	}
	out := buf.String()
	if !strings.Contains(out, "FAIL:") {
		t.Errorf("expected 'FAIL:' in output, got: %s", out)
	}
	if !strings.Contains(out, "bad-queue.json") {
		t.Errorf("expected filename in FAIL output, got: %s", out)
	}
}

func TestValidateTypeOverrideConflictFails(t *testing.T) {
	dir := t.TempDir()
	// Write a spec-queue file but try to validate it as a plan.
	file := writeValidSpecQueueFile(t, dir)

	exited := false
	origExit := osExit
	osExit = func(code int) { exited = true }
	defer func() { osExit = origExit }()

	var buf bytes.Buffer
	validateCmd.SetOut(&buf)
	validateType = "plan"
	defer func() { validateType = "" }()

	err := runValidate(validateCmd, []string{file})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A spec-queue file parsed as plan.json should fail validation.
	if !exited {
		t.Error("expected validation failure (osExit) when --type plan is used on a spec-queue file")
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
	if !strings.Contains(out, "valid plan") {
		t.Errorf("expected 'valid plan' with explicit --type plan, got: %s", out)
	}
}

func TestValidateEmptyObjectFailsAutoDetect(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{}`)
	path := filepath.Join(dir, "empty.json")
	os.WriteFile(path, data, 0644)

	validateType = ""
	err := runValidate(validateCmd, []string{path})
	if err == nil {
		t.Error("expected error for empty {} object (cannot determine file type)")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot determine file type") && !strings.Contains(err.Error(), "cannot detect file type") {
		t.Errorf("expected 'cannot determine file type' in error, got: %v", err)
	}
}

// --- Logging integration tests ---

// Test 7: Init command writes log entry with correct cmd/phase/state/detail.
func TestInitLogDetail(t *testing.T) {
	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateOrient,
		StartedAtPhase: state.PhaseSpecifying,
		Config: state.ForgeConfig{
			General:    state.GeneralConfig{UserGuided: true},
			Specifying: state.SpecifyingConfig{Batch: 2, Eval: state.EvalConfig{MinRounds: 1, MaxRounds: 3}},
			Logs:       state.LogsConfig{Enabled: true, RetentionDays: 90, MaxFiles: 50},
		},
	}
	detail := buildInitLogDetail(s)

	if detail["guided"] != true {
		t.Errorf("detail[guided] = %v, want true", detail["guided"])
	}
	if detail["batch"] != 2 {
		t.Errorf("detail[batch] = %v, want 2", detail["batch"])
	}
	if detail["rounds"] != "1-3" {
		t.Errorf("detail[rounds] = %v, want 1-3", detail["rounds"])
	}
}

// Test 8: Advance command writes log entry with prev_state set correctly.
func TestAdvanceLogEntryPrevState(t *testing.T) {
	s := &state.ForgeState{
		Phase: state.PhaseSpecifying,
		State: state.StateSelect, // after advance from ORIENT
		Config: state.ForgeConfig{
			General: state.GeneralConfig{EnableEvalOutput: false},
		},
		Specifying: &state.SpecifyingState{},
	}
	prevSnap := advanceSnapshot{}
	in := state.AdvanceInput{}

	detail := buildAdvanceLogDetail(s, "ORIENT", state.PhaseSpecifying, prevSnap, in)
	// ORIENT in specifying has no specific detail case, so detail should be nil.
	if detail != nil {
		t.Errorf("expected nil detail for ORIENT advance, got %v", detail)
	}
}

// Test 9: Advance from implementing ORIENT logs layer/unblocked/remaining.
func TestAdvanceLogDetailImplOrient(t *testing.T) {
	s := &state.ForgeState{
		Phase: state.PhaseImplementing,
		State: state.StateImplement, // after ORIENT → IMPLEMENT
		Config: state.ForgeConfig{
			General: state.GeneralConfig{EnableEvalOutput: false},
		},
		Implementing: &state.ImplementingState{
			CurrentLayer: &state.LayerRef{ID: "L0", Name: "Foundation"},
			CurrentBatch: &state.BatchState{
				Items: []string{"item.a", "item.b"},
			},
		},
	}
	prevSnap := advanceSnapshot{} // before ORIENT, no batch yet
	in := state.AdvanceInput{}

	detail := buildAdvanceLogDetail(s, "ORIENT", state.PhaseImplementing, prevSnap, in)
	if detail == nil {
		t.Fatal("expected detail for ORIENT advance in implementing")
	}
	if detail["layer"] != "L0" {
		t.Errorf("detail[layer] = %v, want L0", detail["layer"])
	}
	if detail["unblocked"] != 2 {
		t.Errorf("detail[unblocked] = %v, want 2", detail["unblocked"])
	}
}

// Test 10: Advance from implementing COMMIT logs layer/batch/items.
func TestAdvanceLogDetailImplCommit(t *testing.T) {
	s := &state.ForgeState{
		Phase: state.PhaseImplementing,
		State: state.StateOrient, // after COMMIT → ORIENT
		Config: state.ForgeConfig{
			General: state.GeneralConfig{EnableEvalOutput: false},
		},
		Implementing: &state.ImplementingState{},
	}
	prevSnap := advanceSnapshot{
		layerID:     "L0",
		batchNumber: 2,
		batchItems:  []string{"item.a", "item.b"},
	}
	in := state.AdvanceInput{}

	detail := buildAdvanceLogDetail(s, "COMMIT", state.PhaseImplementing, prevSnap, in)
	if detail == nil {
		t.Fatal("expected detail for COMMIT advance in implementing")
	}
	if detail["layer"] != "L0" {
		t.Errorf("detail[layer] = %v, want L0", detail["layer"])
	}
	if detail["batch"] != 2 {
		t.Errorf("detail[batch] = %v, want 2", detail["batch"])
	}
	items, ok := detail["items"].([]string)
	if !ok {
		t.Fatalf("detail[items] type = %T, want []string", detail["items"])
	}
	if len(items) != 2 || items[0] != "item.a" {
		t.Errorf("detail[items] = %v, want [item.a item.b]", items)
	}
}

// Test 11: Advance from specifying EVALUATE logs round/verdict with/without eval_report.
func TestAdvanceLogDetailSpecifyingEvaluate(t *testing.T) {
	// Without eval_report (EnableEvalOutput = false).
	s := &state.ForgeState{
		Phase: state.PhaseSpecifying,
		State: state.StateAccept,
		Config: state.ForgeConfig{
			General: state.GeneralConfig{EnableEvalOutput: false},
		},
		Specifying: &state.SpecifyingState{},
	}
	prevSnap := advanceSnapshot{specRound: 2}
	in := state.AdvanceInput{Verdict: "PASS", EvalReport: "report.md"}

	detail := buildAdvanceLogDetail(s, "EVALUATE", state.PhaseSpecifying, prevSnap, in)
	if detail == nil {
		t.Fatal("expected detail for specifying EVALUATE advance")
	}
	if detail["round"] != 2 {
		t.Errorf("detail[round] = %v, want 2", detail["round"])
	}
	if detail["verdict"] != "PASS" {
		t.Errorf("detail[verdict] = %v, want PASS", detail["verdict"])
	}
	if _, ok := detail["eval_report"]; ok {
		t.Error("detail[eval_report] should not be present when EnableEvalOutput=false")
	}

	// With eval_report (EnableEvalOutput = true).
	s.Config.General.EnableEvalOutput = true
	detail2 := buildAdvanceLogDetail(s, "EVALUATE", state.PhaseSpecifying, prevSnap, in)
	if detail2["eval_report"] != "report.md" {
		t.Errorf("detail[eval_report] = %v, want report.md", detail2["eval_report"])
	}
}
