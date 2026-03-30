package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRootFoundInCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	// Resolve both to handle symlinks (macOS /var → /private/var).
	gotAbs, _ := filepath.EvalSymlinks(root)
	wantAbs, _ := filepath.EvalSymlinks(dir)
	if gotAbs != wantAbs {
		t.Errorf("root = %q, want %q", gotAbs, wantAbs)
	}
}

func TestFindProjectRootFoundTwoLevelsUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".forgectl"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory two levels down.
	subDir := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot: %v", err)
	}
	gotAbs, _ := filepath.EvalSymlinks(root)
	wantAbs, _ := filepath.EvalSymlinks(dir)
	if gotAbs != wantAbs {
		t.Errorf("root = %q, want %q", gotAbs, wantAbs)
	}
}

func TestFindProjectRootNotFound(t *testing.T) {
	// Use /tmp directly — guaranteed to have no .forgectl unless someone put one there.
	dir := t.TempDir()

	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err := FindProjectRoot()
	if err == nil {
		t.Error("expected error when .forgectl not found, got nil")
	}
}
