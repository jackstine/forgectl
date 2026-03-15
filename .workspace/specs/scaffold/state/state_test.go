package state

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func seedState(t *testing.T, dir string, s *ScaffoldState) {
	t.Helper()
	if err := Save(dir, s); err != nil {
		t.Fatalf("seeding state: %v", err)
	}
}

func twoSpecQueue() []QueueSpec {
	return []QueueSpec{
		{
			Name:            "Config Models",
			Domain:          "optimizer",
			Topic:           "The optimizer defines structured config schemas",
			File:            "optimizer/specs/configuration-models.md",
			PlanningSources: []string{"plan1.md"},
			DependsOn:       []string{},
		},
		{
			Name:            "Repository Loading",
			Domain:          "optimizer",
			Topic:           "The optimizer clones or locates a repository",
			File:            "optimizer/specs/repository-loading.md",
			PlanningSources: []string{"plan2.md"},
			DependsOn:       []string{"Config Models"},
		},
	}
}

func adv(file, verdict string) AdvanceInput {
	return AdvanceInput{File: file, Verdict: verdict}
}

func advWithDef(verdict string, defs []string) AdvanceInput {
	return AdvanceInput{Verdict: verdict, Deficiencies: defs}
}

func advWithFixed(fixed string) AdvanceInput {
	return AdvanceInput{Fixed: fixed}
}

// --- Save / Load round-trip ---

func TestSaveAndLoad(t *testing.T) {
	dir := tempDir(t)
	s := NewState(1, 3, true, twoSpecQueue())
	seedState(t, dir, s)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.MaxRounds != 3 {
		t.Errorf("max_rounds: got %d, want 3", loaded.MaxRounds)
	}
	if loaded.MinRounds != 1 {
		t.Errorf("min_rounds: got %d, want 1", loaded.MinRounds)
	}
	if !loaded.UserGuided {
		t.Error("user_guided: got false, want true")
	}
	if loaded.State != PhaseOrient {
		t.Errorf("state: got %s, want ORIENT", loaded.State)
	}
	if len(loaded.Queue) != 2 {
		t.Errorf("queue length: got %d, want 2", len(loaded.Queue))
	}
}

func TestLoad_NoFile(t *testing.T) {
	dir := tempDir(t)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing state file")
	}
}

func TestLoad_CorruptJSON(t *testing.T) {
	dir := tempDir(t)
	os.WriteFile(filepath.Join(dir, StateFileName), []byte("{bad json"), 0644)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestLoad_InvalidState(t *testing.T) {
	dir := tempDir(t)
	os.WriteFile(filepath.Join(dir, StateFileName), []byte(`{"state":"BOGUS","queue":[],"completed":[]}`), 0644)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
}

// --- Full lifecycle: single spec, PASS ---

func TestFullLifecycle_SingleSpec_Pass(t *testing.T) {
	s := NewState(1, 1, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "PASS"), PhaseAccept)
	assertAdvance(t, s, adv("", ""), PhaseDone)

	if len(s.Completed) != 1 {
		t.Fatalf("completed: got %d, want 1", len(s.Completed))
	}
	if s.Completed[0].RoundsTaken != 1 {
		t.Errorf("rounds_taken: got %d, want 1", s.Completed[0].RoundsTaken)
	}
}

// --- Full lifecycle: two specs ---

func TestFullLifecycle_TwoSpecs(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())

	// Spec 1
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "PASS"), PhaseAccept)
	assertAdvance(t, s, adv("", ""), PhaseOrient) // queue not empty

	// Spec 2
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "PASS"), PhaseAccept)
	assertAdvance(t, s, adv("", ""), PhaseDone)

	if len(s.Completed) != 2 {
		t.Fatalf("completed: got %d, want 2", len(s.Completed))
	}
}

// --- FAIL goes to REFINE when under min_rounds ---

func TestEvaluateFailUnderMinRounds_GoesToRefine(t *testing.T) {
	s := NewState(2, 3, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)

	// Round 1, FAIL — under min_rounds(2), auto-refine
	assertAdvance(t, s, advWithDef("FAIL", []string{"Completeness"}), PhaseRefine)
	assertAdvance(t, s, advWithFixed("Added missing section"), PhaseEvaluate)

	if s.CurrentSpec.Round != 2 {
		t.Fatalf("round: got %d, want 2", s.CurrentSpec.Round)
	}

	// Round 2, FAIL — at min_rounds, goes to REVIEW
	assertAdvance(t, s, advWithDef("FAIL", []string{"Testability"}), PhaseReview)
}

// --- FAIL at max_rounds goes to REVIEW ---

func TestEvaluateFailAtMaxRounds_GoesToReview(t *testing.T) {
	s := NewState(1, 2, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)

	// Round 1 FAIL — at min_rounds(1), goes to REVIEW
	assertAdvance(t, s, advWithDef("FAIL", []string{"Precision"}), PhaseReview)
}

// --- REVIEW: accept or grant extra round ---

func TestReview_Accept(t *testing.T) {
	s := NewState(1, 1, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "FAIL"), PhaseReview)

	// Accept from REVIEW
	assertAdvance(t, s, adv("", ""), PhaseAccept)
}

func TestReview_GrantExtraRound(t *testing.T) {
	s := NewState(1, 1, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "FAIL"), PhaseReview)

	// Grant extra round from REVIEW
	assertAdvance(t, s, adv("", "FAIL"), PhaseRefine)
	assertAdvance(t, s, advWithFixed("Fixed issues"), PhaseEvaluate)

	if s.CurrentSpec.Round != 2 {
		t.Errorf("round: got %d, want 2", s.CurrentSpec.Round)
	}
}

// --- Deficiencies recorded ---

func TestDeficienciesRecorded(t *testing.T) {
	s := NewState(1, 2, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, advWithDef("FAIL", []string{"Completeness", "Precision"}), PhaseReview)

	if len(s.CurrentSpec.Evals) != 1 {
		t.Fatalf("evals: got %d, want 1", len(s.CurrentSpec.Evals))
	}
	eval := s.CurrentSpec.Evals[0]
	if eval.Verdict != "FAIL" {
		t.Errorf("verdict: got %s, want FAIL", eval.Verdict)
	}
	if len(eval.Deficiencies) != 2 {
		t.Errorf("deficiencies: got %d, want 2", len(eval.Deficiencies))
	}
}

// --- Fixed recorded on refine ---

func TestFixedRecordedOnRefine(t *testing.T) {
	s := NewState(2, 3, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, advWithDef("FAIL", []string{"Completeness"}), PhaseRefine)
	assertAdvance(t, s, advWithFixed("Added Observability section"), PhaseEvaluate)

	eval := s.CurrentSpec.Evals[0]
	if eval.Fixed != "Added Observability section" {
		t.Errorf("fixed: got %q, want 'Added Observability section'", eval.Fixed)
	}
}

// --- Evals carried to completed ---

func TestEvalsCarriedToCompleted(t *testing.T) {
	s := NewState(2, 3, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, advWithDef("FAIL", []string{"Completeness"}), PhaseRefine)
	assertAdvance(t, s, advWithFixed("Fixed it"), PhaseEvaluate)
	assertAdvance(t, s, adv("", "PASS"), PhaseAccept)
	assertAdvance(t, s, adv("", ""), PhaseDone)

	if len(s.Completed[0].Evals) != 2 {
		t.Fatalf("completed evals: got %d, want 2", len(s.Completed[0].Evals))
	}
	if s.Completed[0].Evals[0].Verdict != "FAIL" {
		t.Error("first eval should be FAIL")
	}
	if s.Completed[0].Evals[1].Verdict != "PASS" {
		t.Error("second eval should be PASS")
	}
}

// --- Next is read-only ---

func TestNextIsReadOnly(t *testing.T) {
	dir := tempDir(t)
	s := NewState(1, 1, false, twoSpecQueue())
	seedState(t, dir, s)

	before, _ := os.ReadFile(StatePath(dir))
	loaded, _ := Load(dir)
	_ = ActionDescription(loaded)
	after, _ := os.ReadFile(StatePath(dir))

	if string(before) != string(after) {
		t.Error("state file was mutated by read-only operation")
	}
}

// --- Invalid transitions ---

func TestAdvance_FileInWrongState(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())
	err := Advance(s, adv("some-file.md", ""))
	if err == nil {
		t.Fatal("expected error for --file in ORIENT")
	}
}

func TestAdvance_VerdictInWrongState(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())
	err := Advance(s, adv("", "PASS"))
	if err == nil {
		t.Fatal("expected error for --verdict in ORIENT")
	}
}

func TestAdvance_EvaluateWithoutVerdict(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)

	err := Advance(s, adv("", ""))
	if err == nil {
		t.Fatal("expected error for EVALUATE without --verdict")
	}
}

func TestAdvance_InvalidVerdict(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)

	err := Advance(s, adv("", "MAYBE"))
	if err == nil {
		t.Fatal("expected error for invalid verdict")
	}
}

func TestAdvance_DoneCannotAdvance(t *testing.T) {
	s := NewState(1, 1, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "PASS"), PhaseAccept)
	assertAdvance(t, s, adv("", ""), PhaseDone)

	err := Advance(s, adv("", ""))
	if err == nil {
		t.Fatal("expected error advancing past DONE")
	}
}

func TestAdvance_VerdictInRefine(t *testing.T) {
	s := NewState(2, 3, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)
	assertAdvance(t, s, adv("", "FAIL"), PhaseRefine)

	err := Advance(s, adv("", "PASS"))
	if err == nil {
		t.Fatal("expected error for --verdict in REFINE")
	}
}

// --- Queue order preserved ---

func TestQueueOrderPreserved(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())

	assertAdvance(t, s, adv("", ""), PhaseSelect)
	if s.CurrentSpec.Name != "Config Models" {
		t.Errorf("first spec: got %q, want Config Models", s.CurrentSpec.Name)
	}
}

// --- Draft uses queue file ---

func TestDraftWithoutFileUsesQueueValue(t *testing.T) {
	s := NewState(1, 1, false, twoSpecQueue())
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("", ""), PhaseEvaluate)

	if s.CurrentSpec.File != "optimizer/specs/configuration-models.md" {
		t.Errorf("file: got %q, want queue value", s.CurrentSpec.File)
	}
}

func TestDraftRecordsFilePath(t *testing.T) {
	s := NewState(1, 1, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, adv("", ""), PhaseSelect)
	assertAdvance(t, s, adv("", ""), PhaseDraft)
	assertAdvance(t, s, adv("custom/path/spec.md", ""), PhaseEvaluate)

	if s.CurrentSpec.File != "custom/path/spec.md" {
		t.Errorf("file: got %q, want custom/path/spec.md", s.CurrentSpec.File)
	}
}

// --- Action descriptions ---

func TestActionDescription_RefineShowsDeficiencies(t *testing.T) {
	s := &ScaffoldState{
		State:     PhaseRefine,
		MaxRounds: 3,
		CurrentSpec: &ActiveSpec{
			Name:  "Test",
			Round: 1,
			Evals: []EvalRecord{{Round: 1, Verdict: "FAIL", Deficiencies: []string{"Completeness", "Precision"}}},
		},
	}
	desc := ActionDescription(s)
	if desc == "" {
		t.Error("empty description")
	}
	if !containsSubstring(desc, "Completeness") {
		t.Errorf("description should mention deficiency, got: %s", desc)
	}
}

func TestActionDescription_ReviewShowsDeficiencies(t *testing.T) {
	s := &ScaffoldState{
		State:     PhaseReview,
		MaxRounds: 1,
		CurrentSpec: &ActiveSpec{
			Name:  "Test",
			Round: 1,
			Evals: []EvalRecord{{Round: 1, Verdict: "FAIL", Deficiencies: []string{"Testability"}}},
		},
	}
	desc := ActionDescription(s)
	if !containsSubstring(desc, "Testability") {
		t.Errorf("review description should mention deficiency, got: %s", desc)
	}
	if !containsSubstring(desc, "another round") {
		t.Errorf("review description should mention option for another round, got: %s", desc)
	}
}

// --- Helpers ---

func assertAdvance(t *testing.T, s *ScaffoldState, in AdvanceInput, expectedState Phase) {
	t.Helper()
	err := Advance(s, in)
	if err != nil {
		t.Fatalf("advance to %s failed: %v", expectedState, err)
	}
	if s.State != expectedState {
		t.Fatalf("state: got %s, want %s", s.State, expectedState)
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
