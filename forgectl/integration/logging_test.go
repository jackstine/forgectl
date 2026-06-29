//go:build integration

package integration

import (
	"testing"
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
