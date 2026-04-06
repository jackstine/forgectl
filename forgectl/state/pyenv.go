package state

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"forgectl/buildinfo"
)

// PythonEnvConfig holds the resolved paths for the Python environment.
type PythonEnvConfig struct {
	PythonBin  string // Absolute path to the venv python binary.
	ProjectDir string // Absolute path to the installed Python project.
}

// ResolvePythonEnv locates the installed Python environment for the current
// forgectl version. If FORGECTL_PYTHON_PROJECT is set, it uses that directory
// instead (dev mode, no checksum verification).
func ResolvePythonEnv() (PythonEnvConfig, error) {
	if devDir := os.Getenv("FORGECTL_PYTHON_PROJECT"); devDir != "" {
		return resolveDevEnv(devDir)
	}
	return resolveInstalledEnv()
}

func resolveDevEnv(devDir string) (PythonEnvConfig, error) {
	pythonBin := venvPythonPath(devDir)
	if _, err := os.Stat(pythonBin); err != nil {
		return PythonEnvConfig{}, fmt.Errorf(
			"FORGECTL_PYTHON_PROJECT=%s but venv python not found at %s — run: cd %s && uv sync",
			devDir, pythonBin, devDir,
		)
	}
	return PythonEnvConfig{PythonBin: pythonBin, ProjectDir: devDir}, nil
}

func resolveInstalledEnv() (PythonEnvConfig, error) {
	installDir := filepath.Join(dataDir(), buildinfo.Version, "python")
	pythonBin := venvPythonPath(installDir)

	if _, err := os.Stat(pythonBin); err != nil {
		return PythonEnvConfig{}, fmt.Errorf(
			"forgectl %s: Python environment not installed at %s\n"+
				"Run the installer: make install-global (from source) or install-forgectl.sh",
			buildinfo.Version, installDir,
		)
	}

	if err := verifyChecksums(installDir); err != nil {
		return PythonEnvConfig{}, err
	}

	return PythonEnvConfig{PythonBin: pythonBin, ProjectDir: installDir}, nil
}

// venvPythonPath returns the path to the venv python binary within a project directory.
func venvPythonPath(projectDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(projectDir, ".venv", "Scripts", "python.exe")
	}
	return filepath.Join(projectDir, ".venv", "bin", "python")
}

// dataDir returns the platform-specific data directory for forgectl.
func dataDir() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "forgectl")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "forgectl")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "forgectl")
}

// cleanEnv returns a copy of the given environment with Python-related
// variables stripped, preventing host Python from leaking into the subprocess.
func cleanEnv(base []string) []string {
	skipPrefixes := []string{
		"PYTHONPATH=",
		"PYTHONHOME=",
		"PYTHONSTARTUP=",
		"VIRTUAL_ENV=",
		"CONDA_PREFIX=",
		"CONDA_DEFAULT_ENV=",
		"CONDA_PYTHON_EXE=",
		"PIP_",
		"PYENV_",
		"_OLD_VIRTUAL_",
	}

	var clean []string
	for _, env := range base {
		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(env, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			clean = append(clean, env)
		}
	}
	return clean
}

// verifyChecksums reads checksums.sha256 from installDir and verifies each
// listed file's SHA-256 hash matches. Returns nil if all checksums match.
func verifyChecksums(installDir string) error {
	checksumFile := filepath.Join(installDir, "checksums.sha256")
	f, err := os.Open(checksumFile)
	if err != nil {
		return fmt.Errorf(
			"Python environment integrity: missing checksums file at %s\nRe-run the installer",
			checksumFile,
		)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: "<sha256hex>  <relative-path>" (two spaces between hash and path)
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}
		expectedHash := parts[0]
		relPath := parts[1]

		absPath := filepath.Join(installDir, relPath)
		actualHash, err := hashFile(absPath)
		if err != nil {
			return fmt.Errorf("Python environment integrity: cannot read %s: %w", relPath, err)
		}
		if actualHash != expectedHash {
			return fmt.Errorf(
				"Python environment integrity: checksum mismatch for %s\n"+
					"  expected: %s\n"+
					"  actual:   %s\n"+
					"Re-run the installer",
				relPath, expectedHash, actualHash,
			)
		}
	}
	return scanner.Err()
}

// hashFile returns the hex-encoded SHA-256 hash of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := bufio.NewReader(f).WriteTo(h); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
