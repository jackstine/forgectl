package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"impl-scaffold/state"
)

func validQueueJSON() string {
	return `{
  "plans": [
    {
      "name": "Protocol Implementation",
      "domain": "protocols",
      "topic": "WS1 and WS2 message contracts",
      "file": "protocols/.workspace/implementation_plan/PLAN.md",
      "specs": ["protocols/ws1/specs/ws1.md"],
      "code_search_roots": ["api/", "optimizer/"]
    }
  ]
}`
}

func writeQueueFile(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func resetFlags() {
	fromFlag = ""
	minRoundsFlag = 1
	maxRoundsFlag = 0
	subAgentsFlag = 3
	userGuidedFlag = false
	advFileFlag = ""
	advVerdictFlag = ""
	advMessageFlag = ""
	advDeficienciesFlag = ""
	advFixedFlag = ""
	advNotesFlag = ""
}

func TestInitCreatesStateFile(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	_, err := executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")
	if err != nil {
		t.Fatal(err)
	}

	if !state.Exists(tmpDir) {
		t.Fatal("state file not created")
	}

	s, err := state.Load(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != state.ORIENT {
		t.Fatalf("expected ORIENT, got %s", s.State)
	}
	if s.MinRounds != 1 {
		t.Fatalf("expected min 1, got %d", s.MinRounds)
	}
	if s.MaxRounds != 3 {
		t.Fatalf("expected max 3, got %d", s.MaxRounds)
	}
	if s.SubAgents != 3 {
		t.Fatalf("expected sub_agents 3, got %d", s.SubAgents)
	}
}

func TestInitWithSubAgents(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	_, err := executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3", "--sub-agents", "5")
	if err != nil {
		t.Fatal(err)
	}

	s, _ := state.Load(tmpDir)
	if s.SubAgents != 5 {
		t.Fatalf("expected sub_agents 5, got %d", s.SubAgents)
	}
}

func TestInitRejectsExistingState(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	// Create state first
	executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")

	resetFlags()
	_, err := executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")
	if err == nil {
		t.Fatal("expected error for existing state file")
	}
}

func TestInitRejectsMinExceedsMax(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	_, err := executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--min-rounds", "5", "--max-rounds", "2")
	if err == nil {
		t.Fatal("expected error for min > max")
	}
}

func TestInitRejectsSubAgentsZero(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	_, err := executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3", "--sub-agents", "0")
	if err == nil {
		t.Fatal("expected error for sub-agents 0")
	}
}

func TestAdvanceBeforeInitFails(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()

	_, err := executeCommand(t, "advance", "--dir", tmpDir)
	if err == nil {
		t.Fatal("expected error for advance before init")
	}
}

func TestStatusBeforeInitFails(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()

	_, err := executeCommand(t, "status", "--dir", tmpDir)
	if err == nil {
		t.Fatal("expected error for status before init")
	}
}

func TestAdvancePrintsState(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")

	resetFlags()
	output, err := executeCommand(t, "advance", "--dir", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "STUDY_SPECS") {
		t.Fatalf("expected STUDY_SPECS in output, got: %s", output)
	}
	if !strings.Contains(output, "Protocol Implementation") {
		t.Fatalf("expected plan name in output, got: %s", output)
	}
}

func TestAdvanceWithNotes(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")

	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir) // → STUDY_SPECS

	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--notes", "Found key specs") // → STUDY_CODE

	s, _ := state.Load(tmpDir)
	if s.CurrentPlan.Study.SpecsNotes != "Found key specs" {
		t.Fatalf("expected notes to be recorded, got: %s", s.CurrentPlan.Study.SpecsNotes)
	}
}

func TestAdvanceOutputIncludesSubAgents(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3", "--sub-agents", "5")

	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir) // → STUDY_SPECS

	resetFlags()
	output, err := executeCommand(t, "advance", "--dir", tmpDir) // → STUDY_CODE
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output, "Sub-agents: 5") {
		t.Fatalf("expected Sub-agents: 5 in output, got: %s", output)
	}
}

func TestFullCLILifecycle(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "2")

	// ORIENT → STUDY_SPECS
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir)

	// STUDY_SPECS → STUDY_CODE
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--notes", "spec notes")

	// STUDY_CODE → STUDY_PACKAGES
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--notes", "code notes")

	// STUDY_PACKAGES → SELECT
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--notes", "pkg notes")

	// SELECT → DRAFT
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir)

	// DRAFT → EVALUATE
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir)

	// EVALUATE FAIL → REFINE
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--verdict", "FAIL", "--deficiencies", "Completeness,Traceability")

	// REFINE → EVALUATE
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--fixed", "Added acceptance criteria")

	// EVALUATE PASS → ACCEPT
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir, "--verdict", "PASS", "--message", "Add protocol impl plan")

	// ACCEPT → DONE
	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir)

	s, err := state.Load(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != state.DONE {
		t.Fatalf("expected DONE, got %s", s.State)
	}
	if len(s.Completed) != 1 {
		t.Fatalf("expected 1 completed, got %d", len(s.Completed))
	}

	c := s.Completed[0]
	if c.Study.SpecsNotes != "spec notes" {
		t.Fatal("specs notes not preserved")
	}
	if c.Study.CodeNotes != "code notes" {
		t.Fatal("code notes not preserved")
	}
	if c.Study.PackagesNotes != "pkg notes" {
		t.Fatal("packages notes not preserved")
	}
	if len(c.Evals) != 2 {
		t.Fatalf("expected 2 evals, got %d", len(c.Evals))
	}
	if c.Evals[0].Verdict != "FAIL" {
		t.Fatal("first eval should be FAIL")
	}
	if c.Evals[1].Verdict != "PASS" {
		t.Fatal("second eval should be PASS")
	}
}

func TestStateFilePersistence(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()
	queueFile := writeQueueFile(t, tmpDir, validQueueJSON())

	executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")

	resetFlags()
	executeCommand(t, "advance", "--dir", tmpDir) // → STUDY_SPECS

	// Read the raw JSON to verify structure
	data, err := os.ReadFile(filepath.Join(tmpDir, "impl-scaffold-state.json"))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// Verify key fields exist
	requiredFields := []string{"min_rounds", "max_rounds", "sub_agents", "state", "current_plan", "queue", "completed"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Fatalf("missing field in state file: %s", field)
		}
	}
}

func TestInitValidationOutput(t *testing.T) {
	resetFlags()
	tmpDir := t.TempDir()

	// Write invalid queue
	badQueue := `{"plans": [{"name": ""}]}`
	queueFile := writeQueueFile(t, tmpDir, badQueue)

	_, err := executeCommand(t, "init", "--dir", tmpDir, "--from", queueFile, "--max-rounds", "3")
	// Should exit with error (os.Exit in the command, but in tests it returns)
	if err == nil {
		// The command calls os.Exit(1) for validation errors, which won't work in tests.
		// Check that no state file was created instead.
		if state.Exists(tmpDir) {
			t.Fatal("state file should not be created on validation failure")
		}
	}
}
