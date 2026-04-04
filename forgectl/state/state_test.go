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
