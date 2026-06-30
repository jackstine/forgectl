//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"forgectl/state"
)

// TestG1BareInitScaffolds covers §G1: a bare `init` (no flags) scaffolds
// .forgectl/ + a default config, exits 0, and prints the domains notice exactly
// once; a re-run is an idempotent no-op that prints no notice.
func TestG1BareInitScaffolds(t *testing.T) {
	p := NewProject(t)

	res := p.mustForge("init")
	if !p.Exists(".forgectl/config") {
		t.Fatal("bare init did not create .forgectl/config")
	}
	const notice = "Created default .forgectl/config"
	if !strings.Contains(res.Stdout, notice) {
		t.Errorf("first init missing scaffold notice:\n%s", res.Stdout)
	}
	if c := strings.Count(res.Stdout, notice); c != 1 {
		t.Errorf("scaffold notice printed %d times, want exactly once", c)
	}
	// No --from → no session created.
	if p.StateExists() {
		t.Error("bare init created a state file")
	}

	before := p.ReadFile(".forgectl/config")
	res2 := p.mustForge("init")
	if strings.Contains(res2.Stdout, notice) {
		t.Errorf("re-run init printed the scaffold notice again:\n%s", res2.Stdout)
	}
	if after := p.ReadFile(".forgectl/config"); after != before {
		t.Error("re-run init mutated the config")
	}
}

// TestG2ExistingConfigPreserved covers §G2: an existing config is never read,
// modified, or overwritten by scaffolding — it is preserved byte-for-byte.
func TestG2ExistingConfigPreserved(t *testing.T) {
	p := NewProject(t)
	custom := "# my custom config\n[specifying]\nbatch = 7\n"
	p.WriteConfig(custom)

	res := p.mustForge("init")
	if strings.Contains(res.Stdout, "Created default") {
		t.Errorf("init claimed to scaffold over an existing config:\n%s", res.Stdout)
	}
	if got := p.ReadFile(".forgectl/config"); got != custom {
		t.Errorf("existing config was modified.\n got: %q\nwant: %q", got, custom)
	}
}

// TestG3PartialConfigMergesAndLocks covers §G3: a partial config merges onto
// defaults, and the effective config is locked into state at init — later edits
// to the file on disk do not change a running session.
func TestG3PartialConfigMergesAndLocks(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[specifying]\nbatch = 5\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	s := p.State()
	if s.Config.Specifying.Batch != 5 {
		t.Errorf("specifying.batch = %d, want merged value 5", s.Config.Specifying.Batch)
	}
	if s.Config.Planning.Batch != 1 {
		t.Errorf("planning.batch = %d, want default 1 (untouched field)", s.Config.Planning.Batch)
	}
	if s.Config.Specifying.Eval.MaxRounds != 3 {
		t.Errorf("specifying.eval.max_rounds = %d, want default 3", s.Config.Specifying.Eval.MaxRounds)
	}

	// Lock check: change the on-disk config, then run a read-only command. The
	// locked-in value must win.
	p.WriteConfig("[specifying]\nbatch = 9\n")
	status := p.mustForge("status")
	if !strings.Contains(status.Stdout, "batch=5") {
		t.Errorf("status used the edited on-disk config instead of the locked one:\n%s", status.Stdout)
	}
}

// TestG3GuidedMutableAtRuntime covers §G3: general.user_guided is the one config
// field mutable at runtime, via --guided / --no-guided on any advance.
func TestG3GuidedMutableAtRuntime(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("") // empty → all defaults; user_guided defaults true
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	if !p.State().Config.General.UserGuided {
		t.Fatal("user_guided should default to true")
	}
	p.mustForge("advance", "--no-guided") // ORIENT → SELECT
	if p.State().Config.General.UserGuided {
		t.Error("--no-guided did not clear user_guided")
	}
	p.mustForge("advance", "--guided") // SELECT → DRAFT
	if !p.State().Config.General.UserGuided {
		t.Error("--guided did not set user_guided")
	}
}

// Test44DefaultConfigEquivalence covers §4.4: the embedded default-config.toml
// template and DefaultForgeConfig() are two independent encodings of the same
// defaults. Parsing the shipped template must yield a config field-for-field
// equal to the Go source of truth and equal to an empty config, and the
// scaffolded default must pass ValidateConfig. This is a pure independent-source
// cross-check (no binary needed).
func Test44DefaultConfigEquivalence(t *testing.T) {
	goDefault := state.DefaultForgeConfig()

	// Parse the embedded template through the real loader.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forgectl", "config"),
		[]byte(state.DefaultConfigTemplate()), 0644); err != nil {
		t.Fatal(err)
	}
	fromTemplate, err := state.LoadConfig(root)
	if err != nil {
		t.Fatalf("loading scaffolded template: %v", err)
	}
	if !reflect.DeepEqual(goDefault, fromTemplate) {
		t.Errorf("embedded template != DefaultForgeConfig():\n default: %+v\ntemplate: %+v", goDefault, fromTemplate)
	}

	// An empty config must resolve to the same effective config.
	if err := os.WriteFile(filepath.Join(root, ".forgectl", "config"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	fromEmpty, err := state.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(goDefault, fromEmpty) {
		t.Errorf("empty config != DefaultForgeConfig()")
	}

	// The scaffolded default must validate clean.
	if v := state.ValidateConfig(fromTemplate); len(v) != 0 {
		t.Errorf("scaffolded default config fails ValidateConfig: %v", v)
	}
}

// TestPlanningStudySpecsConfigIsHonored asserts the documented behavior of the
// [planning.study_specs] config block: a value set there should be reflected in
// the loaded config (the shipped template documents it as configurable, and
// DefaultForgeConfig has a StudySpecs field).
//
// KNOWN BUG (skipped): tomlPlanningConfig has no study_specs field, so
// mergeTomlConfig silently drops any [planning.study_specs] the operator sets —
// the loaded value stays at the default. This is the default-config.toml ⟷
// loader mismatch flagged in the config-scaffolding spec. The assertion below is
// written against the intended behavior; un-skip it once the loader is fixed.
func TestPlanningStudySpecsConfigIsHonored(t *testing.T) {

	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".forgectl"), 0755)
	_ = os.WriteFile(filepath.Join(root, ".forgectl", "config"),
		[]byte("[planning.study_specs]\nmodel = \"opus\"\ncount = 9\n"), 0644)
	cfg, err := state.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Planning.StudySpecs.Model != "opus" || cfg.Planning.StudySpecs.Count != 9 {
		t.Errorf("study_specs = %q/%d, want opus/9 (config block was dropped)",
			cfg.Planning.StudySpecs.Model, cfg.Planning.StudySpecs.Count)
	}
}
