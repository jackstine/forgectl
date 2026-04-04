package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateOrient,
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Phase != PhaseSpecifying || loaded.State != StateOrient {
		t.Errorf("got phase=%s state=%s, want specifying ORIENT", loaded.Phase, loaded.State)
	}
}

func TestSaveCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{Phase: PhaseSpecifying, State: StateOrient}

	// First save.
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	// Second save should create backup.
	s.State = StateSelect
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}

	bakPath := filepath.Join(dir, stateBackup)
	if _, err := os.Stat(bakPath); err != nil {
		t.Errorf("backup file should exist: %v", err)
	}
}

func TestRecoverFromMissingJsonWithBak(t *testing.T) {
	dir := t.TempDir()

	// Create bak file.
	s := &ForgeState{Phase: PhaseSpecifying, State: StateOrient}
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(dir, stateBackup), data, 0644)

	if err := Recover(dir); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	// json file should now exist.
	if !fileExists(filepath.Join(dir, stateFile)) {
		t.Error("state file should have been restored from backup")
	}
}

func TestRecoverFromMissingJsonWithTmp(t *testing.T) {
	dir := t.TempDir()

	s := &ForgeState{Phase: PhaseSpecifying, State: StateOrient}
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(dir, stateTmp), data, 0644)

	if err := Recover(dir); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	if !fileExists(filepath.Join(dir, stateFile)) {
		t.Error("state file should have been restored from tmp")
	}
}

func TestRecoverCorruptWithBackup(t *testing.T) {
	dir := t.TempDir()

	// Write corrupt json.
	os.WriteFile(filepath.Join(dir, stateFile), []byte("{bad json"), 0644)

	// Write valid backup.
	s := &ForgeState{Phase: PhaseSpecifying, State: StateOrient}
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(dir, stateBackup), data, 0644)

	if err := Recover(dir); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	// Corrupt file should be moved.
	if !fileExists(filepath.Join(dir, stateCorrupt)) {
		t.Error("corrupt file should have been renamed")
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after recovery: %v", err)
	}
	if loaded.State != StateOrient {
		t.Errorf("state should be ORIENT, got %s", loaded.State)
	}
}

func TestRecoverCleansUpStaleTmp(t *testing.T) {
	dir := t.TempDir()

	s := &ForgeState{Phase: PhaseSpecifying, State: StateOrient}
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(dir, stateFile), data, 0644)
	os.WriteFile(filepath.Join(dir, stateTmp), []byte("stale"), 0644)

	if err := Recover(dir); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	if fileExists(filepath.Join(dir, stateTmp)) {
		t.Error("stale tmp should have been removed")
	}
}

func TestExistsReturnsFalseWhenNoState(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir) {
		t.Error("Exists should return false for empty dir")
	}
}

func TestLoadReturnsErrorWhenNoState(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Error("Load should return error when no state file")
	}
}

func TestArchiveSessionCreatesFile(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{
		Phase: PhaseImplementing,
		State: StateDone,
	}

	if err := ArchiveSession(dir, "myproject", s); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatalf("sessions dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 archive file, got %d", len(entries))
	}

	name := entries[0].Name()
	if len(name) < len("myproject-") || name[:len("myproject-")] != "myproject-" {
		t.Errorf("archive name %q should start with domain prefix", name)
	}
}

func TestArchiveSessionContainsValidJSON(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{
		Phase: PhaseImplementing,
		State: StateDone,
	}

	if err := ArchiveSession(dir, "test", s); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(dir, "sessions"))
	archivePath := filepath.Join(dir, "sessions", entries[0].Name())
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("reading archive: %v", err)
	}

	var loaded ForgeState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Errorf("archive is not valid JSON: %v", err)
	}
	if loaded.Phase != PhaseImplementing {
		t.Errorf("archived phase = %s, want implementing", loaded.Phase)
	}
}

func TestArchiveSessionCreatesSessionsDir(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{Phase: PhaseImplementing, State: StateDone}

	// sessions/ does not exist yet — ArchiveSession must create it.
	sessionsDir := filepath.Join(dir, "sessions")
	if _, err := os.Stat(sessionsDir); !os.IsNotExist(err) {
		t.Fatal("sessions dir should not exist before archive")
	}

	if err := ArchiveSession(dir, "domain", s); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}

	if _, err := os.Stat(sessionsDir); err != nil {
		t.Errorf("sessions dir should exist after archive: %v", err)
	}
}
