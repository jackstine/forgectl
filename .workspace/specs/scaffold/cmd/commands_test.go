package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scaffold/state"
)

func writeQueueFile(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "queue.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing queue file: %v", err)
	}
	return path
}

func validQueueJSON() string {
	return `{
		"specs": [
			{
				"name": "Config Models",
				"domain": "optimizer",
				"topic": "The optimizer defines structured config schemas",
				"file": "optimizer/specs/configuration-models.md",
				"planning_sources": ["plan1.md"],
				"depends_on": []
			},
			{
				"name": "Repository Loading",
				"domain": "optimizer",
				"topic": "The optimizer clones or locates a repository",
				"file": "optimizer/specs/repository-loading.md",
				"planning_sources": ["plan2.md"],
				"depends_on": ["Config Models"]
			}
		]
	}`
}

func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func resetFlags() {
	initMinRounds = 1
	initMaxRounds = 0
	initFrom = ""
	initUserGuided = false
	advanceFile = ""
	advanceVerdict = ""
	advanceMessage = ""
	advanceDeficiencies = ""
	advanceFixed = ""
}

// --- Init command ---

func TestInitCommand_Success(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--max-rounds", "3", "--from", queuePath)
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "2 specs") {
		t.Errorf("expected '2 specs' in output, got: %s", out)
	}

	if !state.Exists(dir) {
		t.Error("state file should exist after init")
	}
}

func TestInitCommand_MinMaxRounds(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--min-rounds", "2", "--max-rounds", "5", "--from", queuePath)
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "2-5") {
		t.Errorf("expected '2-5' in output, got: %s", out)
	}

	s, _ := state.Load(dir)
	if s.MinRounds != 2 || s.MaxRounds != 5 {
		t.Errorf("rounds: got %d-%d, want 2-5", s.MinRounds, s.MaxRounds)
	}
}

func TestInitCommand_MinExceedsMax(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()

	_, err := executeCommand("init", "--dir", dir, "--min-rounds", "5", "--max-rounds", "2", "--from", queuePath)
	if err == nil {
		t.Fatal("expected error when min > max")
	}
}

func TestInitCommand_ExistingStateFile(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--max-rounds", "1", "--from", queuePath)
	resetFlags()

	_, err := executeCommand("init", "--dir", dir, "--max-rounds", "1", "--from", queuePath)
	if err == nil {
		t.Fatal("expected error for existing state file")
	}
}

func TestInitCommand_InvalidQueue(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, `{"specs": [{"name": "A"}]}`)
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--max-rounds", "1", "--from", queuePath)
	if err == nil {
		t.Fatal("expected error for invalid queue")
	}
	if !strings.Contains(out, "Validation errors") {
		t.Errorf("expected validation errors in output, got: %s", out)
	}
}

// --- Advance with deficiencies ---

func TestAdvanceCommand_FailWithDeficiencies(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--max-rounds", "2", "--from", queuePath)
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir) // DRAFT → EVALUATE
	resetFlags()

	out, err := executeCommand("advance", "--dir", dir, "--verdict", "FAIL", "--deficiencies", "Completeness,Precision")
	if err != nil {
		t.Fatalf("advance failed: %v\noutput: %s", err, out)
	}

	s, _ := state.Load(dir)
	if len(s.CurrentSpec.Evals) != 1 {
		t.Fatalf("evals: got %d, want 1", len(s.CurrentSpec.Evals))
	}
	if len(s.CurrentSpec.Evals[0].Deficiencies) != 2 {
		t.Errorf("deficiencies: got %d, want 2", len(s.CurrentSpec.Evals[0].Deficiencies))
	}
}

// --- Advance with --fixed in REFINE ---

func TestAdvanceCommand_RefineWithFixed(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--min-rounds", "2", "--max-rounds", "3", "--from", queuePath)
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir) // DRAFT → EVALUATE
	resetFlags()
	executeCommand("advance", "--dir", dir, "--verdict", "FAIL", "--deficiencies", "Completeness") // → REFINE
	resetFlags()

	out, err := executeCommand("advance", "--dir", dir, "--fixed", "Added Observability section")
	if err != nil {
		t.Fatalf("advance failed: %v\noutput: %s", err, out)
	}

	s, _ := state.Load(dir)
	if s.CurrentSpec.Evals[0].Fixed != "Added Observability section" {
		t.Errorf("fixed: got %q", s.CurrentSpec.Evals[0].Fixed)
	}
}

// --- REVIEW state ---

func TestAdvanceCommand_ReviewToAccept(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--max-rounds", "1", "--from", queuePath)
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir) // DRAFT → EVALUATE
	resetFlags()
	executeCommand("advance", "--dir", dir, "--verdict", "FAIL") // → REVIEW
	resetFlags()

	out, err := executeCommand("advance", "--dir", dir) // REVIEW → ACCEPT
	if err != nil {
		t.Fatalf("advance failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "ACCEPT") {
		t.Errorf("expected ACCEPT, got: %s", out)
	}
}

func TestAdvanceCommand_ReviewGrantExtraRound(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--max-rounds", "1", "--from", queuePath)
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir) // DRAFT → EVALUATE
	resetFlags()
	executeCommand("advance", "--dir", dir, "--verdict", "FAIL") // → REVIEW
	resetFlags()

	out, err := executeCommand("advance", "--dir", dir, "--verdict", "FAIL") // REVIEW → REFINE
	if err != nil {
		t.Fatalf("advance failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "REFINE") {
		t.Errorf("expected REFINE, got: %s", out)
	}
}

// --- Pass requires message ---

func TestAdvanceCommand_PassRequiresMessage(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--max-rounds", "1", "--from", queuePath)
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir) // DRAFT → EVALUATE
	resetFlags()

	_, err := executeCommand("advance", "--dir", dir, "--verdict", "PASS")
	if err == nil {
		t.Fatal("expected error for PASS without --message")
	}
}

// --- Status ---

func TestStatusCommand(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--max-rounds", "2", "--from", queuePath)
	resetFlags()

	out, err := executeCommand("status", "--dir", dir)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "Session") {
		t.Errorf("expected session header, got: %s", out)
	}
	if !strings.Contains(out, "1-2") {
		t.Errorf("expected '1-2' rounds, got: %s", out)
	}
}

// --- Next ---

func TestNextCommand_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	resetFlags()

	_, err := executeCommand("next", "--dir", dir)
	if err == nil {
		t.Fatal("expected error for missing state file")
	}
}
