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
	initRounds = 0
	initFrom = ""
	initUserGuided = false
	advanceFile = ""
	advanceVerdict = ""
}

// --- Init command ---

func TestInitCommand_Success(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--rounds", "3", "--from", queuePath)
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "2 specs") {
		t.Errorf("expected '2 specs' in output, got: %s", out)
	}
	if !strings.Contains(out, "3 evaluation rounds") {
		t.Errorf("expected '3 evaluation rounds' in output, got: %s", out)
	}

	// Verify state file was created.
	if !state.Exists(dir) {
		t.Error("state file should exist after init")
	}
}

func TestInitCommand_UserGuided(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath, "--user-guided")
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "user-guided") {
		t.Errorf("expected 'user-guided' in output, got: %s", out)
	}

	s, _ := state.Load(dir)
	if !s.UserGuided {
		t.Error("user_guided should be true")
	}
}

func TestInitCommand_ExistingStateFile(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()

	// First init.
	executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	resetFlags()

	// Second init should fail.
	_, err := executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	if err == nil {
		t.Fatal("expected error for existing state file")
	}
}

func TestInitCommand_InvalidQueue(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, `{"specs": [{"name": "A"}]}`)
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	if err == nil {
		t.Fatal("expected error for invalid queue")
	}
	if !strings.Contains(out, "Validation errors") {
		t.Errorf("expected validation errors in output, got: %s", out)
	}
	if !strings.Contains(out, "Expected schema") {
		t.Errorf("expected schema in output, got: %s", out)
	}
}

func TestInitCommand_ExtraFields(t *testing.T) {
	dir := t.TempDir()
	json := `{
		"specs": [
			{
				"name": "A",
				"domain": "api",
				"topic": "T",
				"file": "f.md",
				"planning_sources": [],
				"depends_on": [],
				"priority": "high"
			}
		]
	}`
	queuePath := writeQueueFile(t, dir, json)
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	if err == nil {
		t.Fatal("expected error for extra fields")
	}
	if !strings.Contains(out, "priority") {
		t.Errorf("expected 'priority' in error output, got: %s", out)
	}
}

func TestInitCommand_DependencyWarning(t *testing.T) {
	dir := t.TempDir()
	json := `{
		"specs": [
			{
				"name": "B",
				"domain": "api",
				"topic": "Topic B",
				"file": "api/specs/b.md",
				"planning_sources": [],
				"depends_on": ["A"]
			}
		]
	}`
	queuePath := writeQueueFile(t, dir, json)
	resetFlags()

	out, err := executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if !strings.Contains(out, "Warning") || !strings.Contains(out, `"A"`) {
		t.Errorf("expected dependency warning about A, got: %s", out)
	}
}

func TestInitCommand_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	resetFlags()

	_, err := executeCommand("init", "--dir", dir, "--rounds", "1", "--from", "/nonexistent/queue.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- Next command ---

func TestNextCommand(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--rounds", "2", "--from", queuePath)
	resetFlags()

	out, err := executeCommand("next", "--dir", dir)
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	if !strings.Contains(out, "ORIENT") {
		t.Errorf("expected ORIENT state, got: %s", out)
	}
}

func TestNextCommand_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	resetFlags()

	_, err := executeCommand("next", "--dir", dir)
	if err == nil {
		t.Fatal("expected error for missing state file")
	}
}

// --- Advance command ---

func TestAdvanceCommand_OrientToSelect(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	resetFlags()

	out, err := executeCommand("advance", "--dir", dir)
	if err != nil {
		t.Fatalf("advance failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "ORIENT → SELECT") {
		t.Errorf("expected transition output, got: %s", out)
	}
	if !strings.Contains(out, "Config Models") {
		t.Errorf("expected spec name in output, got: %s", out)
	}
}

func TestAdvanceCommand_DraftRequiresFile(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()

	// DRAFT without --file should fail.
	_, err := executeCommand("advance", "--dir", dir)
	if err == nil {
		t.Fatal("expected error for DRAFT without --file")
	}
}

func TestAdvanceCommand_FullCycle(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)

	// ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir)

	// SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir)

	// DRAFT → EVALUATE
	resetFlags()
	executeCommand("advance", "--dir", dir, "--file", "optimizer/specs/cm.md")

	// EVALUATE → ACCEPT
	resetFlags()
	executeCommand("advance", "--dir", dir, "--verdict", "PASS")

	// ACCEPT → ORIENT (second spec in queue)
	resetFlags()
	out, _ := executeCommand("advance", "--dir", dir)
	if !strings.Contains(out, "ORIENT") {
		t.Errorf("expected back to ORIENT, got: %s", out)
	}

	// Verify state.
	s, _ := state.Load(dir)
	if len(s.Completed) != 1 {
		t.Errorf("completed: got %d, want 1", len(s.Completed))
	}
	if len(s.Queue) != 1 {
		t.Errorf("queue: got %d, want 1", len(s.Queue))
	}
}

// --- Status command ---

func TestStatusCommand(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--rounds", "2", "--from", queuePath)
	resetFlags()

	out, err := executeCommand("status", "--dir", dir)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "Session") {
		t.Errorf("expected session header, got: %s", out)
	}
	if !strings.Contains(out, "Queue") {
		t.Errorf("expected queue section, got: %s", out)
	}
	if !strings.Contains(out, "optimizer") {
		t.Errorf("expected domain grouping, got: %s", out)
	}
}

func TestStatusCommand_WithCompleted(t *testing.T) {
	dir := t.TempDir()
	queuePath := writeQueueFile(t, dir, validQueueJSON())
	resetFlags()
	executeCommand("init", "--dir", dir, "--rounds", "1", "--from", queuePath)

	// Complete first spec.
	resetFlags()
	executeCommand("advance", "--dir", dir) // ORIENT → SELECT
	resetFlags()
	executeCommand("advance", "--dir", dir) // SELECT → DRAFT
	resetFlags()
	executeCommand("advance", "--dir", dir, "--file", "f.md") // DRAFT → EVALUATE
	resetFlags()
	executeCommand("advance", "--dir", dir, "--verdict", "PASS") // EVALUATE → ACCEPT
	resetFlags()
	executeCommand("advance", "--dir", dir) // ACCEPT → ORIENT
	resetFlags()

	out, err := executeCommand("status", "--dir", dir)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !strings.Contains(out, "Completed") {
		t.Errorf("expected completed section, got: %s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected checkmark in completed, got: %s", out)
	}
}
