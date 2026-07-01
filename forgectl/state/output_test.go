package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// outputOf runs PrintAdvanceOutput and returns the result as a string.
func outputOf(s *ForgeState, dir string) string {
	var buf bytes.Buffer
	PrintAdvanceOutput(&buf, s, dir)
	return buf.String()
}

// TestOutputCommitEnableCommitsShowsMessage verifies that COMMIT with enable_commits=true
// instructs the user to advance with --message.
func TestOutputCommitEnableCommitsShowsMessage(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = true
	advanceImplToCommit(t, s, dir)

	out := outputOf(s, dir)
	if !strings.Contains(out, "--message") {
		t.Errorf("expected --message in COMMIT output with enable_commits=true, got:\n%s", out)
	}
	if strings.Contains(out, "Advance to continue.") {
		t.Errorf("unexpected 'Advance to continue.' in COMMIT output with enable_commits=true, got:\n%s", out)
	}
}

// TestOutputCommitNoCommitsShowsAdvance verifies that COMMIT with enable_commits=false
// shows a simple "Advance to continue." action.
func TestOutputCommitNoCommitsShowsAdvance(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = false
	advanceImplToCommit(t, s, dir)

	out := outputOf(s, dir)
	if !strings.Contains(out, "advance to continue") {
		t.Errorf("expected 'advance to continue' in COMMIT output with enable_commits=false, got:\n%s", out)
	}
	if strings.Contains(out, "--message") {
		t.Errorf("unexpected --message in COMMIT output with enable_commits=false, got:\n%s", out)
	}
}

// TestOutputAcceptEnableCommitsShowsMessage verifies that ACCEPT with enable_commits=true
// instructs the user to advance with --message.
func TestOutputAcceptEnableCommitsShowsMessage(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningStateForCommit(t, dir)
	s.Config.General.EnableCommits = true
	s.Planning.Round = 1
	s.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "PASS"}}
	s.State = StateAccept

	out := outputOf(s, dir)
	if !strings.Contains(out, "--message") {
		t.Errorf("expected --message in ACCEPT output with enable_commits=true, got:\n%s", out)
	}
}

// TestOutputAcceptNoCommitsShowsAdvance verifies that ACCEPT with enable_commits=false
// shows "Advance to continue.".
func TestOutputAcceptNoCommitsShowsAdvance(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningStateForCommit(t, dir)
	s.Config.General.EnableCommits = false
	s.Planning.Round = 1
	s.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "PASS"}}
	s.State = StateAccept

	out := outputOf(s, dir)
	if !strings.Contains(out, "Advance to continue.") {
		t.Errorf("expected 'Advance to continue.' in ACCEPT output with enable_commits=false, got:\n%s", out)
	}
	if strings.Contains(out, "--message") {
		t.Errorf("unexpected --message in ACCEPT output with enable_commits=false, got:\n%s", out)
	}
}

// TestOutputImplementSpecsAndRefsMultiline verifies that IMPLEMENT output shows
// Specs:/Refs: labels with multiline formatting.
func TestOutputImplementSpecsAndRefsMultiline(t *testing.T) {
	dir := t.TempDir()

	// Build a plan with multi-spec, multi-ref item.
	planPath := filepath.Join(dir, "impl", "plan.json")
	os.MkdirAll(filepath.Dir(planPath), 0755)
	notesDir := filepath.Join(filepath.Dir(planPath), "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "a.md"), []byte("notes"), 0644)
	os.WriteFile(filepath.Join(notesDir, "b.md"), []byte("notes"), 0644)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "mod"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"x.item"}}},
		Items: []PlanItem{
			{
				ID:          "x.item",
				Name:        "X Item",
				Description: "desc",
				DependsOn:   []string{},
				Passes:      "pending",
				Specs:       []string{"spec-a.md#section", "spec-b.md#other"},
				Refs:        []string{"notes/a.md", "notes/b.md"},
				Tests:       []PlanTest{{Category: "functional", Description: "works"}},
			},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)

	s := &ForgeState{
		Phase: PhaseImplementing,
		State: StateOrient,
		Config: ForgeConfig{
			Implementing: ImplementingConfig{
				Batch: 2,
				Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{ID: 1, Name: "Test Plan", Domain: "test", File: "impl/plan.json"},
		},
		Implementing: NewImplementingState(),
	}

	// Advance to IMPLEMENT.
	Advance(s, AdvanceInput{}, dir)
	if s.State != StateImplement {
		t.Fatalf("expected IMPLEMENT, got %s", s.State)
	}

	out := outputOf(s, dir)
	if !strings.Contains(out, "Specs:   spec-a.md#section") {
		t.Errorf("expected 'Specs:   spec-a.md#section', got:\n%s", out)
	}
	if !strings.Contains(out, "         spec-b.md#other") {
		t.Errorf("expected indented second spec, got:\n%s", out)
	}
	if !strings.Contains(out, "Refs:    notes/a.md") {
		t.Errorf("expected 'Refs:    notes/a.md', got:\n%s", out)
	}
	if !strings.Contains(out, "         notes/b.md") {
		t.Errorf("expected indented second ref, got:\n%s", out)
	}
	if strings.Contains(out, "Spec:    ") {
		t.Errorf("unexpected old 'Spec:' label in output, got:\n%s", out)
	}
	if strings.Contains(out, "Ref:     ") {
		t.Errorf("unexpected old 'Ref:' label in output, got:\n%s", out)
	}
}

// implSpecReadState builds an implementing ForgeState at IMPLEMENT (round 1)
// whose single item carries the given specs/refs and whose current plan carries
// the given spec_commits. Used to exercise the per-spec `Read:` git command and
// the IMPLEMENT review reminders.
func implSpecReadState(t *testing.T, dir string, specs, refs, commits []string) *ForgeState {
	t.Helper()
	planPath := filepath.Join(dir, "impl", "plan.json")
	os.MkdirAll(filepath.Dir(planPath), 0755)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "mod"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"x.item"}}},
		Items: []PlanItem{
			{
				ID:          "x.item",
				Name:        "X Item",
				Description: "desc",
				DependsOn:   []string{},
				Passes:      "pending",
				Specs:       specs,
				Refs:        refs,
				Tests:       []PlanTest{{Category: "functional", Description: "works"}},
			},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)

	s := &ForgeState{
		Phase: PhaseImplementing,
		State: StateOrient,
		Config: ForgeConfig{
			Implementing: ImplementingConfig{
				Batch: 1,
				Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{ID: 1, Name: "Test Plan", Domain: "test", File: "impl/plan.json", SpecCommits: commits},
		},
		Implementing: NewImplementingState(),
	}

	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT
	if s.State != StateImplement {
		t.Fatalf("expected IMPLEMENT, got %s", s.State)
	}
	return s
}

// TestOutputImplementReadCommandPerSpec verifies a bounded `git show` Read
// command is emitted under a spec entry when the plan has spec_commits. It uses
// git show (not git log) so the command resolves to exactly the named commits.
func TestOutputImplementReadCommandPerSpec(t *testing.T) {
	dir := t.TempDir()
	s := implSpecReadState(t, dir, []string{"spec-sqlc-schemas.md#x"}, nil, []string{"e742a1b", "694ca99"})
	out := outputOf(s, dir)

	want := "Read: git show e742a1b 694ca99 -- '**/spec-sqlc-schemas.md'"
	if !strings.Contains(out, want) {
		t.Errorf("expected %q, got:\n%s", want, out)
	}
	if strings.Contains(out, "git log") {
		t.Errorf("Read command must use git show, not git log, got:\n%s", out)
	}
}

// TestOutputImplementOmitsReadCommandWhenNoSpecCommits verifies no Read line is
// emitted when spec_commits is empty, and the spec-review reminder falls back to
// "read the spec file(s) listed above."
func TestOutputImplementOmitsReadCommandWhenNoSpecCommits(t *testing.T) {
	dir := t.TempDir()
	s := implSpecReadState(t, dir, []string{"spec-a.md#x"}, nil, nil)
	out := outputOf(s, dir)

	if strings.Contains(out, "Read:") {
		t.Errorf("expected no Read line when spec_commits empty, got:\n%s", out)
	}
	if !strings.Contains(out, "read the spec file(s) listed above") {
		t.Errorf("expected fallback reminder 'read the spec file(s) listed above', got:\n%s", out)
	}
}

// TestOutputImplementReadCommandPerSpecEntry verifies multi-spec items get one
// Read line each, with the #anchor stripped from the pathspec glob.
func TestOutputImplementReadCommandPerSpecEntry(t *testing.T) {
	dir := t.TempDir()
	s := implSpecReadState(t, dir, []string{"a.md#x", "b.md#y"}, nil, []string{"abc1234"})
	out := outputOf(s, dir)

	for _, want := range []string{
		"Read: git show abc1234 -- '**/a.md'",
		"Read: git show abc1234 -- '**/b.md'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q, got:\n%s", want, out)
		}
	}
}

// implEvalSpecReadState builds an implementing ForgeState at EVALUATE (round 1)
// whose single batch item carries the given specs and whose current plan
// carries the given spec_commits, for a given eval_mode. Used to verify that
// `forgectl eval` surfaces the same bounded `git show` Read command as
// IMPLEMENT, regardless of eval_mode.
func implEvalSpecReadState(t *testing.T, dir, mode string, specs, commits []string) *ForgeState {
	t.Helper()
	planPath := filepath.Join(dir, "impl", "plan.json")
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.MkdirAll(filepath.Join(dir, "test"), 0755)
	os.WriteFile(filepath.Join(dir, "test", "main.go"), []byte("package main"), 0644)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "mod"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"x.item"}}},
		Items: []PlanItem{
			{
				ID:          "x.item",
				Name:        "X Item",
				Description: "desc",
				DependsOn:   []string{},
				Passes:      "pending",
				Specs:       specs,
				Tests:       []PlanTest{{Category: "functional", Description: "works"}},
			},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)

	s := &ForgeState{
		Phase: PhaseImplementing,
		State: StateOrient,
		Config: ForgeConfig{
			Implementing: ImplementingConfig{
				Batch: 1,
				Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: mode},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{ID: 1, Name: "Test Plan", Domain: "test", File: "impl/plan.json", SpecCommits: commits},
		},
		Implementing: NewImplementingState(),
	}

	advanceImplToEvaluate(t, s, dir)
	return s
}

// TestEvalOutputImplementingReadCommandPerSpec verifies `forgectl eval` emits
// the same bounded `git show` Read command under each Specs: entry as
// IMPLEMENT, in every eval_mode — the eval sub-agent needs the exact spec
// definition to judge or correct code against it regardless of whether it is
// writing a report, correcting files directly, or reporting verbally.
func TestEvalOutputImplementingReadCommandPerSpec(t *testing.T) {
	for _, mode := range []string{"report", "direct", "conversational"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			s := implEvalSpecReadState(t, dir, mode, []string{"spec-sqlc-schemas.md#x"}, []string{"e742a1b", "694ca99"})
			var buf bytes.Buffer
			if err := PrintEvalOutput(&buf, s, dir); err != nil {
				t.Fatalf("PrintEvalOutput: %v", err)
			}
			out := buf.String()

			want := "Read:        git show e742a1b 694ca99 -- '**/spec-sqlc-schemas.md'"
			if !strings.Contains(out, want) {
				t.Errorf("mode %q: expected %q, got:\n%s", mode, want, out)
			}
			if strings.Contains(out, "git log") {
				t.Errorf("mode %q: Read command must use git show, not git log, got:\n%s", mode, out)
			}
		})
	}
}

// TestEvalOutputImplementingOmitsReadCommandWhenNoSpecCommits verifies no Read
// line is emitted in eval output when spec_commits is empty.
func TestEvalOutputImplementingOmitsReadCommandWhenNoSpecCommits(t *testing.T) {
	dir := t.TempDir()
	s := implEvalSpecReadState(t, dir, "report", []string{"spec-a.md#x"}, nil)
	var buf bytes.Buffer
	if err := PrintEvalOutput(&buf, s, dir); err != nil {
		t.Fatalf("PrintEvalOutput: %v", err)
	}
	if strings.Contains(buf.String(), "Read:") {
		t.Errorf("expected no Read line when spec_commits empty, got:\n%s", buf.String())
	}
}

// TestOutputImplementReviewReminderEveryRound verifies the spec-review reminder
// appears on subsequent (post-eval) rounds, not only the first round.
func TestOutputImplementReviewReminderEveryRound(t *testing.T) {
	dir := t.TempDir()
	s := implReentryState(t, dir, "report")
	// The re-entry plan has no specs; give the current plan spec_commits so the
	// git-command variant of the reminder is exercised on round 2+.
	s.Planning.CurrentPlan.SpecCommits = []string{"abc1234"}
	out := outputOf(s, dir)

	want := "Please review the specification(s) above if you have not already done so"
	if !strings.Contains(out, want) {
		t.Errorf("expected spec-review reminder on round 2+, got:\n%s", out)
	}
}

// TestOutputImplementRefsReminderGatedOnRefs verifies the Refs-review reminder
// is present iff the item has Refs.
func TestOutputImplementRefsReminderGatedOnRefs(t *testing.T) {
	const refsReminder = "Please review the reference file(s) under Refs if you have not already done so."

	dirWith := t.TempDir()
	withRefs := implSpecReadState(t, dirWith, []string{"a.md#x"}, []string{"notes/a.md"}, []string{"abc1234"})
	if out := outputOf(withRefs, dirWith); !strings.Contains(out, refsReminder) {
		t.Errorf("expected Refs reminder when item has Refs, got:\n%s", out)
	}

	dirWithout := t.TempDir()
	noRefs := implSpecReadState(t, dirWithout, []string{"a.md#x"}, nil, []string{"abc1234"})
	if out := outputOf(noRefs, dirWithout); strings.Contains(out, refsReminder) {
		t.Errorf("expected no Refs reminder when item has no Refs, got:\n%s", out)
	}
}

// TestOutputOrientNextBatchCount verifies that after a COMMIT within a layer,
// the ORIENT output shows "Next: N unblocked items in next batch".
func TestOutputOrientNextBatchCount(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 3, 1) // 3 items, batch=1
	s.Config.General.EnableCommits = false

	// Advance through first item (ORIENT→IMPLEMENT→EVALUATE→COMMIT→ORIENT).
	Advance(s, AdvanceInput{}, dir)               // ORIENT→IMPLEMENT
	Advance(s, AdvanceInput{Message: "msg"}, dir) // IMPLEMENT→EVALUATE

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir) // EVALUATE→COMMIT
	Advance(s, AdvanceInput{Message: "commit"}, dir)                     // COMMIT→ORIENT

	if s.State != StateOrient {
		t.Fatalf("expected ORIENT, got %s", s.State)
	}

	out := outputOf(s, dir)
	if !strings.Contains(out, "Next:") {
		t.Errorf("expected 'Next:' line in ORIENT output, got:\n%s", out)
	}
	if !strings.Contains(out, "unblocked items in next batch") {
		t.Errorf("expected 'unblocked items in next batch' in ORIENT output, got:\n%s", out)
	}
}

// TestOutputOrientFinalLayerLabel verifies that the Progress line shows "(final layer)"
// when the current layer is the last layer and all its items are terminal.
func TestOutputOrientFinalLayerLabel(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1) // 1 item, 1 layer

	// Advance to IMPLEMENT to set CurrentLayer and CurrentBatch.
	Advance(s, AdvanceInput{}, dir) // initial ORIENT → IMPLEMENT

	// Mark the item as passed in the plan file and set state to ORIENT to test output.
	plan, err := loadPlan(s, dir)
	if err != nil {
		t.Fatalf("loadPlan: %v", err)
	}
	for i := range plan.Items {
		plan.Items[i].Passes = "passed"
		plan.Items[i].Rounds = 1
	}
	if err := savePlan(s, dir, plan); err != nil {
		t.Fatalf("savePlan: %v", err)
	}
	s.State = StateOrient

	out := outputOf(s, dir)
	if !strings.Contains(out, "final layer") {
		t.Errorf("expected 'final layer' in Progress line, got:\n%s", out)
	}
}

// TestOutputStudySpecsSpawnInstruction verifies that STUDY_SPECS output instructs the
// architect to spawn the configured number of haiku explore sub-agents and distribute
// specs evenly, without a Commits line when SpecCommits is empty.
func TestOutputStudySpecsSpawnInstruction(t *testing.T) {
	s := newPlanningState()
	s.Config.Planning.StudySpecs = StudySpecsConfig{
		AgentConfig: AgentConfig{Model: "haiku", Type: "explore", Count: 3},
	}
	s.State = StateStudySpecs

	out := outputOf(s, "")
	for _, want := range []string{
		"Please spawn 3 haiku explore sub-agents to study the specs.",
		"Distribute the specs above evenly across sub-agents; each receives a disjoint subset.",
		"spec content, dependencies, cross-references.",
		"After completion of the above, advance to continue.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("STUDY_SPECS output missing %q\nfull output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Commits:") {
		t.Errorf("STUDY_SPECS output must not include Commits: line when SpecCommits is empty\nfull output:\n%s", out)
	}
	if strings.Contains(out, "git diffs for commits above") {
		t.Errorf("STUDY_SPECS action must omit git diffs reference when SpecCommits is empty\nfull output:\n%s", out)
	}
}

// TestOutputStudySpecsWithCommits verifies that STUDY_SPECS output includes the Commits:
// line and references git diffs in the action when SpecCommits is non-empty.
func TestOutputStudySpecsWithCommits(t *testing.T) {
	s := newPlanningState()
	s.Config.Planning.StudySpecs = StudySpecsConfig{
		AgentConfig: AgentConfig{Model: "haiku", Type: "explore", Count: 2},
	}
	s.Planning.CurrentPlan.SpecCommits = []string{"abc1234", "def5678"}
	s.State = StateStudySpecs

	out := outputOf(s, "")
	if !strings.Contains(out, "Commits: abc1234, def5678") {
		t.Errorf("STUDY_SPECS output missing Commits: line\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "git diffs for commits above") {
		t.Errorf("STUDY_SPECS action must reference git diffs when SpecCommits is non-empty\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "Please spawn 2 haiku explore sub-agents") {
		t.Errorf("STUDY_SPECS output must reflect configured count\nfull output:\n%s", out)
	}
}

// TestEvalOutputPlanningReportSectionWithEnableEvalOutput verifies that planning eval output
// includes '--- REPORT OUTPUT ---' when enable_eval_output is true.
func TestEvalOutputPlanningReportSectionWithEnableEvalOutput(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningState()
	s.Planning.CurrentPlan.File = "impl/plan.json"
	s.Planning.Round = 1
	s.Config.General.EnableEvalOutput = true
	s.State = StateEvaluate

	var buf bytes.Buffer
	if err := PrintEvalOutput(&buf, s, dir); err != nil {
		t.Fatalf("PrintEvalOutput: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("expected '--- REPORT OUTPUT ---' in planning eval output with enable_eval_output=true, got:\n%s", out)
	}
}

// TestEvalOutputPlanningReportSectionOmittedWithoutEnableEvalOutput verifies that planning eval
// output omits '--- REPORT OUTPUT ---' when enable_eval_output is false.
func TestEvalOutputPlanningReportSectionOmittedWithoutEnableEvalOutput(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningState()
	s.Planning.CurrentPlan.File = "impl/plan.json"
	s.Planning.Round = 1
	s.Config.General.EnableEvalOutput = false
	s.State = StateEvaluate

	var buf bytes.Buffer
	if err := PrintEvalOutput(&buf, s, dir); err != nil {
		t.Fatalf("PrintEvalOutput: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("unexpected '--- REPORT OUTPUT ---' in planning eval output with enable_eval_output=false, got:\n%s", out)
	}
}

// TestEvalOutputReconcileEvalContainsEvaluatorPrompt verifies that eval in RECONCILE_EVAL
// state outputs the reconcile-eval.md contents.
func TestEvalOutputReconcileEvalContainsEvaluatorPrompt(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateReconcileEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Reconciliation: ReconciliationConfig{
					MinRounds: 1,
					MaxRounds: 2,
				},
			},
		},
		Specifying: &SpecifyingState{
			Reconcile: &ReconcileState{Round: 1},
			Completed: []CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "optimizer", File: "optimizer/specs/spec-a.md", RoundsTaken: 1},
				{ID: 2, Name: "spec-b.md", Domain: "portal", File: "portal/specs/spec-b.md", RoundsTaken: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintReconcileEvalOutput(&buf, s); err != nil {
		t.Fatalf("PrintReconcileEvalOutput: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "RECONCILIATION EVALUATION") {
		t.Errorf("expected 'RECONCILIATION EVALUATION' header, got:\n%s", out)
	}
	if !strings.Contains(out, "--- EVALUATOR INSTRUCTIONS ---") {
		t.Errorf("expected evaluator instructions section, got:\n%s", out)
	}
	if !strings.Contains(out, "Reconciliation Evaluation Prompt") {
		t.Errorf("expected reconcile-eval.md contents, got:\n%s", out)
	}
	if !strings.Contains(out, "--- DOMAINS ---") {
		t.Errorf("expected domains section, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer: 1") {
		t.Errorf("expected optimizer domain count, got:\n%s", out)
	}
	if !strings.Contains(out, "--- RECONCILIATION CONTEXT ---") {
		t.Errorf("expected reconciliation context section, got:\n%s", out)
	}
	if strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("unexpected report output section when enable_eval_output=false, got:\n%s", out)
	}
}

// TestEvalOutputCrossRefEvalContainsEvaluatorPrompt verifies that eval in CROSS_REFERENCE_EVAL
// state outputs the cross-reference-eval.md contents.
func TestEvalOutputCrossRefEvalContainsEvaluatorPrompt(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateCrossReferenceEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				CrossReference: CrossRefConfig{
					MinRounds: 1,
					MaxRounds: 2,
				},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain: "optimizer",
			CrossReference: map[string]*CrossReferenceState{
				"optimizer": {Domain: "optimizer", Round: 1},
			},
			Completed: []CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "optimizer", File: "optimizer/specs/spec-a.md", RoundsTaken: 1},
				{ID: 2, Name: "spec-b.md", Domain: "optimizer", File: "optimizer/specs/spec-b.md", RoundsTaken: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintCrossRefEvalOutput(&buf, s); err != nil {
		t.Fatalf("PrintCrossRefEvalOutput: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "CROSS-REFERENCE EVALUATION") {
		t.Errorf("expected 'CROSS-REFERENCE EVALUATION' header, got:\n%s", out)
	}
	if !strings.Contains(out, "--- EVALUATOR INSTRUCTIONS ---") {
		t.Errorf("expected evaluator instructions section, got:\n%s", out)
	}
	if !strings.Contains(out, "Cross-Reference Evaluation Prompt") {
		t.Errorf("expected cross-reference-eval.md contents, got:\n%s", out)
	}
	if !strings.Contains(out, "--- DOMAIN ---") {
		t.Errorf("expected domain section, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer: 2") {
		t.Errorf("expected optimizer domain with 2 specs, got:\n%s", out)
	}
	if !strings.Contains(out, "--- SPECS ---") {
		t.Errorf("expected specs section, got:\n%s", out)
	}
}

// TestEvalOutputSpecEvaluateContainsEvaluatorPrompt is the regression for the
// routing gap: `forgectl eval` in specifying EVALUATE must emit the SPEC
// EVALUATION block (header, evaluator instructions, and the spec listing with
// filename, topic, and full path) instead of falling through to the default
// "eval is only valid in ..." error.
func TestEvalOutputSpecEvaluateContainsEvaluatorPrompt(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateEvaluate,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain: "optimizer",
			BatchNumber:   1,
			CurrentSpecs: []*ActiveSpec{
				{ID: 1, Name: "Repository Loading", Domain: "optimizer", Topic: "The scaffold loads repository snapshots for diffing.", File: "optimizer/specs/repository-loading.md", Round: 1},
				{ID: 2, Name: "Snapshot Diffing", Domain: "optimizer", Topic: "The scaffold diffs repository snapshots to detect changes.", File: "optimizer/specs/snapshot-diffing.md", Round: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintSpecEvalOutput(&buf, s, "."); err != nil {
		t.Fatalf("PrintSpecEvalOutput: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "=== SPEC EVALUATION ROUND 1/3 ===") {
		t.Errorf("expected 'SPEC EVALUATION ROUND 1/3' header, got:\n%s", out)
	}
	if !strings.Contains(out, "Domain: optimizer") {
		t.Errorf("expected domain line, got:\n%s", out)
	}
	if !strings.Contains(out, "Batch:  1") {
		t.Errorf("expected batch line, got:\n%s", out)
	}
	if !strings.Contains(out, "--- EVALUATOR INSTRUCTIONS ---") {
		t.Errorf("expected evaluator instructions section, got:\n%s", out)
	}
	if !strings.Contains(out, "Spec Evaluation Prompt") {
		t.Errorf("expected spec-eval.md contents, got:\n%s", out)
	}
	if !strings.Contains(out, "--- SPECS TO EVALUATE ---") {
		t.Errorf("expected specs section, got:\n%s", out)
	}
	if !strings.Contains(out, "[1] repository-loading.md") {
		t.Errorf("expected spec filename listing, got:\n%s", out)
	}
	if !strings.Contains(out, "Topic: The scaffold loads repository snapshots for diffing.") {
		t.Errorf("expected spec topic listing, got:\n%s", out)
	}
	if !strings.Contains(out, "File:  optimizer/specs/repository-loading.md") {
		t.Errorf("expected spec full path listing, got:\n%s", out)
	}
}

// TestEvalOutputSpecEvaluateRejectsWrongState verifies PrintSpecEvalOutput
// rejects being called outside specifying EVALUATE, naming the current state.
func TestEvalOutputSpecEvaluateRejectsWrongState(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateDraft,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Specifying: &SpecifyingState{},
	}

	var buf bytes.Buffer
	err := PrintSpecEvalOutput(&buf, s, ".")
	if err == nil {
		t.Fatal("expected error when calling PrintSpecEvalOutput outside specifying EVALUATE")
	}
	if !strings.Contains(err.Error(), string(StateDraft)) {
		t.Errorf("expected error to mention current state %q, got: %v", StateDraft, err)
	}
}

// specEvaluateState builds a specifying EVALUATE ForgeState with the given
// eval_mode and eval agent model. Count is 1, Type is "eval".
func specEvaluateState(mode, model string) *ForgeState {
	return &ForgeState{
		Phase: PhaseSpecifying,
		State: StateEvaluate,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3, AgentConfig: AgentConfig{Model: model, Type: "eval", Count: 1}, EvalMode: mode},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain: "optimizer",
			BatchNumber:   1,
			CurrentSpecs: []*ActiveSpec{
				{ID: 1, Name: "Repo", Domain: "optimizer", Topic: "t", File: "optimizer/specs/repo.md", Round: 1},
			},
		},
	}
}

// planEvaluateState builds a planning EVALUATE ForgeState with the given eval_mode and agent model.
// Count is 1, Type is "eval".
func planEvaluateState(mode, model string) *ForgeState {
	return &ForgeState{
		Phase: PhasePlanning,
		State: StateEvaluate,
		Config: ForgeConfig{
			Planning: PlanningConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3, AgentConfig: AgentConfig{Model: model, Type: "eval", Count: 1}, EvalMode: mode},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{ID: 1, Name: "Service Configuration", Domain: "launcher", File: "launcher/plan.json"},
			Round:       1,
		},
	}
}

// implEvaluateState builds an implementing EVALUATE ForgeState with the given
// eval_mode and agent model, driving the in-memory plan from ORIENT through to EVALUATE.
// Count is 1, Type is "eval".
func implEvaluateState(t *testing.T, dir, mode, model string) *ForgeState {
	t.Helper()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.EvalMode = mode
	s.Config.Implementing.Eval.Model = model
	s.Config.Implementing.Eval.Type = "eval"
	s.Config.Implementing.Eval.Count = 1
	advanceImplToEvaluate(t, s, dir)
	return s
}

// TestEvalEntryActionReportMode verifies that every eval-entry state renders the
// report-mode Action: spawn-to-evaluate, run forgectl eval, and an advance line
// carrying --eval-report.
func TestEvalEntryActionReportMode(t *testing.T) {
	dir := t.TempDir()

	specState := specEvaluateState("report", "opus")
	spec := outputOf(specState, ".")
	specPath := specEvalReportPath(specState)
	if !strings.Contains(spec, "Please spawn 1 opus eval sub-agent to evaluate the spec batch.") {
		t.Errorf("specifying report spawn line missing, got:\n%s", spec)
	}
	if !strings.Contains(spec, "The sub-agent should run: forgectl eval") {
		t.Errorf("specifying report run line missing, got:\n%s", spec)
	}
	// The advance line carries the concrete report path (no literal <path>), the
	// path is also surfaced on the "must write" line, and the file-vs-text note
	// is present so the engineer does not pass report prose.
	if !strings.Contains(spec, "advance with --verdict PASS|FAIL --eval-report "+specPath) {
		t.Errorf("specifying report advance line missing concrete path %q, got:\n%s", specPath, spec)
	}
	if !strings.Contains(spec, "The sub-agent must write its report to this exact path:") {
		t.Errorf("specifying report 'must write' label line missing, got:\n%s", spec)
	}
	if !strings.Contains(spec, "--eval-report takes this file path, not the report text.") {
		t.Errorf("specifying report file-vs-text note missing, got:\n%s", spec)
	}
	if strings.Contains(spec, "--eval-report <path>") {
		t.Errorf("specifying report must not contain literal <path> placeholder, got:\n%s", spec)
	}

	planState := planEvaluateState("report", "opus")
	plan := outputOf(planState, ".")
	planPath := planEvalReportPath(planState)
	if !strings.Contains(plan, "Please spawn 1 opus eval sub-agent to evaluate the plan.") {
		t.Errorf("planning report spawn line missing, got:\n%s", plan)
	}
	if !strings.Contains(plan, "Sub-agent runs: forgectl eval") {
		t.Errorf("planning report run line missing, got:\n%s", plan)
	}
	if !strings.Contains(plan, "advance with --verdict PASS|FAIL --eval-report "+planPath) {
		t.Errorf("planning report advance line missing concrete path %q, got:\n%s", planPath, plan)
	}

	implState := implEvaluateState(t, dir, "report", "opus")
	impl := outputOf(implState, dir)
	implPath := implEvalReportPath(implState)
	if !strings.Contains(impl, "Please spawn 1 opus eval sub-agent to evaluate the implementation batch.") {
		t.Errorf("implementing report spawn line missing, got:\n%s", impl)
	}
	if !strings.Contains(impl, "advance with --eval-report "+implPath+" --verdict PASS|FAIL") {
		t.Errorf("implementing report advance line missing concrete path %q, got:\n%s", implPath, impl)
	}
	if !strings.Contains(impl, "--eval-report takes this file path, not the report text.") {
		t.Errorf("implementing report file-vs-text note missing, got:\n%s", impl)
	}
}

// TestEvalReportPathConsistentAcrossActionAndEval verifies the eval-report-contract
// invariant that the report path printed in the EVALUATE status Action (engineer's
// copy) is identical to the path printed in the forgectl eval REPORT OUTPUT
// (sub-agent's copy), for every report-mode eval state.
func TestEvalReportPathConsistentAcrossActionAndEval(t *testing.T) {
	dir := t.TempDir()

	specState := specEvaluateState("report", "opus")
	planState := planEvaluateState("report", "opus")
	implState := implEvaluateState(t, dir, "report", "opus")
	crossState := crossRefEvalState("report", 1, nil)
	reconState := reconcileEvalState("report", 1, nil)
	reState := reOutputState(StateReconcileEval, []string{"optimizer", "api"}, 1, nil)
	reState.ReverseEngineering.ReconcileRound = 2

	cases := []struct {
		name     string
		state    *ForgeState
		dir      string
		wantPath string
		render   func(w *bytes.Buffer, s *ForgeState, dir string) error
	}{
		{"specifying", specState, ".", specEvalReportPath(specState),
			func(w *bytes.Buffer, s *ForgeState, dir string) error { return PrintSpecEvalOutput(w, s, dir) }},
		{"planning", planState, ".", planEvalReportPath(planState),
			func(w *bytes.Buffer, s *ForgeState, dir string) error { return PrintEvalOutput(w, s, dir) }},
		{"implementing", implState, dir, implEvalReportPath(implState),
			func(w *bytes.Buffer, s *ForgeState, dir string) error { return PrintEvalOutput(w, s, dir) }},
		{"cross-reference", crossState, ".", crossRefEvalReportPath(crossState),
			func(w *bytes.Buffer, s *ForgeState, _ string) error { return PrintCrossRefEvalOutput(w, s) }},
		{"reconcile", reconState, ".", reconcileEvalReportPath(reconState),
			func(w *bytes.Buffer, s *ForgeState, _ string) error { return PrintReconcileEvalOutput(w, s) }},
		{"reverse-engineering", reState, ".", reverseEngineeringReconcileEvalReportPath(reState),
			func(w *bytes.Buffer, s *ForgeState, _ string) error { return PrintReverseEngineeringEvalOutput(w, s) }},
	}
	for _, c := range cases {
		// A "" wantPath would make strings.Contains pass vacuously; the path
		// must be a real, non-empty path for the consistency check to mean
		// anything.
		if c.wantPath == "" {
			t.Errorf("%s: expected a non-empty report path", c.name)
			continue
		}
		action := outputOf(c.state, c.dir)
		var eb bytes.Buffer
		if err := c.render(&eb, c.state, c.dir); err != nil {
			t.Fatalf("%s: render eval: %v", c.name, err)
		}
		eval := eb.String()
		if !strings.Contains(action, c.wantPath) {
			t.Errorf("%s: EVALUATE action missing report path %q, got:\n%s", c.name, c.wantPath, action)
		}
		if !strings.Contains(eval, c.wantPath) {
			t.Errorf("%s: eval REPORT OUTPUT missing report path %q, got:\n%s", c.name, c.wantPath, eval)
		}
	}
}

// TestEvalEntryActionDirectMode verifies the direct-mode Action across eval-entry
// states: spawn to evaluate AND correct, a staged-files note, and an advance line
// with no --eval-report.
func TestEvalEntryActionDirectMode(t *testing.T) {
	dir := t.TempDir()

	spec := outputOf(specEvaluateState("direct", "opus"), ".")
	if !strings.Contains(spec, "Please spawn 1 opus eval sub-agent to evaluate and correct the spec.") {
		t.Errorf("specifying direct spawn line missing, got:\n%s", spec)
	}
	if !strings.Contains(spec, "Spec files have been staged. Sub-agent makes corrections directly.") {
		t.Errorf("specifying direct staged note missing, got:\n%s", spec)
	}
	if !strings.Contains(spec, "Sub-agent runs: forgectl eval") {
		t.Errorf("specifying direct run line missing, got:\n%s", spec)
	}
	if strings.Contains(spec, "--eval-report") {
		t.Errorf("specifying direct must not mention --eval-report, got:\n%s", spec)
	}

	plan := outputOf(planEvaluateState("direct", "opus"), ".")
	if !strings.Contains(plan, "evaluate and correct the plan.") {
		t.Errorf("planning direct spawn line missing, got:\n%s", plan)
	}
	if !strings.Contains(plan, "Plan files have been staged. Sub-agent makes corrections directly.") {
		t.Errorf("planning direct staged note missing, got:\n%s", plan)
	}

	impl := outputOf(implEvaluateState(t, dir, "direct", "opus"), dir)
	if !strings.Contains(impl, "evaluate and correct the batch.") {
		t.Errorf("implementing direct spawn line missing, got:\n%s", impl)
	}
	if !strings.Contains(impl, "Batch files have been staged. Sub-agent makes corrections directly.") {
		t.Errorf("implementing direct staged note missing, got:\n%s", impl)
	}
	if strings.Contains(impl, "--eval-report") {
		t.Errorf("implementing direct must not mention --eval-report, got:\n%s", impl)
	}
}

// TestEvalEntryActionConversationalMode verifies the conversational-mode Action:
// spawn to evaluate (no "and correct"), no staged-files note, and an advance line
// with no --eval-report.
func TestEvalEntryActionConversationalMode(t *testing.T) {
	dir := t.TempDir()

	spec := outputOf(specEvaluateState("conversational", "opus"), ".")
	if !strings.Contains(spec, "Please spawn 1 opus eval sub-agent to evaluate the spec batch.") {
		t.Errorf("specifying conversational spawn line missing, got:\n%s", spec)
	}
	if strings.Contains(spec, "--eval-report") {
		t.Errorf("specifying conversational must not mention --eval-report, got:\n%s", spec)
	}
	if strings.Contains(spec, "have been staged") {
		t.Errorf("specifying conversational must not include a staged-files note, got:\n%s", spec)
	}
	if !strings.Contains(spec, "advance with --verdict PASS|FAIL") {
		t.Errorf("specifying conversational advance line missing, got:\n%s", spec)
	}

	plan := outputOf(planEvaluateState("conversational", "opus"), ".")
	if !strings.Contains(plan, "Please spawn 1 opus eval sub-agent to evaluate the plan.") {
		t.Errorf("planning conversational spawn line missing, got:\n%s", plan)
	}
	if strings.Contains(plan, "--eval-report") {
		t.Errorf("planning conversational must not mention --eval-report, got:\n%s", plan)
	}

	impl := outputOf(implEvaluateState(t, dir, "conversational", "opus"), dir)
	if !strings.Contains(impl, "Please spawn 1 opus eval sub-agent to evaluate the implementation batch.") {
		t.Errorf("implementing conversational spawn line missing, got:\n%s", impl)
	}
	if strings.Contains(impl, "--eval-report") {
		t.Errorf("implementing conversational must not mention --eval-report, got:\n%s", impl)
	}
}

// TestEvalEntryActionEdgeCases covers the cross-reference / reconciliation eval
// states (whose mode resolves from specifying.eval) and back-compat resolution
// (enable_eval_output:true + no eval_mode → report wording, exercising EvalModeFor).
func TestEvalEntryActionEdgeCases(t *testing.T) {
	// CROSS_REFERENCE_EVAL — direct mode resolved from specifying.eval.
	crEval := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateCrossReferenceEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval:           EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: "direct"},
				CrossReference: CrossRefConfig{MinRounds: 1, MaxRounds: 2, Eval: AgentConfig{Model: "opus", Type: "eval", Count: 1}},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain:  "optimizer",
			CrossReference: map[string]*CrossReferenceState{"optimizer": {Domain: "optimizer", Round: 1}},
			Completed:      []CompletedSpec{{ID: 1, Name: "a", Domain: "optimizer", File: "optimizer/specs/a.md"}},
		},
	}
	cr := outputOf(crEval, ".")
	if !strings.Contains(cr, "Please spawn 1 opus eval sub-agent to evaluate and correct cross-references.") {
		t.Errorf("cross-ref direct spawn line missing, got:\n%s", cr)
	}
	if !strings.Contains(cr, "Spec files have been staged. Sub-agent makes corrections directly.") {
		t.Errorf("cross-ref direct staged note missing, got:\n%s", cr)
	}

	// RECONCILE_EVAL — conversational mode resolved from specifying.eval.
	rcEval := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateReconcileEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval:           EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: "conversational"},
				Reconciliation: ReconciliationConfig{MinRounds: 0, MaxRounds: 3, AgentConfig: AgentConfig{Model: "opus", Type: "eval", Count: 1}},
			},
		},
		Specifying: &SpecifyingState{
			Reconcile: &ReconcileState{Round: 1},
			Completed: []CompletedSpec{{ID: 1, Name: "a", Domain: "optimizer", File: "optimizer/specs/a.md"}},
		},
	}
	rc := outputOf(rcEval, ".")
	if !strings.Contains(rc, "Please spawn 1 opus eval sub-agent to evaluate cross-domain reconciliation.") {
		t.Errorf("reconcile conversational spawn line missing, got:\n%s", rc)
	}
	if strings.Contains(rc, "--eval-report") {
		t.Errorf("reconcile conversational must not mention --eval-report, got:\n%s", rc)
	}

	// Back-compat: a locked session with enable_eval_output:true and no eval_mode
	// must resolve to report wording (carrying --eval-report) via EvalModeFor.
	legacy := specEvaluateState("", "opus")
	legacy.Config.Specifying.Eval.EnableEvalOutput = true
	out := outputOf(legacy, ".")
	if !strings.Contains(out, "advance with --verdict PASS|FAIL --eval-report "+specEvalReportPath(legacy)) {
		t.Errorf("legacy enable_eval_output session should resolve to report wording with concrete path, got:\n%s", out)
	}
}

// specRefineState builds a specifying REFINE ForgeState with the given eval_mode.
func specRefineState(mode string) *ForgeState {
	return &ForgeState{
		Phase: PhaseSpecifying,
		State: StateRefine,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: mode},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain: "optimizer",
			BatchNumber:   1,
			CurrentSpecs: []*ActiveSpec{
				{ID: 1, Name: "Repo", Domain: "optimizer", File: "optimizer/specs/repo.md", Round: 1},
			},
		},
	}
}

// planRefineState builds a planning REFINE ForgeState with the given eval_mode
// and last-eval verdict ("FAIL" or "PASS"; PASS below min_rounds adds a preamble).
func planRefineState(mode, verdict string) *ForgeState {
	return &ForgeState{
		Phase: PhasePlanning,
		State: StateRefine,
		Config: ForgeConfig{
			Planning: PlanningConfig{
				Eval: EvalConfig{MinRounds: 2, MaxRounds: 3, EvalMode: mode},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{ID: 1, Name: "Service Configuration", Domain: "launcher", File: "launcher/plan.json"},
			Round:       1,
			Evals:       []EvalRecord{{Round: 1, Verdict: verdict}},
		},
	}
}

// implReentryState drives an in-memory implementing plan to the IMPLEMENT
// after-eval re-entry (round 2+) under the given eval_mode and returns it.
func implReentryState(t *testing.T, dir, mode string) *ForgeState {
	t.Helper()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.EvalMode = mode
	advanceImplToEvaluate(t, s, dir)
	in := AdvanceInput{Verdict: "FAIL"}
	if mode == "report" {
		ef := filepath.Join(dir, "ef.md")
		os.WriteFile(ef, []byte("x"), 0644)
		in.EvalReport = ef
	}
	if err := Advance(s, in, dir); err != nil {
		t.Fatalf("FAIL back to IMPLEMENT: %v", err)
	}
	if s.State != StateImplement {
		t.Fatalf("expected IMPLEMENT round 2, got %s", s.State)
	}
	return s
}

// TestRefineActionReportMode verifies REFINE (specifying/planning) and the
// implementing IMPLEMENT after-eval re-entry render the report-mode guidance:
// "Study the eval file <path>".
func TestRefineActionReportMode(t *testing.T) {
	dir := t.TempDir()

	spec := outputOf(specRefineState("report"), ".")
	if !strings.Contains(spec, `Study the eval file "optimizer/specs/.eval/batch-1-r1.md"`) {
		t.Errorf("specifying report refine missing eval file line, got:\n%s", spec)
	}
	if !strings.Contains(spec, "and implement any corrections as needed.") {
		t.Errorf("specifying report refine missing follow-up line, got:\n%s", spec)
	}
	if !strings.Contains(spec, `Apply "fresh" eyes and a tightened lens when reviewing the work,`) {
		t.Errorf("specifying refine missing shared fresh-eyes line, got:\n%s", spec)
	}
	if !strings.Contains(spec, "Format:      references/spec-format.md") {
		t.Errorf("specifying report refine should keep Format/Process/Scoping, got:\n%s", spec)
	}
	if !strings.Contains(spec, "advance to continue evaluation.") {
		t.Errorf("specifying report refine should end 'advance to continue evaluation.', got:\n%s", spec)
	}

	plan := outputOf(planRefineState("report", "FAIL"), ".")
	if !strings.Contains(plan, `Study the eval file "launcher/evals/round-1.md"`) {
		t.Errorf("planning report refine missing eval file line, got:\n%s", plan)
	}

	// PASS below min_rounds prints the preamble line.
	planPass := outputOf(planRefineState("report", "PASS"), ".")
	if !strings.Contains(planPass, "Minimum evaluation rounds not met.") {
		t.Errorf("planning PASS-below-min refine missing preamble, got:\n%s", planPass)
	}
	if !strings.Contains(planPass, `Study the eval file "launcher/evals/round-1.md"`) {
		t.Errorf("planning PASS-below-min refine missing eval file line, got:\n%s", planPass)
	}

	impl := outputOf(implReentryState(t, dir, "report"), dir)
	if !strings.Contains(impl, "Study the eval file") {
		t.Errorf("implementing report re-entry missing eval file line, got:\n%s", impl)
	}
}

// TestRefineActionDirectMode verifies the direct-mode guidance across REFINE and
// the implementing re-entry: "Review unstaged changes from the evaluator (git diff)."
func TestRefineActionDirectMode(t *testing.T) {
	dir := t.TempDir()

	spec := outputOf(specRefineState("direct"), ".")
	if !strings.Contains(spec, "Review unstaged changes from the evaluator (git diff).") {
		t.Errorf("specifying direct refine missing review line, got:\n%s", spec)
	}
	if !strings.Contains(spec, "Accept, revise, or revert corrections as needed.") {
		t.Errorf("specifying direct refine missing accept/revert line, got:\n%s", spec)
	}
	if strings.Contains(spec, "Format:") {
		t.Errorf("specifying direct refine must not include Format/Process/Scoping, got:\n%s", spec)
	}
	if !strings.Contains(spec, "advance to continue.") || strings.Contains(spec, "advance to continue evaluation.") {
		t.Errorf("specifying direct refine should end 'advance to continue.', got:\n%s", spec)
	}

	plan := outputOf(planRefineState("direct", "FAIL"), ".")
	if !strings.Contains(plan, "Review unstaged changes from the evaluator (git diff).") {
		t.Errorf("planning direct refine missing review line, got:\n%s", plan)
	}

	impl := outputOf(implReentryState(t, dir, "direct"), dir)
	if !strings.Contains(impl, "Review unstaged changes from the evaluator (git diff).") {
		t.Errorf("implementing direct re-entry missing review line, got:\n%s", impl)
	}
}

// TestRefineActionConversationalMode verifies the conversational-mode guidance
// across REFINE and the implementing re-entry.
func TestRefineActionConversationalMode(t *testing.T) {
	dir := t.TempDir()

	spec := outputOf(specRefineState("conversational"), ".")
	if !strings.Contains(spec, "Make corrections based off communication with the evaluator.") {
		t.Errorf("specifying conversational refine missing line, got:\n%s", spec)
	}
	if !strings.Contains(spec, "Implement any corrections as needed.") {
		t.Errorf("specifying conversational refine missing follow-up, got:\n%s", spec)
	}
	if !strings.Contains(spec, "advance to continue evaluation.") {
		t.Errorf("specifying conversational refine should end 'advance to continue evaluation.', got:\n%s", spec)
	}

	plan := outputOf(planRefineState("conversational", "FAIL"), ".")
	if !strings.Contains(plan, "Make corrections based off communication with the evaluator.") {
		t.Errorf("planning conversational refine missing line, got:\n%s", plan)
	}

	impl := outputOf(implReentryState(t, dir, "conversational"), dir)
	if !strings.Contains(impl, "Make corrections based off communication with the evaluator.") {
		t.Errorf("implementing conversational re-entry missing line, got:\n%s", impl)
	}
}

// crossRefEvalState builds a CROSS_REFERENCE_EVAL state at the given round with
// prior evals, eval_mode resolved from specifying.eval.
func crossRefEvalState(mode string, round int, evals []EvalRecord) *ForgeState {
	return &ForgeState{
		Phase: PhaseSpecifying,
		State: StateCrossReferenceEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval:           EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: mode},
				CrossReference: CrossRefConfig{MinRounds: 1, MaxRounds: 2},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain:  "optimizer",
			CrossReference: map[string]*CrossReferenceState{"optimizer": {Domain: "optimizer", Round: round, Evals: evals}},
			Completed:      []CompletedSpec{{ID: 1, Name: "a", Domain: "optimizer", File: "optimizer/specs/a.md"}},
		},
	}
}

// reconcileEvalState builds a RECONCILE_EVAL state at the given round with prior
// evals, eval_mode resolved from specifying.eval.
func reconcileEvalState(mode string, round int, evals []EvalRecord) *ForgeState {
	return &ForgeState{
		Phase: PhaseSpecifying,
		State: StateReconcileEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval:           EvalConfig{MinRounds: 1, MaxRounds: 3, EvalMode: mode},
				Reconciliation: ReconciliationConfig{MinRounds: 0, MaxRounds: 3},
			},
		},
		Specifying: &SpecifyingState{
			Reconcile: &ReconcileState{Round: round, Evals: evals},
			Completed: []CompletedSpec{{ID: 1, Name: "a", Domain: "optimizer", File: "optimizer/specs/a.md"}},
		},
	}
}

// TestEvalContextReportModeSections verifies report-mode REPORT OUTPUT (with the
// report path) and PREVIOUS EVALUATIONS (with prior report paths) across the
// planning, specifying, cross-reference, and reconciliation eval-context output.
func TestEvalContextReportModeSections(t *testing.T) {
	// Planning round 2 with a prior FAIL report.
	ps := planEvaluateState("report", "opus")
	ps.Planning.Round = 2
	ps.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "FAIL", EvalReport: "launcher/evals/round-1.md"}}
	var pb bytes.Buffer
	if err := PrintEvalOutput(&pb, ps, "."); err != nil {
		t.Fatalf("planning eval: %v", err)
	}
	p := pb.String()
	if !strings.Contains(p, "--- PREVIOUS EVALUATIONS ---") || !strings.Contains(p, "Round 1: FAIL — launcher/evals/round-1.md") {
		t.Errorf("planning report previous-evals missing, got:\n%s", p)
	}
	if !strings.Contains(p, "--- REPORT OUTPUT ---") || !strings.Contains(p, "Write your evaluation report to this exact path") || !strings.Contains(p, "launcher/evals/round-2.md") {
		t.Errorf("planning report output missing, got:\n%s", p)
	}

	// Specifying EVALUATE round 2 with a prior FAIL report.
	ss := specEvaluateState("report", "opus")
	ss.Specifying.CurrentSpecs[0].Round = 2
	ss.Specifying.CurrentSpecs[0].Evals = []EvalRecord{{Round: 1, Verdict: "FAIL", EvalReport: "optimizer/specs/.eval/batch-1-r1.md"}}
	var sb bytes.Buffer
	if err := PrintSpecEvalOutput(&sb, ss, "."); err != nil {
		t.Fatalf("spec eval: %v", err)
	}
	sp := sb.String()
	if !strings.Contains(sp, "Round 1: FAIL — optimizer/specs/.eval/batch-1-r1.md") {
		t.Errorf("specifying report previous-evals missing, got:\n%s", sp)
	}
	if !strings.Contains(sp, "Write your evaluation report to this exact path") || !strings.Contains(sp, "optimizer/specs/.eval/batch-1-r2.md") {
		t.Errorf("specifying report output missing, got:\n%s", sp)
	}

	// Cross-reference round 1 — report output present.
	var cb bytes.Buffer
	if err := PrintCrossRefEvalOutput(&cb, crossRefEvalState("report", 1, nil)); err != nil {
		t.Fatalf("crossref eval: %v", err)
	}
	if !strings.Contains(cb.String(), "Write your evaluation report to this exact path") || !strings.Contains(cb.String(), "optimizer/specs/.eval/cross-reference-r1.md") {
		t.Errorf("crossref report output missing, got:\n%s", cb.String())
	}

	// Reconciliation round 1 — report output present.
	var rb bytes.Buffer
	if err := PrintReconcileEvalOutput(&rb, reconcileEvalState("report", 1, nil)); err != nil {
		t.Fatalf("reconcile eval: %v", err)
	}
	if !strings.Contains(rb.String(), "Write your evaluation report to this exact path") || !strings.Contains(rb.String(), "optimizer/specs/.eval/reconciliation-r1.md") {
		t.Errorf("reconcile report output missing, got:\n%s", rb.String())
	}
}

// TestEvalContextDirectModeSections verifies direct-mode REPORT OUTPUT instructs
// in-place corrections (planning/implementing/specifying/cross-ref) and PREVIOUS
// EVALUATIONS lists "(direct corrections)".
func TestEvalContextDirectModeSections(t *testing.T) {
	dir := t.TempDir()

	// Planning direct with a prior round.
	ps := planEvaluateState("direct", "opus")
	ps.Planning.Round = 2
	ps.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "FAIL", EvalReport: "launcher/evals/round-1.md"}}
	var pb bytes.Buffer
	if err := PrintEvalOutput(&pb, ps, "."); err != nil {
		t.Fatalf("planning eval: %v", err)
	}
	p := pb.String()
	if !strings.Contains(p, "Round 1: FAIL — (direct corrections)") {
		t.Errorf("planning direct previous-evals should show (direct corrections), got:\n%s", p)
	}
	if !strings.Contains(p, "Make corrections directly to the plan files.") {
		t.Errorf("planning direct report output missing, got:\n%s", p)
	}
	if strings.Contains(p, "Write your evaluation report to this exact path") {
		t.Errorf("planning direct must not print a report path, got:\n%s", p)
	}

	// Implementing direct (round 1, no priors).
	var ib bytes.Buffer
	if err := PrintEvalOutput(&ib, implEvaluateState(t, dir, "direct", "opus"), dir); err != nil {
		t.Fatalf("implementing eval: %v", err)
	}
	if !strings.Contains(ib.String(), "Make corrections directly to the batch files.") {
		t.Errorf("implementing direct report output missing, got:\n%s", ib.String())
	}

	// Specifying direct.
	ss := specEvaluateState("direct", "opus")
	var sb bytes.Buffer
	if err := PrintSpecEvalOutput(&sb, ss, "."); err != nil {
		t.Fatalf("spec eval: %v", err)
	}
	if !strings.Contains(sb.String(), "Make corrections directly to the spec files.") {
		t.Errorf("specifying direct report output missing, got:\n%s", sb.String())
	}

	// Cross-reference direct.
	var cb bytes.Buffer
	if err := PrintCrossRefEvalOutput(&cb, crossRefEvalState("direct", 1, nil)); err != nil {
		t.Fatalf("crossref eval: %v", err)
	}
	if !strings.Contains(cb.String(), "Make corrections directly to the spec files.") {
		t.Errorf("crossref direct report output missing, got:\n%s", cb.String())
	}
}

// TestEvalContextConversationalModeSections verifies conversational mode omits
// both --- REPORT OUTPUT --- and --- PREVIOUS EVALUATIONS --- everywhere.
func TestEvalContextConversationalModeSections(t *testing.T) {
	dir := t.TempDir()

	ps := planEvaluateState("conversational", "opus")
	ps.Planning.Round = 2
	ps.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "FAIL", EvalReport: "launcher/evals/round-1.md"}}
	var pb bytes.Buffer
	PrintEvalOutput(&pb, ps, ".")
	if strings.Contains(pb.String(), "--- REPORT OUTPUT ---") || strings.Contains(pb.String(), "--- PREVIOUS EVALUATIONS ---") {
		t.Errorf("planning conversational should omit both sections, got:\n%s", pb.String())
	}

	var ib bytes.Buffer
	PrintEvalOutput(&ib, implEvaluateState(t, dir, "conversational", "opus"), dir)
	if strings.Contains(ib.String(), "--- REPORT OUTPUT ---") {
		t.Errorf("implementing conversational should omit report output, got:\n%s", ib.String())
	}

	var cb bytes.Buffer
	PrintCrossRefEvalOutput(&cb, crossRefEvalState("conversational", 2, []EvalRecord{{Round: 1, Verdict: "FAIL"}}))
	if strings.Contains(cb.String(), "--- REPORT OUTPUT ---") || strings.Contains(cb.String(), "--- PREVIOUS EVALUATIONS ---") {
		t.Errorf("crossref conversational should omit both sections, got:\n%s", cb.String())
	}

	var rb bytes.Buffer
	PrintReconcileEvalOutput(&rb, reconcileEvalState("conversational", 2, []EvalRecord{{Round: 1, Verdict: "FAIL"}}))
	if strings.Contains(rb.String(), "--- REPORT OUTPUT ---") || strings.Contains(rb.String(), "--- PREVIOUS EVALUATIONS ---") {
		t.Errorf("reconcile conversational should omit both sections, got:\n%s", rb.String())
	}
}

// TestEvalContextReconcileDirectOmitsReportOutput verifies the reconciliation
// special case: direct mode omits --- REPORT OUTPUT --- (it works on staged
// changes) but still backfills --- PREVIOUS EVALUATIONS --- on later rounds.
func TestEvalContextReconcileDirectOmitsReportOutput(t *testing.T) {
	s := reconcileEvalState("direct", 2, []EvalRecord{{Round: 1, Verdict: "FAIL"}})
	var rb bytes.Buffer
	if err := PrintReconcileEvalOutput(&rb, s); err != nil {
		t.Fatalf("reconcile eval: %v", err)
	}
	out := rb.String()
	if strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("reconcile direct must omit REPORT OUTPUT, got:\n%s", out)
	}
	if !strings.Contains(out, "--- PREVIOUS EVALUATIONS ---") || !strings.Contains(out, "Round 1: FAIL — (direct corrections)") {
		t.Errorf("reconcile direct should backfill PREVIOUS EVALUATIONS with (direct corrections), got:\n%s", out)
	}
}

// TestEvalContextPreviousEvalsMultipleRounds is an edge case: multiple prior
// rounds render in order, with report paths in report mode and "(direct
// corrections)" in direct mode (mixed verdicts).
func TestEvalContextPreviousEvalsMultipleRounds(t *testing.T) {
	evals := []EvalRecord{
		{Round: 1, Verdict: "FAIL", EvalReport: "optimizer/specs/.eval/cross-reference-r1.md"},
		{Round: 2, Verdict: "FAIL", EvalReport: "optimizer/specs/.eval/cross-reference-r2.md"},
	}

	var rep bytes.Buffer
	if err := PrintCrossRefEvalOutput(&rep, crossRefEvalState("report", 3, evals)); err != nil {
		t.Fatalf("crossref report: %v", err)
	}
	r := rep.String()
	if !strings.Contains(r, "Round 1: FAIL — optimizer/specs/.eval/cross-reference-r1.md") ||
		!strings.Contains(r, "Round 2: FAIL — optimizer/specs/.eval/cross-reference-r2.md") {
		t.Errorf("report mode should list each prior round with its report path, got:\n%s", r)
	}
	// Ordering: round 1 must appear before round 2.
	if strings.Index(r, "Round 1:") > strings.Index(r, "Round 2:") {
		t.Errorf("previous evaluations should be in round order, got:\n%s", r)
	}

	var dir bytes.Buffer
	if err := PrintCrossRefEvalOutput(&dir, crossRefEvalState("direct", 3, evals)); err != nil {
		t.Fatalf("crossref direct: %v", err)
	}
	d := dir.String()
	if !strings.Contains(d, "Round 1: FAIL — (direct corrections)") || !strings.Contains(d, "Round 2: FAIL — (direct corrections)") {
		t.Errorf("direct mode should list each prior round as (direct corrections), got:\n%s", d)
	}
	if strings.Contains(d, "cross-reference-r1.md") {
		t.Errorf("direct mode must not leak report paths into previous evals, got:\n%s", d)
	}
}

// TestEvalOutputOutsideValidStatesReturnsError verifies that eval command outside
// valid states returns an error naming the current state.
func TestEvalOutputOutsideValidStatesReturnsError(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateDraft,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
	}

	var buf bytes.Buffer
	err := PrintEvalOutput(&buf, s, ".")
	if err == nil {
		t.Fatal("expected error when calling PrintEvalOutput in non-evaluation state")
	}
	if !strings.Contains(err.Error(), string(StateDraft)) {
		t.Errorf("expected error to mention current state %q, got: %v", StateDraft, err)
	}
}

// TestOutputDoneDomainVariantWhenPlansRemain verifies that the DONE output shows
// "Domain complete. Advance to continue to next domain." when plans remain.
func TestOutputDoneDomainVariantWhenPlansRemain(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = false
	// Add a plan queue entry to simulate remaining domains.
	s.Implementing.PlanQueue = []PlanQueueEntry{{Name: "Next Plan", Domain: "next", File: "next/plan.json"}}
	s.Implementing.CurrentPlanDomain = "test"
	s.State = StateDone

	out := outputOf(s, dir)
	if !strings.Contains(out, "Domain complete.") {
		t.Errorf("expected 'Domain complete.' in DONE output with plans remaining, got:\n%s", out)
	}
	if !strings.Contains(out, "Advance to continue to next domain.") {
		t.Errorf("expected 'Advance to continue to next domain.' in DONE output, got:\n%s", out)
	}
}

// reOutputState builds a reverse_engineering ForgeState with default config in
// the given state, domain index, and queue.
func reOutputState(st StateName, domains []string, domainIndex int, queue []REQueueEntry) *ForgeState {
	re := NewReverseEngineeringState("auth refactor", domains)
	re.DomainIndex = domainIndex
	re.Queue = queue
	return &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              st,
		Config:             DefaultForgeConfig(),
		StartedAtPhase:     PhaseReverseEngineering,
		ReverseEngineering: re,
	}
}

// Functional: ORIENT lists every domain with its 1-based index and the domain
// processing order.
func TestREOutputOrientListsDomainsAndOrder(t *testing.T) {
	dir := t.TempDir()
	s := reOutputState(StateOrient, []string{"optimizer", "api", "portal"}, 1, nil)

	out := outputOf(s, dir)
	if !strings.Contains(out, "Phase: reverse_engineering") {
		t.Errorf("expected RE phase header, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer (1/3)") || !strings.Contains(out, "portal (3/3)") {
		t.Errorf("expected indexed domain list, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer → api → portal") {
		t.Errorf("expected domain order arrow line, got:\n%s", out)
	}
	if !strings.Contains(out, "Advance to begin SURVEY on domain: optimizer") {
		t.Errorf("expected SURVEY advance hint, got:\n%s", out)
	}
}

// Functional: SURVEY and GAP_ANALYSIS interpolate the configured sub-agent
// counts/models/types, and GAP_ANALYSIS includes topic-of-concern rules.
func TestREOutputSurveyAndGapAnalysisConfig(t *testing.T) {
	dir := t.TempDir()

	survey := outputOf(reOutputState(StateSurvey, []string{"optimizer"}, 1, nil), dir)
	if !strings.Contains(survey, "Please spawn 2 haiku explorer sub-agents") {
		t.Errorf("SURVEY should reflect configured sub-agents, got:\n%s", survey)
	}
	if !strings.Contains(survey, "optimizer/specs/") {
		t.Errorf("SURVEY should scope to the domain specs dir, got:\n%s", survey)
	}

	gap := outputOf(reOutputState(StateGapAnalysis, []string{"optimizer"}, 1, nil), dir)
	if !strings.Contains(gap, "Please spawn 5 sonnet explorer sub-agents") {
		t.Errorf("GAP_ANALYSIS should reflect configured sub-agents, got:\n%s", gap)
	}
	if !strings.Contains(gap, "Must not contain \"and\" conjoining unrelated capabilities") {
		t.Errorf("GAP_ANALYSIS should include topic-of-concern rules, got:\n%s", gap)
	}
}

// Functional: EXECUTE_REVERSE_ENGINEER renders the current item's index, domain,
// spec, action, target file, topic, code_search_roots (domain-relative, with the
// root-of-core-code definition), and the configured execute sub-agents.
func TestREOutputExecuteItem(t *testing.T) {
	dir := t.TempDir()
	queue := []REQueueEntry{
		{Name: "Repository Loading", Domain: "optimizer", Topic: "loads a repo", File: "specs/repo.md", Action: "create", CodeSearchRoots: []string{"src/repo/", "src/config/"}, DependsOn: []string{}},
	}
	s := reOutputState(StateExecuteReverseEngineer, []string{"optimizer"}, 1, queue)
	s.ReverseEngineering.ExecuteItemIndex = 1

	out := outputOf(s, dir)
	for _, want := range []string{
		"EXECUTE_REVERSE_ENGINEER",
		"Item: 1/1",
		"Spec: Repository Loading  (create)",
		"Target file: optimizer/specs/repo.md",
		"Topic of concern: \"loads a repo\"",
		"- optimizer/src/repo/",
		"- optimizer/src/config/",
		"root of the core",
		"Please spawn 3 haiku explorer sub-agents",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("EXECUTE output missing %q, got:\n%s", want, out)
		}
	}
}

// Functional: RECONCILE lists the current domain's specs with depends_on, and
// RECONCILE_EVAL instructs running forgectl eval with the configured sub-agents.
func TestREOutputReconcileAndEval(t *testing.T) {
	dir := t.TempDir()
	queue := []REQueueEntry{
		{Name: "A", Domain: "optimizer", File: "specs/a.md", Action: "create", DependsOn: []string{"B"}},
		{Name: "B", Domain: "optimizer", File: "specs/b.md", Action: "update", DependsOn: []string{}},
		{Name: "C", Domain: "api", File: "specs/c.md", Action: "create", DependsOn: []string{}},
	}
	rec := reOutputState(StateReconcile, []string{"optimizer", "api"}, 1, queue)
	rec.ReverseEngineering.ReconcileRound = 1
	out := outputOf(rec, dir)
	if !strings.Contains(out, "optimizer/specs/a.md  (create)") || !strings.Contains(out, "depends_on: [B]") {
		t.Errorf("RECONCILE should list domain specs with depends_on, got:\n%s", out)
	}
	if strings.Contains(out, "api/specs/c.md") {
		t.Errorf("RECONCILE should not list other-domain specs, got:\n%s", out)
	}

	ev := reOutputState(StateReconcileEval, []string{"optimizer", "api"}, 1, queue)
	ev.ReverseEngineering.ReconcileRound = 2
	evOut := outputOf(ev, dir)
	if !strings.Contains(evOut, "Round: 2/3") {
		t.Errorf("RECONCILE_EVAL should show round/max, got:\n%s", evOut)
	}
	if !strings.Contains(evOut, "forgectl eval") {
		t.Errorf("RECONCILE_EVAL should instruct running forgectl eval, got:\n%s", evOut)
	}
	if !strings.Contains(evOut, "Please spawn 1 opus general-purpose sub-agent") {
		t.Errorf("RECONCILE_EVAL should reflect configured eval sub-agents, got:\n%s", evOut)
	}
	rePath := filepath.Join("optimizer", "specs", ".eval", "reconciliation-r2.md")
	if !strings.Contains(evOut, rePath) {
		t.Errorf("RECONCILE_EVAL should show the report path, got:\n%s", evOut)
	}
	// The Action carries the concrete path (no literal <path>) and the
	// file-vs-text note, per the eval-report contract.
	if !strings.Contains(evOut, "must write the report to this exact path") {
		t.Errorf("RECONCILE_EVAL action should surface the exact report path, got:\n%s", evOut)
	}
	if !strings.Contains(evOut, "--eval-report "+rePath) {
		t.Errorf("RECONCILE_EVAL action advance line should carry the concrete path, got:\n%s", evOut)
	}
	if strings.Contains(evOut, "--eval-report <path>") {
		t.Errorf("RECONCILE_EVAL action must not contain literal <path>, got:\n%s", evOut)
	}

	// The sub-agent's forgectl eval REPORT OUTPUT uses the imperative create-the-
	// file wording and names the same path.
	var reb bytes.Buffer
	if err := PrintReverseEngineeringEvalOutput(&reb, ev); err != nil {
		t.Fatalf("PrintReverseEngineeringEvalOutput: %v", err)
	}
	reportOut := reb.String()
	if !strings.Contains(reportOut, "Write your evaluation report to this exact path (create the file") {
		t.Errorf("RE eval REPORT OUTPUT should use imperative create-the-file wording, got:\n%s", reportOut)
	}
	if !strings.Contains(reportOut, rePath) {
		t.Errorf("RE eval REPORT OUTPUT should name the report path, got:\n%s", reportOut)
	}
	if !strings.Contains(reportOut, "your final message must be only this path and the verdict") {
		t.Errorf("RE eval REPORT OUTPUT should state the path+verdict return contract, got:\n%s", reportOut)
	}
}

// Functional: POST_REVERSE_ENGINEER emits the STOP / clear-context message and
// points to the next item (mid-loop) or to RECONCILE (last item).
func TestREOutputPostReverseEngineer(t *testing.T) {
	dir := t.TempDir()
	queue := []REQueueEntry{
		{Name: "One", Domain: "optimizer", File: "specs/one.md", Action: "create"},
		{Name: "Two", Domain: "optimizer", File: "specs/two.md", Action: "create"},
	}

	// Mid-loop: item 1 of 2 → points to the next item.
	mid := reOutputState(StatePostReverseEngineer, []string{"optimizer"}, 1, queue)
	mid.ReverseEngineering.ExecuteItemIndex = 1
	midOut := outputOf(mid, dir)
	if !strings.Contains(midOut, "STOP ensure you have created the specification that you need,") {
		t.Errorf("POST should emit the STOP message, got:\n%s", midOut)
	}
	if !strings.Contains(midOut, "clear your context window for the next iteration.") {
		t.Errorf("POST should emit the clear-context message, got:\n%s", midOut)
	}
	if !strings.Contains(midOut, "continue with item 2/2") {
		t.Errorf("POST mid-loop should point to the next item, got:\n%s", midOut)
	}

	// Last item: item 2 of 2 → points to RECONCILE.
	last := reOutputState(StatePostReverseEngineer, []string{"optimizer"}, 1, queue)
	last.ReverseEngineering.ExecuteItemIndex = 2
	lastOut := outputOf(last, dir)
	if !strings.Contains(lastOut, "proceed to RECONCILE") {
		t.Errorf("POST on last item should point to RECONCILE, got:\n%s", lastOut)
	}
}

// Edge case: SURVEY notes when the domain has no specs/ directory, and omits the
// note once the directory exists.
func TestREOutputSurveyNotesMissingSpecsDir(t *testing.T) {
	dir := t.TempDir()
	s := reOutputState(StateSurvey, []string{"optimizer"}, 1, nil)

	// No optimizer/specs/ yet — the absence must be noted.
	missing := outputOf(s, dir)
	if !strings.Contains(missing, "optimizer/specs/ does not exist") {
		t.Errorf("SURVEY should note the missing specs dir, got:\n%s", missing)
	}

	// Create the directory — the note must disappear.
	if err := os.MkdirAll(filepath.Join(dir, "optimizer", "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	present := outputOf(s, dir)
	if strings.Contains(present, "does not exist") {
		t.Errorf("SURVEY should not note absence once specs/ exists, got:\n%s", present)
	}
}

// Edge case: QUEUE shows the "write" variant on the first advance (no stored
// hash) and the "add entries" variant on subsequent advances (hash present);
// RECONCILE_ADVANCE shows the next-domain vs DONE variant by domain position.
func TestREOutputQueueAndReconcileAdvanceVariants(t *testing.T) {
	dir := t.TempDir()

	// First QUEUE advance — no stored hash.
	first := reOutputState(StateQueue, []string{"optimizer", "api"}, 1, nil)
	firstOut := outputOf(first, dir)
	if !strings.Contains(firstOut, "Write the reverse engineering queue file") {
		t.Errorf("first QUEUE should show the write variant, got:\n%s", firstOut)
	}

	// Subsequent QUEUE advance — hash already recorded.
	sub := reOutputState(StateQueue, []string{"optimizer", "api"}, 2, nil)
	sub.ReverseEngineering.QueueContentHash = "deadbeef"
	subOut := outputOf(sub, dir)
	if !strings.Contains(subOut, "Add entries for domain api to the existing queue file.") {
		t.Errorf("subsequent QUEUE should show the add-entries variant, got:\n%s", subOut)
	}

	// RECONCILE_ADVANCE with a domain remaining → next domain.
	next := reOutputState(StateReconcileAdvance, []string{"optimizer", "api"}, 1, nil)
	nextOut := outputOf(next, dir)
	if !strings.Contains(nextOut, "Next: RECONCILE for domain api (2/2)") {
		t.Errorf("RECONCILE_ADVANCE should point to the next domain, got:\n%s", nextOut)
	}

	// RECONCILE_ADVANCE on the last domain → DONE.
	last := reOutputState(StateReconcileAdvance, []string{"optimizer", "api"}, 2, nil)
	lastOut := outputOf(last, dir)
	if !strings.Contains(lastOut, "All domains reconciled. Advancing to DONE.") {
		t.Errorf("RECONCILE_ADVANCE on last domain should advance to DONE, got:\n%s", lastOut)
	}
}

// uiOutputState builds a ui_implementing ForgeState with a 2-item L0 plan on
// disk at ui/plan.json and a current batch over both items, ready to have its
// State and batch counters set by each test.
func uiOutputState(t *testing.T, dir string) *ForgeState {
	t.Helper()
	planDir := filepath.Join(dir, "ui")
	os.MkdirAll(planDir, 0755)
	plan := PlanJSON{
		Context: PlanContext{Domain: "portal", Module: "portal"},
		Layers: []PlanLayerDef{
			{ID: "L0", Name: "Shell", Items: []string{"shell.layout", "shell.theme"}},
		},
		Items: []PlanItem{
			{ID: "shell.layout", Name: "App shell", Description: "shell", Passes: "done", Tests: []PlanTest{{Category: "functional", Description: "x"}}},
			{ID: "shell.theme", Name: "Theme tokens", Description: "theme", Passes: "done", Tests: []PlanTest{{Category: "functional", Description: "x"}}},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(filepath.Join(planDir, "plan.json"), data, 0644)

	cfg := DefaultForgeConfig()
	cfg.UIImplementing.App.LaunchCommand = "npm run dev"
	cfg.UIImplementing.App.URL = "http://localhost:5173"
	cfg.UIImplementing.E2E.TestCommand = "npm run e2e"
	cfg.UIImplementing.E2E.TestDir = "e2e/"

	return &ForgeState{
		Phase:  PhaseUIImplementing,
		State:  StateQATest,
		Config: cfg,
		Planning: &PlanningState{CurrentPlan: &ActivePlan{
			ID: 1, Name: "Portal Dashboard", Domain: "portal", File: "ui/plan.json",
		}},
		UIImplementing: &UIImplementingState{
			CurrentLayer:      &LayerRef{ID: "L0", Name: "Shell"},
			BatchNumber:       1,
			CurrentPlanFile:   "ui/plan.json",
			CurrentPlanDomain: "portal",
			CurrentBatch: &UIBatchState{
				Items:            []string{"shell.layout", "shell.theme"},
				CurrentItemIndex: 0,
			},
		},
	}
}

// Functional: QA_TEST output shows Phase ui_implementing, Loop qa, the App
// launch/url, and the step-list Steps path; UI_REFINE shows the QA report path
// and the refine action.
func TestOutputUIQATestAndRefine(t *testing.T) {
	dir := t.TempDir()

	s := uiOutputState(t, dir)
	s.State = StateQATest
	s.UIImplementing.CurrentBatch.QARound = 1
	qa := outputOf(s, dir)
	for _, want := range []string{
		"Phase:    ui_implementing",
		"Loop:     qa",
		`launch="npm run dev"`,
		"url=http://localhost:5173",
		filepath.Join("ui", "qa", "batch-1-steps.json"),
	} {
		if !strings.Contains(qa, want) {
			t.Errorf("QA_TEST output missing %q, got:\n%s", want, qa)
		}
	}

	s2 := uiOutputState(t, dir)
	s2.State = StateUIRefine
	s2.UIImplementing.CurrentBatch.QARound = 1
	s2.UIImplementing.CurrentBatch.QAEvals = []EvalRecord{
		{Round: 1, Verdict: "FAIL", EvalReport: "ui/qa/batch-1-round-1.md"},
	}
	ref := outputOf(s2, dir)
	if !strings.Contains(ref, "ui/qa/batch-1-round-1.md") {
		t.Errorf("UI_REFINE output missing QA report path, got:\n%s", ref)
	}
	if !strings.Contains(ref, "iterate on UI placement") {
		t.Errorf("UI_REFINE output missing refine action, got:\n%s", ref)
	}
	if !strings.Contains(ref, "FAIL recorded for QA round 1") {
		t.Errorf("UI_REFINE output missing the FAIL note, got:\n%s", ref)
	}
}

// Functional: when the current batch has handed-off artifacts, the output renders
// a Review: line listing each artifact before the Action line.
func TestOutputUIReviewLine(t *testing.T) {
	dir := t.TempDir()
	s := uiOutputState(t, dir)
	s.State = StateQATest
	s.UIImplementing.CurrentBatch.QARound = 1
	s.UIImplementing.CurrentBatch.HandedOffArtifacts = []string{
		"ui/qa/batch-1-round-1.md",
		"ui/qa/batch-1-steps.json",
	}
	out := outputOf(s, dir)

	reviewIdx := strings.Index(out, "Review:")
	actionIdx := strings.Index(out, "Action:")
	if reviewIdx < 0 {
		t.Fatalf("expected a Review: line, got:\n%s", out)
	}
	if actionIdx < 0 || reviewIdx > actionIdx {
		t.Errorf("Review: must appear before Action:, got:\n%s", out)
	}
	for _, want := range []string{"ui/qa/batch-1-round-1.md", "ui/qa/batch-1-steps.json"} {
		if !strings.Contains(out, want) {
			t.Errorf("Review block missing artifact %q, got:\n%s", want, out)
		}
	}
}

// Functional: COMMIT marks items passed when no loop force-accepted, and failed
// (naming the force-accepted loop and rounds) when one did; the DONE summary
// reports per-loop round totals.
func TestOutputUICommitAndDoneSummary(t *testing.T) {
	dir := t.TempDir()

	// COMMIT — clean pass.
	s := uiOutputState(t, dir)
	s.State = StateCommit
	setItemPassesOnDisk(t, dir, "ui/plan.json", map[string]string{"shell.layout": "passed", "shell.theme": "passed"})
	out := outputOf(s, dir)
	if !strings.Contains(out, "[shell.layout] passed") {
		t.Errorf("COMMIT clean-pass output should mark items passed, got:\n%s", out)
	}

	// COMMIT — e2e force-accept.
	s2 := uiOutputState(t, dir)
	s2.State = StateCommit
	s2.UIImplementing.CurrentBatch.E2EForceAccepted = true
	setItemPassesOnDisk(t, dir, "ui/plan.json", map[string]string{"shell.layout": "failed", "shell.theme": "failed"})
	out2 := outputOf(s2, dir)
	if !strings.Contains(out2, "failed (e2e force-accept, 3/3 rounds)") {
		t.Errorf("COMMIT force-accept output should name the loop and rounds, got:\n%s", out2)
	}

	// DONE — per-loop round totals.
	s3 := uiOutputState(t, dir)
	s3.State = StateDone
	setItemPassesOnDisk(t, dir, "ui/plan.json", map[string]string{"shell.layout": "passed", "shell.theme": "passed"})
	s3.UIImplementing.LayerHistory = []UILayerHistory{
		{LayerID: "L0", Batches: []UIBatchHistory{
			{BatchNumber: 1, Items: []string{"shell.layout", "shell.theme"}, EvalRounds: 2, QARounds: 1, E2ERounds: 3},
		}},
	}
	out3 := outputOf(s3, dir)
	if !strings.Contains(out3, "Rounds:    code 2, qa 1, e2e 3 (across 1 batches)") {
		t.Errorf("DONE summary should report per-loop round totals, got:\n%s", out3)
	}
}

// setItemPassesOnDisk rewrites the plan.json at planFile, setting each named
// item's passes field, so output that reloads the plan sees terminal statuses.
func setItemPassesOnDisk(t *testing.T, dir, planFile string, passes map[string]string) {
	t.Helper()
	full := filepath.Join(dir, planFile)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	var plan PlanJSON
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	for i := range plan.Items {
		if p, ok := passes[plan.Items[i].ID]; ok {
			plan.Items[i].Passes = p
		}
	}
	out, _ := json.Marshal(plan)
	os.WriteFile(full, out, 0644)
}

// evalOf runs PrintEvalOutput and returns the result as a string.
func evalOf(t *testing.T, s *ForgeState, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := PrintEvalOutput(&buf, s, dir); err != nil {
		t.Fatalf("PrintEvalOutput: %v", err)
	}
	return buf.String()
}

// Functional: PrintUIQAEvalOutput embeds the QA prompt and shows the APPLICATION
// URL, the STEP LIST OUTPUT path, and a HANDOFF section in every eval_mode.
func TestEvalOutputUIQAAllModes(t *testing.T) {
	dir := t.TempDir()
	stepList := filepath.Join("ui", "qa", "batch-1-steps.json")

	for _, mode := range []string{"report", "direct", "conversational"} {
		t.Run(mode, func(t *testing.T) {
			s := uiOutputState(t, dir)
			s.State = StateQATest
			s.Config.UIImplementing.QA.EvalMode = mode
			s.UIImplementing.CurrentBatch.QARound = 1
			out := evalOf(t, s, dir)

			for _, want := range []string{
				"# UI QA Evaluation Prompt",
				"URL:           http://localhost:5173",
				"--- STEP LIST OUTPUT ---",
				stepList,
				"--- HANDOFF ---",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("mode %s: QA eval output missing %q, got:\n%s", mode, want, out)
				}
			}
			// REPORT OUTPUT is present in report/direct, omitted in conversational.
			hasReport := strings.Contains(out, "--- REPORT OUTPUT ---")
			if mode == "conversational" && hasReport {
				t.Errorf("conversational mode should omit REPORT OUTPUT, got:\n%s", out)
			}
			if mode != "conversational" && !hasReport {
				t.Errorf("mode %s should include REPORT OUTPUT, got:\n%s", mode, out)
			}
		})
	}
}

// Functional: PrintUIE2EEvalOutput embeds the e2e prompt and shows the E2E SUITE
// step-list path, test command, and test dir; REPORT OUTPUT and HANDOFF appear
// only in report mode.
func TestEvalOutputUIE2EReportVsOther(t *testing.T) {
	dir := t.TempDir()
	stepList := filepath.Join("ui", "qa", "batch-1-steps.json")

	for _, mode := range []string{"report", "direct", "conversational"} {
		t.Run(mode, func(t *testing.T) {
			s := uiOutputState(t, dir)
			s.State = StateE2EVerify
			s.Config.UIImplementing.E2E.EvalMode = mode
			s.UIImplementing.CurrentBatch.E2ERound = 1
			out := evalOf(t, s, dir)

			for _, want := range []string{
				"# UI E2E Verification Prompt",
				"--- E2E SUITE ---",
				stepList,
				"Test command: npm run e2e",
				"Test dir:     e2e/",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("mode %s: e2e eval output missing %q, got:\n%s", mode, want, out)
				}
			}
			// Match concrete rendered section content: the embedded prompt itself
			// mentions the --- HANDOFF --- header, so header markers are unreliable.
			hasReport := strings.Contains(out, "Write your e2e verification report to:")
			hasHandoff := strings.Contains(out, "When finished, register your report with:")
			if mode == "report" {
				if !hasReport || !hasHandoff {
					t.Errorf("report mode must include the REPORT OUTPUT and HANDOFF sections, got:\n%s", out)
				}
			} else if hasReport || hasHandoff {
				t.Errorf("mode %s must omit the REPORT OUTPUT and HANDOFF sections, got:\n%s", mode, out)
			}
		})
	}
}

// Edge case: round 2+ QA and e2e eval output include a PREVIOUS EVALUATIONS
// section listing prior verdicts and report paths.
func TestEvalOutputUIPreviousEvaluations(t *testing.T) {
	dir := t.TempDir()

	qa := uiOutputState(t, dir)
	qa.State = StateQATest
	qa.UIImplementing.CurrentBatch.QARound = 2
	qa.UIImplementing.CurrentBatch.QAEvals = []EvalRecord{
		{Round: 1, Verdict: "FAIL", EvalReport: "ui/qa/batch-1-round-1.md"},
	}
	qaOut := evalOf(t, qa, dir)
	if !strings.Contains(qaOut, "--- PREVIOUS EVALUATIONS ---") {
		t.Errorf("round 2 QA eval should include PREVIOUS EVALUATIONS, got:\n%s", qaOut)
	}
	if !strings.Contains(qaOut, "Round 1: FAIL — ui/qa/batch-1-round-1.md") {
		t.Errorf("PREVIOUS EVALUATIONS should list prior verdict + report, got:\n%s", qaOut)
	}

	e2e := uiOutputState(t, dir)
	e2e.State = StateE2EVerify
	e2e.UIImplementing.CurrentBatch.E2ERound = 2
	e2e.UIImplementing.CurrentBatch.E2EEvals = []EvalRecord{
		{Round: 1, Verdict: "FAIL", EvalReport: "ui/e2e/batch-1-round-1.md"},
	}
	e2eOut := evalOf(t, e2e, dir)
	if !strings.Contains(e2eOut, "--- PREVIOUS EVALUATIONS ---") {
		t.Errorf("round 2 e2e eval should include PREVIOUS EVALUATIONS, got:\n%s", e2eOut)
	}
}

// Rejection: eval in a non-evaluator ui_implementing state (e.g. UI_REFINE) is
// rejected naming the current state and phase.
func TestEvalOutputUIRejectsNonEvaluatorState(t *testing.T) {
	dir := t.TempDir()
	s := uiOutputState(t, dir)
	s.State = StateUIRefine
	var buf bytes.Buffer
	err := PrintEvalOutput(&buf, s, dir)
	if err == nil {
		t.Fatal("expected error for eval in UI_REFINE, got nil")
	}
	if !strings.Contains(err.Error(), "UI_REFINE") || !strings.Contains(err.Error(), "ui_implementing") {
		t.Errorf("error should name the state and phase, got: %v", err)
	}
}
