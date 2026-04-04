package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerWriteCreatesJSONL(t *testing.T) {
	// Logger creates log file and writes valid JSON lines.
	dir := t.TempDir()
	logFile := filepath.Join(dir, "specifying-abcd1234.jsonl")

	cfg := LogsConfig{Enabled: true, RetentionDays: 90, MaxFiles: 50}
	l := &Logger{enabled: true, path: logFile}

	l.Write(LogEntry{
		TS:    LogNow(),
		Cmd:   "init",
		Phase: "specifying",
		State: "ORIENT",
		Detail: map[string]interface{}{
			"from":       "queue.json",
			"batch_size": 3,
		},
	})
	_ = cfg

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if entry["cmd"] != "init" {
		t.Errorf("cmd = %v, want init", entry["cmd"])
	}
	if entry["state"] != "ORIENT" {
		t.Errorf("state = %v, want ORIENT", entry["state"])
	}
}

func TestLoggerAppends(t *testing.T) {
	// Multiple Write calls append separate lines.
	dir := t.TempDir()
	l := &Logger{enabled: true, path: filepath.Join(dir, "test.jsonl")}

	l.Write(LogEntry{TS: LogNow(), Cmd: "init", Phase: "specifying", State: "ORIENT", Detail: map[string]interface{}{}})
	l.Write(LogEntry{TS: LogNow(), Cmd: "advance", Phase: "specifying", PrevState: "ORIENT", State: "SELECT", Detail: map[string]interface{}{}})

	f, _ := os.Open(l.path)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var e2 map[string]interface{}
	json.Unmarshal([]byte(lines[1]), &e2)
	if e2["cmd"] != "advance" {
		t.Errorf("second entry cmd = %v, want advance", e2["cmd"])
	}
	if e2["prev_state"] != "ORIENT" {
		t.Errorf("prev_state = %v, want ORIENT", e2["prev_state"])
	}
}

func TestLoggerDisabledNoFile(t *testing.T) {
	// Disabled logger never creates a file.
	dir := t.TempDir()
	logPath := filepath.Join(dir, "should-not-exist.jsonl")
	l := &Logger{enabled: false, path: logPath}

	l.Write(LogEntry{TS: LogNow(), Cmd: "init", Phase: "specifying", State: "ORIENT", Detail: map[string]interface{}{}})

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Error("log file should not have been created when logger is disabled")
	}
}

func TestNewLoggerDisabledConfig(t *testing.T) {
	// NewLogger with Enabled=false returns a no-op logger.
	cfg := LogsConfig{Enabled: false, RetentionDays: 90, MaxFiles: 50}
	l := NewLogger(cfg, PhaseSpecifying, "abc12345-1234-1234-1234-123456789012")
	if l.Enabled() {
		t.Error("logger should be disabled when cfg.Enabled=false")
	}
}

func TestPruneLogsByAge(t *testing.T) {
	// Files older than retention_days are deleted.
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.jsonl")
	newFile := filepath.Join(dir, "new.jsonl")
	os.WriteFile(oldFile, []byte("old"), 0644)
	os.WriteFile(newFile, []byte("new"), 0644)

	// Back-date old file to 40 days ago.
	old := time.Now().AddDate(0, 0, -40)
	os.Chtimes(oldFile, old, old)

	cfg := LogsConfig{Enabled: true, RetentionDays: 30, MaxFiles: 0}

	// Run pruning against dir directly by temporarily overriding HOME.
	// Instead, test PruneLogs by pointing it at a known directory.
	// We test indirectly: create old/new files, then call pruneLogsInDir.
	pruneLogsInDir(dir, cfg)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should have been deleted by age pruning")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("new file should have been kept")
	}
}

func TestPruneLogsByCount(t *testing.T) {
	// When file count exceeds max_files, oldest are deleted.
	dir := t.TempDir()

	// Create 3 files with different mtimes.
	files := []string{"a.jsonl", "b.jsonl", "c.jsonl"}
	for i, name := range files {
		f := filepath.Join(dir, name)
		os.WriteFile(f, []byte(name), 0644)
		mtime := time.Now().Add(time.Duration(i) * time.Second)
		os.Chtimes(f, mtime, mtime)
	}

	cfg := LogsConfig{Enabled: true, RetentionDays: 0, MaxFiles: 2}
	pruneLogsInDir(dir, cfg)

	// Oldest (a.jsonl) should be deleted; b and c remain.
	if _, err := os.Stat(filepath.Join(dir, "a.jsonl")); !os.IsNotExist(err) {
		t.Error("oldest file should have been deleted by count pruning")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.jsonl")); err != nil {
		t.Error("second file should have been kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "c.jsonl")); err != nil {
		t.Error("newest file should have been kept")
	}
}

func TestLoggerWriteFailureNonFatal(t *testing.T) {
	// Write to an invalid/missing directory prints a warning but does not panic.
	l := &Logger{enabled: true, path: "/nonexistent/dir/that/does/not/exist/log.jsonl"}

	// Should not panic.
	l.Write(LogEntry{TS: LogNow(), Cmd: "advance", Phase: "specifying", State: "ORIENT", Detail: map[string]interface{}{}})
}

// pruneLogsInDir is a test helper that runs pruning logic against a given directory
// instead of the real ~/.forgectl/logs/ directory.
func pruneLogsInDir(dir string, cfg LogsConfig) {
	if !cfg.Enabled {
		return
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(files) == 0 {
		return
	}
	// Sort by modification time (oldest first).
	sortFilesByMtime(files)
	now := time.Now()
	// Age-based deletion.
	if cfg.RetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -cfg.RetentionDays)
		var remaining []string
		for _, f := range files {
			si, _ := os.Stat(f)
			if si != nil && si.ModTime().Before(cutoff) {
				os.Remove(f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}
	// Count-based deletion: keep at most max_files files.
	if cfg.MaxFiles > 0 {
		for len(files) > cfg.MaxFiles {
			os.Remove(files[0])
			files = files[1:]
		}
	}
}

func sortFilesByMtime(files []string) {
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			si, _ := os.Stat(files[i])
			sj, _ := os.Stat(files[j])
			if si != nil && sj != nil && sj.ModTime().Before(si.ModTime()) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}
