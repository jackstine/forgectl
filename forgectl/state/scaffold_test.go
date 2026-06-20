package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestEmbeddedTemplateMatchesDocs guards against drift between the embedded
// default config and its canonical reference at docs/default-config.toml.
func TestEmbeddedTemplateMatchesDocs(t *testing.T) {
	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "default-config.toml"))
	if err != nil {
		t.Fatalf("reading docs/default-config.toml: %v", err)
	}
	if string(docs) != DefaultConfigTemplate() {
		t.Error("embedded default-config.toml differs from docs/default-config.toml — copy the canonical reference into forgectl/state/default-config.toml")
	}
}

// TestScaffoldCreatesDirAndConfig verifies bootstrap when nothing exists.
func TestScaffoldCreatesDirAndConfig(t *testing.T) {
	dir := t.TempDir()

	res, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.ProjectRoot != dir {
		t.Errorf("ProjectRoot = %q, want %q", res.ProjectRoot, dir)
	}
	if !res.CreatedDir || !res.CreatedConfig {
		t.Errorf("CreatedDir=%v CreatedConfig=%v, want both true", res.CreatedDir, res.CreatedConfig)
	}

	if info, err := os.Stat(filepath.Join(dir, ".forgectl")); err != nil || !info.IsDir() {
		t.Errorf(".forgectl directory not created: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".forgectl", "config"))
	if err != nil {
		t.Fatalf("reading written config: %v", err)
	}
	if string(got) != DefaultConfigTemplate() {
		t.Error("written config does not equal embedded default template")
	}
}

// TestScaffoldReusesAncestorRoot verifies project-root discovery and the
// create-at-most-one-root invariant.
func TestScaffoldReusesAncestorRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	// Seed a config so this run does not write one — isolating root discovery.
	if err := os.WriteFile(filepath.Join(root, ".forgectl", "config"), []byte("# seeded\n"), 0644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(deep)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.ProjectRoot != root {
		t.Errorf("ProjectRoot = %q, want ancestor %q", res.ProjectRoot, root)
	}
	if res.CreatedDir {
		t.Error("CreatedDir should be false when an ancestor .forgectl/ exists")
	}
	if _, err := os.Stat(filepath.Join(deep, ".forgectl")); !os.IsNotExist(err) {
		t.Error("no new .forgectl/ should be created in the working directory")
	}
}

// TestScaffoldWritesConfigIntoExistingDir verifies that directory presence and
// config presence are independent conditions.
func TestScaffoldWritesConfigIntoExistingDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "sub")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(deep)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.ProjectRoot != root {
		t.Errorf("ProjectRoot = %q, want %q", res.ProjectRoot, root)
	}
	if res.CreatedDir {
		t.Error("CreatedDir should be false; directory already existed")
	}
	if !res.CreatedConfig {
		t.Error("CreatedConfig should be true; config was missing")
	}
	got, err := os.ReadFile(filepath.Join(root, ".forgectl", "config"))
	if err != nil {
		t.Fatalf("reading written config: %v", err)
	}
	if string(got) != DefaultConfigTemplate() {
		t.Error("written config does not equal embedded default template")
	}
}

// TestScaffoldNeverOverwritesExistingConfig verifies the non-destructive invariant,
// including for malformed/invalid content.
func TestScaffoldNeverOverwritesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("this is not valid toml = = =\n")
	cfgPath := filepath.Join(dir, ".forgectl", "config")
	if err := os.WriteFile(cfgPath, custom, 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.CreatedConfig {
		t.Error("CreatedConfig should be false; an existing config must not be overwritten")
	}
	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Error("existing config content changed — must be byte-for-byte unchanged")
	}
}

// TestScaffoldIdempotent verifies the idempotency invariant across two runs.
func TestScaffoldIdempotent(t *testing.T) {
	dir := t.TempDir()

	first, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("first Scaffold: %v", err)
	}
	if !first.CreatedDir || !first.CreatedConfig {
		t.Fatalf("first run should create both, got %+v", first)
	}
	before, err := os.ReadFile(filepath.Join(dir, ".forgectl", "config"))
	if err != nil {
		t.Fatal(err)
	}

	second, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}
	if second.CreatedDir || second.CreatedConfig {
		t.Errorf("second run should create nothing, got %+v", second)
	}
	after, err := os.ReadFile(filepath.Join(dir, ".forgectl", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("config content changed across idempotent runs")
	}
}

// TestScaffoldWrittenDefaultEqualsBuiltinDefaults verifies the defaults-equivalence
// and written-default-is-valid invariants.
func TestScaffoldWrittenDefaultEqualsBuiltinDefaults(t *testing.T) {
	dir := t.TempDir()
	if _, err := Scaffold(dir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig on written default: %v", err)
	}
	if violations := ValidateConfig(loaded); len(violations) > 0 {
		t.Errorf("written default config failed validation: %v", violations)
	}

	// An absent config yields the built-in defaults. The written default must
	// load to the identical effective configuration.
	if !reflect.DeepEqual(loaded, DefaultForgeConfig()) {
		t.Error("loaded default config differs from built-in DefaultForgeConfig()")
	}
}

// TestScaffoldErrorsWhenDirCannotBeCreated verifies directory-creation failure
// handling when a regular file occupies the .forgectl path.
func TestScaffoldErrorsWhenDirCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	// A regular file named .forgectl occupies the path; no ancestor .forgectl/ exists.
	if err := os.WriteFile(filepath.Join(dir, ".forgectl"), []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(dir)
	if err == nil {
		t.Fatalf("expected error, got result %+v", res)
	}
	if res.CreatedConfig {
		t.Error("no config should be written when the directory cannot be created")
	}
}

// TestScaffoldStopsWalkAtGitRoot verifies the git-root boundary: a .forgectl/
// above the git root is never discovered; scaffolding creates .forgectl/ at cwd.
func TestScaffoldStopsWalkAtGitRoot(t *testing.T) {
	base := t.TempDir()
	// A .forgectl/ above the git root (simulating a home-level config).
	if err := os.MkdirAll(filepath.Join(base, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	// The git repository lives under base/, so base/.forgectl/ is above its root.
	repoDir := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(repoDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(srcDir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.ProjectRoot != srcDir {
		t.Errorf("ProjectRoot = %q, want %q (cwd)", res.ProjectRoot, srcDir)
	}
	if !res.CreatedDir {
		t.Error("CreatedDir should be true; walk stopped at git root and created .forgectl/ at cwd")
	}
	// The home-level .forgectl/ must not have been used.
	if _, err := os.Stat(filepath.Join(base, ".forgectl", "config")); !os.IsNotExist(err) {
		t.Error("home-level .forgectl/config must not be written; the boundary was crossed")
	}
	// A new .forgectl/ must exist at srcDir.
	if info, err := os.Stat(filepath.Join(srcDir, ".forgectl")); err != nil || !info.IsDir() {
		t.Errorf(".forgectl/ not created at cwd: %v", err)
	}
}

// TestScaffoldUsesForgetclColocatedWithGitDir verifies that a .forgectl/
// co-located with .git/ is recognized as the project root.
func TestScaffoldUsesForgetclColocatedWithGitDir(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	// Seed a config so this run does not write one — isolating root discovery.
	if err := os.WriteFile(filepath.Join(base, ".forgectl", "config"), []byte("# seeded\n"), 0644); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(base, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(srcDir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.ProjectRoot != base {
		t.Errorf("ProjectRoot = %q, want git root %q", res.ProjectRoot, base)
	}
	if res.CreatedDir {
		t.Error("CreatedDir should be false; .forgectl/ co-located with .git/ must be reused")
	}
	if _, err := os.Stat(filepath.Join(srcDir, ".forgectl")); !os.IsNotExist(err) {
		t.Error("no new .forgectl/ should be created in the working directory")
	}
}

// TestScaffoldFallsBackToFullWalkOutsideGitRepo verifies that when no .git/
// exists anywhere, the walk continues to the filesystem root as before.
func TestScaffoldFallsBackToFullWalkOutsideGitRepo(t *testing.T) {
	base := t.TempDir()
	// .forgectl/ two levels up, no .git/ anywhere in the tree.
	if err := os.MkdirAll(filepath.Join(base, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, ".forgectl", "config"), []byte("# seeded\n"), 0644); err != nil {
		t.Fatal(err)
	}
	deepDir := filepath.Join(base, "a", "b")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}

	res, err := Scaffold(deepDir)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	if res.ProjectRoot != base {
		t.Errorf("ProjectRoot = %q, want ancestor %q", res.ProjectRoot, base)
	}
	if res.CreatedDir {
		t.Error("CreatedDir should be false; ancestor .forgectl/ must be reused")
	}
}

// TestScaffoldResumableAfterConfigWriteFailure verifies partial-bootstrap error
// handling: a directory occupying the config path fails the write, leaving the
// created .forgectl/ in place and no partial config; a re-run completes the bootstrap.
func TestScaffoldResumableAfterConfigWriteFailure(t *testing.T) {
	dir := t.TempDir()
	forgectlDir := filepath.Join(dir, ".forgectl")
	if err := os.MkdirAll(forgectlDir, 0755); err != nil {
		t.Fatal(err)
	}
	// A directory occupies the config path, so the atomic write's rename fails.
	cfgPath := filepath.Join(forgectlDir, "config")
	if err := os.MkdirAll(cfgPath, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Scaffold(dir); err == nil {
		t.Fatal("expected config write to fail when a directory occupies the config path")
	}
	// No partial temp file should remain.
	if _, err := os.Stat(cfgPath + ".tmp"); !os.IsNotExist(err) {
		t.Error("partial temp config file left behind after failed write")
	}

	// Remove the cause and re-run: bootstrap completes, reusing the directory.
	if err := os.Remove(cfgPath); err != nil {
		t.Fatal(err)
	}
	res, err := Scaffold(dir)
	if err != nil {
		t.Fatalf("re-run Scaffold: %v", err)
	}
	if res.CreatedDir {
		t.Error("CreatedDir should be false on re-run; .forgectl/ already existed")
	}
	if !res.CreatedConfig {
		t.Error("CreatedConfig should be true on re-run after the cause was removed")
	}
}
