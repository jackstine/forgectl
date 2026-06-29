//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"forgectl/state"
)

// stateDirAbs is the resolved (default) state directory under the project root.
func (p *Project) stateDirAbs() string {
	return filepath.Join(p.Root, filepath.FromSlash(p.StateDirRel))
}

func (p *Project) stateFile() string { return filepath.Join(p.stateDirAbs(), "forgectl-state.json") }
func (p *Project) bakFile() string   { return p.stateFile() + ".bak" }
func (p *Project) tmpFile() string   { return p.stateFile() + ".tmp" }
func (p *Project) corruptFile() string {
	return p.stateFile() + ".corrupt"
}

func fileThere(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// initSpecifying drives a fresh project to specifying ORIENT and returns it.
func initSpecifying(t *testing.T) *Project {
	t.Helper()
	p := NewProject(t)
	p.WriteConfig("")
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	return p
}

// TestE1AtomicWriteAndBackup covers §E1: the atomic write keeps a backup of the
// previous state once one exists, and never leaves a stale .tmp behind.
func TestE1AtomicWriteAndBackup(t *testing.T) {
	p := initSpecifying(t)

	// After the very first save (init) there is a state file but no backup yet.
	if !fileThere(p.stateFile()) {
		t.Fatal("init did not write the state file")
	}
	if fileThere(p.bakFile()) {
		t.Error("a .bak exists after the first save; nothing should have been backed up yet")
	}
	if fileThere(p.tmpFile()) {
		t.Error("a stale .tmp exists after init")
	}

	// The next save (an advance) renames the existing json to .bak.
	p.mustForge("advance")
	if !fileThere(p.stateFile()) {
		t.Fatal("state file missing after advance")
	}
	if !fileThere(p.bakFile()) {
		t.Error("no .bak after a second save; backup step did not run")
	}
	if fileThere(p.tmpFile()) {
		t.Error("a stale .tmp survived the advance")
	}
}

// TestE1AbsoluteStateDir covers §E1: an absolute paths.state_dir is used as-is,
// not joined onto the project root.
func TestE1AbsoluteStateDir(t *testing.T) {
	p := NewProject(t)
	abs := t.TempDir()
	p.WriteConfig("[paths]\nstate_dir = \"" + filepath.ToSlash(abs) + "\"\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	if !fileThere(filepath.Join(abs, "forgectl-state.json")) {
		t.Errorf("state file not written to the absolute state_dir %s", abs)
	}
	if p.Exists(".forgectl/state/forgectl-state.json") {
		t.Error("state file also written under the project root; absolute path was joined, not used as-is")
	}
}

// TestE2RecoverFromBackup covers §E2: a missing .json with a valid .bak is
// restored on the next command, with a warning.
func TestE2RecoverFromBackup(t *testing.T) {
	p := initSpecifying(t)
	p.mustForge("advance") // creates a .bak

	// Simulate a crash between backup and rename: the json is gone, the bak holds
	// the latest good state.
	if err := os.Remove(p.stateFile()); err != nil {
		t.Fatal(err)
	}
	res := p.mustForge("status")
	if !strings.Contains(res.Stderr, "recovered state from backup") {
		t.Errorf("expected a backup-recovery warning on stderr:\n%s", res.Stderr)
	}
	if !fileThere(p.stateFile()) {
		t.Error("state file was not restored from backup")
	}
}

// TestE2CorruptJSONRestored covers §E2: a corrupt .json with a valid .bak is
// moved aside to .corrupt, the backup is restored, and a warning is printed.
func TestE2CorruptJSONRestored(t *testing.T) {
	p := initSpecifying(t)
	p.mustForge("advance") // creates a .bak

	if err := os.WriteFile(p.stateFile(), []byte("{ this is not valid json"), 0644); err != nil {
		t.Fatal(err)
	}
	res := p.mustForge("status")
	if !strings.Contains(res.Stderr, "state file was corrupt") {
		t.Errorf("expected a corruption warning on stderr:\n%s", res.Stderr)
	}
	if !fileThere(p.corruptFile()) {
		t.Error("corrupt json was not moved aside to .corrupt")
	}
	// The restored state must parse. The backup is one save behind the corrupted
	// json (it holds the pre-advance ORIENT, since Save backs up the prior file
	// before writing the new one), so recovery lands at ORIENT.
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)
}

// TestE2StaleTmpCleaned covers §E2: a stale .tmp alongside a good .json is
// cleaned up silently (no warning, json untouched).
func TestE2StaleTmpCleaned(t *testing.T) {
	p := initSpecifying(t)
	if err := os.WriteFile(p.tmpFile(), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	res := p.mustForge("status")
	if fileThere(p.tmpFile()) {
		t.Error("stale .tmp was not cleaned up")
	}
	if strings.Contains(res.Stderr, "Warning") {
		t.Errorf("cleaning a stale .tmp should be silent, got:\n%s", res.Stderr)
	}
}

// TestE3RootDiscoveryStopsAtGitBoundary covers §E3: project-root discovery walks
// up looking for .forgectl/ but stops at the git boundary — a .forgectl/ that
// lives above the git root is never adopted.
func TestE3RootDiscoveryStopsAtGitBoundary(t *testing.T) {
	p := NewProject(t) // gives us an isolated HOME to reuse

	base := t.TempDir()
	// Decoy .forgectl ABOVE the git root.
	if err := os.MkdirAll(filepath.Join(base, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	// A git repo one level down.
	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	gitInit := exec.Command("git", "init", "-q")
	gitInit.Dir = repo
	gitInit.Env = p.env()
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// From inside the repo, discovery must hit the git boundary and refuse to
	// climb to base/.forgectl.
	res := p.forgeAt(repo, "status")
	if res.Exit == 0 {
		t.Fatal("status succeeded; the decoy .forgectl above the git root was adopted")
	}
	if !strings.Contains(res.Out(), "No .forgectl directory found") {
		t.Errorf("expected the no-.forgectl error, got:\n%s", res.Out())
	}
}
