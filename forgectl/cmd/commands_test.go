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

// TestInitScaffoldsConfigWhenAbsent verifies the session-init criterion "Init
// creates .forgectl and default config when none exists": configuration
// scaffolding runs first, bootstraps a bare project, prints the fresh-default
// notice, and init proceeds against the default config.
func TestInitScaffoldsConfigWhenAbsent(t *testing.T) {
	// A bare working directory with no .forgectl/ in it or any ancestor.
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	input := state.SpecQueueInput{
		Specs: []state.SpecQueueEntry{
			{Name: "Spec A", Domain: "test", Topic: "topic A", File: "specs/a.md", PlanningSources: []string{}, DependsOn: []string{}},
		},
	}
	data, _ := json.Marshal(input)
	queueFile := filepath.Join(dir, "specs-queue.json")
	os.WriteFile(queueFile, data, 0644)

	initFrom = queueFile
	initPhase = "specifying"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("init on bare project: %v", err)
	}

	// .forgectl/ and a default config were created.
	if info, err := os.Stat(filepath.Join(dir, ".forgectl")); err != nil || !info.IsDir() {
		t.Fatalf(".forgectl/ not created: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".forgectl", "config"))
	if err != nil {
		t.Fatalf("reading scaffolded config: %v", err)
	}
	if string(got) != state.DefaultConfigTemplate() {
		t.Error("scaffolded config does not equal the embedded default template")
	}

	// The fresh-default notice was printed.
	if !strings.Contains(buf.String(), "Created default .forgectl/config") {
		t.Errorf("expected fresh-default notice, got output: %q", buf.String())
	}

	// Init proceeded: a state file exists at the resolved state dir.
	if _, err := state.Load(resolvedStateDir(dir)); err != nil {
		t.Fatalf("state not created after scaffolded init: %v", err)
	}
}

// TestBareInitCreatesScaffolding verifies the spec criterion "Bare init creates
// scaffolding when nothing exists": forgectl init with no flags bootstraps
// .forgectl/ and a default config, exits with code 0, and creates no state file.
func TestBareInitCreatesScaffolding(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(orig)
		initFrom = ""
		initPhase = "specifying"
	})

	initFrom = ""
	initPhase = "specifying"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("bare init: %v", err)
	}

	// .forgectl/ and config were created.
	if info, err := os.Stat(filepath.Join(dir, ".forgectl")); err != nil || !info.IsDir() {
		t.Fatalf(".forgectl/ not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".forgectl", "config")); err != nil {
		t.Fatalf(".forgectl/config not created: %v", err)
	}

	// No state file was created.
	if _, err := state.Load(resolvedStateDir(dir)); err == nil {
		t.Error("state file should not be created by bare init")
	}
}

// TestBareInitIsIdempotent verifies the spec criterion "Bare init is idempotent
// on an existing project": when .forgectl/ and config already exist, no file is
// written and no notice is printed.
func TestBareInitIsIdempotent(t *testing.T) {
	dir := setupProjectDir(t)
	t.Cleanup(func() {
		initFrom = ""
		initPhase = "specifying"
	})

	// Record config mtime before bare init.
	configPath := filepath.Join(dir, ".forgectl", "config")
	before, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}

	initFrom = ""
	initPhase = "specifying"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("bare init on existing project: %v", err)
	}

	// No notice printed.
	if buf.Len() != 0 {
		t.Errorf("expected no output on idempotent bare init, got: %q", buf.String())
	}

	// No state file created.
	if _, err := state.Load(resolvedStateDir(dir)); err == nil {
		t.Error("state file should not be created by bare init")
	}

	// Config file not rewritten (mtime unchanged).
	after, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("config file was rewritten during idempotent bare init")
	}
}

// TestPhaseWithoutFromIsRejected verifies the spec criterion "--phase without
// --from is rejected": forgectl init --phase specifying (no --from) exits with
// code 1 and error "--from is required when --phase is set."
func TestPhaseWithoutFromIsRejected(t *testing.T) {
	setupProjectDir(t)
	t.Cleanup(func() {
		initCmd.Flags().Lookup("phase").Changed = false
		initFrom = ""
		initPhase = "specifying"
	})

	// Simulate explicit --phase flag via cobra so Changed("phase") returns true.
	if err := initCmd.Flags().Set("phase", "specifying"); err != nil {
		t.Fatal(err)
	}
	initFrom = ""

	err := runInit(initCmd, nil)
	if err == nil {
		t.Fatal("expected error when --phase is set without --from")
	}
	if err.Error() != "--from is required when --phase is set." {
		t.Errorf("unexpected error: %v", err)
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

// uiImplementingConfig is a TOML config with all four required ui_implementing
// keys populated. Tests override individual sections to exercise rejection.
const uiImplementingConfig = `[ui_implementing.app]
launch_command = "npm run dev"
url = "http://localhost:3000"

[ui_implementing.e2e]
test_command = "npx playwright test"
test_dir = "e2e"
`

// writeUIPlan writes a minimal valid plan.json (no refs to validate on disk) and
// returns its path. Items carry pre-existing passes/rounds to confirm init resets them.
func writeUIPlan(t *testing.T, dir string) string {
	t.Helper()
	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "ui", Module: "ui-mod"},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Foundation", Items: []string{"item.1"}}},
		Items: []state.PlanItem{
			{
				ID:          "item.1",
				Name:        "First Item",
				Description: "Renders the thing",
				DependsOn:   []string{},
				Refs:        []string{},
				Passes:      "passed", // should be reset to "pending"
				Rounds:      4,        // should be reset to 0
				Tests:       []state.PlanTest{{Category: "functional", Description: "it works"}},
			},
		},
	}
	data, _ := json.Marshal(plan)
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	return planPath
}

// Functional: init at ui_implementing with all required UI config keys present
// builds the ui_implementing state at ORIENT and resets plan item passes/rounds.
func TestInitUIImplementing(t *testing.T) {
	dir := setupProjectDir(t)
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(uiImplementingConfig), 0644)
	planPath := writeUIPlan(t, dir)

	initFrom = planPath
	initPhase = "ui_implementing"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("init at ui_implementing: %v", err)
	}

	sd := resolvedStateDir(dir)
	s, err := state.Load(sd)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Phase != state.PhaseUIImplementing {
		t.Errorf("phase = %s, want ui_implementing", s.Phase)
	}
	if s.State != state.StateOrient {
		t.Errorf("state = %s, want ORIENT", s.State)
	}
	if s.StartedAtPhase != state.PhaseUIImplementing {
		t.Errorf("started_at_phase = %s, want ui_implementing", s.StartedAtPhase)
	}
	if s.UIImplementing == nil {
		t.Fatal("ui_implementing state not built")
	}

	// plan.json items reset to passes:"pending", rounds:0.
	planData, _ := os.ReadFile(planPath)
	var plan state.PlanJSON
	json.Unmarshal(planData, &plan)
	if plan.Items[0].Passes != "pending" {
		t.Errorf("item passes = %q, want pending", plan.Items[0].Passes)
	}
	if plan.Items[0].Rounds != 0 {
		t.Errorf("item rounds = %d, want 0", plan.Items[0].Rounds)
	}
}

// Rejection: init at ui_implementing fails when any required UI config key is
// empty, naming the missing key. Exercised once per key.
func TestInitUIImplementingRejectsMissingConfig(t *testing.T) {
	cases := []struct {
		name    string
		config  string
		wantKey string
	}{
		{
			name: "missing launch_command",
			config: `[ui_implementing.app]
url = "http://localhost:3000"
[ui_implementing.e2e]
test_command = "npx playwright test"
test_dir = "e2e"
`,
			wantKey: "ui_implementing.app.launch_command",
		},
		{
			name: "missing url",
			config: `[ui_implementing.app]
launch_command = "npm run dev"
[ui_implementing.e2e]
test_command = "npx playwright test"
test_dir = "e2e"
`,
			wantKey: "ui_implementing.app.url",
		},
		{
			name: "missing test_command",
			config: `[ui_implementing.app]
launch_command = "npm run dev"
url = "http://localhost:3000"
[ui_implementing.e2e]
test_dir = "e2e"
`,
			wantKey: "ui_implementing.e2e.test_command",
		},
		{
			name: "missing test_dir",
			config: `[ui_implementing.app]
launch_command = "npm run dev"
url = "http://localhost:3000"
[ui_implementing.e2e]
test_command = "npx playwright test"
`,
			wantKey: "ui_implementing.e2e.test_dir",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupProjectDir(t)
			os.WriteFile(filepath.Join(dir, ".forgectl", "config"), []byte(tc.config), 0644)
			planPath := writeUIPlan(t, dir)

			initFrom = planPath
			initPhase = "ui_implementing"

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)

			err := runInit(initCmd, nil)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(buf.String(), tc.wantKey) {
				t.Errorf("output should name missing key %q, got:\n%s", tc.wantKey, buf.String())
			}
			// No state file should be written on rejection.
			if state.Exists(resolvedStateDir(dir)) {
				t.Error("state file should not exist after rejection")
			}
		})
	}
}

// Rejection: a plan queue entry with a kind other than code/ui is rejected at
// planning init, naming the offending entry and value.
func TestInitRejectsInvalidPlanQueueKind(t *testing.T) {
	dir := setupProjectDir(t)

	queueFile := filepath.Join(dir, "plans-queue.json")
	os.WriteFile(queueFile, []byte(`{"plans":[{"name":"Bad Plan","domain":"ui","kind":"frontend","file":"ui/plan.json","specs":[],"spec_commits":[],"code_search_roots":[]}]}`), 0644)

	initFrom = queueFile
	initPhase = "planning"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runInit(initCmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid plan queue kind")
	}
	if !strings.Contains(buf.String(), "frontend") {
		t.Errorf("output should name the offending kind value, got:\n%s", buf.String())
	}
}

// saveUISession builds a ui_implementing session in the given state with a
// minimal valid plan on disk and saves the state file. Returns the project dir.
func saveUISession(t *testing.T, st state.StateName) string {
	t.Helper()
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)

	planDir := filepath.Join(dir, "ui")
	os.MkdirAll(planDir, 0755)
	plan := state.PlanJSON{
		Context: state.PlanContext{Domain: "portal", Module: "portal"},
		Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Shell", Items: []string{"shell.layout"}}},
		Items: []state.PlanItem{
			{ID: "shell.layout", Name: "App shell", Description: "shell", Passes: "done", Tests: []state.PlanTest{{Category: "functional", Description: "x"}}},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(filepath.Join(planDir, "plan.json"), data, 0644)

	cfg := state.DefaultForgeConfig()
	cfg.UIImplementing.App.LaunchCommand = "npm run dev"
	cfg.UIImplementing.App.URL = "http://localhost:5173"
	cfg.UIImplementing.E2E.TestCommand = "npm run e2e"
	cfg.UIImplementing.E2E.TestDir = "e2e/"

	s := &state.ForgeState{
		Phase:    state.PhaseUIImplementing,
		State:    st,
		Config:   cfg,
		Planning: &state.PlanningState{CurrentPlan: &state.ActivePlan{ID: 1, Name: "Portal", Domain: "portal", File: "ui/plan.json"}},
		UIImplementing: &state.UIImplementingState{
			CurrentLayer:      &state.LayerRef{ID: "L0", Name: "Shell"},
			BatchNumber:       1,
			CurrentPlanFile:   "ui/plan.json",
			CurrentPlanDomain: "portal",
			CurrentBatch:      &state.UIBatchState{Items: []string{"shell.layout"}, CurrentItemIndex: 0},
		},
	}
	if err := state.Save(sd, s); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Functional: eval in ui_implementing QA_TEST routes to the QA evaluator output.
func TestEvalUIImplementingQATest(t *testing.T) {
	saveUISession(t, state.StateQATest)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	if err := runEval(evalCmd, nil); err != nil {
		t.Fatalf("eval in QA_TEST: %v", err)
	}
	if !strings.Contains(buf.String(), "# UI QA Evaluation Prompt") {
		t.Errorf("QA_TEST eval should embed the QA prompt, got:\n%s", buf.String())
	}
}

// Functional: eval in ui_implementing E2E_VERIFY routes to the e2e evaluator output.
func TestEvalUIImplementingE2EVerify(t *testing.T) {
	saveUISession(t, state.StateE2EVerify)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	if err := runEval(evalCmd, nil); err != nil {
		t.Fatalf("eval in E2E_VERIFY: %v", err)
	}
	if !strings.Contains(buf.String(), "# UI E2E Verification Prompt") {
		t.Errorf("E2E_VERIFY eval should embed the e2e prompt, got:\n%s", buf.String())
	}
}

// Rejection: eval in a ui_implementing non-evaluator state (UI_REFINE) is rejected.
func TestEvalUIImplementingRejectsNonEvalState(t *testing.T) {
	saveUISession(t, state.StateUIRefine)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	err := runEval(evalCmd, nil)
	if err == nil {
		t.Fatal("expected error for eval in UI_REFINE")
	}
	// The rejection must name both the current state and the phase.
	if !strings.Contains(err.Error(), "UI_REFINE") || !strings.Contains(err.Error(), "ui_implementing") {
		t.Errorf("error should name the current state and phase, got: %v", err)
	}
}

// Functional: --verdict is accepted in ui_implementing QA_TEST and E2E_VERIFY.
func TestAdvanceVerdictValidInUILoops(t *testing.T) {
	for _, st := range []state.StateName{state.StateQATest, state.StateE2EVerify} {
		s := &state.ForgeState{Phase: state.PhaseUIImplementing, State: st}
		advanceVerdict = "PASS"
		err := validateAdvanceFlags(s)
		advanceVerdict = ""
		if err != nil {
			t.Errorf("--verdict should be valid in %s: %v", st, err)
		}
	}
}

// --- handoff command tests ---

// Functional: handoff registers the named artifacts on the current batch and a
// subsequent status surfaces them on a Review: line, with no verdict recorded.
func TestHandoffRegistersArtifactsForReview(t *testing.T) {
	dir := saveUISession(t, state.StateQATest)
	qaReport := filepath.Join(dir, "ui", "qa", "batch-1-round-1.md")
	stepList := filepath.Join(dir, "ui", "qa", "batch-1-steps.json")
	os.MkdirAll(filepath.Dir(qaReport), 0755)
	os.WriteFile(qaReport, []byte("# QA report"), 0644)
	os.WriteFile(stepList, []byte(`{"scenarios":[]}`), 0644)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	if err := runHandoff(handoffCmd, []string{qaReport, stepList}); err != nil {
		t.Fatalf("handoff: %v", err)
	}

	// Artifacts recorded on the current batch (latest hand-off wins).
	sd := resolvedStateDir(dir)
	s, _ := state.Load(sd)
	got := s.UIImplementing.CurrentBatch.HandedOffArtifacts
	if len(got) != 2 || got[0] != qaReport || got[1] != stepList {
		t.Errorf("handed-off artifacts = %v, want [%s %s]", got, qaReport, stepList)
	}
	// State unchanged — hand-off carries no verdict and does not transition.
	if s.State != state.StateQATest {
		t.Errorf("state = %s, want QA_TEST (no transition)", s.State)
	}

	// A subsequent status surfaces them on a Review: line.
	buf.Reset()
	if err := runStatus(statusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Review:") {
		t.Errorf("status should show a Review: line, got:\n%s", out)
	}
	if !strings.Contains(out, qaReport) || !strings.Contains(out, stepList) {
		t.Errorf("status Review: should list both artifacts, got:\n%s", out)
	}
}

// Rejection: handoff in a non-evaluator state (UI_REFINE) is rejected, naming
// the current state and phase, and registers nothing.
func TestHandoffRejectedOutsideEvaluatorState(t *testing.T) {
	dir := saveUISession(t, state.StateUIRefine)
	f := filepath.Join(dir, "ui", "some.md")
	os.MkdirAll(filepath.Dir(f), 0755)
	os.WriteFile(f, []byte("x"), 0644)

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	err := runHandoff(handoffCmd, []string{f})
	if err == nil {
		t.Fatal("expected error for handoff in UI_REFINE")
	}
	if !strings.Contains(err.Error(), "UI_REFINE") || !strings.Contains(err.Error(), "ui_implementing") {
		t.Errorf("error should name current state and phase, got: %v", err)
	}
	s, _ := state.Load(resolvedStateDir(dir))
	if len(s.UIImplementing.CurrentBatch.HandedOffArtifacts) != 0 {
		t.Error("no artifacts should be registered on rejection")
	}
}

// Rejection: handoff naming a file that does not exist is rejected, naming the
// path, and registers nothing.
func TestHandoffRejectsNonExistentFile(t *testing.T) {
	dir := saveUISession(t, state.StateE2EVerify)
	missing := filepath.Join(dir, "ui", "e2e", "batch-1-round-1.md")

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	err := runHandoff(handoffCmd, []string{missing})
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error should name the missing path, got: %v", err)
	}
	s, _ := state.Load(resolvedStateDir(dir))
	if len(s.UIImplementing.CurrentBatch.HandedOffArtifacts) != 0 {
		t.Error("no artifacts should be registered when a file is missing")
	}
}

// Rejection: handoff with no file arguments is rejected by arg validation.
func TestHandoffRejectsNoArgs(t *testing.T) {
	if err := handoffCmd.Args(handoffCmd, []string{}); err == nil {
		t.Error("expected error for handoff with no file arguments")
	}
}

// Functional: a valid {concept, domains} input initializes a reverse_engineering
// session — RE state built with domain index 1, count N, ORIENT, and
// colleague_review carried from the locked config.
func TestInitAcceptsReverseEngineeringPhase(t *testing.T) {
	dir := setupProjectDir(t)
	// Enable colleague_review in config so we can assert it locks into state.
	os.WriteFile(filepath.Join(dir, ".forgectl", "config"),
		[]byte("[reverse_engineering.reconcile]\ncolleague_review = true\n"), 0644)

	inputFile := filepath.Join(dir, "input.json")
	os.WriteFile(inputFile, []byte(`{"concept": "auth refactor", "domains": ["optimizer", "api", "portal"]}`), 0644)

	initFrom = inputFile
	initPhase = "reverse_engineering"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("init at reverse_engineering: %v", err)
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
	if s.StartedAtPhase != state.PhaseReverseEngineering {
		t.Errorf("started_at_phase = %s, want reverse_engineering", s.StartedAtPhase)
	}
	re := s.ReverseEngineering
	if re == nil {
		t.Fatal("reverse_engineering state not built")
	}
	if re.Concept != "auth refactor" {
		t.Errorf("concept = %q, want auth refactor", re.Concept)
	}
	if re.DomainIndex != 1 || re.DomainCount != 3 {
		t.Errorf("domain index/count = %d/%d, want 1/3", re.DomainIndex, re.DomainCount)
	}
	if !re.ColleagueReview {
		t.Error("colleague_review should be locked from config (true)")
	}
}

// Rejection: an invalid RE init input (missing concept) is rejected.
func TestInitRejectsInvalidReverseEngineeringInput(t *testing.T) {
	dir := setupProjectDir(t)
	inputFile := filepath.Join(dir, "input.json")
	os.WriteFile(inputFile, []byte(`{"domains": ["api"]}`), 0644)

	initFrom = inputFile
	initPhase = "reverse_engineering"

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := runInit(initCmd, nil)
	if err == nil {
		t.Fatal("expected error for RE input missing concept")
	}
	// No state file should have been written.
	if state.Exists(resolvedStateDir(dir)) {
		t.Error("state file should not exist after a rejected init")
	}
}

// Rejection: generate_planning_queue stays non-initializable even though
// reverse_engineering was added to the valid set.
func TestInitRejectsGeneratePlanningQueueStillNonInitializable(t *testing.T) {
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

func TestSetCommitHashesInCrossReferenceReviewState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	sd := setupSpecifyingState(t, dir, forgeState)

	setCommitHashesDomain = ""

	if err := runSetCommitHashes(setCommitHashesCmd, []string{"abc1234", "def5678"}); err != nil {
		t.Fatalf("set-commit-hashes: %v", err)
	}

	s, _ := state.Load(sd)
	if len(s.Specifying.Completed) != 1 {
		t.Fatalf("expected 1 completed spec, got %d", len(s.Specifying.Completed))
	}
	hashes := s.Specifying.Completed[0].CommitHashes
	if len(hashes) != 2 || hashes[0] != "abc1234" || hashes[1] != "def5678" {
		t.Errorf("expected hashes [abc1234 def5678], got %v", hashes)
	}
}

func TestSetCommitHashesInDoneState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDone)
	sd := setupSpecifyingState(t, dir, forgeState)

	setCommitHashesDomain = "test"

	if err := runSetCommitHashes(setCommitHashesCmd, []string{"abc1234"}); err != nil {
		t.Fatalf("set-commit-hashes: %v", err)
	}

	s, _ := state.Load(sd)
	if s.Specifying.Completed[0].CommitHashes[0] != "abc1234" {
		t.Errorf("expected hash 'abc1234', got %v", s.Specifying.Completed[0].CommitHashes)
	}
}

func TestSetCommitHashesOverwritesPreviousValue(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	forgeState.Specifying.Completed[0].CommitHashes = []string{"old1234"}
	sd := setupSpecifyingState(t, dir, forgeState)

	setCommitHashesDomain = ""

	if err := runSetCommitHashes(setCommitHashesCmd, []string{"new5678"}); err != nil {
		t.Fatalf("set-commit-hashes: %v", err)
	}

	s, _ := state.Load(sd)
	hashes := s.Specifying.Completed[0].CommitHashes
	if len(hashes) != 1 || hashes[0] != "new5678" {
		t.Errorf("expected hashes [new5678], got %v", hashes)
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

// --- set-commit-hashes rejection tests ---

func TestSetCommitHashesRejectsWrongPhase(t *testing.T) {
	dir := setupProjectDir(t)
	sd := resolvedStateDir(dir)
	os.MkdirAll(sd, 0755)
	s := &state.ForgeState{
		Phase: state.PhasePlanning, State: state.StateOrient, Config: state.DefaultForgeConfig(),
		Planning: &state.PlanningState{CurrentPlan: &state.ActivePlan{Name: "p", Domain: "d", File: "plan.json"}},
	}
	state.Save(sd, s)

	setCommitHashesDomain = "test"
	err := runSetCommitHashes(setCommitHashesCmd, []string{"abc1234"})
	if err == nil {
		t.Error("expected error for wrong phase")
	}
}

func TestSetCommitHashesRejectsWrongState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateDraft)
	setupSpecifyingState(t, dir, forgeState)

	setCommitHashesDomain = "test"
	err := runSetCommitHashes(setCommitHashesCmd, []string{"abc1234"})
	if err == nil {
		t.Error("expected error for wrong state")
	}
}

func TestSetCommitHashesRejectsNoHashes(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	setupSpecifyingState(t, dir, forgeState)

	setCommitHashesDomain = "test"
	err := runSetCommitHashes(setCommitHashesCmd, []string{}) // no hashes
	if err == nil {
		t.Error("expected error for no hash arguments")
	}
}

func TestSetCommitHashesRejectsDomainWithNoCompletedSpecs(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newSpecifyingForgeState(state.StateCrossReferenceReview)
	// Override current domain to one that has no completed specs.
	forgeState.Specifying.CurrentDomain = "unknown-domain"
	setupSpecifyingState(t, dir, forgeState)

	setCommitHashesDomain = ""
	err := runSetCommitHashes(setCommitHashesCmd, []string{"abc1234"})
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

// TestEvalCommandImplementingRendersSpecReadCommand verifies that `forgectl
// eval` in implementing EVALUATE resolves the plan from disk, loads the active
// plan's spec_commits, and emits the same bounded `git show` Read: command
// under each Specs: entry as IMPLEMENT — across all three eval_modes. This
// exercises the full CLI path (resolveSession -> state.Load -> loadPlan from
// the real plan.json on disk), not just PrintEvalOutput against an in-memory
// state.
func TestEvalCommandImplementingRendersSpecReadCommand(t *testing.T) {
	for _, mode := range []string{"report", "direct", "conversational"} {
		t.Run(mode, func(t *testing.T) {
			dir := setupProjectDir(t)
			sd := resolvedStateDir(dir)
			os.MkdirAll(sd, 0755)

			planRelPath := filepath.Join("impl", "plan.json")
			os.MkdirAll(filepath.Join(dir, "impl"), 0755)
			plan := state.PlanJSON{
				Context: state.PlanContext{Domain: "test", Module: "mod"},
				Layers:  []state.PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"x.item"}}},
				Items: []state.PlanItem{
					{
						ID:          "x.item",
						Name:        "X Item",
						Description: "desc",
						Passes:      "done",
						Specs:       []string{"spec-sqlc-schemas.md#x"},
						Tests:       []state.PlanTest{{Category: "functional", Description: "works"}},
					},
				},
			}
			data, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, planRelPath), data, 0644); err != nil {
				t.Fatal(err)
			}

			s := &state.ForgeState{
				Phase: state.PhaseImplementing,
				State: state.StateEvaluate,
				Config: state.ForgeConfig{
					Implementing: state.ImplementingConfig{
						Batch: 1,
						Eval:  state.EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: mode},
					},
				},
				Planning: &state.PlanningState{
					CurrentPlan: &state.ActivePlan{
						ID: 1, Name: "Test Plan", Domain: "test", File: planRelPath,
						SpecCommits: []string{"e742a1b", "694ca99"},
					},
				},
				Implementing: &state.ImplementingState{
					CurrentLayer:    &state.LayerRef{ID: "L0", Name: "Base"},
					BatchNumber:     1,
					CurrentPlanFile: planRelPath,
					CurrentBatch:    &state.BatchState{Items: []string{"x.item"}},
				},
			}
			if err := state.Save(sd, s); err != nil {
				t.Fatal(err)
			}

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			if err := runEval(evalCmd, nil); err != nil {
				t.Fatalf("eval: %v", err)
			}

			want := "Read:        git show e742a1b 694ca99 -- '**/spec-sqlc-schemas.md'"
			if !strings.Contains(buf.String(), want) {
				t.Errorf("mode %q: expected %q, got:\n%s", mode, want, buf.String())
			}
		})
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

// Functional: forgectl eval during reverse_engineering RECONCILE_EVAL outputs
// the reconciliation evaluator prompt populated with the current domain's spec
// list, depends_on, round, and the report path.
func TestEvalCommandReverseEngineeringReconcileEval(t *testing.T) {
	dir := setupProjectDir(t)
	re := state.NewReverseEngineeringState("auth refactor", []string{"optimizer", "api"})
	re.DomainIndex = 1
	re.ReconcileRound = 2
	re.Queue = []state.REQueueEntry{
		{Name: "Repo Loading", Domain: "optimizer", File: "specs/repo.md", Action: "create", DependsOn: []string{"Config"}},
		{Name: "Config", Domain: "optimizer", File: "specs/config.md", Action: "update", DependsOn: []string{}},
		{Name: "Handlers", Domain: "api", File: "specs/handlers.md", Action: "create", DependsOn: []string{}},
	}
	forgeState := &state.ForgeState{
		Phase:              state.PhaseReverseEngineering,
		State:              state.StateReconcileEval,
		Config:             state.DefaultForgeConfig(),
		StartedAtPhase:     state.PhaseReverseEngineering,
		ReverseEngineering: re,
	}
	setupSpecifyingState(t, dir, forgeState)

	var buf bytes.Buffer
	evalCmd.SetOut(&buf)

	if err := runEval(evalCmd, nil); err != nil {
		t.Fatalf("eval at RE RECONCILE_EVAL: %v", err)
	}
	out := buf.String()

	// Evaluator prompt + 7-dimension checklist surfaced.
	if !strings.Contains(out, "RECONCILIATION EVALUATION ROUND 2/3") {
		t.Errorf("expected round 2/3 header, got:\n%s", out)
	}
	if !strings.Contains(out, "Topic of concern") {
		t.Errorf("expected 7-dimension checklist content, got:\n%s", out)
	}
	// Current domain's specs only (optimizer), with depends_on.
	if !strings.Contains(out, "optimizer/specs/repo.md") || !strings.Contains(out, "optimizer/specs/config.md") {
		t.Errorf("expected optimizer spec files listed, got:\n%s", out)
	}
	if strings.Contains(out, "api/specs/handlers.md") {
		t.Errorf("api (other domain) spec should not be listed, got:\n%s", out)
	}
	if !strings.Contains(out, "depends_on: [Config]") {
		t.Errorf("expected depends_on rendered, got:\n%s", out)
	}
	// Report path for the current domain and round.
	if !strings.Contains(out, filepath.Join("optimizer", "specs", ".eval", "reconciliation-r2.md")) {
		t.Errorf("expected report path, got:\n%s", out)
	}
}

// Rejection: forgectl eval in a reverse_engineering state other than
// RECONCILE_EVAL is blocked with the spec message.
func TestEvalCommandReverseEngineeringBlockedOutsideReconcileEval(t *testing.T) {
	dir := setupProjectDir(t)
	re := state.NewReverseEngineeringState("auth refactor", []string{"optimizer"})
	forgeState := &state.ForgeState{
		Phase:              state.PhaseReverseEngineering,
		State:              state.StateReconcile,
		Config:             state.DefaultForgeConfig(),
		StartedAtPhase:     state.PhaseReverseEngineering,
		ReverseEngineering: re,
	}
	setupSpecifyingState(t, dir, forgeState)

	var buf bytes.Buffer
	evalCmd.SetOut(&buf)

	err := runEval(evalCmd, nil)
	if err == nil {
		t.Fatal("expected error when calling eval outside RECONCILE_EVAL")
	}
	if !strings.Contains(err.Error(), "forgectl eval is only available during RECONCILE_EVAL.") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Functional: advancing in the reverse_engineering phase appends a JSONL log
// entry carrying the domain and reconcile round/verdict, written best-effort to
// the reverse_engineering-prefixed session log file under ~/.forgectl/logs/.
func TestAdvanceReverseEngineeringLogsDomainDetail(t *testing.T) {
	dir := setupProjectDir(t)
	// Redirect the home directory so the best-effort logger writes into the
	// test sandbox rather than the developer's real ~/.forgectl/logs/.
	home := t.TempDir()
	t.Setenv("HOME", home)

	re := state.NewReverseEngineeringState("auth refactor", []string{"optimizer", "api"})
	re.DomainIndex = 1
	re.ReconcileRound = 1
	forgeState := &state.ForgeState{
		Phase:              state.PhaseReverseEngineering,
		State:              state.StateReconcileEval,
		Config:             state.DefaultForgeConfig(),
		SessionID:          "abcd1234efgh5678",
		StartedAtPhase:     state.PhaseReverseEngineering,
		ReverseEngineering: re,
	}
	setupSpecifyingState(t, dir, forgeState)

	// Drive a real advance with a PASS verdict (terminal for the domain).
	advanceVerdict = "PASS"
	t.Cleanup(func() { advanceVerdict = "" })

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	if err := runAdvance(advanceCmd, nil); err != nil {
		t.Fatalf("advance at RE RECONCILE_EVAL: %v", err)
	}

	// Log file is named with the started-at phase prefix and the session id prefix.
	logPath := filepath.Join(home, ".forgectl", "logs", "reverse_engineering-abcd1234.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file %s: %v", logPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("invalid JSON log line: %v", err)
	}

	if entry["cmd"] != "advance" {
		t.Errorf("cmd = %v, want advance", entry["cmd"])
	}
	if entry["phase"] != "reverse_engineering" {
		t.Errorf("phase = %v, want reverse_engineering", entry["phase"])
	}
	if entry["prev_state"] != "RECONCILE_EVAL" {
		t.Errorf("prev_state = %v, want RECONCILE_EVAL", entry["prev_state"])
	}
	detail, ok := entry["detail"].(map[string]interface{})
	if !ok {
		t.Fatalf("detail missing or wrong type: %v", entry["detail"])
	}
	if detail["domain"] != "optimizer" {
		t.Errorf("detail.domain = %v, want optimizer", detail["domain"])
	}
	if detail["round"] != float64(1) {
		t.Errorf("detail.round = %v, want 1", detail["round"])
	}
	if detail["verdict"] != "PASS" {
		t.Errorf("detail.verdict = %v, want PASS", detail["verdict"])
	}
}

// Edge case: the RE log context guards the 1-based domain index and only
// attaches the round in RECONCILE_EVAL — an out-of-range index omits the domain
// (no panic) and a non-eval state omits the round, while non-RE phases yield no
// RE context at all.
func TestBuildAdvanceDetailReverseEngineeringContext(t *testing.T) {
	// Non-RE phase: no RE context captured.
	if ctx := captureRELogContext(&state.ForgeState{Phase: state.PhaseSpecifying}); ctx != nil {
		t.Errorf("expected nil context for non-RE phase, got %+v", ctx)
	}

	re := state.NewReverseEngineeringState("c", []string{"optimizer", "api"})

	// Out-of-range domain index omits the domain but still records the round
	// only when in RECONCILE_EVAL.
	re.DomainIndex = 0
	re.ReconcileRound = 2
	ctxReconcile := captureRELogContext(&state.ForgeState{
		Phase:              state.PhaseReverseEngineering,
		State:              state.StateReconcileEval,
		ReverseEngineering: re,
	})
	detail := buildAdvanceDetail(state.AdvanceInput{Verdict: "FAIL"}, ctxReconcile)
	if _, has := detail["domain"]; has {
		t.Errorf("expected no domain for out-of-range index, got %v", detail["domain"])
	}
	if detail["round"] != 2 {
		t.Errorf("detail.round = %v, want 2", detail["round"])
	}
	if detail["verdict"] != "FAIL" {
		t.Errorf("detail.verdict = %v, want FAIL", detail["verdict"])
	}

	// Non-eval RE state: domain present, round omitted.
	re.DomainIndex = 2
	ctxSurvey := captureRELogContext(&state.ForgeState{
		Phase:              state.PhaseReverseEngineering,
		State:              state.StateSurvey,
		ReverseEngineering: re,
	})
	detail = buildAdvanceDetail(state.AdvanceInput{}, ctxSurvey)
	if detail["domain"] != "api" {
		t.Errorf("detail.domain = %v, want api", detail["domain"])
	}
	if _, has := detail["round"]; has {
		t.Errorf("expected no round outside RECONCILE_EVAL, got %v", detail["round"])
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
			"refs":  []string{"notes/notes.md"},
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

// newREForgeState builds a reverse_engineering ForgeState in the given state
// with the supplied domain list locked in.
func newREForgeState(st state.StateName, domains []string) *state.ForgeState {
	re := state.NewReverseEngineeringState("auth refactor", domains)
	return &state.ForgeState{
		Phase:              state.PhaseReverseEngineering,
		State:              st,
		Config:             state.DefaultForgeConfig(),
		StartedAtPhase:     state.PhaseReverseEngineering,
		ReverseEngineering: re,
	}
}

// Functional: add-domain during QUEUE appends the domain and updates the count.
func TestAddDomainInQueueState(t *testing.T) {
	dir := setupProjectDir(t)
	forgeState := newREForgeState(state.StateQueue, []string{"optimizer", "api"})
	sd := setupSpecifyingState(t, dir, forgeState)

	if err := runAddDomain(addDomainCmd, []string{"portal"}); err != nil {
		t.Fatalf("add-domain: %v", err)
	}

	s, _ := state.Load(sd)
	want := []string{"optimizer", "api", "portal"}
	if len(s.ReverseEngineering.Domains) != 3 {
		t.Fatalf("domains = %v, want %v", s.ReverseEngineering.Domains, want)
	}
	for i, d := range want {
		if s.ReverseEngineering.Domains[i] != d {
			t.Errorf("domain[%d] = %q, want %q", i, s.ReverseEngineering.Domains[i], d)
		}
	}
	if s.ReverseEngineering.DomainCount != 3 {
		t.Errorf("domain count = %d, want 3", s.ReverseEngineering.DomainCount)
	}
}

// Rejection: add-domain is blocked outside QUEUE and rejects a duplicate domain.
func TestAddDomainRejections(t *testing.T) {
	// Blocked outside QUEUE.
	dir := setupProjectDir(t)
	forgeState := newREForgeState(state.StateSurvey, []string{"optimizer", "api"})
	setupSpecifyingState(t, dir, forgeState)

	if err := runAddDomain(addDomainCmd, []string{"portal"}); err == nil {
		t.Error("expected error when not in QUEUE state")
	} else if !strings.Contains(err.Error(), "only available during the QUEUE state") {
		t.Errorf("unexpected wrong-state error: %v", err)
	}

	// Duplicate domain rejected during QUEUE.
	dir2 := setupProjectDir(t)
	qState := newREForgeState(state.StateQueue, []string{"optimizer", "api"})
	sd := setupSpecifyingState(t, dir2, qState)

	if err := runAddDomain(addDomainCmd, []string{"api"}); err == nil {
		t.Error("expected error for duplicate domain")
	} else if !strings.Contains(err.Error(), `domain "api" already exists`) {
		t.Errorf("unexpected duplicate error: %v", err)
	}
	// State unchanged: still two domains.
	s, _ := state.Load(sd)
	if len(s.ReverseEngineering.Domains) != 2 {
		t.Errorf("domain list should be unchanged on rejection, got %v", s.ReverseEngineering.Domains)
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

// captureStderr redirects os.Stderr around fn and returns what was written there.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestAdvanceWarnMessageIgnoredWhenCommitsDisabled verifies the cmd-layer warning
// for --message when commits are disabled uses the exact spec wording and does not
// instruct how to enable commits.
func TestAdvanceWarnMessageIgnoredWhenCommitsDisabled(t *testing.T) {
	s := &state.ForgeState{
		Phase:  state.PhaseImplementing,
		State:  state.StateCommit,
		Config: state.DefaultForgeConfig(), // enable_commits defaults to false
	}
	advanceMessage = "some commit"
	advanceEvalReport = ""
	defer func() { advanceMessage = "" }()

	var buf bytes.Buffer
	printAdvanceWarnings(&buf, s)

	got := buf.String()
	if !strings.Contains(got, "--message is ignored, commits are not enabled") {
		t.Errorf("expected spec-worded --message warning, got: %q", got)
	}
	if strings.Contains(got, "enable") && strings.Contains(strings.ToLower(got), "set ") {
		t.Errorf("warning must not instruct how to enable commits, got: %q", got)
	}
}

// TestAdvanceWarnEvalReportIgnoredInCrossRefNonReport verifies that supplying
// --eval-report in CROSS_REFERENCE_EVAL while eval_mode is not "report" emits the
// spec-worded ignore warning (CROSS_REFERENCE_EVAL is now in the warning's state set).
func TestAdvanceWarnEvalReportIgnoredInCrossRefNonReport(t *testing.T) {
	cfg := state.DefaultForgeConfig()
	cfg.Specifying.Eval.EvalMode = "conversational"
	s := &state.ForgeState{
		Phase:  state.PhaseSpecifying,
		State:  state.StateCrossReferenceEval,
		Config: cfg,
	}
	advanceEvalReport = "some/report.md"
	advanceMessage = ""
	defer func() { advanceEvalReport = "" }()

	var buf bytes.Buffer
	printAdvanceWarnings(&buf, s)

	if !strings.Contains(buf.String(), "--eval-report is ignored, --eval-report is only used in report mode") {
		t.Errorf("expected spec-worded --eval-report warning in CROSS_REFERENCE_EVAL, got: %q", buf.String())
	}
}

// TestAdvanceEvalReportWarningPrintedExactlyOnce verifies the ignore warning is not
// duplicated across the cmd layer (printAdvanceWarnings) and the state transition
// layer (state.Advance): summed over both output streams it appears exactly once.
func TestAdvanceEvalReportWarningPrintedExactlyOnce(t *testing.T) {
	dir := t.TempDir()

	cfg := state.DefaultForgeConfig()
	cfg.Specifying.Eval.EvalMode = "conversational"
	cfg.Specifying.CrossReference.MinRounds = 1
	cfg.Specifying.CrossReference.MaxRounds = 3

	spec := state.NewSpecifyingState([]state.SpecQueueEntry{})
	spec.CurrentDomain = "test"
	spec.CrossReference = map[string]*state.CrossReferenceState{
		"test": {Domain: "test", Round: 1},
	}
	s := &state.ForgeState{
		Phase:          state.PhaseSpecifying,
		State:          state.StateCrossReferenceEval,
		Config:         cfg,
		StartedAtPhase: state.PhaseSpecifying,
		Specifying:     spec,
	}

	reportFile := filepath.Join(dir, "report.md")
	os.WriteFile(reportFile, []byte("report"), 0644)
	advanceEvalReport = reportFile
	advanceMessage = ""
	defer func() { advanceEvalReport = "" }()

	const warn = "--eval-report is ignored"

	// cmd layer.
	var buf bytes.Buffer
	printAdvanceWarnings(&buf, s)

	// state transition layer.
	in := state.AdvanceInput{Verdict: "PASS", EvalReport: advanceEvalReport}
	stderr := captureStderr(t, func() {
		if err := state.Advance(s, in, dir); err != nil {
			t.Fatalf("advance should proceed in conversational mode: %v", err)
		}
	})

	total := strings.Count(buf.String(), warn) + strings.Count(stderr, warn)
	if total != 1 {
		t.Errorf("ignore warning should appear exactly once across cmd+state layers, got %d (cmd=%q, state=%q)",
			total, buf.String(), stderr)
	}
}

// --- Planning readiness gate at init --phase planning ---
//
// init --phase planning is a cold-start entry into a planning cycle, so it is
// the first place the gate has to hold. The tests below pin the two halves of
// that guarantee: a clean queue is untouched by the gate, and a dirty one stops
// init before it writes anything — a half-initialized session over stale
// artifacts is worse than no session at all.

// runInitCmd invokes init with the given flags and captures stdout separately
// from the returned error, which Execute() routes to stderr with exit 1.
func runInitCmd(t *testing.T, from, phase string) (string, error) {
	t.Helper()
	initFrom = from
	initPhase = phase
	t.Cleanup(func() { initFrom = ""; initPhase = "specifying" })

	var out bytes.Buffer
	initCmd.SetOut(&out)
	t.Cleanup(func() { initCmd.SetOut(nil) })

	err := runInit(initCmd, nil)
	return out.String(), err
}

// Functional: a clean incoming queue passes the gate and init behaves exactly
// as it did before the gate existed.
func TestInitPlanningWithCleanWorkspacesProceeds(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core", "api")

	out, err := runInitCmd(t, queue, "planning")
	if err != nil {
		t.Fatalf("init should proceed over clean workspaces: %v", err)
	}
	if strings.Contains(out, "Planning readiness") {
		t.Errorf("a ready verdict must produce no gate output, got:\n%s", out)
	}

	s, err := state.Load(resolvedStateDir(dir))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if s.Phase != state.PhasePlanning || s.State != state.StateOrient {
		t.Errorf("phase/state = %s/%s, want planning/ORIENT", s.Phase, s.State)
	}
	if s.Planning.CurrentPlan == nil || s.Planning.CurrentPlan.Domain != "core" {
		t.Errorf("current plan should be the first queue entry, got %+v", s.Planning.CurrentPlan)
	}
}

// Rejection: a dirty incoming domain blocks init and the operator is told which
// domain and what to do about it — the remediation is the whole point of the
// message, so it is asserted alongside the domain name.
func TestInitPlanningBlockedByDirtyWorkspace(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core", "api")
	writeWorkspaceFile(t, dir, "api", "implementation_plan/plan.json")

	out, err := runInitCmd(t, queue, "planning")
	if err == nil {
		t.Fatal("a dirty incoming workspace must fail init (exit non-zero)")
	}
	if !strings.Contains(out, "Planning readiness: BLOCKED") {
		t.Errorf("output missing BLOCKED verdict:\n%s", out)
	}
	if !strings.Contains(out, "api") || !strings.Contains(out, "api/.forge_workspace/") {
		t.Errorf("output must name the dirty domain and its workspace path:\n%s", out)
	}
	if !strings.Contains(out, "Run the workspace close-out procedure") {
		t.Errorf("output missing the close-out remediation:\n%s", out)
	}
	// The clean domain is not an operator problem, so it stays out of the list.
	if strings.Contains(out, "- core") {
		t.Errorf("clean domain core should not be listed as dirty:\n%s", out)
	}
}

// Rejection: no partial entry. The gate runs before any mutation, so a blocked
// init leaves the state directory exactly as it found it.
func TestInitPlanningBlockedWritesNoStateFile(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core")
	writeWorkspaceFile(t, dir, "core", ".gitkeep")

	if _, err := runInitCmd(t, queue, "planning"); err == nil {
		t.Fatal("expected a blocked init")
	}

	stateDir := resolvedStateDir(dir)
	if state.Exists(stateDir) {
		t.Error("a blocked init must not create a state file")
	}
	if entries, err := os.ReadDir(stateDir); err == nil && len(entries) > 0 {
		t.Errorf("state dir should be unchanged, found %d entries", len(entries))
	}
}

// Edge case: the gate reports the full set of offending domains in one pass.
// Naming only the first would make close-out an iterative guessing game.
func TestInitPlanningNamesEveryDirtyDomain(t *testing.T) {
	dir := setupProjectDir(t)
	queue := filepath.Join(dir, "plan-queue.json")
	writePreflightQueue(t, queue, "core", "api", "web")
	writeWorkspaceFile(t, dir, "core", "notes/study.md")
	writeWorkspaceFile(t, dir, "web", "implementation/IMPLEMENTATION_LOG.md")

	out, err := runInitCmd(t, queue, "planning")
	if err == nil {
		t.Fatal("expected a blocked init")
	}
	for _, want := range []string{"core/.forge_workspace/", "web/.forge_workspace/"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing dirty domain path %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "api/.forge_workspace/") {
		t.Errorf("clean domain api should not be listed:\n%s", out)
	}
}

// Functional: the gate guards the start of a planning cycle only. The other
// init phases do not begin one and carry no incoming plan queue, so a dirty
// workspace is none of their business.
func TestInitNonPlanningPhasesAreUngated(t *testing.T) {
	t.Run("implementing", func(t *testing.T) {
		dir := setupProjectDir(t)
		planPath := writeUIPlan(t, dir) // plan context domain is "ui"
		writeWorkspaceFile(t, dir, "ui", "implementation_plan/plan.json")

		if _, err := runInitCmd(t, planPath, "implementing"); err != nil {
			t.Fatalf("init --phase implementing must be ungated: %v", err)
		}
		s, err := state.Load(resolvedStateDir(dir))
		if err != nil {
			t.Fatalf("load state: %v", err)
		}
		if s.Phase != state.PhaseImplementing {
			t.Errorf("phase = %s, want implementing", s.Phase)
		}
	})

	t.Run("specifying", func(t *testing.T) {
		dir := setupProjectDir(t)
		input := state.SpecQueueInput{
			Specs: []state.SpecQueueEntry{
				{Name: "Spec A", Domain: "core", Topic: "topic A", File: "specs/a.md", PlanningSources: []string{}, DependsOn: []string{}},
			},
		}
		data, _ := json.Marshal(input)
		queueFile := filepath.Join(dir, "specs-queue.json")
		if err := os.WriteFile(queueFile, data, 0644); err != nil {
			t.Fatal(err)
		}
		writeWorkspaceFile(t, dir, "core", "implementation_plan/plan.json")

		if _, err := runInitCmd(t, queueFile, "specifying"); err != nil {
			t.Fatalf("init --phase specifying must be ungated: %v", err)
		}
		s, err := state.Load(resolvedStateDir(dir))
		if err != nil {
			t.Fatalf("load state: %v", err)
		}
		if s.Phase != state.PhaseSpecifying {
			t.Errorf("phase = %s, want specifying", s.Phase)
		}
	})
}
