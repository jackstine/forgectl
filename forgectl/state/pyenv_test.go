package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"forgectl/buildinfo"
)

func TestResolvePythonEnvDevOverride(t *testing.T) {
	// Create a fake venv with a python binary.
	tmp := t.TempDir()
	var pythonBin string
	if runtime.GOOS == "windows" {
		pythonBin = filepath.Join(tmp, ".venv", "Scripts", "python.exe")
	} else {
		pythonBin = filepath.Join(tmp, ".venv", "bin", "python")
	}
	os.MkdirAll(filepath.Dir(pythonBin), 0755)
	os.WriteFile(pythonBin, []byte("#!/bin/sh\n"), 0755)

	t.Setenv("FORGECTL_PYTHON_PROJECT", tmp)

	cfg, err := ResolvePythonEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PythonBin != pythonBin {
		t.Errorf("PythonBin = %q, want %q", cfg.PythonBin, pythonBin)
	}
	if cfg.ProjectDir != tmp {
		t.Errorf("ProjectDir = %q, want %q", cfg.ProjectDir, tmp)
	}
}

func TestResolvePythonEnvDevOverrideMissingVenv(t *testing.T) {
	tmp := t.TempDir() // no .venv created

	t.Setenv("FORGECTL_PYTHON_PROJECT", tmp)

	_, err := ResolvePythonEnv()
	if err == nil {
		t.Fatal("expected error for missing venv, got nil")
	}
	if got := err.Error(); !contains(got, "venv python not found") {
		t.Errorf("error = %q, want it to mention venv python not found", got)
	}
}

func TestResolvePythonEnvMissingInstall(t *testing.T) {
	// Point dataDir at a temp location with no installed env.
	tmp := t.TempDir()
	t.Setenv("FORGECTL_PYTHON_PROJECT", "")
	origVersion := buildinfo.Version
	buildinfo.Version = "v99.99.99-test"
	defer func() { buildinfo.Version = origVersion }()

	// Override HOME so dataDir() resolves to the temp dir.
	t.Setenv("HOME", tmp)

	_, err := ResolvePythonEnv()
	if err == nil {
		t.Fatal("expected error for missing install, got nil")
	}
	if got := err.Error(); !contains(got, "Python environment not installed") {
		t.Errorf("error = %q, want it to mention not installed", got)
	}
}

func TestCleanEnvStripsVars(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"PYTHONPATH=/bad",
		"PYTHONHOME=/bad",
		"VIRTUAL_ENV=/bad",
		"CONDA_PREFIX=/bad",
		"CONDA_DEFAULT_ENV=base",
		"PIP_INDEX_URL=http://bad",
		"PYENV_ROOT=/bad",
		"_OLD_VIRTUAL_PATH=/bad",
		"EDITOR=vim",
	}

	result := cleanEnv(input)

	allowed := map[string]bool{
		"PATH=/usr/bin":  true,
		"HOME=/home/user": true,
		"EDITOR=vim":      true,
	}

	if len(result) != len(allowed) {
		t.Errorf("cleanEnv returned %d vars, want %d: %v", len(result), len(allowed), result)
	}
	for _, env := range result {
		if !allowed[env] {
			t.Errorf("cleanEnv should have stripped %q", env)
		}
	}
}

func TestVerifyChecksumsValid(t *testing.T) {
	tmp := t.TempDir()

	// Create a file and compute its checksum.
	srcDir := filepath.Join(tmp, "src", "reverse_engineer")
	os.MkdirAll(srcDir, 0755)
	testFile := filepath.Join(srcDir, "cli.py")
	content := []byte("print('hello')\n")
	os.WriteFile(testFile, content, 0644)

	hash, err := hashFile(testFile)
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}

	// Write checksums file.
	checksumContent := hash + "  src/reverse_engineer/cli.py\n"
	os.WriteFile(filepath.Join(tmp, "checksums.sha256"), []byte(checksumContent), 0644)

	if err := verifyChecksums(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyChecksumsMismatch(t *testing.T) {
	tmp := t.TempDir()

	srcDir := filepath.Join(tmp, "src", "reverse_engineer")
	os.MkdirAll(srcDir, 0755)
	testFile := filepath.Join(srcDir, "cli.py")
	os.WriteFile(testFile, []byte("print('hello')\n"), 0644)

	// Write a wrong checksum.
	checksumContent := "0000000000000000000000000000000000000000000000000000000000000000  src/reverse_engineer/cli.py\n"
	os.WriteFile(filepath.Join(tmp, "checksums.sha256"), []byte(checksumContent), 0644)

	err := verifyChecksums(tmp)
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
	if got := err.Error(); !contains(got, "checksum mismatch") {
		t.Errorf("error = %q, want it to mention checksum mismatch", got)
	}
}

func TestVerifyChecksumsMissingFile(t *testing.T) {
	tmp := t.TempDir() // no checksums.sha256

	err := verifyChecksums(tmp)
	if err == nil {
		t.Fatal("expected error for missing checksums file, got nil")
	}
	if got := err.Error(); !contains(got, "missing checksums file") {
		t.Errorf("error = %q, want it to mention missing checksums file", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
