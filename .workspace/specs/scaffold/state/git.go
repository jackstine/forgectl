package state

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitCommit stages the given files, commits with the message, and returns the short commit hash.
// It runs git commands from the given working directory.
func GitCommit(workDir string, files []string, message string) (string, error) {
	// Stage files.
	addArgs := append([]string{"add"}, files...)
	addCmd := exec.Command("git", addArgs...)
	addCmd.Dir = workDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add failed: %s\n%s", err, string(out))
	}

	// Commit.
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = workDir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit failed: %s\n%s", err, string(out))
	}

	// Get short hash.
	hashCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	hashCmd.Dir = workDir
	out, err := hashCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %s", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// GitHashExists checks if a commit hash exists in the repository.
func GitHashExists(workDir string, hash string) error {
	cmd := exec.Command("git", "cat-file", "-t", hash)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("commit %q does not exist in the repository", hash)
	}
	objType := strings.TrimSpace(string(out))
	if objType != "commit" {
		return fmt.Errorf("%q is a %s, not a commit", hash, objType)
	}
	return nil
}

// GitRepoRoot returns the root directory of the git repository containing workDir.
func GitRepoRoot(workDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", err)
	}
	return strings.TrimSpace(string(out)), nil
}
