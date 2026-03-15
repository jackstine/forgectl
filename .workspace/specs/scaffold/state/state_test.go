package state

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
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

// --- Save / Load round-trip ---

func TestSaveAndLoad(t *testing.T) {
	dir := tempDir(t)
	s := NewState(3, true, twoSpecQueue())
	seedState(t, dir, s)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.EvaluationRounds != 3 {
		t.Errorf("rounds: got %d, want 3", loaded.EvaluationRounds)
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
	if loaded.CurrentSpec != nil {
		t.Error("current_spec should be nil")
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

func TestExists(t *testing.T) {
	dir := tempDir(t)
	if Exists(dir) {
		t.Error("should not exist yet")
	}
	seedState(t, dir, NewState(1, false, twoSpecQueue()))
	if !Exists(dir) {
		t.Error("should exist after save")
	}
}

// --- Full lifecycle: single spec ---

func TestFullLifecycle_SingleSpec_Pass(t *testing.T) {
	dir := tempDir(t)
	specs := []QueueSpec{twoSpecQueue()[0]}
	s := NewState(1, false, specs)
	seedState(t, dir, s)

	s, _ = Load(dir)

	// ORIENT → SELECT
	assertAdvance(t, s, "", "", PhaseSelect)
	if s.CurrentSpec == nil {
		t.Fatal("current_spec should be set after ORIENT advance")
	}
	if s.CurrentSpec.Name != "Config Models" {
		t.Errorf("current spec: got %q, want Config Models", s.CurrentSpec.Name)
	}

	// SELECT → DRAFT
	assertAdvance(t, s, "", "", PhaseDraft)

	// DRAFT → EVALUATE
	assertAdvance(t, s, "optimizer/specs/configuration-models.md", "", PhaseEvaluate)
	if s.CurrentSpec.Round != 1 {
		t.Errorf("round: got %d, want 1", s.CurrentSpec.Round)
	}

	// EVALUATE (PASS) → ACCEPT
	assertAdvance(t, s, "", "PASS", PhaseAccept)

	// ACCEPT → DONE (queue empty)
	assertAdvance(t, s, "", "", PhaseDone)
	if s.CurrentSpec != nil {
		t.Error("current_spec should be nil after ACCEPT")
	}
	if len(s.Completed) != 1 {
		t.Fatalf("completed: got %d, want 1", len(s.Completed))
	}
	if s.Completed[0].RoundsTaken != 1 {
		t.Errorf("rounds_taken: got %d, want 1", s.Completed[0].RoundsTaken)
	}
}

// --- Full lifecycle: two specs ---

func TestFullLifecycle_TwoSpecs(t *testing.T) {
	dir := tempDir(t)
	s := NewState(1, false, twoSpecQueue())
	seedState(t, dir, s)

	s, _ = Load(dir)

	// Spec 1: ORIENT → SELECT → DRAFT → EVALUATE(PASS) → ACCEPT → back to ORIENT
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "optimizer/specs/configuration-models.md", "", PhaseEvaluate)
	assertAdvance(t, s, "", "PASS", PhaseAccept)
	assertAdvance(t, s, "", "", PhaseOrient) // queue not empty, back to ORIENT

	if len(s.Completed) != 1 {
		t.Fatalf("completed after spec 1: got %d, want 1", len(s.Completed))
	}

	// Spec 2: ORIENT → SELECT → DRAFT → EVALUATE(PASS) → ACCEPT → DONE
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "optimizer/specs/repository-loading.md", "", PhaseEvaluate)
	assertAdvance(t, s, "", "PASS", PhaseAccept)
	assertAdvance(t, s, "", "", PhaseDone)

	if len(s.Completed) != 2 {
		t.Fatalf("completed after spec 2: got %d, want 2", len(s.Completed))
	}
}

// --- Evaluate-refine loop ---

func TestEvaluateRefineLoop_RespectsRoundLimit(t *testing.T) {
	s := NewState(2, false, []QueueSpec{twoSpecQueue()[0]})

	// ORIENT → SELECT → DRAFT → EVALUATE
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)

	// Round 1: FAIL → REFINE
	if s.CurrentSpec.Round != 1 {
		t.Fatalf("round: got %d, want 1", s.CurrentSpec.Round)
	}
	assertAdvance(t, s, "", "FAIL", PhaseRefine)

	// REFINE → EVALUATE (round 2)
	assertAdvance(t, s, "", "", PhaseEvaluate)
	if s.CurrentSpec.Round != 2 {
		t.Fatalf("round: got %d, want 2", s.CurrentSpec.Round)
	}

	// Round 2 (max): FAIL → ACCEPT (not REFINE)
	assertAdvance(t, s, "", "FAIL", PhaseAccept)
}

func TestEvaluateRefineLoop_PassOnSecondRound(t *testing.T) {
	s := NewState(3, false, []QueueSpec{twoSpecQueue()[0]})

	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)

	// Round 1: FAIL
	assertAdvance(t, s, "", "FAIL", PhaseRefine)
	assertAdvance(t, s, "", "", PhaseEvaluate)

	// Round 2: PASS
	assertAdvance(t, s, "", "PASS", PhaseAccept)
	if s.CurrentSpec.Round != 2 {
		t.Errorf("round at accept: got %d, want 2", s.CurrentSpec.Round)
	}
}

// --- Next is read-only ---

func TestNextIsReadOnly(t *testing.T) {
	dir := tempDir(t)
	s := NewState(1, false, twoSpecQueue())
	seedState(t, dir, s)

	before, _ := os.ReadFile(StatePath(dir))

	// Load and call ActionDescription (what 'next' does).
	loaded, _ := Load(dir)
	_ = ActionDescription(loaded)

	after, _ := os.ReadFile(StatePath(dir))

	if string(before) != string(after) {
		t.Error("state file was mutated by read-only operation")
	}
}

// --- Invalid transitions ---

func TestAdvance_FileInWrongState(t *testing.T) {
	s := NewState(1, false, twoSpecQueue())
	// State is ORIENT, --file should be rejected.
	err := Advance(s, "some-file.md", "")
	if err == nil {
		t.Fatal("expected error for --file in ORIENT")
	}
}

func TestAdvance_VerdictInWrongState(t *testing.T) {
	s := NewState(1, false, twoSpecQueue())
	// State is ORIENT, --verdict should be rejected.
	err := Advance(s, "", "PASS")
	if err == nil {
		t.Fatal("expected error for --verdict in ORIENT")
	}
}

func TestAdvance_DraftWithoutFile(t *testing.T) {
	s := NewState(1, false, twoSpecQueue())
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)

	err := Advance(s, "", "")
	if err == nil {
		t.Fatal("expected error for DRAFT without --file")
	}
}

func TestAdvance_EvaluateWithoutVerdict(t *testing.T) {
	s := NewState(1, false, twoSpecQueue())
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)

	err := Advance(s, "", "")
	if err == nil {
		t.Fatal("expected error for EVALUATE without --verdict")
	}
}

func TestAdvance_InvalidVerdict(t *testing.T) {
	s := NewState(1, false, twoSpecQueue())
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)

	err := Advance(s, "", "MAYBE")
	if err == nil {
		t.Fatal("expected error for invalid verdict")
	}
}

func TestAdvance_DoneCannotAdvance(t *testing.T) {
	s := NewState(1, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)
	assertAdvance(t, s, "", "PASS", PhaseAccept)
	assertAdvance(t, s, "", "", PhaseDone)

	err := Advance(s, "", "")
	if err == nil {
		t.Fatal("expected error advancing past DONE")
	}
}

func TestAdvance_VerdictInRefine(t *testing.T) {
	s := NewState(2, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)
	assertAdvance(t, s, "", "FAIL", PhaseRefine)

	err := Advance(s, "", "PASS")
	if err == nil {
		t.Fatal("expected error for --verdict in REFINE")
	}
}

func TestAdvance_FileInEvaluate(t *testing.T) {
	s := NewState(1, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)

	err := Advance(s, "other.md", "")
	if err == nil {
		t.Fatal("expected error for --file in EVALUATE")
	}
}

// --- Action descriptions ---

func TestActionDescription_AllStates(t *testing.T) {
	tests := []struct {
		state Phase
		want  string
	}{
		{PhaseOrient, "Read planning docs"},
		{PhaseSelect, "Review topic"},
		{PhaseDraft, "Write the spec file"},
		{PhaseEvaluate, "Spawn Opus evaluation sub-agent"},
		{PhaseRefine, "Address deficiencies"},
		{PhaseAccept, "Spec finalized"},
		{PhaseDone, "All specs complete"},
	}

	for _, tt := range tests {
		s := &ScaffoldState{
			State:            tt.state,
			EvaluationRounds: 3,
			CurrentSpec: &ActiveSpec{
				Name:  "Test",
				Round: 1,
			},
			Queue: twoSpecQueue(),
		}
		desc := ActionDescription(s)
		if desc == "" {
			t.Errorf("empty description for state %s", tt.state)
		}
		// Just verify it contains the expected keyword.
		if !containsSubstring(desc, tt.want) {
			t.Errorf("state %s: description %q does not contain %q", tt.state, desc, tt.want)
		}
	}
}

func TestActionDescription_SelectUserGuided(t *testing.T) {
	s := &ScaffoldState{
		State:      PhaseSelect,
		UserGuided: true,
		CurrentSpec: &ActiveSpec{
			Name: "Test",
		},
	}
	desc := ActionDescription(s)
	if !containsSubstring(desc, "Discuss topic with user") {
		t.Errorf("user-guided SELECT should mention user discussion, got: %s", desc)
	}
}

func TestActionDescription_FinalEvalRound(t *testing.T) {
	s := &ScaffoldState{
		State:            PhaseEvaluate,
		EvaluationRounds: 2,
		CurrentSpec: &ActiveSpec{
			Name:  "Test",
			Round: 2,
		},
	}
	desc := ActionDescription(s)
	if !containsSubstring(desc, "Final evaluation round") {
		t.Errorf("final round should say so, got: %s", desc)
	}
}

// --- Queue order preserved ---

func TestQueueOrderPreserved(t *testing.T) {
	specs := twoSpecQueue()
	s := NewState(1, false, specs)

	// First advance pulls first item.
	assertAdvance(t, s, "", "", PhaseSelect)
	if s.CurrentSpec.Name != "Config Models" {
		t.Errorf("first spec: got %q, want Config Models", s.CurrentSpec.Name)
	}

	// Complete first spec.
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "a.md", "", PhaseEvaluate)
	assertAdvance(t, s, "", "PASS", PhaseAccept)
	assertAdvance(t, s, "", "", PhaseOrient) // back to orient

	// Second advance pulls second item.
	assertAdvance(t, s, "", "", PhaseSelect)
	if s.CurrentSpec.Name != "Repository Loading" {
		t.Errorf("second spec: got %q, want Repository Loading", s.CurrentSpec.Name)
	}
}

// --- Draft records file path ---

func TestDraftRecordsFilePath(t *testing.T) {
	s := NewState(1, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "custom/path/spec.md", "", PhaseEvaluate)

	if s.CurrentSpec.File != "custom/path/spec.md" {
		t.Errorf("file: got %q, want custom/path/spec.md", s.CurrentSpec.File)
	}
}

// --- Completed records round count ---

func TestCompletedRecordsRounds(t *testing.T) {
	s := NewState(3, false, []QueueSpec{twoSpecQueue()[0]})
	assertAdvance(t, s, "", "", PhaseSelect)
	assertAdvance(t, s, "", "", PhaseDraft)
	assertAdvance(t, s, "f.md", "", PhaseEvaluate)
	assertAdvance(t, s, "", "FAIL", PhaseRefine)
	assertAdvance(t, s, "", "", PhaseEvaluate) // round 2
	assertAdvance(t, s, "", "PASS", PhaseAccept)
	assertAdvance(t, s, "", "", PhaseDone)

	if s.Completed[0].RoundsTaken != 2 {
		t.Errorf("rounds_taken: got %d, want 2", s.Completed[0].RoundsTaken)
	}
}

// --- Helpers ---

func assertAdvance(t *testing.T, s *ScaffoldState, file, verdict string, expectedState Phase) {
	t.Helper()
	err := Advance(s, file, verdict)
	if err != nil {
		t.Fatalf("advance to %s failed: %v", expectedState, err)
	}
	if s.State != expectedState {
		t.Fatalf("state: got %s, want %s", s.State, expectedState)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
