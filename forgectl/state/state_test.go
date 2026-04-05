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

func TestSaveAndLoadReverseEngineeringState(t *testing.T) {
	dir := t.TempDir()
	re := NewReverseEngineeringState("understand the auth module", []string{"domain-a", "domain-b"}, true)
	s := &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              StateOrient,
		ReverseEngineering: re,
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ReverseEngineering == nil {
		t.Fatal("ReverseEngineering should not be nil after load")
	}
	re2 := loaded.ReverseEngineering
	if re2.Concept != "understand the auth module" {
		t.Errorf("Concept = %q, want %q", re2.Concept, "understand the auth module")
	}
	if re2.TotalDomains != 2 {
		t.Errorf("TotalDomains = %d, want 2", re2.TotalDomains)
	}
	if re2.Domains[0] != "domain-a" || re2.Domains[1] != "domain-b" {
		t.Errorf("Domains = %v, want [domain-a domain-b]", re2.Domains)
	}
	if !re2.ColleagueReview {
		t.Error("ColleagueReview should be true")
	}
	if re2.CurrentDomain != 0 || re2.ReconcileDomain != 0 || re2.Round != 0 {
		t.Errorf("unexpected initial counters: current=%d reconcile=%d round=%d",
			re2.CurrentDomain, re2.ReconcileDomain, re2.Round)
	}
}

func TestSaveAndLoadReverseEngineeringStateAllFields(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{
		Phase: PhaseReverseEngineering,
		State: StateExecute,
		ReverseEngineering: &ReverseEngineeringState{
			Concept:         "map the billing system",
			Domains:         []string{"billing"},
			CurrentDomain:   0,
			TotalDomains:    1,
			QueueFile:       "/tmp/queue.json",
			QueueHash:       "abc123",
			ExecuteFile:     "/tmp/execute.json",
			Round:           2,
			ColleagueReview: false,
			ReconcileDomain: 0,
			Evals: []EvalRecord{
				{Round: 1, Verdict: "FAIL", EvalReport: "needs more detail"},
				{Round: 2, Verdict: "PASS"},
			},
		},
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	re := loaded.ReverseEngineering
	if re == nil {
		t.Fatal("ReverseEngineering should not be nil")
	}
	if re.QueueFile != "/tmp/queue.json" {
		t.Errorf("QueueFile = %q, want /tmp/queue.json", re.QueueFile)
	}
	if re.QueueHash != "abc123" {
		t.Errorf("QueueHash = %q, want abc123", re.QueueHash)
	}
	if re.ExecuteFile != "/tmp/execute.json" {
		t.Errorf("ExecuteFile = %q, want /tmp/execute.json", re.ExecuteFile)
	}
	if re.Round != 2 {
		t.Errorf("Round = %d, want 2", re.Round)
	}
	if len(re.Evals) != 2 {
		t.Fatalf("Evals len = %d, want 2", len(re.Evals))
	}
	if re.Evals[0].Verdict != "FAIL" || re.Evals[1].Verdict != "PASS" {
		t.Errorf("Evals verdicts = %v/%v, want FAIL/PASS", re.Evals[0].Verdict, re.Evals[1].Verdict)
	}
}

func TestRecoverReverseEngineeringStateFromBackup(t *testing.T) {
	dir := t.TempDir()
	s := &ForgeState{
		Phase: PhaseReverseEngineering,
		State: StateGapAnalysis,
		ReverseEngineering: &ReverseEngineeringState{
			Concept:       "trace payment flow",
			Domains:       []string{"payments"},
			TotalDomains:  1,
			CurrentDomain: 0,
			Round:         1,
		},
	}

	// Save to put a valid state in place, then put it in the backup path.
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(filepath.Join(dir, stateBackup), data, 0644)

	if err := Recover(dir); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after recovery: %v", err)
	}

	if loaded.ReverseEngineering == nil {
		t.Fatal("ReverseEngineering should not be nil after recovery")
	}
	if loaded.ReverseEngineering.Concept != "trace payment flow" {
		t.Errorf("Concept = %q, want %q", loaded.ReverseEngineering.Concept, "trace payment flow")
	}
	if loaded.State != StateGapAnalysis {
		t.Errorf("State = %s, want GAP_ANALYSIS", loaded.State)
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
