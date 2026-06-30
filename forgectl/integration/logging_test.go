//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgectl/state"
)

// TestH1OnlyInitAndAdvanceLog covers §H1: state-mutating commands (init,
// advance) write activity-log entries; read-only/auxiliary commands (status,
// validate, --version) do not.
func TestH1OnlyInitAndAdvanceLog(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[general]\nuser_guided = false\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.mustForge("advance") // ORIENT → SELECT

	// These must not append any log entries.
	p.mustForge("status")
	p.mustForge("--version")
	p.WriteFile("sq2.json", oneSpecQueue)
	p.mustForge("validate", "sq2.json")

	logs := p.LogFiles()
	if len(logs) != 1 {
		t.Fatalf("log files = %v, want exactly one", logs)
	}
	entries := p.LogEntries(logs[0])
	if len(entries) != 2 {
		var cmds []any
		for _, e := range entries {
			cmds = append(cmds, e["cmd"])
		}
		t.Fatalf("log has %d entries (%v), want exactly 2 (init, advance)", len(entries), cmds)
	}
	if entries[0]["cmd"] != "init" || entries[1]["cmd"] != "advance" {
		t.Errorf("log entries = %q,%q want init,advance", entries[0]["cmd"], entries[1]["cmd"])
	}
}

// TestH2LogFilenameStableAcrossSession covers §H2: the log filename is
// <initial-phase>-<sessionid[:8]>.jsonl and stays the same file across the
// session (it does not rotate per command).
func TestH2LogFilenameStableAcrossSession(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[general]\nuser_guided = false\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	sid := p.State().SessionID
	want := "specifying-" + sid[:8] + ".jsonl"
	if logs := p.LogFiles(); len(logs) != 1 || logs[0] != want {
		t.Fatalf("after init: log files = %v, want [%s]", logs, want)
	}

	// Several advances must keep writing to the same single file.
	p.mustForge("advance")
	p.mustForge("advance")
	if logs := p.LogFiles(); len(logs) != 1 || logs[0] != want {
		t.Fatalf("after advances: log files = %v, want still [%s]", logs, want)
	}
}

// TestH3ErrorEntryLoggedBeforeExit covers §H3: when advance exits non-zero, the
// activity log must contain an entry with cmd:"error" written during that
// command's execution, before the non-zero exit.
func TestH3ErrorEntryLoggedBeforeExit(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingCommitsOffConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.mustForge("advance") // ORIENT → SELECT
	p.mustForge("advance") // SELECT → DRAFT
	p.WriteFile("specs/a.md", "# stub spec\n")
	p.mustForge("advance") // DRAFT → EVALUATE
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

	// advance at EVALUATE without --verdict must exit non-zero.
	res := p.forge("advance")
	if res.Exit == 0 {
		t.Fatal("advance at EVALUATE without --verdict: expected non-zero exit, got 0")
	}

	// The log must contain an error entry written during the failed command.
	logs := p.LogFiles()
	if len(logs) == 0 {
		t.Fatal("no activity log file found after failed advance")
	}
	entries := p.LogEntries(logs[0])
	for _, e := range entries {
		if e["cmd"] == "error" {
			return // found the required error entry
		}
	}
	var cmds []any
	for _, e := range entries {
		cmds = append(cmds, e["cmd"])
	}
	t.Errorf("no cmd:\"error\" entry in activity log after non-zero exit; entries: %v", cmds)
}

// TestH4ReadOnlyLogDirWarning covers §H4: when the log directory is read-only,
// forgectl warns on stderr but the command still exits 0.
func TestH4ReadOnlyLogDirWarning(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[general]\nuser_guided = false\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)

	// init creates the log dir with proper permissions and writes the session log.
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	// Lock the log dir so the next write attempt fails with a permission error.
	logsDir := filepath.Join(p.Home, ".forgectl", "logs")
	if err := os.Chmod(logsDir, 0o444); err != nil {
		t.Fatalf("chmod logs dir: %v", err)
	}
	// Restore writable on cleanup so t.TempDir() can remove the sandbox tree.
	t.Cleanup(func() { os.Chmod(logsDir, 0o755) })

	// advance must still exit 0 (logging failure is non-fatal) but warn on stderr.
	res := p.mustForge("advance")
	if !strings.Contains(res.Stderr, "activity logging") {
		t.Errorf("expected activity-logging warning on stderr; got:\n%s", res.Stderr)
	}
}

// TestH5PruningOnlyAtInit covers §H5: PruneLogs runs only at init. status and
// eval must not remove log files; init must prune excess files down to max_files.
func TestH5PruningOnlyAtInit(t *testing.T) {
	const cfg = "[logs]\nenabled = true\nretention_days = 0\nmax_files = 1\n[general]\nuser_guided = false\n"

	p := NewProject(t)
	p.WriteConfig(cfg)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	// Pre-create three fake log files before the first init so we can check
	// that status and eval do not prune them.
	logsDir := filepath.Join(p.Home, ".forgectl", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("old-%d.jsonl", i)
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte(`{"stub":true}`), 0644); err != nil {
			t.Fatalf("write fake log %s: %v", name, err)
		}
	}

	// status and eval must NOT prune (they don't call PruneLogs).
	p.forge("status") // no session yet → non-zero exit is expected
	p.forge("eval")   // no session yet → non-zero exit is expected
	if got := p.LogFiles(); len(got) < 3 {
		t.Errorf("status/eval pruned log files: %d remain, want all 3", len(got))
	}

	// init calls PruneLogs before writing the new session log.
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	filesAfter := p.LogFiles()

	// At most max_files (1) of the pre-created old-*.jsonl files should survive.
	var oldCount int
	for _, f := range filesAfter {
		if strings.HasPrefix(f, "old-") {
			oldCount++
		}
	}
	if oldCount > 1 {
		t.Errorf("after init: %d old log files survived, want at most max_files=1", oldCount)
	}

	// Separately verify that advance does NOT trigger pruning.
	p2 := NewProject(t)
	p2.WriteConfig(cfg)
	p2.WriteFile("spec-queue.json", oneSpecQueue)
	p2.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	// Create extra files to push the count above max_files.
	logsDir2 := filepath.Join(p2.Home, ".forgectl", "logs")
	for i := 0; i < 2; i++ {
		name := fmt.Sprintf("extra-%d.jsonl", i)
		if err := os.WriteFile(filepath.Join(logsDir2, name), []byte(`{}`), 0644); err != nil {
			t.Fatalf("write extra log %s: %v", name, err)
		}
	}
	countBefore := len(p2.LogFiles())
	p2.mustForge("advance") // ORIENT → SELECT; must not prune
	countAfter := len(p2.LogFiles())
	if countAfter < countBefore {
		t.Errorf("advance pruned log files: before=%d, after=%d", countBefore, countAfter)
	}
}
